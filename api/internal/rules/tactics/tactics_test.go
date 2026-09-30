package tactics_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/tactics"
)

func TestStyleFollowsIntelligenceUnlessOverridden(t *testing.T) {
	t.Parallel()
	cases := []struct {
		override string
		int      int
		want     tactics.Style
	}{
		{tactics.FromIntelligence, 11, tactics.Simple},
		{tactics.FromIntelligence, 12, tactics.Cunning},
		{"", 3, tactics.Simple},
		{tactics.OverrideSimple, 18, tactics.Simple},
		{tactics.OverrideCunning, 3, tactics.Cunning},
		{tactics.OverrideOff, 12, tactics.Off},
	}
	for _, c := range cases {
		if got := tactics.For(c.override, c.int); got != c.want {
			t.Errorf("%q with Int %d = %v, want %v", c.override, c.int, got, c.want)
		}
	}
}

var (
	sword = tactics.Attack{ReachFt: 5}
	bow   = tactics.Attack{RangeFt: 80, LongRangeFt: 320}
	spear = tactics.Attack{ReachFt: 5, RangeFt: 20, LongRangeFt: 60}
)

func TestSimpleGoesForTheNearestEnemy(t *testing.T) {
	t.Parallel()
	targets := []tactics.Target{{ID: "b", DistanceFt: 10, RangedDamageSeen: 20}, {ID: "c", DistanceFt: 5}, {ID: "a", DistanceFt: 5}}
	s, ok := tactics.Suggest(tactics.Simple, []tactics.Attack{bow, sword}, targets)
	if !ok || s.Target.ID != "a" || s.Attack != 1 || s.Reason != tactics.Nearest {
		t.Fatalf("simple = %+v %v", s, ok)
	}
	s, _ = tactics.Suggest(tactics.Simple, []tactics.Attack{sword, bow}, []tactics.Target{{ID: "x", DistanceFt: 60}})
	if s.Attack != 1 || s.Reason != tactics.Nearest {
		t.Fatalf("out of reach, the bow = %+v", s)
	}
	s, _ = tactics.Suggest(tactics.Simple, []tactics.Attack{sword, spear}, []tactics.Target{{ID: "x", DistanceFt: 40}})
	if s.Attack != 1 {
		t.Fatalf("long range beats walking = %+v", s)
	}
	s, _ = tactics.Suggest(tactics.Simple, []tactics.Attack{spear, bow}, []tactics.Target{{ID: "x", DistanceFt: 70}})
	if s.Attack != 1 {
		t.Fatalf("normal range beats long range = %+v", s)
	}
	s, ok = tactics.Suggest(tactics.Simple, []tactics.Attack{sword}, []tactics.Target{{ID: "x", DistanceFt: 30}})
	if !ok || s.Attack != -1 || s.Reason != tactics.Approach || s.Target.ID != "x" {
		t.Fatalf("nothing reaches = %+v", s)
	}
}

func TestCunningShootsWhoeverItSawShootingHardest(t *testing.T) {
	t.Parallel()
	targets := []tactics.Target{
		{ID: "near", DistanceFt: 5},
		{ID: "archer", DistanceFt: 60, RangedDamageSeen: 9},
		{ID: "mage", DistanceFt: 30, RangedDamageSeen: 14},
		{ID: "far", DistanceFt: 400, RangedDamageSeen: 30},
		{ID: "twin", DistanceFt: 40, RangedDamageSeen: 14},
	}
	s, ok := tactics.Suggest(tactics.Cunning, []tactics.Attack{sword, bow}, targets)
	if !ok || s.Target.ID != "mage" || s.Attack != 1 || s.Reason != tactics.Sniper {
		t.Fatalf("cunning = %+v", s)
	}
	s, _ = tactics.Suggest(tactics.Cunning, []tactics.Attack{sword}, targets)
	if s.Target.ID != "near" || s.Reason != tactics.Nearest {
		t.Fatalf("no ranged attack, nearest = %+v", s)
	}
	s, _ = tactics.Suggest(tactics.Cunning, []tactics.Attack{sword, bow}, []tactics.Target{{ID: "near", DistanceFt: 5}, {ID: "quiet", DistanceFt: 50}})
	if s.Target.ID != "near" || s.Attack != 0 || s.Reason != tactics.Nearest {
		t.Fatalf("nobody seen shooting, nearest = %+v", s)
	}
	s, _ = tactics.Suggest(tactics.Cunning, []tactics.Attack{spear}, []tactics.Target{{ID: "close", DistanceFt: 5, RangedDamageSeen: 3}})
	if s.Target.ID != "close" || s.Reason != tactics.Sniper || s.Attack != 0 {
		t.Fatalf("a thrown spear counts, reach does not = %+v", s)
	}
}

func TestRangeEdges(t *testing.T) {
	t.Parallel()
	short := tactics.Attack{RangeFt: 30, LongRangeFt: 120}
	cases := []struct {
		attacks []tactics.Attack
		dist    int
		want    int
	}{
		{[]tactics.Attack{short, bow}, 30, 0},
		{[]tactics.Attack{{RangeFt: 10, LongRangeFt: 40}}, 40, 0},
		{[]tactics.Attack{{RangeFt: 10, LongRangeFt: 40}}, 45, -1},
	}
	for _, c := range cases {
		if s, _ := tactics.Suggest(tactics.Simple, c.attacks, []tactics.Target{{ID: "x", DistanceFt: c.dist}}); s.Attack != c.want {
			t.Errorf("%+v at %d ft = %d, want %d", c.attacks, c.dist, s.Attack, c.want)
		}
	}
	s, _ := tactics.Suggest(tactics.Cunning, []tactics.Attack{sword}, []tactics.Target{{ID: "x", DistanceFt: 0, RangedDamageSeen: 5}})
	if s.Reason != tactics.Nearest || s.Attack != 0 {
		t.Fatalf("a sword is not a ranged attack even at 0 ft = %+v", s)
	}
}

func TestNoSuggestion(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		s       tactics.Style
		attacks []tactics.Attack
		targets []tactics.Target
	}{
		"off":        {tactics.Off, []tactics.Attack{sword}, []tactics.Target{{ID: "x"}}},
		"no enemies": {tactics.Simple, []tactics.Attack{sword}, nil},
		"no attacks": {tactics.Cunning, nil, []tactics.Target{{ID: "x"}}},
	} {
		if s, ok := tactics.Suggest(c.s, c.attacks, c.targets); ok || s.Attack != -1 {
			t.Errorf("%s = %+v", name, s)
		}
	}
}
