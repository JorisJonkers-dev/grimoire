//go:build integration

package pgstore_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarydomain "github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
)

// Designs stored as rows of the builders' own part types, every type of row among them, still build the
// same Effects in play.
func TestDesignsStoredAsBuilderRowsStillPlay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	store := pgstore.New(tb.pool)
	lib := &libraryapp.Service{
		Repo: librarypg.New(tb.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Now: time.Now, Surfaces: store.SurfaceKinds,
	}
	long := strings.Repeat("m", 500)
	stored := map[string]struct{ kind, design string }{
		"Marsh Lantern": {"spell", `{"targeting":{"shape":"emanation","sizeFt":10,"rangeFt":0},"save":"wisdom","concentration":true,
			"duration":{"unit":"minutes","amount":1},"ritual":true,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
			"parts":[{"type":"light","brightFt":20,"dimFt":20},{"type":"reveal","qualities":["invisible"]},
			{"type":"condition","condition":"charmed","onlyTypes":["undead","fey"]},{"type":"damage","when":"start_of_turn","dice":"1d6","damageType":"radiant"}]}`},
		"Burst": {"spell", `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":150},"save":"dexterity","concentration":false,
			"duration":{"unit":"rounds","amount":3},"ritual":false,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
			"parts":[{"type":"damage","when":"on_cast","dice":"8d6","damageType":"fire","half":true},{"type":"condition","condition":"prone"},
			{"type":"surface","surface":"grease","rounds":3},{"type":"manual","text":"` + long + `"}]}`},
		"Frostbite": {"condition", `{"icon":"snow","color":"#7fa8dd","text":"Cold seeps in.","ends":"rest","stacks":true,"maxLevel":4,
			"perLevel":{"d20":1,"speedFt":5,"deathAt":0},"parts":[{"type":"attack_disadvantage"},{"type":"attacked_advantage"},
			{"type":"save_disadvantage","ability":"dexterity"},{"type":"save_fails","ability":"strength"},{"type":"save_advantage","ability":"constitution"},
			{"type":"attack_advantage"},{"type":"attacked_disadvantage"},{"type":"incapacitated"},{"type":"immobile"},{"type":"speed_penalty","feet":10},
			{"type":"crit_within","feet":5},{"type":"manual","text":"` + long + `"}]}`},
	}
	slugs := map[string]string{}
	for name, s := range stored {
		e, err := lib.Create(ctx, dm, librarydomain.Draft{Kind: s.kind, Name: name})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lib.Link(ctx, dm, tb.campaign, e.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := tb.pool.Exec(ctx, "UPDATE library.entries SET design = $2 WHERE id = $1", e.ID, s.design); err != nil {
			t.Fatal(err)
		}
		slugs[name] = spellbuild.Slug(e.ID.String())
	}
	brew, err := store.Homebrew(ctx, tb.campaign)
	if err != nil || len(brew.Spells) != 2 || len(brew.Conditions) != 1 {
		t.Fatalf("homebrew = %+v %v", brew, err)
	}
	got := map[string]effects.Definition{brew.Conditions[0].Definition.Name: brew.Conditions[0].Definition}
	for _, b := range brew.Spells {
		got[b.Definition.Name] = b.Definition
	}
	failed := func(ability, condition string) effects.Part {
		return effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1, Type: ""}, Then: []effects.Part{effects.SaveCondition{Ability: ability, Slug: condition}}}
	}
	edge := func(against, advantage bool) effects.Part {
		return effects.Edge{Against: against, Advantage: advantage, Range: effects.AnyRange, SourceOnly: false}
	}
	want := map[string]effects.Definition{
		"Marsh Lantern": {
			Slug: slugs["Marsh Lantern"], Name: "Marsh Lantern", Owner: effects.OwnedBySpell, Concentration: true,
			Duration: effects.Duration{Kind: effects.Minutes, Amount: 1, RepeatSave: ""}, Scaling: nil,
			Parts: []effects.Part{
				effects.Area{Shape: hex.EmanationArea, SizeFt: 10, RangeFt: 0},
				effects.AreaSave{Ability: "wisdom"},
				effects.Light{BrightFt: 20, DimFt: 20},
				effects.Reveal{Qualities: []string{"invisible"}},
				effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: "undead"}, Then: []effects.Part{failed("wisdom", "charmed")}},
				effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: "fey"}, Then: []effects.Part{failed("wisdom", "charmed")}},
				effects.CreateSurface{Surface: surface.Kind(slugs["Marsh Lantern"] + "-ground"), Rounds: 10},
			},
		},
		"Burst": {
			Slug: slugs["Burst"], Name: "Burst", Owner: effects.OwnedBySpell, Concentration: false,
			Duration: effects.Duration{Kind: effects.Rounds, Amount: 3, RepeatSave: ""}, Scaling: nil,
			Parts: []effects.Part{
				effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
				effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
				failed("dexterity", "prone"),
				effects.CreateSurface{Surface: "grease", Rounds: 3},
				effects.Manual{Instruction: long},
			},
		},
		"Frostbite": {
			Slug: slugs["Frostbite"], Name: "Frostbite", Owner: effects.OwnedByCondition, Concentration: false,
			Duration: effects.Duration{Kind: effects.UntilRest, Amount: 0, RepeatSave: ""}, Scaling: nil,
			Parts: []effects.Part{
				effects.Exhausting{D20PerLevel: 1, SpeedFtPerLevel: 5, DeathAt: 0, MaxLevel: 4},
				edge(false, false), edge(true, true),
				effects.SaveEdge{Ability: "dexterity", Mode: effects.SaveDisadvantage},
				effects.SaveEdge{Ability: "strength", Mode: effects.SaveFails},
				effects.SaveEdge{Ability: "constitution", Mode: effects.SaveAdvantage},
				edge(false, true), edge(true, false),
				effects.Incapacitated{},
				effects.Immobile{},
				effects.SpeedPenalty{Ft: 10},
				effects.CritWithin{Feet: 5},
				effects.Manual{Instruction: long},
			},
		},
	}
	for name, w := range want {
		if !reflect.DeepEqual(got[name], w) {
			t.Errorf("%s =\n%+v\nwant\n%+v", name, got[name], w)
		}
	}
}
