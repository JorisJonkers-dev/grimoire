package domain_test

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
)

func TestImportsTakeWhatTheyCanAndNoteTheRest(t *testing.T) {
	t.Parallel()
	many := map[string]any{}
	for i := range domain.MaxFields + 1 {
		many[fmt.Sprintf("f%03d", i)] = "x"
	}
	entries, cols, manual := domain.PlanImport([]domain.Incoming{
		{
			Key: "hag", Kind: "creature", Name: " Bog Hag ", Fields: map[string]any{"HP": "52", "AC": 17.0, " ": "x", "Lore": strings.Repeat("a", 4001)},
			Parts: []map[string]any{{"type": "effect"}, {}},
		},
		{Key: "cart", Kind: "vehicle", Name: "Cart"},
		{Key: "blank", Kind: "npc", Name: "  "},
		{Key: "hag", Kind: "npc", Name: "Twin"},
		{Key: "lots", Kind: "item", Name: "Lots", Fields: many},
	}, []domain.ExportedCollection{
		{Name: "Fey", Entries: []string{"hag", "cart", "lots"}},
		{Name: " ", Entries: []string{"hag"}},
	})
	if len(entries) != 2 || entries[0].Name != "Bog Hag" || !maps.Equal(entries[0].Fields, domain.Fields{"HP": "52"}) || len(entries[1].Fields) != domain.MaxFields {
		t.Fatalf("entries = %+v", entries)
	}
	if len(cols) != 1 || cols[0].Name != "Fey" || strings.Join(cols[0].Entries, ",") != "hag,lots" {
		t.Fatalf("collections = %+v", cols)
	}
	var reasons []string
	for _, m := range manual {
		reasons = append(reasons, m.Where+": "+m.Reason)
	}
	got := strings.Join(reasons, "\n")
	for _, want := range []string{
		`entries[0] "Bog Hag" fields.AC: only text values are kept`,
		`entries[0] "Bog Hag" fields. : a field needs a name`,
		`entries[0] "Bog Hag" fields.Lore: a field needs`,
		`entries[0] "Bog Hag" parts[0]: effect parts are not supported yet`,
		`entries[0] "Bog Hag" parts[1]: untyped parts are not supported yet`,
		`entries[1] "Cart": the Library keeps no vehicle entries`,
		`entries[2] "": a name needs 1 to 80 characters`,
		`entries[3] "Twin": another entry already has the key hag`,
		`entries[4] "Lots" fields.f100: an entry keeps at most 100 fields`,
		`collections[0] "Fey": no imported entry has the key cart`,
		`collections[1] " ": a Collection needs a name`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if len(manual) != 11 {
		t.Errorf("%d manual parts:\n%s", len(manual), got)
	}
}
