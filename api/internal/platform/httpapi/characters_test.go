package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

const build = `{"name":"Kara","species":"human","class":"fighter","background":"soldier","method":"point-buy",
"base":{"strength":15,"dexterity":14,"constitution":13,"intelligence":8,"wisdom":10,"charisma":10},
"bonus":{"strength":1,"dexterity":1,"constitution":1},"skills":["perception","survival"],"armor":"half-plate","shield":true,"weapons":["longsword"]}`

func TestBuilderOptionsOverHTTP(t *testing.T) {
	t.Parallel()
	h := compendiumServer(t, &fakeCompendium{})
	rec := getWith(h, "/api/v1/compendium/builder?ruleset=srd-2024", nil)
	b := decode(t, rec)
	skills, _ := b["skills"].([]any)
	armor, _ := b["armor"].([]any)
	if rec.Code != 200 || b["pointBuyBudget"] != float64(27) || len(skills) != 18 || len(armor) != 2 {
		t.Fatalf("options: %d %v", rec.Code, b)
	}
	if rec := getWith(h, "/api/v1/compendium/builder?ruleset=srd-2024", map[string]string{"If-None-Match": `"c7"`}); rec.Code != http.StatusNotModified {
		t.Fatalf("conditional: %d", rec.Code)
	}
	for _, c := range []*fakeCompendium{{versionErr: errors.New("x")}, {builderErr: errors.New("x")}} {
		if rec := getWith(compendiumServer(t, c), "/api/v1/compendium/builder?ruleset=srd-2024", nil); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("failure: %d", rec.Code)
		}
	}
}

func TestCharacterLifecycleOverHTTP(t *testing.T) {
	t.Parallel()
	h := realCampaigns(t)
	id, _ := campaignWithPlayer(t, h)
	base := "/api/v1/campaigns/" + id + "/characters"
	rec := call(h, http.MethodPost, base+"/preview", "player", build)
	if rec.Code != 200 || decode(t, rec)["armorClass"] != float64(19) {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPost, base, "player", build)
	sheet := decode(t, rec)
	chID, _ := sheet["id"].(string)
	if rec.Code != http.StatusCreated || sheet["hpMax"] != float64(12) || chID == "" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	bonus, _ := sheet["bonus"].(map[string]any)
	if len(bonus) != 3 {
		t.Fatalf("bonus = %v", bonus)
	}
	rec = call(h, http.MethodGet, base, "dm", "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ownerName":"Tamsin"`) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodGet, base+"/"+chID, "dm", "")
	if rec.Code != 200 || decode(t, rec)["mine"] != false {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	rec = call(h, http.MethodPatch, base+"/"+chID, "player", `{"name":"Kara Two","hpCurrent":3,"armor":"","shield":false,"weapons":[]}`)
	edited := decode(t, rec)
	if rec.Code != 200 || edited["hpCurrent"] != float64(3) || edited["armorClass"] != float64(12) {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body.String())
	}
	bad := strings.Replace(build, `"skills":["perception","survival"]`, `"skills":["athletics","survival"]`, 1)
	rec = call(h, http.MethodPost, base, "player", bad)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "already grants athletics") {
		t.Fatalf("rule error: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, base, "stranger", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("stranger: %d", rec.Code)
	}
	if rec := call(h, http.MethodDelete, base+"/"+chID, "player", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

type brokenCharacters struct{ err error }

func (b brokenCharacters) Preview(context.Context, caller.Caller, domain.CampaignID, domain.Build) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) Create(context.Context, caller.Caller, domain.CampaignID, domain.Build) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) List(context.Context, caller.Caller, domain.CampaignID) ([]domain.CharacterSummary, error) {
	return nil, b.err
}

func (b brokenCharacters) Get(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) Update(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, app.Edit) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) Delete(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID) error {
	return b.err
}

func (b brokenCharacters) SetImage(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, domain.ImageKind, []byte) error {
	return b.err
}

func (b brokenCharacters) ClearToken(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID) error {
	return b.err
}

func (b brokenCharacters) Image(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, domain.ImageKind) (domain.Image, []byte, error) {
	return domain.Image{}, nil, b.err
}

func (b brokenCharacters) Mine(context.Context, caller.Caller) ([]domain.OwnedCharacter, error) {
	return nil, b.err
}

func (b brokenCharacters) Owned(context.Context, caller.Caller, domain.OwnedID) (domain.OwnedCharacter, error) {
	return domain.OwnedCharacter{}, b.err
}

func (b brokenCharacters) UpdateOwned(context.Context, caller.Caller, domain.OwnedID, string, string) (domain.OwnedCharacter, error) {
	return domain.OwnedCharacter{}, b.err
}

func (b brokenCharacters) Join(context.Context, caller.Caller, domain.OwnedID, domain.CampaignID) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) Draft(context.Context, caller.Caller, domain.CampaignID) (domain.Draft, error) {
	return domain.Draft{}, b.err
}

func (b brokenCharacters) SaveDraft(context.Context, caller.Caller, domain.CampaignID, int, []byte) (domain.Draft, error) {
	return domain.Draft{}, b.err
}

func (b brokenCharacters) DiscardDraft(context.Context, caller.Caller, domain.CampaignID) error {
	return b.err
}

func (b brokenCharacters) RollScores(context.Context, caller.Caller, domain.CampaignID) (domain.Draft, error) {
	return domain.Draft{}, b.err
}

func (b brokenCharacters) PlanLevelUp(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, string) (app.LevelUpPlan, error) {
	return app.LevelUpPlan{}, b.err
}

func (b brokenCharacters) LevelUp(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, app.LevelUpRequest) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) Spells(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID) (app.Spellcasting, error) {
	return app.Spellcasting{}, b.err
}

func (b brokenCharacters) Prepare(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, string, []string) (app.Spellcasting, error) {
	return app.Spellcasting{}, b.err
}

func (b brokenCharacters) CastRitual(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, string) (app.Ritual, error) {
	return app.Ritual{}, b.err
}

func (b brokenCharacters) PassInspiration(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, domain.CharacterID) (app.Sheet, error) {
	return app.Sheet{}, b.err
}

func (b brokenCharacters) CopySpell(context.Context, caller.Caller, domain.CampaignID, domain.CharacterID, string) (app.Spellcasting, error) {
	return app.Spellcasting{}, b.err
}

func TestCharacterErrorsBecomeProblems(t *testing.T) {
	t.Parallel()
	base := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/characters"
	one := base + "/0190c7a8-0000-7000-8000-000000000002"
	ops := []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodPost, base, build},
		{http.MethodPost, base + "/preview", build},
		{http.MethodGet, one, ""},
		{http.MethodPatch, one, `{"name":"X"}`},
		{http.MethodDelete, one, ""},
		{http.MethodGet, one + "/portrait", ""},
		{http.MethodGet, one + "/token", ""},
		{http.MethodDelete, one + "/token", ""},
		{http.MethodGet, "/api/v1/characters", ""},
		{http.MethodGet, "/api/v1/characters/0190c7a8-0000-7000-8000-000000000003", ""},
		{http.MethodPut, "/api/v1/characters/0190c7a8-0000-7000-8000-000000000003", `{"name":"X","backstory":""}`},
		{http.MethodPost, "/api/v1/characters/0190c7a8-0000-7000-8000-000000000003/campaigns", `{"campaignId":"0190c7a8-0000-7000-8000-000000000001"}`},
		{http.MethodGet, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/character-draft", ""},
		{http.MethodPut, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/character-draft", `{"step":1,"build":{}}`},
		{http.MethodDelete, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/character-draft", ""},
		{http.MethodPost, "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001/character-draft/roll", ""},
		{http.MethodGet, one + "/level-up", ""},
		{http.MethodPost, one + "/level-up", `{"class":"fighter"}`},
		{http.MethodGet, one + "/spells", ""},
		{http.MethodPut, one + "/spells/prepared", `{"class":"wizard","spells":[]}`},
		{http.MethodPost, one + "/spells/rituals", `{"spell":"alarm"}`},
		{http.MethodPost, one + "/spellbook", `{"spell":"alarm"}`},
		{http.MethodPost, one + "/inspiration/pass", `{"to":"0190c7a8-0000-7000-8000-000000000003"}`},
	}
	for err, code := range map[error]int{domain.ErrLocked: http.StatusConflict, errors.New("disk"): http.StatusServiceUnavailable} {
		h := campaignServer(t, brokenCampaigns{}, httpapi.CharacterService(brokenCharacters{err: err}))
		for _, o := range ops {
			if rec := call(h, o.method, o.path, "u", o.body); rec.Code != code {
				t.Errorf("%v %s %s: %d", err, o.method, o.path, rec.Code)
			}
		}
	}
	ctx := context.Background()
	h := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(h.ListCharacters(ctx, oas.ListCharactersParams{}))
	add(h.CreateCharacter(ctx, &oas.CharacterBuild{}, oas.CreateCharacterParams{}))
	add(h.PreviewCharacter(ctx, &oas.CharacterBuild{}, oas.PreviewCharacterParams{}))
	add(h.GetCharacter(ctx, oas.GetCharacterParams{}))
	add(h.UpdateCharacter(ctx, &oas.CharacterEdit{}, oas.UpdateCharacterParams{}))
	add(h.DeleteCharacter(ctx, oas.DeleteCharacterParams{}))
	add(h.SetPortrait(ctx, oas.SetPortraitReq{}, oas.SetPortraitParams{}))
	add(h.SetTokenIcon(ctx, oas.SetTokenIconReq{}, oas.SetTokenIconParams{}))
	add(h.ClearTokenIcon(ctx, oas.ClearTokenIconParams{}))
	add(h.GetPortrait(ctx, oas.GetPortraitParams{}))
	add(h.GetTokenIcon(ctx, oas.GetTokenIconParams{}))
	add(h.ListMyCharacters(ctx))
	add(h.GetMyCharacter(ctx, oas.GetMyCharacterParams{}))
	add(h.UpdateMyCharacter(ctx, &oas.OwnedCharacterChange{}, oas.UpdateMyCharacterParams{}))
	add(h.JoinCampaign(ctx, &oas.CharacterJoin{}, oas.JoinCampaignParams{}))
	add(h.GetCharacterDraft(ctx, oas.GetCharacterDraftParams{}))
	add(h.SaveCharacterDraft(ctx, &oas.CharacterDraftSave{}, oas.SaveCharacterDraftParams{}))
	add(h.DiscardCharacterDraft(ctx, oas.DiscardCharacterDraftParams{}))
	add(h.RollCharacterScores(ctx, oas.RollCharacterScoresParams{}))
	add(h.PlanLevelUp(ctx, oas.PlanLevelUpParams{}))
	add(h.LevelUp(ctx, &oas.LevelUpRequest{}, oas.LevelUpParams{}))
	add(h.GetSpellcasting(ctx, oas.GetSpellcastingParams{}))
	add(h.PrepareSpells(ctx, &oas.SpellPreparation{}, oas.PrepareSpellsParams{}))
	add(h.CastRitual(ctx, &oas.SpellChoice{}, oas.CastRitualParams{}))
	add(h.CopySpell(ctx, &oas.SpellChoice{}, oas.CopySpellParams{}))
	add(h.PassInspiration(ctx, &oas.InspirationPass{}, oas.PassInspirationParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
