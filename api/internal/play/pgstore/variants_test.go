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

// The live store reads the Campaign's own Rule Variants and the Roll Tables it sees, leaving out a
// design the rules do not take, and keeps a roll on a table that is still open across a restart.
func TestRuleHooksRollTablesAndOpenTableRollsAreKept(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	var elsewhere uuid.UUID
	if err := tb.pool.QueryRow(ctx, `INSERT INTO campaign.campaigns (name, ruleset_pref, created_by, created_at, updated_at)
		VALUES ('Elsewhere', 'srd-2024', 'someone', now(), now()) RETURNING id`).Scan(&elsewhere); err != nil {
		t.Fatal(err)
	}
	table := func(name, design string, campaigns ...uuid.UUID) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := tb.pool.Exec(ctx, `INSERT INTO library.entries (id, owner_subject, kind, name, design, created_at, updated_at) VALUES ($1, 'dm', 'table', $2, $3, now(), now())`, id, name, design); err != nil {
			t.Fatal(err)
		}
		for _, c := range campaigns {
			if _, err := tb.pool.Exec(ctx, `INSERT INTO library.campaign_links (campaign_id, entry_id, linked_at, updated_at) VALUES ($1, $2, now(), now())`, c, id); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	sound := `{"dice":"1d6","results":[{"from":1,"to":6,"text":"Something.","effect":"prone"}]}`
	fumbles := table("Fumbles", sound, tb.campaign)
	table("Overlapping", `{"dice":"1d6","results":[{"from":1,"to":3,"text":"A."},{"from":3,"to":6,"text":"B."}]}`, tb.campaign)
	table("Not a table", `{"dice":7}`, tb.campaign)
	table("Not linked", sound)
	table("Theirs", sound, elsewhere)
	tables, err := store.RollTables(ctx, tb.campaign)
	if err != nil || len(tables) != 1 || tables[0].ID != fumbles || tables[0].Name != "Fumbles" || tables[0].Design.Dice != "1d6" || tables[0].Design.Results[0].Effect != "prone" {
		t.Fatalf("the Campaign's tables = %+v %v", tables, err)
	}
	for _, h := range []struct {
		campaign uuid.UUID
		name     string
		point    string
		table    *uuid.UUID
		effect   any
	}{{tb.campaign, "Critical fumbles", "natural-1", &fumbles, nil}, {tb.campaign, "Winded", "drop-to-0", nil, "exhaustion"}, {elsewhere, "Theirs", "rest", nil, "prone"}} {
		if _, err := tb.pool.Exec(ctx, `INSERT INTO campaign.rule_variant_hooks (id, campaign_id, name, hook, roll_table, effect, created_at) VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp())`,
			uuid.New(), h.campaign, h.name, h.point, h.table, h.effect); err != nil {
			t.Fatal(err)
		}
	}
	hooks, err := store.RuleHooks(ctx, tb.campaign)
	want := []live.Hook{{Name: "Critical fumbles", Point: "natural-1", Table: &fumbles, Effect: ""}, {Name: "Winded", Point: "drop-to-0", Table: nil, Effect: "exhaustion"}}
	if err != nil || !reflect.DeepEqual(hooks, want) {
		t.Fatalf("the Campaign's hooks = %+v %v", hooks, err)
	}

	// A roll on the table is opened with the hook that asked for it, read back after a restart, and gone once it lands.
	me := tb.dmMember(t)
	raider := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Raider", Kind: domain.TokenEnemy}
	commit := func(repo *pgstore.Store, w live.Write) error {
		_, err := repo.Commit(ctx, s, nil, w, me, dm, time.Now())
		return err
	}
	if err := commit(store, live.Write{Kind: domain.ActionTokenPlaced, Token: raider}); err != nil {
		t.Fatal(err)
	}
	fired := func() live.Write {
		roll := domain.Roll{
			ID: domain.RollID(uuid.New()), CampaignID: tb.campaign, Purpose: "Critical fumbles: Fumbles", Notation: "1d6", RequestedBy: me.Name, Roller: me,
			Status: domain.StatusPending, Dice: []domain.Die{{No: 0, Group: 0, Faces: 6}},
		}
		return live.Write{
			Kind: domain.ActionHookFired, Token: raider, Note: "Critical fumbles", Rolls: []domain.Roll{roll},
			Pending: &domain.PendingAction{RollID: roll.ID, Actor: raider.ID, Target: &raider.ID, Action: "roll_table", Table: &fumbles, Hook: "Critical fumbles"},
		}
	}
	open := fired()
	if err := commit(store, open); err != nil {
		t.Fatal(err)
	}
	pending, err := store.LoadPendingActions(ctx, s.ID)
	if err != nil || len(pending) != 1 || pending[0].RollID != open.Pending.RollID || pending[0].Action != "roll_table" || pending[0].Table == nil || *pending[0].Table != fumbles || pending[0].Hook != "Critical fumbles" || pending[0].DC != 0 {
		t.Fatalf("the open roll = %+v %v", pending, err)
	}
	if roll, err := store.Roll(ctx, tb.campaign, open.Pending.RollID); err != nil || roll.Purpose != "Critical fumbles: Fumbles" || roll.Status != domain.StatusPending {
		t.Fatalf("its Roll Request = %+v %v", roll, err)
	}
	landed := live.Write{Kind: domain.ActionTableRolled, Token: raider, Settled: open.Pending.RollID, Note: "You fall flat on your face."}
	if err := commit(store, landed); err != nil {
		t.Fatal(err)
	}
	if pending, err := store.LoadPendingActions(ctx, s.ID); err != nil || len(pending) != 0 {
		t.Fatalf("once landed = %+v %v", pending, err)
	}
	var labels []string
	rows, err := tb.pool.Query(ctx, `SELECT a.kind || ' ' || e.label FROM play.actions a JOIN play.action_token_events e ON e.action_id = a.id
		WHERE a.session_id = $1 AND a.kind IN ('rule_hook_fired', 'roll_table_rolled') ORDER BY a.seq`, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			t.Fatal(err)
		}
		labels = append(labels, l)
	}
	if !reflect.DeepEqual(labels, []string{"rule_hook_fired Raider: Critical fumbles", "roll_table_rolled Raider: You fall flat on your face."}) {
		t.Fatalf("the Action Log = %q", labels)
	}
	for name, op := range map[string]func(repo *pgstore.Store) error{
		"hooks":  func(repo *pgstore.Store) error { _, err := repo.RuleHooks(ctx, tb.campaign); return err },
		"tables": func(repo *pgstore.Store) error { _, err := repo.RollTables(ctx, tb.campaign); return err },
		"fired":  func(repo *pgstore.Store) error { return commit(repo, fired()) },
		"landed": func(repo *pgstore.Store) error {
			again := fired()
			if err := commit(store, again); err != nil {
				t.Fatal(err)
			}
			w := landed
			w.Settled = again.Pending.RollID
			return commit(repo, w)
		},
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
