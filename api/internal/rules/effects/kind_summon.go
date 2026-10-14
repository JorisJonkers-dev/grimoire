package effects

import "strconv"

// Summon brings creatures in under the caster's control: Count of one monster, acting on the caster's
// turn (Shares) or on their own Initiative, and taking only the Dodge action unless the caster spends a
// Bonus Action to command them (NeedsCommand).
type Summon struct {
	Monster      string
	Count        int
	Shares       bool
	NeedsCommand bool
}

func (Summon) isPart() {}

// Kind is summon.
func (Summon) Kind() string { return "summon" }

// Text says who comes, when they act and whether they need commanding.
func (s Summon) Text() string {
	who, acts, rolls, it, them := strconv.Itoa(s.Count)+" "+title(s.Monster)+" creatures", "act", "roll their", "They take", "them"
	if s.Count == 1 {
		who, acts, rolls, it, them = article(s.Monster)+" "+title(s.Monster), "acts", "rolls its", "It takes", "it"
	}
	text := "The caster summons " + who + ", which " + rolls + " own Initiative."
	if s.Shares {
		text = "The caster summons " + who + ", which " + acts + " on the caster's turn."
	}
	if s.NeedsCommand {
		text += " " + it + " the Dodge action unless the caster commands " + them + " with a Bonus Action."
	}
	return text
}

// LandsOnBearer brings the creatures in for the caster.
func (s Summon) LandsOnBearer() []Act {
	return []Act{SummonCreatures(s)}
}

type summonKind struct{}

func (summonKind) Describe() Description {
	return Description{
		Slug: "summon", Label: "Summon", Summary: "Brings creatures in under the caster's control.",
		Fields: []Field{
			{Name: "monster", Label: "Creature", Type: TextField, Options: nil, Required: true, Min: 0, Max: 80},
			{Name: "count", Label: "How many", Type: NumberField, Options: nil, Required: true, Min: 1, Max: 20},
			{Name: "shares", Label: "Acts on the caster's turn", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
			{Name: "needs_command", Label: "Dodges unless commanded", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k summonKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	monster, err := v.text(d.field("monster"))
	if err != nil {
		return nil, err
	}
	count, err := v.number(d.field("count"))
	if err != nil {
		return nil, err
	}
	return Summon{Monster: monster, Count: count, Shares: v["shares"].Flag, NeedsCommand: v["needs_command"].Flag}, nil
}
