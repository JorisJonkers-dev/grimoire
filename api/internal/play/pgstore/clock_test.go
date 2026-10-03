package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
)

// The Game Clock and the Marching Order are kept with the Campaign. A Character of another Campaign
// takes no place in this one's order.
func TestTheClockAndTheMarchingOrderAreKept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	character := func(campaign uuid.UUID, name string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := tb.pool.Exec(ctx, `WITH owner AS (SELECT id, auth_subject FROM campaign.members WHERE campaign_id = $2 LIMIT 1),
			hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
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
	aria, brom, stranger := character(tb.campaign, "Aria"), character(tb.campaign, "Brom"), character(elsewhere, "Stranger")
	commit := func(repo *pgstore.Store, w live.Write) error {
		_, err := repo.Commit(ctx, s, nil, w, tb.dmMember(t), dm, time.Now())
		return err
	}
	day := 4
	if err := commit(store, live.Write{Kind: domain.ActionClockSet, Day: &day, Minute: 375}); err != nil {
		t.Fatal(err)
	}
	if now, err := store.GameClock(ctx, tb.campaign); err != nil || now != (clock.Time{Day: 4, Minute: 375}) {
		t.Fatalf("the Game Clock = %+v %v", now, err)
	}
	order := func() []uuid.UUID {
		t.Helper()
		got, err := store.MarchingOrder(ctx, tb.campaign)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := order(); len(got) != 0 {
		t.Fatalf("before it is arranged = %v", got)
	}
	if err := commit(store, live.Write{Kind: domain.ActionMarchingOrderSet, March: []uuid.UUID{brom, stranger, aria}}); err != nil {
		t.Fatal(err)
	}
	if got := order(); !slices.Equal(got, []uuid.UUID{brom, aria}) {
		t.Fatalf("the order, without the stranger = %v", got)
	}
	if got, err := store.MarchingOrder(ctx, elsewhere); err != nil || len(got) != 0 {
		t.Fatalf("the other Campaign's order = %v %v", got, err)
	}
	if err := commit(store, live.Write{Kind: domain.ActionMarchingOrderSet, March: []uuid.UUID{aria}}); err != nil {
		t.Fatal(err)
	}
	if got := order(); !slices.Equal(got, []uuid.UUID{aria}) {
		t.Fatalf("rearranged = %v", got)
	}
	for name, w := range map[string]live.Write{
		"the clock": {Kind: domain.ActionClockSet, Day: &day, Minute: 10, Dawned: []domain.Recharge{{Instance: domain.InstanceID(uuid.New()), Charges: 1}}},
		"the order": {Kind: domain.ActionMarchingOrderSet, March: []uuid.UUID{brom, aria}},
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := commit(pgstore.NewFaulty(tb.pool, f), w)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	for name, read := range map[string]func(repo *pgstore.Store) error{
		"read the clock": func(repo *pgstore.Store) error { _, err := repo.GameClock(ctx, tb.campaign); return err },
		"read the order": func(repo *pgstore.Store) error { _, err := repo.MarchingOrder(ctx, tb.campaign); return err },
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := read(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
