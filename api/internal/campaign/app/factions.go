package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// FactionRepository keeps a Campaign's Factions, the Standing they hold the party and its Characters
// in, and the Standing Changes suggested and decided.
type FactionRepository interface {
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	Factions(ctx context.Context, id domain.CampaignID) ([]domain.Faction, error)
	InsertFaction(ctx context.Context, f domain.Faction, now time.Time) error
	// UpdateFaction and DeleteFaction report ErrNotFound for a Faction the Campaign does not have.
	UpdateFaction(ctx context.Context, f domain.Faction, now time.Time) error
	DeleteFaction(ctx context.Context, id domain.CampaignID, faction domain.FactionID) error
	Faction(ctx context.Context, id domain.CampaignID, faction domain.FactionID) (domain.Faction, error)
	PersonalStandings(ctx context.Context, id domain.CampaignID) ([]domain.PersonalStanding, error)
	// CharacterOwner reports ErrNotFound for a Character the Campaign does not have.
	CharacterOwner(ctx context.Context, id domain.CampaignID, character domain.CharacterID) (domain.MemberID, error)
	StandingChanges(ctx context.Context, id domain.CampaignID) ([]domain.StandingChange, error)
	InsertStandingChange(ctx context.Context, c domain.StandingChange) error
	// DecideStandingChange holds a pending change of the Campaign while decide rules on it, and keeps
	// what it rules: the change as decided, and the score it leaves when it is confirmed. It reports
	// ErrNotFound for a change the Campaign does not have, and ErrConflict for one already decided.
	DecideStandingChange(ctx context.Context, id domain.CampaignID, change domain.ChangeID, now time.Time, decide func(domain.StandingChange, int) (domain.StandingChange, *int)) error
}

// Factions runs the Faction use cases. The DM keeps Factions and decides every Standing Change; a
// change is only ever suggested, by whatever proposes it, and Players are shown tiers and the reasons
// the DM chose to share.
type Factions struct {
	Repo FactionRepository
	Now  func() time.Time
}

// FactionInput is the editable part of a Faction.
type FactionInput struct {
	Name      string
	Archetype string
	Goals     string
	Territory string
	Notes     string
}

// Personal is a Character's own Standing with a Faction. Score is the DM's alone.
type Personal struct {
	CharacterID domain.CharacterID
	Character   string
	Tier        standing.Tier
	Score       *int
}

// Change is a Standing Change as one Member may see it. A Player sees only confirmed ones: which way
// the Standing moved, and the reason when the DM shared it. The amount, where it came from and
// whether the reason is shared are the DM's alone.
type Change struct {
	ID          domain.ChangeID
	CharacterID *domain.CharacterID
	Rose        bool
	Reason      string
	Status      string
	CreatedAt   time.Time
	DM          *ChangeDetail
}

// ChangeDetail is what only the DM sees of a Standing Change.
type ChangeDetail struct {
	Delta       int
	ShareReason bool
	Origin      string
	Client      string
}

// FactionView is a Faction as one Member may see it: its name and the tier of its Standing for
// everyone; its goals, territory, notes and score for the DM alone.
type FactionView struct {
	Faction  domain.Faction
	Tier     standing.Tier
	DM       bool
	Personal []Personal
	Changes  []Change
}

func (s *Factions) member(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Member, error) {
	return s.Repo.Membership(ctx, id, c.Subject)
}

func (s *Factions) dm(ctx context.Context, c caller.Caller, id domain.CampaignID) error {
	me, err := s.member(ctx, c, id)
	if err != nil {
		return err
	}
	if me.Role != domain.RoleDM {
		return domain.ErrForbidden
	}
	return nil
}

// List shows the Campaign's Factions to a Member. Players get names, tiers, their own Characters'
// Personal Standing, and the confirmed changes with the reasons the DM shared: never a number.
func (s *Factions) List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]FactionView, error) {
	me, err := s.member(ctx, c, id)
	if err != nil {
		return nil, err
	}
	factions, err := s.Repo.Factions(ctx, id)
	if err != nil {
		return nil, err
	}
	personal, err := s.Repo.PersonalStandings(ctx, id)
	if err != nil {
		return nil, err
	}
	changes, err := s.Repo.StandingChanges(ctx, id)
	if err != nil {
		return nil, err
	}
	dm := me.Role == domain.RoleDM
	mine := map[domain.CharacterID]bool{}
	for _, p := range personal {
		mine[p.CharacterID] = p.Owner == me.ID
	}
	out := make([]FactionView, 0, len(factions))
	for _, f := range factions {
		out = append(out, factionView(f, dm, mine, personal, changes))
	}
	return out, nil
}

// factionView is a Faction as the DM or a Player may see it, with the Personal Standings and the
// Standing Changes they may see.
func factionView(f domain.Faction, dm bool, mine map[domain.CharacterID]bool, personal []domain.PersonalStanding, changes []domain.StandingChange) FactionView {
	v := FactionView{Faction: f, Tier: standing.TierOf(f.Score), DM: dm, Personal: []Personal{}, Changes: []Change{}}
	if !dm {
		v.Faction.Goals, v.Faction.Territory, v.Faction.Notes, v.Faction.Score = "", "", "", 0
	}
	for _, p := range personal {
		if p.FactionID == f.ID && (dm || mine[p.CharacterID]) {
			v.Personal = append(v.Personal, personalView(p, dm))
		}
	}
	for _, ch := range changes {
		if view, ok := changeView(ch, dm, mine); ok && ch.FactionID == f.ID {
			v.Changes = append(v.Changes, view)
		}
	}
	return v
}

func personalView(p domain.PersonalStanding, dm bool) Personal {
	out := Personal{CharacterID: p.CharacterID, Character: p.Character, Tier: standing.TierOf(p.Score), Score: nil}
	if dm {
		score := p.Score
		out.Score = &score
	}
	return out
}

// changeView is a Standing Change as the DM or a Player may see it, and whether they may see it at
// all: a Player sees confirmed changes for the party and for their own Characters.
func changeView(ch domain.StandingChange, dm bool, mine map[domain.CharacterID]bool) (Change, bool) {
	out := Change{ID: ch.ID, CharacterID: ch.Character, Rose: ch.Delta > 0, Reason: ch.Reason, Status: ch.Status, CreatedAt: ch.CreatedAt, DM: nil}
	if dm {
		out.DM = &ChangeDetail{Delta: ch.Delta, ShareReason: ch.ShareReason, Origin: ch.Origin, Client: ch.Client}
		return out, true
	}
	if ch.Status != domain.ChangeConfirmed || (ch.Character != nil && !mine[*ch.Character]) {
		return Change{}, false
	}
	if !ch.ShareReason {
		out.Reason = ""
	}
	return out, true
}

func (s *Factions) checked(ctx context.Context, c caller.Caller, id domain.CampaignID, in FactionInput) (domain.Faction, error) {
	if err := s.dm(ctx, c, id); err != nil {
		return domain.Faction{}, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 80 || utf8.RuneCountInString(in.Goals) > 2000 || utf8.RuneCountInString(in.Territory) > 2000 || utf8.RuneCountInString(in.Notes) > 2000 {
		return domain.Faction{}, domain.ErrInvalid
	}
	if _, known := standing.Archetype(in.Archetype); in.Archetype != "" && !known {
		return domain.Faction{}, refuse("there is no such Faction Archetype")
	}
	return domain.Faction{
		ID: uuid.Nil, CampaignID: id, Name: name, Archetype: in.Archetype, Goals: in.Goals, Territory: in.Territory, Notes: in.Notes, Score: 0, UpdatedAt: s.Now(),
	}, nil
}

// Create adds a Faction, Neutral towards the party. DM only.
func (s *Factions) Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in FactionInput) (FactionView, error) {
	f, err := s.checked(ctx, c, id, in)
	if err != nil {
		return FactionView{}, err
	}
	f.ID = uuid.New()
	return FactionView{Faction: f, Tier: standing.TierOf(f.Score), DM: true, Personal: []Personal{}, Changes: []Change{}}, s.Repo.InsertFaction(ctx, f, f.UpdatedAt)
}

// Update changes a Faction's name and what the DM knows of it; its Standing only moves by a Standing
// Change. DM only.
func (s *Factions) Update(ctx context.Context, c caller.Caller, id domain.CampaignID, faction domain.FactionID, in FactionInput) error {
	f, err := s.checked(ctx, c, id, in)
	if err != nil {
		return err
	}
	f.ID = faction
	return s.Repo.UpdateFaction(ctx, f, f.UpdatedAt)
}

// Delete removes a Faction with its Standing and its changes. DM only.
func (s *Factions) Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, faction domain.FactionID) error {
	if err := s.dm(ctx, c, id); err != nil {
		return err
	}
	return s.Repo.DeleteFaction(ctx, id, faction)
}

// Suggestion is a Standing Change someone proposes.
type Suggestion struct {
	Character   *domain.CharacterID
	Delta       int
	Reason      string
	ShareReason bool
}

func (g Suggestion) valid() bool {
	reason := strings.TrimSpace(g.Reason)
	return standing.ValidDelta(g.Delta) && reason != "" && utf8.RuneCountInString(reason) <= 500
}

// Propose suggests a Standing Change. It moves nothing: it waits for the DM to confirm it, whoever or
// whatever proposed it. DM only.
func (s *Factions) Propose(ctx context.Context, c caller.Caller, id domain.CampaignID, faction domain.FactionID, g Suggestion) (domain.StandingChange, error) {
	if err := s.dm(ctx, c, id); err != nil {
		return domain.StandingChange{}, err
	}
	if !g.valid() {
		return domain.StandingChange{}, domain.ErrInvalid
	}
	if _, err := s.Repo.Faction(ctx, id, faction); err != nil {
		return domain.StandingChange{}, err
	}
	if g.Character != nil {
		if _, err := s.Repo.CharacterOwner(ctx, id, *g.Character); errors.Is(err, domain.ErrNotFound) {
			return domain.StandingChange{}, refuse("a Personal Standing belongs to a Character of this Campaign")
		} else if err != nil {
			return domain.StandingChange{}, err
		}
	}
	change := domain.StandingChange{
		ID: uuid.New(), FactionID: faction, Character: g.Character, Delta: g.Delta, Reason: strings.TrimSpace(g.Reason), ShareReason: g.ShareReason,
		Status: domain.ChangePending, Origin: string(c.Origin), Client: c.Client, ProposedBy: c.Subject, CreatedAt: s.Now(), DecidedAt: nil,
	}
	return change, s.Repo.InsertStandingChange(ctx, change)
}

// Decision is the DM's word on a pending Standing Change: confirm it, as suggested or edited, or
// dismiss it. What is left nil stays as it was suggested.
type Decision struct {
	Confirm     bool
	Delta       *int
	Reason      *string
	ShareReason *bool
}

// Decide confirms or dismisses a pending Standing Change. Only the DM decides, and only in person: an
// agent acting for the DM may suggest a change, never confirm one. A confirmed change moves the
// party's Standing, or the Character's Personal Standing, which starts from the party's.
func (s *Factions) Decide(ctx context.Context, c caller.Caller, id domain.CampaignID, change domain.ChangeID, d Decision) error {
	if err := s.dm(ctx, c, id); err != nil {
		return err
	}
	if c.Origin != caller.OriginUI {
		return domain.ErrForbidden
	}
	edited := Suggestion{Character: nil, Delta: 1, Reason: "-", ShareReason: false}
	if d.Delta != nil {
		edited.Delta = *d.Delta
	}
	if d.Reason != nil {
		edited.Reason = *d.Reason
	}
	if !edited.valid() {
		return domain.ErrInvalid
	}
	return s.Repo.DecideStandingChange(ctx, id, change, s.Now(), func(ch domain.StandingChange, score int) (domain.StandingChange, *int) {
		if !d.Confirm {
			ch.Status = domain.ChangeDismissed
			return ch, nil
		}
		if d.Delta != nil {
			ch.Delta = *d.Delta
		}
		if d.Reason != nil {
			ch.Reason = strings.TrimSpace(*d.Reason)
		}
		if d.ShareReason != nil {
			ch.ShareReason = *d.ShareReason
		}
		ch.Status = domain.ChangeConfirmed
		after := standing.Apply(score, ch.Delta)
		return ch, &after
	})
}
