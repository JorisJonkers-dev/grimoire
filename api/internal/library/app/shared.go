package app

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Shared lists the Shared Library, of one kind when kind is set.
func (s *Service) Shared(ctx context.Context, kind string) ([]domain.Entry, error) {
	return s.Repo.Entries(ctx, domain.SharedOwner, kind)
}

// Share asks the Admins to put the latest Revision of one of the caller's entries in the Shared Library.
func (s *Service) Share(ctx context.Context, c caller.Caller, entry uuid.UUID, note string) (domain.Submission, error) {
	e, err := s.owned(ctx, c, entry)
	if err != nil {
		return domain.Submission{}, err
	}
	if note, err = domain.CleanMessage(note); err != nil {
		return domain.Submission{}, err
	}
	waiting, err := s.Repo.PendingSubmission(ctx, entry)
	if err != nil {
		return domain.Submission{}, err
	}
	if waiting {
		return domain.Submission{}, apperr.Refuse("this entry already waits for an Admin")
	}
	x := domain.Submission{
		ID: uuid.New(), Entry: e.ID, Revision: e.Revision, Draft: domain.Draft{Kind: e.Kind, Name: e.Name, Fields: e.Fields}, Design: e.Design, Note: note,
		Submitter: c.Subject, Status: domain.SubmissionPending, CreatedAt: s.Now(),
	}
	return x, s.Repo.InsertSubmission(ctx, x)
}

// Submissions lists the caller's requests to share, pending first.
func (s *Service) Submissions(ctx context.Context, c caller.Caller) ([]domain.Submission, error) {
	return s.Repo.Submissions(ctx, c.Subject)
}

func (s *Service) admin(ctx context.Context, c caller.Caller) error {
	if s.Admins == nil || !s.Admins.IsAdmin(ctx, c.Subject) {
		return apperr.ErrForbidden
	}
	return nil
}

// AllSubmissions lists every request to share, pending first. Admins only.
func (s *Service) AllSubmissions(ctx context.Context, c caller.Caller) ([]domain.Submission, error) {
	if err := s.admin(ctx, c); err != nil {
		return nil, err
	}
	return s.Repo.Submissions(ctx, "")
}

// ReviewSubmission decides a request to share, with the IP check recorded either way. Approval needs
// the Admin's check that the entry carries no non-SRD text, and puts a read-only copy of the submitted
// Revision in the Shared Library. Admins only.
func (s *Service) ReviewSubmission(ctx context.Context, c caller.Caller, id uuid.UUID, approve, ipClear bool, ipNote, message string) (domain.Submission, error) {
	if err := s.admin(ctx, c); err != nil {
		return domain.Submission{}, err
	}
	x, err := s.Repo.Submission(ctx, id)
	if err != nil {
		return x, err
	}
	switch {
	case x.Status != domain.SubmissionPending:
		return x, apperr.Refuse("this request was already reviewed")
	case approve && !ipClear:
		return x, apperr.Refuse("confirm the IP check: the entry carries no non-SRD text")
	}
	if x.IPNote, err = domain.CleanMessage(ipNote); err != nil {
		return x, err
	}
	if x.Message, err = domain.CleanMessage(message); err != nil {
		return x, err
	}
	now := s.Now()
	x.IPClear, x.Reviewer, x.DecidedAt, x.Status = &ipClear, c.Subject, &now, domain.SubmissionDeclined
	return x, s.Repo.InTx(ctx, func(r Repository) error {
		if approve {
			copied := domain.Entry{
				ID: uuid.New(), Owner: domain.SharedOwner, Kind: x.Draft.Kind, Name: x.Draft.Name, Fields: x.Draft.Fields, Design: x.Design, Revision: 1,
				Shared: true, CreatedAt: now, UpdatedAt: now,
			}
			if err := r.InsertEntry(ctx, copied); err != nil {
				return err
			}
			if err := r.InsertRevision(ctx, copied.ID, domain.Revision{No: 1, Name: copied.Name, Fields: copied.Fields, Design: copied.Design, Author: c.Subject, At: now}); err != nil {
				return err
			}
			x.Status, x.Shared = domain.SubmissionApproved, &copied.ID
		}
		return r.DecideSubmission(ctx, x)
	})
}
