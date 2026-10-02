// Package app runs Friends: requests by Username, accepting, declining and blocking.
package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// Repository persists friendships, requests and blocks.
type Repository interface {
	AccountBySubject(ctx context.Context, subject string) (domain.Person, error)
	AccountByUsername(ctx context.Context, username string) (domain.Person, error)
	InsertRequest(ctx context.Context, id uuid.UUID, from, to domain.AccountID, now time.Time) error
	PendingRequest(ctx context.Context, id uuid.UUID) (domain.AccountID, domain.AccountID, error)
	PendingBetween(ctx context.Context, from, to domain.AccountID) (uuid.UUID, error)
	DeleteRequest(ctx context.Context, id uuid.UUID) error
	DeclineRequest(ctx context.Context, id uuid.UUID, now time.Time) error
	Befriend(ctx context.Context, x, y domain.AccountID, now time.Time) error
	Unfriend(ctx context.Context, x, y domain.AccountID) (bool, error)
	AreFriends(ctx context.Context, x, y domain.AccountID) (bool, error)
	Blocks(ctx context.Context, blocker, blocked domain.AccountID) (bool, error)
	Block(ctx context.Context, blocker, blocked domain.AccountID, now time.Time) error
	Unblock(ctx context.Context, blocker, blocked domain.AccountID) (bool, error)
	Friends(ctx context.Context, me domain.AccountID) (domain.Friends, error)
	InTx(ctx context.Context, fn func(Repository) error) error
}

// Service is the Friends use cases.
type Service struct {
	Repo Repository
	Now  func() time.Time
}

func (s *Service) me(ctx context.Context, subject string) (domain.Person, error) {
	p, err := s.Repo.AccountBySubject(ctx, subject)
	if errors.Is(err, domain.ErrNotFound) {
		return p, domain.ErrNoAccount
	}
	return p, err
}

// Request asks the Account with a Username to be Friends. A request the other side already sent is
// accepted at once; one to an Account that blocked the sender is kept from them, and looks the same.
func (s *Service) Request(ctx context.Context, subject, username string) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	them, err := s.Repo.AccountByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		return err
	}
	if them.ID == me.ID {
		return domain.ErrInvalid
	}
	return s.Repo.InTx(ctx, func(r Repository) error {
		if friends, err := r.AreFriends(ctx, me.ID, them.ID); err != nil || friends {
			return firstErr(err, domain.ErrConflict)
		}
		now := s.Now()
		accepted, err := s.acceptCrossing(ctx, r, me.ID, them.ID, now)
		if err != nil || accepted {
			return err
		}
		return r.InsertRequest(ctx, uuid.New(), me.ID, them.ID, now)
	})
}

// acceptCrossing makes two Accounts Friends when the other already asked, unless the caller blocked
// them.
func (s *Service) acceptCrossing(ctx context.Context, r Repository, me, them domain.AccountID, now time.Time) (bool, error) {
	crossing, err := r.PendingBetween(ctx, them, me)
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if blocked, err := r.Blocks(ctx, me, them); err != nil || blocked {
		return false, err
	}
	if err := r.DeleteRequest(ctx, crossing); err != nil {
		return false, err
	}
	return true, r.Befriend(ctx, me, them, now)
}

func firstErr(err, otherwise error) error {
	if err != nil {
		return err
	}
	return otherwise
}

// incoming loads a pending request sent to the caller.
func (s *Service) incoming(ctx context.Context, r Repository, me domain.Person, id uuid.UUID) (domain.AccountID, error) {
	from, to, err := r.PendingRequest(ctx, id)
	if err != nil {
		return from, err
	}
	if to != me.ID {
		return from, domain.ErrNotFound
	}
	return from, nil
}

// Accept accepts a Friend request sent to the caller; the two are Friends from now on.
func (s *Service) Accept(ctx context.Context, subject string, id uuid.UUID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	return s.Repo.InTx(ctx, func(r Repository) error {
		from, err := s.incoming(ctx, r, me, id)
		if err != nil {
			return err
		}
		if err := r.DeleteRequest(ctx, id); err != nil {
			return err
		}
		return r.Befriend(ctx, me.ID, from, s.Now())
	})
}

// Decline turns down a Friend request sent to the caller, and with block also hides every later one
// from its sender.
func (s *Service) Decline(ctx context.Context, subject string, id uuid.UUID, block bool) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	return s.Repo.InTx(ctx, func(r Repository) error {
		from, err := s.incoming(ctx, r, me, id)
		if err != nil {
			return err
		}
		now := s.Now()
		if err := r.DeclineRequest(ctx, id, now); err != nil || !block {
			return err
		}
		return r.Block(ctx, me.ID, from, now)
	})
}

// Cancel withdraws a Friend request the caller sent.
func (s *Service) Cancel(ctx context.Context, subject string, id uuid.UUID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	from, _, err := s.Repo.PendingRequest(ctx, id)
	if err != nil {
		return err
	}
	if from != me.ID {
		return domain.ErrNotFound
	}
	return s.Repo.DeleteRequest(ctx, id)
}

// Unfriend ends a friendship.
func (s *Service) Unfriend(ctx context.Context, subject string, friend domain.AccountID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	gone, err := s.Repo.Unfriend(ctx, me.ID, friend)
	if err != nil {
		return err
	}
	if !gone {
		return domain.ErrNotFound
	}
	return nil
}

// Unblock lets an Account's Friend requests reach the caller again.
func (s *Service) Unblock(ctx context.Context, subject string, blocked domain.AccountID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	gone, err := s.Repo.Unblock(ctx, me.ID, blocked)
	if err != nil {
		return err
	}
	if !gone {
		return domain.ErrNotFound
	}
	return nil
}

// Friends reads the caller's Friends, requests both ways, and the Accounts they blocked.
func (s *Service) Friends(ctx context.Context, subject string) (domain.Friends, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return domain.Friends{}, err
	}
	return s.Repo.Friends(ctx, me.ID)
}
