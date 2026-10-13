package kindproto

import "errors"

// SaveDamage is damage everyone in the area saves against.
type SaveDamage struct {
	Ability    string
	Dice       string
	DamageType string
	Half       bool
}

type saveDamageKind struct{}

func (saveDamageKind) Describe() Description {
	return Description{
		Slug: "save_damage", Label: "Save for damage", Summary: "Each creature in the area saves or takes damage.",
		Fields: []Field{
			{Name: "ability", Label: "Saving throw", Type: "enum", Options: []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}, Required: true},
			{Name: "dice", Label: "Damage", Type: "dice", Required: true, Help: "Such as 8d6."},
			{Name: "damage_type", Label: "Damage type", Type: "enum", Options: []string{"acid", "cold", "fire", "force", "lightning", "necrotic", "poison", "psychic", "radiant", "thunder"}, Required: true},
			{Name: "half", Label: "Half damage on a success", Type: "bool"},
		},
		Table:    Table{Name: "effect_save_damage", Columns: []string{"ability text CHECK (ability IN (...))", "dice text CHECK (dice ~ '^[0-9]+d[0-9]+$')", "damage_type text CHECK (damage_type IN (...))", "half boolean"}},
		Template: "Each creature in the area makes a {ability} saving throw, taking {dice} {damage_type} damage on a failure{?half:, or half as much on a success}.",
		Files:    files("save_damage"),
	}
}

func (saveDamageKind) Decode(v Values) (Part, error) {
	s := SaveDamage{Ability: str(v, "ability"), Dice: str(v, "dice"), DamageType: str(v, "damage_type"), Half: boolean(v, "half")}
	if s.Ability == "" || s.DamageType == "" {
		return nil, errors.New("Choose the saving throw and the damage type.")
	}
	if !dicePattern.MatchString(s.Dice) {
		return nil, errors.New("Damage must read like 8d6.")
	}
	return s, nil
}

// Text is the rules sentence.
func (s SaveDamage) Text([]string) string {
	t := "Each creature in the area makes a " + s.Ability + " saving throw, taking " + s.Dice + " " + s.DamageType + " damage on a failure"
	if s.Half {
		t += ", or half as much on a success"
	}
	return t + "."
}

// Land asks for the save and deals the damage, as steps the runtime applies.
func (s SaveDamage) Land(t Target) []Step {
	return []Step{{Do: "roll_save", Detail: t.Name + ": " + s.Ability}, {Do: "damage", Detail: s.Dice + " " + s.DamageType}}
}
