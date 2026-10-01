package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// liveStack is the API with MCP and a live hub over a real database, with goblins in the compendium.
func liveStack(t *testing.T) http.Handler {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	pool := store.Pool()
	compendium := comppg.New(pool)
	goblin := snapshot.Monster{
		Entry: snapshot.Entry{Document: "srd-2024", Slug: "goblin", Name: "Goblin"}, Size: "small", Type: "humanoid", Alignment: "neutral", ArmorClass: 12,
		HitPoints: 7, HitDice: "2d6", XP: 50, Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves: map[string]int{}, Skills: map[string]int{}, Speeds: map[string]int{"walk": 30}, Senses: map[string]int{}, Resistances: []string{}, Immunities: []string{},
		Vulnerabilities: []string{}, ConditionImmunities: []string{}, Traits: []snapshot.Named{}, Actions: []snapshot.Action{},
	}
	if _, err := compendium.Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Monsters:  []snapshot.Monster{goblin},
	}, "live"); err != nil {
		t.Fatal(err)
	}
	repo := campaignpg.New(pool)
	members := playpg.CampaignMembers{Store: repo}
	characters := &app.Characters{Repo: repo, Compendium: compendium, Combat: app.NoCombat{}, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now}
	hub := &live.Hub{
		Store: playpg.New(pool), Stats: playpg.Statblocks{Store: playpg.New(pool), Characters: characters}, Members: members, Owner: playpg.Owner{Pool: pool},
		Now: time.Now, Log: quiet,
	}
	t.Cleanup(hub.Shutdown)
	h, err := httpapi.New(httpapi.Options{
		Handler: &httpapi.Handler{
			Version: "1", Store: fakeStore{}, Compendium: compendium, Campaigns: app.NewService(repo), NPCs: &app.NPCs{Repo: repo, Now: time.Now},
			Characters: characters, Hub: hub, LiveMembers: members, Log: quiet,
			Sessions: &playapp.Sessions{Repo: playpg.New(pool), Members: members, Live: hub, Now: time.Now},
		},
		RateLimit: 1000, Now: time.Now, Edits: repo,
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestAnAgentRunsALiveSessionAndUndoesFromTheActionLog(t *testing.T) {
	t.Parallel()
	h := liveStack(t)
	id, _ := campaignWithPlayer(t, h)
	rec := call(h, http.MethodPost, "/api/v1/campaigns/"+id+"/sessions", "dm", "")
	sid, _ := decode(t, rec)["id"].(string)
	base := "/api/v1/campaigns/" + id + "/sessions/" + sid
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	dm := newAgent(t, srv.URL, "dm")
	where := map[string]any{"campaignId": id, "sessionId": sid}
	with := func(body map[string]any) map[string]any {
		return map[string]any{"campaignId": id, "sessionId": sid, "body": body}
	}
	var spawned struct {
		Seq       int   `json:"seq"`
		ActionSeq int64 `json:"actionSeq"`
	}
	out := dm.must("spawn_encounter", with(map[string]any{"monsters": []any{map[string]any{"monsterSlug": "goblin", "count": 2}}}))
	if err := json.Unmarshal(out.Result, &spawned); err != nil || spawned.ActionSeq < 1 {
		t.Fatalf("spawn: %s", out.Result)
	}
	var view struct {
		Tokens []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			HP    int    `json:"hp"`
		} `json:"tokens"`
	}
	state := dm.must("get_session_state", where)
	if err := json.Unmarshal(state.Result, &view); err != nil || len(view.Tokens) != 2 || view.Tokens[0].HP != 7 {
		t.Fatalf("state: %s", state.Result)
	}
	goblin := view.Tokens[0].ID
	hurt := dm.must("adjust_hp", with(map[string]any{"tokenId": goblin, "hpDelta": -3}))
	var adjusted struct {
		ActionSeq int64 `json:"actionSeq"`
	}
	_ = json.Unmarshal(hurt.Result, &adjusted)
	logged := dm.must("get_session_log", where)
	if !strings.Contains(string(logged.Result), `"kind":"hp_adjusted"`) || !strings.Contains(string(logged.Result), `"client":"prep-agent"`) {
		t.Fatalf("log: %s", logged.Result)
	}
	dm.must("undo_action", with(map[string]any{"seq": adjusted.ActionSeq}))
	if _, refused := dm.call("undo_action", with(map[string]any{"seq": adjusted.ActionSeq})); refused != "That action is already undone." {
		t.Fatalf("undo twice: %q", refused)
	}
	if _, refused := dm.call("reveal_area", with(map[string]any{"hexes": []any{map[string]any{"q": 0, "r": 0}}})); refused != "Choose a map first." {
		t.Fatalf("reveal without a map: %q", refused)
	}
	if _, refused := dm.call("run_encounter_check", with(map[string]any{"tableId": uuid.NewString()})); refused != "No such encounter table." {
		t.Fatalf("check without a table: %q", refused)
	}
	if _, refused := dm.call("spawn_encounter", with(map[string]any{"monsters": "lots"})); refused == "" {
		t.Fatal("a malformed spawn ran")
	}
	if _, refused := dm.call("spawn_encounter", map[string]any{"campaignId": id, "sessionId": sid, "body": []int{1}}); refused != "The body must be a JSON object." {
		t.Fatalf("array body: %q", refused)
	}
	dm.must("undo_action", with(map[string]any{"seq": spawned.ActionSeq}))
	if rec := call(h, http.MethodGet, base+"/view", "dm", ""); rec.Code != 200 || strings.Contains(rec.Body.String(), "Goblin") {
		t.Fatalf("after undoing the spawn: %d %s", rec.Code, rec.Body.String())
	}

	send := func(who, body string) *httptest.ResponseRecorder {
		return call(h, http.MethodPost, base+"/commands", who, body)
	}
	if rec := send("player", `{"nonce":"x","kind":"spawn_encounter","q":0,"r":0,"hidden":false,"monsters":[{"monsterSlug":"goblin","count":1}]}`); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "Only the DM can change the table.") {
		t.Fatalf("player spawn: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send("player", `{"nonce":"x","kind":"resync","q":0,"r":0,"hidden":false}`); rec.Code != 200 {
		t.Fatalf("resync: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send("dm", `{"nonce":"x","kind":"ping","q":1,"r":0,"hidden":false}`); rec.Code != 200 {
		t.Fatalf("ping: %d %s", rec.Code, rec.Body.String())
	}
	placed := send("dm", `{"nonce":"x","kind":"place_token","label":"Scout","tokenKind":"party","q":0,"r":0,"hidden":false}`)
	if placed.Code != 200 || decode(t, placed)["actionSeq"] == nil {
		t.Fatalf("place: %d %s", placed.Code, placed.Body.String())
	}
	var party struct {
		Tokens []struct{ ID string } `json:"tokens"`
	}
	_ = json.Unmarshal(call(h, http.MethodGet, base+"/view", "player", "").Body.Bytes(), &party)
	path := send("dm", `{"nonce":"x","kind":"plan_walk","tokenId":"`+party.Tokens[0].ID+`","q":1,"r":0,"hidden":false}`)
	if path.Code != 200 || !strings.Contains(path.Body.String(), `"costFt":5`) {
		t.Fatalf("plan walk: %d %s", path.Code, path.Body.String())
	}
	if rec := call(h, http.MethodGet, base+"/log?limit=2", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("player log: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, base+"/log?limit=2", "dm", ""); rec.Code != 200 || strings.Count(rec.Body.String(), `"seq"`) != 2 {
		t.Fatalf("log: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/campaigns/"+id+"/sessions/"+uuid.NewString()+"/log", "dm", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("log of no session: %d", rec.Code)
	}
	if rec := call(h, http.MethodGet, base+"/view", "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger view: %d", rec.Code)
	}
	call(h, http.MethodPost, base+"/end", "dm", "")
	if rec := send("dm", `{"nonce":"x","kind":"resync","q":0,"r":0,"hidden":false}`); rec.Code != http.StatusConflict {
		t.Fatalf("ended: %d %s", rec.Code, rec.Body.String())
	}
}

// quietHub joins without ever answering, or closes at once, or refuses.
type quietHub struct {
	closed, refuse bool
}

func (q quietHub) Join(context.Context, playdomain.SessionID, playdomain.Member, caller.Caller, live.Audience) (*live.Subscriber, error) {
	if q.refuse {
		return nil, live.ErrClosed
	}
	out := make(chan live.Update, 1)
	if q.closed {
		close(out)
	} else {
		out <- live.Update{Kind: live.UpdSnapshot}
	}
	return &live.Subscriber{Out: out}, nil
}
func (quietHub) Leave(*live.Subscriber)                {}
func (quietHub) Submit(*live.Subscriber, live.Command) {}

type liveSessions struct{ httpapi.SessionService }

func (liveSessions) Get(context.Context, caller.Caller, uuid.UUID, playdomain.SessionID) (playdomain.Session, error) {
	return playdomain.Session{Status: "live"}, nil
}

type anyMember struct{}

func (anyMember) Membership(context.Context, uuid.UUID, string) (playdomain.Member, error) {
	return playdomain.Member{DM: true}, nil
}

func TestLiveCallsGiveUpOnASilentSession(t *testing.T) {
	t.Parallel()
	one := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/sessions/0190c7a8-0000-7000-8000-000000000002"
	cmd := `{"nonce":"x","kind":"resync","q":0,"r":0,"hidden":false}`
	for _, hub := range []quietHub{{}, {closed: true}, {refuse: true}} {
		h, err := httpapi.New(httpapi.Options{
			Handler: &httpapi.Handler{
				Store: fakeStore{}, Campaigns: brokenCampaigns{}, Sessions: liveSessions{}, Hub: hub, LiveMembers: anyMember{}, LiveTimeout: 20 * time.Millisecond, Log: quiet,
			},
			RateLimit: 1000, Now: time.Now,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range []*httptest.ResponseRecorder{call(h, http.MethodPost, one+"/commands", "dm", cmd), call(h, http.MethodGet, one+"/view", "dm", "")} {
			if r.Code != http.StatusServiceUnavailable {
				t.Errorf("%+v: %d %s", hub, r.Code, r.Body.String())
			}
		}
	}
	hh := &httpapi.Handler{Log: quiet}
	ctx := context.Background()
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetSessionView(ctx, oas.GetSessionViewParams{}))
	add(hh.SendLiveCommand(ctx, &oas.LiveCommand{}, oas.SendLiveCommandParams{}))
	add(hh.GetSessionLog(ctx, oas.GetSessionLogParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
	broken := campaignServer(t, brokenCampaigns{}, httpapi.SessionService(brokenSessions{err: context.DeadlineExceeded}))
	if rec := call(broken, http.MethodGet, one+"/log", "dm", ""); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("broken log: %d", rec.Code)
	}
}
