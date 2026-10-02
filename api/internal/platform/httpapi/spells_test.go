package httpapi_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const srdWizard = `{"name":"Mira","species":"human","class":"wizard","background":"sage","method":"point-buy",
"base":{"strength":8,"dexterity":14,"constitution":13,"intelligence":15,"wisdom":12,"charisma":8},
"bonus":{"intelligence":2,"constitution":1},"skills":["investigation","medicine"],"armor":"","shield":false,"weapons":[]}`

const srdDruid = `{"name":"Oak","species":"human","class":"druid","background":"sage","method":"point-buy",
"base":{"strength":8,"dexterity":14,"constitution":13,"intelligence":12,"wisdom":15,"charisma":8},
"bonus":{"wisdom":2,"constitution":1},"skills":["nature","survival"],"armor":"","shield":false,"weapons":[]}`

const srdBard = `{"name":"Lute","species":"human","class":"bard","background":"sage","method":"point-buy",
"base":{"strength":8,"dexterity":14,"constitution":13,"intelligence":12,"wisdom":8,"charisma":15},
"bonus":{"intelligence":2,"constitution":1},"skills":["performance","persuasion","deception"],"armor":"","shield":false,"weapons":[]}`

func spellSlugs(list any) []string {
	var out []string
	for _, s := range list.([]any) {
		out = append(out, s.(map[string]any)["slug"].(string))
	}
	return out
}

func casting(t *testing.T, h http.Handler, path string) map[string]any {
	t.Helper()
	rec := call(h, http.MethodGet, path+"/spells", "player", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("spells: %d %s", rec.Code, rec.Body.String())
	}
	return decode(t, rec)
}

func firstClass(sc map[string]any) map[string]any {
	return sc["classes"].([]any)[0].(map[string]any)
}

func giveGold(t *testing.T, pool *pgxpool.Pool, campaign, character string, gp int) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, character_id, label, created_at)
		VALUES (gen_random_uuid(), $1, 'character', $2, 'Pack', now()) ON CONFLICT DO NOTHING`, campaign, character); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO campaign.container_coins (container_id, coin, amount)
		SELECT id, 'gp', $2 FROM campaign.containers WHERE character_id = $1`, character, gp); err != nil {
		t.Fatal(err)
	}
}

// A wizard fills its spellbook for free up to its allotment, then pays 50 gp and 2 hours a spell level;
// it prepares from the book within its limit once per long rest, and casts rituals from the book.
func TestWizardSpellbookAndRituals(t *testing.T) {
	t.Parallel()
	h, pool := srdStack(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	chID := decode(t, call(h, http.MethodPost, base, "player", srdWizard))["id"].(string)
	path := base + "/" + chID

	sc := casting(t, h, path)
	wizard := firstClass(sc)
	if copyable := spellSlugs(wizard["copyable"]); len(copyable) < 20 || strings.Contains(strings.Join(copyable, ","), "fireball") {
		t.Fatalf("copyable = %v", copyable)
	}
	if wizard["limit"] != float64(4) || wizard["allotment"] != float64(6) || wizard["keepsSpellbook"] != true || sc["canPrepare"] != true || len(wizard["options"].([]any)) != 0 {
		t.Fatalf("new wizard = %v", sc)
	}
	for _, spell := range []string{"magic-missile", "shield", "detect-magic", "find-familiar", "sleep", "mage-armor"} {
		if rec := call(h, http.MethodPost, path+"/spellbook", "player", `{"spell":"`+spell+`"}`); rec.Code != http.StatusOK {
			t.Fatalf("copy %s: %d %s", spell, rec.Code, rec.Body.String())
		}
	}
	for body, why := range map[string]string{`{"spell":"magic-missile"}`: "twice", `{"spell":"fireball"}`: "too high", `{"spell":"cure-wounds"}`: "not wizard"} {
		if rec := call(h, http.MethodPost, path+"/spellbook", "player", body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("copy %s: %d", why, rec.Code)
		}
	}
	rec := call(h, http.MethodPost, path+"/spellbook", "player", `{"spell":"comprehend-languages"}`)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "costs 50 gold pieces") {
		t.Fatalf("copy without gold: %d %s", rec.Code, rec.Body.String())
	}
	giveGold(t, pool, id, chID, 60)
	sc = decode(t, call(h, http.MethodPost, path+"/spellbook", "player", `{"spell":"comprehend-languages"}`))
	purse := sc["purse"].([]any)
	if len(spellSlugs(firstClass(sc)["spellbook"])) != 7 || len(purse) != 1 || purse[0].(map[string]any)["coin"] != "pp" || purse[0].(map[string]any)["count"] != float64(1) {
		t.Fatalf("paid copy = %v", sc)
	}
	if clock := sc["clock"].(map[string]any); clock["minute"] != float64(120) || clock["day"] != float64(0) {
		t.Fatalf("clock after copying = %v", clock)
	}

	for body, why := range map[string]string{
		`{"class":"wizard","spells":["magic-missile","shield","sleep","mage-armor","detect-magic"]}`: "over the limit",
		`{"class":"wizard","spells":["burning-hands"]}`:                                              "not in the book",
		`{"class":"wizard","spells":["magic-missile","magic-missile"]}`:                              "twice",
		`{"class":"fighter","spells":[]}`:                                                            "not a caster",
	} {
		if rec := call(h, http.MethodPut, path+"/spells/prepared", "player", body); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("prepare %s: %d", why, rec.Code)
		}
	}
	sc = decode(t, call(h, http.MethodPut, path+"/spells/prepared", "player", `{"class":"wizard","spells":["magic-missile","shield","sleep","mage-armor"]}`))
	if got := spellSlugs(firstClass(sc)["prepared"]); len(got) != 4 || sc["canPrepare"] != false || len(spellSlugs(firstClass(sc)["spellbook"])) != 7 {
		t.Fatalf("prepared = %v %v", got, sc["canPrepare"])
	}
	if rec := call(h, http.MethodPut, path+"/spells/prepared", "player", `{"class":"wizard","spells":["shield"]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("preparing twice without a rest: %d", rec.Code)
	}

	ritual := decode(t, call(h, http.MethodPost, path+"/spells/rituals", "player", `{"spell":"find-familiar"}`))
	if ritual["minutes"] != float64(70) || ritual["clock"].(map[string]any)["minute"] != float64(190) {
		t.Fatalf("ritual = %v", ritual)
	}
	for spell, why := range map[string]string{"magic-missile": "not a ritual", "alarm": "not known"} {
		if rec := call(h, http.MethodPost, path+"/spells/rituals", "player", `{"spell":"`+spell+`"}`); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("ritual %s: %d", why, rec.Code)
		}
	}
	fighter := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	if sc := casting(t, h, fighter); len(sc["classes"].([]any)) != 0 {
		t.Fatalf("a fighter's spells = %v", sc)
	}
	if rec := call(h, http.MethodPost, fighter+"/spellbook", "player", `{"spell":"shield"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a fighter's spellbook: %d", rec.Code)
	}
}

// A druid prepares from its whole list around the spell it always has prepared, and casts that spell as
// a ritual; a bard swaps only one spell when it gains a level.
func TestPreparedCastersAndLevelSwaps(t *testing.T) {
	t.Parallel()
	h := srdCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	druid := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdDruid))["id"].(string)
	cls := firstClass(casting(t, h, druid))
	if got := spellSlugs(cls["always"]); len(got) != 1 || got[0] != "speak-with-animals" || strings.Contains(strings.Join(spellSlugs(cls["options"]), ","), "speak-with-animals") {
		t.Fatalf("druid always = %v", cls)
	}
	sc := decode(t, call(h, http.MethodPut, druid+"/spells/prepared", "player", `{"class":"druid","spells":["cure-wounds","entangle","goodberry","healing-word"]}`))
	if len(spellSlugs(firstClass(sc)["prepared"])) != 4 {
		t.Fatalf("druid prepared = %v", sc)
	}
	if r := decode(t, call(h, http.MethodPost, druid+"/spells/rituals", "player", `{"spell":"speak-with-animals"}`)); r["minutes"] != float64(10) {
		t.Fatalf("always-prepared ritual = %v", r)
	}

	bard := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdBard))["id"].(string)
	options := spellSlugs(firstClass(casting(t, h, bard))["options"])
	first := `{"class":"bard","spells":["` + strings.Join(options[:4], `","`) + `"]}`
	if rec := call(h, http.MethodPut, bard+"/spells/prepared", "player", first); rec.Code != http.StatusOK {
		t.Fatalf("bard prepares: %d %s", rec.Code, rec.Body.String())
	}
	unlock(t, h, bard)
	plan := decode(t, call(h, http.MethodGet, bard+"/level-up", "player", ""))
	var spell string
	for _, s := range plan["spellList"].([]any) {
		if sp := s.(map[string]any); sp["level"] == float64(1) {
			spell = sp["slug"].(string)
			break
		}
	}
	expertise := []string{}
	for slug := range choiceOptions(plan, "expertise") {
		expertise = append(expertise, `"`+slug+`"`)
	}
	levelUp(t, h, bard, `{"class":"bard","picks":[{"choice":"expertise","values":[`+strings.Join(expertise[:2], ",")+`]}],"spells":["`+spell+`"]}`)
	prepared := spellSlugs(firstClass(casting(t, h, bard))["prepared"])
	var spare []string
	for _, o := range options {
		if !strings.Contains(strings.Join(prepared, ","), o) {
			spare = append(spare, o)
		}
	}
	two := `{"class":"bard","spells":["` + strings.Join(append(prepared[2:], spare[:2]...), `","`) + `"]}`
	if rec := call(h, http.MethodPut, bard+"/spells/prepared", "player", two); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a bard swapping two: %d", rec.Code)
	}
	one := `{"class":"bard","spells":["` + strings.Join(append(prepared[1:], spare[0]), `","`) + `"]}`
	if rec := call(h, http.MethodPut, bard+"/spells/prepared", "player", one); rec.Code != http.StatusOK {
		t.Fatalf("a bard swapping one: %d %s", rec.Code, rec.Body.String())
	}
}
