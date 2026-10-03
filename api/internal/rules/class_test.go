package rules_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

// lamplighter casts from a custom slot table: two 1st-level slots from level 1, a 2nd-level slot from
// level 4, and nothing higher.
func lamplighter() rules.Class {
	c := rules.Class{
		Slug: "hb-lamp", Name: "Lamplighter", Primary: []rules.Ability{rules.Charisma}, AnyPrimary: false,
		Casting:       rules.Spellcasting{Kind: rules.CustomSlots, Cantrips: [20]int{}, Prepared: [20]int{}, Slots: [20][9]int{}, Points: [20]int{}, Costs: [9]int{}, MaxSpell: [20]int{}, Spellbook: false, AfterRest: true},
		Proficiencies: rules.Proficiencies{Armor: []string{"Light armor"}, Weapons: []string{"Simple weapons"}},
		Skills:        3,
	}
	for l := range 20 {
		c.Casting.Cantrips[l] = 2
		c.Casting.Prepared[l] = 2 + l/2
		c.Casting.Slots[l][0] = 2
		if l >= 3 {
			c.Casting.Slots[l][1] = 1
		}
	}
	return c
}

// runeweaver casts with spell points: 4 points at level 1, growing by 2 a level, spells up to level
// (class level + 1) / 2, each costing its level plus one.
func runeweaver() rules.Class {
	c := rules.Class{
		Slug: "hb-rune", Name: "Runeweaver", Primary: []rules.Ability{rules.Intelligence, rules.Wisdom}, AnyPrimary: true,
		Casting:       rules.Spellcasting{Kind: rules.SpellPoints, Cantrips: [20]int{}, Prepared: [20]int{}, Slots: [20][9]int{}, Points: [20]int{}, Costs: [9]int{}, MaxSpell: [20]int{}, Spellbook: true, AfterRest: true},
		Proficiencies: rules.Proficiencies{Armor: nil, Weapons: []string{"Simple weapons"}},
		Skills:        2,
	}
	for l := range 20 {
		c.Casting.Points[l] = 4 + 2*l
		c.Casting.MaxSpell[l] = min((l+2)/2, 9)
		c.Casting.Prepared[l] = 3 + l
	}
	for i := range 9 {
		c.Casting.Costs[i] = i + 2
	}
	return c
}

func TestSRDClassesKeepTheirTables(t *testing.T) {
	t.Parallel()
	wizard, fighter, warlock, paladin := rules.SRD("wizard"), rules.SRD("fighter"), rules.SRD("warlock"), rules.SRD("paladin")
	if wizard.Name != "Wizard" || wizard.Casting.Kind != rules.FullCaster || !wizard.Casting.Spellbook || !wizard.Casting.AfterRest || wizard.CantripsAt(4) != 4 || wizard.PreparedAt(1) != 4 {
		t.Fatalf("wizard = %+v", wizard)
	}
	if fighter.Caster() || fighter.MaxSpellLevel(20) != 0 || !fighter.AnyPrimary || fighter.Skills != 2 || len(fighter.Proficiencies.Armor) != 4 {
		t.Fatalf("fighter = %+v", fighter)
	}
	if warlock.MaxSpellLevel(9) != 5 || warlock.SlotsAt(9)[4] != 2 || warlock.Casting.AfterRest || paladin.MaxSpellLevel(5) != 2 || paladin.SlotsAt(5)[1] != 2 {
		t.Fatalf("warlock %v, paladin %v", warlock.SlotsAt(9), paladin.SlotsAt(5))
	}
	if odd := rules.SRD("artificer"); odd.Caster() || odd.Skills != 2 || odd.Name != "Artificer" || len(odd.Primary) != 0 {
		t.Fatalf("an unknown class = %+v", odd)
	}
	if none := rules.SRD(""); none.Name != "" || none.Caster() {
		t.Fatalf("no class = %+v", none)
	}
}

func TestACustomSlotTable(t *testing.T) {
	t.Parallel()
	c := lamplighter()
	if !c.Caster() || c.MaxSpellLevel(3) != 1 || c.MaxSpellLevel(4) != 2 || c.CantripsAt(0) != 2 || c.PreparedAt(25) != 11 {
		t.Fatalf("lamplighter: max %d/%d", c.MaxSpellLevel(3), c.MaxSpellLevel(4))
	}
	if c.SlotsAt(3)[1] != 0 || c.SlotsAt(4)[1] != 1 {
		t.Fatalf("the 2nd-level slot comes at level 4: %v %v", c.SlotsAt(3), c.SlotsAt(4))
	}
	got := rules.ResourcesAt(c, 8, 4)
	want := []rules.Resource{
		{Key: "hit-dice", Label: "Hit Dice (d8)", Current: 4, Max: 4},
		{Key: "spell-slots-1", Label: "Level 1 spell slots", Current: 2, Max: 2},
		{Key: "spell-slots-2", Label: "Level 2 spell slots", Current: 1, Max: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resources = %+v", got)
	}
}

func TestSpellPoints(t *testing.T) {
	t.Parallel()
	c := runeweaver()
	if c.MaxSpellLevel(5) != 3 || c.SlotsAt(5) != [9]int{} {
		t.Fatalf("runeweaver at 5: max %d slots %v", c.MaxSpellLevel(5), c.SlotsAt(5))
	}
	got := rules.ResourcesAt(c, 6, 5)
	if len(got) != 2 || got[1] != (rules.Resource{Key: "spell-points", Label: "Spell points", Current: 12, Max: 12}) {
		t.Fatalf("resources = %+v", got)
	}
	if cost, err := rules.PointCost(c, 5, 3); err != nil || cost != 4 {
		t.Fatalf("a 3rd-level spell at level 5 = %d %v", cost, err)
	}
	for _, spell := range []int{0, 4} {
		if _, err := rules.PointCost(c, 5, spell); err == nil {
			t.Errorf("a level %d spell at class level 5 casts", spell)
		}
	}
	if _, err := rules.PointCost(lamplighter(), 5, 1); err == nil || !strings.Contains(err.Error(), "spell points") {
		t.Fatalf("a slot caster spending points: %v", err)
	}
	left, err := rules.SpendPoints(c, 5, 12, 3)
	if err != nil || left != 8 {
		t.Fatalf("spending 4 of 12 = %d %v", left, err)
	}
	if left, err := rules.SpendPoints(c, 5, 4, 3); err != nil || left != 0 {
		t.Fatalf("spending the last 4 = %d %v", left, err)
	}
	if _, err := rules.SpendPoints(c, 5, 3, 3); err == nil || !strings.Contains(err.Error(), "needs 4 spell points") {
		t.Fatalf("spending more than is left: %v", err)
	}
	if _, err := rules.SpendPoints(c, 5, 12, 9); err == nil {
		t.Fatal("a spell too high casts")
	}
}

func TestMulticlassingWithHomebrewSpellcasting(t *testing.T) {
	t.Parallel()
	wizard := rules.SRD("wizard")
	got := rules.MulticlassResources([]rules.ClassLevel{{Class: wizard, Level: 3, HitDie: 6}, {Class: lamplighter(), Level: 4, HitDie: 8}, {Class: runeweaver(), Level: 2, HitDie: 6}})
	keys := make([]string, 0, len(got))
	for _, r := range got {
		keys = append(keys, r.Key+"="+r.Label)
	}
	want := []string{
		"hit-dice=Hit Dice (4d8, 5d6)", "spell-slots-1=Level 1 spell slots", "spell-slots-2=Level 2 spell slots",
		"hb-lamp-slots-1=Lamplighter level 1 slots", "hb-lamp-slots-2=Lamplighter level 2 slots", "hb-rune-points=Runeweaver spell points",
	}
	if strings.Join(keys, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resources:\n%s", strings.Join(keys, "\n"))
	}
	if got[5].Max != 6 {
		t.Fatalf("runeweaver points at 2 = %d", got[5].Max)
	}
	scores := map[rules.Ability]int{rules.Strength: 8, rules.Charisma: 12, rules.Intelligence: 13, rules.Wisdom: 8}
	if unmet := rules.MulticlassUnmet(scores, []rules.Class{rules.SRD("wizard")}, lamplighter()); len(unmet) != 1 || unmet[0] != "Charisma 13+ (Lamplighter)" {
		t.Fatalf("into a Lamplighter: %v", unmet)
	}
	if unmet := rules.MulticlassUnmet(scores, []rules.Class{rules.SRD("wizard")}, runeweaver()); unmet != nil {
		t.Fatalf("a Runeweaver needs Intelligence or Wisdom: %v", unmet)
	}
	free := runeweaver()
	free.Primary = nil
	if unmet := rules.MulticlassUnmet(scores, nil, free); unmet != nil {
		t.Fatalf("a class without primary abilities asks nothing: %v", unmet)
	}
	if unmet := rules.MulticlassUnmet(map[rules.Ability]int{}, []rules.Class{wizard}, rules.SRD("fighter")); len(unmet) != 2 || unmet[1] != "Strength 13+ or Dexterity 13+ (Fighter)" {
		t.Fatalf("into a fighter with nothing: %v", unmet)
	}
}
