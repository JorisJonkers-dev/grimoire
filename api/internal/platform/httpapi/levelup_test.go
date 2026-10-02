package httpapi_test

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// srdCampaigns is the Campaign API over the real SRD 5.2 compendium, with a Hit Die that always rolls 1.
func srdCampaigns(t *testing.T) http.Handler {
	t.Helper()
	h, _ := srdStack(t)
	return h
}

// srdStack is srdCampaigns with its database, for setting up what the API cannot.
func srdStack(t *testing.T) (http.Handler, *pgxpool.Pool) {
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
	chars := &app.Characters{
		Repo: repo, Compendium: compendium, Combat: app.NoCombat{}, Blobs: storage.Dir{Path: t.TempDir()}, Now: time.Now,
		Die: func(int) int { return 1 },
	}
	inv := &playapp.Inventories{
		Store: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: repo},
		Roll: func(count, _ int) int { return count },
	}
	return campaignServer(t, app.NewService(repo), chars, httpapi.InventoryService(inv)), store.Pool()
}

const srdFighter = `{"name":"Kara","species":"human","class":"fighter","background":"soldier","method":"point-buy",
"base":{"strength":15,"dexterity":14,"constitution":13,"intelligence":8,"wisdom":10,"charisma":10},
"bonus":{"strength":1,"dexterity":1,"constitution":1},"skills":["perception","survival"],"armor":"chain-mail","shield":false,"weapons":["longsword"]}`

const srdScholar = `{"name":"Ines","species":"human","class":"fighter","background":"soldier","method":"point-buy",
"base":{"strength":15,"dexterity":13,"constitution":13,"intelligence":13,"wisdom":10,"charisma":8},
"bonus":{"strength":2,"constitution":1},"skills":["perception","survival"],"armor":"","shield":false,"weapons":[]}`

func unlock(t *testing.T, h http.Handler, path string) {
	t.Helper()
	if rec := call(h, http.MethodPatch, path, "dm", `{"levelUpReady":true}`); rec.Code != http.StatusOK {
		t.Fatalf("unlock: %d %s", rec.Code, rec.Body.String())
	}
}

func levelUp(t *testing.T, h http.Handler, path, body string) map[string]any {
	t.Helper()
	rec := call(h, http.MethodPost, path+"/level-up", "player", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("level up %s: %d %s", body, rec.Code, rec.Body.String())
	}
	return decode(t, rec)
}

func choiceOptions(plan map[string]any, slug string) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, c := range plan["choices"].([]any) {
		ch, _ := c.(map[string]any)
		if ch["slug"] != slug {
			continue
		}
		for _, o := range ch["options"].([]any) {
			opt, _ := o.(map[string]any)
			out[opt["slug"].(string)] = opt
		}
	}
	return out
}

// A fighter levels up only once the DM or a rest unlocks it, taking the Hit Die's average or a roll,
// a subclass at 3 and an Ability Score Improvement at 4 that raises earlier levels' hit points too.
func TestLevelUpFromFeatureData(t *testing.T) {
	t.Parallel()
	h := srdCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	created := decode(t, call(h, http.MethodPost, base, "player", srdFighter))
	path := base + "/" + created["id"].(string)

	plan := decode(t, call(h, http.MethodGet, path+"/level-up", "player", ""))
	if plan["ready"] != false || plan["level"] != float64(2) || plan["average"] != float64(8) || plan["hitDie"] != float64(10) {
		t.Fatalf("locked plan = %v", plan)
	}
	if rec := call(h, http.MethodPost, path+"/level-up", "player", `{"class":"fighter"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("level up while locked: %d", rec.Code)
	}
	if rec := call(h, http.MethodPatch, path, "player", `{"levelUpReady":true}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a player unlocking: %d", rec.Code)
	}
	unlock(t, h, path)
	sheet := decode(t, call(h, http.MethodGet, path, "player", ""))
	if sheet["levelUpReady"] != true {
		t.Fatalf("unlocked sheet = %v", sheet["levelUpReady"])
	}
	sheet = levelUp(t, h, path, `{"class":"fighter","hitPoints":"average"}`)
	if sheet["level"] != float64(2) || sheet["hpMax"] != float64(20) || sheet["levelUpReady"] != false {
		t.Fatalf("level 2 = level %v hp %v ready %v", sheet["level"], sheet["hpMax"], sheet["levelUpReady"])
	}
	if rec := call(h, http.MethodPost, path+"/level-up", "player", `{"class":"fighter"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a second level from one unlock: %d", rec.Code)
	}

	unlock(t, h, path)
	plan = decode(t, call(h, http.MethodGet, path+"/level-up?class=fighter", "player", ""))
	if _, ok := choiceOptions(plan, "subclass")["champion"]; !ok {
		t.Fatalf("level 3 choices = %v", plan["choices"])
	}
	if rec := call(h, http.MethodPost, path+"/level-up", "player", `{"class":"fighter"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("skipping the subclass: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, path+"/level-up", "player", `{"class":"fighter","picks":[{"choice":"subclass","values":["thief"]}]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("another class's subclass: %d", rec.Code)
	}
	if rec := call(h, http.MethodPost, path+"/level-up", "player", `{"class":"fighter","picks":[{"choice":"subclass","values":["champion"]},{"choice":"extra","values":["x"]}]}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("an unknown choice: %d", rec.Code)
	}
	sheet = levelUp(t, h, path, `{"class":"fighter","hitPoints":"roll","picks":[{"choice":"subclass","values":["champion"]}]}`)
	classes, _ := sheet["classes"].([]any)
	fighter, _ := classes[0].(map[string]any)
	if sheet["hpMax"] != float64(23) || fighter["subclass"] != "champion" || fighter["level"] != float64(3) || !hasTrait(sheet, "Improved Critical") {
		t.Fatalf("level 3 = hp %v classes %v", sheet["hpMax"], classes)
	}

	unlock(t, h, path)
	plan = decode(t, call(h, http.MethodGet, path+"/level-up", "player", ""))
	feats := choiceOptions(plan, "feat")
	if _, ok := feats["ability-score-improvement"]; !ok || len(feats["grappler"]["unmet"].([]any)) != 0 {
		t.Fatalf("level 4 feats = %v", feats)
	}
	for _, bad := range []string{
		`{"class":"fighter","picks":[{"choice":"feat","values":["ability-score-improvement"]}],"increase":{"strength":2,"dexterity":1}}`,
		`{"class":"fighter","picks":[{"choice":"feat","values":["grappler"]}],"increase":{"strength":2}}`,
		`{"class":"fighter","picks":[{"choice":"feat","values":["alert"]}]}`,
	} {
		if rec := call(h, http.MethodPost, path+"/level-up", "player", bad); rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: %d", bad, rec.Code)
		}
	}
	sheet = levelUp(t, h, path, `{"class":"fighter","picks":[{"choice":"feat","values":["ability-score-improvement"]}],"increase":{"constitution":2}}`)
	if sheet["hpMax"] != float64(35) || score(sheet, "constitution") != 16 {
		t.Fatalf("level 4 = hp %v con %v", sheet["hpMax"], score(sheet, "constitution"))
	}
}

// A Character with 13 Intelligence multiclasses into wizard and writes six spells into its book; one
// without it is told what it lacks. A DM's hold shows on the plan, and a level the DM granted stays open.
func TestMulticlassLevelUp(t *testing.T) {
	t.Parallel()
	h := srdCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	fighter := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdFighter))["id"].(string)
	rec := call(h, http.MethodGet, fighter+"/level-up?class=wizard", "player", "")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Intelligence 13+ (wizard)") {
		t.Fatalf("multiclass without the score: %d %s", rec.Code, rec.Body.String())
	}

	scholar := base + "/" + decode(t, call(h, http.MethodPost, base, "player", srdScholar))["id"].(string)
	unlock(t, h, scholar)
	plan := decode(t, call(h, http.MethodGet, scholar+"/level-up?class=wizard", "player", ""))
	if plan["classLevel"] != float64(1) || plan["cantrips"] != float64(3) || plan["spells"] != float64(6) || plan["average"] != float64(6) || len(plan["choices"].([]any)) != 0 {
		t.Fatalf("wizard plan = %v", plan)
	}
	var cantrips, spells []string
	for _, s := range plan["spellList"].([]any) {
		sp, _ := s.(map[string]any)
		switch {
		case sp["level"] == float64(0) && len(cantrips) < 3:
			cantrips = append(cantrips, `"`+sp["slug"].(string)+`"`)
		case sp["level"] == float64(1) && len(spells) < 6:
			spells = append(spells, `"`+sp["slug"].(string)+`"`)
		case sp["level"].(float64) > 1:
			t.Fatalf("a level %v spell at wizard 1", sp["level"])
		}
	}
	short := `{"class":"wizard","spells":[` + strings.Join(cantrips, ",") + `]}`
	if rec := call(h, http.MethodPost, scholar+"/level-up", "player", short); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("too few spells: %d", rec.Code)
	}
	twice := `{"class":"wizard","spells":[` + strings.Join(append(cantrips, cantrips[0]), ",") + `]}`
	if rec := call(h, http.MethodPost, scholar+"/level-up", "player", twice); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a spell twice: %d", rec.Code)
	}
	sheet := levelUp(t, h, scholar, `{"class":"wizard","spells":[`+strings.Join(append(cantrips, spells...), ",")+`]}`)
	if len(sheet["classes"].([]any)) != 2 || len(sheet["spells"].([]any)) != 9 || sheet["hpMax"] != float64(18) {
		t.Fatalf("multiclassed = classes %v spells %v hp %v", sheet["classes"], sheet["spells"], sheet["hpMax"])
	}
	resources := map[string]float64{}
	for _, r := range sheet["resources"].([]any) {
		res, _ := r.(map[string]any)
		resources[res["label"].(string)] = res["max"].(float64)
	}
	if resources["Hit Dice (1d10, 1d6)"] != 2 || resources["Level 1 spell slots"] != 2 {
		t.Fatalf("resources = %v", resources)
	}

	unlock(t, h, scholar)
	plan = decode(t, call(h, http.MethodGet, scholar+"/level-up?class=wizard", "player", ""))
	learned := map[string]bool{}
	for _, s := range sheet["spells"].([]any) {
		learned[s.(map[string]any)["slug"].(string)] = true
	}
	for _, s := range plan["spellList"].([]any) {
		if sp := s.(map[string]any); learned[sp["slug"].(string)] || sp["level"] == float64(0) {
			t.Fatalf("wizard 2 offers %v again", sp["slug"])
		}
	}
	if plan["classLevel"] != float64(2) || plan["cantrips"] != float64(0) || plan["spells"] != float64(2) {
		t.Fatalf("wizard 2 plan = %v", plan)
	}

	unlock(t, h, fighter)
	plan = decode(t, call(h, http.MethodGet, fighter+"/level-up?class=rogue", "player", ""))
	skills, expertise := choiceOptions(plan, "skills"), choiceOptions(plan, "expertise")
	if _, ok := skills["stealth"]; !ok || len(skills) == 0 {
		t.Fatalf("rogue skills = %v", plan["choices"])
	}
	if _, ok := expertise["perception"]; !ok || expertise["stealth"] != nil {
		t.Fatalf("rogue expertise = %v", expertise)
	}
	sheet = levelUp(t, h, fighter, `{"class":"rogue","picks":[{"choice":"skills","values":["stealth"]},{"choice":"expertise","values":["perception","athletics"]}]}`)
	for _, s := range sheet["skills"].([]any) {
		sk := s.(map[string]any)
		if (sk["skill"] == "stealth" && sk["proficient"] != true) || (sk["skill"] == "perception" && sk["expertise"] != true) {
			t.Fatalf("rogue skills on the sheet = %v", sk)
		}
	}

	if rec := call(h, http.MethodPatch, "/api/v1/campaigns/"+id, "dm", `{"holdLevelUps":true}`); rec.Code != http.StatusOK || decode(t, rec)["holdLevelUps"] != true {
		t.Fatalf("hold: %d", rec.Code)
	}
	if plan := decode(t, call(h, http.MethodGet, scholar+"/level-up", "player", "")); plan["held"] != true || plan["ready"] != true {
		t.Fatalf("held plan = %v", plan)
	}
}

func hasTrait(sheet map[string]any, name string) bool {
	for _, t := range sheet["traits"].([]any) {
		if t.(map[string]any)["name"] == name {
			return true
		}
	}
	return false
}

func score(sheet map[string]any, ability string) float64 {
	for _, a := range sheet["abilities"].([]any) {
		if line := a.(map[string]any); line["ability"] == ability {
			return line["score"].(float64)
		}
	}
	return 0
}
