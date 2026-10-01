package pgstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

type closed struct{ ids []domain.SessionID }

func (c *closed) Close(id domain.SessionID) { c.ids = append(c.ids, id) }

func sessions(tb table, repo app.SessionRepository, c *closed) *app.Sessions {
	return &app.Sessions{Repo: repo, Members: pgstore.CampaignMembers{Store: campaignpg.New(tb.pool)}, Live: c, Now: time.Now}
}

func TestSessionsStartListEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	c := &closed{}
	s := sessions(tb, pgstore.New(tb.pool), c)
	one, err := s.Start(ctx, dm, tb.campaign)
	if err != nil || one.Number != 1 || one.Status != domain.SessionLive || one.GridRadius != 10 {
		t.Fatalf("start = %+v %v", one, err)
	}
	two, _ := s.Start(ctx, dm, tb.campaign)
	if two.Number != 2 {
		t.Fatalf("second = %+v", two)
	}
	list, err := s.List(ctx, player, tb.campaign)
	if err != nil || len(list) != 2 || list[0].Number != 2 {
		t.Fatalf("list = %+v %v", list, err)
	}
	if got, err := s.Get(ctx, player, tb.campaign, one.ID); err != nil || got.ID != one.ID {
		t.Fatalf("get = %+v %v", got, err)
	}
	ended, err := s.End(ctx, dm, tb.campaign, one.ID)
	if err != nil || ended.Status != domain.SessionEnded || len(c.ids) != 1 {
		t.Fatalf("end = %+v %v", ended, err)
	}
	if _, err := s.End(ctx, dm, tb.campaign, one.ID); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("end twice = %v", err)
	}
	checkSessionRefusals(t, s, tb, one.ID, two.ID)
}

func checkSessionRefusals(t *testing.T, s *app.Sessions, tb table, one, two domain.SessionID) {
	t.Helper()
	ctx := context.Background()
	refused := map[string]error{}
	_, refused["player start"] = s.Start(ctx, player, tb.campaign)
	_, refused["player end"] = s.End(ctx, player, tb.campaign, two)
	for name, err := range refused {
		if !errors.Is(err, apperr.ErrForbidden) {
			t.Errorf("%s: %v", name, err)
		}
	}
	missing := map[string]error{}
	_, missing["stranger start"] = s.Start(ctx, stranger, tb.campaign)
	_, missing["stranger get"] = s.Get(ctx, stranger, tb.campaign, one)
	_, missing["stranger list"] = s.List(ctx, stranger, tb.campaign)
	_, missing["unknown"] = s.Get(ctx, dm, tb.campaign, domain.SessionID(uuid.New()))
	_, _, _, missing["load"] = pgstore.New(tb.pool).Load(ctx, domain.SessionID(uuid.New()))
	for name, err := range missing {
		if !errors.Is(err, apperr.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEverySessionDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	base := sessions(tb, pgstore.New(tb.pool), &closed{})
	live1, _ := base.Start(ctx, dm, tb.campaign)
	store := pgstore.New(tb.pool)
	tok := domain.Token{ID: domain.TokenID(uuid.New()), Label: "A", Kind: domain.TokenEnemy}
	done, err := store.Commit(ctx, live1, nil, live.Write{Kind: domain.ActionTokenPlaced, Token: tok}, tb.dmMember(t), dm, time.Now())
	if err != nil || done.Seq != 1 || done.Action < 1 {
		t.Fatalf("commit = %+v %v", done, err)
	}
	m, err := store.InsertMap(ctx, domain.Map{CampaignID: tb.campaign, Name: "Crypt", Kind: domain.MapLocal, ImageKey: "k", ImageType: "image/png", Width: 400, Height: 300, HexSize: 40, OriginX: 35, OriginY: 40}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	board, err := store.LoadMap(ctx, tb.campaign, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	mid := m.ID
	if _, err := store.Commit(ctx, live1, board, live.Write{Kind: domain.ActionMapSet, MapID: &mid}, tb.dmMember(t), dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	light := domain.MapLight{ID: domain.LightID(uuid.New()), At: hex.Coord{Q: 1, R: 0}, BrightFt: 5, DimFt: 10}
	writes := []live.Write{
		{Kind: domain.ActionHexesRevealed, Hexes: []hex.Coord{{Q: 0, R: 0}}, AutoReveal: []hex.Coord{{Q: 1, R: 0}}},
		{Kind: domain.ActionHexesConcealed, Hexes: []hex.Coord{{Q: 0, R: 0}}},
		{Kind: domain.ActionWallsSet, Hexes: []hex.Coord{{Q: 2, R: 0}}},
		{Kind: domain.ActionWallsCleared, Hexes: []hex.Coord{{Q: 2, R: 0}}},
		{Kind: domain.ActionLightPlaced, Light: light},
		{Kind: domain.ActionLightRemoved, Light: light},
		{Kind: domain.ActionAmbientSet, Ambient: domain.AmbientDark},
		{Kind: domain.ActionLightPlaced, Light: domain.MapLight{ID: domain.LightID(uuid.New()), At: hex.Coord{Q: 3, R: 0}, DimFt: 10}},
		{Kind: domain.ActionWallsSet, Hexes: []hex.Coord{{Q: 4, R: 0}}},
		{Kind: domain.ActionMapSet},
	}
	for _, w := range writes {
		if _, err := store.Commit(ctx, live1, board, w, tb.dmMember(t), dm, time.Now()); err != nil {
			t.Fatalf("%s: %v", w.Kind, err)
		}
	}
	reloaded, _ := store.LoadMap(ctx, tb.campaign, m.ID)
	if !reloaded.Reveals[hex.Coord{Q: 1, R: 0}] || reloaded.Reveals[hex.Coord{Q: 0, R: 0}] || len(reloaded.Lights) != 1 ||
		!reloaded.Walls[hex.Coord{Q: 4, R: 0}] || reloaded.Walls[hex.Coord{Q: 2, R: 0}] || reloaded.Map.Ambient != domain.AmbientDark {
		t.Fatalf("map state = %+v", reloaded)
	}
	if _, err := store.Commit(ctx, live1, board, live.Write{Kind: domain.ActionMapSet, MapID: &mid}, tb.dmMember(t), dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	if sess, _, loaded, err := store.Load(ctx, live1.ID); err != nil || loaded == nil || *sess.MapID != mid {
		t.Fatalf("load with map = %+v %v", loaded, err)
	}
	fight := func(status string) live.Write {
		me := tb.dmMember(t)
		roll := domain.Roll{
			ID: domain.RollID(uuid.New()), CampaignID: tb.campaign, Purpose: "Initiative for A", Notation: "1d20", RequestedBy: me.Name,
			Roller: me, Status: domain.StatusPending, Dice: []domain.Die{{No: 0, Group: 0, Faces: 20}},
		}
		twelve := 12
		c := &domain.Combat{
			ID: domain.CombatID(uuid.New()), Status: status, Round: 1, Turn: 12, StartedAt: time.Now(),
			Combatants: []domain.Combatant{{ID: domain.CombatantID(uuid.New()), TokenID: tok.ID, RollID: roll.ID, SpeedFt: 30, Initiative: &twelve}},
		}
		kind := domain.ActionCombatStarted
		if status == domain.CombatEnded {
			c.EndedAt, kind = time.Now(), domain.ActionCombatEnded
		}
		return live.Write{Kind: kind, Combat: c, Rolls: []domain.Roll{roll}}
	}
	running := fight(domain.CombatActive)
	if _, err := store.Commit(ctx, live1, nil, running, tb.dmMember(t), dm, time.Now()); err != nil {
		t.Fatal(err)
	}
	if c, err := store.LoadCombat(ctx, live1.ID); err != nil || c.Turn != 12 || *c.Combatants[0].Initiative != 12 || c.Combatants[0].RollID != running.Rolls[0].ID {
		t.Fatalf("load combat = %+v %v", c, err)
	}
	unreachable := tb.pool.Config().Copy()
	unreachable.ConnConfig.Port, unreachable.ConnConfig.Fallbacks = 1, nil
	gone, err := pgxpool.NewWithConfig(ctx, unreachable)
	if err != nil {
		t.Fatal(err)
	}
	defer gone.Close()
	if _, err := (pgstore.Owner{Pool: gone}).Acquire(ctx, live1.ID); err == nil {
		t.Fatal("acquire without a database")
	}
	ops := map[string]func(s *app.Sessions, repo *pgstore.Store) error{
		"start": func(s *app.Sessions, _ *pgstore.Store) error { _, err := s.Start(ctx, dm, tb.campaign); return err },
		"list":  func(s *app.Sessions, _ *pgstore.Store) error { _, err := s.List(ctx, dm, tb.campaign); return err },
		"end": func(s *app.Sessions, _ *pgstore.Store) error {
			fresh, err := base.Start(ctx, dm, tb.campaign)
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.End(ctx, dm, tb.campaign, fresh.ID)
			return err
		},
		"load": func(_ *app.Sessions, repo *pgstore.Store) error { _, _, _, err := repo.Load(ctx, live1.ID); return err },
		"commit": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, live1, board, live.Write{Kind: domain.ActionHexesRevealed, Hexes: []hex.Coord{{Q: 0, R: 1}}, AutoReveal: []hex.Coord{{Q: 1, R: 1}}}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"conceal": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, live1, board, live.Write{Kind: domain.ActionHexesConcealed, Hexes: []hex.Coord{{Q: 0, R: 1}}}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"walls": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, live1, board, live.Write{Kind: domain.ActionWallsSet, Hexes: []hex.Coord{{Q: 0, R: 1}}}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"token": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, live1, nil, live.Write{Kind: domain.ActionTokenMoved, Token: tok}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"combat": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.Commit(ctx, live1, nil, fight(domain.CombatEnded), tb.dmMember(t), dm, time.Now())
			return err
		},
		"effects": func(_ *app.Sessions, repo *pgstore.Store) error {
			src := tok.ID
			fx := domain.Effects{
				Active: []domain.Effect{{ID: domain.EffectID(uuid.New()), Target: tok.ID, Source: &src, Slug: "bless", Name: "Bless", RoundsLeft: 3, SaveAbility: "wisdom", SaveDC: 12}},
				Manual: []domain.ManualPrompt{{ID: uuid.New(), Text: "Resolve it."}},
			}
			roll := running.Rolls[0].ID
			fx.Saves = []domain.PendingSave{{RollID: roll, Effect: fx.Active[0].ID, DC: 12}}
			_, err := repo.Commit(ctx, live1, nil, live.Write{Kind: domain.ActionEffectApplied, Token: tok, Effects: &fx}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"terrain": func(_ *app.Sessions, repo *pgstore.Store) error {
			roll := running.Rolls[0].ID
			cast := &domain.AreaCast{
				ID: uuid.New(), Caster: tok.ID, Spell: "fireball", DC: 13, DamageRoll: &roll, Hexes: []hex.Coord{{Q: 0, R: 0}},
				Targets: []domain.AreaTarget{{Token: tok.ID, SaveRoll: &roll}},
			}
			w := live.Write{
				Kind: domain.ActionAreaCast, Token: tok, Cast: cast, SaveCast: true, SaveSurfaces: true,
				Surfaces: map[hex.Coord]domain.Surface{{Q: 1, R: 1}: {Kind: surface.Fire, RoundsLeft: 2}, {Q: 2, R: 1}: {Kind: surface.Water}},
			}
			if _, err := repo.Commit(ctx, live1, nil, w, tb.dmMember(t), dm, time.Now()); err != nil {
				return err
			}
			for _, h := range []int{10, 0} {
				el := live.Write{Kind: domain.ActionElevationSet, Hexes: []hex.Coord{{Q: 0, R: 0}}, ElevationFt: h}
				if _, err := repo.Commit(ctx, live1, board, el, tb.dmMember(t), dm, time.Now()); err != nil {
					return err
				}
			}
			_, err := repo.Commit(ctx, live1, nil, live.Write{Kind: domain.ActionAreaResolved, Token: tok, SaveCast: true}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"table": func(_ *app.Sessions, repo *pgstore.Store) error {
			tbl := domain.DefaultTable()
			tbl.Scene, tbl.MapID = domain.SceneWorld, &mid
			_, err := repo.Commit(ctx, live1, nil, live.Write{Kind: domain.ActionTableSet, Table: &tbl}, tb.dmMember(t), dm, time.Now())
			return err
		},
		"loadtable": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.LoadTable(ctx, live1.ID)
			return err
		},
		"loadterrain": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, _, err := repo.LoadTerrain(ctx, live1.ID)
			return err
		},
		"loadeffects": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.LoadEffects(ctx, live1.ID)
			return err
		},
		"observations": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.Observations(ctx, live1.ID)
			return err
		},
		"loadcombat": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.LoadCombat(ctx, live1.ID)
			return err
		},
		"loadmap": func(_ *app.Sessions, repo *pgstore.Store) error {
			_, err := repo.LoadMap(ctx, tb.campaign, m.ID)
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			repo := pgstore.NewFaulty(tb.pool, f)
			err := op(sessions(tb, repo, &closed{}), repo)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

func TestRunningSessionsDoNotStarveThePool(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tb := setup(t)
	small := tb.pool.Config().Copy()
	small.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, small)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for range 5 {
		release, err := (pgstore.Owner{Pool: pool}).Acquire(ctx, domain.SessionID(uuid.New()))
		if err != nil {
			t.Fatal(err)
		}
		defer release()
	}
	quick, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(quick); err != nil {
		t.Fatalf("five running Sessions left no connection for requests: %v", err)
	}
}
