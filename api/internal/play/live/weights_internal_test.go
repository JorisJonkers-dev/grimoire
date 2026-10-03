package live

import (
	"slices"
	"testing"

	"github.com/google/uuid"

	play "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
)

// An Encounter Table's entries weigh what the DM gave them, but for a Faction's own: those weigh by
// how the Faction regards the party. A forced check leaves Nothing out; an unknown Faction changes nothing.
func TestEntryWeightsFollowStanding(t *testing.T) {
	t.Parallel()
	watch, cult, gone := uuid.New(), uuid.New(), uuid.New()
	table := prep.Table{Entries: []prep.Entry{
		{Weight: 10, Kind: prep.EntryEncounter, Label: "Watch patrol", FactionID: &watch},
		{Weight: 10, Kind: prep.EntryEncounter, Label: "Wolves"},
		{Weight: 5, Kind: prep.EntryNothing, Label: "Nothing"},
		{Weight: 10, Kind: prep.EntryEncounter, Label: "Cultists", FactionID: &cult},
		{Weight: 10, Kind: prep.EntryEncounter, Label: "Strangers", FactionID: &gone},
	}}
	standings := func(watchScore, cultScore int) []play.Standing {
		// A Personal Standing plays no part: an encounter meets the party.
		return []play.Standing{{Faction: watch, Name: "Watch", Score: watchScore, Personal: map[uuid.UUID]int{uuid.New(): 100}}, {Faction: cult, Name: "Cult", Score: cultScore}}
	}
	for _, c := range []struct {
		name  string
		mode  string
		watch int
		cult  int
		want  []int
	}{
		{"Neutral with both", prep.ModeNormal, 0, 0, []int{10, 10, 5, 10, 10}},
		{"Hostile with the Watch, Allied with the Cult", prep.ModeNormal, -80, 80, []int{20, 10, 5, 2, 10}},
		{"Friendly with the Watch, Unfriendly with the Cult", prep.ModeNormal, 30, -30, []int{5, 10, 5, 15, 10}},
		{"forced, Hostile with the Watch", prep.ModeForce, -80, 0, []int{20, 10, 0, 10, 10}},
	} {
		if got := entryWeights(table, c.mode, standings(c.watch, c.cult)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	if got := entryWeights(table, prep.ModeNormal, nil); !slices.Equal(got, []int{10, 10, 5, 10, 10}) {
		t.Errorf("with no Standing read: %v", got)
	}
}
