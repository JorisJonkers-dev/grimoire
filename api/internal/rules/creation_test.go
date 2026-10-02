package rules_test

import (
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

func TestPrimaryAbilities(t *testing.T) {
	t.Parallel()
	for class, want := range map[string][]rules.Ability{
		"barbarian": {rules.Strength}, "bard": {rules.Charisma}, "sorcerer": {rules.Charisma}, "warlock": {rules.Charisma},
		"cleric": {rules.Wisdom}, "druid": {rules.Wisdom}, "fighter": {rules.Strength, rules.Dexterity},
		"monk": {rules.Dexterity, rules.Wisdom}, "ranger": {rules.Dexterity, rules.Wisdom}, "paladin": {rules.Strength, rules.Charisma},
		"rogue": {rules.Dexterity}, "wizard": {rules.Intelligence}, "artificer": nil,
	} {
		if got := rules.PrimaryAbilities(class); !slices.Equal(got, want) {
			t.Errorf("%s = %v, want %v", class, got, want)
		}
	}
}

func TestHitPointsAt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ die, con, level, want int }{
		{10, 2, 1, 12},
		{10, 2, 3, 12 + 2*8},
		{8, 0, 5, 8 + 4*5},
		{6, -3, 3, 3 + 2*1},
		{12, -5, 2, 7 + 2},
		{6, -6, 2, 1 + 1},
		{10, 1, 0, 11},
	} {
		if got := rules.HitPointsAt(c.die, c.con, c.level); got != c.want {
			t.Errorf("d%d con %+d level %d = %d, want %d", c.die, c.con, c.level, got, c.want)
		}
	}
}

// seq yields its values in turn, as die faces from 0.
type seq struct {
	faces []int
	at    int
}

func (s *seq) IntN(int) int {
	v := s.faces[s.at%len(s.faces)]
	s.at++
	return v
}

func TestRollAbilityScores(t *testing.T) {
	t.Parallel()
	src := &seq{faces: []int{0, 5, 2, 3, 5, 5, 5, 5, 0, 0, 0, 0, 1, 1, 1, 0, 4, 3, 2, 1, 5, 0, 0, 5}}
	got := rules.RollAbilityScores(src)
	if want := []int{6 + 3 + 4, 18, 3, 2 + 2 + 2, 5 + 4 + 3, 6 + 6 + 1}; !slices.Equal(got, want) {
		t.Fatalf("rolled = %v, want %v", got, want)
	}
	if src.at != 24 {
		t.Fatalf("dice thrown = %d", src.at)
	}
}

func TestValidateRolled(t *testing.T) {
	t.Parallel()
	rolled := []int{15, 12, 9, 14, 8, 17}
	ok := map[rules.Ability]int{rules.Strength: 17, rules.Dexterity: 15, rules.Constitution: 14, rules.Intelligence: 12, rules.Wisdom: 9, rules.Charisma: 8}
	if err := rules.ValidateRolled(ok, rolled); err != nil {
		t.Fatalf("a rolled set = %v", err)
	}
	bad := map[rules.Ability]int{rules.Strength: 18, rules.Dexterity: 15, rules.Constitution: 14, rules.Intelligence: 12, rules.Wisdom: 9, rules.Charisma: 8}
	if err := rules.ValidateRolled(bad, rolled); err == nil || err.Error() != "rules: place the six scores you rolled" {
		t.Fatalf("a score not rolled = %v", err)
	}
	if !slices.Equal(rolled, []int{15, 12, 9, 14, 8, 17}) {
		t.Fatal("the rolled scores are left as they were")
	}
}

// slots reads the spell slots per spell level from a character's resources.
func slots(rs []rules.Resource) map[string]int {
	out := map[string]int{}
	for _, r := range rs {
		out[r.Key] = r.Max
		if r.Current != r.Max {
			out[r.Key+" current"] = r.Current
		}
	}
	return out
}

func TestResourcesAtALevel(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		class string
		level int
		want  map[string]int
	}{
		{"fighter", 5, map[string]int{"hit-dice": 5}},
		{"wizard", 3, map[string]int{"hit-dice": 3, "spell-slots-1": 4, "spell-slots-2": 2}},
		{"cleric", 20, map[string]int{"hit-dice": 20, "spell-slots-1": 4, "spell-slots-2": 3, "spell-slots-3": 3, "spell-slots-4": 3, "spell-slots-5": 3, "spell-slots-6": 2, "spell-slots-7": 2, "spell-slots-8": 1, "spell-slots-9": 1}},
		{"wizard", 25, map[string]int{"hit-dice": 25, "spell-slots-1": 4, "spell-slots-2": 3, "spell-slots-3": 3, "spell-slots-4": 3, "spell-slots-5": 3, "spell-slots-6": 2, "spell-slots-7": 2, "spell-slots-8": 1, "spell-slots-9": 1}},
		{"paladin", 2, map[string]int{"hit-dice": 2, "spell-slots-1": 2}},
		{"paladin", 5, map[string]int{"hit-dice": 5, "spell-slots-1": 4, "spell-slots-2": 2}},
		{"ranger", 20, map[string]int{"hit-dice": 20, "spell-slots-1": 4, "spell-slots-2": 3, "spell-slots-3": 3, "spell-slots-4": 3, "spell-slots-5": 2}},
		{"warlock", 1, map[string]int{"hit-dice": 1, "spell-slots-1": 1}},
		{"warlock", 2, map[string]int{"hit-dice": 2, "spell-slots-1": 2}},
		{"warlock", 3, map[string]int{"hit-dice": 3, "spell-slots-2": 2}},
		{"warlock", 10, map[string]int{"hit-dice": 10, "spell-slots-5": 2}},
		{"warlock", 11, map[string]int{"hit-dice": 11, "spell-slots-5": 3}},
		{"warlock", 16, map[string]int{"hit-dice": 16, "spell-slots-5": 3}},
		{"warlock", 17, map[string]int{"hit-dice": 17, "spell-slots-5": 4}},
		{"warlock", 9, map[string]int{"hit-dice": 9, "spell-slots-5": 2}},
		{"warlock", 8, map[string]int{"hit-dice": 8, "spell-slots-4": 2}},
	} {
		got := slots(rules.ResourcesAt(c.class, 8, c.level))
		if len(got) != len(c.want) {
			t.Errorf("%s %d = %v, want %v", c.class, c.level, got, c.want)
			continue
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Errorf("%s %d %s = %d, want %d", c.class, c.level, k, got[k], v)
			}
		}
	}
	labels := rules.ResourcesAt("wizard", 6, 9)
	if labels[len(labels)-1].Label != "Level 5 spell slots" || labels[0].Label != "Hit Dice (d6)" {
		t.Errorf("labels = %+v", labels)
	}
}

// Every full-caster level gains or keeps slots.
func TestFullCasterSlotsGrow(t *testing.T) {
	t.Parallel()
	total := func(level int) int {
		n := 0
		for _, r := range rules.ResourcesAt("sorcerer", 6, level) {
			if r.Key != "hit-dice" {
				n += r.Max
			}
		}
		return n
	}
	want := []int{2, 3, 6, 7, 9, 10, 11, 12, 14, 15, 16, 16, 17, 17, 18, 18, 19, 20, 21, 22}
	for level := 1; level <= 20; level++ {
		if got := total(level); got != want[level-1] {
			t.Errorf("level %d slots = %d, want %d", level, got, want[level-1])
		}
	}
}
