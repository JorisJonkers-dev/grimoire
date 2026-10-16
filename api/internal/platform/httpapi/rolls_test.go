package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

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

// karmicRolls answers every read with one resolved roll whose first d20 was karmic.
type karmicRolls struct{ brokenRolls }

func (karmicRolls) Get(_ context.Context, _ caller.Caller, campaign uuid.UUID, id playdomain.RollID) (playdomain.Roll, error) {
	let := 6
	return playdomain.Roll{
		ID: id, CampaignID: campaign, Purpose: "Save", Notation: "1d20+1d4", Status: playdomain.StatusResolved, Total: 17,
		Dice: []playdomain.Die{
			{No: 0, Group: 0, Faces: 20, Value: 14, Mode: playdomain.ModeAuto, Kept: false, KarmicDropped: &let},
			{No: 1, Group: 1, Faces: 4, Value: 3, Mode: playdomain.ModeAuto, Kept: false, KarmicDropped: nil},
		},
	}, nil
}

// A karmic d20 says on the wire which face it let go; a die that is not karmic says nothing of it.
func TestAKarmicDieSaysWhatItLetGo(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.RollService(karmicRolls{brokenRolls: brokenRolls{err: nil}}))
	rec := call(h, http.MethodGet, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/rolls/0190c7a8-0000-7000-8000-000000000002", "u", "")
	dice, _ := decode(t, rec)["dice"].([]any)
	if rec.Code != 200 || len(dice) != 2 {
		t.Fatalf("roll: %d %s", rec.Code, rec.Body.String())
	}
	karmic, plain := dice[0].(map[string]any), dice[1].(map[string]any)
	if karmic["karmicDropped"] != float64(6) || karmic["value"] != float64(14) {
		t.Fatalf("karmic die = %v", karmic)
	}
	if _, said := plain["karmicDropped"]; said {
		t.Fatalf("plain die = %v", plain)
	}
}
