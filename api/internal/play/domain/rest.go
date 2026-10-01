package domain

import (
	"slices"

	"github.com/google/uuid"
)

// Rest statuses: proposed and waiting on agreement, then under way.
const (
	RestProposed = "proposed"
	RestResting  = "resting"
)

// Rest action kinds in the Action Log.
const (
	ActionRestProposed    = "rest_proposed"
	ActionRestAgreed      = "rest_agreed"
	ActionRestStarted     = "rest_started"
	ActionHitDieSpent     = "hit_die_spent"
	ActionHitDieHealed    = "hit_die_healed"
	ActionRestInterrupted = "rest_interrupted"
)

// Rester is a Character taking part in a rest, as the rest reads them.
type Rester struct {
	CharacterID uuid.UUID
	TokenID     TokenID
	Name        string
	Class       string
	Level       int
	HitDie      int
	HitDiceLeft int
	// Abilities are scores by ability; Used is how many uses of each Resource are spent.
	Abilities map[string]int
	Used      map[string]int
	// RollID is the Hit Die roll still out, if any.
	RollID *RollID
}

// Rest is a rest the party proposed: who must still agree, and who is resting.
type Rest struct {
	Kind       string
	Status     string
	ProposedBy uuid.UUID
	Agreed     []uuid.UUID
	// DMAgreed is set once a DM has agreed; a rest never starts without one.
	DMAgreed bool
	Resters  []Rester
}

// Clone copies the Rest so a change never touches the committed state.
func (r *Rest) Clone() *Rest {
	if r == nil {
		return nil
	}
	c := *r
	c.Agreed = slices.Clone(r.Agreed)
	c.Resters = slices.Clone(r.Resters)
	return &c
}

// Supply is what a Container holds of an item after a Long Rest ate from it.
type Supply struct {
	Container ContainerID
	Item      string
	Left      int
}

// RestResult is what a finished rest leaves a Character with.
type RestResult struct {
	CharacterID  uuid.UUID
	HPCurrent    int
	HitDiceSpent int
	Used         map[string]int
	LevelUpReady bool
}
