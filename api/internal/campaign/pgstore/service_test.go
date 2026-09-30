package pgstore_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	dmCaller     = caller.UI("dm-subject")
	playerCaller = caller.UI("player-subject")
	stranger     = caller.UI("stranger")
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func open(t *testing.T) *pg.Store {
	t.Helper()
	store, err := pg.Open(context.Background(), pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

func service(t *testing.T, repo app.Repository) (*app.Service, *clock) {
	t.Helper()
	c := &clock{now: time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)}
	n := 0
	tokens := func() (string, error) {
		n++
		return fmt.Sprintf("%043d", n), nil
	}
	return &app.Service{Repo: repo, Now: c.Now, Token: tokens, InviteTTL: time.Hour}, c
}

// table creates a Campaign run by dmCaller with playerCaller joined through an invite.
func table(t *testing.T, s *app.Service) domain.Detail {
	t.Helper()
	ctx := context.Background()
	d, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "  Curse of Strahd ", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := s.CreateInvite(ctx, dmCaller, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvite(ctx, playerCaller, inv.Token, "Ireena"); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestCreateMakesTheCreatorDM(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	if d.Name != "Curse of Strahd" || d.Ruleset != "srd-2024" || d.Me.Role != domain.RoleDM || len(d.Members) != 1 {
		t.Fatalf("created = %+v", d)
	}
	home, err := s.Get(ctx, playerCaller, d.ID)
	if err != nil || home.Me.Role != domain.RolePlayer || len(home.Members) != 2 || home.Members[0].Role != domain.RoleDM {
		t.Fatalf("player home = %+v %v", home, err)
	}
	for _, in := range []app.CreateInput{{Name: " ", DisplayName: "x"}, {Name: "ok", DisplayName: ""}} {
		if _, err := s.Create(ctx, dmCaller, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%+v: %v", in, err)
		}
	}
	old, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "Tomb", Ruleset: "srd-2014", DisplayName: "Joris"})
	if err != nil || old.Ruleset != "srd-2014" {
		t.Fatalf("2014 campaign = %+v %v", old, err)
	}
}

func TestListPagesNewestFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clk := service(t, pgstore.New(open(t).Pool()))
	for _, name := range []string{"One", "Two", "Three"} {
		clk.now = clk.now.Add(time.Minute)
		if _, err := s.Create(ctx, dmCaller, app.CreateInput{Name: name, DisplayName: "DM"}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.List(ctx, dmCaller, nil, 2)
	if err != nil || len(first) != 2 || first[0].Name != "Three" || first[0].MyRole != domain.RoleDM || first[0].MemberCount != 1 {
		t.Fatalf("first page = %+v %v", first, err)
	}
	rest, err := s.List(ctx, dmCaller, &domain.ListCursor{CreatedAt: first[1].CreatedAt, ID: first[1].ID}, 2)
	if err != nil || len(rest) != 1 || rest[0].Name != "One" {
		t.Fatalf("second page = %+v %v", rest, err)
	}
	if none, _ := s.List(ctx, stranger, nil, 10); len(none) != 0 {
		t.Fatalf("stranger sees %+v", none)
	}
}

func TestNonMembersAndPlayersAreRefused(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	name := "Renamed"
	checks := map[string]func(c caller.Caller) error{
		"get": func(c caller.Caller) error { _, err := s.Get(ctx, c, d.ID); return err },
		"update": func(c caller.Caller) error {
			_, err := s.Update(ctx, c, d.ID, app.UpdateInput{Name: &name})
			return err
		},
		"role":    func(c caller.Caller) error { _, err := s.SetRole(ctx, c, d.ID, d.Me.ID, domain.RolePlayer); return err },
		"remove":  func(c caller.Caller) error { return s.RemoveMember(ctx, c, d.ID, d.Me.ID) },
		"invite":  func(c caller.Caller) error { _, err := s.CreateInvite(ctx, c, d.ID); return err },
		"invites": func(c caller.Caller) error { _, err := s.Invites(ctx, c, d.ID); return err },
		"revoke":  func(c caller.Caller) error { return s.RevokeInvite(ctx, c, d.ID, domain.InviteID{}) },
	}
	for name, check := range checks {
		if err := check(stranger); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("stranger %s: %v", name, err)
		}
		if name == "get" {
			continue
		}
		if err := check(playerCaller); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("player %s: %v", name, err)
		}
	}
}

func TestUpdateChangesSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	name, ruleset := " Out of the Abyss ", "srd-2014"
	got, err := s.Update(ctx, dmCaller, d.ID, app.UpdateInput{Name: &name, Ruleset: &ruleset})
	if err != nil || got.Name != "Out of the Abyss" || got.Ruleset != "srd-2014" {
		t.Fatalf("updated = %+v %v", got, err)
	}
	if got, err := s.Update(ctx, dmCaller, d.ID, app.UpdateInput{}); err != nil || got.Name != "Out of the Abyss" {
		t.Fatalf("empty update = %+v %v", got, err)
	}
	blank := ""
	if _, err := s.Update(ctx, dmCaller, d.ID, app.UpdateInput{Name: &blank}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("blank name: %v", err)
	}
}

func TestRolesKeepAtLeastOneDM(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	home, _ := s.Get(ctx, dmCaller, d.ID)
	player := home.Members[1]
	if _, err := s.SetRole(ctx, dmCaller, d.ID, d.Me.ID, domain.RolePlayer); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("last DM stepped down: %v", err)
	}
	if err := s.RemoveMember(ctx, dmCaller, d.ID, d.Me.ID); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("last DM left: %v", err)
	}
	if _, err := s.SetRole(ctx, dmCaller, d.ID, player.ID, "owner"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("bad role: %v", err)
	}
	if _, err := s.SetRole(ctx, dmCaller, d.ID, domain.MemberID{}, domain.RoleDM); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown member: %v", err)
	}
	coDM, err := s.SetRole(ctx, dmCaller, d.ID, player.ID, domain.RoleDM)
	if err != nil || coDM.Role != domain.RoleDM {
		t.Fatalf("co-DM = %+v %v", coDM, err)
	}
	if _, err := s.SetRole(ctx, playerCaller, d.ID, d.Me.ID, domain.RolePlayer); err != nil {
		t.Fatalf("co-DM demotes the creator: %v", err)
	}
	if err := s.RemoveMember(ctx, playerCaller, d.ID, d.Me.ID); err != nil {
		t.Fatalf("DM removes a player: %v", err)
	}
	if _, err := s.Get(ctx, dmCaller, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("removed member still sees the campaign: %v", err)
	}
}

func TestPlayersMayLeaveButNotRemoveOthers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, _ := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	home, _ := s.Get(ctx, playerCaller, d.ID)
	if err := s.RemoveMember(ctx, playerCaller, d.ID, d.Me.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("player removed the DM: %v", err)
	}
	if err := s.RemoveMember(ctx, dmCaller, d.ID, domain.MemberID{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown member: %v", err)
	}
	if err := s.RemoveMember(ctx, playerCaller, d.ID, home.Me.ID); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, err := s.Get(ctx, playerCaller, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("left player still sees the campaign: %v", err)
	}
	if err := s.RemoveMember(ctx, dmCaller, domain.CampaignID{}, d.Me.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown campaign: %v", err)
	}
}

func TestInvitesExpireAndRevoke(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clk := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	inv, err := s.CreateInvite(ctx, dmCaller, d.ID)
	if err != nil || inv.CreatedByName != "Joris" || !inv.ExpiresAt.Equal(clk.now.Add(time.Hour)) {
		t.Fatalf("invite = %+v %v", inv, err)
	}
	pv, err := s.PreviewInvite(ctx, inv.Token)
	if err != nil || pv.CampaignName != "Curse of Strahd" || pv.InvitedBy != "Joris" {
		t.Fatalf("preview = %+v %v", pv, err)
	}
	open, _ := s.Invites(ctx, dmCaller, d.ID)
	if len(open) != 2 || open[0].CreatedByName != "Joris" {
		t.Fatalf("open invites = %+v", open)
	}
	id, err := s.AcceptInvite(ctx, dmCaller, inv.Token, "Someone else")
	if err != nil || id != d.ID {
		t.Fatalf("member re-accepting = %v %v", id, err)
	}
	if home, _ := s.Get(ctx, dmCaller, d.ID); home.Me.Role != domain.RoleDM || home.Me.DisplayName != "Joris" {
		t.Fatalf("re-accepting changed the member: %+v", home.Me)
	}
	if _, err := s.AcceptInvite(ctx, stranger, inv.Token, " "); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("blank name: %v", err)
	}
	if err := s.RevokeInvite(ctx, dmCaller, d.ID, inv.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.RevokeInvite(ctx, dmCaller, d.ID, inv.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("revoked twice: %v", err)
	}
	if _, err := s.AcceptInvite(ctx, stranger, inv.Token, "Late"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("revoked invite accepted: %v", err)
	}
}

func TestInvitesExpire(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, clk := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	fresh, _ := s.CreateInvite(ctx, dmCaller, d.ID)
	clk.now = clk.now.Add(2 * time.Hour)
	if _, err := s.PreviewInvite(ctx, fresh.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expired invite previewed: %v", err)
	}
	if open, _ := s.Invites(ctx, dmCaller, d.ID); len(open) != 0 {
		t.Fatalf("expired invites listed: %+v", open)
	}
}

func TestTokenFailureStopsInvite(t *testing.T) {
	t.Parallel()
	s, _ := service(t, pgstore.New(open(t).Pool()))
	d := table(t, s)
	boom := errors.New("no entropy")
	s.Token = func() (string, error) { return "", boom }
	if _, err := s.CreateInvite(context.Background(), dmCaller, d.ID); !errors.Is(err, boom) {
		t.Fatalf("token error = %v", err)
	}
	token, err := app.RandomToken()
	if err != nil || len(token) != 43 {
		t.Fatalf("random token = %q %v", token, err)
	}
	if app.NewService(nil).InviteTTL != app.DefaultInviteTTL {
		t.Fatal("default TTL")
	}
}

func TestEveryDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	base, _ := service(t, pgstore.New(db.Pool()))
	d := table(t, base)
	home, _ := base.Get(ctx, dmCaller, d.ID)
	player := home.Members[1]
	inv, _ := base.CreateInvite(ctx, dmCaller, d.ID)
	doomed, _ := base.CreateInvite(ctx, dmCaller, d.ID)
	name := "Renamed"
	ops := map[string]func(s *app.Service) error{
		"create": func(s *app.Service) error {
			_, err := s.Create(ctx, dmCaller, app.CreateInput{Name: "X", DisplayName: "Y"})
			return err
		},
		"list": func(s *app.Service) error { _, err := s.List(ctx, dmCaller, nil, 5); return err },
		"get":  func(s *app.Service) error { _, err := s.Get(ctx, dmCaller, d.ID); return err },
		"update": func(s *app.Service) error {
			_, err := s.Update(ctx, dmCaller, d.ID, app.UpdateInput{Name: &name})
			return err
		},
		"role": func(s *app.Service) error {
			_, err := s.SetRole(ctx, dmCaller, d.ID, player.ID, domain.RolePlayer)
			return err
		},
		"demote": func(s *app.Service) error {
			_, err := s.SetRole(ctx, dmCaller, d.ID, d.Me.ID, domain.RoleDM)
			return err
		},
		"invite":  func(s *app.Service) error { _, err := s.CreateInvite(ctx, dmCaller, d.ID); return err },
		"invites": func(s *app.Service) error { _, err := s.Invites(ctx, dmCaller, d.ID); return err },
		"preview": func(s *app.Service) error { _, err := s.PreviewInvite(ctx, inv.Token); return err },
		"accept": func(s *app.Service) error {
			_, err := s.AcceptInvite(ctx, playerCaller, inv.Token, "Ireena")
			return err
		},
		"revoke": func(s *app.Service) error { return s.RevokeInvite(ctx, dmCaller, d.ID, doomed.ID) },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			s, _ := service(t, pgstore.NewFaulty(db.Pool(), f))
			s.Token = app.RandomToken
			err := op(s)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	extra, _ := base.Create(ctx, dmCaller, app.CreateInput{Name: "Two DMs", DisplayName: "A"})
	inv2, _ := base.CreateInvite(ctx, dmCaller, extra.ID)
	_, _ = base.AcceptInvite(ctx, playerCaller, inv2.Token, "B")
	two, _ := base.Get(ctx, dmCaller, extra.ID)
	_, _ = base.SetRole(ctx, dmCaller, extra.ID, two.Members[1].ID, domain.RoleDM)
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		s, _ := service(t, pgstore.NewFaulty(db.Pool(), f))
		return s.RemoveMember(ctx, dmCaller, extra.ID, two.Members[1].ID)
	})
}
