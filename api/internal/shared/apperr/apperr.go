// Package apperr holds the errors every use case reports; each transport maps them once.
package apperr

import "errors"

// Use-case errors.
var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
	ErrInvalid   = errors.New("invalid")
	ErrLocked    = errors.New("locked")
)

// RuleError is a request the rules refuse, with a reason fit to show the player.
type RuleError struct {
	Reason string
}

func (e *RuleError) Error() string { return e.Reason }

// Unwrap lets callers match a RuleError as ErrInvalid.
func (e *RuleError) Unwrap() error { return ErrInvalid }

// Refuse is a RuleError with the given reason.
func Refuse(reason string) error { return &RuleError{Reason: reason} }
