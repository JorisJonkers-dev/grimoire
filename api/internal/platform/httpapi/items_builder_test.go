package httpapi_test

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	libraryapp "github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	librarypg "github.com/JorisJonkers-dev/grimoire/api/internal/library/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// armouryStack is the SRD stack with the Library, so homebrew items reach Inventories.
func armouryStack(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	sub, err := fs.Sub(db.Seeds, "seeds")
	if err != nil {
		t.Fatal(err)
	}
	snap, hash, err := snapshot.Load(sub)
	if err != nil {
		t.Fatal(err)
	}
	compendium := comppg.New(store.Pool())
	if _, err := compendium.Import(ctx, snap, hash); err != nil {
		t.Fatal(err)
	}
	repo := campaignpg.New(store.Pool())
	chars := &app.Characters{Repo: repo, Compendium: compendium, Combat: app.NoCombat{}, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now, Die: func(int) int { return 1 }}
	inv := &playapp.Inventories{Store: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Roll: func(count, _ int) int { return count }}
	lib := &libraryapp.Service{Repo: librarypg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo}, Now: time.Now, Surfaces: playpg.New(store.Pool()).SurfaceKinds}
	hooks := &app.RuleHooks{Repo: repo, Now: time.Now}
	h := campaignServer(t, app.NewService(repo), chars, httpapi.InventoryService(inv), httpapi.LibraryService(lib), httpapi.RuleHookService(hooks))
	return h, store.Pool()
}

const ashwoodDesign = `{"kind":"weapon","base":"longbow","rarity":"rare","enchantment":1,"weightLb":2,"valueGp":4000,"attunement":{},
"weapon":{"properties":["ammunition","heavy","two-handed"],"mastery":"slow"},"charges":{"max":3,"on":"dawn","dice":1,"faces":4},
"properties":[{"type":"cantrip","spell":"light","name":"Light"},{"type":"spell","spell":"hunters-mark","name":"Hunter's Mark","level":1,"cost":1},
{"type":"curse","text":"It whispers of the old wood.","hidden":true}]}`

// An author builds the Ashwood Longbow in the item builder; carried in a Campaign that links it, it casts
// Light at will and Hunter's Mark for a charge, once attuned.
func TestTheItemBuilderAndItsBow(t *testing.T) {
	t.Parallel()
	h, pool := armouryStack(t)
	campaign, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + campaign + "/characters"
	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	bow := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"item","name":"Ashwood Longbow","fields":[]}`))["id"].(string)
	npc := decode(t, call(h, http.MethodPost, "/api/v1/library", "dm", `{"kind":"npc","name":"Odo","fields":[]}`))["id"].(string)
	path := "/api/v1/builders/items/" + bow
	if fresh := decode(t, call(h, http.MethodGet, path, "dm", "")); fresh["design"].(map[string]any)["kind"] != "trinket" || !strings.HasPrefix(fresh["slug"].(string), "hb-") {
		t.Fatalf("a fresh item = %v", fresh)
	}
	rec := call(h, http.MethodPost, "/api/v1/builders/items/preview", "dm", `{"name":"Ashwood Longbow","design":`+ashwoodDesign+`}`)
	preview := decode(t, rec)
	card := strings.Join(toStrings(preview["card"].([]any)), "\n")
	price := preview["price"].(map[string]any)
	if rec.Code != http.StatusOK || !strings.Contains(card, "You can expend 1 charge to cast Hunter's Mark from it.") || !strings.Contains(card, "whispers") ||
		price["suggested"] != "rare" || price["fits"] != true {
		t.Fatalf("preview: %d %v", rec.Code, preview)
	}
	if rec := call(h, http.MethodPost, "/api/v1/builders/items/preview", "dm", `{"name":"X","design":{"kind":"spoon","rarity":"common","enchantment":0,"weightLb":0,"valueGp":0,"properties":[]}}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "choose a kind") {
		t.Fatalf("a spoon: %d %s", rec.Code, rec.Body.String())
	}
	for who, want := range map[string]int{"player": http.StatusNotFound} {
		if rec := call(h, http.MethodPut, path, who, ashwoodDesign); rec.Code != want {
			t.Fatalf("%s saves: %d", who, rec.Code)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		if rec := call(h, method, "/api/v1/builders/items/"+npc, "dm", ashwoodDesign); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("an NPC in the item builder (%s): %d", method, rec.Code)
		}
	}
	if rec := call(h, http.MethodPut, path, "dm", strings.Replace(ashwoodDesign, `"enchantment":1`, `"enchantment":4`, 1)); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("saving a +4 bow: %d", rec.Code)
	}
	rec = call(h, http.MethodPut, path, "dm", ashwoodDesign)
	saved := decode(t, rec)
	if rec.Code != http.StatusOK || saved["entry"].(map[string]any)["revision"] != float64(2) {
		t.Fatalf("save: %d %v", rec.Code, saved)
	}
	slug := saved["slug"].(string)
	call(h, http.MethodPost, "/api/v1/campaigns/"+campaign+"/library", "dm", `{"entryId":"`+bow+`"}`)
	call(h, http.MethodGet, kara+"/inventory", "player", "")
	if _, err := pool.Exec(context.Background(), `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, charges, identified, attuned, created_at)
		SELECT gen_random_uuid(), id, $2, 1, 3, true, false, now() FROM campaign.containers WHERE character_id = $1`, kara[strings.LastIndex(kara, "/")+1:], slug); err != nil {
		t.Fatal(err)
	}
	carried := func() map[string]any {
		t.Helper()
		for _, c := range decode(t, call(h, http.MethodGet, kara+"/inventory", "player", ""))["bag"].([]any) {
			if m := c.(map[string]any); m["slug"] == slug {
				return m
			}
		}
		t.Fatal("the bow is not in the bag")
		return nil
	}
	held := carried()
	if held["name"] != "Ashwood Longbow" || held["category"] != "weapon" || held["maxCharges"] != float64(3) || len(held["spells"].([]any)) != 2 ||
		!strings.Contains(strings.Join(toStrings(held["lines"].([]any)), "\n"), "Hunter's Mark") {
		t.Fatalf("the bow's card = %v", held)
	}
	id := held["instanceId"].(string)
	use := func(body string) *httptest.ResponseRecorder {
		return call(h, http.MethodPost, kara+"/inventory/use", "player", `{"instanceId":"`+id+`",`+body+`}`)
	}
	if rec := use(`"use":"cast","spell":"hunters-mark"`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "attune") {
		t.Fatalf("casting before attuning: %d %s", rec.Code, rec.Body.String())
	}
	if rec := use(`"use":"attune"`); rec.Code != http.StatusOK {
		t.Fatalf("attune: %d %s", rec.Code, rec.Body.String())
	}
	if rec := use(`"use":"cast","spell":"light"`); rec.Code != http.StatusOK || carried()["charges"] != float64(3) {
		t.Fatalf("Light at will spends nothing: %d", rec.Code)
	}
	for left := 2; left >= 0; left-- {
		if rec := use(`"use":"cast","spell":"hunters-mark"`); rec.Code != http.StatusOK || carried()["charges"] != float64(left) {
			t.Fatalf("Hunter's Mark spends a charge, %d left: %d", left, rec.Code)
		}
	}
	for body, want := range map[string]string{`"use":"cast","spell":"hunters-mark"`: "not that many charges", `"use":"cast","spell":"fireball"`: "grants no such spell"} {
		if rec := use(body); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/use", "player", `{"instanceId":"`+uuid.NewString()+`","use":"cast","spell":"light"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("casting from nothing: %d", rec.Code)
	}
}
