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
	// ErrManaged is a change to what a linked OIDC login provides, or an unlink that would leave no
	// password.
	ErrManaged = errors.New("identity: set by the linked login")
	// ErrDisabled is an OIDC sign-in when none is set up.
	ErrDisabled = errors.New("identity: sign-in method not set up")
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

// Claims are what an OIDC provider vouches for about a login.
type Claims struct {
	Issuer   string
	Subject  string
	Email    string
	Username string
	Name     string
	Roles    []string
	Nonce    string
}

// Link is an OIDC login linked to an Account, with the claims the provider last sent.
type Link struct {
	Issuer   string
	Subject  string
	Email    string
	Username string
	Name     string
	LinkedAt time.Time
}

// Profile is an Account with how it signs in.
type Profile struct {
	Account     Account
	HasPassword bool
	Link        *Link
}

// Pending is an OIDC login no Account has yet; its holder creates an Account or links one.
type Pending struct {
	Token    string
	Email    string
	Username string
	Name     string
}

// ProfileChange is what an Account holder changes about themselves.
type ProfileChange struct {
	Username string
	Nickname string
	Email    string
}
