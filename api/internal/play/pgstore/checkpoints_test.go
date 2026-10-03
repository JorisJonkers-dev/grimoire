package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// A Checkpoint keeps the Session's rows and a rewind puts them back: the token that was there returns
// where it stood, the one placed since goes, and the fog closes again.
func TestACheckpointKeepsTheSessionAndARewindPutsItBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	store := pgstore.New(tb.pool)
	sess, _ := sessions(tb, store, &closed{}).Start(ctx, dm, tb.campaign)
	actor := tb.dmMember(t)
	m, err := store.InsertMap(ctx, domain.Map{CampaignID: tb.campaign, Name: "Crypt", Kind: domain.MapLocal, ImageKey: "k", ImageType: "image/png", Width: 400, Height: 300, HexSize: 40, OriginX: 35, OriginY: 40}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	board, _ := store.LoadMap(ctx, tb.campaign, m.ID)
	commit := func(w live.Write) {
		t.Helper()
		if _, err := store.Commit(ctx, sess, board, w, actor, dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	first := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Aria", Kind: domain.TokenParty}
	commit(live.Write{Kind: domain.ActionMapSet, MapID: &m.ID})
	commit(live.Write{Kind: domain.ActionTokenPlaced, Token: first})
	commit(live.Write{Kind: domain.ActionHexesRevealed, Hexes: []hex.Coord{{Q: 0, R: 0}}})

	readable := func(live.Store) error { return nil }
	kept := domain.Checkpoint{ID: uuid.New(), Name: "Here", Kind: domain.CheckpointNamed, Round: 0, ActionSeq: 0, CreatedAt: time.Now()}
	saved, err := store.SaveCheckpoint(ctx, sess, kept, actor, dm)
	if err != nil || saved.Seq != 4 || saved.Action < 1 {
		t.Fatalf("save = %+v, %v", saved, err)
	}
	kept.ActionSeq = saved.Action
	round := domain.Checkpoint{ID: uuid.New(), Name: "Round 1", Kind: domain.CheckpointRound, Round: 1, ActionSeq: saved.Action, CreatedAt: time.Now().Add(time.Second)}
	if err := store.MarkRound(ctx, sess, round, 20); err != nil {
		t.Fatal(err)
	}

	moved := first
	moved.Q = 2
	commit(live.Write{Kind: domain.ActionTokenMoved, Token: moved})
	commit(live.Write{Kind: domain.ActionTokenPlaced, Token: domain.Token{ID: domain.TokenID(uuid.New()), Label: "Goblin", Kind: domain.TokenEnemy, Q: 3}})
	commit(live.Write{Kind: domain.ActionHexesRevealed, Hexes: []hex.Coord{{Q: 4, R: 0}}})
	// The DM even puts the map away: the rewind brings the Session back onto it.
	commit(live.Write{Kind: domain.ActionMapSet})
	late := domain.Checkpoint{ID: uuid.New(), Name: "Later", Kind: domain.CheckpointNamed, Round: 0, ActionSeq: 0, CreatedAt: time.Now().Add(2 * time.Second)}
	if _, err := store.SaveCheckpoint(ctx, sess, late, actor, dm); err != nil {
		t.Fatal(err)
	}
	if list, err := store.Checkpoints(ctx, sess.ID); err != nil || len(list) != 3 || list[0].Name != "Here" || list[1].Name != "Round 1" || list[2].Name != "Later" || list[0].ActionSeq != saved.Action {
		t.Fatalf("checkpoints = %+v, %v", list, err)
	}

	done, err := store.Rewind(ctx, sess, kept, actor, dm, time.Now(), readable)
	if err != nil || done.Seq != 10 || done.Action <= saved.Action {
		t.Fatalf("rewind = %+v, %v", done, err)
	}
	_, tokens, back, err := store.Load(ctx, sess.ID)
	if err != nil || len(tokens) != 1 || tokens[0].Label != "Aria" || tokens[0].Q != 0 {
		t.Fatalf("tokens after the rewind = %+v, %v", tokens, err)
	}
	if back == nil || !back.Reveals[hex.Coord{Q: 0, R: 0}] || back.Reveals[hex.Coord{Q: 4, R: 0}] {
		t.Fatalf("fog after the rewind = %+v", back)
	}
	if list, _ := store.Checkpoints(ctx, sess.ID); len(list) != 2 || list[1].Name != "Round 1" {
		t.Fatalf("checkpoints after the rewind = %+v", list)
	}
	// The log keeps what the rewind took back, and offers none of it for undo; what came before the
	// Checkpoint, and what is done after the rewind, still can be undone.
	commit(live.Write{Kind: domain.ActionTokenPlaced, Token: domain.Token{ID: domain.TokenID(uuid.New()), Label: "Wolf", Kind: domain.TokenEnemy, Q: 5}})
	log, err := store.SessionLog(ctx, sess.ID, 50)
	if err != nil {
		t.Fatal(err)
	}
	var offered []string
	for _, a := range log {
		if a.Undoable() {
			offered = append(offered, a.Kind+" "+a.Label)
		}
		rec, err := store.Action(ctx, sess.ID, a.Seq)
		if want := a.Seq > kept.ActionSeq && a.Seq < done.Action; err != nil || rec.Rewound != want {
			t.Fatalf("action %d (%s) rewound = %v, want %v (%v)", a.Seq, a.Kind, rec.Rewound, want, err)
		}
	}
	if got := strings.Join(offered, "; "); got != "token_placed Wolf; hexes_revealed ; token_placed Aria" {
		t.Fatalf("offered for undo = %s", got)
	}
	// A rewind whose Session cannot be read back from it is not made: nothing of it stays.
	gone := errors.New("gone")
	if _, err := store.Rewind(ctx, sess, kept, actor, dm, time.Now(), func(tx live.Store) error {
		if _, tokens, _, err := tx.Load(ctx, sess.ID); err != nil || len(tokens) != 1 {
			t.Errorf("inside the rewind the Session has %d tokens, %v", len(tokens), err)
		}
		return gone
	}); !errors.Is(err, gone) {
		t.Fatalf("a rewind that cannot be read back = %v", err)
	}
	if s, tokens, _, err := store.Load(ctx, sess.ID); err != nil || len(tokens) != 2 || s.Seq != done.Seq+1 {
		t.Fatalf("after a rewind that was not made: %d tokens, seq %d, %v", len(tokens), s.Seq, err)
	}
	// A Checkpoint of another Session puts nothing back here.
	other := kept
	other.ID = uuid.New()
	if _, err := store.Rewind(ctx, sess, other, actor, dm, time.Now(), readable); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("a rewind to a checkpoint that is not there = %v", err)
	}
	if s, _, _, err := store.Load(ctx, sess.ID); err != nil || s.Seq != done.Seq+1 {
		t.Fatalf("after a refused rewind the Session is at %d, %v", s.Seq, err)
	}

	// Every statement of keeping and putting back reports a database fault, and leaves the Session whole.
	for name, op := range map[string]func(s *pgstore.Store) error{
		"save": func(s *pgstore.Store) error {
			_, err := s.SaveCheckpoint(ctx, sess, domain.Checkpoint{ID: uuid.New(), Name: "Again", Kind: domain.CheckpointNamed, CreatedAt: time.Now()}, actor, dm)
			return err
		},
		"round": func(s *pgstore.Store) error {
			return s.MarkRound(ctx, sess, domain.Checkpoint{ID: uuid.New(), Name: "Round 2", Kind: domain.CheckpointRound, Round: 2, ActionSeq: done.Action, CreatedAt: time.Now()}, 2)
		},
		"list": func(s *pgstore.Store) error { _, err := s.Checkpoints(ctx, sess.ID); return err },
		"rewind": func(s *pgstore.Store) error {
			_, err := s.Rewind(ctx, sess, kept, actor, dm, time.Now(), readable)
			return err
		},
		"undo": func(s *pgstore.Store) error { _, err := s.NoUndo(ctx, tb.campaign); return err },
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(pgstore.NewFaulty(tb.pool, f))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	if _, tokens, _, err := store.Load(ctx, sess.ID); err != nil || len(tokens) != 1 {
		t.Fatalf("after the faults = %+v, %v", tokens, err)
	}
	// Only the latest rounds are kept; what the DM named is never let go for them.
	for i := range 4 {
		c := domain.Checkpoint{ID: uuid.New(), Name: "Late round", Kind: domain.CheckpointRound, Round: 10 + i, ActionSeq: done.Action + 100 + int64(i), CreatedAt: time.Now()}
		if err := store.MarkRound(ctx, sess, c, 2); err != nil {
			t.Fatal(err)
		}
	}
	list, err := store.Checkpoints(ctx, sess.ID)
	var rounds, named []int
	for _, c := range list {
		if c.Kind == domain.CheckpointRound {
			rounds = append(rounds, c.Round)
		} else {
			named = append(named, c.Round)
		}
	}
	// The Checkpoint the DM named is still there; one the faults made may have gone with a rewind.
	if err != nil || !slices.Equal(rounds, []int{12, 13}) || len(named) < 1 {
		t.Fatalf("rounds kept = %v, named %d, %v", rounds, len(named), err)
	}
}

// Whatever hangs off a row a Checkpoint keeps has to be kept too, or a rewind would delete it with its
// parent and never put it back. A new table that refers to a token, a Combat or an Effect belongs in
// the list, or in the short list here of what a rewind leaves alone on purpose.
func TestACheckpointKeepsEveryTableThatHangsOffWhatItKeeps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	kept := pgstore.CheckpointTables()
	// Rolls are opened again in place, never deleted; a Checkpoint is not part of itself.
	alone := []string{"play.roll_requests", "play.roll_dice"}
	rows, err := tb.pool.Query(ctx, `SELECT DISTINCT conrelid::regclass::text, confrelid::regclass::text FROM pg_constraint
		WHERE contype = 'f' AND confdeltype IN ('c', 'n') AND confrelid::regclass::text = ANY($1)`, kept)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(kept, child) && !slices.Contains(alone, child) {
			t.Errorf("%s hangs off %s, which a Checkpoint keeps, and is not kept itself", child, parent)
		}
		if at, by := slices.Index(kept, child), slices.Index(kept, parent); at >= 0 && at < by {
			t.Errorf("%s is kept before %s, which it refers to", child, parent)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(kept) < 30 || !strings.HasPrefix(kept[0], "play.tokens") {
		t.Fatalf("kept = %v", kept)
	}
}
