package rules_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

func scores(s, d, c, i, w, ch int) map[rules.Ability]int {
	return map[rules.Ability]int{
		rules.Strength: s, rules.Dexterity: d, rules.Constitution: c,
		rules.Intelligence: i, rules.Wisdom: w, rules.Charisma: ch,
	}
}

func isViolation(t *testing.T, err error, want string) {
	t.Helper()
	var v *rules.ViolationError
	if !errors.As(err, &v) || !strings.Contains(v.Reason, want) || !strings.HasPrefix(err.Error(), "rules: ") {
		t.Errorf("err = %v, want violation containing %q", err, want)
	}
}

func TestModifierAndProficiency(t *testing.T) {
	t.Parallel()
	for score, want := range map[int]int{1: -5, 2: -4, 3: -4, 8: -1, 9: -1, 10: 0, 11: 0, 12: 1, 15: 2, 20: 5, 30: 10} {
		if got := rules.Modifier(score); got != want {
			t.Errorf("Modifier(%d) = %d, want %d", score, got, want)
		}
	}
	for level, want := range map[int]int{1: 2, 4: 2, 5: 3, 9: 4, 13: 5, 17: 6, 20: 6} {
		if got := rules.ProficiencyBonus(level); got != want {
			t.Errorf("ProficiencyBonus(%d) = %d, want %d", level, got, want)
		}
	}
	if len(rules.Abilities()) != 6 || !rules.Wisdom.Valid() || rules.Ability("luck").Valid() {
		t.Fatal("abilities")
	}
}

func TestPointBuyCost(t *testing.T) {
	t.Parallel()
	for score, want := range map[int]int{8: 0, 9: 1, 13: 5, 14: 7, 15: 9} {
		if got, ok := rules.PointBuyCost(score); !ok || got != want {
			t.Errorf("cost(%d) = %d %v", score, got, ok)
		}
	}
	for _, bad := range []int{7, 16} {
		if _, ok := rules.PointBuyCost(bad); ok {
			t.Errorf("%d buyable", bad)
		}
	}
}

func TestValidateBase(t *testing.T) {
	t.Parallel()
	ok := map[rules.Method]map[rules.Ability]int{
		rules.StandardArray: scores(8, 15, 14, 10, 13, 12),
		rules.PointBuy:      scores(15, 15, 15, 8, 8, 8),
		rules.Rolled:        scores(3, 18, 11, 12, 9, 17),
	}
	for m, s := range ok {
		if err := rules.ValidateBase(m, s); err != nil {
			t.Errorf("%s: %v", m, err)
		}
	}
	bad := []struct {
		method rules.Method
		scores map[rules.Ability]int
		want   string
	}{
		{rules.StandardArray, map[rules.Ability]int{rules.Strength: 15}, "all six"},
		{rules.StandardArray, map[rules.Ability]int{"luck": 1, rules.Dexterity: 1, rules.Constitution: 1, rules.Intelligence: 1, rules.Wisdom: 1, rules.Charisma: 1}, "not an ability"},
		{rules.StandardArray, scores(15, 15, 13, 12, 10, 8), "standard array"},
		{rules.PointBuy, scores(16, 8, 8, 8, 8, 8), "from 8 to 15"},
		{rules.PointBuy, scores(15, 15, 15, 15, 8, 8), "allows 27"},
		{rules.Rolled, scores(2, 10, 10, 10, 10, 10), "impossible"},
		{rules.Rolled, scores(19, 10, 10, 10, 10, 10), "impossible"},
		{"dream", scores(10, 10, 10, 10, 10, 10), "unknown method"},
	}
	for _, b := range bad {
		isViolation(t, rules.ValidateBase(b.method, b.scores), b.want)
	}
}

func TestOriginBonuses(t *testing.T) {
	t.Parallel()
	allowed := []rules.Ability{rules.Intelligence, rules.Wisdom, rules.Charisma}
	for _, ok := range []map[rules.Ability]int{
		{rules.Intelligence: 2, rules.Wisdom: 1},
		{rules.Intelligence: 1, rules.Wisdom: 1, rules.Charisma: 1},
	} {
		if err := rules.ValidateOriginBonuses(ok, allowed); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	if err := rules.ValidateOriginBonuses(map[rules.Ability]int{rules.Strength: 2, rules.Dexterity: 1}, nil); err != nil {
		t.Errorf("free choice refused: %v", err)
	}
	isViolation(t, rules.ValidateOriginBonuses(map[rules.Ability]int{"luck": 2}, nil), "not an ability")
	isViolation(t, rules.ValidateOriginBonuses(map[rules.Ability]int{rules.Strength: 2, rules.Wisdom: 1}, allowed), "does not raise strength")
	for _, bad := range []map[rules.Ability]int{
		{rules.Wisdom: 3},
		{rules.Wisdom: 2, rules.Charisma: 2},
		{rules.Wisdom: 1, rules.Charisma: 1},
		{},
	} {
		isViolation(t, rules.ValidateOriginBonuses(bad, allowed), "origin increases")
	}
}

func TestFinalScores(t *testing.T) {
	t.Parallel()
	got, err := rules.FinalScores(scores(15, 14, 13, 12, 10, 8), map[rules.Ability]int{rules.Strength: 2, rules.Constitution: 1})
	if err != nil || got[rules.Strength] != 17 || got[rules.Constitution] != 14 || got[rules.Charisma] != 8 {
		t.Fatalf("final = %v %v", got, err)
	}
	if top, err := rules.FinalScores(scores(18, 10, 10, 10, 10, 10), map[rules.Ability]int{rules.Strength: 2}); err != nil || top[rules.Strength] != 20 {
		t.Fatalf("20 refused: %v", err)
	}
	_, err = rules.FinalScores(scores(19, 10, 10, 10, 10, 10), map[rules.Ability]int{rules.Strength: 2})
	isViolation(t, err, "cannot exceed 20")
}

func TestSkills(t *testing.T) {
	t.Parallel()
	if len(rules.Skills()) != 18 || !rules.Skill("stealth").Valid() || rules.Skill("cooking").Valid() {
		t.Fatal("skills")
	}
	for class, want := range map[string]int{"rogue": 4, "bard": 3, "ranger": 3, "fighter": 2} {
		if got := rules.ClassSkillCount(class); got != want {
			t.Errorf("%s: %d", class, got)
		}
	}
	bg := []rules.Skill{"insight", "religion"}
	if err := rules.ValidateSkills("fighter", []rules.Skill{"athletics", "perception"}, bg); err != nil {
		t.Fatal(err)
	}
	isViolation(t, rules.ValidateSkills("fighter", []rules.Skill{"athletics"}, bg), "choose 2")
	isViolation(t, rules.ValidateSkills("fighter", []rules.Skill{"athletics", "cooking"}, bg), "not a skill")
	isViolation(t, rules.ValidateSkills("fighter", []rules.Skill{"athletics", "athletics"}, bg), "chosen twice")
	isViolation(t, rules.ValidateSkills("fighter", []rules.Skill{"athletics", "insight"}, bg), "already grants insight")
}

func TestArmorClassAndHP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dex    int
		armor  *rules.Armor
		shield int
		want   int
	}{
		{3, nil, 0, 13},
		{3, nil, 2, 15},
		{3, &rules.Armor{Base: 11, AddDex: true, DexCap: -1}, 0, 14},
		{3, &rules.Armor{Base: 14, AddDex: true, DexCap: 2}, 2, 18},
		{1, &rules.Armor{Base: 14, AddDex: true, DexCap: 2}, 0, 15},
		{3, &rules.Armor{Base: 18}, 0, 18},
		{3, &rules.Armor{Base: 12, AddDex: true, DexCap: 0}, 0, 12},
		{-1, &rules.Armor{Base: 12, AddDex: true, DexCap: 2}, 0, 11},
	}
	for _, c := range cases {
		if got := rules.ArmorClass(c.dex, c.armor, c.shield); got != c.want {
			t.Errorf("AC(%d, %+v, %d) = %d, want %d", c.dex, c.armor, c.shield, got, c.want)
		}
	}
	if rules.FirstLevelHP(10, 2) != 12 || rules.FirstLevelHP(6, -6) != 1 {
		t.Fatal("hp")
	}
}

func TestResourcesByCaster(t *testing.T) {
	t.Parallel()
	slots := func(class string) int {
		for _, r := range rules.FirstLevelResources(class, 8) {
			if r.Key == "spell-slots-1" {
				return r.Max
			}
		}
		return 0
	}
	cases := map[string]int{"wizard": 2, "paladin": 2, "ranger": 2, "warlock": 1, "fighter": 0}
	for class, want := range cases {
		if got := slots(class); got != want {
			t.Errorf("%s: %d slots", class, got)
		}
	}
	if n := len(rules.FirstLevelResources("fighter", 10)); n != 1 {
		t.Fatalf("fighter has %d resources", n)
	}
	if r := rules.FirstLevelResources("wizard", 6)[0]; r.Label != "Hit Dice (d6)" || r.Max != 1 {
		t.Fatalf("hit dice = %+v", r)
	}
}

func TestBuildSheet(t *testing.T) {
	t.Parallel()
	in := rules.SheetInput{
		Class: "fighter", Level: 1, HitDie: 10, Scores: scores(13, 14, 15, 8, 12, 10),
		SaveProfs: []rules.Ability{rules.Strength, rules.Constitution}, SkillProfs: []rules.Skill{"perception", "athletics"},
		Armor: &rules.Armor{Base: 18, DexCap: 0, StrengthRequired: 15, Stealth: true}, ShieldBonus: 2, SpeedFeet: 30,
	}
	s := rules.BuildSheet(in)
	if s.ProficiencyBonus != 2 || s.ArmorClass != 20 || s.Initiative != 2 || s.SpeedFeet != 20 || s.PassivePerception != 13 {
		t.Fatalf("sheet = %+v", s)
	}
	if s.Saves[0].Bonus != 3 || !s.Saves[0].Proficient || s.Saves[1].Bonus != 2 || s.Saves[2].Bonus != 4 {
		t.Fatalf("saves = %+v", s.Saves)
	}
	if len(s.Skills) != 18 || s.Skills[3].Skill != "athletics" || s.Skills[3].Bonus != 3 || len(s.Warnings) != 2 {
		t.Fatalf("skills/warnings = %+v %v", s.Skills[3], s.Warnings)
	}
	in.Armor = &rules.Armor{Base: 16, StrengthRequired: 13}
	if exact := rules.BuildSheet(in); exact.SpeedFeet != 30 || len(exact.Warnings) != 0 {
		t.Fatalf("strength exactly enough = %+v", exact)
	}
	in.Armor = nil
	if light := rules.BuildSheet(in); light.SpeedFeet != 30 || len(light.Warnings) != 0 || light.ArmorClass != 14 {
		t.Fatalf("unarmoured = %+v", light)
	}
}

func TestProficiencyByChallenge(t *testing.T) {
	t.Parallel()
	for cr, want := range map[float64]int{0: 2, 0.25: 2, 4: 2, 5: 3, 8: 3, 9: 4, 12: 4, 13: 5, 17: 6, 21: 7, 25: 8, 29: 9, 30: 9} {
		if got := rules.ProficiencyByChallenge(cr); got != want {
			t.Errorf("ProficiencyByChallenge(%v) = %d, want %d", cr, got, want)
		}
	}
}

func TestGroupChecks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		totals []int
		dc     int
		want   bool
	}{
		{[]int{15, 9}, 12, true},
		{[]int{15, 9, 8}, 12, false},
		{[]int{12, 12, 1, 1}, 12, true},
		{[]int{11}, 12, false},
		{nil, 1, false},
	} {
		if got := rules.GroupCheck(c.totals, c.dc); got != c.want {
			t.Errorf("%v against %d = %v", c.totals, c.dc, got)
		}
	}
	if rules.PassivePerception(3) != 13 {
		t.Error("passive Perception")
	}
}
