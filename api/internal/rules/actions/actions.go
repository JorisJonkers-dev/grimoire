// Package actions describes the 2024 actions a creature takes on its turn, the Unarmed Strike options
// that resolve as saving throws, the cost of dragging a grappled creature, and when a readied action
// fires.
package actions

// Action is one of the actions every creature can take.
type Action string

// The 2024 actions besides Attack.
const (
	Dash      Action = "dash"
	Disengage Action = "disengage"
	Dodge     Action = "dodge"
	Help      Action = "help"
	Hide      Action = "hide"
	Influence Action = "influence"
	Magic     Action = "magic"
	Ready     Action = "ready"
	Search    Action = "search"
	Study     Action = "study"
	Utilize   Action = "utilize"
)

// Info is what an action is called, what it does in a line, and the ability check it makes, if any.
type Info struct {
	Action  Action
	Name    string
	Summary string
	// Check is the ability (and skill) the action's check uses; empty when it makes none.
	Check string
	Skill string
}

// Standard lists every action in the order the action bar shows them.
func Standard() []Info {
	return []Info{
		{Action: Dash, Name: "Dash", Summary: "Gain extra movement equal to your Speed this turn.", Check: "", Skill: ""},
		{Action: Disengage, Name: "Disengage", Summary: "Your movement provokes no Opportunity Attacks this turn.", Check: "", Skill: ""},
		{Action: Dodge, Name: "Dodge", Summary: "Attacks against you have Disadvantage and you have Advantage on Dexterity saves until your next turn.", Check: "", Skill: ""},
		{Action: Help, Name: "Help", Summary: "Give an ally Advantage on their next check or attack against a creature next to you.", Check: "", Skill: ""},
		{Action: Hide, Name: "Hide", Summary: "Make a DC 15 Dexterity (Stealth) check; on a success you are Invisible.", Check: "dexterity", Skill: "stealth"},
		{Action: Influence, Name: "Influence", Summary: "Try to sway a creature with a Charisma or Wisdom check.", Check: "charisma", Skill: ""},
		{Action: Magic, Name: "Magic", Summary: "Cast a spell, use a magic item or a magical feature.", Check: "", Skill: ""},
		{Action: Ready, Name: "Ready", Summary: "Choose a trigger and an attack; when the trigger happens, use your Reaction to make it.", Check: "", Skill: ""},
		{Action: Search, Name: "Search", Summary: "Make a Wisdom (Perception) check to find what is hidden.", Check: "wisdom", Skill: "perception"},
		{Action: Study, Name: "Study", Summary: "Make an Intelligence check to recall or work something out.", Check: "intelligence", Skill: ""},
		{Action: Utilize, Name: "Utilize", Summary: "Use a non-magical object: open a door, pull a lever, drink a potion.", Check: "", Skill: ""},
	}
}

// Find looks an action up by its key.
func Find(a Action) (Info, bool) {
	for _, i := range Standard() {
		if i.Action == a {
			return i, true
		}
	}
	return Info{Action: "", Name: "", Summary: "", Check: "", Skill: ""}, false
}

// HideDC is the Stealth check a Hide action must meet.
const HideDC = 15

// UnarmedDC is the saving throw a Grapple or Shove forces: 8 plus Strength modifier plus proficiency.
func UnarmedDC(strengthMod, proficiency int) int {
	return 8 + strengthMod + proficiency
}

// Option is what an Unarmed Strike does besides damage.
type Option string

// Unarmed Strike options resolved as saving throws.
const (
	Grapple   Option = "grapple"
	ShovePush Option = "shove_push"
	ShoveDown Option = "shove_prone"
)

// Resist is the saving throw a creature makes against a Grapple or Shove: Strength or Dexterity,
// whichever it is better at.
func Resist(saves map[string]int) (string, int) {
	if saves["dexterity"] > saves["strength"] {
		return "dexterity", saves["dexterity"]
	}
	return "strength", saves["strength"]
}

// DragCostFt is what moving costs a grappler dragging its grappled creature: every foot costs one more.
func DragCostFt(costFt int) int {
	return costFt * 2
}

// TriggerKind is what a readied action waits for.
type TriggerKind string

// Trigger kinds.
const (
	// EntersReach fires when a creature moves to within reach of the readier.
	EntersReach TriggerKind = "enters_reach"
)

// Trigger is a readied action's condition: a kind, and the one creature it watches, if any.
type Trigger struct {
	Kind TriggerKind
	Who  string
}

// Step is a creature moving one hex, as a readied action sees it.
type Step struct {
	Mover string
	// BeforeFt and AfterFt are the mover's distances from the readier before and after the step.
	BeforeFt int
	AfterFt  int
}

// Fires reports whether a step sets off the trigger of a readied action whose attack reaches ReachFt.
func (t Trigger) Fires(s Step, reachFt int) bool {
	if t.Who != "" && t.Who != s.Mover {
		return false
	}
	return t.Kind == EntersReach && s.BeforeFt > reachFt && s.AfterFt <= reachFt
}
