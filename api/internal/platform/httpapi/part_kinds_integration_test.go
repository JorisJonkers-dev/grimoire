//go:build integration

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// Every kind the rules know is served as its definition and its hints, in the rules' order, field for
// field with whether it scales and may follow a formula; a kind missing from what is served, or served
// differently from its definition, fails here.
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
		scales := map[string]effects.Scalable{}
		for _, s := range effects.ScalesOf(k) {
			scales[s.Field] = s
		}
		for _, f := range d.Fields {
			options := f.Options
			if options == nil {
				options = []string{}
			}
			s, scalable := scales[f.Name]
			fields = append(fields, map[string]any{
				"name": f.Name, "label": f.Label, "type": string(f.Type), "options": options, "required": f.Required, "min": f.Min, "max": f.Max,
				"scalable": scalable, "formula": s.Formula,
			})
		}
		groups := d.Groups
		if groups == nil {
			groups = []string{}
		}
		want := map[string]any{
			"slug": d.Slug, "label": d.Label, "summary": d.Summary, "fields": fields, "groups": groups, "openGroups": d.OpenGroups,
			"hints": hintsJSON(effects.HintsOf(k)),
		}
		sameJSON(t, d.Slug, served[i], mustJSON(t, want))
	}
}

func hintsJSON(h effects.Hints) map[string]any {
	sections := make([]map[string]any, 0, len(h.Sections))
	for _, s := range h.Sections {
		sections = append(sections, map[string]any{"label": s.Label, "fields": s.Fields})
	}
	help := make([]map[string]any, 0, len(h.Help))
	for _, l := range h.Help {
		help = append(help, map[string]any{"field": l.Field, "text": l.Text})
	}
	order := h.Order
	if order == nil {
		order = []string{}
	}
	return map[string]any{"order": order, "sections": sections, "help": help}
}
