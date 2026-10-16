package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

type brokenJournal struct{ err error }

func (b brokenJournal) Read(context.Context, caller.Caller, domain.CampaignID) (campaignapp.JournalView, error) {
	return campaignapp.JournalView{}, b.err
}

func (b brokenJournal) CreateQuest(context.Context, caller.Caller, domain.CampaignID, campaignapp.QuestInput) (domain.Quest, error) {
	return domain.Quest{}, b.err
}

func (b brokenJournal) UpdateQuest(context.Context, caller.Caller, domain.CampaignID, domain.QuestID, campaignapp.QuestInput) (domain.Quest, error) {
	return domain.Quest{}, b.err
}

func (b brokenJournal) DeleteQuest(context.Context, caller.Caller, domain.CampaignID, domain.QuestID) error {
	return b.err
}

func (b brokenJournal) CreateLore(context.Context, caller.Caller, domain.CampaignID, campaignapp.LoreInput) (domain.Lore, error) {
	return domain.Lore{}, b.err
}

func (b brokenJournal) UpdateLore(context.Context, caller.Caller, domain.CampaignID, domain.LoreID, campaignapp.LoreInput) (domain.Lore, error) {
	return domain.Lore{}, b.err
}

func (b brokenJournal) DeleteLore(context.Context, caller.Caller, domain.CampaignID, domain.LoreID) error {
	return b.err
}

func (b brokenJournal) ReadItem(context.Context, caller.Caller, domain.CampaignID, string) (int, error) {
	return 0, b.err
}

// A fault in the Journal service is a fault, and nobody signed out gets anywhere.
func TestJournalErrors(t *testing.T) {
	t.Parallel()
	c := "/api/v1/campaigns/0190c7a8-0000-7000-8000-000000000001"
	quest, lore := c+"/quests/0190c7a8-0000-7000-8000-000000000002", c+"/lore/0190c7a8-0000-7000-8000-000000000003"
	h := campaignServer(t, brokenCampaigns{}, httpapi.JournalService(brokenJournal{err: errors.New("disk")}))
	for _, o := range []struct{ method, path, body string }{
		{http.MethodGet, c + "/journal", ""},
		{http.MethodPost, c + "/journal/readings", `{"itemSlug":"black-book"}`},
		{http.MethodPost, c + "/quests", `{"name":"A","status":"active"}`},
		{http.MethodPut, quest, `{"name":"A","status":"active"}`},
		{http.MethodDelete, quest, ""},
		{http.MethodPost, c + "/lore", `{"title":"A"}`},
		{http.MethodPut, lore, `{"title":"A"}`},
		{http.MethodDelete, lore, ""},
	} {
		if rec := call(h, o.method, o.path, "u", o.body); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "disk") {
			t.Errorf("%s %s: %d %s", o.method, o.path, rec.Code, rec.Body.String())
		}
	}
	ctx := context.Background()
	hh := &httpapi.Handler{Log: quiet}
	results := []any{}
	add := func(res any, _ error) { results = append(results, res) }
	add(hh.GetJournal(ctx, oas.GetJournalParams{}))
	add(hh.ReadItem(ctx, &oas.ItemReading{}, oas.ReadItemParams{}))
	add(hh.CreateQuest(ctx, &oas.QuestInput{}, oas.CreateQuestParams{}))
	add(hh.UpdateQuest(ctx, &oas.QuestInput{}, oas.UpdateQuestParams{}))
	add(hh.DeleteQuest(ctx, oas.DeleteQuestParams{}))
	add(hh.CreateLore(ctx, &oas.LoreInput{}, oas.CreateLoreParams{}))
	add(hh.UpdateLore(ctx, &oas.LoreInput{}, oas.UpdateLoreParams{}))
	add(hh.DeleteLore(ctx, oas.DeleteLoreParams{}))
	for i, r := range results {
		if p, ok := r.(*oas.ProblemStatusCodeWithHeaders); !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("operation %d: %+v", i, r)
		}
	}
}
