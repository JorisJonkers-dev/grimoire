// Package domain holds Grimoire's own Accounts (ADR-0008): who someone is, how they set up an Account
// from an Admin's invite, and the sessions they sign in with.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors the identity service returns.
var (
	ErrInvalid         = errors.New("identity: invalid")
	ErrNotFound        = errors.New("identity: not found")
	ErrConflict        = errors.New("identity: already taken")
	ErrUnauthenticated = errors.New("identity: not signed in")
	ErrForbidden       = errors.New("identity: not allowed")
	ErrExpired         = errors.New("identity: expired or used")
)

// AccountID identifies an Account.
type AccountID = uuid.UUID

// Account is a person's Grimoire Account. Subject is the identity every other part of Grimoire keys
// on.
type Account struct {
	ID        AccountID
	Subject   string
	Username  string
	Nickname  string
	Email     string
	Admin     bool
	Disabled  bool
	CreatedAt time.Time
}

// SubjectFor is the subject of an internal Account.
func SubjectFor(id AccountID) string {
	return "account:" + id.String()
}

// Invite is an Account Invite: an Admin's one-time link to set up an Account before it expires.
type Invite struct {
	ID        uuid.UUID
	CreatedBy string
	Admin     bool
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// Open reports whether an Invite can still set up an Account.
func (i Invite) Open(now time.Time) bool {
	return i.UsedAt == nil && now.Before(i.ExpiresAt)
}

// Setup is what an invitee chooses for their Account.
type Setup struct {
	Username string
	Nickname string
	Email    string
	Password string
}

// Session is a signed-in device.
type Session struct {
	ID        uuid.UUID
	Account   AccountID
	UserAgent string
	ExpiresAt time.Time
}
