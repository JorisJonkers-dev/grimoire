package actions_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
)

func TestTheStandardActions(t *testing.T) {
	t.Parallel()
	all := actions.Standard()
	if len(all) != 11 || all[0].Action != actions.Dash || all[len(all)-1].Action != actions.Utilize {
		t.Fatalf("standard = %+v", all)
	}
	for _, a := range all {
		if a.Name == "" || a.Summary == "" {
			t.Errorf("%s needs a name and a summary", a.Action)
		}
	}
	for a, check := range map[actions.Action][2]string{
		actions.Search: {"wisdom", "perception"}, actions.Hide: {"dexterity", "stealth"}, actions.Study: {"intelligence", ""},
		actions.Influence: {"charisma", ""}, actions.Dash: {"", ""}, actions.Magic: {"", ""},
	} {
		info, ok := actions.Find(a)
		if !ok || info.Check != check[0] || info.Skill != check[1] {
			t.Errorf("%s = %+v", a, info)
		}
	}
	if _, ok := actions.Find("juggle"); ok {
		t.Error("juggling is no action")
	}
}

func TestGrappleAndShove(t *testing.T) {
	t.Parallel()
	if got := actions.UnarmedDC(3, 2); got != 13 {
		t.Errorf("DC = %d", got)
	}
	if a, b := actions.Resist(map[string]int{"strength": 1, "dexterity": 4}); a != "dexterity" || b != 4 {
		t.Errorf("a nimble creature resists with Dexterity = %s %d", a, b)
	}
	if a, b := actions.Resist(map[string]int{"strength": 2, "dexterity": 2}); a != "strength" || b != 2 {
		t.Errorf("a tie resists with Strength = %s %d", a, b)
	}
	if a, b := actions.Resist(nil); a != "strength" || b != 0 {
		t.Errorf("no saves = %s %d", a, b)
	}
	if got := actions.DragCostFt(15); got != 30 {
		t.Errorf("dragging costs double = %d", got)
	}
}

func TestReadiedTriggers(t *testing.T) {
	t.Parallel()
	anyone := actions.Trigger{Kind: actions.EntersReach, Who: ""}
	for _, c := range []struct {
		step  actions.Step
		reach int
		want  bool
	}{
		{actions.Step{Mover: "goblin", BeforeFt: 10, AfterFt: 5}, 5, true},
		{actions.Step{Mover: "goblin", BeforeFt: 5, AfterFt: 5}, 5, false},
		{actions.Step{Mover: "goblin", BeforeFt: 15, AfterFt: 10}, 5, false},
		{actions.Step{Mover: "goblin", BeforeFt: 15, AfterFt: 10}, 10, true},
		{actions.Step{Mover: "goblin", BeforeFt: 5, AfterFt: 10}, 5, false},
	} {
		if got := anyone.Fires(c.step, c.reach); got != c.want {
			t.Errorf("%+v at reach %d = %v", c.step, c.reach, got)
		}
	}
	only := actions.Trigger{Kind: actions.EntersReach, Who: "boss"}
	if only.Fires(actions.Step{Mover: "goblin", BeforeFt: 10, AfterFt: 5}, 5) || !only.Fires(actions.Step{Mover: "boss", BeforeFt: 10, AfterFt: 5}, 5) {
		t.Error("a trigger on one creature ignores the rest")
	}
	if (actions.Trigger{Kind: "sneezes", Who: ""}).Fires(actions.Step{Mover: "goblin", BeforeFt: 10, AfterFt: 5}, 5) {
		t.Error("an unknown trigger never fires")
	}
}
