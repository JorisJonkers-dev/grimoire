package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestHexPreviews(t *testing.T) {
	t.Parallel()
	h := compendiumServer(t, &fakeCompendium{})
	cells := `[{"q":0,"r":0},{"q":1,"r":0,"difficult":true},{"q":2,"r":0},{"q":1,"r":-1},{"q":2,"r":-1},{"q":0,"r":-1,"blocked":true},
{"q":-1,"r":0,"elevationFt":10},{"q":-1,"r":1,"cover":"half","blocksSight":false},{"q":0,"r":1,"cover":"three_quarters"}]`
	body := `{"cells":` + cells + `,"occupants":[{"q":1,"r":-1,"side":"ally"},{"q":2,"r":-1,"side":"enemy"}],"from":{"q":0,"r":0},"to":{"q":2,"r":0},"speedFt":15}`
	rec := call(h, http.MethodPost, "/api/v1/rules/hex/reach", "u", body)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"pathCostFt":15`) || !strings.Contains(rec.Body.String(), `"q":1,"r":-1,"costFt":5,"canEnd":false`) ||
		strings.Contains(rec.Body.String(), `"q":2,"r":-1,"costFt"`) {
		t.Fatalf("reach: %d %s", rec.Code, rec.Body.String())
	}
	climb := strings.Replace(body, `"speedFt":15`, `"speedFt":5,"climbSpeed":true`, 1)
	rec = call(h, http.MethodPost, "/api/v1/rules/hex/reach", "u", climb)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"q":-1,"r":0,"costFt":5`) || strings.Contains(rec.Body.String(), `"path"`) {
		t.Fatalf("climb: %d %s", rec.Code, rec.Body.String())
	}
	for _, tc := range []struct{ to, want string }{
		{`{"q":-2,"r":2}`, `"visible":true,"cover":"half","acBonus":2`},
		{`{"q":0,"r":2}`, `"visible":true,"cover":"three_quarters","acBonus":5`},
		{`{"q":3,"r":-2}`, `"visible":true,"cover":"half"`},
		{`{"q":2,"r":0}`, `"visible":true,"cover":"none","acBonus":0`},
	} {
		sight := `{"cells":` + cells + `,"occupants":[{"q":1,"r":-1,"side":"ally"},{"q":2,"r":-1,"side":"enemy"}],"from":{"q":0,"r":0},"to":` + tc.to + `}`
		if rec := call(h, http.MethodPost, "/api/v1/rules/hex/sight", "u", sight); rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("sight to %s: %d %s", tc.to, rec.Code, rec.Body.String())
		}
	}
	wall := `{"cells":[{"q":0,"r":0},{"q":1,"r":0,"cover":"total"},{"q":2,"r":0}],"from":{"q":0,"r":0},"to":{"q":2,"r":0}}`
	if rec := call(h, http.MethodPost, "/api/v1/rules/hex/sight", "u", wall); !strings.Contains(rec.Body.String(), `"visible":false,"cover":"total"`) {
		t.Fatalf("wall: %s", rec.Body.String())
	}
}
