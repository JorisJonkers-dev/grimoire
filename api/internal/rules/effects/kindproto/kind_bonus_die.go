package kindproto

import (
	"errors"
	"regexp"
	"strings"
)

var dicePattern = regexp.MustCompile(`^[0-9]+d[0-9]+$`) //nolint:gochecknoglobals // prototype

// BonusDie adds a die to some of the bearer's rolls (Bless).
type BonusDie struct {
	Dice string
	On   []string
}

type bonusDieKind struct{}

func (bonusDieKind) Describe() Description {
	return Description{
		Slug: "bonus_die", Label: "Bonus die", Summary: "Adds a die to some of the bearer's rolls while the Effect lasts.",
		Fields: []Field{
			{Name: "dice", Label: "Die", Type: "dice", Required: true, Help: "Such as 1d4."},
			{Name: "on", Label: "Added to", Type: "enums", Options: []string{"attack rolls", "saving throws"}, Required: true},
		},
		Table:    Table{Name: "effect_bonus_dice", Columns: []string{"dice text CHECK (dice ~ '^[0-9]+d[0-9]+$')", "on_attacks boolean", "on_saves boolean"}},
		Template: "The bearer adds {dice} to its {on}.",
		Files:    files("bonus_die"),
	}
}

func (bonusDieKind) Decode(v Values) (Part, error) {
	b := BonusDie{Dice: str(v, "dice"), On: list(v, "on")}
	if !dicePattern.MatchString(b.Dice) {
		return nil, errors.New("Die must read like 1d4.")
	}
	if len(b.On) == 0 {
		return nil, errors.New("Choose at least one kind of roll.")
	}
	return b, nil
}

// Text is the rules sentence.
func (b BonusDie) Text([]string) string {
	return "The bearer adds " + b.Dice + " to its " + strings.Join(b.On, " and ") + "."
}

// FoldAttack joins the die to an attack roll: the kind's whole part in play, next to its definition.
func (b BonusDie) FoldAttack(p *AttackProfile) {
	for _, on := range b.On {
		if on == "attack rolls" {
			p.BonusDice = append(p.BonusDice, b.Dice)
		}
	}
}
