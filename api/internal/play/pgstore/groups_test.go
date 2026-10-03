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

// A split party plays in the Session it split from and one Session for each group that left. A member
// belongs with a group their own party token is in; the Table Display with the group the DM has it
// follow; the DM wherever they look; and everyone back with the party once a group's Session is over.
func TestWhereEveryoneBelongsWhenThePartySplits(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	store := pgstore.New(tb.pool)
	sess, _ := sessions(tb, store, &closed{}).Start(ctx, dm, tb.campaign)
	actor := tb.dmMember(t)
	readable := func(live.Store) error { return nil }
	tower, err := store.InsertMap(ctx, domain.Map{CampaignID: tb.campaign, Name: "Tower", Kind: domain.MapLocal, ImageKey: "k", ImageType: "image/png", Width: 400, Height: 300, Grid: "hexes", GridStrength: 20, ScaleMiles: 6, HexSize: 40, OriginX: 35, OriginY: 40}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	aria := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Aria", Kind: domain.TokenParty, Controller: &tb.playerID}
	brom := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Brom", Kind: domain.TokenParty, Q: 1}
	rat := domain.Token{ID: domain.TokenID(uuid.New()), Label: "Rat", Kind: domain.TokenEnemy, Q: 2, Controller: &tb.playerID}
	for _, tok := range []domain.Token{aria, brom, rat} {
		if _, err := store.Commit(ctx, sess, nil, live.Write{Kind: domain.ActionTokenPlaced, Token: tok}, actor, dm, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	kept := domain.Checkpoint{ID: uuid.New(), Name: "Before", Kind: domain.CheckpointNamed, CreatedAt: time.Now()}
	if _, err := store.SaveCheckpoint(ctx, sess, kept, actor, dm); err != nil {
		t.Fatal(err)
	}
	// who asks: the player, the DM, or someone who is no DM and plays no token.
	playerM := domain.Member{ID: tb.playerID, Subject: "player", Name: "Tamsin", DM: false}
	watcher := domain.Member{ID: tb.dmID, Subject: "watcher", Name: "Watcher", DM: false}
	place := func(id domain.SessionID, m domain.Member, a live.Audience) domain.SessionID {
		t.Helper()
		at, err := store.Place(ctx, id, m, a)
		if err != nil {
			t.Fatalf("place of %s as %s: %v", m.Name, a, err)
		}
		return at
	}
	barred := func(id domain.SessionID, m domain.Member, a live.Audience) {
		t.Helper()
		if at, err := store.Place(ctx, id, m, a); !errors.Is(err, apperr.ErrForbidden) {
			t.Fatalf("%s as %s at %s = %s, %v", m.Name, a, uuid.UUID(id), uuid.UUID(at), err)
		}
	}
	// Together, everyone belongs in the one Session, and the party is one group.
	if g, err := store.Groups(ctx, sess); err != nil || len(g) != 1 || !g[0].Home || !g[0].Table || len(g[0].Tokens) != 2 || g[0].Tokens[0].Label != "Aria" || *g[0].Tokens[0].Controller != tb.playerID || g[0].Tokens[1].Controller != nil {
		t.Fatalf("a party that is together = %+v, %v", g, err)
	}
	for _, a := range []live.Audience{live.AudienceParty, live.AudienceTable, live.AudienceDM} {
		for _, m := range []domain.Member{playerM, watcher, actor} {
			if place(sess.ID, m, a) != sess.ID {
				t.Fatalf("a party that is together sends %s as %s elsewhere", m.Name, a)
			}
		}
	}

	// Aria goes to the tower.
	side, done, err := store.SplitParty(ctx, sess, "The tower", tower.ID, []domain.TokenPlace{{ID: aria.ID, Q: 3, R: 1}}, actor, dm, time.Now(), readable)
	if err != nil || done.Action < 1 || side.Parent == nil || *side.Parent != sess.ID || side.Group != "The tower" || side.MapID == nil || *side.MapID != tower.ID || side.Number != sess.Number+1 || side.Root() != sess.ID {
		t.Fatalf("split = %+v, %+v, %v", side, done, err)
	}
	if _, moved, _, err := store.Load(ctx, side.ID); err != nil || len(moved) != 1 || moved[0].Label != "Aria" || moved[0].Q != 3 || moved[0].R != 1 {
		t.Fatalf("the tower's tokens = %+v, %v", moved, err)
	}
	if _, left, _, err := store.Load(ctx, sess.ID); err != nil || len(left) != 2 {
		t.Fatalf("the crypt's tokens = %+v, %v", left, err)
	}
	if list, err := store.Checkpoints(ctx, sess.ID); err != nil || len(list) != 0 {
		t.Fatalf("checkpoints after the split = %+v, %v", list, err)
	}
	g, err := store.Groups(ctx, side)
	if err != nil || len(g) != 2 || !g[0].Home || g[0].Session != sess.ID || !g[0].Table || g[1].Home || g[1].Session != side.ID || g[1].Name != "The tower" || g[1].Table || len(g[0].Tokens) != 1 || len(g[1].Tokens) != 1 {
		t.Fatalf("groups = %+v, %v", g, err)
	}
	// Aria's player belongs in the tower, wherever they knock. The enemy token they also run does not
	// make the crypt theirs to watch: only a party token does.
	for _, id := range []domain.SessionID{sess.ID, side.ID} {
		if at := place(id, playerM, live.AudienceParty); at != side.ID {
			t.Fatalf("Aria's player, knocking at %s, belongs at %s", uuid.UUID(id), uuid.UUID(at))
		}
		// The DM is let in wherever they look, and their Table Display stays with the party it follows.
		if at := place(id, actor, live.AudienceDM); at != id {
			t.Fatalf("the DM, looking at %s, is sent to %s", uuid.UUID(id), uuid.UUID(at))
		}
		if at := place(id, actor, live.AudienceTable); at != sess.ID {
			t.Fatalf("the DM's table, opened at %s, belongs at %s", uuid.UUID(id), uuid.UUID(at))
		}
		// A member who plays no party token watches the Session the party split from, table and all.
		for _, a := range []live.Audience{live.AudienceParty, live.AudienceTable} {
			if at := place(id, watcher, a); at != sess.ID {
				t.Fatalf("someone with no token, knocking at %s as %s, belongs at %s", uuid.UUID(id), a, uuid.UUID(at))
			}
		}
		// The Table Display shows the party's Party Vision, which is not Aria's player's to see while
		// Aria is in the tower: a table of theirs is let in nowhere.
		barred(id, playerM, live.AudienceTable)
	}
	if _, err := store.Place(ctx, domain.SessionID(uuid.New()), playerM, live.AudienceParty); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("a place in a Session that is not there = %v", err)
	}

	// The Table Display follows the tower, and then the party again.
	if _, err := store.FollowTable(ctx, sess, &side.ID, actor, dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	// Now it shows the tower: that is Aria's player's own to see, the DM's as ever, and no longer the
	// watcher's, who watches the party in the crypt.
	for _, id := range []domain.SessionID{sess.ID, side.ID} {
		for _, m := range []domain.Member{playerM, actor} {
			if at := place(id, m, live.AudienceTable); at != side.ID {
				t.Fatalf("%s's table, opened at %s, belongs at %s", m.Name, uuid.UUID(id), uuid.UUID(at))
			}
		}
		barred(id, watcher, live.AudienceTable)
	}
	if g, _ := store.Groups(ctx, sess); g[0].Table || !g[1].Table {
		t.Fatalf("groups while the table follows the tower = %+v", g)
	}
	main, _, _, _ := store.Load(ctx, sess.ID)
	if main.Table == nil || *main.Table != side.ID {
		t.Fatalf("the Session's table = %v", main.Table)
	}

	// A group that cannot be read back is not brought back.
	gone := errors.New("gone")
	if _, err := store.RejoinParty(ctx, main, side.ID, []domain.TokenPlace{{ID: aria.ID, Q: 0, R: 1}}, actor, dm, time.Now(), func(live.Store) error { return gone }); !errors.Is(err, gone) {
		t.Fatalf("a rejoin that cannot be read back = %v", err)
	}
	if _, moved, _, _ := store.Load(ctx, side.ID); len(moved) != 1 {
		t.Fatalf("after a rejoin that was not made the tower has %d tokens", len(moved))
	}
	// Nor does a token that is not in the group come back with it, or a group that is not there.
	if _, err := store.RejoinParty(ctx, main, side.ID, []domain.TokenPlace{{ID: brom.ID}}, actor, dm, time.Now(), readable); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("bringing back a token that never left = %v", err)
	}
	if _, err := store.RejoinParty(ctx, main, domain.SessionID(uuid.New()), nil, actor, dm, time.Now(), readable); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("bringing back a group that is not there = %v", err)
	}

	// Every statement of splitting, following and coming back reports a database fault and leaves the
	// groups as they were.
	for name, op := range map[string]func(s *pgstore.Store) error{
		"groups": func(s *pgstore.Store) error { _, err := s.Groups(ctx, sess); return err },
		"place":  func(s *pgstore.Store) error { _, err := s.Place(ctx, sess.ID, playerM, live.AudienceParty); return err },
		"table":  func(s *pgstore.Store) error { _, err := s.Place(ctx, sess.ID, playerM, live.AudienceTable); return err },
		"follow": func(s *pgstore.Store) error {
			_, err := s.FollowTable(ctx, main, &side.ID, actor, dm, time.Now())
			return err
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
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, err := pgstore.NewFaulty(tb.pool, f).RejoinParty(ctx, main, side.ID, []domain.TokenPlace{{ID: aria.ID, Q: 0, R: 1}}, actor, dm, time.Now(), readable)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("rejoin: %v", err)
		}
		return err
	})
	// The group is back: the tower is over, the Table Display is with the party, and everyone belongs
	// in the one Session again.
	if _, back, _, err := store.Load(ctx, sess.ID); err != nil || len(back) != 3 {
		t.Fatalf("the crypt after the group came back = %+v, %v", back, err)
	}
	over, _, _, _ := store.Load(ctx, side.ID)
	whole, _, _, _ := store.Load(ctx, sess.ID)
	if over.Status != domain.SessionEnded || whole.Table != nil {
		t.Fatalf("the tower is %s, the table follows %v", over.Status, whole.Table)
	}
	for _, id := range []domain.SessionID{sess.ID, side.ID} {
		for name, at := range map[string]domain.SessionID{"a player": place(id, playerM, live.AudienceParty), "the table": place(id, playerM, live.AudienceTable), "the DM": place(id, actor, live.AudienceDM)} {
			if at != sess.ID {
				t.Fatalf("with the party together, %s knocking at %s belongs at %s", name, uuid.UUID(id), uuid.UUID(at))
			}
		}
	}
	if g, _ := store.Groups(ctx, whole); len(g) != 1 {
		t.Fatalf("groups after the group came back = %+v", g)
	}
	// Splitting faults too, and a split that cannot be read back leaves no Session behind.
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		_, _, err := pgstore.NewFaulty(tb.pool, f).SplitParty(ctx, whole, "Again", tower.ID, []domain.TokenPlace{{ID: brom.ID}}, actor, dm, time.Now(), readable)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("split: %v", err)
		}
		return err
	})
	if _, _, err := store.SplitParty(ctx, whole, "Never", tower.ID, []domain.TokenPlace{{ID: aria.ID}}, actor, dm, time.Now(), func(live.Store) error { return gone }); !errors.Is(err, gone) {
		t.Fatalf("a split that cannot be read back = %v", err)
	}
	if _, _, err := store.SplitParty(ctx, whole, "Nobody", tower.ID, []domain.TokenPlace{{ID: domain.TokenID(uuid.New())}}, actor, dm, time.Now(), readable); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("splitting off a token that is not there = %v", err)
	}
	var live1 int
	if err := tb.pool.QueryRow(ctx, "SELECT count(*) FROM play.sessions WHERE campaign_id = $1 AND status = 'live'", tb.campaign).Scan(&live1); err != nil || live1 != 2 {
		t.Fatalf("live Sessions = %d, %v", live1, err)
	}

	// Ending the Session the party split from ends every group's Session with it, and closes them all.
	shut := &closed{}
	if _, err := sessions(tb, store, shut).End(ctx, dm, tb.campaign, sess.ID); err != nil {
		t.Fatal(err)
	}
	if err := tb.pool.QueryRow(ctx, "SELECT count(*) FROM play.sessions WHERE campaign_id = $1 AND status = 'live'", tb.campaign).Scan(&live1); err != nil || live1 != 0 {
		t.Fatalf("live Sessions after the party's Session ended = %d, %v", live1, err)
	}
	if len(shut.ids) != 2 || shut.ids[0] != sess.ID {
		t.Fatalf("runtimes closed = %v", shut.ids)
	}
}
