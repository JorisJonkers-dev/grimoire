// Package spellbuild turns a homebrew spell's design, its Targeting and typed parts listed as rows, into
// an Effect the rules engine runs, with its rules text.
package spellbuild

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (p DesignError) Error() string { return string(p) }

// Design is a homebrew spell as its author builds it.
type Design struct {
	Targeting     Targeting   `json:"targeting"`
	Save          string      `json:"save,omitempty"`
	Concentration bool        `json:"concentration"`
	Duration      Duration    `json:"duration"`
	Ritual        bool        `json:"ritual"`
	CastingTime   CastingTime `json:"castingTime"`
	Components    Components  `json:"components"`
	Parts         []Part      `json:"parts"`
}

// Targeting is the spell's area: its shape and size, and how far away its point may be (0 from the caster).
type Targeting struct {
	Shape   string `json:"shape"`
	SizeFt  int    `json:"sizeFt"`
	RangeFt int    `json:"rangeFt"`
}

// Duration is how long the spell lasts: instant, or an amount of rounds, minutes or hours, or until dispelled.
type Duration struct {
	Unit   string `json:"unit"`
	Amount int    `json:"amount,omitempty"`
}

// CastingTime is an action, a bonus action, a reaction with the trigger it answers, or minutes.
type CastingTime struct {
	Kind    string `json:"kind"`
	Minutes int    `json:"minutes,omitempty"`
	Trigger string `json:"trigger,omitempty"`
}

// Components are the verbal, somatic and material components; a Material Component may cost gold, be
// consumed, and name the item it is.
type Components struct {
	Verbal   bool      `json:"verbal"`
	Somatic  bool      `json:"somatic"`
	Material *Material `json:"material,omitempty"`
}

// Material is a Material Component.
type Material struct {
	Text     string `json:"text"`
	CostGP   int    `json:"costGp,omitempty"`
	Consumed bool   `json:"consumed,omitempty"`
	Item     string `json:"item,omitempty"`
}

// Part kinds.
const (
	DamagePart    = "damage"
	ConditionPart = "condition"
	LightPart     = "light"
	RevealPart    = "reveal"
	SurfacePart   = "surface"
	ManualPart    = "manual"
)

// When a damage part lands: as the spell is cast, or on each creature that starts its turn in the area.
const (
	OnCast      = "on_cast"
	StartOfTurn = "start_of_turn"
)

// Part is one row of a design; which fields count depends on its Type.
type Part struct {
	Type       string   `json:"type"`
	When       string   `json:"when,omitempty"`
	Dice       string   `json:"dice,omitempty"`
	DamageType string   `json:"damageType,omitempty"`
	Half       bool     `json:"half,omitempty"`
	Condition  string   `json:"condition,omitempty"`
	OnlyTypes  []string `json:"onlyTypes,omitempty"`
	BrightFt   int      `json:"brightFt,omitempty"`
	DimFt      int      `json:"dimFt,omitempty"`
	Qualities  []string `json:"qualities,omitempty"`
	Surface    string   `json:"surface,omitempty"`
	Rounds     int      `json:"rounds,omitempty"`
	Text       string   `json:"text,omitempty"`
}

// Surface is the ground a spell's start-of-turn damage lies on: a Surface of its own, harming whoever
// starts a turn in it.
type Surface struct {
	Slug string
	Name string
	Dice string
	Type string
}

// Built is a design ready for play: its Effect, the Surface its start-of-turn damage needs, and its
// rules text.
type Built struct {
	Definition effects.Definition
	Surface    *Surface
	Text       []string
}

// Abilities are the saves a spell can call for.
func Abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

// DamageTypes are the kinds of damage.
func DamageTypes() []string {
	return []string{"acid", "bludgeoning", "cold", "fire", "force", "lightning", "necrotic", "piercing", "poison", "psychic", "radiant", "slashing", "thunder"}
}

// Conditions are the conditions a part can impose.
func Conditions() []string {
	return []string{"blinded", "charmed", "deafened", "frightened", "grappled", "incapacitated", "invisible", "paralyzed", "petrified", "poisoned", "prone", "restrained", "stunned", "unconscious"}
}

// CreatureTypes are the creature types a part can be limited to.
func CreatureTypes() []string {
	return []string{"aberration", "beast", "celestial", "construct", "dragon", "elemental", "fey", "fiend", "giant", "humanoid", "monstrosity", "ooze", "plant", "undead"}
}

// Triggers are what a reaction spell answers.
func Triggers() []string {
	return []string{"when_hit", "when_damaged", "ally_attacked", "creature_casts", "creature_enters_reach"}
}

// Shapes are the areas the builder draws.
func Shapes() []string {
	return []string{"sphere", "cylinder", "emanation", "ring", "cone", "cube", "line", "wall"}
}

var dicePattern = regexp.MustCompile(`^[1-9][0-9]?d(4|6|8|10|12|20)$`)

// Build checks a design and turns it into an Effect named slug and name. surfaces are the Surface kinds
// a part may lay down.
func Build(slug, name string, d Design, surfaces []string) (Built, error) {
	if err := d.check(); err != nil {
		return Built{}, err
	}
	b := builder{slug: slug, name: name, d: d, surfaces: surfaces, comps: nil, text: nil, surface: nil, saveDamage: false, light: false, ground: false}
	for _, p := range d.Parts {
		if err := b.part(p); err != nil {
			return Built{}, err
		}
	}
	return b.finish(), nil
}

// check validates everything but the parts.
func (d Design) check() error {
	t := d.Targeting
	switch {
	case !slices.Contains(Shapes(), t.Shape):
		return DesignError("choose an area: " + strings.Join(Shapes(), ", "))
	case t.SizeFt < 5 || t.SizeFt > 300 || t.SizeFt%5 != 0:
		return DesignError("the area's size is 5 to 300 feet, in steps of 5")
	case t.RangeFt < 0 || t.RangeFt > 1000 || t.RangeFt%5 != 0:
		return DesignError("the range is 0 to 1000 feet, in steps of 5")
	case d.Save != "" && !slices.Contains(Abilities(), d.Save):
		return DesignError("the save is an ability: " + strings.Join(Abilities(), ", "))
	case len(d.Parts) > 30:
		return DesignError("keep it to 30 parts")
	}
	if err := d.Duration.check(d.Concentration); err != nil {
		return err
	}
	if err := d.CastingTime.check(d.Ritual); err != nil {
		return err
	}
	return d.Components.check()
}

func (u Duration) check(concentration bool) error {
	switch u.Unit {
	case "instant":
		if concentration {
			return DesignError("an instantaneous spell needs no concentration")
		}
		return nil
	case "rounds", "minutes", "hours":
		if u.Amount < 1 || u.Amount > 100 {
			return DesignError("a duration lasts 1 to 100 rounds, minutes or hours")
		}
		return nil
	case "until_dispelled":
		return nil
	}
	return DesignError("the duration is instant, rounds, minutes, hours, or until dispelled")
}

func (c CastingTime) check(ritual bool) error {
	switch {
	case !slices.Contains([]string{"action", "bonus_action", "reaction", "minutes"}, c.Kind):
		return DesignError("the casting time is an action, a bonus action, a reaction or minutes")
	case c.Kind == "reaction" && !slices.Contains(Triggers(), c.Trigger):
		return DesignError("a reaction spell answers a trigger: " + strings.Join(Triggers(), ", "))
	case c.Kind == "minutes" && (c.Minutes < 1 || c.Minutes > 1440):
		return DesignError("a spell takes 1 to 1440 minutes to cast")
	case ritual && (c.Kind == "reaction" || c.Kind == "bonus_action"):
		return DesignError("a ritual takes an action or minutes to cast")
	}
	return nil
}

func (c Components) check() error {
	m := c.Material
	switch {
	case !c.Verbal && !c.Somatic && m == nil:
		return DesignError("a spell has at least one component")
	case m == nil:
		return nil
	case strings.TrimSpace(m.Text) == "" || len([]rune(m.Text)) > 200:
		return DesignError("describe the Material Component in up to 200 characters")
	case m.CostGP < 0 || m.CostGP > 100000:
		return DesignError("a Material Component costs 0 to 100000 gp")
	case m.Consumed && m.CostGP == 0:
		return DesignError("only a Material Component with a cost is consumed")
	}
	return nil
}

// builder gathers the Effect's components and text as the parts come.
type builder struct {
	slug, name string
	d          Design
	surfaces   []string
	comps      []effects.Component
	text       []string
	surface    *Surface
	saveDamage bool
	light      bool
	ground     bool
}

func (b *builder) part(p Part) error {
	switch p.Type {
	case DamagePart:
		return b.damage(p)
	case ConditionPart:
		return b.condition(p)
	case LightPart:
		return b.lights(p)
	case RevealPart:
		return b.reveal(p)
	case SurfacePart:
		return b.lay(p)
	case ManualPart:
		if strings.TrimSpace(p.Text) == "" || len([]rune(p.Text)) > 500 {
			return DesignError("a manual part says in up to 500 characters what the DM does")
		}
		b.comps = append(b.comps, effects.Manual{Instruction: p.Text})
		b.text = append(b.text, p.Text)
		return nil
	}
	return DesignError("a part is damage, condition, light, reveal, surface or manual")
}

func (b *builder) damage(p Part) error {
	switch {
	case !dicePattern.MatchString(p.Dice):
		return DesignError("damage dice look like 2d6")
	case !slices.Contains(DamageTypes(), p.DamageType):
		return DesignError("choose a damage type: " + strings.Join(DamageTypes(), ", "))
	case p.When == StartOfTurn:
		return b.turnDamage(p)
	case p.When != OnCast:
		return DesignError("damage lands on cast or at the start of a turn")
	case b.d.Save == "":
		return DesignError("damage on cast needs the save the targets make")
	case b.saveDamage:
		return DesignError("a spell deals its damage on cast once")
	}
	b.saveDamage = true
	b.comps = append(b.comps, effects.SaveDamage{Ability: b.d.Save, Dice: p.Dice, Type: p.DamageType, Half: p.Half})
	text := "Each creature in the area takes " + p.Dice + " " + p.DamageType + " damage on a failed " + title(b.d.Save) + " saving throw"
	if p.Half {
		text += ", or half as much on a successful one"
	}
	b.text = append(b.text, text+".")
	return nil
}

func (b *builder) turnDamage(p Part) error {
	switch {
	case b.ground:
		return DesignError("a spell leaves one Surface or one start-of-turn damage, not both")
	case b.rounds() == 0:
		return DesignError("start-of-turn damage needs a duration in rounds, minutes or hours")
	}
	b.ground = true
	b.surface = &Surface{Slug: b.slug + "-ground", Name: b.name, Dice: p.Dice, Type: p.DamageType}
	b.comps = append(b.comps, effects.CreateSurface{Kind: surface.Kind(b.surface.Slug), Rounds: b.rounds()})
	b.text = append(b.text, "A creature that starts its turn in the area takes "+p.Dice+" "+p.DamageType+" damage.")
	return nil
}

// rounds is how many rounds the spell lasts, 0 when its end is not counted.
func (b *builder) rounds() int {
	return effects.Duration{Kind: effects.DurationKind(b.d.Duration.Unit), Amount: b.d.Duration.Amount, RepeatSave: ""}.Rounds()
}

func (b *builder) condition(p Part) error {
	switch {
	case !slices.Contains(Conditions(), p.Condition):
		return DesignError("choose a condition: " + strings.Join(Conditions(), ", "))
	case b.d.Save == "":
		return DesignError("a condition needs the save the targets make")
	}
	for _, t := range p.OnlyTypes {
		if !slices.Contains(CreatureTypes(), t) {
			return DesignError("limit it to creature types: " + strings.Join(CreatureTypes(), ", "))
		}
	}
	failed := effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1, Type: ""}, Then: []effects.Component{effects.SaveCondition{Ability: b.d.Save, Slug: p.Condition}}}
	text := "A creature that fails the " + title(b.d.Save) + " saving throw has the " + title(p.Condition) + " condition"
	if len(p.OnlyTypes) == 0 {
		b.comps = append(b.comps, failed)
		b.text = append(b.text, text+".")
		return nil
	}
	names := make([]string, 0, len(p.OnlyTypes))
	for _, t := range p.OnlyTypes {
		b.comps = append(b.comps, effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: t}, Then: []effects.Component{failed}})
		names = append(names, title(t))
	}
	b.text = append(b.text, text+", if it is "+strings.Join(names, " or ")+".")
	return nil
}

func (b *builder) lights(p Part) error {
	switch {
	case b.light:
		return DesignError("a spell sheds one light")
	case p.BrightFt < 5 || p.BrightFt > 120 || p.BrightFt%5 != 0 || p.DimFt < 0 || p.DimFt > 120 || p.DimFt%5 != 0:
		return DesignError("light reaches 5 to 120 feet bright and 0 to 120 feet dim, in steps of 5")
	}
	b.light = true
	l := effects.Light{BrightFt: p.BrightFt, DimFt: p.DimFt}
	b.comps = append(b.comps, l)
	b.text = append(b.text, "Bright light fills "+strconv.Itoa(l.BrightFt)+" feet around the spell, and dim light another "+strconv.Itoa(l.DimFt)+" feet.")
	return nil
}

func (b *builder) reveal(p Part) error {
	if len(p.Qualities) == 0 {
		return DesignError("a reveal strips at least one Visibility Quality")
	}
	for _, q := range p.Qualities {
		if !slices.Contains(qualities(), q) {
			return DesignError("reveal Visibility Qualities: " + strings.Join(qualities(), ", "))
		}
	}
	b.comps = append(b.comps, effects.Reveal{Qualities: p.Qualities})
	names := make([]string, 0, len(p.Qualities))
	for _, q := range p.Qualities {
		names = append(names, title(q))
	}
	b.text = append(b.text, "Every creature and object in the area loses "+strings.Join(names, " and ")+".")
	return nil
}

func (b *builder) lay(p Part) error {
	switch {
	case b.ground:
		return DesignError("a spell leaves one Surface or one start-of-turn damage, not both")
	case !slices.Contains(b.surfaces, p.Surface):
		return DesignError("choose a Surface: " + strings.Join(b.surfaces, ", "))
	case p.Rounds < 1 || p.Rounds > 100:
		return DesignError("a Surface lasts 1 to 100 rounds")
	}
	b.ground = true
	b.comps = append(b.comps, effects.CreateSurface{Kind: surface.Kind(p.Surface), Rounds: p.Rounds})
	b.text = append(b.text, "The area is covered in "+strings.ReplaceAll(p.Surface, "-", " ")+" for "+strconv.Itoa(p.Rounds)+" rounds.")
	return nil
}

// finish puts the Effect together: the area first, the save the targets make when no damage names it,
// then the parts in order; and the rules text with the spell's metadata.
func (b *builder) finish() Built {
	d := b.d
	comps := []effects.Component{effects.Area{Shape: hex.Shape(d.Targeting.Shape), SizeFt: d.Targeting.SizeFt, RangeFt: d.Targeting.RangeFt}}
	if d.Save != "" && !b.saveDamage {
		comps = append(comps, effects.AreaSave{Ability: d.Save})
	}
	duration := effects.Duration{Kind: effects.DurationKind(d.Duration.Unit), Amount: d.Duration.Amount, RepeatSave: ""}
	def := effects.Definition{
		Slug: b.slug, Name: b.name, Owner: effects.OwnedBySpell, Concentration: d.Concentration, Duration: duration, Scaling: nil,
		Components: append(comps, b.comps...),
	}
	text := []string{castingText(d), componentsText(d.Components), areaText(d.Targeting), duration.Text(d.Concentration)}
	return Built{Definition: def, Surface: b.surface, Text: append(text, b.text...)}
}

func castingText(d Design) string {
	c := d.CastingTime
	text := map[string]string{"action": "1 action", "bonus_action": "1 bonus action", "minutes": strconv.Itoa(c.Minutes) + " minutes"}[c.Kind]
	if c.Kind == "reaction" {
		text = "1 reaction, which you take " + strings.ReplaceAll(c.Trigger, "_", " ")
	}
	if d.Ritual {
		text += " or Ritual"
	}
	return "Casting Time: " + text + "."
}

func componentsText(c Components) string {
	var parts []string
	if c.Verbal {
		parts = append(parts, "V")
	}
	if c.Somatic {
		parts = append(parts, "S")
	}
	if m := c.Material; m != nil {
		what := m.Text
		if m.CostGP > 0 {
			what += " worth " + strconv.Itoa(m.CostGP) + "+ gp"
		}
		if m.Consumed {
			what += ", which the spell consumes"
		}
		parts = append(parts, "M ("+what+")")
	}
	return "Components: " + strings.Join(parts, ", ") + "."
}

func areaText(t Targeting) string {
	where := "from the caster"
	if t.RangeFt > 0 {
		where = "at a point within " + strconv.Itoa(t.RangeFt) + " feet"
	}
	return "Area: a " + strconv.Itoa(t.SizeFt) + "-foot " + t.Shape + " " + where + "."
}

// Preview is the area as hexes around the origin, aimed east when it starts at the caster.
func Preview(t Targeting) []hex.Coord {
	aim := hex.Coord{Q: 0, R: 0}
	if t.RangeFt == 0 && t.Shape != "emanation" {
		aim = hex.Coord{Q: 1, R: 0}
	}
	return hex.Area(hex.Shape(t.Shape), hex.Coord{Q: 0, R: 0}, aim, t.SizeFt)
}

func qualities() []string {
	return []string{"hidden", "invisible", "disguised", "illusory", "ethereal"}
}

// title capitalises a non-empty name.
func title(s string) string {
	return strings.ToUpper(s[:1]) + s[1:]
}

// Slug is the Effect slug a Library entry's homebrew spell runs under in play.
func Slug(entryID string) string {
	return "hb-" + strings.ReplaceAll(entryID, "-", "")[:12]
}
