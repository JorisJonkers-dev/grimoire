package reactions_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/reactions"
)

func TestDecide(t *testing.T) {
	t.Parallel()
	fresh, hurt, half := reactions.Situation{TargetHP: 10, TargetHPMax: 10}, reactions.Situation{TargetHP: 4, TargetHPMax: 10}, reactions.Situation{TargetHP: 5, TargetHPMax: 10}
	for _, c := range []struct {
		s    reactions.Setting
		at   reactions.Situation
		want reactions.Decision
	}{
		{reactions.Setting{}, fresh, reactions.Decision{Offer: true, Auto: false}},
		{reactions.Setting{Mode: reactions.Ask}, fresh, reactions.Decision{Offer: true, Auto: false}},
		{reactions.Setting{Mode: reactions.Never}, hurt, reactions.Decision{Offer: false, Auto: false}},
		{reactions.Setting{Mode: reactions.Always}, fresh, reactions.Decision{Offer: true, Auto: true}},
		{reactions.Setting{Mode: reactions.Always, Condition: reactions.TargetBloodied}, fresh, reactions.Decision{Offer: true, Auto: false}},
		{reactions.Setting{Mode: reactions.Always, Condition: reactions.TargetBloodied}, hurt, reactions.Decision{Offer: true, Auto: true}},
		{reactions.Setting{Mode: reactions.Always, Condition: reactions.TargetBloodied}, half, reactions.Decision{Offer: true, Auto: true}},
		{reactions.Setting{Mode: reactions.Always, Condition: reactions.TargetBloodied}, reactions.Situation{}, reactions.Decision{Offer: true, Auto: false}},
		{reactions.Setting{Mode: reactions.Always, Condition: "moonlit"}, hurt, reactions.Decision{Offer: true, Auto: false}},
	} {
		if got := reactions.Decide(c.s, c.at); got != c.want {
			t.Errorf("%+v at %+v = %+v, want %+v", c.s, c.at, got, c.want)
		}
	}
}

func TestValid(t *testing.T) {
	t.Parallel()
	for s, want := range map[reactions.Setting]bool{
		{Mode: reactions.Ask}: true, {Mode: reactions.Always, Condition: reactions.TargetBloodied}: true, {Mode: reactions.Never}: true,
		{Mode: "sometimes"}: false, {Mode: reactions.Always, Condition: "moonlit"}: false,
	} {
		if reactions.Valid(s) != want {
			t.Errorf("%+v valid = %v", s, !want)
		}
	}
}
