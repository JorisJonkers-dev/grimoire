package speciesbuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/speciesbuild"
)

func marshkin() speciesbuild.Design {
	return speciesbuild.Design{
		Sizes: []string{"small", "medium"}, CreatureType: "humanoid", SpeedFt: 30,
		Speeds:      []speciesbuild.Speed{{Kind: "swim", Feet: 30}},
		Senses:      []speciesbuild.Sense{{Kind: "darkvision", Feet: 60}},
		Resistances: []string{"poison"},
		Traits:      []speciesbuild.Trait{{Name: "Reed Breath", Text: "You can hold your breath for an hour."}},
		Spells:      []speciesbuild.Spell{{Level: 3, Spell: "fog-cloud", Name: "Fog Cloud", Uses: "long_rest"}},
		Lineages: []speciesbuild.Lineage{
			{Name: "Bog", Text: "You know the Druidcraft cantrip.", Spells: []speciesbuild.Spell{{Level: 1, Spell: "druidcraft", Name: "Druidcraft", Uses: "at_will"}}},
			{Name: "Reed", Text: "Your Speed rises by 5 feet.", Spells: nil},
		},
	}
}

func TestAMarshkinCompilesIntoOneOptionPerLineage(t *testing.T) {
	t.Parallel()
	d := marshkin()
	if err := speciesbuild.Check(d); err != nil {
		t.Fatal(err)
	}
	opts := speciesbuild.Compile("hb-0190c7a80002", "Marshkin", d)
	if len(opts) != 2 || opts[0].Slug != "hb-0190c7a80002-bog" || opts[0].Name != "Marshkin, Bog lineage" || opts[1].Slug != "hb-0190c7a80002-reed" || opts[0].SpeedFeet != 30 {
		t.Fatalf("options = %+v", opts)
	}
	bog := map[string]speciesbuild.Gained{}
	for _, tr := range opts[0].Traits {
		bog[tr.Name] = tr
	}
	want := map[string]speciesbuild.Gained{
		"Size":          {Name: "Size", Level: 0, Text: "Small or Medium, chosen when you select this species."},
		"Creature Type": {Name: "Creature Type", Level: 0, Text: "Humanoid."},
		"Speed":         {Name: "Speed", Level: 0, Text: "30 feet; Swim 30 feet."},
		"Senses":        {Name: "Senses", Level: 0, Text: "Darkvision 60 feet."},
		"Resistances":   {Name: "Resistances", Level: 0, Text: "You have Resistance to Poison damage."},
		"Reed Breath":   {Name: "Reed Breath", Level: 0, Text: "You can hold your breath for an hour."},
		"Bog lineage":   {Name: "Bog lineage", Level: 0, Text: "You know the Druidcraft cantrip."},
		"Druidcraft":    {Name: "Druidcraft", Level: 1, Text: "You can cast Druidcraft at will."},
		"Fog Cloud":     {Name: "Fog Cloud", Level: 3, Text: "You can cast Fog Cloud once, and again after a Long Rest."},
	}
	if !reflect.DeepEqual(bog, want) {
		t.Fatalf("bog traits = %+v", bog)
	}
	for _, tr := range opts[1].Traits {
		if tr.Name == "Druidcraft" {
			t.Fatal("the Reed lineage has no Druidcraft")
		}
	}
	plain := marshkin()
	plain.Lineages, plain.Sizes, plain.Speeds, plain.Senses, plain.Resistances = nil, []string{"medium"}, nil, nil, nil
	one := speciesbuild.Compile("hb-x", "Marshkin", plain)
	if len(one) != 1 || len(one[0].Traits) != 5 || one[0].Slug != "hb-x" || one[0].Name != "Marshkin" || one[0].Traits[0].Text != "Medium." || one[0].Traits[2].Text != "30 feet." {
		t.Fatalf("a species without lineages = %+v", one)
	}
	lines := strings.Join(speciesbuild.Lines(opts), "\n")
	if !strings.Contains(lines, "\nSize. Small or Medium, chosen when you select this species.\n") {
		t.Errorf("a trait gained from the start reads without a level:\n%s", lines)
	}
	for _, w := range []string{"Marshkin, Bog lineage: 30 feet", "Level 1: Druidcraft. You can cast Druidcraft at will.", "Level 3: Fog Cloud. You can cast Fog Cloud once, and again after a Long Rest.", "Marshkin, Reed lineage"} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
}

func TestSpeciesDesignsThatDoNotBuild(t *testing.T) {
	t.Parallel()
	for want, change := range map[string]func(*speciesbuild.Design){
		"choose one or two sizes":        func(d *speciesbuild.Design) { d.Sizes = nil },
		"choose sizes from tiny":         func(d *speciesbuild.Design) { d.Sizes = []string{"vast"} },
		"choose a creature type":         func(d *speciesbuild.Design) { d.CreatureType = "robot" },
		"a Speed of 10 to 60 feet":       func(d *speciesbuild.Design) { d.SpeedFt = 5 },
		"climb, fly, swim or burrow":     func(d *speciesbuild.Design) { d.Speeds[0].Kind = "teleport" },
		"each speed 5 to 120 feet":       func(d *speciesbuild.Design) { d.Speeds[0].Feet = 0 },
		"darkvision, blindsight":         func(d *speciesbuild.Design) { d.Senses[0].Kind = "sonar" },
		"each sense 5 to 300 feet":       func(d *speciesbuild.Design) { d.Senses[0].Feet = 301 },
		"resist real damage types":       func(d *speciesbuild.Design) { d.Resistances = []string{"cheese"} },
		"name each trait":                func(d *speciesbuild.Design) { d.Traits[0].Name = "" },
		"describe Reed Breath":           func(d *speciesbuild.Design) { d.Traits[0].Text = strings.Repeat("x", 2001) },
		"up to 20 traits":                func(d *speciesbuild.Design) { d.Traits = make([]speciesbuild.Trait, 21) },
		"from character level 1 to 20":   func(d *speciesbuild.Design) { d.Spells[0].Level = 0 },
		"by its slug":                    func(d *speciesbuild.Design) { d.Spells[0].Spell = "Fog!" },
		"name each spell":                func(d *speciesbuild.Design) { d.Spells[0].Name = "" },
		"at will, or once per long rest": func(d *speciesbuild.Design) { d.Spells[0].Uses = "daily" },
		"up to 10 lineages":              func(d *speciesbuild.Design) { d.Lineages = make([]speciesbuild.Lineage, 11) },
		"name each lineage":              func(d *speciesbuild.Design) { d.Lineages[0].Name = "!!" },
		"two lineages by one name":       func(d *speciesbuild.Design) { d.Lineages[1].Name = "bog" },
		"describe the Bog lineage":       func(d *speciesbuild.Design) { d.Lineages[0].Text = strings.Repeat("x", 2001) },
		"lineage spell":                  func(d *speciesbuild.Design) { d.Lineages[0].Spells[0].Uses = "never" },
	} {
		d := marshkin()
		change(&d)
		if err := speciesbuild.Check(d); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	d := marshkin()
	d.Sizes, d.SpeedFt, d.Speeds[0].Feet, d.Senses[0].Feet = []string{"tiny", "huge"}, 60, 120, 300
	d.Traits = make([]speciesbuild.Trait, 20)
	for i := range d.Traits {
		d.Traits[i] = speciesbuild.Trait{Name: strings.Repeat("n", 60), Text: strings.Repeat("x", 2000)}
	}
	d.Spells[0].Level = 20
	for len(d.Lineages) < 10 {
		d.Lineages = append(d.Lineages, speciesbuild.Lineage{Name: "L" + strings.Repeat("x", len(d.Lineages)), Text: "", Spells: nil})
	}
	if err := speciesbuild.Check(d); err != nil {
		t.Fatalf("a design at its limits: %v", err)
	}
	d.SpeedFt, d.Speeds[0].Feet, d.Senses[0].Feet, d.Spells[0].Level = 10, 5, 5, 1
	if err := speciesbuild.Check(d); err != nil {
		t.Fatalf("a design at its lower limits: %v", err)
	}
	d.Sizes = []string{"small", "medium", "large"}
	if err := speciesbuild.Check(d); err == nil {
		t.Fatal("three sizes build")
	}
}
