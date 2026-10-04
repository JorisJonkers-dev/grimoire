package difficulty_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/difficulty"
)

func TestPresetsAreStoryStandardAndHard(t *testing.T) {
	t.Parallel()
	all := difficulty.Presets()
	if len(all) != 3 || all[0].Slug != difficulty.Story || all[1].Slug != difficulty.Standard || all[2].Slug != difficulty.Hard {
		t.Fatalf("presets = %+v", all)
	}
	for _, p := range all {
		if p.Name == "" || p.Description == "" || !difficulty.Valid(p.Slug) || difficulty.Of(p.Slug) != p {
			t.Errorf("preset %+v", p)
		}
	}
	if difficulty.Valid("") || difficulty.Valid("nightmare") {
		t.Error("an unknown preset is valid")
	}
	// A Campaign whose preset is not known plays by the rules as written.
	if got := difficulty.Of("nightmare"); got.Slug != difficulty.Standard {
		t.Errorf("unknown preset = %+v", got)
	}
}

func TestAPresetChangesEnemiesHitPointsAndAttacks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		slug        string
		hp, want    int
		toHit       int
		changesPlay bool
	}{
		{difficulty.Story, 20, 15, -2, true},
		{difficulty.Story, 7, 5, -2, true},
		{difficulty.Story, 1, 1, -2, true},
		{difficulty.Story, 0, 0, -2, true},
		{difficulty.Standard, 20, 20, 0, false},
		{difficulty.Standard, 7, 7, 0, false},
		{difficulty.Hard, 20, 25, 2, true},
		{difficulty.Hard, 7, 8, 2, true},
		{difficulty.Hard, 1, 1, 2, true},
	}
	for _, c := range cases {
		p := difficulty.Of(c.slug)
		if got := p.HitPoints(c.hp); got != c.want {
			t.Errorf("%s: HitPoints(%d) = %d, want %d", c.slug, c.hp, got, c.want)
		}
		if p.ToHit != c.toHit || p.Changes() != c.changesPlay {
			t.Errorf("%s: to hit %d, changes %t", c.slug, p.ToHit, p.Changes())
		}
	}
}
