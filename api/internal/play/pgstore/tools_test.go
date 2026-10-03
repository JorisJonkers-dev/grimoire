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
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

func TestToolActionsKeepWhatUndoNeeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	s, _ := sessions(tb, pgstore.New(tb.pool), &closed{}).Start(ctx, dm, tb.campaign)
	store, me := pgstore.New(tb.pool), tb.dmMember(t)
	commit := func(w live.Write) live.Committed {
		t.Helper()
		done, err := store.Commit(ctx, s, nil, w, me, dm, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return done
	}
	goblin := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Goblin 1", Kind: domain.TokenEnemy, Stats: &domain.Stats{HP: 7, HPMax: 7, SpeedFt: 30}}
	wolf := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Wolf", Kind: domain.TokenEnemy, Q: 1}
	spawned := commit(live.Write{Kind: domain.ActionEncounterSpawned, Spawned: []domain.Token{goblin, wolf}})
	hurt := commit(live.Write{Kind: domain.ActionHPAdjusted, Token: goblin, HP: &live.HPChange{Token: goblin.ID, Before: 7, After: 3}})
	crate := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Crate", Kind: domain.TokenObject, Q: 2}
	placed := commit(live.Write{Kind: domain.ActionTokenPlaced, Token: crate})
	if spawned.Action < 1 || hurt.Action != spawned.Action+1 || placed.Seq != spawned.Seq+2 {
		t.Fatalf("sequences %+v %+v %+v", spawned, hurt, placed)
	}
	a, err := store.Action(ctx, s.ID, spawned.Action)
	if err != nil || a.Kind != domain.ActionEncounterSpawned || len(a.Spawned) != 2 || a.Spawned[0] != goblin.ID || a.Undone {
		t.Fatalf("spawn = %+v %v", a, err)
	}
	if h, err := store.Action(ctx, s.ID, hurt.Action); err != nil || h.HP != (live.HPChange{Token: goblin.ID, Before: 7, After: 3}) {
		t.Fatalf("hp = %+v %v", h, err)
	}
	if p, err := store.Action(ctx, s.ID, placed.Action); err != nil || p.Token != crate.ID {
		t.Fatalf("placement = %+v %v", p, err)
	}
	commit(live.Write{Kind: domain.ActionTokenRemoved, Token: wolf, Undoes: a.ID})
	if a, _ := store.Action(ctx, s.ID, spawned.Action); !a.Undone {
		t.Fatal("the spawn is undone once an undo names it")
	}
	if _, err := store.Action(ctx, s.ID, 99999); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("missing action = %v", err)
	}
	log, err := store.SessionLog(ctx, s.ID, 3)
	if err != nil || len(log) != 3 || log[0].Kind != domain.ActionTokenRemoved || log[0].Label != "Wolf" || log[2].Kind != domain.ActionHPAdjusted {
		t.Fatalf("log = %+v %v", log, err)
	}
	if log[0].Token == nil || *log[0].Token != wolf.ID || log[2].Token == nil || *log[2].Token != goblin.ID {
		t.Fatalf("each entry names the token it touched = %+v", log)
	}
	if !log[1].Undoable() || !log[2].Undoable() || log[0].Undoable() {
		t.Fatalf("undoable = %+v", log)
	}
	all, _ := store.SessionLog(ctx, s.ID, 10)
	if last := all[3]; last.Kind != domain.ActionEncounterSpawned || last.Label != "Goblin 1, Wolf" || !last.Undone || last.Undoable() || last.Token != nil {
		t.Fatalf("the spawn in the log = %+v", last)
	}
	ops := map[string]func(st *pgstore.Store) error{
		"spawn": func(st *pgstore.Store) error {
			_, err := st.Commit(ctx, s, nil, live.Write{Kind: domain.ActionEncounterSpawned, Spawned: []domain.Token{{ID: domain.TokenID(uuid.New()), Label: "Rat", Kind: domain.TokenEnemy}}}, me, dm, time.Now())
			return err
		},
		"undo": func(st *pgstore.Store) error {
			gone := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Bat", Kind: domain.TokenEnemy}
			if _, err := store.Commit(ctx, s, nil, live.Write{Kind: domain.ActionTokenPlaced, Token: gone}, me, dm, time.Now()); err != nil {
				return err
			}
			made, _ := store.SessionLog(ctx, s.ID, 1)
			rec, _ := store.Action(ctx, s.ID, made[0].Seq)
			_, err := st.Commit(ctx, s, nil, live.Write{Kind: domain.ActionTokenRemoved, Token: gone, Undoes: rec.ID}, me, dm, time.Now())
			return err
		},
		"spawn record": func(st *pgstore.Store) error { _, err := st.Action(ctx, s.ID, spawned.Action); return err },
		"hp record":    func(st *pgstore.Store) error { _, err := st.Action(ctx, s.ID, hurt.Action); return err },
		"token record": func(st *pgstore.Store) error { _, err := st.Action(ctx, s.ID, placed.Action); return err },
		"log":          func(st *pgstore.Store) error { _, err := st.SessionLog(ctx, s.ID, 5); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
