package pgstore_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/standing"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// hero puts a Character of a Member in a Campaign.
func hero(t *testing.T, db *pg.Store, campaign domain.CampaignID, subject, name string) domain.CharacterID {
	t.Helper()
	id := uuid.New()
	if _, err := db.Pool().Exec(context.Background(), `WITH owner AS (SELECT id, auth_subject FROM campaign.members WHERE campaign_id = $2 AND auth_subject = $4),
		sheet AS (INSERT INTO campaign.account_characters (id, owner_subject, name, ruleset, species_slug, class_slug, background_slug, created_at, updated_at)
			SELECT $1, auth_subject, $3, 'srd-2024', 'human', 'fighter', 'soldier', now(), now() FROM owner)
		INSERT INTO campaign.characters (character_id, id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug, ability_method, hp_max, hp_current)
		SELECT $1, $1, $2, id, $3, 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 10, 10 FROM owner`, id, campaign, name, subject); err != nil {
		t.Fatal(err)
	}
	return domain.CharacterID(id)
}

func factionNamed(t *testing.T, list []app.FactionView, name string) app.FactionView {
	t.Helper()
	for _, f := range list {
		if f.Faction.Name == name {
			return f
		}
	}
	t.Fatalf("no Faction %s in %+v", name, list)
	return app.FactionView{}
}

// A DM keeps Factions and decides every Standing Change. A change is only ever suggested, by a person
// or by an agent, and moves nothing until the DM confirms it in person. Players see tiers and the
// reasons the DM shared: never a number, an unshared reason, a pending change or the DM's notes.
func TestFactionsStandingAndTheChangesTheDMDecides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	s := &app.Factions{Repo: pgstore.New(db.Pool()), Now: func() time.Time { return time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC) }}
	tamsin, joris := hero(t, db, d.ID, playerCaller.Subject, "Tamsin"), hero(t, db, d.ID, dmCaller.Subject, "Old Joris")
	agent := caller.MCP(dmCaller.Subject, "Claude")

	watch, err := s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "  The Lantern Watch ", Archetype: "city-watch", Goals: "Keep the peace.", Territory: "Oakford", Notes: "The captain takes bribes."})
	if err != nil || watch.Faction.Name != "The Lantern Watch" || watch.Tier != standing.Neutral || watch.Faction.Score != 0 {
		t.Fatalf("create = %+v %v", watch, err)
	}
	guild, _ := s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "Grey Hands"})
	lists := func(c caller.Caller) []app.FactionView {
		t.Helper()
		list, err := s.List(ctx, c, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		return list
	}
	suggest := func(c caller.Caller, faction domain.FactionID, g app.Suggestion) domain.StandingChange {
		t.Helper()
		ch, err := s.Propose(ctx, c, d.ID, faction, g)
		if err != nil {
			t.Fatal(err)
		}
		return ch
	}

	// A suggestion waits: from the DM's own hand and from an agent's alike, and the Standing stays put.
	byHand := suggest(dmCaller, watch.Faction.ID, app.Suggestion{Delta: 30, Reason: " Returned the stolen seal. ", ShareReason: true})
	byAgent := suggest(agent, watch.Faction.ID, app.Suggestion{Delta: -70, Reason: "Killed a watchman.", ShareReason: false})
	own := suggest(dmCaller, watch.Faction.ID, app.Suggestion{Character: &tamsin, Delta: 45, Reason: "Tamsin saved the captain's son."})
	other := suggest(dmCaller, watch.Faction.ID, app.Suggestion{Character: &joris, Delta: -30, Reason: "Old Joris insulted the captain.", ShareReason: true})
	dmWatch := factionNamed(t, lists(dmCaller), "The Lantern Watch")
	if byHand.Status != domain.ChangePending || byAgent.Status != domain.ChangePending || byHand.Reason != "Returned the stolen seal." || dmWatch.Tier != standing.Neutral || dmWatch.Faction.Score != 0 || len(dmWatch.Changes) != 4 {
		t.Fatalf("suggested = %+v %+v, the Watch %+v", byHand, byAgent, dmWatch)
	}
	for _, ch := range dmWatch.Changes {
		if ch.Status != domain.ChangePending || ch.DM == nil {
			t.Fatalf("a suggestion on the DM's list = %+v", ch)
		}
		if ch.ID == byAgent.ID && (ch.DM.Origin != "mcp" || ch.DM.Client != "Claude" || ch.DM.Delta != -70) {
			t.Fatalf("the agent's suggestion = %+v", ch.DM)
		}
	}
	// A Player sees no suggestion at all.
	if seen := factionNamed(t, lists(playerCaller), "The Lantern Watch"); len(seen.Changes) != 0 || len(seen.Personal) != 0 || seen.Tier != standing.Neutral {
		t.Fatalf("a Player before anything is decided = %+v", seen)
	}

	// Only the DM decides, and only in person: an agent acting for the DM is refused.
	confirm := app.Decision{Confirm: true}
	if err := s.Decide(ctx, agent, d.ID, byHand.ID, confirm); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("an agent confirms = %v", err)
	}
	if err := s.Decide(ctx, playerCaller, d.ID, byHand.ID, confirm); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("a Player confirms = %v", err)
	}
	if err := s.Decide(ctx, stranger, d.ID, byHand.ID, confirm); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a stranger confirms = %v", err)
	}
	if got := factionNamed(t, lists(dmCaller), "The Lantern Watch"); got.Faction.Score != 0 {
		t.Fatalf("a refused decision moved the Standing: %+v", got.Faction)
	}

	// Confirmed as suggested, the party's Standing moves; edited, it moves by what the DM says, with
	// the DM's reason, shared or not as the DM chooses.
	if err := s.Decide(ctx, dmCaller, d.ID, byHand.ID, confirm); err != nil {
		t.Fatal(err)
	}
	if got := factionNamed(t, lists(dmCaller), "The Lantern Watch"); got.Faction.Score != 30 || got.Tier != standing.Friendly {
		t.Fatalf("after the first change = %+v", got.Faction)
	}
	delta, reason, share := -50, " A watchman died in the brawl. ", false
	if err := s.Decide(ctx, dmCaller, d.ID, byAgent.ID, app.Decision{Confirm: true, Delta: &delta, Reason: &reason, ShareReason: &share}); err != nil {
		t.Fatal(err)
	}
	// A Personal Standing starts from the party's and then goes its own way.
	if err := s.Decide(ctx, dmCaller, d.ID, own.ID, confirm); err != nil {
		t.Fatal(err)
	}
	if err := s.Decide(ctx, dmCaller, d.ID, other.ID, app.Decision{Confirm: false}); err != nil {
		t.Fatal(err)
	}
	dmWatch = factionNamed(t, lists(dmCaller), "The Lantern Watch")
	if dmWatch.Faction.Score != -20 || dmWatch.Tier != standing.Unfriendly || len(dmWatch.Personal) != 1 || dmWatch.Personal[0].CharacterID != tamsin || *dmWatch.Personal[0].Score != 25 || dmWatch.Personal[0].Tier != standing.Friendly {
		t.Fatalf("the DM's Watch = %+v %+v", dmWatch.Faction, dmWatch.Personal)
	}
	for _, ch := range dmWatch.Changes {
		switch ch.ID {
		case byAgent.ID:
			if ch.Status != domain.ChangeConfirmed || ch.DM.Delta != -50 || ch.Reason != "A watchman died in the brawl." || ch.DM.ShareReason || ch.Rose {
				t.Fatalf("the edited change = %+v %+v", ch, ch.DM)
			}
		case other.ID:
			if ch.Status != domain.ChangeDismissed || ch.DM.Delta != -30 {
				t.Fatalf("the dismissed change = %+v %+v", ch, ch.DM)
			}
		}
	}
	// Once a Character has a Personal Standing, the next change moves that, not the party's again.
	again := suggest(dmCaller, watch.Faction.ID, app.Suggestion{Character: &tamsin, Delta: 10, Reason: "Tamsin stood a round."})
	if err := s.Decide(ctx, dmCaller, d.ID, again.ID, confirm); err != nil {
		t.Fatal(err)
	}
	if got := factionNamed(t, lists(dmCaller), "The Lantern Watch"); *got.Personal[0].Score != 35 || got.Faction.Score != -20 {
		t.Fatalf("a second Personal change = %+v %+v", got.Personal, got.Faction)
	}
	// A change is decided through its own Campaign only: the same DM, asking through another of their
	// Campaigns, finds nothing there to decide.
	campaigns, _ := service(t, pgstore.New(db.Pool()))
	elsewhere, err := campaigns.Create(ctx, dmCaller, app.CreateInput{Name: "Elsewhere", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	stray := suggest(dmCaller, watch.Faction.ID, app.Suggestion{Delta: 60, Reason: "A stray change."})
	if err := s.Decide(ctx, dmCaller, elsewhere.ID, stray.ID, confirm); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a change decided through another Campaign = %v", err)
	}
	if _, err := s.Propose(ctx, dmCaller, elsewhere.ID, watch.Faction.ID, app.Suggestion{Delta: 60, Reason: "A stray change."}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a change proposed through another Campaign = %v", err)
	}
	if got := factionNamed(t, lists(dmCaller), "The Lantern Watch"); got.Faction.Score != -20 {
		t.Fatalf("the Standing after a stray decision = %+v", got.Faction)
	}
	if err := s.Decide(ctx, dmCaller, d.ID, stray.ID, app.Decision{Confirm: false}); err != nil {
		t.Fatal(err)
	}
	// Decided once, a change is not decided again.
	for _, id := range []domain.ChangeID{byHand.ID, other.ID} {
		if err := s.Decide(ctx, dmCaller, d.ID, id, confirm); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("decided twice = %v", err)
		}
	}
	if err := s.Decide(ctx, dmCaller, d.ID, uuid.New(), confirm); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deciding nothing = %v", err)
	}

	// What a Player sees: the tier, their own Character's Personal Standing as a tier, the changes that
	// were confirmed for the party and for their own Character, and a reason only where it was shared.
	seen := factionNamed(t, lists(playerCaller), "The Lantern Watch")
	if seen.Tier != standing.Unfriendly || seen.DM || seen.Faction.Score != 0 || seen.Faction.Notes != "" || seen.Faction.Goals != "" || seen.Faction.Territory != "" {
		t.Fatalf("a Player's Watch = %+v", seen)
	}
	if len(seen.Personal) != 1 || seen.Personal[0].CharacterID != tamsin || seen.Personal[0].Tier != standing.Friendly || seen.Personal[0].Score != nil {
		t.Fatalf("a Player's Personal Standing = %+v", seen.Personal)
	}
	if len(seen.Changes) != 4 {
		t.Fatalf("a Player's changes = %+v", seen.Changes)
	}
	for _, ch := range seen.Changes {
		want := map[domain.ChangeID]struct {
			reason string
			rose   bool
		}{byHand.ID: {"Returned the stolen seal.", true}, byAgent.ID: {"", false}, own.ID: {"", true}, again.ID: {"", true}}[ch.ID]
		if ch.DM != nil || ch.Status != domain.ChangeConfirmed || ch.Reason != want.reason || ch.Rose != want.rose {
			t.Fatalf("a Player's change = %+v", ch)
		}
	}
	raw, _ := json.Marshal(lists(playerCaller))
	for _, secret := range []string{"watchman", "bribes", "Old Joris", "insulted", "Keep the peace", "Oakford", "captain's son", "stood a round", "Claude", "mcp", ":-50", ":-20", `"Score":25`, `"Delta"`} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("%q reached a Player: %s", secret, raw)
		}
	}
	// The DM, whose own Character it is not, sees every Personal Standing; a Player sees only their own.
	if err := s.Decide(ctx, dmCaller, d.ID, suggest(dmCaller, guild.Faction.ID, app.Suggestion{Character: &joris, Delta: 80, Reason: "An old debt.", ShareReason: true}).ID, confirm); err != nil {
		t.Fatal(err)
	}
	if got := factionNamed(t, lists(playerCaller), "Grey Hands"); len(got.Personal) != 0 || len(got.Changes) != 0 {
		t.Fatalf("another Player's Personal Standing reached a Player: %+v", got)
	}
	if got := factionNamed(t, lists(dmCaller), "Grey Hands"); len(got.Personal) != 1 || got.Personal[0].Tier != standing.Allied || got.Faction.Score != 0 {
		t.Fatalf("the DM's Grey Hands = %+v", got)
	}

	// Refusals.
	var rule *apperr.RuleError
	bad := map[string]error{}
	_, bad["no name"] = s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: " "})
	_, bad["a long name"] = s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: strings.Repeat("a", 81)})
	_, bad["long goals"] = s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "A", Goals: strings.Repeat("a", 2001)})
	_, bad["long territory"] = s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "A", Territory: strings.Repeat("a", 2001)})
	_, bad["long notes"] = s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "A", Notes: strings.Repeat("a", 2001)})
	_, bad["no change"] = s.Propose(ctx, dmCaller, d.ID, watch.Faction.ID, app.Suggestion{Delta: 0, Reason: "Nothing."})
	_, bad["too much"] = s.Propose(ctx, dmCaller, d.ID, watch.Faction.ID, app.Suggestion{Delta: 101, Reason: "Everything."})
	_, bad["no reason"] = s.Propose(ctx, dmCaller, d.ID, watch.Faction.ID, app.Suggestion{Delta: 5, Reason: "  "})
	_, bad["a long reason"] = s.Propose(ctx, dmCaller, d.ID, watch.Faction.ID, app.Suggestion{Delta: 5, Reason: strings.Repeat("a", 501)})
	pending := suggest(dmCaller, watch.Faction.ID, app.Suggestion{Delta: 5, Reason: "A favour."})
	zero, blank := 0, " "
	bad["confirmed at nothing"] = s.Decide(ctx, dmCaller, d.ID, pending.ID, app.Decision{Confirm: true, Delta: &zero})
	bad["confirmed for no reason"] = s.Decide(ctx, dmCaller, d.ID, pending.ID, app.Decision{Confirm: true, Reason: &blank})
	for name, err := range bad {
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	nobody := domain.CharacterID(uuid.New())
	if _, err := s.Propose(ctx, dmCaller, d.ID, watch.Faction.ID, app.Suggestion{Character: &nobody, Delta: 5, Reason: "Who?"}); !errors.As(err, &rule) {
		t.Errorf("a Personal Standing for nobody: %v", err)
	}
	if _, err := s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "Harpers", Archetype: "harpers"}); !errors.As(err, &rule) {
		t.Errorf("an unknown archetype: %v", err)
	}
	if _, err := s.Propose(ctx, dmCaller, d.ID, uuid.New(), app.Suggestion{Delta: 5, Reason: "Where?"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a change for no Faction: %v", err)
	}
	forbidden := map[string]error{}
	_, forbidden["create"] = s.Create(ctx, playerCaller, d.ID, app.FactionInput{Name: "Mine"})
	forbidden["update"] = s.Update(ctx, playerCaller, d.ID, watch.Faction.ID, app.FactionInput{Name: "Mine"})
	forbidden["delete"] = s.Delete(ctx, playerCaller, d.ID, watch.Faction.ID)
	_, forbidden["propose"] = s.Propose(ctx, playerCaller, d.ID, watch.Faction.ID, app.Suggestion{Delta: 100, Reason: "We are great."})
	for name, err := range forbidden {
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("a Player may %s: %v", name, err)
		}
	}
	if _, err := s.List(ctx, stranger, d.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a stranger lists: %v", err)
	}
	// The edges of what is allowed are allowed, and the score stops at its ends.
	for _, delta := range []int{100, 100, -100, -100, -100} {
		if err := s.Decide(ctx, dmCaller, d.ID, suggest(dmCaller, guild.Faction.ID, app.Suggestion{Delta: delta, Reason: strings.Repeat("a", 500)}).ID, confirm); err != nil {
			t.Fatal(err)
		}
	}
	if got := factionNamed(t, lists(dmCaller), "Grey Hands"); got.Faction.Score != -100 || got.Tier != standing.Hostile {
		t.Fatalf("at the end of the scale = %+v", got.Faction)
	}

	// A DM renames a Faction and removes one; neither is done to another Campaign's.
	if err := s.Update(ctx, dmCaller, d.ID, watch.Faction.ID, app.FactionInput{Name: "The Watch", Archetype: "", Notes: "Honest now."}); err != nil {
		t.Fatal(err)
	}
	if got := factionNamed(t, lists(dmCaller), "The Watch"); got.Faction.Notes != "Honest now." || got.Faction.Score != -20 || got.Faction.Archetype != "" {
		t.Fatalf("renamed = %+v", got.Faction)
	}
	for name, err := range map[string]error{
		"update": s.Update(ctx, dmCaller, d.ID, uuid.New(), app.FactionInput{Name: "Nobody"}),
		"delete": s.Delete(ctx, dmCaller, d.ID, uuid.New()),
	} {
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s of no Faction: %v", name, err)
		}
	}
	if err := s.Delete(ctx, dmCaller, d.ID, guild.Faction.ID); err != nil {
		t.Fatal(err)
	}
	if list := lists(dmCaller); len(list) != 1 {
		t.Fatalf("after removing one = %+v", list)
	}
}

// Every Faction operation reports a database fault at any of its calls.
func TestEveryFactionDatabaseFaultSurfaces(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	svc := func(repo app.FactionRepository) *app.Factions { return &app.Factions{Repo: repo, Now: time.Now} }
	base := svc(pgstore.New(db.Pool()))
	tamsin := hero(t, db, d.ID, playerCaller.Subject, "Tamsin")
	f, err := base.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "The Watch"})
	if err != nil {
		t.Fatal(err)
	}
	if ch, err := base.Propose(ctx, dmCaller, d.ID, f.Faction.ID, app.Suggestion{Character: &tamsin, Delta: 5, Reason: "A favour."}); err != nil || base.Decide(ctx, dmCaller, d.ID, ch.ID, app.Decision{Confirm: true}) != nil {
		t.Fatal(err)
	}
	pending := func(character *domain.CharacterID) domain.ChangeID {
		ch, err := base.Propose(ctx, dmCaller, d.ID, f.Faction.ID, app.Suggestion{Character: character, Delta: 5, Reason: "A favour."})
		if err != nil {
			t.Fatal(err)
		}
		return ch.ID
	}
	ops := map[string]func(s *app.Factions) error{
		"list": func(s *app.Factions) error { _, err := s.List(ctx, dmCaller, d.ID); return err },
		"create": func(s *app.Factions) error {
			_, err := s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "Other"})
			return err
		},
		"update": func(s *app.Factions) error {
			return s.Update(ctx, dmCaller, d.ID, f.Faction.ID, app.FactionInput{Name: "The Watch"})
		},
		"delete": func(s *app.Factions) error {
			doomed, err := base.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "Doomed"})
			if err != nil {
				t.Fatal(err)
			}
			return s.Delete(ctx, dmCaller, d.ID, doomed.Faction.ID)
		},
		"propose": func(s *app.Factions) error {
			_, err := s.Propose(ctx, dmCaller, d.ID, f.Faction.ID, app.Suggestion{Character: &tamsin, Delta: 5, Reason: "A favour."})
			return err
		},
		"confirm for the party": func(s *app.Factions) error {
			return s.Decide(ctx, dmCaller, d.ID, pending(nil), app.Decision{Confirm: true})
		},
		"confirm for a Character": func(s *app.Factions) error {
			return s.Decide(ctx, dmCaller, d.ID, pending(&tamsin), app.Decision{Confirm: true})
		},
		"dismiss": func(s *app.Factions) error {
			return s.Decide(ctx, dmCaller, d.ID, pending(nil), app.Decision{Confirm: false})
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
	// A decision that failed part-way left nothing behind: the Standing is what the one confirmed change made it.
	list, err := base.List(ctx, dmCaller, d.ID)
	if err != nil || list[0].Faction.Score%5 != 0 {
		t.Fatalf("after the faults = %+v %v", list, err)
	}
}
