package rules_test

import (
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

func TestHitPointChanges(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ hp, temp, amount, wantHP, wantTemp int }{
		{20, 0, 5, 15, 0},
		{20, 4, 3, 20, 1},
		{20, 4, 4, 20, 0},
		{20, 4, 10, 14, 0},
		{5, 0, 9, 0, 0},
		{5, 2, -3, 5, 2},
	} {
		if hp, temp := rules.TakeDamage(c.hp, c.temp, c.amount); hp != c.wantHP || temp != c.wantTemp {
			t.Errorf("damage %d to %d+%d = %d+%d, want %d+%d", c.amount, c.hp, c.temp, hp, temp, c.wantHP, c.wantTemp)
		}
	}
	for _, c := range []struct{ hp, maxHP, amount, want int }{{5, 12, 4, 9}, {10, 12, 4, 12}, {12, 12, 1, 12}, {5, 12, -2, 5}} {
		if got := rules.Heal(c.hp, c.maxHP, c.amount); got != c.want {
			t.Errorf("heal %d to %d/%d = %d, want %d", c.amount, c.hp, c.maxHP, got, c.want)
		}
	}
	if rules.GainTempHP(3, 5) != 5 || rules.GainTempHP(6, 5) != 6 || rules.GainTempHP(0, 0) != 0 {
		t.Error("temporary hit points keep the higher amount")
	}
}

func TestClassProficiencies(t *testing.T) {
	t.Parallel()
	heavy := []string{"Light armor", "Medium armor", "Heavy armor", "Shields"}
	medium := []string{"Light armor", "Medium armor", "Shields"}
	both := []string{"Simple weapons", "Martial weapons"}
	simple := []string{"Simple weapons"}
	for class, want := range map[string]rules.Proficiencies{
		"barbarian": {Armor: medium, Weapons: both}, "fighter": {Armor: heavy, Weapons: both}, "paladin": {Armor: heavy, Weapons: both},
		"ranger": {Armor: medium, Weapons: both}, "cleric": {Armor: medium, Weapons: simple}, "druid": {Armor: medium, Weapons: simple},
		"bard": {Armor: []string{"Light armor"}, Weapons: simple}, "warlock": {Armor: []string{"Light armor"}, Weapons: simple},
		"rogue":    {Armor: []string{"Light armor"}, Weapons: []string{"Simple weapons", "Martial weapons with the Finesse or Light property"}},
		"monk":     {Armor: nil, Weapons: []string{"Simple weapons", "Martial weapons with the Light property"}},
		"sorcerer": {Armor: nil, Weapons: simple}, "wizard": {Armor: nil, Weapons: simple}, "artificer": {Armor: nil, Weapons: nil},
	} {
		got := rules.ClassProficiencies(class)
		if !slices.Equal(got.Armor, want.Armor) || !slices.Equal(got.Weapons, want.Weapons) {
			t.Errorf("%s = %+v, want %+v", class, got, want)
		}
	}
}

func TestExpertiseDoublesProficiency(t *testing.T) {
	t.Parallel()
	s := rules.BuildSheet(rules.SheetInput{
		Class: "rogue", Level: 5, HitDie: 8, Scores: scores(10, 16, 12, 10, 14, 10),
		SkillProfs: []rules.Skill{"stealth", "perception"}, Expertise: []rules.Skill{"stealth", "arcana"},
	})
	for _, k := range s.Skills {
		switch k.Skill {
		case "stealth":
			if !k.Expertise || k.Bonus != 3+3+3 {
				t.Errorf("stealth = %+v", k)
			}
		case "perception":
			if k.Expertise || k.Bonus != 2+3 || s.PassivePerception != 15 {
				t.Errorf("perception = %+v", k)
			}
		case "arcana":
			if k.Expertise || k.Bonus != 0 {
				t.Errorf("expertise without proficiency = %+v", k)
			}
		}
	}
}
