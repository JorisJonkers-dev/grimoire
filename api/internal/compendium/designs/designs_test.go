package designs_test

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/designs"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

func seed() snapshot.Snapshot {
	return snapshot.Snapshot{
		Spells: []snapshot.Spell{
			{Document: "srd-2024", Slug: "fireball", Name: "Fireball"},
			{Document: "srd-2024", Slug: "augury", Name: "Augury"},
			{Document: "srd-2014", Slug: "light", Name: "Light"},
			{Document: "srd-2024", Slug: "prone", Name: "Prone"},
		},
		Conditions: []snapshot.Condition{{Document: "srd-2024", Slug: "prone", Name: "Prone"}},
		Classes: []snapshot.Class{{
			Entry:    snapshot.Entry{Document: "srd-2024", Slug: "barbarian", Name: "Barbarian"},
			Features: []snapshot.ClassFeature{{Slug: "rage", Name: "Rage"}, {Slug: "weapon-mastery", Name: "Weapon Mastery"}},
		}},
		Items: []snapshot.Item{{Entry: snapshot.Entry{Document: "srd-2024", Slug: "rope", Name: "Rope"}}},
	}
}

const fireball = `entry: spells/fireball
name: Fireball
automation: full
pattern: save-area-damage
effects:
  - activation:
      duration: {unit: instant}
      targeting: {kind: area, range_ft: 150}
    parts:
      - kind: area
        shape: sphere
        size_ft: 20
        range_ft: 150
      - kind: save_damage
        ability: dexterity
        dice: 8d6
        damage_type: fire
        half: true
        scaling:
          - {field: dice, axis: slot_level, rule: linear, base: 3, per: 1d6}
`

const prone = `entry: conditions/prone
name: Prone
automation: full
effects:
  - activation:
      concentration: false
      duration: {unit: rounds, amount: 1, repeat_save: strength}
    parts:
      - kind: edge
        against: true
        advantage: true
        range: within_five
    scaling:
      - field: duration
        axis: character_level
        rule: table
        steps: [{at: 5, value: 2}]
`

const augury = `entry: spells/augury
name: Augury
automation: exception
exception: [the omen]
effects:
  - parts:
      - kind: manual
        instruction: Tell the caster the omen.
`

const exceptions = `- entry: spells/augury
  ground: dm_knowledge
  parts: [the omen]
  reason: Only the DM knows what the future holds.
`

func files(over map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{
		"srd-2024/spells/fireball.yaml":  {Data: []byte(fireball)},
		"srd-2024/conditions/prone.yaml": {Data: []byte(prone)},
		"srd-2024/spells/augury.yaml":    {Data: []byte(augury)},
		"srd-2024/_exceptions.yaml":      {Data: []byte(exceptions)},
		"srd-2024/README.md":             {Data: []byte("# Design files\n")},
	}
	for path, body := range over {
		if body == "" {
			delete(fsys, path)
			continue
		}
		fsys[path] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

func read(t *testing.T, fsys fs.FS) designs.Set {
	t.Helper()
	set, err := designs.Read(fsys, seed())
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func find(t *testing.T, set designs.Set, slug string) designs.Loaded {
	t.Helper()
	for _, l := range set.Effects {
		if l.Definition.Slug == slug {
			return l
		}
	}
	t.Fatalf("no Effect %s in %+v", slug, set.Effects)
	return designs.Loaded{}
}

// A spell's file reads as its Effect: the Parts in the engine's kinds, a Part's own Scaling, owned by the
// spell it names.
func TestASpellFileReadsAsItsEffect(t *testing.T) {
	t.Parallel()
	got := find(t, read(t, files(nil)), "fireball")
	want := designs.Loaded{
		Owner: effects.Owner{Kind: effects.OwnedBySpell, Slug: "fireball"},
		Definition: effects.Definition{
			Slug: "fireball", Name: "Fireball", Owner: effects.OwnedBySpell, Concentration: false,
			Duration:  effects.Duration{Kind: effects.Instant, Amount: 0, RepeatSave: ""},
			Targeting: effects.Targeting{Kind: effects.TargetArea, RangeFt: 150},
			Scaling: []effects.Scaling{{
				Part: 1, Field: "dice", Axis: effects.SlotLevel, Class: "",
				Rule: effects.Linear{Base: 3, Per: effects.Value{Text: "1d6", Number: 0, Flag: false, Texts: nil}},
			}},
			Parts: []effects.Part{
				effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
				effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fireball =\n%+v\nwant\n%+v", got, want)
	}
}

// A condition's file reads as the condition's Effect, with its Activation's duration, repeat save and
// the Scaling of its own values.
func TestAConditionFileReadsAsItsEffect(t *testing.T) {
	t.Parallel()
	got := find(t, read(t, files(nil)), "prone")
	if got.Owner != (effects.Owner{Kind: effects.OwnedByCondition, Slug: "prone"}) || got.Definition.Owner != effects.OwnedByCondition {
		t.Fatalf("owner = %+v", got)
	}
	if got.Definition.Duration != (effects.Duration{Kind: effects.Rounds, Amount: 1, RepeatSave: "strength"}) {
		t.Fatalf("duration = %+v", got.Definition.Duration)
	}
	want := []effects.Scaling{{
		Part: effects.OnActivation, Field: "duration", Axis: effects.CharacterLevel, Class: "",
		Rule: effects.Table{Steps: []effects.Step{{At: 5, Value: effects.Value{Text: "", Number: 2, Flag: false, Texts: nil}}}},
	}}
	if !reflect.DeepEqual(got.Definition.Scaling, want) {
		t.Fatalf("scaling = %+v", got.Definition.Scaling)
	}
	if len(got.Definition.Parts) != 1 || got.Definition.Parts[0].Kind() != "edge" {
		t.Fatalf("parts = %+v", got.Definition.Parts)
	}
}

// An approved exception keeps its Manual Part.
func TestAnApprovedExceptionKeepsItsManualPart(t *testing.T) {
	t.Parallel()
	got := find(t, read(t, files(nil)), "augury")
	if got.Definition.Automated() || got.Definition.Parts[0].Kind() != "manual" {
		t.Fatalf("augury = %+v", got.Definition)
	}
}

// An entry with several Effects names each, and each is its own Effect under the entry's slug and its
// label.
func TestSeveralEffectsAreNamedByTheirLabels(t *testing.T) {
	t.Parallel()
	two := strings.Replace(fireball, "effects:\n  - activation:", "effects:\n  - label: Blast\n    activation:", 1) + `  - label: Smoke
    parts:
      - kind: light
        bright_ft: 5
`
	set := read(t, files(map[string]string{"srd-2024/spells/fireball.yaml": two}))
	blast, smoke := find(t, set, "fireball-blast"), find(t, set, "fireball-smoke")
	if blast.Definition.Name != "Fireball: Blast" || smoke.Definition.Name != "Fireball: Smoke" || smoke.Owner.Slug != "fireball" {
		t.Fatalf("blast = %+v, smoke = %+v", blast, smoke)
	}
}

const branch = `      - kind: branch
        when: hp_at_most
        n: 20
        then:
          - kind: bonus_die
            dice: 1d4
            on: [attack_rolls, saving_throws]
`

// A Part holds the Parts beneath it under its group's name, and a field of several options takes a list.
func TestPartsNestAndTakeLists(t *testing.T) {
	t.Parallel()
	got := find(t, read(t, files(map[string]string{"srd-2024/spells/fireball.yaml": fireball + branch})), "fireball")
	b, ok := got.Definition.Parts[2].(effects.Branch)
	if !ok || len(b.Then) != 1 {
		t.Fatalf("parts = %+v", got.Definition.Parts)
	}
	if die, ok := b.Then[0].(effects.BonusDie); !ok || die.Dice != "1d4" || len(die.On) != 2 {
		t.Fatalf("then = %+v", b.Then)
	}
}

// The hash covers every file, so any change to one loads them all again, and only a change does.
func TestTheHashCoversEveryFile(t *testing.T) {
	t.Parallel()
	a, b := read(t, files(nil)), read(t, files(nil))
	if a.Hash == "" || a.Hash != b.Hash {
		t.Fatalf("hashes %q and %q", a.Hash, b.Hash)
	}
	changed := read(t, files(map[string]string{"srd-2024/README.md": "# Design files, again\n"}))
	if changed.Hash == a.Hash {
		t.Fatal("a changed file kept the hash")
	}
}

// Entries with no Effect are listed with their reason, and count as read.
func TestEntriesWithNoEffectAreListed(t *testing.T) {
	t.Parallel()
	set := read(t, files(map[string]string{
		"srd-2024/features/_no-effect.yaml": "- {entry: features/barbarian/weapon-mastery, reason: choice}\n",
		"srd-2024/items/_no-effect.yaml":    "- {entry: items/rope, reason: plain_gear}\n",
	}))
	want := []designs.NoEffect{{Entry: "features/barbarian/weapon-mastery", Reason: "choice"}, {Entry: "items/rope", Reason: "plain_gear"}}
	if !reflect.DeepEqual(set.NoEffect, want) {
		t.Fatalf("no effect = %+v", set.NoEffect)
	}
}

// Every file the reader cannot resolve is refused with its path and why; every problem is reported at
// once.
func TestFilesThatDoNotRead(t *testing.T) {
	t.Parallel()
	const fb = "srd-2024/spells/fireball.yaml"
	cases := map[string]map[string]string{
		"spells/fireball.yaml: does not read: yaml: unmarshal errors:\n  line 4: field colour not found": {fb: strings.Replace(fireball, "pattern:", "colour: red\npattern:", 1)},
		"spells/fireball.yaml: entry is spells/fire-ball, not the file's own path":                       {fb: strings.Replace(fireball, "entry: spells/fireball", "entry: spells/fire-ball", 1)},
		"spells/meteor.yaml: no such entry in the SRD 2024 seed":                                         {"srd-2024/spells/meteor.yaml": strings.ReplaceAll(fireball, "fireball", "meteor")},
		"spells/light.yaml: no such entry in the SRD 2024 seed":                                          {"srd-2024/spells/light.yaml": strings.NewReplacer("fireball", "light", "Fireball", "Light").Replace(fireball)},
		"spells/fireball.yaml: name is Fire Ball; the seed calls it Fireball":                            {fb: strings.Replace(fireball, "name: Fireball", "name: Fire Ball", 1)},
		"spells/fireball.yaml: automation is full or exception, not partial":                             {fb: strings.Replace(fireball, "automation: full", "automation: partial", 1)},
		"spells/fireball.yaml: a full entry states no exception":                                         {fb: strings.Replace(fireball, "automation: full", "automation: full\nexception: [the blast]", 1)},
		"spells/fireball.yaml: no effects":                                                               {fb: strings.SplitAfter(fireball, "pattern: save-area-damage\n")[0] + "effects: []\n"},
		"spells/fireball.yaml: effect 1: part 2: dice: write the damage like 8d6":                        {fb: strings.Replace(fireball, "dice: 8d6", "dice: eight", 1)},
		"spells/fireball.yaml: effect 1: part 1: size_ft: write a whole number":                          {fb: strings.Replace(fireball, "size_ft: 20", "size_ft: twenty", 1)},
		"spells/fireball.yaml: effect 1: part 2: half: write true or false":                              {fb: strings.Replace(fireball, "half: true", "half: mostly", 1)},
		"spells/fireball.yaml: effect 1: part 1: kind: name the Part's kind":                             {fb: strings.Replace(fireball, "- kind: area", "- form: area", 1)},
		"spells/fireball.yaml: effect 1: part 2: colour: Save for damage has no such field":              {fb: strings.Replace(fireball, "half: true", "half: true\n        colour: red", 1)},
		"spells/fireball.yaml: effect 1: part 2: scaling 1: field: choose a field that scales":           {fb: strings.Replace(fireball, "field: dice", "field: damage_type", 1)},
		"spells/fireball.yaml: effect 1: scaling 1: does not read: yaml: unmarshal errors:\n  line 1: field grows not found": {
			fb: fireball + "    scaling:\n      - {grows: duration}\n",
		},
		"spells/fireball.yaml: effect 1: duration: unit: choose one of instant, rounds, minutes, hours, until_dispelled, until_rest, permanent, end_of_next_turn, until_cured": {
			fb: strings.Replace(fireball, "unit: instant", "unit: fortnights", 1),
		},
		"spells/fireball.yaml: effect 1: duration: amount: runs from 0 to 1000": {fb: strings.Replace(fireball, "unit: instant", "unit: rounds, amount: 2000", 1)},
		"spells/fireball.yaml: effect 1: duration: repeat_save: choose one of strength, dexterity, constitution, intelligence, wisdom, charisma": {
			fb: strings.Replace(fireball, "unit: instant", "unit: rounds, amount: 1, repeat_save: luck", 1),
		},
		"spells/fireball.yaml: a full entry holds no Manual Part":                         {fb: fireball + "      - kind: manual\n        instruction: Roll it by hand.\n"},
		"spells/fireball.yaml: name every Effect with a label when the entry has several": {fb: fireball + "  - parts:\n      - kind: light\n        bright_ft: 5\n"},
		"spells/fireball.yaml: label each Effect once":                                    {fb: strings.Replace(fireball, "  - activation:", "  - label: Blast\n    activation:", 1) + "  - label: blast\n    parts:\n      - kind: light\n        bright_ft: 5\n"},
		"features/barbarian/rage.yaml: features load once their link lands (#262)":        {"srd-2024/features/barbarian/rage.yaml": "entry: features/barbarian/rage\n"},
		"spells/fireball.yaml: an exception must be on the approved list":                 {fb: strings.Replace(fireball, "automation: full", "automation: exception\nexception: [the blast]", 1)},
		"spells/augury.yaml: an exception states the parts left to the DM":                {"srd-2024/spells/augury.yaml": strings.Replace(augury, "exception: [the omen]\n", "", 1)},
		"spells/augury.yaml: the approved list leaves the omen to the DM, not the sign":   {"srd-2024/spells/augury.yaml": strings.Replace(augury, "[the omen]", "[the sign]", 1)},
		"spells/augury.yaml: the approved list marks this entry an exception":             {"srd-2024/spells/augury.yaml": strings.NewReplacer("automation: exception", "automation: full", "exception: [the omen]\n", "", "kind: manual\n        instruction: Tell the caster the omen.", "kind: light\n        bright_ft: 5").Replace(augury)},
		"_exceptions.yaml: spells/meteor: no such entry in the SRD 2024 seed":             {"srd-2024/_exceptions.yaml": exceptions + "- {entry: spells/meteor, ground: on_the_spot, parts: [the rocks], reason: Made up.}\n"},
		"_exceptions.yaml: spells/augury: ground is dm_knowledge, on_the_spot or free_form, not whim": {
			"srd-2024/_exceptions.yaml": strings.Replace(exceptions, "dm_knowledge", "whim", 1),
		},
		"_exceptions.yaml: spells/augury: state the parts left to the DM and the reason": {"srd-2024/_exceptions.yaml": strings.Replace(exceptions, "  reason: Only the DM knows what the future holds.\n", "", 1)},
		"_exceptions.yaml: spells/augury: list each entry once":                          {"srd-2024/_exceptions.yaml": exceptions + exceptions},
		"_exceptions.yaml: does not read: yaml: unmarshal errors:\n  line 5: field why not found": {
			"srd-2024/_exceptions.yaml": exceptions + "  why: because\n",
		},
		"features/_no-effect.yaml: features/barbarian/frenzy: no such entry in the SRD 2024 seed": {"srd-2024/features/_no-effect.yaml": "- {entry: features/barbarian/frenzy, reason: choice}\n"},
		"items/_no-effect.yaml: items/rope: reason is table_column, choice, sheet_value or plain_gear, not boring": {
			"srd-2024/items/_no-effect.yaml": "- {entry: items/rope, reason: boring}\n",
		},
		"spells/_no-effect.yaml: only features and items list entries with no Effect": {"srd-2024/spells/_no-effect.yaml": "- {entry: spells/fireball, reason: choice}\n"},
		"items/_no-effect.yaml: items/rope: list each entry once":                     {"srd-2024/items/_no-effect.yaml": "- {entry: items/rope, reason: plain_gear}\n- {entry: items/rope, reason: plain_gear}\n"},
		"items/_no-effect.yaml: does not read: yaml: unmarshal errors:\n  line 1: field why not found": {
			"srd-2024/items/_no-effect.yaml": "- {entry: items/rope, reason: plain_gear, why: x}\n",
		},
		"spells/fireball.txt: a Design file is YAML, named <entry>.yaml":                    {"srd-2024/spells/fireball.txt": "x"},
		"spells/fireball.yaml: effect 1: scaling 1: field: choose a field that scales":      {fb: fireball + "    scaling:\n      - {field: dice, axis: slot_level, rule: linear, base: 1, per: 1}\n"},
		"spells/fireball.yaml: effect 1: targeting: count: runs from 0 to 50":               {fb: strings.Replace(fireball, "{kind: area, range_ft: 150}", "{kind: creatures, count: 99}", 1)},
		"spells/fireball.yaml: effect 1: targeting: kind: choose one of the options":        {fb: strings.Replace(fireball, "{kind: area, range_ft: 150}", "{range_ft: 150}", 1)},
		"spellz/fireball.yaml: no kind of entry is kept in spellz":                          {"srd-2024/spellz/fireball.yaml": fireball},
		"items/rope.yaml: items load once their link lands (#263)":                          {"srd-2024/items/rope.yaml": "entry: items/rope\n"},
		"monsters/wolf.yaml: monsters load once their link lands (#264)":                    {"srd-2024/monsters/wolf.yaml": "entry: monsters/wolf\n"},
		"spells/fireball.yaml: effect 1: part 3: then: part 1: on: write a list of options": {fb: fireball + strings.Replace(branch, "[attack_rolls, saving_throws]", "attack_rolls", 1)},
		"spells/fireball.yaml: effect 1: part 3: then: part 1: dice: write one value":       {fb: fireball + strings.Replace(branch, "dice: 1d4", "dice: [1d4]", 1)},
		"spells/fireball.yaml: effect 1: part 3: write the Part as its fields":              {fb: fireball + "      - branch\n"},
		"fireball.yaml: a full entry holds no Manual Part":                                  {fb: fireball + strings.Replace(branch, "kind: bonus_die\n            dice: 1d4\n            on: [attack_rolls, saving_throws]", "kind: manual\n            instruction: Decide.", 1)},
		"spells/fireball.yaml: effect 1: part 1: scaling 1: per: write a whole number": {
			fb: strings.Replace(fireball, "        range_ft: 150\n", "        range_ft: 150\n        scaling: [{field: size_ft, axis: slot_level, rule: linear, base: 3, per: five}]\n", 1),
		},
		"spells/fireball.yaml: effect 1: part 1: scaling 1: steps: write a whole number": {
			fb: strings.Replace(fireball, "        range_ft: 150\n", "        range_ft: 150\n        scaling: [{field: size_ft, axis: slot_level, rule: table, steps: [{at: 5, value: big}]}]\n", 1),
		},
		"spells/prone.yaml: an Effect named prone is already loaded from conditions/prone.yaml": {
			"srd-2024/spells/prone.yaml": strings.NewReplacer("conditions/prone", "spells/prone").Replace(prone),
		},
	}
	for want, over := range cases {
		_, err := designs.Read(files(over), seed())
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v", want, err)
		}
	}
}

// Before the maintainer lists any exception there is no list, and no entry may be an exception.
func TestNoExceptionListApprovesNone(t *testing.T) {
	t.Parallel()
	read(t, files(map[string]string{"srd-2024/_exceptions.yaml": "", "srd-2024/spells/augury.yaml": ""}))
	_, err := designs.Read(files(map[string]string{"srd-2024/_exceptions.yaml": ""}), seed())
	if err == nil || !strings.Contains(err.Error(), "spells/augury.yaml: an exception must be on the approved list") {
		t.Fatalf("err = %v", err)
	}
}

// Without the folder of Design files there is nothing to load, and the read says so.
func TestAMissingFolderIsRefused(t *testing.T) {
	t.Parallel()
	if _, err := designs.Read(fstest.MapFS{}, seed()); err == nil || !strings.Contains(err.Error(), "designs: open srd-2024") {
		t.Fatalf("err = %v", err)
	}
}

// Every problem is reported, not only the first.
func TestEveryProblemIsReported(t *testing.T) {
	t.Parallel()
	_, err := designs.Read(files(map[string]string{
		"srd-2024/spells/fireball.yaml":  strings.Replace(fireball, "name: Fireball", "name: Fire Ball", 1),
		"srd-2024/conditions/prone.yaml": strings.Replace(prone, "name: Prone", "name: Lying Down", 1),
	}), seed())
	if err == nil || !strings.Contains(err.Error(), "Fire Ball") || !strings.Contains(err.Error(), "Lying Down") {
		t.Fatalf("err = %v", err)
	}
}

// The Design files the binary carries read against the seed it carries; this is the check every batch
// runs in CI.
func TestTheShippedDesignFilesRead(t *testing.T) {
	t.Parallel()
	seeds, err := fs.Sub(db.Seeds, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	snap, _, err := snapshot.Load(seeds)
	if err != nil {
		t.Fatal(err)
	}
	shipped, err := fs.Sub(db.Designs, "designs")
	if err != nil {
		t.Fatal(err)
	}
	set, err := designs.Read(shipped, snap)
	if err != nil {
		t.Fatal(err)
	}
	if set.Hash == "" {
		t.Fatal("no hash")
	}
}
