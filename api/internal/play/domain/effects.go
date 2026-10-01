package domain

import "github.com/google/uuid"

// EffectID identifies an Effect on a token.
type EffectID uuid.UUID

// Effect is a spell, feature or condition on a token. RoundsLeft of 0 lasts until it is ended; with a
// save ability the bearer rolls against SaveDC at the end of each of its turns to end it.
type Effect struct {
	ID            EffectID
	Target        TokenID
	Source        *TokenID
	Slug          string
	Name          string
	Concentration bool
	RoundsLeft    int
	SaveAbility   string
	SaveDC        int
	// Level is how many levels of a stacking Effect (exhaustion) the bearer has.
	Level int
}

// Holder is whose turns count an Effect down: its source's, or the bearer's when it has none.
func (e Effect) Holder() TokenID {
	if e.Source != nil {
		return *e.Source
	}
	return e.Target
}

// ManualPrompt is part of an Effect the engine cannot compute; the DM resolves it by hand.
type ManualPrompt struct {
	ID   uuid.UUID
	Text string
}

// PendingSave is a saving throw that ends an Effect when its roll meets the DC.
type PendingSave struct {
	RollID RollID
	Effect EffectID
	DC     int
}

// Effects is everything effect-related in a live Session.
type Effects struct {
	Active []Effect
	Manual []ManualPrompt
	Saves  []PendingSave
}

// Effect action kinds in the Action Log.
const (
	ActionEffectApplied  = "effect_applied"
	ActionEffectEnded    = "effect_ended"
	ActionSavePassed     = "save_passed"
	ActionSaveFailed     = "save_failed"
	ActionManualResolved = "manual_resolved"
)
