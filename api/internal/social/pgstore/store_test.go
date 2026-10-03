package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/pgstore"
)

func account(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(), `INSERT INTO identity.accounts (id, subject, username, nickname, email, created_at)
		VALUES ($1, $2, $2, $2, $2 || '@example.org', now())`, id, name); err != nil {
		t.Fatal(err)
	}
	return id
}

// Every Friends operation reports a database fault at any of its calls instead of half-applying.
func TestEveryFriendsDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	account(t, pool, "aria")
	bram := account(t, pool, "bram")
	cara := account(t, pool, "cara")
	base := &app.Service{Repo: pgstore.New(pool), Now: time.Now}
	for _, step := range []func() error{
		func() error { return base.Request(ctx, "bram", "aria") },
		func() error { return base.Request(ctx, "cara", "aria") },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	page, _ := base.Friends(ctx, "aria")
	fromBram, fromCara := page.Incoming[0].ID, page.Incoming[1].ID
	if page.Incoming[0].Person.ID != bram {
		fromBram, fromCara = fromCara, fromBram
	}
	ops := map[string]func(s *app.Service) error{
		"friends": func(s *app.Service) error { _, err := s.Friends(ctx, "aria"); return err },
		"request": func(s *app.Service) error { return s.Request(ctx, "aria", "cara") },
		"accept":  func(s *app.Service) error { return s.Accept(ctx, "aria", fromBram) },
		"unfriend": func(s *app.Service) error {
			return s.Unfriend(ctx, "aria", bram)
		},
		"decline": func(s *app.Service) error { return s.Decline(ctx, "aria", fromCara, true) },
		"unblock": func(s *app.Service) error { return s.Unblock(ctx, "aria", cara) },
	}
	for _, name := range []string{"friends", "accept", "unfriend", "decline", "unblock", "request"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	if err := base.Request(ctx, "bram", "cara"); err != nil {
		t.Fatal(err)
	}
	sent, _ := base.Friends(ctx, "bram")
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		err := (&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now}).Cancel(ctx, "bram", sent.Outgoing[0].ID)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatalf("cancel: %v", err)
		}
		return err
	})
}

// A Location shows only to its Campaign's DMs, who may also mention it; anything else is no Mention.
func TestLocationMentions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	aria, bram := account(t, pool, "aria"), account(t, pool, "bram")
	campaign, world, node := uuid.New(), uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO campaign.campaigns (id, name, created_by) VALUES ($1, 'Morvain', 'aria')`, []any{campaign}},
		{`INSERT INTO campaign.members (campaign_id, auth_subject, display_name, role) VALUES ($1, 'aria', 'Aria', 'dm'), ($1, 'bram', 'Bram', 'player')`, []any{campaign}},
		{`INSERT INTO campaign.maps (id, campaign_id, name, image_key, image_type, width_px, height_px, hex_size_px, origin_x, origin_y, kind)
			VALUES ($1, $2, 'The Reach', 'k', 'image/png', 100, 100, 20, 0, 0, 'world')`, []any{world, campaign}},
		{`INSERT INTO campaign.map_nodes (id, map_id, name, q, r) VALUES ($1, $2, 'Saltmarsh', 0, 0)`, []any{node, world}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("%s: %v", q.sql, err)
		}
	}
	s := pgstore.New(pool)
	m := domain.Mention{Kind: domain.MentionLocation, CampaignID: campaign, TargetID: node}
	if r, err := s.Resolve(ctx, aria, m); err != nil || !r.Open || r.Label != "Saltmarsh" || r.MapID != world {
		t.Fatalf("the DM sees = %+v %v", r, err)
	}
	if r, err := s.Resolve(ctx, bram, m); err != nil || r.Open || r.Label != "" {
		t.Fatalf("a player sees = %+v %v", r, err)
	}
	if _, err := s.Resolve(ctx, aria, domain.Mention{Kind: "proposal", CampaignID: campaign, TargetID: node}); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("an unknown kind = %v", err)
	}
	found, err := s.Mentionable(ctx, aria, "salt")
	if err != nil || len(found) != 1 || found[0].Kind != domain.MentionLocation || found[0].CampaignName != "Morvain" {
		t.Fatalf("mentionable = %+v %v", found, err)
	}
	if found, _ := s.Mentionable(ctx, bram, "salt"); len(found) != 0 {
		t.Fatalf("a player may mention = %+v", found)
	}
}

// Every Conversation operation reports a database fault at any of its calls.
func TestEveryConversationDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	account(t, pool, "aria")
	bram, cara := account(t, pool, "bram"), account(t, pool, "cara")
	base := &app.Service{Repo: pgstore.New(pool), Now: time.Now}
	for _, pair := range [][2]string{{"aria", "bram"}, {"aria", "cara"}} {
		_ = base.Request(ctx, pair[0], pair[1])
		page, _ := base.Friends(ctx, pair[1])
		if err := base.Accept(ctx, pair[1], page.Incoming[0].ID); err != nil {
			t.Fatal(err)
		}
	}
	direct, err := base.StartConversation(ctx, "aria", "", []domain.AccountID{bram})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base.Send(ctx, "aria", direct, "hello", nil); err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(s *app.Service) error{
		"start group": func(s *app.Service) error {
			_, err := s.StartConversation(ctx, "aria", "Party", []domain.AccountID{bram, cara})
			return err
		},
		"list":        func(s *app.Service) error { _, err := s.Conversations(ctx, "aria"); return err },
		"send":        func(s *app.Service) error { _, err := s.Send(ctx, "bram", direct, "hi", nil); return err },
		"read":        func(s *app.Service) error { _, err := s.Messages(ctx, "bram", direct, nil); return err },
		"mentionable": func(s *app.Service) error { _, err := s.Mentionable(ctx, "aria", ""); return err },
	}
	for _, name := range []string{"start group", "list", "send", "read", "mentionable"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}

// Every Notification operation reports a database fault at any of its calls.
func TestEveryNotificationDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	aria := account(t, pool, "aria")
	base := &app.Service{Repo: pgstore.New(pool), Now: time.Now}
	if err := base.Alert(ctx, aria, "password_set", "Your password changed", ""); err != nil {
		t.Fatal(err)
	}
	list, _, _ := base.Notifications(ctx, "aria")
	ops := map[string]func(s *app.Service) error{
		"notify": func(s *app.Service) error {
			return s.Notify(ctx, aria, domain.Notice{Kind: domain.KindFriendRequest, Title: "x", ActionLabel: "Open", ActionPath: "/friends"})
		},
		"list":  func(s *app.Service) error { _, _, err := s.Notifications(ctx, "aria"); return err },
		"read":  func(s *app.Service) error { return s.ReadNotification(ctx, "aria", list[0].ID) },
		"all":   func(s *app.Service) error { return s.ReadAll(ctx, "aria") },
		"prefs": func(s *app.Service) error { _, err := s.Preferences(ctx, "aria"); return err },
		"set": func(s *app.Service) error {
			_, err := s.SetPreferences(ctx, "aria", []domain.Preference{{Kind: domain.KindConversation, InApp: true, Push: false, Email: false}})
			return err
		},
		"enabled": func(s *app.Service) error {
			_, err := s.Enabled(ctx, aria, domain.KindConversation, domain.ChannelPush)
			return err
		},
	}
	for _, name := range []string{"notify", "list", "read", "all", "prefs", "set", "enabled"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	long := strings.Repeat("é", 300)
	if err := base.Notify(ctx, aria, domain.Notice{Kind: domain.KindConversation, Title: long, Body: long, ActionLabel: "Open", ActionPath: "/x"}); err != nil {
		t.Fatalf("a long notice is cut to fit: %v", err)
	}
	if ok, _ := base.Enabled(ctx, aria, domain.KindConversation, domain.ChannelPush); ok {
		t.Fatal("a chosen channel keeps its choice")
	}
}

type sink struct{ sent int }

func (k *sink) Send(context.Context, mail.Message) error { k.sent++; return nil }
func (k *sink) Push(string, string, string, string)      {}

// Email and push delivery, and the Digest, report a database fault at any of their calls.
func TestEveryDeliveryDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	aria := account(t, pool, "aria")
	out := &sink{}
	service := func(repo app.Repository) *app.Service {
		return &app.Service{Repo: repo, Now: time.Now, Mailer: out, Devices: out, BaseURL: "https://grimoire.example"}
	}
	base := service(pgstore.New(pool))
	if _, err := base.SetPreferences(ctx, "aria", []domain.Preference{{Kind: domain.KindConversation, InApp: true, Push: true, Email: true}}); err != nil {
		t.Fatal(err)
	}
	talk := domain.Notice{Kind: domain.KindConversation, Title: "Bram wrote", Body: "hi", ActionLabel: "Open", ActionPath: "/conversations", Dedupe: ""}
	ops := map[string]func(s *app.Service) error{
		"queue":    func(s *app.Service) error { return s.Notify(ctx, aria, talk) },
		"security": func(s *app.Service) error { return s.Alert(ctx, aria, "two_step_reset", "Reset", "") },
		"digest":   func(s *app.Service) error { return s.SendDigests(ctx) },
	}
	for _, name := range []string{"queue", "security", "queue", "digest"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](service(pgstore.NewFaulty(pool, f)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	if out.sent == 0 {
		t.Fatal("nothing was mailed")
	}
	if err := (&app.Service{Repo: pgstore.New(pool), Now: time.Now}).SendDigests(ctx); err != nil {
		t.Fatalf("no mailer, no Digest: %v", err)
	}
}

// Every Release Note operation reports a database fault at any of its calls.
func TestEveryReleaseDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	account(t, pool, "aria")
	base := &app.Service{Repo: pgstore.New(pool), Now: time.Now}
	scheduled, err := base.DraftRelease(ctx, "root", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base.PublishRelease(ctx, scheduled.ID, nil); err != nil {
		t.Fatal(err)
	}
	draft, _ := base.DraftRelease(ctx, "root", "1.1.0")
	ops := map[string]func(s *app.Service) error{
		"draft":   func(s *app.Service) error { _, err := s.DraftRelease(ctx, "root", "2.0.0"); return err },
		"list":    func(s *app.Service) error { _, err := s.Releases(ctx); return err },
		"edit":    func(s *app.Service) error { _, err := s.EditRelease(ctx, draft.ID, "Title", "Body"); return err },
		"unseen":  func(s *app.Service) error { _, err := s.UnseenRelease(ctx, "aria"); return err },
		"see":     func(s *app.Service) error { return s.SeeRelease(ctx, "aria", scheduled.ID) },
		"publish": func(s *app.Service) error { _, err := s.PublishRelease(ctx, draft.ID, nil); return err },
	}
	for _, name := range []string{"draft", "list", "edit", "unseen", "see", "publish"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	if _, err := base.EditRelease(ctx, draft.ID, " ", ""); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a blank title = %v", err)
	}
	if n, err := base.UnseenRelease(ctx, "aria"); err != nil || n == nil {
		t.Fatalf("unseen = %v %v", n, err)
	}
}

// A Notice for a subject rings the bell of the Account it signs in as; a subject without one has no bell.
func TestNotifyingASubject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	account(t, db.Pool(), "aria")
	s := &app.Service{Repo: pgstore.New(db.Pool()), Now: time.Now}
	n := domain.Notice{Kind: domain.KindProposal, Title: "Bram proposes Frost Lance", ActionLabel: "Review", ActionPath: "/campaigns/x/proposals/y"}
	if err := s.NotifySubject(ctx, "aria", n); err != nil {
		t.Fatal(err)
	}
	if err := s.NotifySubject(ctx, "nobody", n); err != nil {
		t.Fatalf("a subject without an Account: %v", err)
	}
	got, unread, err := s.Notifications(ctx, "aria")
	if err != nil || unread != 1 || got[0].Title != "Bram proposes Frost Lance" {
		t.Fatalf("aria's bell = %+v %d %v", got, unread, err)
	}
	pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
		err := (&app.Service{Repo: pgstore.NewFaulty(db.Pool(), f), Now: time.Now}).NotifySubject(ctx, "aria", n)
		if err != nil && !errors.Is(err, pgtest.ErrInjected) {
			t.Fatal(err)
		}
		return err
	})
}
