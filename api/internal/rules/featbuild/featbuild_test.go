package featbuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/featbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

func lampwright() featbuild.Feat {
	return featbuild.Feat{
		Category: "general", Text: "Your lanterns burn twice as long.", Repeatable: true,
		Prerequisites: []featbuild.Prerequisite{
			{Kind: "level", Minimum: 4, Ability: "", Feat: "", Group: 0},
			{Kind: "ability", Ability: "wisdom", Minimum: 13, Feat: "", Group: 1},
			{Kind: "ability", Ability: "intelligence", Minimum: 13, Feat: "", Group: 1},
			{Kind: "spellcasting", Minimum: 0, Ability: "", Feat: "", Group: 2},
			{Kind: "feat", Feat: "alert", Minimum: 0, Ability: "", Group: 3},
		},
	}
}

func bogwarden() featbuild.Background {
	return featbuild.Background{
		Abilities: []string{"strength", "wisdom", "constitution"}, Skills: []string{"survival", "nature"},
		Feat: "alert", FeatName: "Alert", Tool: "Herbalism Kit", Equipment: "A lantern, a pole and 8 GP", Gold: 50,
		Text: "You kept the causeways open through the fens.",
	}
}

func TestALampwrightFeat(t *testing.T) {
	t.Parallel()
	d := lampwright()
	if err := featbuild.CheckFeat(d); err != nil {
		t.Fatal(err)
	}
	f := featbuild.CompileFeat("hb-0190c7a80003", "Lampwright", d)
	if f.Slug != "hb-0190c7a80003" || f.Name != "Lampwright" || f.Category != "General" || !f.Repeatable || f.Description != "Your lanterns burn twice as long." {
		t.Fatalf("feat = %+v", f)
	}
	want := []features.Requirement{
		{Kind: features.MinLevel, Ability: "", Minimum: 4, Slug: "", Group: 0},
		{Kind: features.MinAbility, Ability: "wisdom", Minimum: 13, Slug: "", Group: 1},
		{Kind: features.MinAbility, Ability: "intelligence", Minimum: 13, Slug: "", Group: 1},
		{Kind: features.Spellcasting, Ability: "", Minimum: 0, Slug: "", Group: 2},
		{Kind: features.HasFeat, Ability: "", Minimum: 0, Slug: "alert", Group: 3},
	}
	if !reflect.DeepEqual(f.Requirements, want) {
		t.Fatalf("requirements = %+v", f.Requirements)
	}
	unmet := features.Unmet(f.Requirements, features.Candidate{Level: 4, Abilities: map[string]int{"intelligence": 14}, Spellcasting: true, Feats: nil, Features: nil})
	if len(unmet) != 1 || unmet[0] != "the alert feat" {
		t.Fatalf("unmet = %v", unmet)
	}
	lines := strings.Join(featbuild.FeatLines(f), "\n")
	for _, w := range []string{"General feat, repeatable", "Prerequisites: Level 4+; Wisdom 13+ or Intelligence 13+; Spellcasting; the alert feat", "Your lanterns burn twice as long."} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
	plain := featbuild.CompileFeat("hb-x", "Plain", featbuild.Feat{Category: "origin", Text: "", Repeatable: false, Prerequisites: nil})
	if got := featbuild.FeatLines(plain); !reflect.DeepEqual(got, []string{"Origin feat"}) || plain.Category != "Origin" {
		t.Fatalf("a plain origin feat = %v", got)
	}
	for kind, name := range map[string]string{"fighting_style": "Fighting Style", "epic_boon": "Epic Boon"} {
		if c := featbuild.CompileFeat("x", "X", featbuild.Feat{Category: kind}).Category; c != name {
			t.Errorf("%s = %s", kind, c)
		}
	}
}

func TestABogwardenBackground(t *testing.T) {
	t.Parallel()
	d := bogwarden()
	if err := featbuild.CheckBackground(d); err != nil {
		t.Fatal(err)
	}
	b := featbuild.CompileBackground("hb-0190c7a80004", "Bogwarden", d)
	if b.Slug != "hb-0190c7a80004" || b.Name != "Bogwarden" || !reflect.DeepEqual(b.Abilities, []string{"strength", "wisdom", "constitution"}) || !reflect.DeepEqual(b.Skills, []string{"survival", "nature"}) || b.Feat != "alert" {
		t.Fatalf("background = %+v", b)
	}
	want := []featbuild.Trait{
		{Name: "Bogwarden", Text: "You kept the causeways open through the fens."},
		{Name: "Origin Feat", Text: "Alert."},
		{Name: "Tool Proficiency", Text: "Herbalism Kit."},
		{Name: "Equipment", Text: "Choose A lantern, a pole and 8 GP; or 50 GP."},
	}
	if !reflect.DeepEqual(b.Traits, want) {
		t.Fatalf("traits = %+v", b.Traits)
	}
	lines := strings.Join(featbuild.BackgroundLines(b), "\n")
	for _, w := range []string{"Ability Scores: Strength, Wisdom, Constitution", "Skill Proficiencies: Survival, Nature", "Equipment. Choose A lantern"} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
	bare := d
	bare.Tool, bare.Equipment, bare.Gold, bare.Text = "", "", 0, ""
	if got := featbuild.CompileBackground("x", "Bare", bare).Traits; len(got) != 1 || got[0].Name != "Origin Feat" {
		t.Fatalf("a bare background = %+v", got)
	}
	gold := d
	gold.Equipment = ""
	if got := featbuild.CompileBackground("x", "Gold", gold).Traits[3].Text; got != "50 GP." {
		t.Fatalf("gold only = %q", got)
	}
	kit := d
	kit.Gold = 0
	if got := featbuild.CompileBackground("x", "Kit", kit).Traits[3].Text; got != "A lantern, a pole and 8 GP." {
		t.Fatalf("equipment only = %q", got)
	}
}

func TestFeatsAndBackgroundsThatDoNotBuild(t *testing.T) {
	t.Parallel()
	for want, change := range map[string]func(*featbuild.Feat){
		"choose a category":            func(f *featbuild.Feat) { f.Category = "heroic" },
		"in up to 4000 characters":     func(f *featbuild.Feat) { f.Text = strings.Repeat("x", 4001) },
		"up to 10 prerequisites":       func(f *featbuild.Feat) { f.Prerequisites = make([]featbuild.Prerequisite, 11) },
		"level, ability, spellcasting": func(f *featbuild.Feat) { f.Prerequisites[0].Kind = "luck" },
		"a level of 1 to 20":           func(f *featbuild.Feat) { f.Prerequisites[0].Minimum = 21 },
		"an ability score of 1 to 30":  func(f *featbuild.Feat) { f.Prerequisites[1].Minimum = 0 },
		"name the ability":             func(f *featbuild.Feat) { f.Prerequisites[1].Ability = "luck" },
		"name the feat it needs":       func(f *featbuild.Feat) { f.Prerequisites[4].Feat = "Alert!" },
		"group of 0 to 9":              func(f *featbuild.Feat) { f.Prerequisites[0].Group = 10 },
	} {
		f := lampwright()
		change(&f)
		if err := featbuild.CheckFeat(f); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	for want, change := range map[string]func(*featbuild.Background){
		"three different abilities":   func(b *featbuild.Background) { b.Abilities = []string{"strength", "strength", "wisdom"} },
		"choose real abilities":       func(b *featbuild.Background) { b.Abilities = []string{"strength", "luck", "wisdom"} },
		"two different skills":        func(b *featbuild.Background) { b.Skills = []string{"nature"} },
		"choose real skills":          func(b *featbuild.Background) { b.Skills = []string{"nature", "juggling"} },
		"its Origin feat by its slug": func(b *featbuild.Background) { b.Feat = "" },
		"name the Origin feat":        func(b *featbuild.Background) { b.FeatName = "" },
		"a tool in up to 80":          func(b *featbuild.Background) { b.Tool = strings.Repeat("t", 81) },
		"equipment in up to 500":      func(b *featbuild.Background) { b.Equipment = strings.Repeat("e", 501) },
		"0 to 1000 GP":                func(b *featbuild.Background) { b.Gold = -1 },
		"in up to 2000 characters":    func(b *featbuild.Background) { b.Text = strings.Repeat("x", 2001) },
	} {
		b := bogwarden()
		change(&b)
		if err := featbuild.CheckBackground(b); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
	at := lampwright()
	at.Text, at.Prerequisites = strings.Repeat("x", 4000), make([]featbuild.Prerequisite, 10)
	for i := range at.Prerequisites {
		at.Prerequisites[i] = featbuild.Prerequisite{Kind: "ability", Ability: "wisdom", Minimum: 30, Feat: "", Group: 9}
	}
	at.Prerequisites[0] = featbuild.Prerequisite{Kind: "level", Minimum: 20, Ability: "", Feat: "", Group: 0}
	at.Prerequisites[1] = featbuild.Prerequisite{Kind: "level", Minimum: 1, Ability: "", Feat: "", Group: 0}
	at.Prerequisites[2] = featbuild.Prerequisite{Kind: "ability", Ability: "wisdom", Minimum: 1, Feat: "", Group: 0}
	if err := featbuild.CheckFeat(at); err != nil {
		t.Fatalf("a feat at its limits: %v", err)
	}
	b := bogwarden()
	b.Tool, b.Equipment, b.Gold, b.Text, b.FeatName = strings.Repeat("t", 80), strings.Repeat("e", 500), 1000, strings.Repeat("x", 2000), strings.Repeat("f", 60)
	if err := featbuild.CheckBackground(b); err != nil {
		t.Fatalf("a background at its limits: %v", err)
	}
	b.Gold = 1001
	if err := featbuild.CheckBackground(b); err == nil {
		t.Fatal("1001 GP builds")
	}
}
