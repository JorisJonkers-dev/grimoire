//go:build integration

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func roster(t *testing.T, h http.Handler, campaign, name string) any {
	t.Helper()
	for _, c := range decodeList(t, call(h, http.MethodGet, "/api/v1/campaigns/"+campaign+"/characters", "player", "")) {
		if c["name"] == name {
			return c["heroicInspiration"]
		}
	}
	return nil
}

// rollD20 asks for a d20 the player throws, and rolls it.
func rollD20(t *testing.T, h http.Handler, rolls string) (string, map[string]any) {
	t.Helper()
	id, _ := decode(t, call(h, http.MethodPost, rolls, "player", `{"purpose":"Athletics","notation":"1d20"}`))["id"].(string)
	return rolls + "/" + id, decode(t, call(h, http.MethodPost, rolls+"/"+id+"/rest", "player", ""))
}

// The DM grants Heroic Inspiration; it shows on the sheet and in the roster. A roll by its holder waits
// to be kept or rerolled once, and a reroll spends it. It can also be passed to an ally that lacks it.
// inspiredTable is a Campaign with Kara, a player's Character, and Ines, the DM's.
type inspiredTable struct {
	h                            http.Handler
	id, chars, rolls, kara, ines string
}

func newInspiredTable(t *testing.T) *inspiredTable {
	t.Helper()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	i := &inspiredTable{h: h, id: id, chars: "/api/v1/campaigns/" + id + "/characters", rolls: "/api/v1/campaigns/" + id + "/rolls"}
	i.kara = i.chars + "/" + decode(t, call(h, http.MethodPost, i.chars, "player", build))["id"].(string)
	i.ines = decode(t, call(h, http.MethodPost, i.chars, "dm", strings.Replace(build, `"Kara"`, `"Ines"`, 1)))["id"].(string)
	return i
}

func (i *inspiredTable) inspire(t *testing.T) {
	t.Helper()
	if sheet := decode(t, call(i.h, http.MethodPatch, i.kara, "dm", `{"heroicInspiration":true}`)); sheet["heroicInspiration"] != true {
		t.Fatalf("granted = %v", sheet["heroicInspiration"])
	}
}

// held is a d20 Kara rolled with Heroic Inspiration, waiting for her choice.
func (i *inspiredTable) held(t *testing.T) string {
	t.Helper()
	i.inspire(t)
	one, _ := rollD20(t, i.h, i.rolls)
	return one
}

func (i *inspiredTable) pass(to string) *httptest.ResponseRecorder {
	return call(i.h, http.MethodPost, i.kara+"/inspiration/pass", "player", `{"to":"`+to+`"}`)
}

func TestARollWithoutHeroicInspirationResolvesAtOnce(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	if _, plain := rollD20(t, i.h, i.rolls); plain["status"] != "resolved" || plain["choosing"] != false {
		t.Fatalf("roll = %v", plain)
	}
}

func TestOnlyTheDMGrantsHeroicInspiration(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	if rec := call(i.h, http.MethodPatch, i.kara, "player", `{"heroicInspiration":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player granting it: %d", rec.Code)
	}
	i.inspire(t)
}

func TestTheRosterShowsWhoHasHeroicInspiration(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	i.inspire(t)
	if roster(t, i.h, i.id, "Kara") != true || roster(t, i.h, i.id, "Ines") != false {
		t.Fatal("the roster")
	}
}

func TestARollWithHeroicInspirationWaitsForAChoice(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	i.inspire(t)
	if _, held := rollD20(t, i.h, i.rolls); held["status"] != "pending" || held["choosing"] != true || held["total"] != nil {
		t.Fatalf("roll = %v", held)
	}
}

func TestTheDMCannotSpendAPlayersInspiration(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	if rec := call(i.h, http.MethodPost, i.held(t)+"/reroll", "dm", `{"die":0}`); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
	}
}

func TestRerollingADieThatIsNotThereIsNotFound(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	if rec := call(i.h, http.MethodPost, i.held(t)+"/reroll", "player", `{"die":3}`); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}

func TestARerollResolvesTheRollAndSpendsTheInspiration(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	rerolled := decode(t, call(i.h, http.MethodPost, i.held(t)+"/reroll", "player", `{"die":0}`))
	if rerolled["status"] != "resolved" || rerolled["rerolled"] != true || rerolled["choosing"] != false {
		t.Fatalf("rerolled = %v", rerolled)
	}
	if roster(t, i.h, i.id, "Kara") != false {
		t.Fatal("the Inspiration was not spent")
	}
}

func TestAResolvedRollIsNotKeptAgain(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	one := i.held(t)
	call(i.h, http.MethodPost, one+"/reroll", "player", `{"die":0}`)
	if rec := call(i.h, http.MethodPost, one+"/keep", "player", ""); rec.Code != http.StatusConflict {
		t.Fatalf("%d", rec.Code)
	}
}

func TestARerollIsInTheActionLog(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	rollD20(t, i.h, i.rolls)
	call(i.h, http.MethodPost, i.held(t)+"/reroll", "player", `{"die":0}`)
	if log := call(i.h, http.MethodGet, "/api/v1/campaigns/"+i.id+"/log?limit=10", "dm", "").Body.String(); strings.Count(log, `"kind":"die_rolled"`) < 3 {
		t.Fatalf("log = %s", log)
	}
}

func TestKeepingTheRollKeepsTheInspiration(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	if kept := decode(t, call(i.h, http.MethodPost, i.held(t)+"/keep", "player", "")); kept["status"] != "resolved" || kept["rerolled"] != false || roster(t, i.h, i.id, "Kara") != true {
		t.Fatalf("kept = %v", kept)
	}
}

func TestInspirationIsPassedOnlyToAnotherCharacterThatIsThere(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	i.inspire(t)
	for to, want := range map[string]int{strings.TrimPrefix(i.kara, i.chars+"/"): http.StatusUnprocessableEntity, "0190c7a8-0000-7000-8000-0000000000ff": http.StatusNotFound} {
		t.Run(to, func(t *testing.T) {
			if rec := i.pass(to); rec.Code != want {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
}

func TestTheDMCannotPassAPlayersInspiration(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	i.inspire(t)
	if rec := call(i.h, http.MethodPost, i.kara+"/inspiration/pass", "dm", `{"to":"`+i.ines+`"}`); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
	}
}

func TestPassingInspirationGivesItAway(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	i.inspire(t)
	if sheet := decode(t, i.pass(i.ines)); sheet["heroicInspiration"] != false || roster(t, i.h, i.id, "Ines") != true {
		t.Fatalf("passed = %v", sheet["heroicInspiration"])
	}
}

func TestInspirationNoLongerHeldIsNotPassed(t *testing.T) {
	t.Parallel()
	i := newInspiredTable(t)
	i.inspire(t)
	i.pass(i.ines)
	if rec := i.pass(i.ines); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func decodeList(t *testing.T, rec *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %d: %v", rec.Code, err)
	}
	return out
}
