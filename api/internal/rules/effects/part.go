package effects

import (
	"regexp"
	"slices"
)

// Part is a Component of a kind defined in a file of its own, kind_<slug>.go, which holds its type,
// its definition, its rules text and what it does in play. No switch over kinds names it: the
// switches that still list the older kinds reach a Part through this interface and its capabilities.
type Part interface {
	Component
	// Kind is the slug of the Part's kind, the one its definition in Kinds describes.
	Kind() string
	// Text is the Part's rules sentence.
	Text() string
}

// Kind is what one kind of Part declares once: what an author fills in, and how those values become
// the Part.
type Kind interface {
	Describe() Description
	// Decode checks authored values, and the Parts beneath when the kind holds any, and makes the Part.
	Decode(v Values, beneath Groups) (Part, error)
}

// Description is a kind's definition as data: what a builder form and a Design file are drawn from.
type Description struct {
	Slug    string
	Label   string
	Summary string
	Fields  []Field
	// Groups names the groups of Parts the kind holds beneath it; most kinds hold none.
	Groups []string
}

// FieldType is the sort of value a field holds.
type FieldType string

// Field types.
const (
	DiceField    FieldType = "dice"
	NumberField  FieldType = "number"
	FlagField    FieldType = "flag"
	TextField    FieldType = "text"
	ChoiceField  FieldType = "choice"
	ChoicesField FieldType = "choices"
)

// Field is one value an author fills in for a Part. Min and Max bound a number.
type Field struct {
	Name     string
	Label    string
	Type     FieldType
	Options  []string
	Required bool
	Min      int
	Max      int
}

// Value is one authored value: Text for dice, text and a choice, Number, Flag, or Texts for several
// choices.
type Value struct {
	Text   string
	Number int
	Flag   bool
	Texts  []string
}

// Values are a Part's authored values by field name, before they are checked.
type Values map[string]Value

// Groups are the Parts beneath a Part by group name, already decoded.
type Groups map[string][]Component

// Group is one named group of Parts beneath a Part.
type Group struct {
	Name  string
	Parts []Component
}

// ValueError is an authored value a kind refuses, with the field it is in.
type ValueError struct {
	Field  string
	Reason string
}

func (e ValueError) Error() string {
	return e.Field + ": " + e.Reason
}

func refuse(field, reason string) error {
	return ValueError{Field: field, Reason: reason}
}

// AttackFolder is a passive Part that changes the attacks its bearer makes.
type AttackFolder interface {
	FoldAttacking(p *AttackProfile, effect string)
}

// SaveFolder is a passive Part that changes its bearer's saving throws.
type SaveFolder interface {
	FoldSave(p *SaveProfile, effect, ability string)
}

// Lander is a Part that acts when its Effect lands on an area: it asks for its rolls once, and then
// says what happens to each creature. Both are Acts the runtime applies.
type Lander interface {
	Part
	Asks() []Act
	Lands(o Outcome) []Act
}

// Outcome is how an area's rolls came out for one creature: whether it saved, and the damage rolled.
type Outcome struct {
	Saved  bool
	Damage int
}

// Holder is a Part with Parts beneath it, in the groups its kind declares.
type Holder interface {
	Part
	Groups() []Group
	// Landed is the Parts beneath that come into play for one creature as the Effect lands on it.
	Landed(s Situation) []Component
}

// Act is one thing the runtime does when an Effect lands. The set is small and closed: kinds are
// many, and each says what it does in these terms rather than reaching into play.
//
//sumtype:decl
type Act interface{ isAct() }

// RollSave asks every creature in the area for a saving throw of Ability.
type RollSave struct {
	Ability string
}

// RollDamage rolls Dice of Type damage once for the whole area.
type RollDamage struct {
	Dice string
	Type string
}

// Hurt deals Amount damage of Type to one creature.
type Hurt struct {
	Amount int
	Type   string
}

func (RollSave) isAct()   {}
func (RollDamage) isAct() {}
func (Hurt) isAct()       {}

var dicePattern = regexp.MustCompile(`^[1-9][0-9]?d(4|6|8|10|12|20)$`)

func abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

func damageTypes() []string {
	return []string{"acid", "bludgeoning", "cold", "fire", "force", "lightning", "necrotic", "piercing", "poison", "psychic", "radiant", "slashing", "thunder"}
}

// oneOf refuses a value that is not one of the options.
func oneOf(field, value string, options []string) error {
	if !slices.Contains(options, value) {
		return refuse(field, "choose one of the options")
	}
	return nil
}
