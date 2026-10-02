package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func TestRollsOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/rolls"
	body := `{"purpose":"Attack","notation":"2d20kh1+1d4","labels":[{"group":1,"label":"Bless"}],"modifiers":[{"label":"Strength","value":3}]}`
	rec := call(h, http.MethodPost, base, "player", body)
	roll := decode(t, rec)
	rollID, _ := roll["id"].(string)
	groups, _ := roll["groups"].([]any)
	if rec.Code != http.StatusCreated || roll["mine"] != true || roll["canRoll"] != true || len(groups) != 2 ||
		!strings.Contains(rec.Body.String(), `"keep":"highest","keepCount":1`) || !strings.Contains(rec.Body.String(), `"label":"Bless"`) {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	one := base + "/" + rollID
	if rec := call(h, http.MethodPost, one+"/dice/0", "player", `{"mode":"manual","value":18}`); rec.Code != 200 ||
		!strings.Contains(rec.Body.String(), `"value":18,"mode":"manual"`) {
		t.Fatalf("manual: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodPost, one+"/dice/1", "player", `{"mode":"manual","value":21}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("face 21: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, one+"/dice/1", "player", `{"mode":"auto"}`); rec.Code != 200 {
		t.Fatalf("auto: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, one+"/rest", "player", "")
	done := decode(t, rec)
	if rec.Code != 200 || done["status"] != "resolved" || done["total"] == nil || done["resolvedAt"] == nil {
		t.Fatalf("rest: %d %s", rec.Code, rec.Body.String())
	}
	checkRollReads(t, h, id, rollID)
}

func checkRollReads(t *testing.T, h http.Handler, id, rollID string) {
	t.Helper()
	base := "/api/v1/campaigns/" + id + "/rolls"
	one := base + "/" + rollID
	if rec := call(h, http.MethodGet, one, "dm", ""); rec.Code != 200 || decode(t, rec)["canRoll"] != true {
		t.Fatalf("dm get: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, base+"?limit=5", "player", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), rollID) {
		t.Fatalf("list: %d", rec.Code)
	}
	rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/log", "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"kind":"die_rolled"`) || !strings.Contains(rec.Body.String(), `"seed":"`) {
		t.Fatalf("log: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/log", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player log: %d", rec.Code)
	}
	home := decode(t, call(h, http.MethodGet, "/api/v1/campaigns/"+id, "dm", ""))
	members, _ := home["members"].([]any)
	playerRow, _ := members[1].(map[string]any)
	playerID, _ := playerRow["id"].(string)
	ask := `{"purpose":"Stealth","notation":"1d20","rollerId":"` + playerID + `"}`
	if rec := call(h, http.MethodPost, base, "dm", ask); rec.Code != http.StatusCreated || decode(t, rec)["mine"] != false {
		t.Fatalf("dm asks player: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, base, "player", `{"purpose":"x","notation":"1d7"}`); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "there is no d7") {
		t.Fatalf("bad notation: %d %s", rec.Code, rec.Body.String())
	}
}

type brokenRolls struct{ err error }

func (b brokenRolls) Create(context.Context, caller.Caller, uuid.UUID, playapp.RollInput) (playdomain.Roll, error) {
	return playdomain.Roll{}, b.err
}

func (b brokenRolls) Get(context.Context, caller.Caller, uuid.UUID, playdomain.RollID) (playdomain.Roll, error) {
	return playdomain.Roll{}, b.err
}

func (b brokenRolls) List(context.Context, caller.Caller, uuid.UUID, int) ([]playdomain.Roll, error) {
	return nil, b.err
}

func (b brokenRolls) Log(context.Context, caller.Caller, uuid.UUID, int) ([]playdomain.Action, error) {
	return nil, b.err
}

func (b brokenRolls) SetDie(context.Context, caller.Caller, uuid.UUID, playdomain.RollID, int, playapp.Fill) (playdomain.Roll, error) {
	return playdomain.Roll{}, b.err
}

func (b brokenRolls) RollRest(context.Context, caller.Caller, uuid.UUID, playdomain.RollID) (playdomain.Roll, error) {
	return playdomain.Roll{}, b.err
}

func (b brokenRolls) Keep(context.Context, caller.Caller, uuid.UUID, playdomain.RollID) (playdomain.Roll, error) {
	return playdomain.Roll{}, b.err
}

func (b brokenRolls) Reroll(context.Context, caller.Caller, uuid.UUID, playdomain.RollID, int) (playdomain.Roll, error) {
	return playdomain.Roll{}, b.err
}

func TestRollErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.RollService(brokenRolls{err: errors.New("disk")}))
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	one := c + "/rolls/0190c7a8-0000-7000-8000-000000000002"
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, c + "/rolls", ""},
		{http.MethodPost, c + "/rolls", `{"purpose":"x","notation":"1d20"}`},
		{http.MethodGet, one, ""},
		{http.MethodPost, one + "/dice/0", `{"mode":"auto"}`},
		{http.MethodPost, one + "/rest", ""},
		{http.MethodPost, one + "/keep", ""},
		{http.MethodPost, one + "/reroll", `{"die":0}`},
		{http.MethodGet, c + "/log", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.ListRolls(ctx, oas.ListRollsParams{}))
	add(hh.CreateRoll(ctx, &oas.RollCreate{}, oas.CreateRollParams{}))
	add(hh.GetRoll(ctx, oas.GetRollParams{}))
	add(hh.SetDie(ctx, &oas.DieFill{}, oas.SetDieParams{}))
	add(hh.RollRest(ctx, oas.RollRestParams{}))
	add(hh.KeepRoll(ctx, oas.KeepRollParams{}))
	add(hh.RerollDie(ctx, &oas.RerollIn{}, oas.RerollDieParams{}))
	add(hh.GetActionLog(ctx, oas.GetActionLogParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
