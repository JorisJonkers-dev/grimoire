// Package itembuild turns a homebrew item's design, its kind, base item, rarity, enchantment,
// attunement and Item Properties listed as rows, into what play needs of it, its item card and a Price
// Check.
package itembuild

import (
	"regexp"
	"slices"
	"strings"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Design is a homebrew item as its author builds it.
type Design struct {
	Kind        string      `json:"kind"`
	Base        string      `json:"base,omitempty"`
	Rarity      string      `json:"rarity"`
	Enchantment int         `json:"enchantment"`
	WeightLb    float64     `json:"weightLb"`
	ValueGP     int         `json:"valueGp"`
	Attunement  *Attunement `json:"attunement,omitempty"`
	Weapon      *Weapon     `json:"weapon,omitempty"`
	Charges     *Charges    `json:"charges,omitempty"`
	Properties  []Property  `json:"properties"`
}

// Attunement says an item needs attuning, and who may attune to it.
type Attunement struct {
	Kind  string `json:"kind,omitempty"`
	Value string `json:"value,omitempty"`
}

// Weapon is a weapon's properties and Weapon Mastery, standard or custom.
type Weapon struct {
	Properties []string `json:"properties"`
	Mastery    string   `json:"mastery,omitempty"`
	Custom     string   `json:"custom,omitempty"`
}

// Charges are how many charges an item holds and what it regains on its schedule: dice and a bonus.
type Charges struct {
	Max   int    `json:"max"`
	On    string `json:"on"`
	Dice  int    `json:"dice,omitempty"`
	Faces int    `json:"faces,omitempty"`
	Bonus int    `json:"bonus,omitempty"`
}

// Property is one row of a design; which fields count depends on its Type. Hidden properties show only
// once the item is identified or attuned.
type Property struct {
	Type   string `json:"type"`
	Hidden bool   `json:"hidden,omitempty"`
	// Skill Boosts, bonuses, resistances, extra damage, senses and speeds.
	Skill  string `json:"skill,omitempty"`
	Mode   string `json:"mode,omitempty"`
	Target string `json:"target,omitempty"`
	Value  int    `json:"value,omitempty"`
	Dice   string `json:"dice,omitempty"`
	Damage string `json:"damage,omitempty"`
	Sense  string `json:"sense,omitempty"`
	Speed  string `json:"speed,omitempty"`
	Feet   int    `json:"feet,omitempty"`
	// Granted cantrips and spells: a spell costs Cost charges, or none at will.
	Spell string `json:"spell,omitempty"`
	Name  string `json:"name,omitempty"`
	Level int    `json:"level,omitempty"`
	Cost  int    `json:"cost,omitempty"`
	// Light.
	BrightFt int `json:"brightFt,omitempty"`
	DimFt    int `json:"dimFt,omitempty"`
	// Consumables: used up at once, or until a long rest, a coating for some hits.
	Uses string `json:"uses,omitempty"`
	Hits int    `json:"hits,omitempty"`
	// Curses, sentient items, growth, sets, triggers and anything else read as text; a curse may stop
	// the item coming off.
	Text       string `json:"text,omitempty"`
	CannotDrop bool   `json:"cannotDrop,omitempty"`
	AtLevel    int    `json:"atLevel,omitempty"`
	Set        string `json:"set,omitempty"`
	Pieces     int    `json:"pieces,omitempty"`
	// Containers.
	CapacityLb int    `json:"capacityLb,omitempty"`
	Weightless bool   `json:"weightless,omitempty"`
	OnlyKind   string `json:"onlyKind,omitempty"`
	// Firearms.
	Misfire int `json:"misfire,omitempty"`
	Reload  int `json:"reload,omitempty"`
	Burst   int `json:"burst,omitempty"`
}

// Property types.
const (
	SkillBoost  = "skill_boost"
	Bonus       = "bonus"
	Resistance  = "resistance"
	ExtraDamage = "extra_damage"
	SenseRow    = "sense"
	SpeedRow    = "speed"
	Cantrip     = "cantrip"
	SpellRow    = "spell"
	LightRow    = "light"
	Consumable  = "consumable"
	Curse       = "curse"
	Sentient    = "sentient"
	Growth      = "growth"
	SetBonus    = "set_bonus"
	Container   = "container"
	Firearm     = "firearm"
	Trigger     = "trigger"
	ManualRow   = "manual"
)

// Kinds are the items the builder makes.
func Kinds() []string {
	return []string{
		"weapon", "armor", "shield", "helmet", "cloak", "gloves", "boots", "amulet", "ring", "clothing", "instrument", "ammunition",
		"potion", "scroll", "throwable", "coating", "wand", "container", "trinket",
	}
}

// Rarities, cheapest first.
func Rarities() []string {
	return []string{"common", "uncommon", "rare", "very_rare", "legendary"}
}

// PropertyTypes are the rows an item can carry.
func PropertyTypes() []string {
	return []string{SkillBoost, Bonus, Resistance, ExtraDamage, SenseRow, SpeedRow, Cantrip, SpellRow, LightRow, Consumable, Curse, Sentient, Growth, SetBonus, Container, Firearm, Trigger, ManualRow}
}

// BoostModes are the one-click Skill Boosts.
func BoostModes() []string {
	return []string{"advantage", "d4", "flat", "proficiency", "expertise"}
}

// Skills are the skills a Skill Boost can name.
func Skills() []string {
	return []string{
		"acrobatics", "animal-handling", "arcana", "athletics", "deception", "history", "insight", "intimidation", "investigation",
		"medicine", "nature", "perception", "performance", "persuasion", "religion", "sleight-of-hand", "stealth", "survival",
	}
}

// BonusTargets are what a flat bonus adds to.
func BonusTargets() []string {
	return []string{"attack", "damage", "ac", "saves", "spell_attack", "spell_dc", "initiative"}
}

// DamageTypes are the kinds of damage.
func DamageTypes() []string {
	return []string{"acid", "bludgeoning", "cold", "fire", "force", "lightning", "necrotic", "piercing", "poison", "psychic", "radiant", "slashing", "thunder"}
}

// WeaponProperties are the standard weapon properties.
func WeaponProperties() []string {
	return []string{"ammunition", "finesse", "heavy", "light", "loading", "reach", "thrown", "two-handed", "versatile"}
}

// Masteries are the standard Weapon Masteries; "custom" names one of the author's own.
func Masteries() []string {
	return []string{"cleave", "graze", "nick", "push", "sap", "slow", "topple", "vex", "custom"}
}

var (
	dicePattern = regexp.MustCompile(`^[1-9][0-9]?d(4|6|8|10|12|20)$`)
	slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,79}$`)
)

// Check validates a design.
func Check(d Design) error {
	for _, check := range []func(Design) error{basics, attunement, weapon, charges} {
		if err := check(d); err != nil {
			return err
		}
	}
	if len(d.Properties) > 40 {
		return DesignError("keep it to 40 Item Properties")
	}
	for _, p := range d.Properties {
		if err := property(p, d); err != nil {
			return err
		}
	}
	return nil
}

func basics(d Design) error {
	switch {
	case !slices.Contains(Kinds(), d.Kind):
		return DesignError("choose a kind of item: " + strings.Join(Kinds(), ", "))
	case d.Base != "" && !slugPattern.MatchString(d.Base):
		return DesignError("name the base item by its slug, like longbow")
	case !slices.Contains(Rarities(), d.Rarity):
		return DesignError("choose a rarity: " + strings.Join(Rarities(), ", "))
	case d.Enchantment < 0 || d.Enchantment > 3:
		return DesignError("an enchantment is +0 to +3")
	case d.WeightLb < 0 || d.WeightLb > 1000:
		return DesignError("an item weighs 0 to 1000 pounds")
	case d.ValueGP < 0 || d.ValueGP > 10000000:
		return DesignError("an item is worth 0 to 10000000 gp")
	}
	return nil
}

func attunement(d Design) error {
	a := d.Attunement
	switch {
	case a == nil || a.Kind == "":
		return nil
	case !slices.Contains([]string{"class", "species", "background", "alignment"}, a.Kind):
		return DesignError("attunement can require a class, species, background or alignment")
	case strings.TrimSpace(a.Value) == "" || len([]rune(a.Value)) > 40:
		return DesignError("name what attunement requires in up to 40 characters")
	}
	return nil
}

func weapon(d Design) error {
	w := d.Weapon
	switch {
	case w == nil:
		return nil
	case d.Kind != "weapon":
		return DesignError("only a weapon has weapon properties and a mastery")
	case w.Mastery != "" && !slices.Contains(Masteries(), w.Mastery):
		return DesignError("choose a Weapon Mastery: " + strings.Join(Masteries(), ", "))
	case w.Mastery == "custom" && strings.TrimSpace(w.Custom) == "":
		return DesignError("describe the custom Weapon Mastery")
	case len([]rune(w.Custom)) > 300:
		return DesignError("describe the custom Weapon Mastery in up to 300 characters")
	}
	for _, p := range w.Properties {
		if !slices.Contains(WeaponProperties(), p) {
			return DesignError("choose weapon properties: " + strings.Join(WeaponProperties(), ", "))
		}
	}
	return nil
}

func charges(d Design) error {
	c := d.Charges
	switch {
	case c == nil:
		return nil
	case c.Max < 1 || c.Max > 100:
		return DesignError("an item holds 1 to 100 charges")
	case !slices.Contains([]string{"dawn", "long_rest", "short_rest"}, c.On):
		return DesignError("charges come back at dawn, on a long rest or on a short rest")
	case c.Dice < 0 || c.Dice > 10 || c.Bonus < 0 || c.Bonus > 100:
		return DesignError("charges come back by up to 10 dice and a bonus of up to 100")
	case c.Dice > 0 && !slices.Contains([]int{4, 6, 8, 10, 12, 20}, c.Faces):
		return DesignError("roll d4, d6, d8, d10, d12 or d20 for the charges that come back")
	}
	return nil
}

func property(p Property, d Design) error {
	if !slices.Contains(PropertyTypes(), p.Type) {
		return DesignError("an Item Property is one of: " + strings.Join(PropertyTypes(), ", "))
	}
	switch p.Type {
	case SkillBoost, Bonus, Resistance, ExtraDamage, SenseRow, SpeedRow:
		return passive(p)
	case Cantrip, SpellRow:
		return spell(p, d)
	case LightRow, Consumable, Container, Firearm:
		return mechanical(p)
	default:
		return story(p)
	}
}

func passive(p Property) error {
	switch {
	case p.Type == SkillBoost && !slices.Contains(Skills(), p.Skill):
		return DesignError("a Skill Boost names a skill")
	case p.Type == SkillBoost && !slices.Contains(BoostModes(), p.Mode):
		return DesignError("a Skill Boost gives advantage, +1d4, a flat bonus, proficiency or expertise")
	case p.Type == SkillBoost && p.Mode == "flat" && (p.Value < 1 || p.Value > 5):
		return DesignError("a flat Skill Boost is +1 to +5")
	case p.Type == Bonus && (!slices.Contains(BonusTargets(), p.Target) || p.Value < 1 || p.Value > 5):
		return DesignError("a bonus adds +1 to +5 to " + strings.Join(BonusTargets(), ", "))
	case (p.Type == Resistance || p.Type == ExtraDamage) && !slices.Contains(DamageTypes(), p.Damage):
		return DesignError("choose a damage type: " + strings.Join(DamageTypes(), ", "))
	case p.Type == ExtraDamage && !dicePattern.MatchString(p.Dice):
		return DesignError("extra damage dice look like 1d6")
	case p.Type == SenseRow && (!slices.Contains([]string{"darkvision", "blindsight", "tremorsense", "truesight"}, p.Sense) || p.Feet < 5 || p.Feet > 300):
		return DesignError("a sense is darkvision, blindsight, tremorsense or truesight out to 5 to 300 feet")
	case p.Type == SpeedRow && (!slices.Contains([]string{"walk", "fly", "swim", "climb", "burrow"}, p.Speed) || p.Feet < 5 || p.Feet > 120):
		return DesignError("a speed is walk, fly, swim, climb or burrow of 5 to 120 feet")
	}
	return nil
}

func spell(p Property, d Design) error {
	switch {
	case !slugPattern.MatchString(p.Spell) || strings.TrimSpace(p.Name) == "":
		return DesignError("a granted spell names a spell")
	case p.Type == Cantrip && (p.Level != 0 || p.Cost != 0):
		return DesignError("a cantrip is level 0 and costs no charges")
	case p.Type == SpellRow && (p.Level < 1 || p.Level > 9):
		return DesignError("a granted spell is level 1 to 9")
	case p.Cost < 0 || p.Cost > 100:
		return DesignError("a spell costs 0 to 100 charges")
	case p.Cost > 0 && d.Charges == nil:
		return DesignError("a spell that costs charges needs an item that holds them")
	}
	return nil
}

func mechanical(p Property) error {
	check := map[string]func(Property) bool{
		LightRow: func(p Property) bool { return p.BrightFt < 5 || p.BrightFt > 120 || p.DimFt < 0 || p.DimFt > 120 },
		Consumable: func(p Property) bool {
			return !slices.Contains([]string{"single", "long_rest", "coating"}, p.Uses) || (p.Uses == "coating" && (p.Hits < 1 || p.Hits > 100))
		},
		Container: func(p Property) bool {
			return p.CapacityLb < 1 || p.CapacityLb > 10000 || (p.OnlyKind != "" && !slices.Contains(Kinds(), p.OnlyKind))
		},
		Firearm: func(p Property) bool {
			return p.Misfire < 0 || p.Misfire > 20 || p.Reload < 0 || p.Reload > 20 || p.Burst < 0 || p.Burst > 10
		},
	}
	reasons := map[string]string{
		LightRow:   "light reaches 5 to 120 feet bright and 0 to 120 feet dim",
		Consumable: "a consumable is used once, until a long rest, or coats a weapon for 1 to 100 hits",
		Container:  "a container holds 1 to 10000 pounds, of one kind of item if you like",
		Firearm:    "a firearm misfires on 0 to 20, reloads after 0 to 20 shots and fires bursts of 0 to 10",
	}
	if check[p.Type](p) {
		return DesignError(reasons[p.Type])
	}
	return nil
}

func story(p Property) error {
	switch {
	case strings.TrimSpace(p.Text) == "" || len([]rune(p.Text)) > 1000:
		return DesignError("say what it does in up to 1000 characters")
	case p.Type == Growth && (p.AtLevel < 1 || p.AtLevel > 20):
		return DesignError("an item grows at a character level from 1 to 20")
	case p.Type == SetBonus && (strings.TrimSpace(p.Set) == "" || p.Pieces < 2 || p.Pieces > 10):
		return DesignError("a set bonus names its set and needs 2 to 10 pieces")
	}
	return nil
}

// Granted is a spell an item lets its bearer cast: at will, or for some charges.
type Granted struct {
	Spell string
	Name  string
	Level int
	Cost  int
}

// Spells are the spells an item grants, in the order they come.
func Spells(d Design) []Granted {
	var out []Granted
	for _, p := range d.Properties {
		if p.Type == Cantrip || p.Type == SpellRow {
			out = append(out, Granted{Spell: p.Spell, Name: p.Name, Level: p.Level, Cost: p.Cost})
		}
	}
	return out
}

// Category is the compendium's kind of item a design counts as in play.
func Category(d Design) string {
	switch d.Kind {
	case "weapon", "ammunition", "throwable":
		return "weapon"
	case "armor", "helmet", "gloves", "boots", "clothing":
		return "armor"
	case "amulet", "cloak", "trinket", "container", "instrument", "coating":
		return "wondrous-item"
	}
	return d.Kind
}
