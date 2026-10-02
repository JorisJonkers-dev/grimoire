// Package surface is terrain an effect leaves on hexes: what it does to movement, to sight and to
// creatures in it, and how damage turns one surface into another. Surfaces are data: a Catalog of
// Definitions, the built-in ones and any an author adds.
package surface

import "slices"

// Kind is a Surface.
type Kind string

// Built-in Surfaces. None is bare ground.
const (
	None          Kind = ""
	Fire          Kind = "fire"
	Grease        Kind = "grease"
	Water         Kind = "water"
	Ice           Kind = "ice"
	Web           Kind = "web"
	Electrified   Kind = "electrified"
	Fog           Kind = "fog"
	Darkness      Kind = "darkness"
	StinkingCloud Kind = "stinking-cloud"
	PlantGrowth   Kind = "plant-growth"
	Spikes        Kind = "spikes"
	Mud           Kind = "mud"
	Lava          Kind = "lava"
	Consecrated   Kind = "consecrated"
)

// Obscurement is how a Surface hides what stands in it, named as the Visibility Quality it gives.
type Obscurement string

// Obscurements.
const (
	Clear         Obscurement = ""
	ObscuredDark  Obscurement = "darkness"
	ObscuredHeavy Obscurement = "heavy"
)

// Reaction is what damage of a type turns a Surface into.
type Reaction struct {
	Damage  string
	Becomes Kind
}

// Definition is a Surface: what moving through it costs (a multiplier on each foot), what it hides,
// the damage it deals a creature that enters it or starts its turn there (or on every step, for
// spikes), the Effect it puts on such a creature, and how damage changes it.
type Definition struct {
	Kind       Kind
	Name       string
	Cost       int
	Obscures   Obscurement
	HazardDice string
	HazardType string
	EveryStep  bool
	Effect     string
	Reactions  []Reaction
}

// Catalog is every Surface the engine knows, keyed by kind.
type Catalog map[Kind]Definition

// Builtin is the Surfaces every Campaign starts with.
func Builtin() Catalog {
	none := Definition{Kind: None, Name: "", Cost: 1, Obscures: Clear, HazardDice: "", HazardType: "", EveryStep: false, Effect: "", Reactions: nil}
	def := func(k Kind, name string, change func(*Definition)) Definition {
		d := none
		d.Kind, d.Name = k, name
		change(&d)
		return d
	}
	burn := Reaction{Damage: "fire", Becomes: None}
	out := Catalog{}
	for _, d := range []Definition{
		def(Fire, "Fire", func(d *Definition) {
			d.HazardDice, d.HazardType, d.Reactions = "1d4", "fire", []Reaction{{Damage: "cold", Becomes: None}}
		}),
		def(Grease, "Grease", func(d *Definition) { d.Cost, d.Reactions = 2, []Reaction{{Damage: "fire", Becomes: Fire}} }),
		def(Water, "Water", func(d *Definition) {
			d.Reactions = []Reaction{{Damage: "cold", Becomes: Ice}, {Damage: "lightning", Becomes: Electrified}}
		}),
		def(Ice, "Ice", func(d *Definition) { d.Cost, d.Reactions = 2, []Reaction{{Damage: "fire", Becomes: Water}} }),
		def(Web, "Web", func(d *Definition) { d.Cost, d.Reactions = 2, []Reaction{burn} }),
		def(Electrified, "Electrified water", func(d *Definition) { d.HazardDice, d.HazardType = "1d4", "lightning" }),
		def(Fog, "Fog", func(d *Definition) { d.Obscures = ObscuredHeavy }),
		def(Darkness, "Magical darkness", func(d *Definition) { d.Obscures = ObscuredDark }),
		def(StinkingCloud, "Stinking cloud", func(d *Definition) { d.Obscures, d.Effect = ObscuredHeavy, "poisoned" }),
		def(PlantGrowth, "Overgrowth", func(d *Definition) { d.Cost, d.Reactions = 4, []Reaction{burn} }),
		def(Spikes, "Spikes", func(d *Definition) { d.Cost, d.HazardDice, d.HazardType, d.EveryStep = 2, "2d4", "piercing", true }),
		def(Mud, "Mud", func(d *Definition) { d.Cost = 2 }),
		def(Lava, "Lava", func(d *Definition) { d.HazardDice, d.HazardType = "10d10", "fire" }),
		def(Consecrated, "Consecrated ground", func(d *Definition) { d.Effect = "bless" }),
	} {
		out[d.Kind] = d
	}
	return out
}

// Kinds lists every Surface in the catalogue, by kind.
func (c Catalog) Kinds() []Kind {
	out := make([]Kind, 0, len(c))
	for k := range c {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Valid reports whether a kind is a Surface in the catalogue.
func (c Catalog) Valid(k Kind) bool {
	_, ok := c[k]
	return ok && k != None
}

// Cost is what each foot of movement through a Surface costs, at least one.
func (c Catalog) Cost(k Kind) int {
	return max(1, c[k].Cost)
}

// Hazard is the damage a creature takes entering a Surface or starting its turn in it; every says it
// strikes on each step instead.
func (c Catalog) Hazard(k Kind) (dice, damageType string, every, ok bool) {
	d := c[k]
	return d.HazardDice, d.HazardType, d.EveryStep, d.HazardDice != ""
}

// Obscures is the Visibility Quality a Surface gives whatever stands in it.
func (c Catalog) Obscures(k Kind) Obscurement {
	return c[k].Obscures
}

// Effect is the Effect a Surface puts on a creature that enters it or starts its turn there.
func (c Catalog) Effect(k Kind) string {
	return c[k].Effect
}

// React is what damage of a type does to a Surface; a Surface without a reaction to it stays.
func (c Catalog) React(k Kind, damageType string) Kind {
	i := slices.IndexFunc(c[k].Reactions, func(r Reaction) bool { return r.Damage == damageType })
	if i < 0 {
		return k
	}
	return c[k].Reactions[i].Becomes
}
