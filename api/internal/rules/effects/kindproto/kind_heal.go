package kindproto

import "errors"

// Heal restores Hit Points. It is the kind the prototype "adds": this file plus one line in kinds.go.
type Heal struct {
	Dice  string
	Bonus string
}

type healKind struct{}

func (healKind) Describe() Description {
	return Description{
		Slug: "heal", Label: "Heal", Summary: "The target regains Hit Points.",
		Fields: []Field{
			{Name: "dice", Label: "Hit Points", Type: "dice", Required: true, Help: "Such as 2d8."},
			{Name: "bonus", Label: "Plus", Type: "enum", Options: []string{"nothing", "spellcasting ability modifier"}},
		},
		Table:    Table{Name: "effect_heals", Columns: []string{"dice text CHECK (dice ~ '^[0-9]+d[0-9]+$')", "bonus text CHECK (bonus IN ('nothing', 'spellcasting ability modifier'))"}},
		Template: "The target regains {dice}{?bonus: plus your {bonus}} Hit Points.",
		Files:    files("heal"),
	}
}

func (healKind) Decode(v Values) (Part, error) {
	h := Heal{Dice: str(v, "dice"), Bonus: str(v, "bonus")}
	if !dicePattern.MatchString(h.Dice) {
		return nil, errors.New("Hit Points must read like 2d8.")
	}
	return h, nil
}

// Text is the rules sentence.
func (h Heal) Text([]string) string {
	t := "The target regains " + h.Dice
	if h.Bonus != "" && h.Bonus != "nothing" {
		t += " plus your " + h.Bonus
	}
	return t + " Hit Points."
}

// Land restores the Hit Points.
func (h Heal) Land(t Target) []Step {
	return []Step{{Do: "heal", Detail: t.Name + ": " + h.Dice}}
}
