//go:build integration

package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
// kit is Kara, a fighter in chain mail with a longsword, who took a shield, two potions of healing
// and a dagger from the stash, and Ines, the DM's scholar.
type kit struct {
	h              http.Handler
	pool           *pgxpool.Pool
	campaign, base string
	kara, karaID   string
	inesID         string
}

func newKit(t *testing.T) *kit {
	t.Helper()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	k := &kit{h: h, pool: pool, campaign: id, base: "/api/v1/campaigns/" + id + "/characters"}
	k.karaID = decode(t, call(h, http.MethodPost, k.base, "player", srdFighter))["id"].(string)
	k.kara = k.base + "/" + k.karaID
	k.inesID = decode(t, call(h, http.MethodPost, k.base, "dm", srdScholar))["id"].(string)
	stashItems(t, pool, id, map[string]int{"shield": 1, "potion-of-healing": 2, "dagger": 1})
	for _, take := range []string{`{"slug":"shield"}`, `{"slug":"potion-of-healing","count":2}`, `{"slug":"dagger"}`} {
		if rec := call(h, http.MethodPost, k.kara+"/inventory/take", "player", take); rec.Code != http.StatusOK {
			t.Fatalf("take %s: %d %s", take, rec.Code, rec.Body.String())
		}
	}
	return k
}

func (k *kit) move(t *testing.T, body string) map[string]any {
	t.Helper()
	rec := call(k.h, http.MethodPost, k.kara+"/inventory/move", "player", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("move %s: %d %s", body, rec.Code, rec.Body.String())
	}
	return decode(t, rec)
}

func (k *kit) inventory(t *testing.T) map[string]any {
	t.Helper()
	return decode(t, call(k.h, http.MethodGet, k.kara+"/inventory", "player", ""))
}

func (k *kit) sheet(t *testing.T) map[string]any {
	t.Helper()
	return decode(t, call(k.h, http.MethodGet, k.kara, "player", ""))
}

func (k *kit) armourOff(t *testing.T) map[string]any {
	t.Helper()
	return k.move(t, `{"instanceId":"`+instanceID(k.inventory(t), "armor")+`","to":"bag"}`)
}

// heirloom puts Grandmother's Dagger, a named dagger, in Kara's bag and returns its instance.
func (k *kit) heirloom(t *testing.T) string {
	t.Helper()
	named := "0190c7a8-0000-7000-8000-0000000000d1"
	if _, err := k.pool.Exec(context.Background(), `INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at)
		SELECT $2, id, 'dagger', 'Grandmother''s Dagger', 1, true, false, now() FROM campaign.containers WHERE character_id = $1`, k.karaID, named); err != nil {
		t.Fatal(err)
	}
	return named
}

func TestAStartingInventoryWearsItsGearWithinCapacity(t *testing.T) {
	t.Parallel()
	h, _ := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	kara := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	if inv := decode(t, call(h, http.MethodGet, kara+"/inventory", "player", "")); slotOf(inv, "armor") != "chain-mail" || slotOf(inv, "main_hand") != "longsword" || inv["weightLb"] != float64(58) || inv["capacityLb"] != float64(240) || inv["load"] != "none" {
		t.Fatalf("inventory = %v", inv)
	}
}

func TestAPlayerCannotOpenAnotherPlayersInventory(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if rec := call(k.h, http.MethodGet, k.base+"/"+k.inesID+"/inventory", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAShieldPutOnRaisesTheArmourClass(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if inv := k.move(t, `{"slug":"shield","to":"slot","slot":"off_hand"}`); slotOf(inv, "off_hand") != "shield" || bagCount(inv["stash"], "shield") != 0 {
		t.Fatalf("inventory = %v", inv)
	}
	if ac := k.sheet(t)["armorClass"]; ac != float64(18) {
		t.Fatalf("AC = %v", ac)
	}
}

func TestDrawingADaggerBagsTheSwordAndChangesTheAttacks(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if inv := k.move(t, `{"slug":"dagger","to":"slot","slot":"main_hand"}`); slotOf(inv, "main_hand") != "dagger" || bagCount(inv["bag"], "longsword") != 1 {
		t.Fatalf("inventory = %v", inv)
	}
	if attacks := k.sheet(t)["attacks"].([]any); len(attacks) != 1 || attacks[0].(map[string]any)["name"] != "Dagger" {
		t.Fatalf("attacks = %v", attacks)
	}
}

func TestTakingArmourOffLowersTheArmourClass(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	k.move(t, `{"slug":"shield","to":"slot","slot":"off_hand"}`)
	if inv := k.armourOff(t); slotOf(inv, "armor") != "" || bagCount(inv["bag"], "chain-mail") != 1 {
		t.Fatalf("inventory = %v", inv)
	}
	if ac := k.sheet(t)["armorClass"]; ac != float64(14) {
		t.Fatalf("AC = %v", ac)
	}
}

func TestAnInventoryMoveOutsideTheRulesIsRefused(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	k.move(t, `{"slug":"dagger","to":"slot","slot":"main_hand"}`)
	for body, want := range map[string]int{
		`{"slug":"potion-of-healing","to":"slot","slot":"armor"}`: http.StatusUnprocessableEntity,
		`{"slug":"longsword","to":"stash","count":5}`:             http.StatusUnprocessableEntity,
		`{"slug":"rope","to":"stash"}`:                            http.StatusNotFound,
		`{"slug":"longsword","to":"character"}`:                   http.StatusUnprocessableEntity,
	} {
		t.Run(body, func(t *testing.T) {
			if rec := call(k.h, http.MethodPost, k.kara+"/inventory/move", "player", body); rec.Code != want {
				t.Fatalf("%d", rec.Code)
			}
		})
	}
}

func TestADaggerDoesNotGoOnTheHead(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/move", "player", `{"slug":"dagger","to":"slot","slot":"head"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func TestDrinkingAPotionOfHealingHeals(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	call(k.h, http.MethodPatch, k.kara, "player", `{"damage":6}`)
	rec := call(k.h, http.MethodPost, k.kara+"/inventory/use", "player", `{"slug":"potion-of-healing","use":"drink"}`)
	if used := decode(t, rec); rec.Code != http.StatusOK || used["healed"] != float64(4) || bagCount(used["inventory"].(map[string]any)["bag"], "potion-of-healing") != 1 {
		t.Fatalf("drink: %d %v", rec.Code, used)
	}
	if hp := k.sheet(t)["hpCurrent"]; hp != float64(10) {
		t.Fatalf("hp = %v", hp)
	}
}

func TestOnlyAPotionIsDrunk(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	k.move(t, `{"slug":"dagger","to":"slot","slot":"main_hand"}`)
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/use", "player", `{"slug":"longsword","use":"drink"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAThrownItemLeavesTheInventory(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	k.move(t, `{"slug":"dagger","to":"slot","slot":"main_hand"}`)
	if used := decode(t, call(k.h, http.MethodPost, k.kara+"/inventory/use", "player", `{"slug":"longsword","use":"throw"}`)); bagCount(used["inventory"].(map[string]any)["bag"], "longsword") != 0 {
		t.Fatalf("thrown = %v", used)
	}
}

func TestAnUnknownUseIsABadRequest(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/use", "player", `{"slug":"dagger","use":"eat"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAGivenItemMovesToTheOtherCharactersBag(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	k.armourOff(t)
	if inv := k.move(t, `{"slug":"chain-mail","to":"character","characterId":"`+k.inesID+`"}`); bagCount(inv["bag"], "chain-mail") != 0 {
		t.Fatalf("Kara's bag = %v", inv["bag"])
	}
	if ines := decode(t, call(k.h, http.MethodGet, k.base+"/"+k.inesID+"/inventory", "dm", "")); bagCount(ines["bag"], "chain-mail") != 1 {
		t.Fatalf("Ines's bag = %v", ines["bag"])
	}
}

func TestAStashedShieldNoLongerCounts(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	k.armourOff(t)
	inv := k.move(t, `{"slug":"shield","to":"slot","slot":"off_hand"}`)
	if inv = k.move(t, `{"instanceId":"`+instanceID(inv, "off_hand")+`","to":"stash"}`); bagCount(inv["stash"], "shield") != 1 || k.sheet(t)["armorClass"] != float64(12) {
		t.Fatalf("inventory = %v", inv)
	}
}

func TestANamedItemIsDrawnAsItself(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	named := k.heirloom(t)
	if inv := k.move(t, `{"instanceId":"`+named+`","to":"slot","slot":"main_hand"}`); slotOf(inv, "main_hand") != "dagger" || instanceID(inv, "main_hand") != named {
		t.Fatalf("inventory = %v", inv)
	}
}

func TestANamedItemGoesEverywhereUntilItIsThrownAway(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	named := k.heirloom(t)
	for _, to := range []string{`"to":"slot","slot":"main_hand"`, `"to":"slot","slot":"off_hand"`, `"to":"bag"`, `"to":"stash"`} {
		k.move(t, `{"instanceId":"`+named+`",`+to+`}`)
	}
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/take", "player", `{"instanceId":"`+named+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("take it back: %d", rec.Code)
	}
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/use", "player", `{"instanceId":"`+named+`","use":"throw"}`); rec.Code != http.StatusOK {
		t.Fatalf("throw it: %d", rec.Code)
	}
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/move", "player", `{"instanceId":"`+named+`","to":"bag"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("move it after: %d", rec.Code)
	}
}

func TestTheInventoryOfNoCharacterIsNotFound(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if rec := call(k.h, http.MethodGet, k.base+"/0190c7a8-0000-7000-8000-0000000000ff/inventory", "player", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}

func TestTakingWhatTheStashLacksIsNotFound(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/take", "player", `{"slug":"rope"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("%d", rec.Code)
	}
}

func TestAHeavyLoadSlowsTheCharacter(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	stashItems(t, k.pool, k.campaign, map[string]int{"plate-armor": 6})
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/take", "player", `{"slug":"plate-armor","count":6}`); rec.Code != http.StatusOK {
		t.Fatalf("take plate: %d", rec.Code)
	}
	if speed := k.sheet(t)["speedFeet"]; speed != float64(5) {
		t.Fatalf("speed = %v", speed)
	}
}

func TestNothingMovesInAnInventoryDuringALiveSession(t *testing.T) {
	t.Parallel()
	k := newKit(t)
	if _, err := k.pool.Exec(context.Background(), "INSERT INTO play.sessions (campaign_id, number, status) VALUES ($1, 1, 'live')", k.campaign); err != nil {
		t.Fatal(err)
	}
	if rec := call(k.h, http.MethodPost, k.kara+"/inventory/move", "player", `{"slug":"dagger","to":"bag"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("%d", rec.Code)
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
// armedForBoth is Kara with a longsword in her melee set and a longbow and shield in her ranged set.
func armedForBoth(t *testing.T) (http.Handler, string, string) {
	t.Helper()
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
	return h, kara, ines
}

func fightsWith(t *testing.T, h http.Handler, kara, weapon string, ac float64) {
	t.Helper()
	sheet := decode(t, call(h, http.MethodGet, kara, "player", ""))
	if attacks := sheet["attacks"].([]any); len(attacks) != 1 || attacks[0].(map[string]any)["name"] != weapon || sheet["armorClass"] != ac {
		t.Fatalf("fights with %v at AC %v, want %s at %v", sheet["attacks"], sheet["armorClass"], weapon, ac)
	}
}

func swapTo(t *testing.T, h http.Handler, kara, want string) {
	t.Helper()
	rec := call(h, http.MethodPost, kara+"/inventory/swap", "player", "")
	if inv := decode(t, rec); rec.Code != http.StatusOK || inv["weaponSet"] != want {
		t.Fatalf("swap to %s: %d %v", want, rec.Code, inv["weaponSet"])
	}
}

func TestACharacterFightsWithTheMeleeSetFirst(t *testing.T) {
	t.Parallel()
	h, kara, _ := armedForBoth(t)
	fightsWith(t, h, kara, "Longsword", 16)
}

func TestSwappingWeaponSetsChangesTheAttacksAndArmourClass(t *testing.T) {
	t.Parallel()
	h, kara, _ := armedForBoth(t)
	swapTo(t, h, kara, "ranged")
	fightsWith(t, h, kara, "Longbow", 18)
	swapTo(t, h, kara, "melee")
	fightsWith(t, h, kara, "Longsword", 16)
}

func TestAPlayerCannotSwapAnotherPlayersWeapons(t *testing.T) {
	t.Parallel()
	h, _, ines := armedForBoth(t)
	if rec := call(h, http.MethodPost, ines+"/inventory/swap", "player", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("%d", rec.Code)
	}
}
