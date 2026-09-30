package attack_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

func TestModes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		adv, dis int
		want     attack.Mode
		d20      string
	}{
		{0, 0, attack.Normal, "1d20"},
		{1, 0, attack.Advantage, "2d20kh1"},
		{2, 0, attack.Advantage, "2d20kh1"},
		{0, 1, attack.Disadvantage, "2d20kl1"},
		{1, 1, attack.Normal, "1d20"},
		{3, 1, attack.Normal, "1d20"},
	}
	for _, c := range cases {
		m := attack.ModeOf(c.adv, c.dis)
		if m != c.want || attack.D20(m) != c.d20 || (m.String() == "normal") != (c.d20 == "1d20") {
			t.Errorf("%d/%d = %v %s", c.adv, c.dis, m, attack.D20(m))
		}
	}
}

func TestModeNames(t *testing.T) {
	t.Parallel()
	for m, want := range map[attack.Mode]string{attack.Normal: "normal", attack.Advantage: "advantage", attack.Disadvantage: "disadvantage"} {
		if m.String() != want {
			t.Errorf("%d = %s", m, m.String())
		}
	}
}

func TestHitChance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		bonus, ac int
		m         attack.Mode
		want      int
	}{
		{5, 15, attack.Normal, 55},
		{5, 15, attack.Advantage, 80},
		{5, 15, attack.Disadvantage, 30},
		{0, 30, attack.Normal, 5},
		{30, 5, attack.Normal, 95},
		{4, 15, attack.Normal, 50},
		{5, 14, attack.Normal, 60},
		{0, 30, attack.Advantage, 10},
		{30, 5, attack.Disadvantage, 90},
	}
	for _, c := range cases {
		if got := attack.HitChance(c.bonus, c.ac, c.m); got != c.want {
			t.Errorf("+%d vs %d (%v) = %d, want %d", c.bonus, c.ac, c.m, got, c.want)
		}
	}
}

func TestHitChanceWithExtraDice(t *testing.T) {
	t.Parallel()
	bless, _ := dice.Parse("1d4")
	bane, _ := dice.Parse("-1d4")
	two, _ := dice.Parse("2d4")
	cases := []struct {
		extra     dice.Spec
		bonus, ac int
		m         attack.Mode
		want      int
	}{
		{dice.Spec{}, 5, 15, attack.Normal, 55},
		{bless, 5, 15, attack.Normal, 68},
		{bane, 5, 15, attack.Normal, 43},
		{bless, 5, 15, attack.Advantage, 89},
		{two, 0, 20, attack.Normal, 30},
		{bless, 30, 5, attack.Normal, 95},
	}
	for _, c := range cases {
		if got := attack.HitChanceDice(c.bonus, c.extra, c.ac, c.m); got != c.want {
			t.Errorf("%s +%d vs %d (%v) = %d, want %d", c.extra.String(), c.bonus, c.ac, c.m, got, c.want)
		}
	}
}

func TestOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		natural, bonus, ac int
		want               attack.Result
	}{
		{20, -5, 30, attack.Critical},
		{1, 30, 5, attack.Miss},
		{10, 5, 15, attack.Hit},
		{9, 5, 15, attack.Miss},
		{19, 0, 19, attack.Hit},
		{2, 0, 3, attack.Miss},
		{2, 1, 3, attack.Hit},
	}
	for _, c := range cases {
		if got := attack.Outcome(c.natural, c.bonus, c.ac); got != c.want {
			t.Errorf("%d+%d vs %d = %v, want %v", c.natural, c.bonus, c.ac, got, c.want)
		}
	}
}

func TestBands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		dist, reach, rng, long int
		want                   attack.Band
	}{
		{5, 5, 0, 0, attack.InReach},
		{10, 5, 0, 0, attack.OutOfRange},
		{10, 10, 0, 0, attack.InReach},
		{20, 5, 20, 60, attack.InRange},
		{25, 5, 20, 60, attack.LongRange},
		{60, 5, 20, 60, attack.LongRange},
		{65, 5, 20, 60, attack.OutOfRange},
		{5, 5, 20, 60, attack.InReach},
		{6, 5, 20, 60, attack.InRange},
	}
	for _, c := range cases {
		if got := attack.BandOf(c.dist, c.reach, c.rng, c.long); got != c.want {
			t.Errorf("%d ft (%d/%d/%d) = %v, want %v", c.dist, c.reach, c.rng, c.long, got, c.want)
		}
	}
}

func TestDamage(t *testing.T) {
	t.Parallel()
	spec, _ := dice.Parse("2d6+1d4")
	if got := attack.CriticalDice(spec).String(); got != "4d6+2d4" {
		t.Fatalf("critical = %s", got)
	}
	if spec.String() != "2d6+1d4" {
		t.Fatal("critical changed its input")
	}
	for _, c := range []struct {
		notation    string
		bonus       int
		least, most int
	}{{"2d6+1d4", 3, 6, 19}, {"1d4", -3, 0, 1}, {"1d6-1d4", 0, 0, 5}, {"1d8", 0, 1, 8}} {
		s, _ := dice.Parse(c.notation)
		if l, m := attack.DamageRange(s, c.bonus); l != c.least || m != c.most {
			t.Errorf("%s%+d = %d–%d, want %d–%d", c.notation, c.bonus, l, m, c.least, c.most)
		}
	}
	if l, m := attack.DamageRange(dice.Spec{}, 4); l != 4 || m != 4 {
		t.Fatalf("flat = %d–%d", l, m)
	}
}

func TestWeaponAttack(t *testing.T) {
	t.Parallel()
	cases := []struct {
		w                    attack.Weapon
		str, dex             int
		toHit, damage, reach int
	}{
		{attack.Weapon{}, 3, 1, 5, 3, 5},
		{attack.Weapon{Finesse: true}, 1, 3, 5, 3, 5},
		{attack.Weapon{Finesse: true}, 3, 1, 5, 3, 5},
		{attack.Weapon{Ammunition: true}, 3, 1, 3, 1, 5},
		{attack.Weapon{Reach: true}, 2, 0, 4, 2, 10},
	}
	for _, c := range cases {
		h, d, r := attack.WeaponAttack(c.w, c.str, c.dex, 2)
		if h != c.toHit || d != c.damage || r != c.reach {
			t.Errorf("%+v = %d %d %d", c.w, h, d, r)
		}
	}
}
