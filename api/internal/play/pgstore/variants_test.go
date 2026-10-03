package pgstore_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
)

// The live store reads a Campaign's Rule Variants and its count of Short Rests, and keeps the count a
// rest leaves; neither is another Campaign's.
func TestRuleVariantsAndShortRestsAreKept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	var elsewhere uuid.UUID
	if err := tb.pool.QueryRow(ctx, `INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at, short_rests)
		VALUES ('Elsewhere', 'srd-2024', 'someone', now(), now(), 7) RETURNING id`).Scan(&elsewhere); err != nil {
		t.Fatal(err)
	}
	for campaign, value := range map[uuid.UUID]string{tb.campaign: variants.RestsGritty, elsewhere: variants.RestsEpic} {
		if _, err := tb.pool.Exec(ctx, "INSERT INTO campaign.rule_variants (campaign_id, variant, value, updated_at) VALUES ($1, 'rests', $2, now())", campaign, value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tb.pool.Exec(ctx, "INSERT INTO campaign.rule_variants (campaign_id, variant, value, updated_at) VALUES ($1, 'flanking', 'on', now())", tb.campaign); err != nil {
		t.Fatal(err)
	}
	if set, err := store.RuleVariants(ctx, tb.campaign); err != nil || !reflect.DeepEqual(set, variants.Set{variants.Flanking: variants.On, variants.Rests: variants.RestsGritty}) {
		t.Fatalf("the Campaign's variants = %v %v", set, err)
	}
	if set, err := store.RuleVariants(ctx, uuid.New()); err != nil || len(set) != 0 {
		t.Fatalf("a Campaign with none = %v %v", set, err)
	}
	if n, err := store.ShortRests(ctx, tb.campaign); err != nil || n != 0 {
		t.Fatalf("Short Rests to begin with = %d %v", n, err)
	}
	two := 2
	rested := live.Write{Kind: domain.ActionRestTaken, Rest: live.RestShort, RestOver: true, ShortRests: &two}
	commit := func(repo *pgstore.Store, w live.Write) error {
		_, err := repo.Commit(ctx, s, nil, w, tb.dmMember(t), dm, time.Now())
		return err
	}
	if err := commit(store, rested); err != nil {
		t.Fatal(err)
	}
	if n, err := store.ShortRests(ctx, tb.campaign); err != nil || n != 2 {
		t.Fatalf("Short Rests once kept = %d %v", n, err)
	}
	// A change that says nothing of rests leaves the count alone, and the other Campaign keeps its own.
	if err := commit(store, live.Write{Kind: domain.ActionRestInterrupted, RestOver: true}); err != nil {
		t.Fatal(err)
	}
	if n, _ := store.ShortRests(ctx, tb.campaign); n != 2 {
		t.Fatalf("Short Rests after another change = %d", n)
	}
	if n, err := store.ShortRests(ctx, elsewhere); err != nil || n != 7 {
		t.Fatalf("the other Campaign's Short Rests = %d %v", n, err)
	}
	for name, op := range map[string]func(repo *pgstore.Store) error{
		"variants":    func(repo *pgstore.Store) error { _, err := repo.RuleVariants(ctx, tb.campaign); return err },
		"short rests": func(repo *pgstore.Store) error { _, err := repo.ShortRests(ctx, tb.campaign); return err },
		"rested":      func(repo *pgstore.Store) error { return commit(repo, rested) },
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
