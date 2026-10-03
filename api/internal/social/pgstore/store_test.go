package pgstore_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
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

// Every Dice Set operation reports a database fault at any of its calls.
func TestEveryDiceSetDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	pool := db.Pool()
	account(t, pool, "aria")
	account(t, pool, "bram")
	blobs := storage.Dir{Path: t.TempDir()}
	base := &app.Service{Repo: pgstore.New(pool), Now: time.Now, Blobs: blobs}
	design := domain.DiceDesign{Dice: map[string]domain.DieLook{"d20": {Pattern: "marble", Body: "#102030", Numbers: "#f0e0d0", Image: nil}}}
	set, err := base.CreateDiceSet(ctx, "aria", "Embers", design)
	if err != nil {
		t.Fatal(err)
	}
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	everyone := func() error { _, err := base.ShareDiceSet(ctx, "aria", set.ID, domain.SharingEveryone); return err }
	ops := map[string]func(s *app.Service) error{
		"create": func(s *app.Service) error { _, err := s.CreateDiceSet(ctx, "aria", "Ash", design); return err },
		"list":   func(s *app.Service) error { _, _, err := s.DiceSets(ctx, "aria"); return err },
		"edit": func(s *app.Service) error {
			_, err := s.EditDiceSet(ctx, "aria", set.ID, "Cinders", design)
			return err
		},
		"share": func(s *app.Service) error {
			_, err := s.ShareDiceSet(ctx, "aria", set.ID, domain.SharingEveryone)
			return err
		},
		"shared": func(s *app.Service) error { _, err := s.SharedDiceSets(ctx, "bram"); return err },
		"copy":   func(s *app.Service) error { _, err := s.CopyDiceSet(ctx, "bram", set.ID); return err },
		"choose": func(s *app.Service) error { return s.ChooseDiceSet(ctx, "aria", &set.ID) },
		"chosen": func(s *app.Service) error { _, _, err := s.DiceSets(ctx, "aria"); return err },
		"plain":  func(s *app.Service) error { return s.ChooseDiceSet(ctx, "aria", nil) },
		"picture": func(s *app.Service) error {
			_, err := s.SetDiceSetPicture(ctx, "aria", set.ID, picture.Bytes())
			return err
		},
		"shown":   func(s *app.Service) error { _, _, err := s.DiceSetPicture(ctx, "bram", set.ID, false); return err },
		"waiting": func(s *app.Service) error { _, err := s.DiceSetsToReview(ctx); return err },
		"review": func(s *app.Service) error {
			if err := everyone(); err != nil {
				return err
			}
			waiting, err := base.Repo.DiceSet(ctx, set.ID)
			if err != nil {
				return err
			}
			_, err = s.ReviewDiceSet(ctx, set.ID, true, waiting.Image.Version())
			return err
		},
		"bare":   func(s *app.Service) error { _, err := s.ClearDiceSetPicture(ctx, "aria", set.ID); return err },
		"delete": func(s *app.Service) error { return s.DeleteDiceSet(ctx, "aria", set.ID) },
	}
	for _, name := range []string{"create", "list", "edit", "share", "shared", "copy", "choose", "chosen", "plain", "picture", "review", "shown", "waiting", "bare", "delete"} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := ops[name](&app.Service{Repo: pgstore.NewFaulty(pool, f), Now: time.Now, Blobs: blobs})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	// The copy outlives the set it was taken from, with the look it had when it was taken.
	kept, _, err := base.DiceSets(ctx, "bram")
	if err != nil || len(kept) != 1 || kept[0].Name != "Cinders" || !kept[0].Copy() || kept[0].Design.Dice["d20"].Pattern != "marble" {
		t.Fatalf("bram's copy = %+v, %v", kept, err)
	}
}

type brokenBlobs struct{}

func (brokenBlobs) Put(context.Context, string, string, []byte) error { return errors.New("disk full") }

func (brokenBlobs) Get(context.Context, string) ([]byte, error) { return nil, errors.New("disk gone") }

// A Dice Set dresses only dice there are, in patterns and colours there are, under a name that fits.
func TestWhatADiceSetMayBe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	account(t, db.Pool(), "aria")
	svc := &app.Service{Repo: pgstore.New(db.Pool()), Now: time.Now, Blobs: brokenBlobs{}}
	look := func(die, pattern, body, numbers string, at *domain.DiePlacement) domain.DiceDesign {
		return domain.DiceDesign{Dice: map[string]domain.DieLook{die: {Pattern: pattern, Body: body, Numbers: numbers, Image: at}}}
	}
	placed := func(x, y, scale, turn float64) *domain.DiePlacement {
		return &domain.DiePlacement{X: x, Y: y, Scale: scale, Rotation: turn}
	}
	good := look("d20", "stripes", "#000000", "#FFffFF", placed(0, 1, 0.1, -360))
	set, err := svc.CreateDiceSet(ctx, "aria", "  Embers  ", good)
	if err != nil || set.Name != "Embers" {
		t.Fatalf("create = %+v, %v", set, err)
	}
	if _, err := svc.EditDiceSet(ctx, "aria", set.ID, strings.Repeat("é", 60), look("d100", "plain", "#abcdef", "#012345", placed(1, 0, 10, 360))); err != nil {
		t.Fatalf("a name of 60 and a picture at the far corner: %v", err)
	}
	for name, d := range map[string]domain.DiceDesign{
		"a d7":              look("d7", "plain", "#000000", "#ffffff", nil),
		"no such pattern":   look("d20", "glitter", "#000000", "#ffffff", nil),
		"a named colour":    look("d20", "plain", "red", "#ffffff", nil),
		"short numbers":     look("d20", "plain", "#000000", "#fff", nil),
		"left of the sheet": look("d20", "plain", "#000000", "#ffffff", placed(-0.01, 0, 1, 0)),
		"right of it":       look("d20", "plain", "#000000", "#ffffff", placed(1.01, 0, 1, 0)),
		"above it":          look("d20", "plain", "#000000", "#ffffff", placed(0, -0.01, 1, 0)),
		"below it":          look("d20", "plain", "#000000", "#ffffff", placed(0, 1.01, 1, 0)),
		"too small":         look("d20", "plain", "#000000", "#ffffff", placed(0, 0, 0.09, 0)),
		"too large":         look("d20", "plain", "#000000", "#ffffff", placed(0, 0, 10.1, 0)),
		"turned too far":    look("d20", "plain", "#000000", "#ffffff", placed(0, 0, 1, 361)),
		"turned back far":   look("d20", "plain", "#000000", "#ffffff", placed(0, 0, 1, -361)),
	} {
		if _, err := svc.CreateDiceSet(ctx, "aria", "Bad", d); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("create with %s = %v", name, err)
		}
		if _, err := svc.EditDiceSet(ctx, "aria", set.ID, "Bad", d); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("edit to %s = %v", name, err)
		}
	}
	for _, name := range []string{"", "   ", strings.Repeat("é", 61), strings.Repeat("🎲", 31)} {
		if _, err := svc.CreateDiceSet(ctx, "aria", name, good); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("create named %q = %v", name, err)
		}
		if _, err := svc.EditDiceSet(ctx, "aria", set.ID, name, good); !errors.Is(err, domain.ErrInvalid) {
			t.Fatalf("rename to %q = %v", name, err)
		}
	}
	if _, err := svc.CreateDiceSet(ctx, "aria", strings.Repeat("🎲", 30), good); err != nil {
		t.Fatalf("thirty dice are sixty units: %v", err)
	}
	if _, err := svc.ShareDiceSet(ctx, "aria", set.ID, "the world"); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("shared with nobody we know = %v", err)
	}
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	// A picture that could not be stored is not put on the set.
	if _, err := svc.SetDiceSetPicture(ctx, "aria", set.ID, picture.Bytes()); err == nil || errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("a picture the store refused = %v", err)
	}
	if got, err := svc.Repo.DiceSet(ctx, set.ID); err != nil || got.Image != nil {
		t.Fatalf("after a failed upload = %+v, %v", got.Image, err)
	}
	if _, _, err := svc.DiceSetPicture(ctx, "aria", set.ID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("the picture of a set without one = %v", err)
	}
	if _, _, err := svc.DiceSetPicture(ctx, "nobody", set.ID, false); !errors.Is(err, domain.ErrNoAccount) {
		t.Fatalf("a picture for someone without an Account = %v", err)
	}
}

// A roll is made with the Dice Set its roller chose. The picture on it shows to others only once an
// Admin approved it, and a copy of an approved set carries that approval with it.
func TestTheDiceSetARollIsMadeWith(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	for _, name := range []string{"aria", "bram", "cara"} {
		account(t, db.Pool(), name)
	}
	svc := &app.Service{Repo: pgstore.New(db.Pool()), Now: time.Now, Blobs: storage.Dir{Path: t.TempDir()}}
	design := domain.DiceDesign{Dice: map[string]domain.DieLook{"d20": {Pattern: "marble", Body: "#102030", Numbers: "#f0e0d0", Image: nil}}}
	var picture bytes.Buffer
	if err := png.Encode(&picture, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.ChosenDiceSet(ctx, "aria"); err != nil || got != nil {
		t.Fatalf("before choosing = %+v, %v", got, err)
	}
	if _, err := svc.ChosenDiceSet(ctx, "nobody"); !errors.Is(err, domain.ErrNoAccount) {
		t.Fatalf("the dice of someone without an Account = %v", err)
	}
	set, _ := svc.CreateDiceSet(ctx, "aria", "Embers", design)
	for _, step := range []func() error{
		func() error { _, err := svc.SetDiceSetPicture(ctx, "aria", set.ID, picture.Bytes()); return err },
		func() error { _, err := svc.ShareDiceSet(ctx, "aria", set.ID, domain.SharingEveryone); return err },
		func() error { return svc.ChooseDiceSet(ctx, "aria", &set.ID) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	// It waits for an Admin: Aria rolls with it, and nobody else may see its picture.
	if got, err := svc.ChosenDiceSet(ctx, "aria"); err != nil || got == nil || got.ID != set.ID || got.Cleared() {
		t.Fatalf("a set that waits = %+v, %v", got, err)
	}
	if _, _, err := svc.DiceSetPicture(ctx, "cara", set.ID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger fetches a picture that waits = %v", err)
	}
	waiting, _ := svc.ChosenDiceSet(ctx, "aria")
	// The store itself refuses a decision on any picture but the one on the set, whatever was read before.
	if err := svc.Repo.SetDiceSetReview(ctx, set.ID, domain.ReviewApproved, "sha256/ffffffffffffffff.png", time.Now()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("approving a picture that is not on the set = %v", err)
	}
	if _, err := svc.ReviewDiceSet(ctx, set.ID, true, "ffffffffffff"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("approving a picture the Admin did not see = %v", err)
	}
	if got, _ := svc.ChosenDiceSet(ctx, "aria"); got == nil || got.Review != domain.ReviewPending {
		t.Fatalf("after a refused decision = %+v", got)
	}
	if _, err := svc.ReviewDiceSet(ctx, set.ID, true, waiting.Image.Version()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Repo.SetDiceSetReview(ctx, set.ID, domain.ReviewRejected, waiting.Image.Key, time.Now()); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("deciding again on a set that no longer waits = %v", err)
	}
	if got, _ := svc.ChosenDiceSet(ctx, "aria"); got == nil || !got.Cleared() {
		t.Fatalf("an approved set = %+v", got)
	}
	// Bram's copy carries the approval: Cara, who knows neither, may see its picture on his rolls.
	copied, err := svc.CopyDiceSet(ctx, "bram", set.ID)
	if err != nil || !copied.Cleared() || copied.Sharing != domain.SharingPrivate {
		t.Fatalf("a copy of an approved set = %+v, %v", copied, err)
	}
	if _, data, err := svc.DiceSetPicture(ctx, "cara", copied.ID, false); err != nil || !bytes.Equal(data, picture.Bytes()) {
		t.Fatalf("a stranger fetches the picture on a copy of an approved set = %v", err)
	}
	if list, err := svc.DiceSetsToReview(ctx); err != nil || len(list) != 0 {
		t.Fatalf("a copy waits for nobody: %+v, %v", list, err)
	}
	// Aria goes back to Friends only: hers is no longer everyone's, and the copy stays as it was.
	if _, err := svc.ShareDiceSet(ctx, "aria", set.ID, domain.SharingFriends); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.ChosenDiceSet(ctx, "aria"); got == nil || got.Cleared() {
		t.Fatalf("a set shared with Friends only = %+v", got)
	}
	if _, _, err := svc.DiceSetPicture(ctx, "cara", set.ID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger fetches the picture of a Friends-only set = %v", err)
	}
	if _, _, err := svc.DiceSetPicture(ctx, "cara", copied.ID, false); err != nil {
		t.Fatalf("the copy's picture after the original went Friends-only = %v", err)
	}
	// A copy of a set nobody approved carries no approval: its picture stays its owner's.
	if err := svc.Request(ctx, "cara", "aria"); err != nil {
		t.Fatal(err)
	}
	page, _ := svc.Friends(ctx, "aria")
	if err := svc.Accept(ctx, "aria", page.Incoming[0].ID); err != nil {
		t.Fatal(err)
	}
	plain, err := svc.CopyDiceSet(ctx, "cara", set.ID)
	if err != nil || plain.Cleared() || plain.Review != domain.ReviewNone {
		t.Fatalf("a Friend's copy of an unapproved set = %+v, %v", plain, err)
	}
	if _, _, err := svc.DiceSetPicture(ctx, "bram", plain.ID, false); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger fetches the picture on a copy nobody approved = %v", err)
	}
	// A set without a picture has nothing to clear, whatever its review says.
	if (domain.DiceSet{Review: domain.ReviewApproved}).Cleared() {
		t.Fatal("a set without a picture is cleared")
	}
}

// Where a set stands with the Admins follows from the set as it is when it changes, not from what was
// read before: a picture and a sharing that arrive together cannot slip past the review.
func TestTheReviewOfADiceSetFollowsTheSetItself(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	account(t, db.Pool(), "aria")
	repo := pgstore.New(db.Pool())
	svc := &app.Service{Repo: repo, Now: time.Now, Blobs: storage.Dir{Path: t.TempDir()}}
	set, err := svc.CreateDiceSet(ctx, "aria", "Embers", domain.DiceDesign{Dice: map[string]domain.DieLook{}})
	if err != nil {
		t.Fatal(err)
	}
	crest := &domain.Picture{Key: "sha256/0123456789abcdef.png", Type: "image/png"}
	review := func() string {
		t.Helper()
		d, err := repo.DiceSet(ctx, set.ID)
		if err != nil {
			t.Fatal(err)
		}
		return d.Sharing + " " + d.Review
	}
	steps := []struct {
		what string
		do   func() error
		want string
	}{
		{"shared with everyone without a picture", func() error { return repo.SetDiceSetSharing(ctx, set.ID, domain.SharingEveryone, time.Now()) }, "everyone none"},
		{"a picture on a set shared with everyone", func() error { return repo.SetDiceSetImage(ctx, set.ID, crest, time.Now()) }, "everyone pending"},
		{"approved", func() error { return repo.SetDiceSetReview(ctx, set.ID, domain.ReviewApproved, crest.Key, time.Now()) }, "everyone approved"},
		{"the same picture again", func() error { return repo.SetDiceSetImage(ctx, set.ID, crest, time.Now()) }, "everyone pending"},
		{"back to Friends", func() error { return repo.SetDiceSetSharing(ctx, set.ID, domain.SharingFriends, time.Now()) }, "friends none"},
		{"to everyone with the picture on", func() error { return repo.SetDiceSetSharing(ctx, set.ID, domain.SharingEveryone, time.Now()) }, "everyone pending"},
		{"the picture off", func() error { return repo.SetDiceSetImage(ctx, set.ID, nil, time.Now()) }, "everyone none"},
		{"private with a picture", func() error {
			if err := repo.SetDiceSetSharing(ctx, set.ID, domain.SharingPrivate, time.Now()); err != nil {
				return err
			}
			return repo.SetDiceSetImage(ctx, set.ID, crest, time.Now())
		}, "private none"},
	}
	for _, step := range steps {
		if err := step.do(); err != nil {
			t.Fatalf("%s: %v", step.what, err)
		}
		if got := review(); got != step.want {
			t.Fatalf("%s = %s, want %s", step.what, got, step.want)
		}
	}
	if crest.Version() != "0123456789ab" {
		t.Fatalf("version = %s", crest.Version())
	}
}
