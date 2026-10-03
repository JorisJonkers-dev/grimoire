package app

import (
	"cmp"
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Notice is a Proposal Notification: what happened, and where to open the Proposal.
type Notice struct {
	Title  string
	Body   string
	Label  string
	Path   string
	Dedupe string
}

// Notifier rings an account's bell about a Proposal.
type Notifier interface {
	Notify(ctx context.Context, subject string, n Notice) error
}

// Proposal reviews the DM makes.
const (
	Approve        = "approve"
	RequestChanges = "request_changes"
	Decline        = "decline"
)

func proposalPath(p domain.Proposal) string {
	return "/campaigns/" + p.Campaign.String() + "/proposals/" + p.ID.String()
}

// tell rings bells after a change is saved; a bell that fails is logged, never undoes the change.
func (s *Service) tell(ctx context.Context, subjects []string, n Notice) {
	if s.Notices == nil {
		return
	}
	for _, sub := range subjects {
		if err := s.Notices.Notify(ctx, sub, n); err != nil && s.Log != nil {
			s.Log.ErrorContext(ctx, "library: proposal notice", "error", err)
		}
	}
}

// tellDMs rings every DM of the Campaign but the one who acted.
func (s *Service) tellDMs(ctx context.Context, p domain.Proposal, me string, n Notice) error {
	dms, err := s.Members.DMs(ctx, p.Campaign)
	if err != nil {
		return err
	}
	var to []string
	for _, d := range dms {
		if d.Subject != me {
			to = append(to, d.Subject)
		}
	}
	s.tell(ctx, to, n)
	return nil
}

// Propose sends the DM a new entry for the Campaign, or a change to one it sees.
func (s *Service) Propose(ctx context.Context, c caller.Caller, campaign uuid.UUID, d domain.Draft, note string, base *uuid.UUID) (domain.Proposal, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return domain.Proposal{}, err
	}
	if d, err = d.Clean(); err != nil {
		return domain.Proposal{}, err
	}
	if note, err = domain.CleanMessage(note); err != nil {
		return domain.Proposal{}, err
	}
	if base != nil {
		if _, err := s.one(ctx, campaign, *base); err != nil {
			return domain.Proposal{}, err
		}
	}
	now := s.Now()
	p := domain.Proposal{
		ID: uuid.New(), Campaign: campaign, Author: c.Subject, AuthorName: me.Name, Draft: d, Note: note, Base: base,
		Status: domain.StatusPending, CreatedAt: now, UpdatedAt: now,
	}
	err = s.Repo.InTx(ctx, func(r Repository) error {
		if err := r.InsertProposal(ctx, p); err != nil {
			return err
		}
		return r.InsertReview(ctx, p.ID, domain.Review{Action: domain.ReviewSubmitted, Message: note, By: me.Name, At: now})
	})
	if err != nil {
		return p, err
	}
	return p, s.tellDMs(ctx, p, c.Subject, Notice{Title: me.Name + " proposes " + d.Name, Body: note, Label: "Review", Path: proposalPath(p), Dedupe: "proposal:" + p.ID.String()})
}

// Proposals lists a Campaign's Proposals, newest first: every one for a DM, a Player's own otherwise.
func (s *Service) Proposals(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Proposal, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return nil, err
	}
	author := c.Subject
	if me.DM {
		author = ""
	}
	return s.Repo.Proposals(ctx, campaign, author)
}

// readable reads a Proposal its author or a DM of its Campaign may open.
func (s *Service) readable(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (domain.Proposal, playdomain.Member, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return domain.Proposal{}, me, err
	}
	p, err := s.Repo.Proposal(ctx, id)
	if err == nil && (p.Campaign != campaign || (!me.DM && p.Author != c.Subject)) {
		err = apperr.ErrNotFound
	}
	return p, me, err
}

// Proposal reads one Proposal with its history and, for a change, the entry as the Campaign sees it now.
func (s *Service) Proposal(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (domain.ProposalDetail, error) {
	p, _, err := s.readable(ctx, c, campaign, id)
	if err != nil {
		return domain.ProposalDetail{}, err
	}
	d := domain.ProposalDetail{Proposal: p}
	if d.Reviews, err = s.Repo.Reviews(ctx, id); err != nil {
		return d, err
	}
	if p.Base != nil {
		cur, err := s.one(ctx, campaign, *p.Base)
		if err != nil && !errors.Is(err, apperr.ErrNotFound) {
			return d, err
		}
		if err == nil {
			d.Current = &cur
		}
	}
	return d, nil
}

// Resubmit sends a Proposal the DM asked changes to again, as its author changed it.
func (s *Service) Resubmit(ctx context.Context, c caller.Caller, campaign, id uuid.UUID, d domain.Draft, note string) (domain.ProposalDetail, error) {
	p, me, err := s.readable(ctx, c, campaign, id)
	switch {
	case err != nil:
		return domain.ProposalDetail{}, err
	case p.Author != c.Subject:
		return domain.ProposalDetail{}, apperr.ErrForbidden
	case p.Status != domain.StatusChangesRequested:
		return domain.ProposalDetail{}, apperr.Refuse("only a Proposal the DM asked changes to can be sent again")
	}
	d.Kind = p.Draft.Kind
	if d, err = d.Clean(); err != nil {
		return domain.ProposalDetail{}, err
	}
	if note, err = domain.CleanMessage(note); err != nil {
		return domain.ProposalDetail{}, err
	}
	now := s.Now()
	p.Draft, p.Note, p.Status, p.Message, p.UpdatedAt = d, note, domain.StatusPending, "", now
	if err := s.save(ctx, &p, domain.Review{Action: domain.ReviewResubmitted, Message: note, By: me.Name, At: now}, nil); err != nil {
		return domain.ProposalDetail{}, err
	}
	if err := s.tellDMs(ctx, p, c.Subject, Notice{Title: me.Name + " changed " + d.Name, Body: note, Label: "Review", Path: proposalPath(p), Dedupe: "proposal:" + p.ID.String()}); err != nil {
		return domain.ProposalDetail{}, err
	}
	return s.Proposal(ctx, c, campaign, id)
}

// save writes a Proposal's new state with the step that led to it, and runs more in the same transaction.
func (s *Service) save(ctx context.Context, p *domain.Proposal, step domain.Review, more func(Repository) error) error {
	return s.Repo.InTx(ctx, func(r Repository) error {
		if more != nil {
			if err := more(r); err != nil {
				return err
			}
		}
		if err := r.UpdateProposal(ctx, *p); err != nil {
			return err
		}
		return r.InsertReview(ctx, p.ID, step)
	})
}

// Review decides a pending Proposal. Approval copies it once into the DM's Library (or saves it as the
// next Revision of an entry of theirs it changes) and links it into the Campaign Collection; the DM may
// edit its name or fields first. Asking for changes needs a message. The author hears either way.
func (s *Service) Review(ctx context.Context, c caller.Caller, campaign, id uuid.UUID, action, message string, edit *domain.Draft) (domain.ProposalDetail, error) {
	p, me, err := s.readable(ctx, c, campaign, id)
	switch {
	case err != nil:
		return domain.ProposalDetail{}, err
	case !me.DM:
		return domain.ProposalDetail{}, apperr.ErrForbidden
	case p.Status != domain.StatusPending:
		return domain.ProposalDetail{}, apperr.Refuse("this Proposal is not waiting for a review")
	}
	if message, err = domain.CleanMessage(message); err != nil {
		return domain.ProposalDetail{}, err
	}
	now := s.Now()
	p.Message, p.UpdatedAt = message, now
	var more func(Repository) error
	verb := ""
	switch action {
	case Approve:
		if edit != nil {
			edit.Kind = p.Draft.Kind
			edit.Name = cmp.Or(edit.Name, p.Draft.Name)
			if edit.Fields == nil {
				edit.Fields = p.Draft.Fields
			}
			if p.Draft, err = edit.Clean(); err != nil {
				return domain.ProposalDetail{}, err
			}
		}
		p.Status, verb = domain.StatusApproved, " approved "
		more = func(r Repository) error { return s.adopt(ctx, r, c, &p) }
	case RequestChanges:
		if message == "" {
			return domain.ProposalDetail{}, apperr.Refuse("say what to change")
		}
		p.Status, verb = domain.StatusChangesRequested, " asks for changes to "
	case Decline:
		p.Status, verb = domain.StatusDeclined, " declined "
	default:
		return domain.ProposalDetail{}, apperr.Refuse("approve, ask for changes or decline")
	}
	if err := s.save(ctx, &p, domain.Review{Action: p.Status, Message: message, By: me.Name, At: now}, more); err != nil {
		return domain.ProposalDetail{}, err
	}
	s.tell(ctx, []string{p.Author}, Notice{Title: me.Name + verb + p.Draft.Name, Body: message, Label: "Open", Path: proposalPath(p), Dedupe: "proposal:" + p.ID.String()})
	return s.Proposal(ctx, c, campaign, id)
}

// adopt puts an approved Proposal into the DM's Library: the next Revision of the entry it changes when
// that entry is theirs, a new entry otherwise; and links it through the Campaign Collection.
func (s *Service) adopt(ctx context.Context, r Repository, c caller.Caller, p *domain.Proposal) error {
	now := s.Now()
	if p.Base != nil {
		if base, err := r.Entry(ctx, *p.Base); err == nil && base.Owner == c.Subject {
			no, err := r.UpdateEntry(ctx, base.ID, p.Draft.Name, p.Draft.Fields, now)
			if err != nil {
				return err
			}
			p.Entry = &base.ID
			return r.InsertRevision(ctx, base.ID, domain.Revision{No: no, Name: p.Draft.Name, Fields: p.Draft.Fields, Author: c.Subject, At: now})
		}
	}
	e := domain.Entry{ID: uuid.New(), Owner: c.Subject, Kind: p.Draft.Kind, Name: p.Draft.Name, Fields: p.Draft.Fields, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := r.InsertEntry(ctx, e); err != nil {
		return err
	}
	if err := r.InsertRevision(ctx, e.ID, domain.Revision{No: 1, Name: e.Name, Fields: e.Fields, Author: c.Subject, At: now}); err != nil {
		return err
	}
	p.Entry = &e.ID
	home, err := s.home(ctx, r, c, p.Campaign)
	if err != nil {
		return err
	}
	return r.AddToCollection(ctx, home, e.ID)
}

// home is the Campaign Collection, made and switched on the first time an approval needs it.
func (s *Service) home(ctx context.Context, r Repository, c caller.Caller, campaign uuid.UUID) (uuid.UUID, error) {
	id, err := r.CampaignHome(ctx, campaign)
	if !errors.Is(err, apperr.ErrNotFound) {
		return id, err
	}
	now := s.Now()
	col := domain.Collection{ID: uuid.New(), Owner: c.Subject, Name: "Campaign Collection", Description: "Entries made or approved for this Campaign.", CreatedAt: now, UpdatedAt: now}
	if err := r.InsertCollection(ctx, col); err != nil {
		return uuid.Nil, err
	}
	if err := r.InsertCampaignHome(ctx, campaign, col.ID); err != nil {
		return uuid.Nil, err
	}
	return col.ID, r.Switch(ctx, campaign, col.ID, true, now)
}
