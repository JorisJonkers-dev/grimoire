package domain

import (
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// CombatID identifies a Combat.
type CombatID uuid.UUID

// CombatantID identifies a Combatant.
type CombatantID uuid.UUID

// Combat statuses: everyone rolls initiative, then turns run; an ended Combat is history.
const (
	CombatRolling = "rolling"
	CombatActive  = "active"
	CombatEnded   = "ended"
)

// Combat is a running fight inside a live Session.
type Combat struct {
	ID     CombatID
	Status string
	Round  int
	// Turn is the initiative count now acting; every Combatant on it acts at the same time.
	Turn       int
	Combatants []Combatant
	// Attack is the attack waiting on a roll, if any.
	Attack *PendingAttack
	// Prompt is the Reaction Prompt the fight waits on, if any.
	Prompt *ReactionPrompt
	// Resume is the rest of a walk an opportunity attack interrupted.
	Resume    *Resume
	StartedAt time.Time
	EndedAt   time.Time
}

// Combatant is a Token taking part in a Combat.
type Combatant struct {
	ID              CombatantID
	TokenID         TokenID
	RollID          RollID
	InitiativeBonus int
	SpeedFt         int
	// Initiative is nil until the initiative Roll Request resolves.
	Initiative *int
	// Done is set once the Combatant ends its turn this round.
	Done    bool
	Economy combat.Economy
	// Shielded adds 5 to AC until the Combatant's next turn starts.
	Shielded bool
	// Surprised Combatants did not notice the ambush and rolled initiative at disadvantage.
	Surprised bool
	// Disengaged movement provokes no opportunity attacks until the Combatant's next turn.
	Disengaged bool
	// Readied is an attack waiting on its trigger until the Combatant's next turn.
	Readied *Readied
	// CleaveFrom is the creature a Cleave hit struck this turn, opening a second attack against one next
	// to it; Cleaved is set once that attack is made.
	CleaveFrom *TokenID
	Cleaved    bool
}

// Readied is a readied attack: what sets it off, and which attack it makes.
type Readied struct {
	Trigger  actions.Trigger
	AttackNo int
}

// PendingAction is a Hide, Grapple or Shove waiting on its roll.
type PendingAction struct {
	RollID RollID
	Actor  TokenID
	Target *TokenID
	Action string
	DC     int
}

// Action kinds for the 2024 actions in the Action Log.
const (
	ActionTaken    = "action_taken"
	ActionUnarmed  = "unarmed_strike"
	ActionResolved = "action_resolved"
	// ActionObjectUsed is the turn's free object interaction.
	ActionObjectUsed = "object_used"
	// ActionMasteryUsed is what a Weapon Mastery did after an attack.
	ActionMasteryUsed = "mastery_used"
	// ActionReactionSet is a Controller changing a token's reaction settings.
	ActionReactionSet = "reaction_set"
)

// Reaction kinds and the stage an attack waits in while its target decides.
const (
	PromptOpportunity = "opportunity_attack"
	PromptShield      = "shield"
	PromptReadied     = "readied"
	// PromptEffect is a reaction an Effect gives, resolved by the DM.
	PromptEffect  = "effect"
	StageReaction = "reaction"
)

// ReactionPrompt asks a Controller, or the DM, whether a creature uses its reaction before the
// triggering action goes on. No answer by the Deadline declines.
type ReactionPrompt struct {
	ID       uuid.UUID
	Kind     string
	Reactor  TokenID
	Trigger  TokenID
	AttackNo int
	Effect   string
	Deadline time.Time
}

// Resume is the rest of a walk, start first, and the movement it costs.
type Resume struct {
	Token  TokenID
	Path   []hex.Coord
	CostFt int
}

// Reaction action kinds in the Action Log.
const (
	ActionReactionOffered  = "reaction_offered"
	ActionReactionUsed     = "reaction_used"
	ActionReactionDeclined = "reaction_declined"
)

// Attack stages: the attack roll, then damage on a hit.
const (
	StageToHit  = "to_hit"
	StageDamage = "damage"
)

// PendingAttack is an attack waiting on its attack or damage Roll Request.
type PendingAttack struct {
	ID         uuid.UUID
	Attacker   TokenID
	Target     TokenID
	AttackNo   int
	Mode       attack.Mode
	CoverBonus int
	Stage      string
	Critical   bool
	RollID     RollID
	// Ranged is set for attacks made from beyond reach; creatures that see them remember the damage.
	Ranged bool
	// Total is the attack roll while the target decides on a reaction.
	Total int
	// Opportunity marks an opportunity attack; the interrupted walk resumes after it.
	Opportunity bool
	// OffHand marks the off-hand attack of a Light weapon; Cleave the second attack Cleave allows.
	OffHand bool
	Cleave  bool
}

// Totals lists every rolled initiative.
func (c *Combat) Totals() []int {
	var out []int
	for _, x := range c.Combatants {
		if x.Initiative != nil {
			out = append(out, *x.Initiative)
		}
	}
	return out
}

// Acting reports whether a Combatant's turn is now.
func (c *Combat) Acting(x Combatant) bool {
	return c.Status == CombatActive && x.Initiative != nil && *x.Initiative == c.Turn && !x.Done
}

// Combat action kinds in the Action Log.
const (
	ActionCombatStarted    = "combat_started"
	ActionInitiativeRolled = "initiative_rolled"
	ActionTurnEnded        = "turn_ended"
	ActionResourceSpent    = "resource_spent"
	ActionCombatEnded      = "combat_ended"
)
