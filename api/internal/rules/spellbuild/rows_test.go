package spellbuild_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

const lanternRows = `{"targeting":{"shape":"emanation","sizeFt":10,"rangeFt":0},"save":"wisdom","concentration":true,
"duration":{"unit":"minutes","amount":1},"ritual":true,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
"parts":[{"type":"light","brightFt":20,"dimFt":20},{"type":"reveal","qualities":["invisible"]},
{"type":"condition","condition":"charmed","onlyTypes":["undead","fey"]},{"type":"damage","when":"start_of_turn","dice":"1d6","damageType":"radiant"}]}`

func burstRows(manual string) string {
	return `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":150},"save":"dexterity","concentration":false,
"duration":{"unit":"rounds","amount":3},"ritual":false,"castingTime":{"kind":"action"},"components":{"verbal":true,"somatic":false},
"parts":[{"type":"damage","when":"on_cast","dice":"8d6","damageType":"fire","half":true},{"type":"condition","condition":"prone"},
{"type":"surface","surface":"grease","rounds":3},{"type":"manual","text":"` + manual + `"}]}`
}

func rows(t *testing.T, raw string) spellbuild.Design {
	t.Helper()
	var d spellbuild.Design
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// A design saved as rows of the builder's own part types, every type among them, builds the same Effect.
func TestDesignsSavedAsRowsStillBuild(t *testing.T) {
	t.Parallel()
	failed := func(ability, condition string) effects.Part {
		return effects.Branch{When: effects.Condition{Kind: effects.FailsBy, N: 1, Type: ""}, Then: []effects.Part{effects.SaveCondition{Ability: ability, Slug: condition}}}
	}
	b, err := spellbuild.Build("hb-lantern", "Marsh Lantern", rows(t, lanternRows), surfaces)
	if err != nil {
		t.Fatal(err)
	}
	want := []effects.Part{
		effects.Area{Shape: hex.EmanationArea, SizeFt: 10, RangeFt: 0},
		effects.AreaSave{Ability: "wisdom"},
		effects.Light{BrightFt: 20, DimFt: 20},
		effects.Reveal{Qualities: []string{"invisible"}},
		effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: "undead"}, Then: []effects.Part{failed("wisdom", "charmed")}},
		effects.Branch{When: effects.Condition{Kind: effects.CreatureIs, N: 0, Type: "fey"}, Then: []effects.Part{failed("wisdom", "charmed")}},
		effects.CreateSurface{Surface: "hb-lantern-ground", Rounds: 10},
	}
	if !reflect.DeepEqual(b.Definition.Parts, want) || b.Surface == nil || *b.Surface != (spellbuild.Surface{Slug: "hb-lantern-ground", Name: "Marsh Lantern", Dice: "1d6", Type: "radiant"}) {
		t.Fatalf("the lantern = %+v %+v", b.Definition.Parts, b.Surface)
	}
	long := strings.Repeat("m", 500)
	b, err = spellbuild.Build("hb-burst", "Burst", rows(t, burstRows(long)), surfaces)
	if err != nil {
		t.Fatal(err)
	}
	want = []effects.Part{
		effects.Area{Shape: hex.SphereArea, SizeFt: 20, RangeFt: 150},
		effects.SaveDamage{Ability: "dexterity", Dice: "8d6", Type: "fire", Half: true},
		failed("dexterity", "prone"),
		effects.CreateSurface{Surface: "grease", Rounds: 3},
		effects.Manual{Instruction: long},
	}
	if !reflect.DeepEqual(b.Definition.Parts, want) || b.Surface != nil {
		t.Fatalf("the burst = %+v", b.Definition.Parts)
	}
}

// A row of a type the builder never had, and a design that is not JSON of a design, do not read as a spell.
func TestRowsThatNeverBuilt(t *testing.T) {
	t.Parallel()
	odd := rows(t, `{"targeting":{"shape":"sphere","sizeFt":20,"rangeFt":0},"save":"wisdom","duration":{"unit":"instant"},"castingTime":{"kind":"action"},
"components":{"verbal":true},"parts":[{"type":"song"},{"type":"damage","when":"later","dice":"2d6","damageType":"fire"}]}`)
	if odd.Parts[0].Kind != "song" || odd.Parts[1].Kind != "save_damage" {
		t.Fatalf("odd rows read as %+v", odd.Parts)
	}
	if _, err := spellbuild.Build("hb-odd", "Odd", odd, surfaces); err == nil || !strings.Contains(err.Error(), "a spell holds Parts of the kinds") {
		t.Fatalf("a song = %v", err)
	}
	for _, raw := range []string{`{"parts":"many"}`, `{"parts":[1]}`, `[]`} {
		var d spellbuild.Design
		if err := json.Unmarshal([]byte(raw), &d); err == nil {
			t.Errorf("%s read as %+v", raw, d)
		}
	}
	if d := rows(t, `{"parts":[]}`); d.Parts == nil || d.TurnDamage != nil {
		t.Fatalf("no parts = %+v", d)
	}
	authored := rows(t, `{"parts":[{"kind":"light","values":{"bright_ft":{"number":5}},"groups":[]}],"turnDamage":{"dice":"1d4","damageType":"acid"}}`)
	if len(authored.Parts) != 1 || authored.Parts[0].Kind != "light" || authored.Parts[0].Values["bright_ft"].Number != 5 || authored.TurnDamage.Dice != "1d4" {
		t.Fatalf("Parts authored by kind read as %+v", authored)
	}
}
