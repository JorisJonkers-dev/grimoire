// Package app holds the campaign use cases. Every one authorises the Caller against the Campaign.
package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Repository is the persistence port.
type Repository interface {
	InTx(ctx context.Context, fn func(Repository) error) error
	CreateCampaign(ctx context.Context, name, ruleset, subject string, now time.Time) (domain.Campaign, error)
	UpdateCampaign(ctx context.Context, id domain.CampaignID, change domain.SettingsChange, now time.Time) (domain.Campaign, error)
	GetCampaign(ctx context.Context, id domain.CampaignID) (domain.Campaign, error)
	ListCampaigns(ctx context.Context, subject string, after *domain.ListCursor, pageSize int) ([]domain.Summary, error)
	LockCampaign(ctx context.Context, id domain.CampaignID) error
	AddMember(ctx context.Context, m domain.Member) (domain.Member, error)
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	Member(ctx context.Context, id domain.CampaignID, member domain.MemberID) (domain.Member, error)
	Members(ctx context.Context, id domain.CampaignID) ([]domain.Member, error)
	SetRole(ctx context.Context, id domain.CampaignID, member domain.MemberID, role domain.Role) error
	RemoveMember(ctx context.Context, id domain.CampaignID, member domain.MemberID) error
	CountDMs(ctx context.Context, id domain.CampaignID) (int, error)
	CreateInvite(ctx context.Context, id domain.CampaignID, hash []byte, by domain.MemberID, now, expires time.Time) (domain.Invite, error)
	Invites(ctx context.Context, id domain.CampaignID, now time.Time) ([]domain.Invite, error)
	RevokeInvite(ctx context.Context, id domain.CampaignID, invite domain.InviteID, now time.Time) (bool, error)
	FindInvite(ctx context.Context, hash []byte, now time.Time) (domain.InvitePreview, error)
	InsertCharacter(ctx context.Context, c domain.Character, now time.Time) (domain.CharacterID, error)
	Character(ctx context.Context, id domain.CampaignID, ch domain.CharacterID) (domain.Character, error)
	Characters(ctx context.Context, id domain.CampaignID) ([]domain.Character, error)
	UpdateCharacter(ctx context.Context, c domain.Character, now time.Time) error
	DeleteCharacter(ctx context.Context, id domain.CampaignID, ch domain.CharacterID) error
	SetCharacterImage(ctx context.Context, id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind, img *domain.Image, now time.Time) error
	InsertNPC(ctx context.Context, id domain.CampaignID, npcID *domain.NPCID, n domain.NPC, now time.Time) (domain.NPCID, error)
	UpdateNPC(ctx context.Context, id domain.CampaignID, n domain.NPC, now time.Time) (bool, error)
	DeleteNPC(ctx context.Context, id domain.CampaignID, npcID domain.NPCID) (bool, error)
	NPC(ctx context.Context, id domain.CampaignID, npcID domain.NPCID) (domain.NPC, error)
	NPCs(ctx context.Context, id domain.CampaignID) ([]domain.NPC, error)
	DeletedNPCs(ctx context.Context, id domain.CampaignID) ([]domain.DeletedNPC, error)
	RecordNPCRevision(ctx context.Context, id domain.CampaignID, r domain.Revision, c caller.Caller, n domain.NPC) (int, error)
	Revisions(ctx context.Context, id domain.CampaignID, t domain.EntityType, entity uuid.UUID) ([]domain.Revision, error)
	NPCRevision(ctx context.Context, id domain.CampaignID, npcID domain.NPCID, no int) (domain.NPC, error)
	Edits(ctx context.Context, id domain.CampaignID, f domain.EditFilter) ([]domain.Edit, error)
}

// DefaultInviteTTL is how long an invite link stays valid.
const DefaultInviteTTL = 7 * 24 * time.Hour

// Service runs the campaign use cases.
type Service struct {
	Repo      Repository
	Now       func() time.Time
	Token     func() (string, error)
	InviteTTL time.Duration
}

// NewService wires the production clock and token source.
func NewService(repo Repository) *Service {
	return &Service{Repo: repo, Now: time.Now, Token: RandomToken, InviteTTL: DefaultInviteTTL}
}

// RandomToken returns 32 random bytes, base64url-encoded.
func RandomToken() (string, error) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails since Go 1.24
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func cleanText(s string, limit int) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > limit {
		return "", domain.ErrInvalid
	}
	return s, nil
}

// member returns the caller's membership; a non-member learns nothing about the Campaign.
func member(ctx context.Context, repo Repository, c caller.Caller, id domain.CampaignID) (domain.Member, error) {
	return repo.Membership(ctx, id, c.Subject)
}

func dm(ctx context.Context, repo Repository, c caller.Caller, id domain.CampaignID) (domain.Member, error) {
	m, err := member(ctx, repo, c, id)
	if err != nil {
		return domain.Member{}, err
	}
	if m.Role != domain.RoleDM {
		return domain.Member{}, domain.ErrForbidden
	}
	return m, nil
}

// CreateInput is a new Campaign; its creator becomes its first DM.
type CreateInput struct {
	Name        string
	Ruleset     string
	DisplayName string
}

// Ruleset is the only rules document a Campaign plays by: SRD 5.2.
const Ruleset = "srd-2024"

// Create starts a Campaign with the caller as DM.
func (s *Service) Create(ctx context.Context, c caller.Caller, in CreateInput) (domain.Detail, error) {
	name, err := cleanText(in.Name, 80)
	if err != nil {
		return domain.Detail{}, err
	}
	display, err := cleanText(in.DisplayName, 60)
	if err != nil {
		return domain.Detail{}, err
	}
	if in.Ruleset == "" {
		in.Ruleset = Ruleset
	}
	if in.Ruleset != Ruleset {
		return domain.Detail{}, domain.ErrInvalid
	}
	var out domain.Detail
	err = s.Repo.InTx(ctx, func(r Repository) error {
		now := s.Now()
		camp, err := r.CreateCampaign(ctx, name, in.Ruleset, c.Subject, now)
		if err != nil {
			return err
		}
		me, err := r.AddMember(ctx, domain.Member{CampaignID: camp.ID, Subject: c.Subject, DisplayName: display, Role: domain.RoleDM, JoinedAt: now})
		if err != nil {
			return err
		}
		out = domain.Detail{Campaign: camp, Me: me, Members: []domain.Member{me}}
		return nil
	})
	return out, err
}

// List returns the caller's Campaigns, newest first.
func (s *Service) List(ctx context.Context, c caller.Caller, after *domain.ListCursor, pageSize int) ([]domain.Summary, error) {
	return s.Repo.ListCampaigns(ctx, c.Subject, after, pageSize)
}

// Get returns a Campaign's home for one of its Members.
func (s *Service) Get(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Detail, error) {
	me, err := member(ctx, s.Repo, c, id)
	if err != nil {
		return domain.Detail{}, err
	}
	camp, err := s.Repo.GetCampaign(ctx, id)
	if err != nil {
		return domain.Detail{}, err
	}
	members, err := s.Repo.Members(ctx, id)
	if err != nil {
		return domain.Detail{}, err
	}
	return domain.Detail{Campaign: camp, Me: me, Members: members}, nil
}

// UpdateInput changes a Campaign's settings; nil leaves a field alone.
type UpdateInput struct {
	Name    *string
	Ruleset *string
	// ReactionTimeoutS is how long Reaction Prompts wait, from 3 to 120 seconds.
	ReactionTimeoutS *int
	// HighGround turns the high-ground optional rule on or off.
	HighGround *bool
	// RestSupplies makes a Long Rest cost each resting Character a day of Rations.
	RestSupplies *bool
}

// Update changes a Campaign's settings. DM only.
func (s *Service) Update(ctx context.Context, c caller.Caller, id domain.CampaignID, in UpdateInput) (domain.Campaign, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return domain.Campaign{}, err
	}
	if in.Name != nil {
		name, err := cleanText(*in.Name, 80)
		if err != nil {
			return domain.Campaign{}, err
		}
		in.Name = &name
	}
	if in.Ruleset != nil && *in.Ruleset != Ruleset {
		return domain.Campaign{}, domain.ErrInvalid
	}
	if t := in.ReactionTimeoutS; t != nil && (*t < 3 || *t > 120) {
		return domain.Campaign{}, refuse("reactions wait between 3 and 120 seconds")
	}
	change := domain.SettingsChange{Name: in.Name, Ruleset: in.Ruleset, ReactionTimeoutS: in.ReactionTimeoutS, HighGround: in.HighGround, RestSupplies: in.RestSupplies}
	return s.Repo.UpdateCampaign(ctx, id, change, s.Now())
}

// SetRole makes a Member a DM or a Player. DM only; the last DM cannot step down.
func (s *Service) SetRole(ctx context.Context, c caller.Caller, id domain.CampaignID, target domain.MemberID, role domain.Role) (domain.Member, error) {
	if !role.Valid() {
		return domain.Member{}, domain.ErrInvalid
	}
	var out domain.Member
	err := s.Repo.InTx(ctx, func(r Repository) error {
		if err := r.LockCampaign(ctx, id); err != nil {
			return err
		}
		if _, err := dm(ctx, r, c, id); err != nil {
			return err
		}
		m, err := r.Member(ctx, id, target)
		if err != nil {
			return err
		}
		if err := keepOneDM(ctx, r, m, role); err != nil {
			return err
		}
		if err := r.SetRole(ctx, id, target, role); err != nil {
			return err
		}
		m.Role = role
		out = m
		return nil
	})
	return out, err
}

// keepOneDM refuses a change that would leave the Campaign without a DM.
func keepOneDM(ctx context.Context, r Repository, m domain.Member, next domain.Role) error {
	if m.Role != domain.RoleDM || next == domain.RoleDM {
		return nil
	}
	n, err := r.CountDMs(ctx, m.CampaignID)
	if err != nil {
		return err
	}
	if n <= 1 {
		return domain.ErrConflict
	}
	return nil
}

// RemoveMember removes a Member. A DM may remove anyone; a Player may only leave.
func (s *Service) RemoveMember(ctx context.Context, c caller.Caller, id domain.CampaignID, target domain.MemberID) error {
	return s.Repo.InTx(ctx, func(r Repository) error {
		if err := r.LockCampaign(ctx, id); err != nil {
			return err
		}
		me, err := member(ctx, r, c, id)
		if err != nil {
			return err
		}
		if me.Role != domain.RoleDM && me.ID != target {
			return domain.ErrForbidden
		}
		m, err := r.Member(ctx, id, target)
		if err != nil {
			return err
		}
		if err := keepOneDM(ctx, r, m, domain.RolePlayer); err != nil {
			return err
		}
		return r.RemoveMember(ctx, id, target)
	})
}

// CreateInvite opens an invite link that adds whoever follows it as a Player. DM only.
func (s *Service) CreateInvite(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.NewInvite, error) {
	me, err := dm(ctx, s.Repo, c, id)
	if err != nil {
		return domain.NewInvite{}, err
	}
	token, err := s.Token()
	if err != nil {
		return domain.NewInvite{}, err
	}
	now := s.Now()
	inv, err := s.Repo.CreateInvite(ctx, id, hashToken(token), me.ID, now, now.Add(s.InviteTTL))
	if err != nil {
		return domain.NewInvite{}, err
	}
	inv.CreatedByName = me.DisplayName
	return domain.NewInvite{Invite: inv, Token: token}, nil
}

// Invites lists the open invites. DM only.
func (s *Service) Invites(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.Invite, error) {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	return s.Repo.Invites(ctx, id, s.Now())
}

// RevokeInvite closes an invite link. DM only.
func (s *Service) RevokeInvite(ctx context.Context, c caller.Caller, id domain.CampaignID, invite domain.InviteID) error {
	if _, err := dm(ctx, s.Repo, c, id); err != nil {
		return err
	}
	ok, err := s.Repo.RevokeInvite(ctx, id, invite, s.Now())
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrNotFound
	}
	return nil
}

// PreviewInvite shows which Campaign an invite link leads to.
func (s *Service) PreviewInvite(ctx context.Context, token string) (domain.InvitePreview, error) {
	return s.Repo.FindInvite(ctx, hashToken(token), s.Now())
}

// AcceptInvite joins the caller to the invite's Campaign as a Player; a Member keeps their role.
func (s *Service) AcceptInvite(ctx context.Context, c caller.Caller, token, displayName string) (domain.CampaignID, error) {
	display, err := cleanText(displayName, 60)
	if err != nil {
		return domain.CampaignID{}, err
	}
	now := s.Now()
	preview, err := s.Repo.FindInvite(ctx, hashToken(token), now)
	if err != nil {
		return domain.CampaignID{}, err
	}
	_, err = s.Repo.AddMember(ctx, domain.Member{CampaignID: preview.CampaignID, Subject: c.Subject, DisplayName: display, Role: domain.RolePlayer, JoinedAt: now})
	if err != nil {
		return domain.CampaignID{}, err
	}
	return preview.CampaignID, nil
}
