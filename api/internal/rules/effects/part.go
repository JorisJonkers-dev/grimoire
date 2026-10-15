package effects

import (
	"regexp"
	"slices"
	"strconv"
	"unicode/utf8"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

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
	// OpenGroups lets the author name each group, as a choice names its modes.
	OpenGroups bool
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

// Field is one value an author fills in for a Part. Min and Max bound a number; Max also bounds the
// length of a text.
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
	Text   string   `json:"text,omitempty"`
	Number int      `json:"number,omitempty"`
	Flag   bool     `json:"flag,omitempty"`
	Texts  []string `json:"texts,omitempty"`
}

// Values are a Part's authored values by field name, before they are checked.
type Values map[string]Value

// Group is one named group of Parts beneath a Part.
type Group struct {
	Name  string
	Parts []Part
}

// Groups are the Parts beneath a Part in their named groups, in order, already decoded.
type Groups []Group

// Of is the Parts of one group; none when it is not given.
func (gs Groups) Of(name string) []Part {
	for _, g := range gs {
		if g.Name == name {
			return g.Parts
		}
	}
	return nil
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

// Held is an Effect as its bearer holds it: the name its notes and reasons carry, and how many levels
// of it count.
type Held struct {
	Name   string
	Levels int
}

// AttackFolder is a passive Part that changes the attacks its bearer makes.
type AttackFolder interface {
	FoldAttacking(p *AttackProfile, h Held)
}

// Attacker is who attacks the bearer: whether it is the Effect's own source, and whether it attacks
// from within 5 feet.
type Attacker struct {
	Source     bool
	WithinFive bool
}

// AttackedFolder is a passive Part that changes the attacks made against its bearer.
type AttackedFolder interface {
	FoldAttacked(p *AttackProfile, h Held, by Attacker)
}

// SaveFolder is a passive Part that changes its bearer's saving throws.
type SaveFolder interface {
	FoldSave(p *SaveProfile, h Held, ability string)
}

// MoveFolder is a passive Part that changes how its bearer moves.
type MoveFolder interface {
	FoldMove(p *MoveProfile, h Held)
}

// ActionFolder is a passive Part that changes what its bearer can do: whether it acts at all, and the
// reactions it has.
type ActionFolder interface {
	FoldActions(p *ActionProfile, h Held)
}

// Stacker is a Part that makes its Effect stack: applied again, it adds a level instead of a copy.
type Stacker interface {
	// Cap is the highest level the Effect rises to; 0 has no cap.
	Cap() int
	// Kills reports whether that many levels kill the bearer.
	Kills(levels int) bool
}

// BearerLander is a Part that acts on its bearer the moment its Effect lands on it.
type BearerLander interface {
	LandsOnBearer() []Act
}

// Asker is a Part that shapes an area Effect: it asks once for what the area's plan needs, such as
// its template, its rolls and what it lays down.
type Asker interface {
	Part
	Asks() []Act
}

// Lander is an area Part that then says what happens to each creature once the rolls are in. Both are
// Acts the runtime applies.
type Lander interface {
	Asker
	Lands(o Outcome) []Act
}

// Outcome is how an area's rolls came out for one creature: whether it saved, and the damage rolled.
type Outcome struct {
	Saved  bool
	Damage int
}

// Consequent is a Part that can be written as what happens to a creature a branch has picked out: "has
// the Charmed condition".
type Consequent interface {
	Consequence() string
}

// Grouper is a Part with Parts beneath it, in named groups.
type Grouper interface {
	Part
	Groups() []Group
	// Regroup is the Part holding other Parts in the same groups.
	Regroup(gs []Group) Part
}

// Holder is a Part whose Parts beneath come into play as the Effect lands.
type Holder interface {
	Grouper
	// Landed is the Parts beneath that come into play for one creature as the Effect lands on it.
	Landed(s Situation) []Part
}

// Chooser is a Part whose groups are modes; whoever applies the Effect picks one.
type Chooser interface {
	Grouper
	// Chosen is the Parts of the mode picked; none when no mode of that name is offered.
	Chosen(mode string) []Part
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

// GainTempHP gives the bearer Amount temporary hit points, which replace smaller ones rather than add.
type GainTempHP struct {
	Amount int
}

// EndSpells ends every spell on the bearer.
type EndSpells struct{}

// Grant gives the bearer a feature, which the DM notes down.
type Grant struct {
	Feature string
}

// ChangeUses gives back (Delta above 0) or takes uses of the bearer's Resource.
type ChangeUses struct {
	Resource string
	Delta    int
}

// TeleportCaster moves the caster up to RangeFt to an open hex.
type TeleportCaster struct {
	RangeFt int
}

// Template lays the area out on the board.
type Template struct {
	Area Area
}

// Afflict puts a condition on a creature.
type Afflict struct {
	Condition string
}

// LaySurface covers the area in a Surface for some rounds.
type LaySurface struct {
	Surface surface.Kind
	Rounds  int
}

// Push moves a creature that failed Ft feet straight away from the origin, or toward it.
type Push struct {
	Ft     int
	Toward bool
}

// ShedLight sheds bright light out to BrightFt and dim light DimFt beyond it.
type ShedLight struct {
	BrightFt int
	DimFt    int
}

// StripQualities strips Visibility Qualities from everything they touch.
type StripQualities struct {
	Qualities []string
}

// AskDM gives the DM an instruction to resolve by hand.
type AskDM struct {
	Instruction string
}

// TakeForm overlays the bearer's stat block with a creature's.
type TakeForm struct {
	Monster string
	TempHP  int
}

// SummonCreatures brings creatures in under the caster's control.
type SummonCreatures struct {
	Monster      string
	Count        int
	Shares       bool
	NeedsCommand bool
}

func (RollSave) isAct()        {}
func (RollDamage) isAct()      {}
func (Hurt) isAct()            {}
func (GainTempHP) isAct()      {}
func (EndSpells) isAct()       {}
func (Grant) isAct()           {}
func (ChangeUses) isAct()      {}
func (TeleportCaster) isAct()  {}
func (Template) isAct()        {}
func (Afflict) isAct()         {}
func (LaySurface) isAct()      {}
func (Push) isAct()            {}
func (ShedLight) isAct()       {}
func (StripQualities) isAct()  {}
func (AskDM) isAct()           {}
func (TakeForm) isAct()        {}
func (SummonCreatures) isAct() {}

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

// field is one of a kind's fields by name; every name a kind decodes is one it describes.
func (d Description) field(name string) Field {
	return d.Fields[slices.IndexFunc(d.Fields, func(f Field) bool { return f.Name == name })]
}

// number reads a number field, refusing one outside the field's bounds.
func (v Values) number(f Field) (int, error) {
	n := v[f.Name].Number
	if n < f.Min || n > f.Max {
		return 0, refuse(f.Name, "runs from "+strconv.Itoa(f.Min)+" to "+strconv.Itoa(f.Max))
	}
	return n, nil
}

// text reads a text field of up to the field's Max letters, refusing an empty one.
func (v Values) text(f Field) (string, error) {
	s := v[f.Name].Text
	if s == "" || utf8.RuneCountInString(s) > f.Max {
		return "", refuse(f.Name, "write up to "+strconv.Itoa(f.Max)+" letters")
	}
	return s, nil
}

// optionalText reads a text field that may be empty, of up to the field's Max letters.
func (v Values) optionalText(f Field) (string, error) {
	s := v[f.Name].Text
	if utf8.RuneCountInString(s) > f.Max {
		return "", refuse(f.Name, "write up to "+strconv.Itoa(f.Max)+" letters")
	}
	return s, nil
}

// choices reads a choices field: at least one option, each once.
func (v Values) choices(f Field) ([]string, error) {
	got := v[f.Name].Texts
	for _, s := range got {
		if err := oneOf(f.Name, s, f.Options); err != nil {
			return nil, err
		}
	}
	if len(got) == 0 || len(slices.Compact(slices.Sorted(slices.Values(got)))) != len(got) {
		return nil, refuse(f.Name, "choose each option once")
	}
	return got, nil
}

// choice reads a choice field, refusing anything but one of its options.
func (v Values) choice(f Field) (string, error) {
	s := v[f.Name].Text
	return s, oneOf(f.Name, s, f.Options)
}
