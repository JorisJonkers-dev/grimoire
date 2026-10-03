package variants_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
)

// The catalogue names every Rule Variant once, each with a first option that is how the rules play
// without it, and says which ones Grimoire applies itself.
func TestTheCatalogueOfRuleVariants(t *testing.T) {
	t.Parallel()
	list := variants.Catalogue()
	slugs := []string{}
	for _, v := range list {
		slugs = append(slugs, v.Slug)
		if v.Name == "" || v.Description == "" || len(v.Options) < 2 {
			t.Errorf("%s is not fully described: %+v", v.Slug, v)
		}
		for _, o := range v.Options {
			if o.Value == "" || o.Label == "" {
				t.Errorf("%s has an unnamed option: %+v", v.Slug, o)
			}
		}
		if got := (variants.Set{}).Get(v.Slug); got != v.Options[0].Value {
			t.Errorf("%s unset plays as %q, want its first option %q", v.Slug, got, v.Options[0].Value)
		}
	}
	want := []string{
		variants.Flanking, variants.CriticalFumble, variants.CriticalHits, variants.Rests, variants.ShortRestCap, variants.MassiveDamage,
		variants.HiddenDeathSaves, variants.HealingSurge, variants.SlowNaturalHealing, variants.Morale, variants.Encumbrance,
	}
	if !slices.Equal(slugs, want) {
		t.Fatalf("catalogue = %v", slugs)
	}
	automated := []string{}
	for _, v := range list {
		if v.Automated {
			automated = append(automated, v.Slug)
		}
	}
	if !slices.Equal(automated, []string{variants.Flanking, variants.CriticalHits, variants.Rests, variants.ShortRestCap, variants.SlowNaturalHealing}) {
		t.Fatalf("automated = %v", automated)
	}
	// Changing the list a caller was given changes nobody else's.
	list[0].Slug, list[0].Options[0].Value = "x", "x"
	if again := variants.Catalogue(); again[0].Slug != variants.Flanking || again[0].Options[0].Value != variants.Off {
		t.Fatalf("the catalogue is shared: %+v", again[0])
	}
}

// A Campaign's set holds only what a variant can be; anything else plays as the rules do without it.
func TestASetOfRuleVariants(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		slug, value string
		valid       bool
	}{
		{variants.Flanking, variants.On, true},
		{variants.Flanking, variants.Off, true},
		{variants.Flanking, "yes", false},
		{variants.Flanking, "", false},
		{variants.CriticalHits, variants.CritMaxDice, true},
		{variants.CriticalHits, variants.On, false},
		{variants.Rests, variants.RestsGritty, true},
		{variants.Rests, variants.RestsEpic, true},
		{variants.Rests, "heroic", false},
		{variants.ShortRestCap, "2", true},
		{variants.ShortRestCap, "4", false},
		{variants.Encumbrance, variants.EncumbranceVariant, true},
		{variants.Encumbrance, variants.Off, true},
		{"spell-points", variants.On, false},
		{"", variants.On, false},
	} {
		if got := variants.Valid(c.slug, c.value); got != c.valid {
			t.Errorf("Valid(%q, %q) = %v", c.slug, c.value, got)
		}
	}
	set := variants.Set{variants.Flanking: variants.On, variants.Rests: "heroic", variants.Morale: variants.Off, "spell-points": variants.On}
	if !set.On(variants.Flanking) || set.On(variants.Morale) || set.On(variants.MassiveDamage) || set.On("spell-points") {
		t.Errorf("On = %v %v %v %v", set.On(variants.Flanking), set.On(variants.Morale), set.On(variants.MassiveDamage), set.On("spell-points"))
	}
	if got := set.Get(variants.Rests); got != variants.RestsStandard {
		t.Errorf("a value the variant cannot be plays as %q", got)
	}
	if got := set.Get("spell-points"); got != "" {
		t.Errorf("an unknown variant has the value %q", got)
	}
	if got := variants.Set(nil).Get(variants.CriticalHits); got != variants.CritDoubleDice {
		t.Errorf("no set at all plays critical hits as %q", got)
	}
}

// Flanking: an attacker next to its target has it flanked when an ally stands on the hex straight
// across the target.
func TestFlanking(t *testing.T) {
	t.Parallel()
	target := hex.Coord{Q: 2, R: 2}
	for _, c := range []struct {
		name           string
		attacker, ally hex.Coord
		want           bool
	}{
		{"east and west", hex.Coord{Q: 3, R: 2}, hex.Coord{Q: 1, R: 2}, true},
		{"west and east", hex.Coord{Q: 1, R: 2}, hex.Coord{Q: 3, R: 2}, true},
		{"north-east and south-west", hex.Coord{Q: 3, R: 1}, hex.Coord{Q: 1, R: 3}, true},
		{"south-east and north-west", hex.Coord{Q: 2, R: 3}, hex.Coord{Q: 2, R: 1}, true},
		{"side by side", hex.Coord{Q: 3, R: 2}, hex.Coord{Q: 3, R: 1}, false},
		{"one hex round from across", hex.Coord{Q: 3, R: 2}, hex.Coord{Q: 1, R: 3}, false},
		{"an ally across, two hexes out", hex.Coord{Q: 3, R: 2}, hex.Coord{Q: 0, R: 2}, false},
		{"an attacker two hexes out, in line", hex.Coord{Q: 4, R: 2}, hex.Coord{Q: 1, R: 2}, false},
		{"an attacker two hexes out, its mirror hex held", hex.Coord{Q: 4, R: 2}, hex.Coord{Q: 0, R: 2}, false},
		{"the ally on the attacker's hex", hex.Coord{Q: 3, R: 2}, hex.Coord{Q: 3, R: 2}, false},
		{"the attacker on the target's hex", target, target, false},
	} {
		if got := variants.Flanks(c.attacker, c.ally, target); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// A critical fumble is a natural 1 on the attack's d20, and nothing else.
func TestCriticalFumble(t *testing.T) {
	t.Parallel()
	for natural, want := range map[int]bool{0: false, 1: true, 2: false, 19: false, 20: false} {
		if got := variants.Fumbles(natural); got != want {
			t.Errorf("a natural %d fumbles: %v", natural, got)
		}
	}
}

// Critical hits roll every damage die twice, or with the variant roll them once and add the most the
// dice could show.
func TestCriticalHitDice(t *testing.T) {
	t.Parallel()
	spec := dice.Spec{Groups: []dice.Group{{Count: 2, Faces: 6, Sign: 1}, {Count: 1, Faces: 8, Sign: 1}, {Count: 1, Faces: 4, Sign: -1}}}
	doubled, extra := variants.CriticalDice(variants.CritDoubleDice, spec)
	if want := []dice.Group{{Count: 4, Faces: 6, Sign: 1}, {Count: 2, Faces: 8, Sign: 1}, {Count: 2, Faces: 4, Sign: -1}}; !reflect.DeepEqual(doubled.Groups, want) || extra != 0 {
		t.Errorf("double dice = %+v + %d", doubled.Groups, extra)
	}
	// 2d6 + 1d8 at their most is 20; dice taken off the total are rolled once and add nothing.
	once, extra := variants.CriticalDice(variants.CritMaxDice, spec)
	if !reflect.DeepEqual(once.Groups, spec.Groups) || extra != 20 {
		t.Errorf("max dice = %+v + %d", once.Groups, extra)
	}
	if spec.Groups[0].Count != 2 {
		t.Errorf("the dice given were changed: %+v", spec.Groups)
	}
	if again, extra := variants.CriticalDice("anything else", spec); !reflect.DeepEqual(again.Groups, doubled.Groups) || extra != 0 {
		t.Errorf("an unknown mode = %+v + %d", again.Groups, extra)
	}
	if none, extra := variants.CriticalDice(variants.CritMaxDice, dice.Spec{}); len(none.Groups) != 0 || extra != 0 {
		t.Errorf("no dice = %+v + %d", none.Groups, extra)
	}
}

// How long a rest takes: an hour and eight; gritty, eight hours and a week; epic, five minutes and an hour.
func TestRestLengths(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode        string
		short, long int
	}{
		{variants.RestsStandard, 60, 480}, {variants.RestsGritty, 480, 7 * 24 * 60}, {variants.RestsEpic, 5, 60}, {"", 60, 480}, {"heroic", 60, 480},
	} {
		if short, long := variants.RestMinutes(c.mode, false), variants.RestMinutes(c.mode, true); short != c.short || long != c.long {
			t.Errorf("%q: %d and %d minutes", c.mode, short, long)
		}
	}
}

// A cap on Short Rests counts the ones taken since the last Long Rest.
func TestTheShortRestCap(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		limit string
		taken int
		want  bool
	}{
		{variants.NoCap, 0, true},
		{variants.NoCap, 50, true},
		{"1", 0, true},
		{"1", 1, false},
		{"2", 1, true},
		{"2", 2, false},
		{"3", 2, true},
		{"3", 3, false},
		{"3", 9, false},
		{"", 9, true},
		{"many", 9, true},
	} {
		if got := variants.ShortRestAllowed(c.limit, c.taken); got != c.want {
			t.Errorf("cap %q with %d taken: %v", c.limit, c.taken, got)
		}
	}
}

// Massive damage: one blow of half a creature's hit point maximum or more, when it has one.
func TestMassiveDamage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		damage, hpMax int
		want          bool
	}{
		{0, 20, false}, {9, 20, false}, {10, 20, true}, {11, 20, true}, {40, 20, true}, {10, 21, false}, {11, 21, true}, {1, 1, true}, {0, 0, false}, {5, 0, false}, {-10, 20, false},
	} {
		if got := variants.SystemShock(c.damage, c.hpMax); got != c.want {
			t.Errorf("%d damage against %d hit points: %v", c.damage, c.hpMax, got)
		}
	}
	if variants.SystemShockDC != 15 {
		t.Errorf("the save is DC %d", variants.SystemShockDC)
	}
}

// Hidden death saves are the DM's to see; without the variant everyone sees them.
func TestHiddenDeathSaves(t *testing.T) {
	t.Parallel()
	hidden, open := variants.Set{variants.HiddenDeathSaves: variants.On}, variants.Set{}
	if !open.ShowsDeathSaves(false) || !open.ShowsDeathSaves(true) || hidden.ShowsDeathSaves(false) || !hidden.ShowsDeathSaves(true) {
		t.Errorf("shown = open %v %v, hidden %v %v", open.ShowsDeathSaves(false), open.ShowsDeathSaves(true), hidden.ShowsDeathSaves(false), hidden.ShowsDeathSaves(true))
	}
}

// A healing surge spends up to half a Character's Hit Dice, never fewer than one and never more than it has left.
func TestHealingSurge(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ level, left, want int }{
		{1, 1, 1}, {2, 2, 1}, {3, 3, 1}, {4, 4, 2}, {11, 11, 5}, {20, 20, 10}, {20, 3, 3}, {1, 0, 0}, {8, 0, 0}, {0, 0, 0}, {6, -1, 0},
	} {
		if got := variants.SurgeDice(c.level, c.left); got != c.want {
			t.Errorf("level %d with %d left: %d", c.level, c.left, got)
		}
	}
}

// Slow natural healing: a Long Rest gives back no hit points.
func TestSlowNaturalHealing(t *testing.T) {
	t.Parallel()
	if got := (variants.Set{}).LongRestHP(7, 30); got != 30 {
		t.Errorf("a Long Rest heals to %d", got)
	}
	if got := (variants.Set{variants.SlowNaturalHealing: variants.On}).LongRestHP(7, 30); got != 7 {
		t.Errorf("with slow natural healing a Long Rest heals to %d", got)
	}
}

// Morale is checked when a creature is down to half its hit points or fewer, or half its side has fallen.
func TestMorale(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name                    string
		hp, hpMax, fallen, side int
		want                    bool
	}{
		{"fresh among the standing", 20, 20, 0, 4, false},
		{"just over half", 11, 20, 0, 4, false},
		{"at half", 10, 20, 0, 4, true},
		{"under half", 3, 20, 0, 4, true},
		{"half of an odd maximum", 10, 21, 0, 4, true},
		{"over half of an odd maximum", 11, 21, 0, 4, false},
		{"one of four fallen", 20, 20, 1, 4, false},
		{"two of four fallen", 20, 20, 2, 4, true},
		{"two of five fallen", 20, 20, 2, 5, false},
		{"three of five fallen", 20, 20, 3, 5, true},
		{"alone and fresh", 20, 20, 0, 1, false},
		{"a side of none", 20, 20, 0, 0, false},
		{"a creature with no hit points to speak of", 0, 0, 0, 4, false},
	} {
		if got := variants.MoraleCheck(c.hp, c.hpMax, c.fallen, c.side); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if variants.MoraleDC != 10 {
		t.Errorf("the save is DC %d", variants.MoraleDC)
	}
}

// Encumbrance: the standard rule slows a creature to 5 feet past 15 pounds a point of Strength; the
// variant takes 10 feet off past 5 a point and 20 past 10; off, a load never slows anyone.
func TestEncumbrance(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode   string
		weight float64
		want   int
	}{
		{variants.EncumbranceStandard, 150, 30},
		{variants.EncumbranceStandard, 150.5, 5},
		{variants.EncumbranceStandard, 300, 5},
		{variants.EncumbranceStandard, 300.5, 0},
		{variants.EncumbranceVariant, 50, 30},
		{variants.EncumbranceVariant, 50.5, 20},
		{variants.EncumbranceVariant, 100, 20},
		{variants.EncumbranceVariant, 100.5, 10},
		{variants.EncumbranceVariant, 150, 10},
		{variants.EncumbranceVariant, 150.5, 5},
		{variants.EncumbranceVariant, 300, 5},
		{variants.EncumbranceVariant, 300.5, 0},
		{variants.Off, 150.5, 30},
		{variants.Off, 9000, 30},
		{"", 150.5, 5},
		{"heavy", 300.5, 0},
	} {
		if got := variants.BurdenedSpeed(c.mode, 30, c.weight, 10); got != c.want {
			t.Errorf("%q carrying %.1f lb at Strength 10: %d ft", c.mode, c.weight, got)
		}
	}
	// A slow creature is never sped up by a load, nor slowed below nothing.
	if got := variants.BurdenedSpeed(variants.EncumbranceStandard, 0, 200, 10); got != 0 {
		t.Errorf("a creature with no speed, encumbered: %d ft", got)
	}
	if got := variants.BurdenedSpeed(variants.EncumbranceVariant, 15, 120, 10); got != 0 {
		t.Errorf("15 ft less 20: %d ft", got)
	}
	if got := variants.BurdenedSpeed(variants.EncumbranceVariant, 15, 60, 10); got != 5 {
		t.Errorf("15 ft less 10: %d ft", got)
	}
	if got := variants.BurdenedSpeed(variants.EncumbranceVariant, 3, 200, 10); got != 3 {
		t.Errorf("3 ft, past capacity: %d ft", got)
	}
}
