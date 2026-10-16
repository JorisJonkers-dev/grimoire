//go:build integration

package httpapi_test

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"testing"

	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
)

var update = flag.Bool("update", false, "rewrite the builder form fixture the web tests read")

const formFixture = "../../../../fixtures/part-kinds.json"

// Each builder serves what it takes as Parts, from the same lists it checks a design against, in every
// answer it gives. The kinds served and what both builders hold are the fixture the web's form tests
// read, so the form is tested against what the server serves.
func TestBuildersServeWhatTheyHold(t *testing.T) {
	t.Parallel()
	h, pool := armouryStack(t)
	surfaces, err := playpg.New(pool).SurfaceKinds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	spell := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"spell","name":"Marsh Lantern","fields":[]}`))["id"].(string)
	cond := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"condition","name":"Frostbite","fields":[]}`))["id"].(string)
	spellHolds, condHolds := holdsJSON(spellbuild.Holds(surfaces)), holdsJSON(conditionbuild.Holds())
	activation := activationJSON()
	for what, rec := range map[string]struct {
		got  map[string]any
		want map[string]any
	}{
		"spell":             {decode(t, call(h, http.MethodGet, "/api/v1/builders/spells/"+spell, "dm", "")), spellHolds},
		"spell saved":       {decode(t, call(h, http.MethodPut, "/api/v1/builders/spells/"+spell, "dm", lanternDesign)), spellHolds},
		"spell preview":     {decode(t, call(h, http.MethodPost, "/api/v1/builders/spells/preview", "dm", `{"name":"X","design":`+lanternDesign+`}`)), spellHolds},
		"condition":         {decode(t, call(h, http.MethodGet, "/api/v1/builders/conditions/"+cond, "dm", "")), condHolds},
		"condition saved":   {decode(t, call(h, http.MethodPut, "/api/v1/builders/conditions/"+cond, "dm", frostbiteDesign)), condHolds},
		"condition preview": {decode(t, call(h, http.MethodPost, "/api/v1/builders/conditions/preview", "dm", `{"name":"X","design":`+frostbiteDesign+`}`)), condHolds},
	} {
		sameJSON(t, what, rec.got["holds"], mustJSON(t, rec.want))
		if _, spelled := rec.got["activation"]; spelled || what[:5] == "spell" {
			sameJSON(t, what+" activation", rec.got["activation"], mustJSON(t, activation))
		}
	}

	var kinds any
	if err := json.Unmarshal(call(h, http.MethodGet, "/api/v1/builders/part-kinds", "dm", "").Body.Bytes(), &kinds); err != nil {
		t.Fatal(err)
	}
	served, err := json.MarshalIndent(map[string]any{
		"kinds": kinds, "holds": map[string]any{"spell": spellHolds, "condition": condHolds}, "activation": map[string]any{"spell": activation},
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(formFixture, append(served, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(formFixture)
	if err != nil {
		t.Fatal(err)
	}
	if string(fixture) != string(served)+"\n" {
		t.Error("fixtures/part-kinds.json is not what is served; run go test ./internal/platform/httpapi -run TestBuildersServeWhatTheyHold -update")
	}
}

// activationJSON is what a spell's own Scaling grows, each field marked as the kinds' are.
func activationJSON() []map[string]any {
	fields, scales := spellbuild.Activation()
	out := make([]map[string]any, 0, len(fields))
	for _, f := range fields {
		options := f.Options
		if options == nil {
			options = []string{}
		}
		formula := false
		for _, s := range scales {
			formula = formula || (s.Field == f.Name && s.Formula)
		}
		out = append(out, map[string]any{
			"name": f.Name, "label": f.Label, "type": string(f.Type), "options": options, "required": f.Required, "min": f.Min, "max": f.Max,
			"scalable": true, "formula": formula,
		})
	}
	return out
}

func holdsJSON(h effects.Holds) map[string]any {
	narrows := make([]map[string]any, 0, len(h.Narrows))
	for _, n := range h.Narrows {
		narrows = append(narrows, map[string]any{"kind": n.Kind, "field": n.Field, "options": n.Options})
	}
	return map[string]any{"kinds": h.Kinds, "beneath": h.Beneath, "fills": h.Fills, "narrows": narrows}
}
