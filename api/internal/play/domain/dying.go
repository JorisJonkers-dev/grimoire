package domain

import "github.com/JorisJonkers-dev/grimoire/api/internal/rules/dying"

// Dying is a Character at 0 hit points: its death saves, the Unconscious Effect it lies under, the death
// save roll still out, and when it died.
type Dying struct {
	Token  TokenID
	State  dying.State
	Effect *EffectID
	RollID *RollID
	Died   dying.Died
}

// Life and death action kinds in the Action Log.
const (
	ActionDowned       = "downed"
	ActionDyingChanged = "dying_changed"
	ActionRevived      = "revived"
)
