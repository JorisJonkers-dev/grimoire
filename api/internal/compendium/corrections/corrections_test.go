package corrections_test

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/corrections"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

func doc() snapshot.Document {
	return snapshot.Document{Key: "srd-2024", Title: "SRD 5.2", License: "CC-BY-4.0", Attribution: "a"}
}

func upstream() snapshot.Snapshot {
	return snapshot.Snapshot{
		Documents: []snapshot.Document{doc()},
		Spells: []snapshot.Spell{{
			Document: "srd-2024", Slug: "moonbeam", Name: "Moonbeam", Level: 2, School: "evocation", Description: "A silvery beam",
			Classes: []string{}, DamageTypes: []string{}, Scaling: []snapshot.Scaling{},
		}},
		Classes: []snapshot.Class{{
			Entry: snapshot.Entry{Document: "srd-2024", Slug: "druid", Name: "Druid"}, SavingThrows: []string{},
			Features: []snapshot.ClassFeature{{Slug: "natures-ward", Name: "Nature's Ward", Description: "Wrong.", Levels: []int{10}}},
		}},
		Items: []snapshot.Item{{Entry: snapshot.Entry{Document: "srd-2024", Slug: "potion-of-healing", Name: "Potion of Healing"}, Category: "potion"}},
		Monsters: []snapshot.Monster{{
			Entry: snapshot.Entry{Document: "srd-2024", Slug: "black-pudding", Name: "Black Pudding"},
			Actions: []snapshot.Action{
				{Name: "Dissolving Pseudopod", Type: "action", Attacks: []snapshot.Attack{}},
				{Name: "Parry", Description: "Splits.", Type: "reaction", Attacks: []snapshot.Attack{}},
			},
		}},
	}
}

func load(t *testing.T) []corrections.Correction {
	t.Helper()
	cs, err := corrections.Load(os.DirFS("testdata/corrections"))
	if err != nil {
		t.Fatal(err)
	}
	return cs
}

func TestLoadReadsOneFilePerEntry(t *testing.T) {
	t.Parallel()
	cs := load(t)
	files := []string{}
	for _, c := range cs {
		files = append(files, c.File)
	}
	want := "class/druid.yaml item/cloak-of-invisibility.yaml item/potion-of-healing.yaml monster/black-pudding.yaml spell/moonbeam.yaml"
	if strings.Join(files, " ") != want {
		t.Fatalf("files = %v", files)
	}
	potion := cs[2]
	if potion.Kind != "item" || potion.Entry != "potion-of-healing" || potion.Document != "srd-2024" ||
		potion.Source != (corrections.Source{Document: "SRD 5.2.1", Page: 229}) || potion.Upstream != nil || len(potion.Fields) != 2 ||
		potion.Fields[0].Field != "magic" || potion.Fields[0].Value != true || potion.Fields[1].Value != "common" {
		t.Fatalf("potion = %+v", potion)
	}
	moon := cs[4]
	if moon.Fields[0].Value != "Fixture: the whole description, as the SRD prints it." || moon.Upstream == nil || moon.Upstream.Filed != "" {
		t.Fatalf("moonbeam = %+v", moon)
	}
	if cs[1].Add["rarity"] != "legendary" || cs[1].Fields != nil {
		t.Fatalf("cloak = %+v", cs[1])
	}
}

func TestLoadWithoutDirectoryHasNoCorrections(t *testing.T) {
	t.Parallel()
	cs, err := corrections.Load(os.DirFS("testdata/absent"))
	if err != nil || len(cs) != 0 {
		t.Fatalf("cs = %v, err = %v", cs, err)
	}
}

const head = "entry: fire-bolt\ndocument: srd-2024\nsource: {document: SRD 5.2.1, page: 1}\n"

func TestLoadRefusesMalformedFiles(t *testing.T) {
	t.Parallel()
	ok := head + "fields: [{field: description, value: Hot.}]\n"
	cases := map[string]fstest.MapFS{
		"unknown field":      {"spell/fire-bolt.yaml": {Data: []byte(ok + "note: x\n")}},
		"unknown in field":   {"spell/fire-bolt.yaml": {Data: []byte(head + "fields: [{field: description, value: x, why: y}]\n")}},
		"unknown in report":  {"spell/fire-bolt.yaml": {Data: []byte(ok + "upstream: {title: t, body: b, link: l}\n")}},
		"missing value":      {"spell/fire-bolt.yaml": {Data: []byte(head + "fields: [{field: description}]\n")}},
		"entry mismatch":     {"spell/fire-ball.yaml": {Data: []byte(ok)}},
		"unknown kind":       {"spells/fire-bolt.yaml": {Data: []byte(ok)}},
		"file at the root":   {"fire-bolt.yaml": {Data: []byte(ok)}},
		"nested too deep":    {"spell/x/fire-bolt.yaml": {Data: []byte(ok)}},
		"not yaml extension": {"spell/fire-bolt.yml": {Data: []byte(ok)}},
		"fields and add":     {"spell/fire-bolt.yaml": {Data: []byte(ok + "add: {name: Fire Bolt}\n")}},
		"neither":            {"spell/fire-bolt.yaml": {Data: []byte(head)}},
		"no document":        {"spell/fire-bolt.yaml": {Data: []byte(strings.Replace(ok, "document: srd-2024\n", "", 1))}},
		"no source page":     {"spell/fire-bolt.yaml": {Data: []byte(strings.Replace(ok, ", page: 1", "", 1))}},
		"no source document": {"spell/fire-bolt.yaml": {Data: []byte(strings.Replace(ok, "document: SRD 5.2.1, ", "", 1))}},
		"bad path":           {"spell/fire-bolt.yaml": {Data: []byte(head + "fields: [{field: 'actions[name=Parry', value: x}]\n")}},
		"empty path step":    {"spell/fire-bolt.yaml": {Data: []byte(head + "fields: [{field: 'a..b', value: x}]\n")}},
		"add names slug":     {"spell/fire-bolt.yaml": {Data: []byte(head + "add: {name: Fire Bolt, slug: fire-bolt}\n")}},
		"add names document": {"spell/fire-bolt.yaml": {Data: []byte(head + "add: {name: Fire Bolt, document: srd-2024}\n")}},
		"report without body": {
			"spell/fire-bolt.yaml": {Data: []byte(ok + "upstream: {title: t}\n")},
		},
		"not a number":        {"spell/fire-bolt.yaml": {Data: []byte(head + "fields: [{field: description, value: .nan}]\n")}},
		"not a number in add": {"spell/fire-bolt.yaml": {Data: []byte(head + "add: {name: Fire Bolt, level: .nan}\n")}},
		"two documents":       {"spell/fire-bolt.yaml": {Data: []byte(ok + "---\n" + ok)}},
		"not yaml":            {"spell/fire-bolt.yaml": {Data: []byte("entry: [\n")}},
	}
	for name, fsys := range cases {
		if _, err := corrections.Load(fsys); !errors.Is(err, corrections.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestApplyCorrectsFieldsAndAddsEntries(t *testing.T) {
	t.Parallel()
	got, warnings, err := corrections.Apply(upstream(), load(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if got.Spells[0].Description != "Fixture: the whole description, as the SRD prints it." {
		t.Errorf("moonbeam = %q", got.Spells[0].Description)
	}
	if got.Monsters[0].Actions[1].Name != "Split" || got.Monsters[0].Actions[1].Description != "Splits." || got.Monsters[0].Actions[0].Name != "Dissolving Pseudopod" {
		t.Errorf("black pudding = %+v", got.Monsters[0].Actions)
	}
	if got.Classes[0].Features[0].Description != "Fixture feature text." || got.Classes[0].Features[0].Levels[0] != 10 {
		t.Errorf("druid = %+v", got.Classes[0].Features)
	}
	if len(got.Items) != 2 {
		t.Fatalf("items = %+v", got.Items)
	}
	potion, cloak := got.Items[0], got.Items[1]
	if !potion.Magic || potion.Rarity != "common" || potion.Category != "potion" {
		t.Errorf("potion = %+v", potion)
	}
	if cloak.Document != "srd-2024" || cloak.Slug != "cloak-of-invisibility" || cloak.Name != "Cloak of Invisibility" ||
		!cloak.Magic || cloak.Rarity != "legendary" || !cloak.RequiresAttunement || cloak.Description != "Fixture text." {
		t.Errorf("cloak = %+v", cloak)
	}
}

func TestApplyWarnsWhereUpstreamCaughtUp(t *testing.T) {
	t.Parallel()
	fixed := upstream()
	fixed.Spells[0].Description = "Fixture: the whole description, as the SRD prints it."
	fixed.Monsters[0].Actions[1].Name = "Split"
	fixed.Items[0].Magic = true
	fixed.Items = append(fixed.Items, snapshot.Item{
		Entry:    snapshot.Entry{Document: "srd-2024", Slug: "cloak-of-invisibility", Name: "Cloak of Invisibility", Description: "Fixture text."},
		Category: "wondrous-item", Magic: true, Rarity: "legendary", RequiresAttunement: true,
	})
	got, warnings, err := corrections.Apply(fixed, load(t))
	if err != nil {
		t.Fatal(err)
	}
	byFile := map[string]corrections.Warning{}
	for _, w := range warnings {
		byFile[w.File] = w
	}
	if len(warnings) != 4 || !byFile["spell/moonbeam.yaml"].Delete || !byFile["monster/black-pudding.yaml"].Delete ||
		!byFile["item/cloak-of-invisibility.yaml"].Delete || byFile["item/potion-of-healing.yaml"].Delete {
		t.Fatalf("warnings = %+v", warnings)
	}
	if !strings.Contains(byFile["item/potion-of-healing.yaml"].Message, "magic") || strings.Contains(byFile["item/potion-of-healing.yaml"].Message, "rarity") {
		t.Errorf("potion warning = %q", byFile["item/potion-of-healing.yaml"].Message)
	}
	if len(got.Items) != 2 || got.Items[0].Rarity != "common" {
		t.Errorf("items = %+v", got.Items)
	}

	fixed.Items[1].Rarity = "very-rare"
	got, warnings, err = corrections.Apply(fixed, load(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range warnings {
		if w.File == "item/cloak-of-invisibility.yaml" && (w.Delete || !strings.Contains(w.Message, "other values")) {
			t.Errorf("cloak warning = %+v", w)
		}
	}
	if got.Items[1].Rarity != "legendary" {
		t.Errorf("cloak = %+v", got.Items[1])
	}
}

func fieldIn(document, kind, entry, path, value string) fstest.MapFS {
	body := "entry: " + entry + "\ndocument: " + document + "\nsource: {document: SRD 5.2.1, page: 1}\nfields: [{field: '" + path + "', value: " + value + "}]\n"
	return fstest.MapFS{kind + "/" + entry + ".yaml": {Data: []byte(body)}}
}

func TestApplyRefusesCorrectionsThatDoNotFit(t *testing.T) {
	t.Parallel()
	field := func(kind, entry, path, value string) fstest.MapFS {
		return fieldIn("srd-2024", kind, entry, path, value)
	}
	cases := map[string]fstest.MapFS{
		"no such entry":          field("spell", "fireball", "description", "x"),
		"no such document":       fieldIn("srd-2014", "spell", "moonbeam", "description", "x"),
		"selector matches none":  field("monster", "black-pudding", "actions[name=Bite].name", "Gnaw"),
		"selector on non-list":   field("monster", "black-pudding", "name[name=x].name", "y"),
		"selector on scalars":    field("spell", "moonbeam", "classes[name=x]", "y"),
		"step through a scalar":  field("monster", "black-pudding", "name.first", "y"),
		"step through a missing": field("monster", "black-pudding", "lair.uses", "3"),
		"unknown field":          field("item", "potion-of-healing", "rarty", "common"),
		"wrong type":             field("item", "potion-of-healing", "magic", "yes"),
		"add unknown field": {"item/cloak.yaml": {Data: []byte(
			"entry: cloak\ndocument: srd-2024\nsource: {document: SRD 5.2.1, page: 1}\nadd: {name: Cloak, cost: 3}\n")}},
		"add breaks the snapshot": {"item/cloak.yaml": {Data: []byte(
			"entry: cloak\ndocument: srd-2024\nsource: {document: SRD 5.2.1, page: 1}\nadd: {category: wondrous-item}\n")}},
	}
	for name, fsys := range cases {
		cs, err := corrections.Load(fsys)
		if err != nil {
			t.Fatalf("%s: load: %v", name, err)
		}
		if _, _, err := corrections.Apply(upstream(), cs); !errors.Is(err, corrections.ErrInvalid) && !errors.Is(err, snapshot.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestApplySurfacesUnencodableSnapshots(t *testing.T) {
	t.Parallel()
	broken := upstream()
	broken.Monsters[0].ChallengeRating = math.NaN()
	if _, _, err := corrections.Apply(broken, nil); err == nil {
		t.Fatal("unencodable snapshot accepted")
	}
}

func TestUnfiledListsReportsToFile(t *testing.T) {
	t.Parallel()
	text := corrections.Unfiled(load(t))
	for _, want := range []string{
		"spell/moonbeam.yaml", "Moonbeam (srd-2024): description cut short", "Fixture: the srd-2024 Moonbeam description stops mid-sentence.",
		"item/cloak-of-invisibility.yaml", "class/druid.yaml", "SRD 5.2.1 p. 150", "item/potion-of-healing.yaml",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "black-pudding") {
		t.Errorf("a filed report is listed:\n%s", text)
	}
	if strings.Index(text, "item/potion-of-healing.yaml") < strings.Index(text, "No report drafted") {
		t.Errorf("a correction without a report is listed among the drafts:\n%s", text)
	}
	if corrections.Unfiled(nil) != "Every upstream report is filed.\n" {
		t.Errorf("empty = %q", corrections.Unfiled(nil))
	}
}

func TestApplyReplacesWholeItemsAndTrimsLists(t *testing.T) {
	t.Parallel()
	fsys := fstest.MapFS{
		"monster/black-pudding.yaml": {Data: []byte("entry: black-pudding\ndocument: srd-2024\nsource: {document: SRD 5.2.1, page: 1}\n" +
			"fields: [{field: 'actions[name=Dissolving Pseudopod]', value: {name: Pseudopod, description: ' Melts. ', type: action, attacks: []}}]\n")},
		"spell/moonbeam.yaml": {Data: []byte("entry: moonbeam\ndocument: srd-2024\nsource: {document: SRD 5.2.1, page: 1}\n" +
			"fields: [{field: classes, value: [' druid ', wizard]}]\n")},
	}
	cs, err := corrections.Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := corrections.Apply(upstream(), cs)
	if err != nil {
		t.Fatal(err)
	}
	if a := got.Monsters[0].Actions[0]; a.Name != "Pseudopod" || a.Description != "Melts." {
		t.Errorf("action = %+v", a)
	}
	if strings.Join(got.Spells[0].Classes, ",") != "druid,wizard" {
		t.Errorf("classes = %v", got.Spells[0].Classes)
	}
	bad := corrections.Correction{File: "spell/moonbeam.yaml", Kind: "spell", Entry: "moonbeam", Document: "srd-2024", Fields: []corrections.Field{{Field: "a..b"}}}
	if _, _, err := corrections.Apply(upstream(), []corrections.Correction{bad}); !errors.Is(err, corrections.ErrInvalid) {
		t.Errorf("bad path: err = %v", err)
	}
}
