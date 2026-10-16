package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
)

const previewCells = `[{"q":0,"r":0},{"q":1,"r":0,"difficult":true},{"q":2,"r":0},{"q":1,"r":-1},{"q":2,"r":-1},{"q":0,"r":-1,"blocked":true},
{"q":-1,"r":0,"elevationFt":10},{"q":-1,"r":1,"cover":"half","blocksSight":false},{"q":0,"r":1,"cover":"three_quarters"}]`

const previewReach = `{"cells":` + previewCells + `,"occupants":[{"q":1,"r":-1,"side":"ally"},{"q":2,"r":-1,"side":"enemy"}],"from":{"q":0,"r":0},"to":{"q":2,"r":0},"speedFt":15}`

func TestAReachPreviewCostsThePathAndShowsWhereTheMoverMayStop(t *testing.T) {
	t.Parallel()
	rec := call(compendiumServer(t, &fakeCompendium{}), http.MethodPost, "/api/v1/rules/hex/reach", "u", previewReach)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"pathCostFt":15`) || !strings.Contains(rec.Body.String(), `"q":1,"r":-1,"costFt":5,"canEnd":false`) ||
		strings.Contains(rec.Body.String(), `"q":2,"r":-1,"costFt"`) {
		t.Fatalf("reach: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAClimberReachesHigherGroundAtItsNormalCost(t *testing.T) {
	t.Parallel()
	climb := strings.Replace(previewReach, `"speedFt":15`, `"speedFt":5,"climbSpeed":true`, 1)
	rec := call(compendiumServer(t, &fakeCompendium{}), http.MethodPost, "/api/v1/rules/hex/reach", "u", climb)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"q":-1,"r":0,"costFt":5`) || strings.Contains(rec.Body.String(), `"path"`) {
		t.Fatalf("climb: %d %s", rec.Code, rec.Body.String())
	}
}

func TestASightPreviewNamesTheCoverAndItsBonus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ to, want string }{
		{`{"q":-2,"r":2}`, `"visible":true,"cover":"half","acBonus":2`},
		{`{"q":0,"r":2}`, `"visible":true,"cover":"three_quarters","acBonus":5`},
		{`{"q":3,"r":-2}`, `"visible":true,"cover":"half"`},
		{`{"q":2,"r":0}`, `"visible":true,"cover":"none","acBonus":0`},
	} {
		t.Run(tc.to, func(t *testing.T) {
			sight := `{"cells":` + previewCells + `,"occupants":[{"q":1,"r":-1,"side":"ally"},{"q":2,"r":-1,"side":"enemy"}],"from":{"q":0,"r":0},"to":` + tc.to + `}`
			if rec := call(compendiumServer(t, &fakeCompendium{}), http.MethodPost, "/api/v1/rules/hex/sight", "u", sight); rec.Code != 200 || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("%d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAWallBlocksSightCompletely(t *testing.T) {
	t.Parallel()
	wall := `{"cells":[{"q":0,"r":0},{"q":1,"r":0,"cover":"total"},{"q":2,"r":0}],"from":{"q":0,"r":0},"to":{"q":2,"r":0}}`
	if rec := call(compendiumServer(t, &fakeCompendium{}), http.MethodPost, "/api/v1/rules/hex/sight", "u", wall); !strings.Contains(rec.Body.String(), `"visible":false,"cover":"total"`) {
		t.Fatalf("wall: %s", rec.Body.String())
	}
}
