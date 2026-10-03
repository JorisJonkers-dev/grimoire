package pgstore_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// told keeps what the Session under way was told of thresholds crossed.
type told struct {
	crossed [][]app.TrackCrossing
	by      []domain.Member
}

func (e *told) TrackCrossed(_ domain.CampaignID, by domain.Member, _ caller.Caller, crossed []app.TrackCrossing) {
	e.crossed, e.by = append(e.crossed, crossed), append(e.by, by)
}

// The DM keeps Tracks and moves their scores, for a Character or for the party. A score stays within
// its Track, and the thresholds it crosses are named and told to the Session under way. A Player sees
// the party's scores and their own Characters', and no threshold.
func TestTracksAndTheirThresholds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	events := &told{}
	s := &app.Tracks{Repo: pgstore.New(db.Pool()), Events: events, Now: func() time.Time { now = now.Add(time.Second); return now }}
	tamsin, joris := hero(t, db, d.ID, playerCaller.Subject, "Tamsin"), hero(t, db, d.ID, dmCaller.Subject, "Old Joris")
	madness, unlinked := rollTable(t, db, &d.ID, "Madness"), rollTable(t, db, nil, "Not linked")

	stress, err := s.Create(ctx, dmCaller, d.ID, app.TrackInput{Name: "  Stress ", Scope: domain.TrackPerCharacter, Min: 0, Max: 10, Start: 1, Thresholds: []domain.TrackThreshold{
		{At: 8, Rising: true, Label: " Breaking point ", RollTable: &madness},
		{At: 4, Rising: true, Label: "Shaken", Effect: " frightened "},
		{At: 2, Rising: false, Label: "Calm again"},
	}})
	if err != nil || stress.Name != "Stress" || len(stress.Thresholds) != 3 || stress.Thresholds[0].Label != "Breaking point" || stress.Thresholds[1].Effect != "frightened" {
		t.Fatalf("create = %+v %v", stress, err)
	}
	renown, err := s.Create(ctx, dmCaller, d.ID, app.TrackInput{Name: "Renown", Scope: domain.TrackParty, Min: -5, Max: 5, Start: 0})
	if err != nil {
		t.Fatal(err)
	}

	// The DM sees every Character's score, at the Track's start until it moves, and the thresholds.
	all, err := s.List(ctx, dmCaller, d.ID)
	if err != nil || !all.DM || len(all.Tracks) != 2 || all.Tracks[0].Track.ID != stress.ID || len(all.Tracks[0].Track.Thresholds) != 3 || all.Tracks[0].Track.Thresholds[0].RollTable == nil {
		t.Fatalf("the DM's list = %+v %v", all, err)
	}
	if st := all.Tracks[0].Standings; len(st) != 2 || st[0].Name != "Old Joris" || st[0].Value != 1 || st[1].Name != "Tamsin" || *st[1].Character != tamsin {
		t.Fatalf("the DM's standings = %+v", st)
	}
	if st := all.Tracks[1].Standings; len(st) != 1 || st[0].Character != nil || st[0].Value != 0 {
		t.Fatalf("the party's standing = %+v", st)
	}

	// Up past the first threshold only: one crossing, told with who moved it.
	got, err := s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, 4)
	if err != nil || got.Value != 5 || len(got.Crossed) != 1 || got.Crossed[0].Label != "Shaken" {
		t.Fatalf("up 4 = %+v %v", got, err)
	}
	if len(events.crossed) != 1 || len(events.crossed[0]) != 1 || events.crossed[0][0].Track != "Stress" || *events.crossed[0][0].Character != tamsin || events.crossed[0][0].Threshold.Effect != "frightened" || events.by[0].Role != domain.RoleDM {
		t.Fatalf("told = %+v by %+v", events.crossed, events.by)
	}
	// Up far past the top: it stops at the top, and crosses the one threshold left on the way.
	if got, err := s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, 50); err != nil || got.Value != 10 || len(got.Crossed) != 1 || got.Crossed[0].Label != "Breaking point" || *got.Crossed[0].RollTable != madness {
		t.Fatalf("up 50 = %+v %v", got, err)
	}
	// A move that crosses nothing tells nobody.
	if got, err := s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, -3); err != nil || got.Value != 7 || len(got.Crossed) != 0 || len(events.crossed) != 2 {
		t.Fatalf("down 3 = %+v %v, told %d times", got, err, len(events.crossed))
	}
	// Down past the falling threshold, to the bottom.
	if got, err := s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, -50); err != nil || got.Value != 0 || len(got.Crossed) != 1 || got.Crossed[0].Label != "Calm again" || len(events.crossed) != 3 {
		t.Fatalf("down 50 = %+v %v", got, err)
	}
	// The party's score moves on its own, and Joris's has not moved at all.
	if got, err := s.Adjust(ctx, dmCaller, d.ID, renown.ID, nil, -9); err != nil || got.Value != -5 {
		t.Fatalf("the party's renown = %+v %v", got, err)
	}
	if got, err := s.Adjust(ctx, dmCaller, d.ID, renown.ID, nil, 2); err != nil || got.Value != -3 {
		t.Fatalf("the party's renown again = %+v %v", got, err)
	}
	all, _ = s.List(ctx, dmCaller, d.ID)
	if st := all.Tracks[0].Standings; st[0].Value != 1 || st[1].Value != 0 || all.Tracks[1].Standings[0].Value != -3 {
		t.Fatalf("after the moves = %+v %+v", st, all.Tracks[1].Standings)
	}

	// A Player sees the party's score and their own Character's, and no threshold.
	seen, err := s.List(ctx, playerCaller, d.ID)
	if err != nil || seen.DM || len(seen.Tracks) != 2 || seen.Tracks[0].Track.Thresholds != nil || seen.Tracks[1].Standings[0].Value != -3 {
		t.Fatalf("a Player's list = %+v %v", seen, err)
	}
	if st := seen.Tracks[0].Standings; len(st) != 1 || st[0].Name != "Tamsin" || st[0].Value != 0 {
		t.Fatalf("a Player's standings = %+v", st)
	}

	// Refusals.
	base := app.TrackInput{Name: "A", Scope: domain.TrackParty, Min: 0, Max: 10, Start: 0}
	with := func(f func(in *app.TrackInput)) app.TrackInput {
		in := base
		f(&in)
		return in
	}
	one := func(th domain.TrackThreshold) app.TrackInput {
		return with(func(in *app.TrackInput) { in.Thresholds = []domain.TrackThreshold{th} })
	}
	for name, in := range map[string]app.TrackInput{
		"no name":                  with(func(in *app.TrackInput) { in.Name = " " }),
		"a long name":              with(func(in *app.TrackInput) { in.Name = strings.Repeat("a", 81) }),
		"no such scope":            with(func(in *app.TrackInput) { in.Scope = "faction" }),
		"bounds the wrong way":     with(func(in *app.TrackInput) { in.Min, in.Max = 10, 0 }),
		"bounds that meet":         with(func(in *app.TrackInput) { in.Min, in.Max, in.Start = 3, 3, 3 }),
		"a bottom too low":         with(func(in *app.TrackInput) { in.Min, in.Start = -1001, 0 }),
		"a top too high":           with(func(in *app.TrackInput) { in.Max = 1001 }),
		"a start below the bottom": with(func(in *app.TrackInput) { in.Start = -1 }),
		"a start above the top":    with(func(in *app.TrackInput) { in.Start = 11 }),
		"too many thresholds": with(func(in *app.TrackInput) {
			in.Thresholds = slices.Repeat([]domain.TrackThreshold{{At: 5, Rising: true, Label: "x"}}, 21)
		}),
		"a threshold with no label":   one(domain.TrackThreshold{At: 5, Rising: true, Label: " "}),
		"a threshold's long label":    one(domain.TrackThreshold{At: 5, Rising: true, Label: strings.Repeat("a", 81)}),
		"a threshold below the Track": one(domain.TrackThreshold{At: -1, Rising: true, Label: "x"}),
		"a threshold above the Track": one(domain.TrackThreshold{At: 11, Rising: true, Label: "x"}),
		"a threshold's long Effect":   one(domain.TrackThreshold{At: 5, Rising: true, Label: "x", Effect: strings.Repeat("a", 81)}),
		"a threshold of two outcomes": one(domain.TrackThreshold{At: 5, Rising: true, Label: "x", Effect: "prone", RollTable: &madness}),
	} {
		if _, err := s.Create(ctx, dmCaller, d.ID, in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	edges := app.TrackInput{Name: strings.Repeat("a", 80), Scope: domain.TrackParty, Min: -1000, Max: 1000, Start: 1000, Thresholds: slices.Repeat([]domain.TrackThreshold{
		{At: 1000, Rising: true, Label: strings.Repeat("b", 80), Effect: strings.Repeat("c", 80)}, {At: -1000, Rising: false, Label: "x"},
	}, 10)}
	wide, err := s.Create(ctx, dmCaller, d.ID, edges)
	if err != nil {
		t.Fatalf("the widest Track: %v", err)
	}
	var rule *apperr.RuleError
	stray := uuid.New()
	for name, table := range map[string]*uuid.UUID{"a table the Campaign does not see": &unlinked, "a table that does not exist": &stray} {
		if _, err := s.Create(ctx, dmCaller, d.ID, one(domain.TrackThreshold{At: 5, Rising: true, Label: "x", RollTable: table})); !errors.As(err, &rule) || !strings.Contains(rule.Reason, "not one this Campaign sees") {
			t.Errorf("%s: %v", name, err)
		}
	}
	invalid := map[string]error{}
	_, invalid["a move of nothing"] = s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, 0)
	_, invalid["a move too far up"] = s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, 2001)
	_, invalid["a move too far down"] = s.Adjust(ctx, dmCaller, d.ID, stress.ID, &tamsin, -2001)
	_, invalid["a Character's Track moved for the party"] = s.Adjust(ctx, dmCaller, d.ID, stress.ID, nil, 1)
	_, invalid["the party's Track moved for a Character"] = s.Adjust(ctx, dmCaller, d.ID, renown.ID, &tamsin, 1)
	for name, err := range invalid {
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got, err := s.Adjust(ctx, dmCaller, d.ID, wide.ID, nil, -2000); err != nil || got.Value != -1000 || len(got.Crossed) != 10 {
		t.Errorf("the widest move: %+v %v", got, err)
	}
	forbidden := map[string]error{}
	_, forbidden["create"] = s.Create(ctx, playerCaller, d.ID, base)
	forbidden["delete"] = s.Delete(ctx, playerCaller, d.ID, stress.ID)
	_, forbidden["adjust"] = s.Adjust(ctx, playerCaller, d.ID, stress.ID, &tamsin, -1)
	for name, err := range forbidden {
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("a Player may %s: %v", name, err)
		}
	}
	campaigns, _ := service(t, pgstore.New(db.Pool()))
	elsewhere, err := campaigns.Create(ctx, dmCaller, app.CreateInput{Name: "Elsewhere", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	outsider := hero(t, db, elsewhere.ID, dmCaller.Subject, "Outsider")
	missing := map[string]error{}
	_, missing["a stranger lists"] = s.List(ctx, stranger, d.ID)
	_, missing["a stranger creates"] = s.Create(ctx, stranger, d.ID, base)
	_, missing["a stranger adjusts"] = s.Adjust(ctx, stranger, d.ID, stress.ID, &tamsin, 1)
	missing["a stranger deletes"] = s.Delete(ctx, stranger, d.ID, stress.ID)
	_, missing["no such Track"] = s.Adjust(ctx, dmCaller, d.ID, uuid.New(), &tamsin, 1)
	_, missing["a Character of another Campaign"] = s.Adjust(ctx, dmCaller, d.ID, stress.ID, &outsider, 1)
	_, missing["a Track through another Campaign"] = s.Adjust(ctx, dmCaller, elsewhere.ID, stress.ID, &outsider, 1)
	missing["a Track deleted through another Campaign"] = s.Delete(ctx, dmCaller, elsewhere.ID, stress.ID)
	missing["the deletion of none"] = s.Delete(ctx, dmCaller, d.ID, uuid.New())
	for name, err := range missing {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if there, err := s.List(ctx, dmCaller, elsewhere.ID); err != nil || len(there.Tracks) != 0 {
		t.Fatalf("the other Campaign = %+v %v", there, err)
	}
	// Each Track keeps its own score: the party's Renown is not the widest Track's.
	if after, _ := s.List(ctx, dmCaller, d.ID); len(after.Tracks) != 3 || after.Tracks[0].Standings[1].Value != 0 || after.Tracks[0].Standings[0].Value != 1 ||
		after.Tracks[1].Standings[0].Value != -3 || after.Tracks[2].Standings[0].Value != -1000 {
		t.Fatalf("after the refusals = %+v", after.Tracks)
	}
	_ = joris

	// A Campaign keeps twenty Tracks and no more; a deleted Track takes its scores with it.
	if err := s.Delete(ctx, dmCaller, d.ID, wide.ID); err != nil {
		t.Fatal(err)
	}
	for i := 2; i < app.MaxTracks; i++ {
		if _, err := s.Create(ctx, dmCaller, d.ID, base); err != nil {
			t.Fatalf("Track %d: %v", i, err)
		}
	}
	if _, err := s.Create(ctx, dmCaller, d.ID, base); !errors.As(err, &rule) || !strings.Contains(rule.Reason, "up to 20") {
		t.Errorf("the twenty-first: %v", err)
	}
	if err := s.Delete(ctx, dmCaller, d.ID, stress.ID); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := db.Pool().QueryRow(ctx, "SELECT count(*) FROM campaign.track_values WHERE track_id = $1", stress.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("scores left behind: %d %v", left, err)
	}

	// Every operation reports a database fault at any of its calls, and a Track is kept with its thresholds or not at all.
	quiet := &told{}
	svc := func(f *pgtest.Faulty) *app.Tracks {
		return &app.Tracks{Repo: pgstore.NewFaulty(db.Pool(), f), Events: quiet, Now: time.Now}
	}
	if err := s.Delete(ctx, dmCaller, d.ID, renown.ID); err != nil {
		t.Fatal(err)
	}
	kept, err := s.Create(ctx, dmCaller, d.ID, app.TrackInput{Name: "Honour", Scope: domain.TrackPerCharacter, Min: 0, Max: 10, Start: 5, Thresholds: []domain.TrackThreshold{{At: 9, Rising: true, Label: "Exalted", RollTable: &madness}}})
	if err != nil {
		t.Fatal(err)
	}
	for name, op := range map[string]func(f *pgtest.Faulty) error{
		"list": func(f *pgtest.Faulty) error { _, err := svc(f).List(ctx, playerCaller, d.ID); return err },
		"adjust": func(f *pgtest.Faulty) error {
			_, err := svc(f).Adjust(ctx, dmCaller, d.ID, kept.ID, &tamsin, 1)
			return err
		},
		"create": func(f *pgtest.Faulty) error {
			_, err := svc(f).Create(ctx, dmCaller, elsewhere.ID, app.TrackInput{Name: "Piety", Scope: domain.TrackParty, Min: 0, Max: 10, Start: 0, Thresholds: []domain.TrackThreshold{{At: 5, Rising: true, Label: "x"}, {At: 6, Rising: true, Label: "y"}}})
			return err
		},
		"delete": func(f *pgtest.Faulty) error {
			err := svc(f).Delete(ctx, dmCaller, d.ID, uuid.New())
			if errors.Is(err, domain.ErrNotFound) {
				return nil
			}
			return err
		},
	} {
		pgtest.EveryFault(t, func(f *pgtest.Faulty) error {
			err := op(f)
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	there, err := s.List(ctx, dmCaller, elsewhere.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range there.Tracks {
		if len(tr.Track.Thresholds) != 2 {
			t.Fatalf("a Track kept with %d of its 2 thresholds", len(tr.Track.Thresholds))
		}
	}
}
