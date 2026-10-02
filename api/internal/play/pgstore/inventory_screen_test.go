package pgstore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

func TestEveryInventoryScreenFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	char := uuid.New()
	if _, err := tb.pool.Exec(ctx, `WITH hero AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, m.auth_subject, 'Hero', 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM campaign.members m WHERE m.id = $3)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug,
		ability_method, hp_max, hp_current, level) VALUES ($1, $1, $2, $3, 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 20, 5, 4)`, char, tb.campaign, tb.playerID); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO compendium.documents (key, title, ruleset_year, precedence, license, attribution, url) VALUES ('screen-doc', 'Screen', 2024, 1, 'CC-BY-4.0', 'a', 'https://a') ON CONFLICT DO NOTHING`,
		`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, requires_attunement)
			SELECT id, 'healing-draught', 'Potion of Healing', '', 'potion', 50, 0.5, true, false FROM compendium.documents WHERE key = 'screen-doc' ON CONFLICT DO NOTHING`,
		`INSERT INTO compendium.items (document_id, slug, name, description, category, cost_gp, weight_lb, magic, requires_attunement)
			SELECT id, 'test-blade', 'Blade', '', 'weapon', 10, 2, false, false FROM compendium.documents WHERE key = 'screen-doc' ON CONFLICT DO NOTHING`,
	} {
		if _, err := tb.pool.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	base := &app.Inventories{Store: pgstore.New(tb.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Roll: func(n, _ int) int { return n }}
	if _, err := base.View(ctx, player, tb.campaign, char); err != nil {
		t.Fatal(err)
	}
	restock := func() {
		for _, q := range []string{
			`DELETE FROM campaign.item_instances WHERE container_id IN (SELECT id FROM campaign.containers WHERE campaign_id = $1)`,
			`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
				SELECT gen_random_uuid(), id, 'test-blade', 2, true, false, now() FROM campaign.containers WHERE character_id IS NOT NULL AND campaign_id = $1`,
			`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at)
				SELECT gen_random_uuid(), id, 'healing-draught', 2, true, false, now() FROM campaign.containers WHERE kind = 'party_stash' AND campaign_id = $1`,
		} {
			if _, err := tb.pool.Exec(ctx, q, tb.campaign); err != nil {
				t.Fatal(err)
			}
		}
	}
	ops := map[string]func(r *app.Inventories) error{
		"view": func(r *app.Inventories) error { _, err := r.View(ctx, player, tb.campaign, char); return err },
		"equip": func(r *app.Inventories) error {
			restock()
			_, err := r.Move(ctx, player, tb.campaign, char, app.ItemMove{Item: app.ItemRef{Slug: "test-blade"}, To: app.ToSlot, Slot: "main_hand", Count: 1})
			return err
		},
		"stash": func(r *app.Inventories) error {
			restock()
			_, err := r.Move(ctx, player, tb.campaign, char, app.ItemMove{Item: app.ItemRef{Slug: "test-blade"}, To: app.ToStash, Count: 1})
			return err
		},
		"take": func(r *app.Inventories) error {
			restock()
			_, err := r.Take(ctx, player, tb.campaign, char, app.ItemRef{Slug: "healing-draught"}, 1)
			return err
		},
		"swap": func(r *app.Inventories) error {
			restock()
			for _, slot := range []string{"main_hand", "ranged_main"} {
				if _, err := base.Move(ctx, player, tb.campaign, char, app.ItemMove{Item: app.ItemRef{Slug: "test-blade"}, To: app.ToSlot, Slot: slot, Count: 1}); err != nil {
					t.Fatal(err)
				}
			}
			_, err := r.Swap(ctx, player, tb.campaign, char)
			return err
		},
		"drink": func(r *app.Inventories) error {
			restock()
			if _, err := base.Take(ctx, player, tb.campaign, char, app.ItemRef{Slug: "healing-draught"}, 2); err != nil {
				t.Fatal(err)
			}
			_, _, err := r.Use(ctx, player, tb.campaign, char, app.ItemRef{Slug: "healing-draught"}, app.Drink, 1)
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			r := &app.Inventories{Store: pgstore.NewFaulty(tb.pool, f), Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Roll: func(n, _ int) int { return n }}
			err := op(r)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
