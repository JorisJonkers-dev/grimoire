package pgstore_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func npcs(t *testing.T, repo app.Repository) (*app.NPCs, domain.Detail) {
	t.Helper()
	s, _ := service(t, repo)
	d := table(t, s)
	return &app.NPCs{Repo: repo, Now: func() time.Time { return time.Date(2026, 9, 30, 21, 0, 0, 0, time.UTC) }}, d
}

func strahd() app.NPCInput {
	return app.NPCInput{Name: " Strahd ", Title: "Count", Description: "A vampire.", DMNotes: "Wants Ireena.", Disposition: "hostile"}
}

func TestNPCRevisionsDiffAndRestore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	n, d := npcs(t, pgstore.New(open(t).Pool()))
	created, err := n.Create(ctx, dmCaller, d.ID, strahd())
	if err != nil || created.Name != "Strahd" {
		t.Fatalf("create = %+v %v", created, err)
	}
	edit := strahd()
	edit.Disposition, edit.DMNotes = "neutral", "Bargains."
	mcp := caller.Caller{Subject: dmCaller.Subject, Origin: caller.OriginMCP, Client: "claude"}
	if _, err := n.Update(ctx, mcp, d.ID, created.ID, edit); err != nil {
		t.Fatal(err)
	}
	changes, err := n.Diff(ctx, dmCaller, d.ID, created.ID, 1, 2)
	if err != nil || len(changes) != 2 || changes[0].Field != "dmNotes" || changes[1].Before != "hostile" || changes[1].After != "neutral" {
		t.Fatalf("diff = %+v %v", changes, err)
	}
	restored, err := n.Restore(ctx, dmCaller, d.ID, created.ID, 1)
	if err != nil || restored.Disposition != "hostile" {
		t.Fatalf("restore = %+v %v", restored, err)
	}
	revs, err := n.Revisions(ctx, dmCaller, d.ID, created.ID)
	if err != nil || len(revs) != 3 || revs[0].Action != domain.ActionRestore || revs[0].RestoredFrom != 1 ||
		revs[1].Origin != "mcp" || revs[1].Client != "claude" || revs[2].Author != "Joris" {
		t.Fatalf("revisions = %+v %v", revs, err)
	}
	if got, _ := n.Get(ctx, dmCaller, d.ID, created.ID); got.DMNotes != "Wants Ireena." {
		t.Fatalf("after restore = %+v", got)
	}
}

func TestDeletedNPCsComeBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	n, d := npcs(t, pgstore.New(open(t).Pool()))
	a, _ := n.Create(ctx, dmCaller, d.ID, strahd())
	b, _ := n.Create(ctx, dmCaller, d.ID, app.NPCInput{Name: "Ismark", Disposition: "friendly"})
	if err := n.Delete(ctx, dmCaller, d.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := n.List(ctx, dmCaller, d.ID); len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("list = %+v", list)
	}
	gone, err := n.Deleted(ctx, dmCaller, d.ID)
	if err != nil || len(gone) != 1 || gone[0].Name != "Strahd" {
		t.Fatalf("deleted = %+v %v", gone, err)
	}
	revs, _ := n.Revisions(ctx, dmCaller, d.ID, a.ID)
	if len(revs) != 2 || revs[0].Action != domain.ActionDelete {
		t.Fatalf("revisions = %+v", revs)
	}
	back, err := n.Restore(ctx, dmCaller, d.ID, a.ID, 2)
	if err != nil || back.ID != a.ID || back.Name != "Strahd" {
		t.Fatalf("restore deleted = %+v %v", back, err)
	}
	if gone, _ := n.Deleted(ctx, dmCaller, d.ID); len(gone) != 0 {
		t.Fatalf("still deleted = %+v", gone)
	}
}

func TestNPCsAreDMPrep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	n, d := npcs(t, pgstore.New(open(t).Pool()))
	a, _ := n.Create(ctx, dmCaller, d.ID, strahd())
	checks := map[string]func(c caller.Caller) error{
		"list":    func(c caller.Caller) error { _, err := n.List(ctx, c, d.ID); return err },
		"deleted": func(c caller.Caller) error { _, err := n.Deleted(ctx, c, d.ID); return err },
		"get":     func(c caller.Caller) error { _, err := n.Get(ctx, c, d.ID, a.ID); return err },
		"create":  func(c caller.Caller) error { _, err := n.Create(ctx, c, d.ID, strahd()); return err },
		"update":  func(c caller.Caller) error { _, err := n.Update(ctx, c, d.ID, a.ID, strahd()); return err },
		"delete":  func(c caller.Caller) error { return n.Delete(ctx, c, d.ID, a.ID) },
		"revs":    func(c caller.Caller) error { _, err := n.Revisions(ctx, c, d.ID, a.ID); return err },
		"diff":    func(c caller.Caller) error { _, err := n.Diff(ctx, c, d.ID, a.ID, 1, 1); return err },
		"restore": func(c caller.Caller) error { _, err := n.Restore(ctx, c, d.ID, a.ID, 1); return err },
	}
	for name, check := range checks {
		if err := check(playerCaller); !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("player %s: %v", name, err)
		}
		if err := check(stranger); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("stranger %s: %v", name, err)
		}
	}
}

func TestNPCInputAndMissingThings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	n, d := npcs(t, pgstore.New(open(t).Pool()))
	a, _ := n.Create(ctx, dmCaller, d.ID, strahd())
	long := strings.Repeat("x", 8001)
	for name, in := range map[string]app.NPCInput{
		"blank":       {Name: " ", Disposition: "neutral"},
		"title":       {Name: "A", Title: strings.Repeat("x", 81), Disposition: "neutral"},
		"notes":       {Name: "A", DMNotes: long, Disposition: "neutral"},
		"disposition": {Name: "A", Disposition: "smitten"},
	} {
		if _, err := n.Create(ctx, dmCaller, d.ID, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("create %s: %v", name, err)
		}
		if _, err := n.Update(ctx, dmCaller, d.ID, a.ID, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("update %s: %v", name, err)
		}
	}
	missing := domain.NPCID{}
	if _, err := n.Update(ctx, dmCaller, d.ID, missing, strahd()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("update missing: %v", err)
	}
	if err := n.Delete(ctx, dmCaller, d.ID, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("delete missing: %v", err)
	}
	if _, err := n.Revisions(ctx, dmCaller, d.ID, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("revisions missing: %v", err)
	}
	if _, err := n.Restore(ctx, dmCaller, d.ID, a.ID, 9); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("restore missing revision: %v", err)
	}
	if _, err := n.Diff(ctx, dmCaller, d.ID, a.ID, 9, 1); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("diff from missing: %v", err)
	}
	if _, err := n.Diff(ctx, dmCaller, d.ID, a.ID, 1, 9); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("diff to missing: %v", err)
	}
	if _, err := n.Get(ctx, dmCaller, d.ID, missing); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("get missing: %v", err)
	}
}

func TestEveryNPCDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	base, d := npcs(t, pgstore.New(db.Pool()))
	a, _ := base.Create(ctx, dmCaller, d.ID, strahd())
	gone, _ := base.Create(ctx, dmCaller, d.ID, strahd())
	_ = base.Delete(ctx, dmCaller, d.ID, gone.ID)
	doomed, _ := base.Create(ctx, dmCaller, d.ID, strahd())
	ops := map[string]func(n *app.NPCs) error{
		"create":   func(n *app.NPCs) error { _, err := n.Create(ctx, dmCaller, d.ID, strahd()); return err },
		"update":   func(n *app.NPCs) error { _, err := n.Update(ctx, dmCaller, d.ID, a.ID, strahd()); return err },
		"restore":  func(n *app.NPCs) error { _, err := n.Restore(ctx, dmCaller, d.ID, a.ID, 1); return err },
		"recreate": func(n *app.NPCs) error { _, err := n.Restore(ctx, dmCaller, d.ID, gone.ID, 1); return err },
		"delete":   func(n *app.NPCs) error { return n.Delete(ctx, dmCaller, d.ID, doomed.ID) },
		"list":     func(n *app.NPCs) error { _, err := n.List(ctx, dmCaller, d.ID); return err },
		"deleted":  func(n *app.NPCs) error { _, err := n.Deleted(ctx, dmCaller, d.ID); return err },
		"get":      func(n *app.NPCs) error { _, err := n.Get(ctx, dmCaller, d.ID, a.ID); return err },
		"revs":     func(n *app.NPCs) error { _, err := n.Revisions(ctx, dmCaller, d.ID, a.ID); return err },
		"diff":     func(n *app.NPCs) error { _, err := n.Diff(ctx, dmCaller, d.ID, a.ID, 1, 1); return err },
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(&app.NPCs{Repo: pgstore.NewFaulty(db.Pool(), f), Now: time.Now})
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
}
