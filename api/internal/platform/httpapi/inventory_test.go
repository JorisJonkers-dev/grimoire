package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func stashItems(t *testing.T, pool *pgxpool.Pool, campaign string, items map[string]int) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, label, created_at)
		SELECT gen_random_uuid(), $1, 'party_stash', 'Party Stash', now()
		WHERE NOT EXISTS (SELECT 1 FROM campaign.containers WHERE campaign_id = $1 AND kind = 'party_stash')`, campaign); err != nil {
		t.Fatal(err)
	}
	for slug, n := range items {
		if _, err := pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
			SELECT gen_random_uuid(), id, $2, $3, true, false, now() FROM campaign.containers WHERE campaign_id = $1 AND kind = 'party_stash'`, campaign, slug, n); err != nil {
			t.Fatal(err)
		}
	}
}

func slotOf(inv map[string]any, slot string) string {
	for _, s := range inv["slots"].([]any) {
		line := s.(map[string]any)
		if line["slot"] == slot {
			if item, ok := line["item"].(map[string]any); ok {
				return item["slug"].(string)
			}
		}
	}
	return ""
}

func bagCount(list any, slug string) float64 {
	for _, c := range list.([]any) {
		if card := c.(map[string]any); card["slug"] == slug {
			return card["quantity"].(float64)
		}
	}
	return 0
}

func instanceID(inv map[string]any, slot string) string {
	for _, s := range inv["slots"].([]any) {
		if line := s.(map[string]any); line["slot"] == slot {
			return line["item"].(map[string]any)["instanceId"].(string)
		}
	}
	return ""
}

// A Character's starting gear sits in its slots; taking, equipping and removing items keeps the sheet's
// armor and weapons in step; potions heal; gear goes to allies and the Party Stash; too much slows it.
func TestInventoryScreen(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	inesID := decode(t, call(h, http.MethodPost, base, "dm", srdScholar))["id"].(string)
	stashItems(t, pool, id, map[string]int{"shield": 1, "potion-of-healing": 2, "dagger": 1})

	inv := decode(t, call(h, http.MethodGet, kara+"/inventory", "player", ""))
	if slotOf(inv, "armor") != "chain-mail" || slotOf(inv, "main_hand") != "longsword" || inv["weightLb"] != float64(58) || inv["capacityLb"] != float64(240) || inv["load"] != "none" {
		t.Fatalf("starting inventory = %v", inv)
	}
	if rec := call(h, http.MethodGet, base+"/"+inesID+"/inventory", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("another player's inventory: %d", rec.Code)
	}
	move := func(body string) map[string]any {
		t.Helper()
		rec := call(h, http.MethodPost, kara+"/inventory/move", "player", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("move %s: %d %s", body, rec.Code, rec.Body.String())
		}
		return decode(t, rec)
	}
	for _, take := range []string{`{"slug":"shield"}`, `{"slug":"potion-of-healing","count":2}`, `{"slug":"dagger"}`} {
		if rec := call(h, http.MethodPost, kara+"/inventory/take", "player", take); rec.Code != http.StatusOK {
			t.Fatalf("take %s: %d %s", take, rec.Code, rec.Body.String())
		}
	}
	inv = move(`{"slug":"shield","to":"slot","slot":"off_hand"}`)
	if slotOf(inv, "off_hand") != "shield" || bagCount(inv["stash"], "shield") != 0 {
		t.Fatalf("shield on = %v", inv)
	}
	if ac := decode(t, call(h, http.MethodGet, kara, "player", ""))["armorClass"]; ac != float64(18) {
		t.Fatalf("AC with a shield = %v", ac)
	}
	inv = move(`{"slug":"dagger","to":"slot","slot":"main_hand"}`)
	if slotOf(inv, "main_hand") != "dagger" || bagCount(inv["bag"], "longsword") != 1 {
		t.Fatalf("dagger drawn = %v", inv)
	}
	sheet := decode(t, call(h, http.MethodGet, kara, "player", ""))
	if attacks := sheet["attacks"].([]any); len(attacks) != 1 || attacks[0].(map[string]any)["name"] != "Dagger" {
		t.Fatalf("attacks = %v", sheet["attacks"])
	}
	inv = move(`{"instanceId":"` + instanceID(inv, "armor") + `","to":"bag"}`)
	if slotOf(inv, "armor") != "" || bagCount(inv["bag"], "chain-mail") != 1 {
		t.Fatalf("armor off = %v", inv)
	}
	if ac := decode(t, call(h, http.MethodGet, kara, "player", ""))["armorClass"]; ac != float64(14) {
		t.Fatalf("AC without armor = %v", ac)
	}
	for body, want := range map[string]int{
		`{"slug":"potion-of-healing","to":"slot","slot":"armor"}`: http.StatusUnprocessableEntity,
		`{"slug":"longsword","to":"stash","count":5}`:             http.StatusUnprocessableEntity,
		`{"slug":"rope","to":"stash"}`:                            http.StatusNotFound,
		`{"slug":"longsword","to":"character"}`:                   http.StatusUnprocessableEntity,
	} {
		if rec := call(h, http.MethodPost, kara+"/inventory/move", "player", body); rec.Code != want {
			t.Fatalf("%s: %d", body, rec.Code)
		}
	}

	call(h, http.MethodPatch, kara, "player", `{"damage":6}`)
	rec := call(h, http.MethodPost, kara+"/inventory/use", "player", `{"slug":"potion-of-healing","use":"drink"}`)
	used := decode(t, rec)
	if rec.Code != http.StatusOK || used["healed"] != float64(4) || bagCount(used["inventory"].(map[string]any)["bag"], "potion-of-healing") != 1 {
		t.Fatalf("drink: %d %v", rec.Code, used)
	}
	if hp := decode(t, call(h, http.MethodGet, kara, "player", ""))["hpCurrent"]; hp != float64(10) {
		t.Fatalf("hp after a potion = %v", hp)
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/use", "player", `{"slug":"longsword","use":"drink"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("drinking a sword: %d", rec.Code)
	}
	used = decode(t, call(h, http.MethodPost, kara+"/inventory/use", "player", `{"slug":"longsword","use":"throw"}`))
	if bagCount(used["inventory"].(map[string]any)["bag"], "longsword") != 0 {
		t.Fatalf("thrown = %v", used)
	}

	inv = move(`{"slug":"chain-mail","to":"character","characterId":"` + inesID + `"}`)
	if bagCount(inv["bag"], "chain-mail") != 0 {
		t.Fatalf("gave chain mail = %v", inv)
	}
	ines := decode(t, call(h, http.MethodGet, base+"/"+inesID+"/inventory", "dm", ""))
	if bagCount(ines["bag"], "chain-mail") != 1 {
		t.Fatalf("Ines's bag = %v", ines["bag"])
	}
	inv = move(`{"instanceId":"` + instanceID(inv, "off_hand") + `","to":"stash"}`)
	if bagCount(inv["stash"], "shield") != 1 || decode(t, call(h, http.MethodGet, kara, "player", ""))["armorClass"] != float64(12) {
		t.Fatalf("shield stashed = %v", inv)
	}

	named := "0190c7a8-0000-7000-8000-0000000000d1"
	if _, err := pool.Exec(context.Background(), `INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at)
		SELECT $2, id, 'dagger', 'Grandmother''s Dagger', 1, true, false, now() FROM campaign.containers WHERE character_id = $1`, strings.TrimPrefix(kara, base+"/"), named); err != nil {
		t.Fatal(err)
	}
	inv = move(`{"instanceId":"` + named + `","to":"slot","slot":"main_hand"}`)
	if slotOf(inv, "main_hand") != "dagger" || bagCount(inv["bag"], "dagger") != 1 || instanceID(inv, "main_hand") != named {
		t.Fatalf("heirloom drawn = %v", inv)
	}
	move(`{"instanceId":"` + named + `","to":"slot","slot":"off_hand"}`)
	move(`{"instanceId":"` + named + `","to":"bag"}`)
	move(`{"instanceId":"` + named + `","to":"stash"}`)
	if rec := call(h, http.MethodPost, kara+"/inventory/take", "player", `{"instanceId":"`+named+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("take the heirloom back: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/use", "player", `{"instanceId":"`+named+`","use":"throw"}`); rec.Code != http.StatusOK {
		t.Fatalf("throw the heirloom: %d", rec.Code)
	}
	for path, want := range map[string]int{
		base + "/0190c7a8-0000-7000-8000-0000000000ff/inventory": http.StatusNotFound,
	} {
		if rec := call(h, http.MethodGet, path, "player", ""); rec.Code != want {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/move", "player", `{"instanceId":"`+named+`","to":"bag"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("a thrown heirloom: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/use", "player", `{"slug":"dagger","use":"eat"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("eating a dagger: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/take", "player", `{"slug":"rope"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("taking what the stash lacks: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/move", "player", `{"slug":"dagger","to":"slot","slot":"head"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a dagger on the head: %d", rec.Code)
	}
	stashItems(t, pool, id, map[string]int{"plate-armor": 6})
	if rec := call(h, http.MethodPost, kara+"/inventory/take", "player", `{"slug":"plate-armor","count":6}`); rec.Code != http.StatusOK {
		t.Fatalf("take plate: %d", rec.Code)
	}
	sheet = decode(t, call(h, http.MethodGet, kara, "player", ""))
	if sheet["speedFeet"] != float64(5) {
		t.Fatalf("speed under 390 lb = %v", sheet["speedFeet"])
	}
	if _, err := pool.Exec(context.Background(), "INSERT INTO play.sessions (campaign_id, number, status) VALUES ($1, 1, 'live')", id); err != nil {
		t.Fatal(err)
	}
	if rec := call(h, http.MethodPost, kara+"/inventory/move", "player", `{"slug":"plate-armor","to":"stash","count":1}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("moving during a live Session: %d", rec.Code)
	}
}

type brokenInventory struct{ err error }

func (b brokenInventory) View(context.Context, caller.Caller, uuid.UUID, uuid.UUID) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Move(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemMove) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Take(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemRef, int) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func (b brokenInventory) Use(context.Context, caller.Caller, uuid.UUID, uuid.UUID, playapp.ItemRef, string, int) (playapp.InventoryView, int, error) {
	return playapp.InventoryView{}, 0, b.err
}

func (b brokenInventory) Swap(context.Context, caller.Caller, uuid.UUID, uuid.UUID) (playapp.InventoryView, error) {
	return playapp.InventoryView{}, b.err
}

func TestInventoryErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, brokenCampaigns{}, httpapi.InventoryService(brokenInventory{err: errors.New("disk")}))
	one := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/characters/0190c7a8-0000-7000-8000-000000000002/inventory"
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, one, ""},
		{http.MethodPost, one + "/move", `{"slug":"rope","to":"stash"}`},
		{http.MethodPost, one + "/take", `{"instanceId":"0190c7a8-0000-7000-8000-000000000003"}`},
		{http.MethodPost, one + "/use", `{"slug":"rope","use":"throw"}`},
		{http.MethodPost, one + "/swap", ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: %d", o.method, o.path, rec.Code)
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetInventory(ctx, oas.GetInventoryParams{}))
	add(hh.MoveItem(ctx, &oas.InventoryMove{}, oas.MoveItemParams{}))
	add(hh.TakeFromStash(ctx, &oas.InventoryTake{}, oas.TakeFromStashParams{}))
	add(hh.UseItem(ctx, &oas.InventoryUse{}, oas.UseItemParams{}))
	add(hh.SwapWeaponSet(ctx, oas.SwapWeaponSetParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}

func cardOf(list any, slug string) map[string]any {
	for _, c := range list.([]any) {
		if card := c.(map[string]any); card["slug"] == slug {
			return card
		}
	}
	return nil
}

// Attunement takes at most three items and checks who may attune; an unidentified item shows only its
// kind to players until studied; charges are spent from the item.
func TestAttunementIdentificationAndCharges(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	stashItems(t, pool, id, map[string]int{
		"amulet-of-health": 1, "amulet-of-the-planes": 1, "belt-of-dwarvenkind": 1, "belt-of-giant-strength-hill": 1,
		"staff-of-healing": 1, "wand-of-magic-missiles": 1,
	})
	use := func(body string, want int) map[string]any {
		t.Helper()
		rec := call(h, http.MethodPost, kara+"/inventory/use", "player", body)
		if rec.Code != want {
			t.Fatalf("use %s: %d %s", body, rec.Code, rec.Body.String())
		}
		if want != http.StatusOK {
			return nil
		}
		return decode(t, rec)["inventory"].(map[string]any)
	}
	for _, slug := range []string{"amulet-of-health", "amulet-of-the-planes", "belt-of-dwarvenkind", "belt-of-giant-strength-hill", "staff-of-healing", "wand-of-magic-missiles"} {
		if rec := call(h, http.MethodPost, kara+"/inventory/take", "player", `{"slug":"`+slug+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("take %s: %d", slug, rec.Code)
		}
	}
	use(`{"slug":"wand-of-magic-missiles","use":"attune"}`, http.StatusUnprocessableEntity)
	rec := call(h, http.MethodPost, kara+"/inventory/use", "player", `{"slug":"staff-of-healing","use":"attune"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Bard, Cleric, or Druid") {
		t.Fatalf("a fighter attuning a healer's staff: %d %s", rec.Code, rec.Body.String())
	}
	var inv map[string]any
	for _, slug := range []string{"amulet-of-health", "amulet-of-the-planes", "belt-of-dwarvenkind"} {
		inv = use(`{"slug":"`+slug+`","use":"attune"}`, http.StatusOK)
	}
	amulet := cardOf(inv["bag"], "amulet-of-health")
	if amulet["attuned"] != true {
		t.Fatalf("attuned = %v", amulet)
	}
	use(`{"slug":"belt-of-giant-strength-hill","use":"attune"}`, http.StatusUnprocessableEntity)
	inv = use(`{"instanceId":"`+amulet["instanceId"].(string)+`","use":"unattune"}`, http.StatusOK)
	if card := cardOf(inv["bag"], "amulet-of-health"); card["attuned"] != false || card["instanceId"] != nil {
		t.Fatalf("unattuned amulet = %v", card)
	}
	use(`{"slug":"belt-of-giant-strength-hill","use":"attune"}`, http.StatusOK)

	inv = use(`{"slug":"wand-of-magic-missiles","use":"charge","count":2}`, http.StatusOK)
	wand := cardOf(inv["bag"], "wand-of-magic-missiles")
	if wand["charges"] != float64(5) || wand["maxCharges"] != float64(7) {
		t.Fatalf("wand after two charges = %v", wand)
	}
	use(`{"instanceId":"`+wand["instanceId"].(string)+`","use":"charge","count":6}`, http.StatusUnprocessableEntity)
	use(`{"slug":"staff-of-healing","use":"identify"}`, http.StatusUnprocessableEntity)

	if _, err := pool.Exec(context.Background(), `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
		SELECT gen_random_uuid(), id, 'wand-of-fireballs', 1, false, false, now() FROM campaign.containers WHERE character_id = $1`, strings.TrimPrefix(kara, base+"/")); err != nil {
		t.Fatal(err)
	}
	playerView := call(h, http.MethodGet, kara+"/inventory", "player", "").Body.String()
	if strings.Contains(playerView, "fireball") || strings.Contains(playerView, "Fireball") || !strings.Contains(playerView, "Unknown wand") {
		t.Fatalf("a player learns what an unidentified wand is: %s", playerView)
	}
	dmView := decode(t, call(h, http.MethodGet, kara+"/inventory", "dm", ""))
	mystery := cardOf(dmView["bag"], "wand-of-fireballs")
	if mystery == nil || mystery["identified"] != false || mystery["name"] != "Wand of Fireballs" {
		t.Fatalf("the DM's view = %v", dmView["bag"])
	}
	use(`{"instanceId":"`+mystery["instanceId"].(string)+`","use":"attune"}`, http.StatusUnprocessableEntity)
	inv = use(`{"instanceId":"`+mystery["instanceId"].(string)+`","use":"identify"}`, http.StatusOK)
	if cardOf(inv["bag"], "wand-of-fireballs") == nil {
		t.Fatalf("identified = %v", inv["bag"])
	}
	wandID := cardOf(inv["bag"], "wand-of-magic-missiles")["instanceId"].(string)
	beltID := ""
	for _, c := range inv["bag"].([]any) {
		if card := c.(map[string]any); card["slug"] == "belt-of-giant-strength-hill" {
			beltID = card["instanceId"].(string)
		}
	}
	for body, want := range map[string]int{
		`{"slug":"amulet-of-health","use":"charge"}`:       http.StatusUnprocessableEntity,
		`{"instanceId":"` + wandID + `","use":"unattune"}`: http.StatusUnprocessableEntity,
		`{"instanceId":"` + wandID + `","use":"identify"}`: http.StatusUnprocessableEntity,
		`{"instanceId":"` + beltID + `","use":"attune"}`:   http.StatusUnprocessableEntity,
		`{"slug":"amulet-of-health","use":"unattune"}`:     http.StatusUnprocessableEntity,
		`{"slug":"amulet-of-health","use":"juggle"}`:       http.StatusBadRequest,
	} {
		use(body, want)
	}
	stashItems(t, pool, id, map[string]int{"potion-of-healing": 2})
	call(h, http.MethodPost, kara+"/inventory/take", "player", `{"slug":"potion-of-healing","count":2}`)
	if _, err := pool.Exec(context.Background(), `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
		SELECT gen_random_uuid(), id, 'potion-of-healing', 1, false, false, now() FROM campaign.containers WHERE character_id = $1`, strings.TrimPrefix(kara, base+"/")); err != nil {
		t.Fatal(err)
	}
	dmView = decode(t, call(h, http.MethodGet, kara+"/inventory", "dm", ""))
	var unknownPotion string
	for _, c := range dmView["bag"].([]any) {
		if card := c.(map[string]any); card["slug"] == "potion-of-healing" && card["identified"] == false {
			unknownPotion = card["instanceId"].(string)
		}
	}
	named, sworn := uuid.New(), uuid.New()
	for _, q := range []struct {
		sql string
		id  uuid.UUID
	}{
		{`INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at)
			SELECT $2, id, 'dagger', 'Whisper', 1, false, false, now() FROM campaign.containers WHERE character_id = $1`, named},
		{`INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at)
			SELECT $2, id, 'amulet-of-health', 'Oath Amulet', 1, true, true, now() FROM campaign.containers WHERE character_id = $1`, sworn},
	} {
		if _, err := pool.Exec(context.Background(), q.sql, strings.TrimPrefix(kara, base+"/"), q.id); err != nil {
			t.Fatal(err)
		}
	}
	inv = use(`{"instanceId":"`+named.String()+`","use":"identify"}`, http.StatusOK)
	if cardOf(inv["bag"], "dagger")["customName"] != "Whisper" {
		t.Fatalf("an identified named dagger keeps its name = %v", inv["bag"])
	}
	if oath := cardOf(use(`{"instanceId":"`+sworn.String()+`","use":"unattune"}`, http.StatusOK)["bag"], "amulet-of-health"); oath == nil {
		t.Fatal("the oath amulet vanished")
	}
	inv = use(`{"instanceId":"`+unknownPotion+`","use":"identify"}`, http.StatusOK)
	if potions := cardOf(inv["bag"], "potion-of-healing"); potions["quantity"] != float64(3) || potions["instanceId"] != nil {
		t.Fatalf("an identified potion joins its stack = %v", potions)
	}
}

// A Character keeps a melee and a ranged weapon set; swapping between them changes the weapons and
// shield the sheet fights with.
func TestWeaponSets(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	ines := base + "/" + decode(t, call(h, http.MethodPost, base, "dm", srdScholar))["id"].(string)
	stashItems(t, pool, id, map[string]int{"shield": 1, "longbow": 1})
	for _, step := range []struct{ path, body string }{
		{"/take", `{"slug":"longbow"}`},
		{"/take", `{"slug":"shield"}`},
		{"/move", `{"slug":"longbow","to":"slot","slot":"ranged_main"}`},
		{"/move", `{"slug":"shield","to":"slot","slot":"ranged_off"}`},
	} {
		if rec := call(h, http.MethodPost, kara+"/inventory"+step.path, "player", step.body); rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", step.path, step.body, rec.Code, rec.Body.String())
		}
	}
	fights := func(weapon string, ac float64) {
		t.Helper()
		sheet := decode(t, call(h, http.MethodGet, kara, "player", ""))
		attacks := sheet["attacks"].([]any)
		if len(attacks) != 1 || attacks[0].(map[string]any)["name"] != weapon || sheet["armorClass"] != ac {
			t.Fatalf("fights with %s at AC %v = %v %v", weapon, ac, sheet["attacks"], sheet["armorClass"])
		}
	}
	fights("Longsword", 16)
	swap := func(want string) {
		t.Helper()
		rec := call(h, http.MethodPost, kara+"/inventory/swap", "player", "")
		if inv := decode(t, rec); rec.Code != http.StatusOK || inv["weaponSet"] != want {
			t.Fatalf("swap to %s: %d %v", want, rec.Code, inv["weaponSet"])
		}
	}
	swap("ranged")
	fights("Longbow", 18)
	swap("melee")
	fights("Longsword", 16)
	if rec := call(h, http.MethodPost, ines+"/inventory/swap", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("swapping another player's weapons: %d", rec.Code)
	}
}
