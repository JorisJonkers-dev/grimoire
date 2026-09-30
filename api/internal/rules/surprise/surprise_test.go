package surprise_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surprise"
)

func TestHiddenCreaturesSetTheirBestPassiveStealth(t *testing.T) {
	for _, c := range []struct {
		stealth []int
		dc      int
	}{{nil, 10}, {[]int{-1}, 9}, {[]int{2, 6, 4}, 16}, {[]int{-3, -2}, 8}} {
		if got := surprise.DC(c.stealth); got != c.dc {
			t.Errorf("DC(%v) = %d, want %d", c.stealth, got, c.dc)
		}
	}
	if surprise.Passive(3) != 13 {
		t.Error("passive 3")
	}
}

func TestPerceptionMeetsTheDCAndTheSurprisedRollLow(t *testing.T) {
	if !surprise.Notices(14, 14) || surprise.Notices(13, 14) || !surprise.Notices(15, 14) {
		t.Error("notices")
	}
	if surprise.Initiative(true) != "2d20kl1" || surprise.Initiative(false) != "1d20" {
		t.Error("initiative dice")
	}
}
