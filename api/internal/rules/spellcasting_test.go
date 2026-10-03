package rules_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

func TestWhoPreparesAndKeepsASpellbook(t *testing.T) {
	t.Parallel()
	for class, want := range map[string][2]bool{
		"cleric": {true, false}, "druid": {true, false}, "paladin": {true, false}, "wizard": {true, true},
		"bard": {false, false}, "sorcerer": {false, false}, "warlock": {false, false}, "ranger": {false, false}, "fighter": {false, false},
	} {
		if got := [2]bool{rules.SRD(class).Casting.AfterRest, rules.SRD(class).Casting.Spellbook}; got != want {
			t.Errorf("%s = %v, want %v", class, got, want)
		}
	}
	for level, want := range map[int]int{1: 6, 2: 8, 5: 14, 20: 44} {
		if got := rules.SpellbookAllotment(level); got != want {
			t.Errorf("spellbook at %d = %d, want %d", level, got, want)
		}
	}
}

func TestCopyingASpellCostsGoldAndTime(t *testing.T) {
	t.Parallel()
	for level, want := range map[int][2]int{1: {50, 120}, 3: {150, 360}, 9: {450, 1080}} {
		gp, minutes := rules.CopyCost(level)
		if [2]int{gp, minutes} != want {
			t.Errorf("level %d = %d gp %d min, want %v", level, gp, minutes, want)
		}
	}
}

func TestRitualsTakeTenMinutesMore(t *testing.T) {
	t.Parallel()
	for text, want := range map[string]int{
		"action": 10, "bonus-action": 10, "reaction": 10, "1minute": 11, "10minutes": 20, "1hour": 70, "8hours": 490, "": 10, "soon": 10,
	} {
		if got := rules.RitualMinutes(text); got != want {
			t.Errorf("%q = %d, want %d", text, got, want)
		}
	}
}

func TestPreparationLimits(t *testing.T) {
	t.Parallel()
	ok := []struct {
		class          string
		limit          int
		previous, next []string
	}{
		{"cleric", 3, []string{"bless"}, []string{"aid", "cure-wounds", "guiding-bolt"}},
		{"cleric", 3, nil, nil},
		{"bard", 2, []string{"charm-person", "sleep"}, []string{"charm-person", "healing-word"}},
		{"bard", 3, []string{"charm-person"}, []string{"healing-word", "sleep", "heroism"}},
		{"sorcerer", 2, nil, []string{"shield", "sleep"}},
	}
	for _, c := range ok {
		if err := rules.CheckPreparation(rules.SRD(c.class), c.limit, c.previous, c.next); err != nil {
			t.Errorf("%s %v -> %v: %v", c.class, c.previous, c.next, err)
		}
	}
	bad := []struct {
		class          string
		limit          int
		previous, next []string
	}{
		{"cleric", 2, nil, []string{"aid", "bless", "bane"}},
		{"cleric", 3, nil, []string{"aid", "aid"}},
		{"bard", 2, []string{"charm-person", "sleep"}, []string{"healing-word", "heroism"}},
	}
	for _, c := range bad {
		if err := rules.CheckPreparation(rules.SRD(c.class), c.limit, c.previous, c.next); err == nil {
			t.Errorf("%s %v -> %v accepted", c.class, c.previous, c.next)
		}
	}
}

func TestTheGameClockRollsOverAtMidnight(t *testing.T) {
	t.Parallel()
	cases := []struct{ day, minute, by, wantDay, wantMinute int }{
		{3, 600, 120, 3, 720}, {3, 1430, 20, 4, 10}, {0, 0, 1440 * 2, 2, 0}, {5, 100, 0, 5, 100}, {5, 100, -50, 5, 100},
	}
	for _, c := range cases {
		if d, m := rules.AdvanceClock(c.day, c.minute, c.by); d != c.wantDay || m != c.wantMinute {
			t.Errorf("%+v = day %d minute %d", c, d, m)
		}
	}
}
