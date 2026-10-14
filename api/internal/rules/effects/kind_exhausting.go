package effects

import (
	"strconv"
	"strings"
)

// Exhausting is exhaustion, or any condition that stacks: each level takes D20PerLevel from every d20
// test and SpeedFtPerLevel from speed, at DeathAt levels the bearer dies, and it rises no higher than
// MaxLevel (0 has no cap).
type Exhausting struct {
	D20PerLevel     int
	SpeedFtPerLevel int
	DeathAt         int
	MaxLevel        int
}

func (Exhausting) isPart() {}

// Kind is exhausting.
func (Exhausting) Kind() string { return "exhausting" }

// Text reads what each level takes, when it kills, and how high it rises.
func (e Exhausting) Text() string {
	var each []string
	if e.D20PerLevel > 0 {
		each = append(each, "gives a −"+strconv.Itoa(e.D20PerLevel)+" penalty to D20 Tests")
	}
	if e.SpeedFtPerLevel > 0 {
		each = append(each, "reduces Speed by "+strconv.Itoa(e.SpeedFtPerLevel)+" feet")
	}
	text := "It stacks"
	if len(each) > 0 {
		text = "Each level " + strings.Join(each, " and ")
	}
	if e.DeathAt > 0 {
		text += "; a creature dies at level " + strconv.Itoa(e.DeathAt)
	}
	if e.MaxLevel > 0 {
		text += "; it rises to level " + strconv.Itoa(e.MaxLevel) + " at most"
	}
	return text + "."
}

// FoldAttacking takes the penalty for every level off the bearer's attack rolls, with a note.
func (e Exhausting) FoldAttacking(p *AttackProfile, h Held) {
	p.Penalty += e.D20PerLevel * h.Levels
	p.Notes = append(p.Notes, h.Name+" "+strconv.Itoa(h.Levels)+": -"+strconv.Itoa(e.D20PerLevel*h.Levels)+" to hit")
}

// FoldSave takes the penalty for every level off the bearer's saving throws, whatever the ability.
func (e Exhausting) FoldSave(p *SaveProfile, h Held, _ string) {
	p.Penalty += e.D20PerLevel * h.Levels
}

// FoldMove takes the feet for every level off the bearer's speed; they add up with other effects.
func (e Exhausting) FoldMove(p *MoveProfile, h Held) {
	p.StackedFt += e.SpeedFtPerLevel * h.Levels
}

// Cap is MaxLevel.
func (e Exhausting) Cap() int { return e.MaxLevel }

// Kills reports whether the bearer dies at that many levels.
func (e Exhausting) Kills(levels int) bool {
	return e.DeathAt > 0 && levels >= e.DeathAt
}

type exhaustingKind struct{}

func (exhaustingKind) Describe() Description {
	return Description{
		Slug: "exhausting", Label: "Stacking levels", Summary: "Stacks in levels, each taking from d20 tests and speed, and may kill at a level.",
		Fields: []Field{
			{Name: "d20_per_level", Label: "D20 penalty per level", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 10},
			{Name: "speed_ft_per_level", Label: "Speed penalty per level", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 30},
			{Name: "death_at", Label: "Dies at level", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 10},
			{Name: "max_level", Label: "Highest level", Type: NumberField, Options: nil, Required: false, Min: 0, Max: 10},
		},
		Groups: nil, OpenGroups: false,
	}
}

func (k exhaustingKind) Decode(v Values, _ Groups) (Part, error) {
	d := k.Describe()
	var n [4]int
	for i, name := range []string{"d20_per_level", "speed_ft_per_level", "death_at", "max_level"} {
		got, err := v.number(d.field(name))
		if err != nil {
			return nil, err
		}
		n[i] = got
	}
	return Exhausting{D20PerLevel: n[0], SpeedFtPerLevel: n[1], DeathAt: n[2], MaxLevel: n[3]}, nil
}
