package dying_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dying"
)

func TestMassiveDamage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		before, damage, max int
		want                bool
	}{{6, 18, 12, true}, {6, 17, 12, false}, {12, 24, 12, true}, {0, 12, 12, true}, {0, 11, 12, false}} {
		if got := dying.MassiveDamage(c.before, c.damage, c.max); got != c.want {
			t.Errorf("%+v = %v", c, got)
		}
	}
}

func TestDeathSaves(t *testing.T) {
	t.Parallel()
	var s dying.State
	if !s.Rolls() {
		t.Fatal("a dying creature rolls")
	}
	s, _ = s.Save(10, 10)
	s, _ = s.Save(9, 9)
	s, _ = s.Save(7, 10)
	if s != (dying.State{Successes: 2, Failures: 1}) {
		t.Fatalf("after 10, 9 and a blessed 7 = %+v", s)
	}
	s, _ = s.Save(12, 12)
	if !s.Stable || s.Successes != 0 || s.Failures != 0 || s.Rolls() {
		t.Fatalf("three successes stabilise = %+v", s)
	}
	down := dying.State{Successes: 1, Failures: 1}
	if got, _ := down.Save(1, 5); !got.Dead || got.Failures != 3 {
		t.Fatalf("a 1 is two failures = %+v", got)
	}
	if got, woke := down.Save(20, 20); !woke || got != (dying.State{}) {
		t.Fatalf("a 20 wakes = %+v %v", got, woke)
	}
	if got, woke := (dying.State{Failures: 2}).Save(5, 9); woke || !got.Dead || got.Rolls() {
		t.Fatalf("a third failure kills = %+v", got)
	}
}

func TestDamageWhileDown(t *testing.T) {
	t.Parallel()
	stable := dying.State{Stable: true}
	if got := stable.Hurt(3, 12, false); got.Stable || got.Failures != 1 || !got.Rolls() {
		t.Fatalf("a hit wakes the dying again = %+v", got)
	}
	if got := (dying.State{Failures: 1}).Hurt(3, 12, true); !got.Dead {
		t.Fatalf("a critical counts twice = %+v", got)
	}
	if got := (dying.State{}).Hurt(12, 12, false); !got.Dead || got.Failures != 0 {
		t.Fatalf("damage of the maximum kills = %+v", got)
	}
	if got := (dying.State{}).Hurt(11, 12, false); got.Dead || got.Failures != 1 {
		t.Fatalf("just under the maximum = %+v", got)
	}
	if got := (dying.State{Failures: 2}).Stabilise(); got != (dying.State{Stable: true}) {
		t.Fatalf("stabilised = %+v", got)
	}
	if got := (dying.State{Dead: true}).Stabilise(); got.Stable {
		t.Fatalf("the dead stay dead = %+v", got)
	}
	if dying.StabiliseDC != 10 {
		t.Fatal("DC 10")
	}
}

func TestRevivalWindows(t *testing.T) {
	t.Parallel()
	died := dying.Died{Fight: "a", Round: 2, Day: 5}
	for _, c := range []struct {
		spell dying.Spell
		now   dying.Now
		want  bool
	}{
		{dying.Revivify, dying.Now{Fight: "a", Round: 12, Day: 5}, true},
		{dying.Revivify, dying.Now{Fight: "a", Round: 13, Day: 5}, false},
		{dying.Revivify, dying.Now{Fight: "b", Round: 1, Day: 5}, false},
		{dying.RaiseDead, dying.Now{Day: 15}, true},
		{dying.RaiseDead, dying.Now{Day: 16}, false},
		{dying.Resurrection, dying.Now{Day: 36505}, true},
		{dying.Resurrection, dying.Now{Day: 36506}, false},
		{"wish", dying.Now{Day: 5}, false},
	} {
		if got := dying.Revives(c.spell, died, c.now); got != c.want {
			t.Errorf("%s at %+v = %v", c.spell, c.now, got)
		}
	}
	if dying.Revives(dying.Revivify, dying.Died{Fight: "", Round: 0, Day: 5}, dying.Now{Fight: "", Round: 0, Day: 5}) {
		t.Error("Revivify needs the death within this fight")
	}
}
