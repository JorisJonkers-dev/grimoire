package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// Every kind the rules know is served as its definition, in the rules' order, field for field; a kind
// missing from what is served, or served differently from its definition, fails here.
func TestEveryKindOfPartIsServed(t *testing.T) {
	t.Parallel()
	h := libraryStack(t)
	if rec := call(h, http.MethodGet, "/api/v1/builders/part-kinds", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("an anonymous caller: %d", rec.Code)
	}
	rec := call(h, http.MethodGet, "/api/v1/builders/part-kinds", "dm", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("part kinds: %d %s", rec.Code, rec.Body.String())
	}
	var served []any
	if err := json.Unmarshal(rec.Body.Bytes(), &served); err != nil {
		t.Fatal(err)
	}
	kinds := effects.Kinds()
	if len(served) != len(kinds) {
		t.Fatalf("served %d kinds of %d", len(served), len(kinds))
	}
	for i, k := range kinds {
		d := k.Describe()
		fields := make([]map[string]any, 0, len(d.Fields))
		for _, f := range d.Fields {
			options := f.Options
			if options == nil {
				options = []string{}
			}
			fields = append(fields, map[string]any{
				"name": f.Name, "label": f.Label, "type": string(f.Type), "options": options, "required": f.Required, "min": f.Min, "max": f.Max,
			})
		}
		groups := d.Groups
		if groups == nil {
			groups = []string{}
		}
		want := map[string]any{"slug": d.Slug, "label": d.Label, "summary": d.Summary, "fields": fields, "groups": groups, "openGroups": d.OpenGroups}
		sameJSON(t, d.Slug, served[i], mustJSON(t, want))
	}
}
