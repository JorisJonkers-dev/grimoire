package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// rollTable puts a Roll Table in the DM's Library, linked to a Campaign unless it is nil.
func rollTable(t *testing.T, db *pg.Store, campaign *domain.CampaignID, name string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	if _, err := db.Pool().Exec(ctx, `INSERT INTO library.entries (id, owner_subject, kind, name, design, created_at, updated_at)
		VALUES ($1, $2, 'table', $3, '{"dice":"1d6","results":[{"from":1,"to":6,"text":"Something."}]}', now(), now())`, id, dmCaller.Subject, name); err != nil {
		t.Fatal(err)
	}
	if campaign != nil {
		if _, err := db.Pool().Exec(ctx, `INSERT INTO library.campaign_links (campaign_id, entry_id, linked_at, updated_at) VALUES ($1, $2, now(), now())`, uuid.UUID(*campaign), id); err != nil {
			t.Fatal(err)
		}
	}
	return id
}

// A DM authors Rule Variants of the Campaign's own from hook points: each applies an Effect, or rolls
// on a Roll Table the Campaign sees. Every Member reads them; only the DM sees the tables to choose from.
func TestTheDMAuthorsRuleVariantsFromHookPoints(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	s := &app.RuleHooks{Repo: pgstore.New(db.Pool()), Now: func() time.Time { now = now.Add(time.Second); return now }}
	fumbles, injuries, unlinked := rollTable(t, db, &d.ID, "Fumbles"), rollTable(t, db, &d.ID, "Critical injuries"), rollTable(t, db, nil, "Not linked")

	first, err := s.List(ctx, playerCaller, d.ID)
	if err != nil || first.DM || len(first.Hooks) != 0 || len(first.Tables) != 0 || len(first.Points) != 5 || first.Points[0].Slug != variants.HookNatural1 {
		t.Fatalf("to begin with, for a Player = %+v %v", first, err)
	}
	fumble, err := s.Create(ctx, dmCaller, d.ID, app.RuleHookInput{Name: "  Critical fumbles ", Hook: variants.HookNatural1, RollTable: &fumbles})
	if err != nil || fumble.Name != "Critical fumbles" || fumble.Hook != variants.HookNatural1 || fumble.RollTable == nil || *fumble.RollTable != fumbles || fumble.Effect != "" {
		t.Fatalf("create with a table = %+v %v", fumble, err)
	}
	winded, err := s.Create(ctx, dmCaller, d.ID, app.RuleHookInput{Name: "Winded", Hook: variants.HookDropTo0, Effect: " exhaustion "})
	if err != nil || winded.Effect != "exhaustion" || winded.RollTable != nil {
		t.Fatalf("create with an Effect = %+v %v", winded, err)
	}
	seen, err := s.List(ctx, playerCaller, d.ID)
	if err != nil || len(seen.Hooks) != 2 || seen.Hooks[0].Hook.ID != fumble.ID || seen.Hooks[0].TableName != "Fumbles" || seen.Hooks[1].Hook.Effect != "exhaustion" || seen.Hooks[1].TableName != "" || len(seen.Tables) != 0 {
		t.Fatalf("a Player's list = %+v %v", seen, err)
	}
	kept, err := s.List(ctx, dmCaller, d.ID)
	names := []string{}
	for _, table := range kept.Tables {
		names = append(names, table.Name)
	}
	if err != nil || !kept.DM || len(kept.Hooks) != 2 || !slices.Equal(names, []string{"Critical injuries", "Fumbles"}) || kept.Tables[0].ID != injuries {
		t.Fatalf("the DM's list = %+v %v", kept, err)
	}

	bad := map[string]app.RuleHookInput{
		"no name":                  {Name: " ", Hook: variants.HookRest, Effect: "prone"},
		"a long name":              {Name: strings.Repeat("a", 81), Hook: variants.HookRest, Effect: "prone"},
		"no such hook point":       {Name: "A", Hook: "on-hit", Effect: "prone"},
		"no outcome":               {Name: "A", Hook: variants.HookRest},
		"an outcome of only space": {Name: "A", Hook: variants.HookRest, Effect: "  "},
		"both outcomes":            {Name: "A", Hook: variants.HookRest, Effect: "prone", RollTable: &fumbles},
		"a long Effect":            {Name: "A", Hook: variants.HookRest, Effect: strings.Repeat("a", 81)},
	}
	for name, in := range bad {
		if _, err := s.Create(ctx, dmCaller, d.ID, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.Create(ctx, dmCaller, d.ID, app.RuleHookInput{Name: strings.Repeat("a", 80), Hook: variants.HookCast, Effect: strings.Repeat("a", 80)}); err != nil {
		t.Errorf("the longest name and Effect: %v", err)
	}
	var rule *apperr.RuleError
	stray := uuid.New()
	for name, table := range map[string]*uuid.UUID{"a table the Campaign does not see": &unlinked, "a table that does not exist": &stray} {
		if _, err := s.Create(ctx, dmCaller, d.ID, app.RuleHookInput{Name: "A", Hook: variants.HookRest, RollTable: table}); !errors.As(err, &rule) || !strings.Contains(rule.Reason, "not one this Campaign sees") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.Create(ctx, playerCaller, d.ID, app.RuleHookInput{Name: "Mine", Hook: variants.HookRest, Effect: "prone"}); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("a Player authors: %v", err)
	}
	if err := s.Delete(ctx, playerCaller, d.ID, fumble.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Errorf("a Player removes: %v", err)
	}
	missing := map[string]error{}
	_, missing["a stranger lists"] = s.List(ctx, stranger, d.ID)
	_, missing["a stranger authors"] = s.Create(ctx, stranger, d.ID, app.RuleHookInput{Name: "A", Hook: variants.HookRest, Effect: "prone"})
	missing["a stranger removes"] = s.Delete(ctx, stranger, d.ID, fumble.ID)
	missing["the removal of none"] = s.Delete(ctx, dmCaller, d.ID, uuid.New())
	campaigns, _ := service(t, pgstore.New(db.Pool()))
	elsewhere, err := campaigns.Create(ctx, dmCaller, app.CreateInput{Name: "Elsewhere", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	missing["a removal through another Campaign"] = s.Delete(ctx, dmCaller, elsewhere.ID, fumble.ID)
	for name, err := range missing {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// The other Campaign sees neither this one's hooks nor its tables.
	if _, err := s.Create(ctx, dmCaller, elsewhere.ID, app.RuleHookInput{Name: "A", Hook: variants.HookRest, RollTable: &fumbles}); !errors.As(err, &rule) {
		t.Errorf("a table of another Campaign: %v", err)
	}
	if there, err := s.List(ctx, dmCaller, elsewhere.ID); err != nil || len(there.Hooks) != 0 || len(there.Tables) != 0 {
		t.Fatalf("the other Campaign = %+v %v", there, err)
	}

	// A table the Campaign no longer sees leaves its hook with no table to name.
	if _, err := db.Pool().Exec(ctx, "DELETE FROM library.campaign_links WHERE entry_id = $1", fumbles); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.List(ctx, dmCaller, d.ID); after.Hooks[0].TableName != "" || after.Hooks[0].Hook.RollTable == nil {
		t.Fatalf("with the table unlinked = %+v", after.Hooks[0])
	}
	if err := s.Delete(ctx, dmCaller, d.ID, fumble.ID); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.List(ctx, dmCaller, d.ID); len(after.Hooks) != 2 || after.Hooks[0].Hook.ID != winded.ID {
		t.Fatalf("after removing = %+v", after.Hooks)
	}

	// A Campaign keeps fifty of its own and no more.
	for i := len(kept.Hooks); i < app.MaxRuleHooks; i++ {
		if _, err := s.Create(ctx, dmCaller, d.ID, app.RuleHookInput{Name: "Filler", Hook: variants.HookRest, Effect: "prone"}); err != nil {
			t.Fatalf("hook %d: %v", i, err)
		}
	}
	if _, err := s.Create(ctx, dmCaller, d.ID, app.RuleHookInput{Name: "One too many", Hook: variants.HookRest, Effect: "prone"}); !errors.As(err, &rule) || !strings.Contains(rule.Reason, "up to 50") {
		t.Errorf("the fifty-first: %v", err)
	}

	// Every operation reports a database fault at any of its calls.
	for name, op := range map[string]func(svc *app.RuleHooks) error{
		"list": func(svc *app.RuleHooks) error { _, err := svc.List(ctx, dmCaller, d.ID); return err },
		"create": func(svc *app.RuleHooks) error {
			_, err := svc.Create(ctx, dmCaller, elsewhere.ID, app.RuleHookInput{Name: "A", Hook: variants.HookRest, Effect: "prone"})
			return err
		},
		"create with a table": func(svc *app.RuleHooks) error {
			_, err := svc.Create(ctx, dmCaller, elsewhere.ID, app.RuleHookInput{Name: "A", Hook: variants.HookRest, RollTable: &unlinked})
			if errors.As(err, &rule) {
				return nil
			}
			return err
		},
		"delete": func(svc *app.RuleHooks) error {
			err := svc.Delete(ctx, dmCaller, d.ID, uuid.New())
			if errors.Is(err, domain.ErrNotFound) {
				return nil
			}
			return err
		},
	} {
		pgtest.EveryFault(t, func(fault *pgtest.Faulty) error {
			err := op(&app.RuleHooks{Repo: pgstore.NewFaulty(db.Pool(), fault), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
