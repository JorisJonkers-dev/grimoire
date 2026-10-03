package domain

import "github.com/google/uuid"

// Injury is a lingering injury a Character carries: the condition it is, how far it has gone, and
// what cures it.
type Injury struct {
	Character uuid.UUID
	Slug      string
	Name      string
	Cure      string
	Level     int
}
