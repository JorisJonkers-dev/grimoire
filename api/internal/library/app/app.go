// Package app runs the Library: an account's entries, their Revisions, and their links into the
// Campaigns its owner runs, with a Campaign Override and an optional pinned Revision.
package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Repository persists Library entries, their Revisions and Campaign links.
type Repository interface {
	InsertEntry(ctx context.Context, e domain.Entry) error
	// UpdateEntry saves a new base and returns its Revision number.
	UpdateEntry(ctx context.Context, id uuid.UUID, name string, fields domain.Fields, now time.Time) (int, error)
	InsertRevision(ctx context.Context, entry uuid.UUID, r domain.Revision) error
	Entry(ctx context.Context, id uuid.UUID) (domain.Entry, error)
	Entries(ctx context.Context, owner, kind string) ([]domain.Entry, error)
	Revisions(ctx context.Context, entry uuid.UUID) ([]domain.Revision, error)
	RevisionExists(ctx context.Context, entry uuid.UUID, no int) (bool, error)
	Uses(ctx context.Context, entry uuid.UUID) ([]domain.Use, error)
	Link(ctx context.Context, campaign, entry uuid.UUID, now time.Time) error
	// Unlink reports false when the DM had not linked the entry themselves.
	Unlink(ctx context.Context, campaign, entry uuid.UUID) (bool, error)
	SetOverride(ctx context.Context, campaign, entry uuid.UUID, override domain.Fields, now time.Time) error
	Pin(ctx context.Context, campaign, entry uuid.UUID, revision *int, now time.Time) error
	// Linked reads the entries a Campaign sees, linked or brought in by a Collection, or one when entry is set.
	Linked(ctx context.Context, campaign uuid.UUID, entry *uuid.UUID) ([]domain.Linked, error)
	InsertCollection(ctx context.Context, c domain.Collection) error
	UpdateCollection(ctx context.Context, c domain.Collection) error
	Collection(ctx context.Context, id uuid.UUID) (domain.Collection, error)
	// Collections lists an owner's Collections and, with a Campaign, every other one switched on there.
	Collections(ctx context.Context, owner string, campaign *uuid.UUID) ([]domain.Collection, error)
	Switch(ctx context.Context, campaign, collection uuid.UUID, on bool, now time.Time) error
	AddToCollection(ctx context.Context, collection, entry uuid.UUID) error
	// CampaignHome is the Campaign Collection's id, or ErrNotFound before the Campaign has one.
	CampaignHome(ctx context.Context, campaign uuid.UUID) (uuid.UUID, error)
	InsertCampaignHome(ctx context.Context, campaign, collection uuid.UUID) error
	InsertProposal(ctx context.Context, p domain.Proposal) error
	UpdateProposal(ctx context.Context, p domain.Proposal) error
	Proposal(ctx context.Context, id uuid.UUID) (domain.Proposal, error)
	// Proposals lists a Campaign's Proposals, newest first; only one author's when author is set.
	Proposals(ctx context.Context, campaign uuid.UUID, author string) ([]domain.Proposal, error)
	InsertReview(ctx context.Context, proposal uuid.UUID, r domain.Review) error
	Reviews(ctx context.Context, proposal uuid.UUID) ([]domain.Review, error)
	InsertSubmission(ctx context.Context, x domain.Submission) error
	DecideSubmission(ctx context.Context, x domain.Submission) error
	Submission(ctx context.Context, id uuid.UUID) (domain.Submission, error)
	// Submissions lists requests to share, pending first; only one submitter's when submitter is set.
	Submissions(ctx context.Context, submitter string) ([]domain.Submission, error)
	PendingSubmission(ctx context.Context, entry uuid.UUID) (bool, error)
	InTx(ctx context.Context, fn func(Repository) error) error
}

// Members finds who a caller is in a Campaign, and its DMs.
type Members interface {
	Membership(ctx context.Context, campaign uuid.UUID, subject string) (playdomain.Member, error)
	DMs(ctx context.Context, campaign uuid.UUID) ([]playdomain.Member, error)
}

// Service is the Library use cases.
type Service struct {
	Repo    Repository
	Members Members
	Now     func() time.Time
	// Notices rings bells about Proposals, and Log records a bell that failed; nil Notices rings none.
	Notices Notifier
	Log     *slog.Logger
	// Admins says who reviews the Shared Library.
	Admins Admins
}

// Admins tells Admins apart.
type Admins interface {
	IsAdmin(ctx context.Context, subject string) bool
}

// Entries lists the caller's entries, of one kind when kind is set.
func (s *Service) Entries(ctx context.Context, c caller.Caller, kind string) ([]domain.Entry, error) {
	return s.Repo.Entries(ctx, c.Subject, kind)
}

// Create adds an entry to the caller's Library as its first Revision.
func (s *Service) Create(ctx context.Context, c caller.Caller, d domain.Draft) (domain.Entry, error) {
	d, err := d.Clean()
	if err != nil {
		return domain.Entry{}, err
	}
	now := s.Now()
	e := domain.Entry{ID: uuid.New(), Owner: c.Subject, Kind: d.Kind, Name: d.Name, Fields: d.Fields, Revision: 1, CreatedAt: now, UpdatedAt: now}
	err = s.Repo.InTx(ctx, func(r Repository) error {
		if err := r.InsertEntry(ctx, e); err != nil {
			return err
		}
		return r.InsertRevision(ctx, e.ID, domain.Revision{No: 1, Name: e.Name, Fields: e.Fields, Author: c.Subject, At: now})
	})
	return e, err
}

// owned reads an entry the caller owns; anyone else's looks like no entry at all.
func (s *Service) owned(ctx context.Context, c caller.Caller, id uuid.UUID) (domain.Entry, error) {
	e, err := s.Repo.Entry(ctx, id)
	if err == nil && e.Owner != c.Subject {
		return domain.Entry{}, apperr.ErrNotFound
	}
	return e, err
}

// linkable reads an entry the caller may link: one of their own, or a Shared Library copy.
func (s *Service) linkable(ctx context.Context, c caller.Caller, id uuid.UUID) (domain.Entry, error) {
	e, err := s.Repo.Entry(ctx, id)
	if err == nil && e.Owner != c.Subject && !e.Shared {
		return domain.Entry{}, apperr.ErrNotFound
	}
	return e, err
}

// Get reads one of the caller's entries with its Revisions and the Campaigns it is linked into, or a
// Shared Library copy with its Revisions; where others use a shared copy is theirs to know.
func (s *Service) Get(ctx context.Context, c caller.Caller, id uuid.UUID) (domain.Detail, error) {
	e, err := s.linkable(ctx, c, id)
	if err != nil {
		return domain.Detail{}, err
	}
	d := domain.Detail{Entry: e, Uses: []domain.Use{}}
	if d.Revisions, err = s.Repo.Revisions(ctx, id); err != nil || e.Shared {
		return d, err
	}
	d.Uses, err = s.Repo.Uses(ctx, id)
	return d, err
}

// Update saves a new base for one of the caller's entries as its next Revision; its kind stays.
// Every Campaign that follows the latest Revision sees the change; a pinned one does not.
func (s *Service) Update(ctx context.Context, c caller.Caller, id uuid.UUID, d domain.Draft) (domain.Detail, error) {
	e, err := s.owned(ctx, c, id)
	if err != nil {
		return domain.Detail{}, err
	}
	d.Kind = e.Kind
	if d, err = d.Clean(); err != nil {
		return domain.Detail{}, err
	}
	now := s.Now()
	err = s.Repo.InTx(ctx, func(r Repository) error {
		no, err := r.UpdateEntry(ctx, id, d.Name, d.Fields, now)
		if err != nil {
			return err
		}
		return r.InsertRevision(ctx, id, domain.Revision{No: no, Name: d.Name, Fields: d.Fields, Author: c.Subject, At: now})
	})
	if err != nil {
		return domain.Detail{}, err
	}
	return s.Get(ctx, c, id)
}

// dm checks the caller runs the Campaign.
func (s *Service) dm(ctx context.Context, c caller.Caller, campaign uuid.UUID) error {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err == nil && !me.DM {
		return apperr.ErrForbidden
	}
	return err
}

// Linked lists the entries linked into a Campaign as it sees them. DM only.
func (s *Service) Linked(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Linked, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	return s.Repo.Linked(ctx, campaign, nil)
}

// one reads a single link as the Campaign sees it.
func (s *Service) one(ctx context.Context, campaign, entry uuid.UUID) (domain.Linked, error) {
	l, err := s.Repo.Linked(ctx, campaign, &entry)
	if err != nil {
		return domain.Linked{}, err
	}
	if len(l) == 0 {
		return domain.Linked{}, apperr.ErrNotFound
	}
	return l[0], nil
}

// Link links one of the caller's entries, or a Shared Library copy, into a Campaign they run; linking it
// twice changes nothing.
func (s *Service) Link(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID) (domain.Linked, error) {
	if err := s.dm(ctx, c, campaign); err != nil {
		return domain.Linked{}, err
	}
	if _, err := s.linkable(ctx, c, entry); err != nil {
		return domain.Linked{}, err
	}
	if err := s.Repo.Link(ctx, campaign, entry, s.Now()); err != nil {
		return domain.Linked{}, err
	}
	return s.one(ctx, campaign, entry)
}

// Unlink takes an entry the DM linked out of a Campaign, with its Campaign Override; a switched-on
// Collection may still bring it in.
func (s *Service) Unlink(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID) error {
	if err := s.dm(ctx, c, campaign); err != nil {
		return err
	}
	return found(s.Repo.Unlink(ctx, campaign, entry))
}

// visible checks the caller runs the Campaign and it sees the entry, linked or through a Collection.
func (s *Service) visible(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID) error {
	if err := s.dm(ctx, c, campaign); err != nil {
		return err
	}
	_, err := s.one(ctx, campaign, entry)
	return err
}

// Override replaces the Campaign Override of an entry the Campaign sees: the fields it sees differently.
func (s *Service) Override(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID, fields domain.Fields) (domain.Linked, error) {
	clean, err := domain.CleanFields(fields)
	if err == nil {
		err = s.visible(ctx, c, campaign, entry)
	}
	if err == nil {
		err = s.Repo.SetOverride(ctx, campaign, entry, clean, s.Now())
	}
	if err != nil {
		return domain.Linked{}, err
	}
	return s.one(ctx, campaign, entry)
}

// Pin holds a Campaign to one Revision of a linked entry, so later edits to the base pass it by; nil
// follows the latest again.
func (s *Service) Pin(ctx context.Context, c caller.Caller, campaign, entry uuid.UUID, revision *int) (domain.Linked, error) {
	if err := s.visible(ctx, c, campaign, entry); err != nil {
		return domain.Linked{}, err
	}
	if revision != nil {
		ok, err := s.Repo.RevisionExists(ctx, entry, *revision)
		if err != nil {
			return domain.Linked{}, err
		}
		if !ok {
			return domain.Linked{}, apperr.Refuse("pin a Revision the entry has")
		}
	}
	if err := s.Repo.Pin(ctx, campaign, entry, revision, s.Now()); err != nil {
		return domain.Linked{}, err
	}
	return s.one(ctx, campaign, entry)
}

// found turns a change that touched no link into ErrNotFound.
func found(ok bool, err error) error {
	if err == nil && !ok {
		return apperr.ErrNotFound
	}
	return err
}
