package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SessionRepository is the persistence port for Sessions.
type SessionRepository interface {
	CreateSession(ctx context.Context, campaign uuid.UUID, actor domain.Member, c caller.Caller, now time.Time) (domain.Session, error)
	Session(ctx context.Context, campaign uuid.UUID, id domain.SessionID) (domain.Session, error)
	Sessions(ctx context.Context, campaign uuid.UUID) ([]domain.Session, error)
	// EndSession ends a live Session and, with the Session a party split from, the Sessions of its groups;
	// it returns those.
	EndSession(ctx context.Context, campaign uuid.UUID, id domain.SessionID, actor domain.Member, c caller.Caller, now time.Time) ([]domain.SessionID, error)
	SessionLog(ctx context.Context, id domain.SessionID, limit int) ([]domain.LoggedAction, error)
}

// Closer shuts a live Session's runtime down once the Session ends.
type Closer interface {
	Close(id domain.SessionID)
}

// Sessions runs the Session use cases.
type Sessions struct {
	Repo    SessionRepository
	Members Members
	Live    Closer
	Now     func() time.Time
}

func (s *Sessions) dm(ctx context.Context, c caller.Caller, campaign uuid.UUID) (domain.Member, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return domain.Member{}, err
	}
	if !me.DM {
		return domain.Member{}, apperr.ErrForbidden
	}
	return me, nil
}

// Start opens a new live Session. DM only.
func (s *Sessions) Start(ctx context.Context, c caller.Caller, campaign uuid.UUID) (domain.Session, error) {
	me, err := s.dm(ctx, c, campaign)
	if err != nil {
		return domain.Session{}, err
	}
	return s.Repo.CreateSession(ctx, campaign, me, c, s.Now())
}

// Get returns a Session to any Member.
func (s *Sessions) Get(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SessionID) (domain.Session, error) {
	if _, err := s.Members.Membership(ctx, campaign, c.Subject); err != nil {
		return domain.Session{}, err
	}
	return s.Repo.Session(ctx, campaign, id)
}

// Log lists a Session's latest Actions, newest first. DM only.
func (s *Sessions) Log(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SessionID, limit int) ([]domain.LoggedAction, error) {
	if _, err := s.dm(ctx, c, campaign); err != nil {
		return nil, err
	}
	if _, err := s.Repo.Session(ctx, campaign, id); err != nil {
		return nil, err
	}
	return s.Repo.SessionLog(ctx, id, limit)
}

// List returns the Campaign's Sessions, newest first.
func (s *Sessions) List(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]domain.Session, error) {
	if _, err := s.Members.Membership(ctx, campaign, c.Subject); err != nil {
		return nil, err
	}
	return s.Repo.Sessions(ctx, campaign)
}

// End closes a live Session and disconnects everyone from it. DM only.
func (s *Sessions) End(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.SessionID) (domain.Session, error) {
	me, err := s.dm(ctx, c, campaign)
	if err != nil {
		return domain.Session{}, err
	}
	groups, err := s.Repo.EndSession(ctx, campaign, id, me, c, s.Now())
	if err != nil {
		return domain.Session{}, err
	}
	s.Live.Close(id)
	for _, g := range groups {
		s.Live.Close(g)
	}
	return s.Repo.Session(ctx, campaign, id)
}
