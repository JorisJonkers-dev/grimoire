// Package spellbuild turns a homebrew spell's design, its Targeting and the Parts it holds, each of a kind
// the rules know, into an Effect the rules engine runs, with its rules text.
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
	// TurnDamage hurts each creature that starts its turn in the area, through a Surface of the spell's own.
	TurnDamage *TurnDamage        `json:"turnDamage,omitempty"`
	Parts      []effects.Authored `json:"parts"`
	// Scaling grows the spell's area or its Activation; a Part's own Scaling sits on the Part.
	Scaling []effects.AuthoredScaling `json:"scaling,omitempty"`
}

// TurnDamage is damage a creature takes when it starts its turn in the spell's area.
type TurnDamage struct {
	Dice       string `json:"dice"`
	DamageType string `json:"damageType"`
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
// a Part may lay down.
func Build(slug, name string, d Design, surfaces []string) (Built, error) {
	if err := d.check(); err != nil {
		return Built{}, err
	}
	parts, err := d.parts(surfaces)
	if err != nil {
		return Built{}, err
	}
	scaling, err := d.scaling()
	if err != nil {
		return Built{}, err
	}
	return d.finish(slug, name, parts, scaling), nil
}

// Activation is what a spell's own Scaling grows, with which of them may follow a formula: its area's
// size and range, and the Activation's duration, its unit and concentration.
func Activation() ([]effects.Field, []effects.Scalable) {
	area := areaPlace()
	own := activationPlace()
	return append(area.Fields, own.Fields...), append(area.Scales, own.Scales...)
}

// activationPlace is the Activation's values a spell's Scaling grows; an area spell picks no targets, so
// their number does not grow.
func activationPlace() effects.Place {
	p := effects.ActivationPlace()
	p.Fields = slices.DeleteFunc(p.Fields, func(f effects.Field) bool { return f.Name == "targets" })
	p.Scales = slices.DeleteFunc(p.Scales, func(s effects.Scalable) bool { return s.Field == "targets" })
	return p
}

// areaPlace is the spell's area as a place Scaling grows: its scalable fields, on the Effect's first Part.
func areaPlace() effects.Place {
	k, _ := effects.KindOf("area")
	scales := effects.ScalesOf(k)
	fields := slices.DeleteFunc(k.Describe().Fields, func(f effects.Field) bool {
		return !slices.ContainsFunc(scales, func(s effects.Scalable) bool { return s.Field == f.Name })
	})
	return effects.Place{Part: 0, Fields: fields, Scales: scales}
}

// scaling decodes the spell's own Scaling, then each Part's, counting the Parts after the area and the
// save it adds ahead of them.
func (d Design) scaling() ([]effects.Scaling, error) {
	own, err := effects.DecodePlaced(d.Scaling, []effects.Place{areaPlace(), activationPlace()}, "the spell")
	if err != nil {
		return nil, DesignError(err.Error())
	}
	parts, err := effects.DecodeScaling(d.Parts, len(d.head()))
	if err != nil {
		return nil, DesignError(err.Error())
	}
	return append(own, parts...), nil
}

// head is what a spell puts ahead of its Parts: its area, and the save the targets make when no damage
// names it.
func (d Design) head() []effects.Part {
	head := []effects.Part{effects.Area{Shape: hex.Shape(d.Targeting.Shape), SizeFt: d.Targeting.SizeFt, RangeFt: d.Targeting.RangeFt}}
	if d.Save != "" && count(d.Parts, "save_damage") == 0 {
		head = append(head, effects.AreaSave{Ability: d.Save})
	}
	return head
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
		return DesignError("keep it to 30 Parts")
	}
	if err := d.Duration.check(d.Concentration); err != nil {
		return err
	}
	if err := d.CastingTime.check(d.Ritual); err != nil {
		return err
	}
	if err := d.Components.check(); err != nil {
		return err
	}
	return d.turnDamage()
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

func (d Design) turnDamage() error {
	t := d.TurnDamage
	if t == nil {
		return nil
	}
	if !dicePattern.MatchString(t.Dice) {
		return DesignError("damage dice look like 2d6")
	}
	if !slices.Contains(DamageTypes(), t.DamageType) {
		return DesignError("choose a damage type: " + strings.Join(DamageTypes(), ", "))
	}
	if d.rounds() == 0 {
		return DesignError("start-of-turn damage needs a duration in rounds, minutes or hours")
	}
	return nil
}

// rounds is how many rounds the spell lasts, 0 when its end is not counted.
func (d Design) rounds() int {
	return effects.Duration{Kind: effects.DurationKind(d.Duration.Unit), Amount: d.Duration.Amount, RepeatSave: ""}.Rounds()
}

// finish puts the Effect together: the area first, the save the targets make when no damage names it,
// then the Parts in order and the Surface its start-of-turn damage lies on; and the rules text with the
// spell's metadata.
func (d Design) finish(slug, name string, parts []effects.Part, scaling []effects.Scaling) Built {
	head := d.head()
	duration := effects.Duration{Kind: effects.DurationKind(d.Duration.Unit), Amount: d.Duration.Amount, RepeatSave: ""}
	text := []string{castingText(d), componentsText(d.Components), areaText(d.Targeting), duration.Text(d.Concentration)}
	for _, p := range parts {
		text = append(text, p.Text())
	}
	var ground *Surface
	if t := d.TurnDamage; t != nil {
		ground = &Surface{Slug: slug + "-ground", Name: name, Dice: t.Dice, Type: t.DamageType}
		parts = append(parts, effects.CreateSurface{Surface: surface.Kind(ground.Slug), Rounds: d.rounds()})
		text = append(text, "A creature that starts its turn in the area takes "+t.Dice+" "+t.DamageType+" damage.")
	}
	def := effects.Definition{
		Slug: slug, Name: name, Owner: effects.OwnedBySpell, Concentration: d.Concentration, Duration: duration, Scaling: scaling,
		Targeting: effects.Targeting{Kind: "", Count: 0, RangeFt: 0, Side: effects.AnySide, Types: nil, Seen: false, Willing: false, SpreadFt: 0},
		Save:      effects.InitialSave{Ability: "", DC: 0},
		Parts:     append(head, parts...),
	}
	for _, s := range scaling {
		f, _ := def.Scaled(s)
		text = append(text, s.Text(f))
	}
	return Built{Definition: def, Surface: ground, Text: text}
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

// Slug is what a Library entry's homebrew is known by in play: a spell's Effect slug, an item's slug.
func Slug(entryID string) string {
	return "hb-" + strings.ReplaceAll(entryID, "-", "")[:12]
}
