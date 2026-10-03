package pgstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// carries puts an item in a Container of the Campaign: a Character's own Inventory, or the Party Stash
// when no Character is named. In a bag, it goes into a new bag inside that Container.
func carries(t *testing.T, db *pg.Store, campaign domain.CampaignID, character *domain.CharacterID, slug string, inBag bool) {
	t.Helper()
	ctx := context.Background()
	kind, owner := "party_stash", any(nil)
	if character != nil {
		kind, owner = "character", uuid.UUID(*character)
	}
	var container uuid.UUID
	err := db.Pool().QueryRow(ctx, `SELECT id FROM campaign.containers WHERE campaign_id = $1 AND kind = $2 AND character_id IS NOT DISTINCT FROM $3`, campaign, kind, owner).Scan(&container)
	if errors.Is(err, pgx.ErrNoRows) {
		container = uuid.New()
		_, err = db.Pool().Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, character_id, label, created_at) VALUES ($1, $2, $3, $4, 'Pack', now())`, container, campaign, kind, owner)
	}
	if err != nil {
		t.Fatal(err)
	}
	if inBag {
		bag := uuid.New()
		if _, err := db.Pool().Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, parent_id, label, created_at) VALUES ($1, $2, 'bag', $3, 'Satchel', now())`, bag, campaign, container); err != nil {
			t.Fatal(err)
		}
		container = bag
	}
	if _, err := db.Pool().Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at) VALUES ($1, $2, $3, 1, true, false, now())`, uuid.New(), container, slug); err != nil {
		t.Fatal(err)
	}
}

// The DM keeps Quests and Lore. A Player sees the Quests the party has been given and the Lore it has
// unlocked, and unlocks Lore for the whole party by reading a book or letter they carry. A hidden
// Quest and a locked Lore entry never reach a Player: not their words, not their names, not their number.
func TestTheJournalKeepsQuestsAndUnlocksLore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	// Each change comes a second after the last: Quests are listed oldest first.
	s := &app.Journal{Repo: pgstore.New(db.Pool()), Now: func() time.Time { now = now.Add(time.Second); return now }}
	tamsin, joris := hero(t, db, d.ID, playerCaller.Subject, "Tamsin"), hero(t, db, d.ID, dmCaller.Subject, "Old Joris")
	// Another Campaign of the same DM has the same letter in its Party Stash, and Lore in the same book.
	campaigns, _ := service(t, pgstore.New(db.Pool()))
	elsewhere, err := campaigns.Create(ctx, dmCaller, app.CreateInput{Name: "Elsewhere", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	carries(t, db, elsewhere.ID, nil, "sealed-letter", false)
	if _, err := s.CreateLore(ctx, dmCaller, elsewhere.ID, app.LoreInput{Title: "Another book", ItemSlug: "black-book"}); err != nil {
		t.Fatal(err)
	}

	seal, err := s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "  The stolen seal ", Summary: "The Watch has lost its seal.", Status: domain.QuestActive, Steps: []domain.QuestStep{
		{Text: " Ask at the Lantern Inn "}, {Text: "Find the fence", Done: true},
	}})
	if err != nil || seal.Name != "The stolen seal" || len(seal.Steps) != 2 || seal.Steps[0].Text != "Ask at the Lantern Inn" || !seal.Steps[1].Done {
		t.Fatalf("create quest = %+v %v", seal, err)
	}
	secret, _ := s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "The traitor in the Watch", Summary: "Captain Vane sold the seal.", Status: domain.QuestHidden})
	letter, err := s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: " The Captain's letter ", Body: "Vane owes the Grey Hands.", ItemSlug: " sealed-letter "})
	if err != nil || letter.Title != "The Captain's letter" || letter.ItemSlug != "sealed-letter" || letter.UnlockedAt != nil {
		t.Fatalf("create lore = %+v %v", letter, err)
	}
	for _, in := range []app.LoreInput{
		{Title: "The second page", Body: "They meet under the mill.", ItemSlug: "sealed-letter"},
		{Title: "The Ashen Hand", Body: "A cult older than the town.", ItemSlug: "black-book"},
		{Title: "Oakford", Body: "A market town on the river.", Unlocked: true},
	} {
		if _, err := s.CreateLore(ctx, dmCaller, d.ID, in); err != nil {
			t.Fatal(err)
		}
	}
	journal := func(who string) app.JournalView {
		t.Helper()
		c := dmCaller
		if who == "player" {
			c = playerCaller
		}
		v, err := s.Read(ctx, c, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	titles := func(v app.JournalView) []string {
		out := []string{}
		for _, l := range v.Lore {
			out = append(out, l.Title)
		}
		return out
	}
	quests := func(v app.JournalView) []string {
		out := []string{}
		for _, q := range v.Quests {
			out = append(out, q.Name)
		}
		return out
	}

	// The DM sees everything; a Player the Quest that was given and the Lore the DM unlocked outright.
	dmView, seen := journal("dm"), journal("player")
	if !dmView.DM || len(dmView.Quests) != 2 || len(dmView.Lore) != 4 || dmView.Lore[1].ItemSlug != "black-book" {
		t.Fatalf("the DM's Journal = %+v", dmView)
	}
	if seen.DM || !slices.Equal(quests(seen), []string{"The stolen seal"}) || !slices.Equal(titles(seen), []string{"Oakford"}) || len(seen.Readable) != 0 || seen.Lore[0].ItemSlug != "" {
		t.Fatalf("a Player's Journal to begin with = %+v", seen)
	}
	raw, _ := json.Marshal(seen)
	for _, hidden := range []string{"traitor", "Vane", "Captain", "Grey Hands", "mill", "Ashen", "cult", "sealed-letter", "black-book", secret.ID.String(), letter.ID.String()} {
		if strings.Contains(string(raw), hidden) {
			t.Fatalf("%q reached a Player: %s", hidden, raw)
		}
	}

	// Reading: only what the reader carries, on their own Character or in the Party Stash.
	var rule *apperr.RuleError
	for name, slug := range map[string]string{"a letter nobody has": "sealed-letter", "nothing at all": "", "an item that is not there": "no-such-thing"} {
		if _, err := s.ReadItem(ctx, playerCaller, d.ID, slug); !errors.As(err, &rule) {
			t.Fatalf("reading %s = %v", name, err)
		}
	}
	// Another Player's Character carries the letter: it is not this Player's to read, nor shown as readable.
	carries(t, db, d.ID, &joris, "sealed-letter", true)
	if _, err := s.ReadItem(ctx, playerCaller, d.ID, "sealed-letter"); !errors.As(err, &rule) {
		t.Fatalf("reading another's letter = %v", err)
	}
	if got := journal("player").Readable; len(got) != 0 {
		t.Fatalf("another's letter is readable: %v", got)
	}
	// A thing carried that holds no Lore reads as nothing.
	carries(t, db, d.ID, &tamsin, "rope", false)
	if _, err := s.ReadItem(ctx, playerCaller, d.ID, "rope"); !errors.As(err, &rule) {
		t.Fatalf("reading a rope = %v", err)
	}
	// Tamsin puts the letter in her satchel: the Journal says she can read it, and reading it unlocks
	// both entries it holds, for the party.
	carries(t, db, d.ID, &tamsin, "sealed-letter", true)
	if got := journal("player").Readable; !slices.Equal(got, []string{"sealed-letter"}) {
		t.Fatalf("readable once carried = %v", got)
	}
	if n, err := s.ReadItem(ctx, playerCaller, d.ID, "sealed-letter"); err != nil || n != 2 {
		t.Fatalf("reading the letter = %d %v", n, err)
	}
	seen = journal("player")
	if !slices.Equal(titles(seen), []string{"Oakford", "The Captain's letter", "The second page"}) || len(seen.Readable) != 0 || seen.Lore[1].Body != "Vane owes the Grey Hands." || seen.Lore[1].ItemSlug != "" {
		t.Fatalf("after reading = %+v", seen)
	}
	if _, err := s.ReadItem(ctx, playerCaller, d.ID, "sealed-letter"); !errors.As(err, &rule) {
		t.Fatalf("reading it twice = %v", err)
	}
	// A book in the Party Stash is anyone's to read.
	carries(t, db, d.ID, nil, "black-book", false)
	if got := journal("player").Readable; !slices.Equal(got, []string{"black-book"}) {
		t.Fatalf("readable from the Stash = %v", got)
	}
	if n, err := s.ReadItem(ctx, playerCaller, d.ID, "black-book"); err != nil || n != 1 {
		t.Fatalf("reading the book = %d %v", n, err)
	}

	// The DM gives the hidden Quest, ticks a step, and locks a Lore entry again.
	if _, err := s.UpdateQuest(ctx, dmCaller, d.ID, secret.ID, app.QuestInput{Name: "The traitor in the Watch", Status: domain.QuestActive, Steps: []domain.QuestStep{{Text: "Confront Vane"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateQuest(ctx, dmCaller, d.ID, seal.ID, app.QuestInput{Name: "The stolen seal", Status: domain.QuestCompleted, Steps: []domain.QuestStep{{Text: "Ask at the Lantern Inn", Done: true}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateLore(ctx, dmCaller, d.ID, letter.ID, app.LoreInput{Title: "The Captain's letter", Body: "Vane owes the Grey Hands.", ItemSlug: "sealed-letter", Unlocked: false}); err != nil {
		t.Fatal(err)
	}
	seen = journal("player")
	if !slices.Equal(quests(seen), []string{"The stolen seal", "The traitor in the Watch"}) || seen.Quests[0].Status != domain.QuestCompleted || len(seen.Quests[0].Steps) != 1 || !seen.Quests[0].Steps[0].Done ||
		!slices.Equal(titles(seen), []string{"Oakford", "The Ashen Hand", "The second page"}) {
		t.Fatalf("after the DM's changes = %+v", seen)
	}

	// Refusals.
	bad := map[string]error{}
	_, bad["a quest with no name"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: " ", Status: domain.QuestActive})
	_, bad["a quest with a long name"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: strings.Repeat("a", 121), Status: domain.QuestActive})
	_, bad["a quest with a long summary"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "A", Summary: strings.Repeat("a", 4001), Status: domain.QuestActive})
	_, bad["a quest of no status"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "A", Status: "paused"})
	_, bad["a quest with an empty step"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "A", Status: domain.QuestActive, Steps: []domain.QuestStep{{Text: " "}}})
	_, bad["a quest with a long step"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "A", Status: domain.QuestActive, Steps: []domain.QuestStep{{Text: strings.Repeat("a", 401)}}})
	_, bad["a quest with too many steps"] = s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "A", Status: domain.QuestActive, Steps: make([]domain.QuestStep, 51)})
	_, bad["lore with no title"] = s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: " "})
	_, bad["lore with a long title"] = s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: strings.Repeat("a", 121)})
	_, bad["lore with a long body"] = s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: "A", Body: strings.Repeat("a", 8001)})
	_, bad["lore of a long item"] = s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: "A", ItemSlug: strings.Repeat("a", 81)})
	for name, err := range bad {
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// The edges of what is allowed are allowed.
	fifty := make([]domain.QuestStep, 50)
	for i := range fifty {
		fifty[i].Text = strings.Repeat("a", 400)
	}
	if q, err := s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: strings.Repeat("a", 120), Summary: strings.Repeat("a", 4000), Status: domain.QuestFailed, Steps: fifty}); err != nil || len(q.Steps) != 50 {
		t.Errorf("the longest quest: %v", err)
	}
	if _, err := s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: strings.Repeat("a", 120), Body: strings.Repeat("a", 8000), ItemSlug: strings.Repeat("a", 80)}); err != nil {
		t.Errorf("the longest lore: %v", err)
	}
	forbidden := map[string]error{}
	_, forbidden["create a quest"] = s.CreateQuest(ctx, playerCaller, d.ID, app.QuestInput{Name: "Mine", Status: domain.QuestActive})
	_, forbidden["update a quest"] = s.UpdateQuest(ctx, playerCaller, d.ID, seal.ID, app.QuestInput{Name: "Mine", Status: domain.QuestActive})
	forbidden["delete a quest"] = s.DeleteQuest(ctx, playerCaller, d.ID, seal.ID)
	_, forbidden["create lore"] = s.CreateLore(ctx, playerCaller, d.ID, app.LoreInput{Title: "Mine", Unlocked: true})
	_, forbidden["update lore"] = s.UpdateLore(ctx, playerCaller, d.ID, letter.ID, app.LoreInput{Title: "Mine", Unlocked: true})
	forbidden["delete lore"] = s.DeleteLore(ctx, playerCaller, d.ID, letter.ID)
	for name, err := range forbidden {
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("a Player may %s: %v", name, err)
		}
	}
	missing := map[string]error{}
	_, missing["a stranger reads the Journal"] = s.Read(ctx, stranger, d.ID)
	_, missing["a stranger reads an item"] = s.ReadItem(ctx, stranger, d.ID, "sealed-letter")
	_, missing["update of no quest"] = s.UpdateQuest(ctx, dmCaller, d.ID, uuid.New(), app.QuestInput{Name: "A", Status: domain.QuestActive})
	missing["delete of no quest"] = s.DeleteQuest(ctx, dmCaller, d.ID, uuid.New())
	_, missing["update of no lore"] = s.UpdateLore(ctx, dmCaller, d.ID, uuid.New(), app.LoreInput{Title: "A"})
	missing["delete of no lore"] = s.DeleteLore(ctx, dmCaller, d.ID, uuid.New())
	// Through another Campaign of the same DM, this Campaign's Quest and Lore are not there to change;
	// and reading the book here unlocked nothing there.
	if there, err := s.Read(ctx, dmCaller, elsewhere.ID); err != nil || len(there.Lore) != 1 || there.Lore[0].UnlockedAt != nil || len(there.Quests) != 0 {
		t.Fatalf("the other Campaign's Journal = %+v %v", there, err)
	}
	_, missing["a quest through another Campaign"] = s.UpdateQuest(ctx, dmCaller, elsewhere.ID, seal.ID, app.QuestInput{Name: "Stolen", Status: domain.QuestFailed})
	missing["a quest deleted through another Campaign"] = s.DeleteQuest(ctx, dmCaller, elsewhere.ID, seal.ID)
	_, missing["lore through another Campaign"] = s.UpdateLore(ctx, dmCaller, elsewhere.ID, letter.ID, app.LoreInput{Title: "Stolen", Unlocked: true})
	missing["lore deleted through another Campaign"] = s.DeleteLore(ctx, dmCaller, elsewhere.ID, letter.ID)
	for name, err := range missing {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if v := journal("dm"); v.Quests[0].Name != "The stolen seal" || !slices.Contains(titles(v), "The Captain's letter") {
		t.Fatalf("after the strays = %+v", v)
	}
	if err := s.DeleteQuest(ctx, dmCaller, d.ID, seal.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLore(ctx, dmCaller, d.ID, letter.ID); err != nil {
		t.Fatal(err)
	}
	if v := journal("dm"); slices.Contains(quests(v), "The stolen seal") || slices.Contains(titles(v), "The Captain's letter") {
		t.Fatalf("after deleting = %+v", v)
	}
}

// Every Journal operation reports a database fault at any of its calls, and a Quest is saved with its
// steps or not at all.
func TestEveryJournalDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	svc := func(repo app.JournalRepository) *app.Journal { return &app.Journal{Repo: repo, Now: time.Now} }
	base := svc(pgstore.New(db.Pool()))
	tamsin := hero(t, db, d.ID, playerCaller.Subject, "Tamsin")
	carries(t, db, d.ID, &tamsin, "sealed-letter", false)
	steps := []domain.QuestStep{{Text: "One"}, {Text: "Two"}}
	q, err := base.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "Seal", Status: domain.QuestActive, Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	l, err := base.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: "Letter", ItemSlug: "sealed-letter"})
	if err != nil {
		t.Fatal(err)
	}
	ops := map[string]func(s *app.Journal) error{
		"read": func(s *app.Journal) error { _, err := s.Read(ctx, playerCaller, d.ID); return err },
		"create quest": func(s *app.Journal) error {
			_, err := s.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "Other", Status: domain.QuestActive, Steps: steps})
			return err
		},
		"update quest": func(s *app.Journal) error {
			_, err := s.UpdateQuest(ctx, dmCaller, d.ID, q.ID, app.QuestInput{Name: "Seal", Status: domain.QuestActive, Steps: steps})
			return err
		},
		"delete quest": func(s *app.Journal) error {
			doomed, err := base.CreateQuest(ctx, dmCaller, d.ID, app.QuestInput{Name: "Doomed", Status: domain.QuestHidden})
			if err != nil {
				t.Fatal(err)
			}
			return s.DeleteQuest(ctx, dmCaller, d.ID, doomed.ID)
		},
		"create lore": func(s *app.Journal) error {
			_, err := s.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: "Other"})
			return err
		},
		"update lore": func(s *app.Journal) error {
			_, err := s.UpdateLore(ctx, dmCaller, d.ID, l.ID, app.LoreInput{Title: "Letter", ItemSlug: "sealed-letter"})
			return err
		},
		"delete lore": func(s *app.Journal) error {
			doomed, err := base.CreateLore(ctx, dmCaller, d.ID, app.LoreInput{Title: "Doomed"})
			if err != nil {
				t.Fatal(err)
			}
			return s.DeleteLore(ctx, dmCaller, d.ID, doomed.ID)
		},
		"read an item": func(s *app.Journal) error {
			if _, err := base.UpdateLore(ctx, dmCaller, d.ID, l.ID, app.LoreInput{Title: "Letter", ItemSlug: "sealed-letter"}); err != nil {
				t.Fatal(err)
			}
			_, err := s.ReadItem(ctx, playerCaller, d.ID, "sealed-letter")
			return err
		},
	}
	for name, op := range ops {
		pgtest.EveryFault(t, func(fault *pgtest.Faulty) error {
			err := op(svc(pgstore.NewFaulty(db.Pool(), fault)))
			if err != nil && !errors.Is(err, pgtest.ErrInjected) {
				t.Fatalf("%s: %v", name, err)
			}
			return err
		})
	}
	// No Quest was left with half its steps.
	v, err := base.Read(ctx, dmCaller, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, quest := range v.Quests {
		if quest.Name != "Doomed" && len(quest.Steps) != 2 {
			t.Fatalf("a Quest left with %d of its 2 steps: %+v", len(quest.Steps), quest)
		}
	}
}
