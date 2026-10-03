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
)

// A lingering injury is kept with the Character: a change that leaves one keeps it, at the level it
// stands; a change that cures one drops it; and each Character carries only its own.
func TestLingeringInjuriesAreKeptWithTheCharacter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	hero := func(name string) uuid.UUID {
		t.Helper()
		id := uuid.New()
		if _, err := tb.pool.Exec(ctx, `WITH owner AS (SELECT id, auth_subject FROM campaign.members WHERE campaign_id = $2 LIMIT 1),
			sheet AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
				SELECT $1, auth_subject, $3, 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM owner)
			INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug, ability_method, hp_max, hp_current)
			SELECT $1, $1, $2, id, $3, 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 10, 10 FROM owner`, id, tb.campaign, name); err != nil {
			t.Fatal(err)
		}
		return id
	}
	aria, brom := hero("Aria"), hero("Brom")
	token := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Aria", Kind: domain.TokenParty}
	commit := func(repo *pgstore.Store, w live.Write) error {
		_, err := repo.Commit(ctx, s, nil, w, tb.dmMember(t), dm, time.Now())
		return err
	}
	if err := commit(store, live.Write{Kind: domain.ActionTokenPlaced, Token: token}); err != nil {
		t.Fatal(err)
	}
	limp := domain.Injury{Character: aria, Slug: "hb-limp", Name: "Limp", Cure: "Regenerate", Level: 1}
	scar := domain.Injury{Character: aria, Slug: "hb-scar", Name: "Scar", Cure: "", Level: 0}
	theirs := domain.Injury{Character: brom, Slug: "hb-limp", Name: "Limp", Cure: "Regenerate", Level: 3}
	injured := live.Write{Kind: domain.ActionEffectEnded, Token: token, Injured: []domain.Injury{limp, scar, theirs}}
	if err := commit(store, injured); err != nil {
		t.Fatal(err)
	}
	read := func(character uuid.UUID) []domain.Injury {
		t.Helper()
		got, err := store.Injuries(ctx, character)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := read(aria); len(got) != 2 || !reflect.DeepEqual(map[string]domain.Injury{got[0].Slug: got[0], got[1].Slug: got[1]}, map[string]domain.Injury{"hb-limp": limp, "hb-scar": scar}) {
		t.Fatalf("Aria's injuries = %+v", got)
	}
	// Worse, it is the same injury at a new level, under the name and cure it has now.
	worse := limp
	worse.Level, worse.Name, worse.Cure = 2, "Bad limp", "Greater Restoration"
	if err := commit(store, live.Write{Kind: domain.ActionEffectEnded, Token: token, Injured: []domain.Injury{worse}}); err != nil {
		t.Fatal(err)
	}
	// Cured, it is gone from Aria alone.
	if err := commit(store, live.Write{Kind: domain.ActionEffectEnded, Token: token, Cured: []domain.Injury{scar}}); err != nil {
		t.Fatal(err)
	}
	if got := read(aria); !reflect.DeepEqual(got, []domain.Injury{worse}) {
		t.Fatalf("after worsening and a cure = %+v", got)
	}
	if got := read(brom); !reflect.DeepEqual(got, []domain.Injury{theirs}) {
		t.Fatalf("Brom's injuries = %+v", got)
	}
	if err := commit(store, live.Write{Kind: domain.ActionEffectEnded, Token: token, Cured: []domain.Injury{worse}}); err != nil {
		t.Fatal(err)
	}
	if got := read(aria); len(got) != 0 || len(read(brom)) != 1 || len(read(uuid.New())) != 0 {
		t.Fatalf("after the last cure = %+v", got)
	}
	for name, op := range map[string]func(repo *pgstore.Store) error{
		"read":    func(repo *pgstore.Store) error { _, err := repo.Injuries(ctx, brom); return err },
		"injured": func(repo *pgstore.Store) error { return commit(repo, injured) },
		"cured": func(repo *pgstore.Store) error {
			return commit(repo, live.Write{Kind: domain.ActionEffectEnded, Token: token, Cured: []domain.Injury{limp, scar}})
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
