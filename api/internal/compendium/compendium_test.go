package compendium_test

import (
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
)

func TestCursorRoundTrip(t *testing.T) {
	t.Parallel()
	c := compendium.Cursor{Name: "Fire Bolt", Slug: "fire-bolt"}
	got, ok := compendium.DecodeCursor(c.Encode())
	if !ok || got != c {
		t.Fatalf("got %+v %v", got, ok)
	}
}

func TestDecodeCursorRejectsForeignTokens(t *testing.T) {
	t.Parallel()
	for _, tok := range []string{"!!!", "e30", "bm90LWpzb24"} {
		if _, ok := compendium.DecodeCursor(tok); ok {
			t.Errorf("%q accepted", tok)
		}
	}
}

func TestFindMentionsMatchesWholeWordsInOrder(t *testing.T) {
	t.Parallel()
	conds := []compendium.Condition{{Name: "Prone"}, {Name: "Blinded"}, {Name: "Invisible"}, {Name: "Charmed"}}
	got := compendium.FindMentions(conds, "The target is knocked prone and BLINDED.", "It becomes charmed-ish? No: it is charmed.")
	names := []string{}
	for _, c := range got {
		names = append(names, c.Name)
	}
	if len(names) != 3 || names[0] != "Blinded" || names[1] != "Charmed" || names[2] != "Prone" {
		t.Fatalf("mentions = %v", names)
	}
	if len(compendium.FindMentions(conds, "invisibleness")) != 0 {
		t.Fatal("partial words must not match")
	}
}
