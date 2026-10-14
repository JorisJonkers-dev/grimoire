package snapshot_test

import (
	"math"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

func refreshPair() (snapshot.Snapshot, snapshot.Snapshot) {
	doc := snapshot.Document{Key: "srd-2024", Title: "SRD 5.2", License: "CC-BY-4.0", Attribution: "a"}
	spell := func(slug, desc string) snapshot.Spell {
		return snapshot.Spell{Document: "srd-2024", Slug: slug, Name: slug, School: "evocation", Description: desc, Classes: []string{}, DamageTypes: []string{}, Scaling: []snapshot.Scaling{}}
	}
	item := func(slug, desc string) snapshot.Item {
		return snapshot.Item{Entry: snapshot.Entry{Document: "srd-2024", Slug: slug, Name: slug, Description: desc}}
	}
	before := snapshot.Snapshot{
		Documents: []snapshot.Document{doc},
		Spells:    []snapshot.Spell{spell("moonbeam", "Cut"), spell("fire-bolt", "Hot.")},
		Items:     []snapshot.Item{item("wand-of-web", ""), item("rope", "Strong.")},
		Monsters: []snapshot.Monster{{
			Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, ArmorClass: 15,
			Actions: []snapshot.Action{{Name: "Scimitar", Type: "action", Attacks: []snapshot.Attack{}}},
		}},
	}
	after := snapshot.Snapshot{
		Documents: before.Documents,
		Spells:    []snapshot.Spell{spell("fire-bolt", "Hot."), spell("moonbeam", "Whole.")},
		Items:     []snapshot.Item{item("rope", ""), item("wand-of-fear", ""), item("cloak-of-invisibility", "")},
		Monsters: []snapshot.Monster{{
			Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, ArmorClass: 15,
			Actions: []snapshot.Action{{Name: "Scimitar", Type: "action", Attacks: []snapshot.Attack{}, UsageLimit: &snapshot.UsageLimit{Kind: "per_day", Param: 1}}},
		}},
	}
	after.Spells[1].HigherLevel = "More."
	return before, after
}

func TestCompareReportsWhatARefreshChanged(t *testing.T) {
	t.Parallel()
	before, after := refreshPair()
	c, err := snapshot.Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	refs := func(rs []snapshot.Ref) string {
		out := []string{}
		for _, r := range rs {
			out = append(out, r.Kind+" "+r.Document+"/"+r.Slug)
		}
		return strings.Join(out, ", ")
	}
	if got := refs(c.Added); got != "item srd-2024/cloak-of-invisibility, item srd-2024/wand-of-fear" {
		t.Errorf("added = %s", got)
	}
	if got := refs(c.Removed); got != "item srd-2024/wand-of-web" {
		t.Errorf("removed = %s", got)
	}
	if len(c.Changed) != 3 || c.Changed[0].Kind != "spell" || c.Changed[0].Slug != "moonbeam" ||
		strings.Join(c.Changed[0].Fields, ",") != "description,higherLevel" ||
		c.Changed[1].Slug != "rope" || strings.Join(c.Changed[1].Fields, ",") != "description" ||
		c.Changed[2].Slug != "goblin" || strings.Join(c.Changed[2].Fields, ",") != "actions" {
		t.Errorf("changed = %+v", c.Changed)
	}
	counts := map[string]snapshot.Count{}
	for _, n := range c.Counts {
		counts[n.Kind] = n
	}
	if len(c.Counts) != len(snapshot.Kinds()) || counts["item"] != (snapshot.Count{Kind: "item", Before: 2, After: 3}) ||
		counts["spell"] != (snapshot.Count{Kind: "spell", Before: 2, After: 2}) || counts["class"].After != 0 {
		t.Errorf("counts = %+v", c.Counts)
	}
	text := c.Text()
	for _, want := range []string{
		"1 removed", "2 added", "3 changed", "item: 2 -> 3", "+ item srd-2024/wand-of-fear", "- item srd-2024/wand-of-web",
		"~ spell srd-2024/moonbeam: description, higherLevel", "~ monster srd-2024/goblin: actions",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
}

func TestCompareOfEqualSnapshotsIsEmpty(t *testing.T) {
	t.Parallel()
	before, _ := refreshPair()
	c, err := snapshot.Compare(before, before)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Added)+len(c.Removed)+len(c.Changed) != 0 || !strings.Contains(c.Text(), "0 added, 0 removed, 0 changed") {
		t.Fatalf("changes = %+v\n%s", c, c.Text())
	}
}

func TestCompareSurfacesUnencodableSnapshots(t *testing.T) {
	t.Parallel()
	before, after := refreshPair()
	after.Monsters[0].ChallengeRating = math.NaN()
	if _, err := snapshot.Compare(before, after); err == nil {
		t.Fatal("after accepted")
	}
	if _, err := snapshot.Compare(after, before); err == nil {
		t.Fatal("before accepted")
	}
}

func TestFromTreeIsStrict(t *testing.T) {
	t.Parallel()
	if _, err := snapshot.FromTree(map[string]any{"spells": []any{map[string]any{"slug": "x", "colour": "red"}}}); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := snapshot.CanonicalEntry("spell", map[string]any{"slug": "x", "colour": "red"}); err == nil {
		t.Fatal("unknown entry field accepted")
	}
	if _, err := snapshot.CanonicalEntry("wand", map[string]any{"slug": "x"}); err == nil {
		t.Fatal("unknown kind accepted")
	}
	if _, err := snapshot.CanonicalEntry("item", map[string]any{"slug": "x", "costGp": math.NaN()}); err == nil {
		t.Fatal("unencodable entry accepted")
	}
	got, err := snapshot.CanonicalEntry("item", map[string]any{"slug": "x", "name": "X"})
	if err != nil || got["magic"] != false || got["costGp"] != 0.0 || got["slug"] != "x" {
		t.Fatalf("canonical = %v, %v", got, err)
	}
}
