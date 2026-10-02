// Package domain holds Friends (and, later, Conversations): who is connected to whom, keyed on
// Accounts.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors the social service returns.
var (
	ErrNoAccount = errors.New("social: no Account")
	ErrNotFound  = errors.New("social: not found")
	ErrConflict  = errors.New("social: already so")
	ErrInvalid   = errors.New("social: invalid")
)

// AccountID identifies an Account.
type AccountID = uuid.UUID

// Person is another Account as social pages show it.
type Person struct {
	ID       AccountID
	Username string
	Nickname string
}

// Friend is a Person in a mutual, accepted friendship.
type Friend struct {
	Person
	Since time.Time
}

// Request is a Friend request waiting for an answer, with the other side of it.
type Request struct {
	ID     uuid.UUID
	Person Person
	At     time.Time
}

// Friends is everything the Friends page shows.
type Friends struct {
	Friends  []Friend
	Incoming []Request
	Outgoing []Request
	Blocked  []Request
}
