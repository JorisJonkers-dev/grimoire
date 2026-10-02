package vision_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/vision"
)

// The reference table in docs/rules/visibility.md, row by row: sight, darkvision, blindsight,
// tremorsense, truesight.
func TestTheReferenceTable(t *testing.T) {
	t.Parallel()
	const no, check, within, ground, sees = vision.No, vision.Check, vision.WithinRange, vision.OnGround, vision.Sees
	table := map[vision.Quality][5]vision.Outcome{
		vision.Hidden:    {check, check, within, ground, check},
		vision.Invisible: {no, no, within, ground, sees},
		vision.Disguised: {check, check, no, no, sees},
		vision.Illusory:  {check, check, sees, sees, sees},
		vision.Ethereal:  {no, no, no, no, sees},
		vision.Darkness:  {no, sees, within, ground, sees},
		vision.Heavy:     {no, no, within, ground, no},
		vision.Secret:    {check, check, no, no, no},
	}
	if len(vision.Qualities()) != len(table) || len(vision.Senses()) != 5 {
		t.Fatalf("qualities %v senses %v", vision.Qualities(), vision.Senses())
	}
	for q, row := range table {
		for i, s := range vision.Senses() {
			if got := vision.Against(q, s); got != row[i] {
				t.Errorf("%s against %s = %d, want %d", s, q, got, row[i])
			}
		}
	}
	if vision.Against(vision.Hidden, "smell") != no || vision.Against("blurry", vision.Truesight) != no {
		t.Error("unknown senses and qualities get nowhere")
	}
}

func TestPerceiving(t *testing.T) {
	t.Parallel()
	eyes := []vision.Perceiver{{Sense: vision.Sight}}
	bat := []vision.Perceiver{{Sense: vision.Sight}, {Sense: vision.Blindsight, RangeFt: 30}}
	mole := []vision.Perceiver{{Sense: vision.Tremorsense, RangeFt: 60}}
	seer := []vision.Perceiver{{Sense: vision.Truesight, RangeFt: 120}}
	for _, c := range []struct {
		name   string
		senses []vision.Perceiver
		target vision.Target
		want   vision.Result
	}{
		{"plain sight", eyes, vision.Target{DistanceFt: 500}, vision.Result{Seen: true, TrueForm: true}},
		{"invisible to eyes", eyes, vision.Target{Qualities: []vision.Quality{vision.Invisible}}, vision.Result{}},
		{"invisible within blindsight", bat, vision.Target{Qualities: []vision.Quality{vision.Invisible}, DistanceFt: 30}, vision.Result{Seen: true, TrueForm: true}},
		{"invisible beyond blindsight", bat, vision.Target{Qualities: []vision.Quality{vision.Invisible}, DistanceFt: 35}, vision.Result{}},
		{"invisible flier above tremorsense", mole, vision.Target{Qualities: []vision.Quality{vision.Invisible}, DistanceFt: 10}, vision.Result{}},
		{"invisible walker on tremorsense", mole, vision.Target{Qualities: []vision.Quality{vision.Invisible}, DistanceFt: 60, OnGround: true}, vision.Result{Seen: true, TrueForm: true}},
		{"walker past tremorsense", mole, vision.Target{Qualities: []vision.Quality{vision.Invisible}, DistanceFt: 65, OnGround: true}, vision.Result{}},
		{"hidden until spotted", eyes, vision.Target{Qualities: []vision.Quality{vision.Hidden}}, vision.Result{}},
		{"hidden and spotted", eyes, vision.Target{Qualities: []vision.Quality{vision.Hidden}, Beaten: []vision.Quality{vision.Hidden}}, vision.Result{Seen: true, TrueForm: true}},
		{"spotted, but the check was for something else", eyes, vision.Target{Qualities: []vision.Quality{vision.Hidden}, Beaten: []vision.Quality{vision.Secret}}, vision.Result{}},
		{"disguised: seen, not for what it is", eyes, vision.Target{Qualities: []vision.Quality{vision.Disguised}}, vision.Result{Seen: true}},
		{"disguised and seen through", eyes, vision.Target{Qualities: []vision.Quality{vision.Disguised}, Beaten: []vision.Quality{vision.Disguised}}, vision.Result{Seen: true, TrueForm: true}},
		{"truesight sees the true form", seer, vision.Target{Qualities: []vision.Quality{vision.Disguised, vision.Invisible}, DistanceFt: 120}, vision.Result{Seen: true, TrueForm: true}},
		{"truesight out of range", seer, vision.Target{Qualities: []vision.Quality{vision.Ethereal}, DistanceFt: 121}, vision.Result{}},
		{"an unseen disguise hides nothing true", eyes, vision.Target{Qualities: []vision.Quality{vision.Disguised, vision.Invisible}, Beaten: []vision.Quality{vision.Disguised}}, vision.Result{}},
		{"a blindsight with no range reaches nothing", []vision.Perceiver{{Sense: vision.Blindsight}}, vision.Target{Qualities: []vision.Quality{vision.Invisible}}, vision.Result{}},
		{"truesight does not beat stealth", seer, vision.Target{Qualities: []vision.Quality{vision.Hidden}, DistanceFt: 5}, vision.Result{}},
		{"a beaten check needs the sense in range", seer, vision.Target{Qualities: []vision.Quality{vision.Hidden}, Beaten: []vision.Quality{vision.Hidden}, DistanceFt: 200}, vision.Result{}},
	} {
		if got := vision.Perceive(c.senses, c.target); got != c.want {
			t.Errorf("%s = %+v", c.name, got)
		}
	}
}

func TestPierces(t *testing.T) {
	t.Parallel()
	for s, want := range map[vision.Sense]string{
		vision.Blindsight:  "Blindsight 30 ft: Hidden, Invisible, Illusory, Darkness, Heavy obscurement",
		vision.Truesight:   "Truesight 30 ft: Invisible, Disguised, Illusory, Ethereal, Darkness",
		vision.Darkvision:  "Darkvision 30 ft: Darkness",
		vision.Sight:       "Sight 30 ft: nothing without a check",
		vision.Tremorsense: "Tremorsense 30 ft: Hidden, Invisible, Illusory, Darkness, Heavy obscurement",
	} {
		if got := vision.Pierces(s, 30); got != want {
			t.Errorf("%s = %q", s, got)
		}
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()
	got := vision.Describe(map[vision.Sense]int{vision.Truesight: 120, vision.Darkvision: 60, vision.Sight: 10})
	want := []string{
		"Sight: a check against Hidden, Disguised, Illusory, Secret; never Invisible, Ethereal, Darkness, Heavy obscurement",
		"Darkvision 60 ft: Darkness",
		"Truesight 120 ft: Invisible, Disguised, Illusory, Ethereal, Darkness",
	}
	if len(got) != len(want) {
		t.Fatalf("describe = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q", i, got[i])
		}
	}
	if got := vision.Describe(nil); len(got) != 1 {
		t.Errorf("eyes only = %v", got)
	}
}
