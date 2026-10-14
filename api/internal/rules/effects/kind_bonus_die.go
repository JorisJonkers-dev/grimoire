package effects

import (
	"slices"
	"strings"
)

// Roll is a kind of d20 roll a bonus die can join.
type Roll int

// Rolls.
const (
	AttackRolls Roll = iota
	SavingThrows
)

// BonusDie adds a die to the bearer's rolls of some kinds (Bless).
type BonusDie struct {
	On   []Roll
	Dice string
}

func (BonusDie) isComponent() {}

// Kind is bonus_die.
func (BonusDie) Kind() string { return "bonus_die" }

// Text is the rules sentence.
func (b BonusDie) Text() string {
	names := make([]string, 0, len(b.On))
	for _, r := range b.On {
		names = append(names, rollNames()[r])
	}
	return "Add " + b.Dice + " to " + strings.Join(names, " and ") + "."
}

// FoldAttacking adds the die to the bearer's attack rolls, with a note naming the Effect.
func (b BonusDie) FoldAttacking(p *AttackProfile, h Held) {
	if slices.Contains(b.On, AttackRolls) {
		p.AttackDice = append(p.AttackDice, b.Dice)
		p.Notes = append(p.Notes, h.Name+": +"+b.Dice+" to hit")
	}
}

// FoldSave adds the die to the bearer's saving throws, whatever the ability.
func (b BonusDie) FoldSave(p *SaveProfile, _ Held, _ string) {
	if slices.Contains(b.On, SavingThrows) {
		p.Dice = append(p.Dice, b.Dice)
	}
}

func rollNames() map[Roll]string {
	return map[Roll]string{AttackRolls: "attack rolls", SavingThrows: "saving throws"}
}

type bonusDieKind struct{}

func (bonusDieKind) Describe() Description {
	return Description{
		Slug: "bonus_die", Label: "Bonus die", Summary: "Adds a die to some of the bearer's d20 rolls while the Effect lasts.",
		Fields: []Field{
			{Name: "dice", Label: "Die", Type: DiceField, Options: nil, Required: true, Min: 0, Max: 0},
			{Name: "on", Label: "Added to", Type: ChoicesField, Options: []string{"attack_rolls", "saving_throws"}, Required: true, Min: 0, Max: 0},
		},
		Groups: nil,
	}
}

func (bonusDieKind) Decode(v Values, _ Groups) (Part, error) {
	b := BonusDie{On: nil, Dice: v["dice"].Text}
	if !dicePattern.MatchString(b.Dice) {
		return nil, refuse("dice", "write the die like 1d4")
	}
	on := v["on"].Texts
	for _, name := range on {
		if err := oneOf("on", name, []string{"attack_rolls", "saving_throws"}); err != nil {
			return nil, err
		}
	}
	if len(on) == 0 || len(slices.Compact(slices.Sorted(slices.Values(on)))) != len(on) {
		return nil, refuse("on", "choose each kind of roll once")
	}
	if slices.Contains(on, "attack_rolls") {
		b.On = append(b.On, AttackRolls)
	}
	if slices.Contains(on, "saving_throws") {
		b.On = append(b.On, SavingThrows)
	}
	return b, nil
}
