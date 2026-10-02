// Package pgstore is the Postgres adapter for Friends.
package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// Store implements app.Repository.
type Store struct {
	pool *pgxpool.Pool
	q    *queries.Queries
	wrap func(queries.DBTX) queries.DBTX
}

var _ app.Repository = (*Store)(nil)

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return newWrapped(pool, func(db queries.DBTX) queries.DBTX { return db })
}

func newWrapped(pool *pgxpool.Pool, wrap func(queries.DBTX) queries.DBTX) *Store {
	return &Store{pool: pool, q: queries.New(wrap(pool)), wrap: wrap}
}

// InTx runs fn against a Store bound to one transaction.
func (s *Store) InTx(ctx context.Context, fn func(app.Repository) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, q: queries.New(s.wrap(tx)), wrap: s.wrap})
	})
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}

// AccountBySubject finds the live Account a subject signs in as.
func (s *Store) AccountBySubject(ctx context.Context, subject string) (domain.Person, error) {
	r, err := s.q.SocialAccountBySubject(ctx, subject)
	return domain.Person{ID: r.ID, Username: r.Username, Nickname: r.Nickname}, notFound(err)
}

// AccountByUsername finds a live Account by its Username.
func (s *Store) AccountByUsername(ctx context.Context, username string) (domain.Person, error) {
	r, err := s.q.SocialAccountByUsername(ctx, username)
	return domain.Person{ID: r.ID, Username: r.Username, Nickname: r.Nickname}, notFound(err)
}

// InsertRequest stores a pending Friend request; one already pending between the two stays as it is.
func (s *Store) InsertRequest(ctx context.Context, id uuid.UUID, from, to domain.AccountID, now time.Time) error {
	_, err := s.q.InsertFriendRequest(ctx, queries.InsertFriendRequestParams{ID: id, FromAccount: from, ToAccount: to, Now: now})
	return err
}

// PendingRequest reads who sent a pending request, and to whom.
func (s *Store) PendingRequest(ctx context.Context, id uuid.UUID) (domain.AccountID, domain.AccountID, error) {
	r, err := s.q.PendingRequest(ctx, id)
	return r.FromAccount, r.ToAccount, notFound(err)
}

// PendingBetween finds a pending request from one Account to another.
func (s *Store) PendingBetween(ctx context.Context, from, to domain.AccountID) (uuid.UUID, error) {
	id, err := s.q.PendingBetween(ctx, queries.PendingBetweenParams{FromAccount: from, ToAccount: to})
	return id, notFound(err)
}

// DeleteRequest removes a request once it is answered or withdrawn.
func (s *Store) DeleteRequest(ctx context.Context, id uuid.UUID) error {
	return s.q.DeleteFriendRequest(ctx, id)
}

// DeclineRequest marks a request declined.
func (s *Store) DeclineRequest(ctx context.Context, id uuid.UUID, now time.Time) error {
	return s.q.DeclineFriendRequest(ctx, queries.DeclineFriendRequestParams{Now: pgtype.Timestamptz{Time: now, Valid: true}, ID: id})
}

// Befriend makes two Accounts Friends.
func (s *Store) Befriend(ctx context.Context, x, y domain.AccountID, now time.Time) error {
	return s.q.InsertFriendship(ctx, queries.InsertFriendshipParams{X: x, Y: y, Now: now})
}

// Unfriend ends a friendship; false when there was none.
func (s *Store) Unfriend(ctx context.Context, x, y domain.AccountID) (bool, error) {
	n, err := s.q.DeleteFriendship(ctx, queries.DeleteFriendshipParams{X: x, Y: y})
	return n == 1, err
}

// AreFriends reports whether two Accounts are Friends.
func (s *Store) AreFriends(ctx context.Context, x, y domain.AccountID) (bool, error) {
	return s.q.AreFriends(ctx, queries.AreFriendsParams{X: x, Y: y})
}

// Blocks reports whether one Account blocked another.
func (s *Store) Blocks(ctx context.Context, blocker, blocked domain.AccountID) (bool, error) {
	return s.q.IsBlocked(ctx, queries.IsBlockedParams{Blocker: blocker, Blocked: blocked})
}

// Block hides an Account's Friend requests from another.
func (s *Store) Block(ctx context.Context, blocker, blocked domain.AccountID, now time.Time) error {
	return s.q.InsertBlock(ctx, queries.InsertBlockParams{Blocker: blocker, Blocked: blocked, Now: now})
}

// Unblock lifts a block; false when there was none.
func (s *Store) Unblock(ctx context.Context, blocker, blocked domain.AccountID) (bool, error) {
	n, err := s.q.DeleteBlock(ctx, queries.DeleteBlockParams{Blocker: blocker, Blocked: blocked})
	return n == 1, err
}

// Friends reads an Account's Friends, requests both ways and blocks.
func (s *Store) Friends(ctx context.Context, me domain.AccountID) (domain.Friends, error) {
	out := domain.Friends{Friends: []domain.Friend{}, Incoming: []domain.Request{}, Outgoing: []domain.Request{}, Blocked: []domain.Request{}}
	friends, err := s.q.ListFriends(ctx, me)
	if err != nil {
		return out, err
	}
	for _, f := range friends {
		out.Friends = append(out.Friends, domain.Friend{Person: domain.Person{ID: f.ID, Username: f.Username, Nickname: f.Nickname}, Since: f.Since})
	}
	in, err := s.q.ListIncomingRequests(ctx, me)
	if err != nil {
		return out, err
	}
	for _, r := range in {
		out.Incoming = append(out.Incoming, domain.Request{ID: r.ID, Person: domain.Person{ID: r.AccountID, Username: r.Username, Nickname: r.Nickname}, At: r.CreatedAt})
	}
	sent, err := s.q.ListOutgoingRequests(ctx, me)
	if err != nil {
		return out, err
	}
	for _, r := range sent {
		out.Outgoing = append(out.Outgoing, domain.Request{ID: r.ID, Person: domain.Person{ID: r.AccountID, Username: r.Username, Nickname: r.Nickname}, At: r.CreatedAt})
	}
	blocked, err := s.q.ListBlocked(ctx, me)
	if err != nil {
		return out, err
	}
	for _, b := range blocked {
		out.Blocked = append(out.Blocked, domain.Request{ID: b.ID, Person: domain.Person{ID: b.ID, Username: b.Username, Nickname: b.Nickname}, At: b.CreatedAt})
	}
	return out, nil
}
