package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

const fumbleDesign = `{"dice":"1d6","results":[{"from":1,"to":1,"text":"You fall flat on your face.","effect":"prone"},
{"from":2,"to":3,"text":"Your weapon slips from your grip."},{"from":5,"to":6,"text":"You find a coin.","item":"gold-piece","quantity":2}]}`

// An author builds a fumble table in the Roll Table builder: a design the rules refuse is not saved,
// and only its owner saves it.
func TestTheRollTableBuilder(t *testing.T) {
	t.Parallel()
	h, _ := armouryStack(t)
	table := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"table","name":"Fumbles","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path, preview := "/api/v1/builders/roll-tables/"+table, "/api/v1/builders/roll-tables/preview"
	fresh := decode(t, call(h, http.MethodGet, path, "dm", ""))
	if design, _ := fresh["design"].(map[string]any); design["dice"] != "1d20" || len(design["results"].([]any)) != 0 || fresh["lines"].([]any)[0] != "Roll 1d20." || fresh["entry"].(map[string]any)["name"] != "Fumbles" {
		t.Fatalf("a fresh table = %v", fresh)
	}
	wrapped := func(design string) string { return `{"name":"Fumbles","design":` + design + `}` }
	rec := call(h, http.MethodPost, preview, "dm", wrapped(fumbleDesign))
	shown := decode(t, rec)
	if lines := toStrings(shown["lines"].([]any)); rec.Code != http.StatusOK || len(lines) != 4 || lines[1] != "1: You fall flat on your face. Applies prone." || lines[3] != "5-6: You find a coin. Gives 2 × gold-piece." || shown["entry"] != nil {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	overlapping := strings.Replace(fumbleDesign, `"from":2,"to":3`, `"from":1,"to":3`, 1)
	for name, c := range map[string]struct {
		who, method, path, body string
		want                    int
	}{
		"a preview of overlapping ranges": {"dm", http.MethodPost, preview, wrapped(overlapping), http.StatusUnprocessableEntity},
		"a preview with no results":       {"dm", http.MethodPost, preview, wrapped(`{"dice":"1d6","results":[]}`), http.StatusUnprocessableEntity},
		"a preview signed out":            {"", http.MethodPost, preview, wrapped(fumbleDesign), http.StatusUnauthorized},
		"a save of overlapping ranges":    {"dm", http.MethodPut, path, overlapping, http.StatusUnprocessableEntity},
		"an NPC in the builder":           {"dm", http.MethodGet, "/api/v1/builders/roll-tables/" + npc, "", http.StatusUnprocessableEntity},
		"an NPC saved as a table":         {"dm", http.MethodPut, "/api/v1/builders/roll-tables/" + npc, fumbleDesign, http.StatusUnprocessableEntity},
		"another's table opened":          {"player", http.MethodGet, path, "", http.StatusNotFound},
		"another's table saved":           {"player", http.MethodPut, path, fumbleDesign, http.StatusNotFound},
		"signed out":                      {"", http.MethodGet, path, "", http.StatusUnauthorized},
		"a save signed out":               {"", http.MethodPut, path, fumbleDesign, http.StatusUnauthorized},
	} {
		if rec := call(h, c.method, c.path, c.who, c.body); rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPost, preview, "dm", wrapped(overlapping)); !strings.Contains(rec.Body.String(), "do not overlap") {
		t.Errorf("the refusal says why: %s", rec.Body.String())
	}
	// A token that may only read previews and opens, and cannot save.
	if rec := withScopes(h, http.MethodPut, path, "dm", "read", fumbleDesign); rec.Code != http.StatusForbidden {
		t.Errorf("a read token saves: %d", rec.Code)
	}
	if rec := withScopes(h, http.MethodPost, preview, "dm", "read", wrapped(fumbleDesign)); rec.Code != http.StatusOK {
		t.Errorf("a read token previews: %d", rec.Code)
	}
	if still := decode(t, call(h, http.MethodGet, path, "dm", "")); len(still["design"].(map[string]any)["results"].([]any)) != 0 || still["entry"].(map[string]any)["revision"] != float64(1) {
		t.Fatalf("after the refusals = %v", still)
	}
	saved := decode(t, call(h, http.MethodPut, path, "dm", fumbleDesign))
	results, _ := saved["design"].(map[string]any)["results"].([]any)
	if saved["entry"].(map[string]any)["revision"] != float64(2) || len(results) != 3 || results[0].(map[string]any)["effect"] != "prone" || results[2].(map[string]any)["quantity"] != float64(2) || len(saved["lines"].([]any)) != 4 {
		t.Fatalf("save = %v", saved)
	}
	if again := decode(t, call(h, http.MethodGet, path, "dm", "")); again["design"].(map[string]any)["dice"] != "1d6" || again["lines"].([]any)[2] != "2-3: Your weapon slips from your grip." {
		t.Fatalf("read back = %v", again)
	}

	// Nobody signed out reaches a handler.
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results2 := []any{}
	add := func(res any, _ error) { results2 = append(results2, res) }
	add(hh.GetRollTableBuild(ctx, oas.GetRollTableBuildParams{}))
	add(hh.SaveRollTableBuild(ctx, &oas.RollTableDesign{}, oas.SaveRollTableBuildParams{}))
	add(hh.PreviewRollTable(ctx, &oas.RollTablePreviewInput{}))
	for i, r := range results2 {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
