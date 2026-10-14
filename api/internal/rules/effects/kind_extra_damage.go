package effects

// ExtraDamage adds dice to hits against the bearer by the effect's source (Hunter's Mark).
type ExtraDamage struct {
	Dice string
}

func (ExtraDamage) isComponent() {}

// Kind is extra_damage.
func (ExtraDamage) Kind() string { return "extra_damage" }

// Text is the rules sentence.
func (x ExtraDamage) Text() string {
	return "The source deals an extra " + x.Dice + " damage whenever it hits the target."
}

// FoldAttacked adds the dice to the damage of the Effect's source, with a note naming the Effect.
func (x ExtraDamage) FoldAttacked(p *AttackProfile, h Held, by Attacker) {
	if by.Source {
		p.DamageDice = append(p.DamageDice, x.Dice)
		p.Notes = append(p.Notes, h.Name+": +"+x.Dice+" damage")
	}
}

type extraDamageKind struct{}

func (extraDamageKind) Describe() Description {
	return Description{
		Slug: "extra_damage", Label: "Extra damage", Summary: "Adds dice to every hit the Effect's source lands on the bearer.",
		Fields: []Field{
			{Name: "dice", Label: "Damage", Type: DiceField, Options: nil, Required: true, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (extraDamageKind) Decode(v Values, _ Groups) (Part, error) {
	x := ExtraDamage{Dice: v["dice"].Text}
	if !dicePattern.MatchString(x.Dice) {
		return nil, refuse("dice", "write the damage like 1d6")
	}
	return x, nil
}
