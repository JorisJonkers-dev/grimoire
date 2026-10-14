package effects

// SaveDamage hurts every creature in the area; a successful save halves it with Half, or avoids it.
type SaveDamage struct {
	Ability string
	Dice    string
	Type    string
	Half    bool
}

func (SaveDamage) isComponent() {}

// Kind is save_damage.
func (SaveDamage) Kind() string { return "save_damage" }

// Text is the rules sentence.
func (s SaveDamage) Text() string {
	text := "Each creature in the area makes a " + title(s.Ability) + " saving throw, taking " + s.Dice + " " + s.Type + " damage on a failed save"
	if s.Half {
		return text + " or half as much damage on a successful one."
	}
	return text + "."
}

// Asks for every creature's save and one roll of the damage.
func (s SaveDamage) Asks() []Act {
	return []Act{RollSave{Ability: s.Ability}, RollDamage{Dice: s.Dice, Type: s.Type}}
}

// Lands hurts a creature by all the damage rolled, by half of it when it saved and the damage halves, or
// not at all.
func (s SaveDamage) Lands(o Outcome) []Act {
	amount := o.Damage
	if o.Saved {
		amount = map[bool]int{true: amount / 2, false: 0}[s.Half]
	}
	return []Act{Hurt{Amount: amount, Type: s.Type}}
}

type saveDamageKind struct{}

func (saveDamageKind) Describe() Description {
	return Description{
		Slug: "save_damage", Label: "Save for damage", Summary: "Each creature in the area makes a saving throw or takes the damage.",
		Fields: []Field{
			{Name: "ability", Label: "Saving throw", Type: ChoiceField, Options: abilities(), Required: true, Min: 0, Max: 0},
			{Name: "dice", Label: "Damage", Type: DiceField, Options: nil, Required: true, Min: 0, Max: 0},
			{Name: "damage_type", Label: "Damage type", Type: ChoiceField, Options: damageTypes(), Required: true, Min: 0, Max: 0},
			{Name: "half", Label: "Half damage on a successful save", Type: FlagField, Options: nil, Required: false, Min: 0, Max: 0},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (saveDamageKind) Decode(v Values, _ Groups) (Part, error) {
	s := SaveDamage{Ability: v["ability"].Text, Dice: v["dice"].Text, Type: v["damage_type"].Text, Half: v["half"].Flag}
	if err := oneOf("ability", s.Ability, abilities()); err != nil {
		return nil, err
	}
	if !dicePattern.MatchString(s.Dice) {
		return nil, refuse("dice", "write the damage like 8d6")
	}
	if err := oneOf("damage_type", s.Type, damageTypes()); err != nil {
		return nil, err
	}
	return s, nil
}
