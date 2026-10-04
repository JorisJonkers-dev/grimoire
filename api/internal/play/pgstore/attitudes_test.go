package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

// A creature's attitude towards a Character is kept with the Session's token, moves when a later
// check moves it, and is never taken towards a Character of another Campaign or by a token of another
// Session.
func TestAttitudesAreKept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	hero := func(campaign uuid.UUID, name string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := tb.pool.Exec(ctx, `WITH owner AS (SELECT id, auth_subject FROM campaign.members WHERE campaign_id = $2 LIMIT 1),
			sheet AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
				SELECT $1, auth_subject, $3, 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM owner)
			INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug, ability_method, hp_max, hp_current)
			SELECT $1, $1, $2, id, $3, 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 10, 10 FROM owner`, id, campaign, name); err != nil {
			t.Fatal(err)
		}
		return id
	}
	var elsewhere uuid.UUID
	if err := tb.pool.QueryRow(ctx, `INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at)
		VALUES ('Elsewhere', 'srd-2024', 'someone', now(), now()) RETURNING id`).Scan(&elsewhere); err != nil {
		t.Fatal(err)
	}
	if _, err := tb.pool.Exec(ctx, `INSERT INTO campaign.members (campaign_id, auth_subject, display_name, role) VALUES ($1, 'someone', 'Someone', 'dm')`, elsewhere); err != nil {
		t.Fatal(err)
	}
	aria, stranger := hero(tb.campaign, "Aria"), hero(elsewhere, "Stranger")
	commit := func(repo *pgstore.Store, w live.Write) error {
		_, err := repo.Commit(ctx, s, nil, w, tb.dmMember(t), dm, time.Now())
		return err
	}
	keeper := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Innkeeper", Kind: domain.TokenNPC}
	if err := commit(store, live.Write{Kind: domain.ActionTokenPlaced, Token: keeper}); err != nil {
		t.Fatal(err)
	}
	settle := func(character uuid.UUID, token domain.TokenID, value string) live.Write {
		return live.Write{Kind: domain.ActionResolved, Token: keeper, Settled: domain.RollID(uuid.New()), Attitude: &domain.Attitude{Token: token, Character: character, Value: value}}
	}
	read := func() []domain.Attitude {
		t.Helper()
		got, err := store.LoadAttitudes(ctx, s.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := read(); len(got) != 0 {
		t.Fatalf("before anyone tried = %+v", got)
	}
	for _, value := range []string{"friendly", "hostile"} {
		if err := commit(store, settle(aria, keeper.ID, value)); err != nil {
			t.Fatal(err)
		}
		if got := read(); len(got) != 1 || got[0] != (domain.Attitude{Token: keeper.ID, Character: aria, Value: value}) {
			t.Fatalf("after a check left it %s = %+v", value, got)
		}
	}
	// Towards a Character of another Campaign, or by a token that is not in this Session: nothing is kept.
	for name, w := range map[string]live.Write{
		"a stranger":      settle(stranger, keeper.ID, "friendly"),
		"a token unknown": settle(aria, domain.TokenID(uuid.New()), "friendly"),
	} {
		if err := commit(store, w); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if got := read(); len(got) != 1 || got[0].Value != "hostile" {
		t.Fatalf("after the strays = %+v", got)
	}
	if preset, err := store.Difficulty(ctx, tb.campaign); err != nil || preset != "standard" {
		t.Fatalf("a Campaign starts at standard = %q %v", preset, err)
	}
	if _, err := tb.pool.Exec(ctx, "UPDATE campaign.campaigns SET difficulty = 'story' WHERE id = $1", tb.campaign); err != nil {
		t.Fatal(err)
	}
	if preset, err := store.Difficulty(ctx, tb.campaign); err != nil || preset != "story" {
		t.Fatalf("set to story = %q %v", preset, err)
	}
	if shown, err := store.ShowDCs(ctx, tb.campaign); err != nil || shown {
		t.Fatalf("DCs shown to begin with = %v %v", shown, err)
	}
	if _, err := tb.pool.Exec(ctx, "UPDATE campaign.campaigns SET show_dcs = true WHERE id = $1", tb.campaign); err != nil {
		t.Fatal(err)
	}
	if shown, err := store.ShowDCs(ctx, tb.campaign); err != nil || !shown {
		t.Fatalf("DCs shown once set = %v %v", shown, err)
	}
	for name, op := range map[string]func(repo *pgstore.Store) error{
		"settle":     func(repo *pgstore.Store) error { return commit(repo, settle(aria, keeper.ID, "indifferent")) },
		"attitudes":  func(repo *pgstore.Store) error { _, err := repo.LoadAttitudes(ctx, s.ID); return err },
		"show dcs":   func(repo *pgstore.Store) error { _, err := repo.ShowDCs(ctx, tb.campaign); return err },
		"difficulty": func(repo *pgstore.Store) error { _, err := repo.Difficulty(ctx, tb.campaign); return err },
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	// When the token leaves the map its attitudes go with it.
	if err := commit(store, live.Write{Kind: domain.ActionTokenRemoved, Token: keeper}); err != nil {
		t.Fatal(err)
	}
	if got := read(); len(got) != 0 {
		t.Fatalf("after the Innkeeper left = %+v", got)
	}
}
