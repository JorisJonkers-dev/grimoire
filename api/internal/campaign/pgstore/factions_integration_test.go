//go:build integration

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
// factionTable is a Campaign with Tamsin, a Player's Character, and Old Joris, the DM's, and two
// Factions: the Lantern Watch and the Grey Hands.
type factionTable struct {
	t             *testing.T
	ctx           context.Context
	db            *pg.Store
	campaign      domain.CampaignID
	s             *app.Factions
	tamsin, joris domain.CharacterID
	agent         caller.Caller
	watch, guild  app.FactionView
}

func newFactionTable(t *testing.T) *factionTable {
	t.Helper()
	ctx := context.Background()
	db := open(t)
	_, d := npcs(t, pgstore.New(db.Pool()))
	f := &factionTable{
		t: t, ctx: ctx, db: db, campaign: d.ID,
		s:     &app.Factions{Repo: pgstore.New(db.Pool()), Now: func() time.Time { return time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC) }},
		agent: caller.MCP(dmCaller.Subject, "Claude"),
	}
	f.tamsin, f.joris = hero(t, db, d.ID, playerCaller.Subject, "Tamsin"), hero(t, db, d.ID, dmCaller.Subject, "Old Joris")
	var err error
	if f.watch, err = f.s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "  The Lantern Watch ", Archetype: "city-watch", Goals: "Keep the peace.", Territory: "Oakford", Notes: "The captain takes bribes."}); err != nil {
		t.Fatal(err)
	}
	f.guild, _ = f.s.Create(ctx, dmCaller, d.ID, app.FactionInput{Name: "Grey Hands"})
	return f
}

func (f *factionTable) list(c caller.Caller) []app.FactionView {
	f.t.Helper()
	list, err := f.s.List(f.ctx, c, f.campaign)
	if err != nil {
		f.t.Fatal(err)
	}
	return list
}

func (f *factionTable) named(c caller.Caller, name string) app.FactionView {
	f.t.Helper()
	return factionNamed(f.t, f.list(c), name)
}

func (f *factionTable) suggest(c caller.Caller, faction domain.FactionID, g app.Suggestion) domain.StandingChange {
	f.t.Helper()
	ch, err := f.s.Propose(f.ctx, c, f.campaign, faction, g)
	if err != nil {
		f.t.Fatal(err)
	}
	return ch
}

func (f *factionTable) decide(id domain.ChangeID, d app.Decision) {
	f.t.Helper()
	if err := f.s.Decide(f.ctx, dmCaller, f.campaign, id, d); err != nil {
		f.t.Fatal(err)
	}
}

var confirmed = app.Decision{Confirm: true}

func (f *factionTable) sealReturned() domain.StandingChange {
	return f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Delta: 30, Reason: " Returned the stolen seal. ", ShareReason: true})
}

func (f *factionTable) watchmanKilled() domain.StandingChange {
	return f.suggest(f.agent, f.watch.Faction.ID, app.Suggestion{Delta: -70, Reason: "Killed a watchman.", ShareReason: false})
}

// editedDown confirms the agent's suggestion as -50, with the DM's own reason, not shared.
func (f *factionTable) editedDown(id domain.ChangeID) {
	delta, reason, share := -50, " A watchman died in the brawl. ", false
	f.decide(id, app.Decision{Confirm: true, Delta: &delta, Reason: &reason, ShareReason: &share})
}

// decided is the Watch after the DM confirmed the seal (+30), edited the agent's suggestion to -50,
// confirmed Tamsin's +45 and +10, and dismissed Old Joris's -30.
type decided struct {
	byHand, byAgent, own, other, again domain.StandingChange
}

func (f *factionTable) decideAll() decided {
	var d decided
	d.byHand, d.byAgent = f.sealReturned(), f.watchmanKilled()
	d.own = f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Character: &f.tamsin, Delta: 45, Reason: "Tamsin saved the captain's son."})
	d.other = f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Character: &f.joris, Delta: -30, Reason: "Old Joris insulted the captain.", ShareReason: true})
	f.decide(d.byHand.ID, confirmed)
	f.editedDown(d.byAgent.ID)
	f.decide(d.own.ID, confirmed)
	f.decide(d.other.ID, app.Decision{Confirm: false})
	d.again = f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Character: &f.tamsin, Delta: 10, Reason: "Tamsin stood a round."})
	f.decide(d.again.ID, confirmed)
	return d
}

func TestANewFactionIsTrimmedAndStartsNeutral(t *testing.T) {
	t.Parallel()
	if watch := newFactionTable(t).watch; watch.Faction.Name != "The Lantern Watch" || watch.Tier != standing.Neutral || watch.Faction.Score != 0 {
		t.Fatalf("created %+v", watch)
	}
}

func TestASuggestionWaitsAndTheStandingStaysPut(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	byHand, byAgent := f.sealReturned(), f.watchmanKilled()
	watch := f.named(dmCaller, "The Lantern Watch")
	if byHand.Status != domain.ChangePending || byAgent.Status != domain.ChangePending || byHand.Reason != "Returned the stolen seal." || watch.Tier != standing.Neutral || watch.Faction.Score != 0 || len(watch.Changes) != 2 {
		t.Fatalf("suggested %+v %+v, the Watch %+v", byHand, byAgent, watch)
	}
}

func TestTheDMSeesEachSuggestionAndWhereItCameFrom(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.sealReturned()
	byAgent := f.watchmanKilled()
	for _, ch := range f.named(dmCaller, "The Lantern Watch").Changes {
		if ch.Status != domain.ChangePending || ch.DM == nil {
			t.Fatalf("a suggestion on the DM's list = %+v", ch)
		}
		if ch.ID == byAgent.ID && (ch.DM.Origin != "mcp" || ch.DM.Client != "Claude" || ch.DM.Delta != -70) {
			t.Fatalf("the agent's suggestion = %+v", ch.DM)
		}
	}
}

func TestAPlayerSeesNoSuggestion(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.sealReturned()
	if seen := f.named(playerCaller, "The Lantern Watch"); len(seen.Changes) != 0 || len(seen.Personal) != 0 || seen.Tier != standing.Neutral {
		t.Fatalf("a Player sees %+v", seen)
	}
}

func TestOnlyTheDMInPersonDecidesAChange(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	byHand := f.sealReturned()
	for name, c := range map[string]struct {
		who  caller.Caller
		want error
	}{"an agent for the DM": {f.agent, domain.ErrForbidden}, "a Player": {playerCaller, domain.ErrForbidden}, "a stranger": {stranger, domain.ErrNotFound}} {
		t.Run(name, func(t *testing.T) {
			if err := f.s.Decide(f.ctx, c.who, f.campaign, byHand.ID, confirmed); !errors.Is(err, c.want) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if got := f.named(dmCaller, "The Lantern Watch"); got.Faction.Score != 0 {
		t.Fatalf("a refused decision moved the Standing: %+v", got.Faction)
	}
}

func TestAConfirmedChangeMovesThePartysStanding(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.decide(f.sealReturned().ID, confirmed)
	if got := f.named(dmCaller, "The Lantern Watch"); got.Faction.Score != 30 || got.Tier != standing.Friendly {
		t.Fatalf("the Watch = %+v", got.Faction)
	}
}

func TestAnEditedChangeMovesByWhatTheDMSaysWithTheirReason(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	byAgent := f.watchmanKilled()
	f.editedDown(byAgent.ID)
	watch := f.named(dmCaller, "The Lantern Watch")
	if watch.Faction.Score != -50 {
		t.Fatalf("score = %d", watch.Faction.Score)
	}
	for _, ch := range watch.Changes {
		if ch.ID == byAgent.ID && (ch.Status != domain.ChangeConfirmed || ch.DM.Delta != -50 || ch.Reason != "A watchman died in the brawl." || ch.DM.ShareReason || ch.Rose) {
			t.Fatalf("the edited change = %+v %+v", ch, ch.DM)
		}
	}
}

func TestAPersonalStandingStartsFromThePartysAndGoesItsOwnWay(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.decide(f.sealReturned().ID, confirmed)
	f.editedDown(f.watchmanKilled().ID)
	f.decide(f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Character: &f.tamsin, Delta: 45, Reason: "Tamsin saved the captain's son."}).ID, confirmed)
	watch := f.named(dmCaller, "The Lantern Watch")
	if watch.Faction.Score != -20 || watch.Tier != standing.Unfriendly || len(watch.Personal) != 1 || watch.Personal[0].CharacterID != f.tamsin || *watch.Personal[0].Score != 25 || watch.Personal[0].Tier != standing.Friendly {
		t.Fatalf("the Watch = %+v %+v", watch.Faction, watch.Personal)
	}
}

func TestADismissedChangeMovesNothing(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	other := f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Character: &f.joris, Delta: -30, Reason: "Old Joris insulted the captain.", ShareReason: true})
	f.decide(other.ID, app.Decision{Confirm: false})
	watch := f.named(dmCaller, "The Lantern Watch")
	if watch.Faction.Score != 0 || len(watch.Personal) != 0 {
		t.Fatalf("the Watch = %+v %+v", watch.Faction, watch.Personal)
	}
	for _, ch := range watch.Changes {
		if ch.ID == other.ID && (ch.Status != domain.ChangeDismissed || ch.DM.Delta != -30) {
			t.Fatalf("the dismissed change = %+v %+v", ch, ch.DM)
		}
	}
}

func TestTheNextChangeForACharacterMovesTheirPersonalStanding(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.decideAll()
	if got := f.named(dmCaller, "The Lantern Watch"); *got.Personal[0].Score != 35 || got.Faction.Score != -20 {
		t.Fatalf("the Watch = %+v %+v", got.Personal, got.Faction)
	}
}

func TestAChangeIsDecidedOnlyThroughItsOwnCampaign(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	campaigns, _ := service(t, pgstore.New(f.db.Pool()))
	elsewhere, err := campaigns.Create(f.ctx, dmCaller, app.CreateInput{Name: "Elsewhere", DisplayName: "Joris"})
	if err != nil {
		t.Fatal(err)
	}
	stray := f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Delta: 60, Reason: "A stray change."})
	if err := f.s.Decide(f.ctx, dmCaller, elsewhere.ID, stray.ID, confirmed); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("decided through another Campaign = %v", err)
	}
	if _, err := f.s.Propose(f.ctx, dmCaller, elsewhere.ID, f.watch.Faction.ID, app.Suggestion{Delta: 60, Reason: "A stray change."}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("proposed through another Campaign = %v", err)
	}
	if got := f.named(dmCaller, "The Lantern Watch"); got.Faction.Score != 0 {
		t.Fatalf("the Standing moved: %+v", got.Faction)
	}
}

func TestAChangeIsNotDecidedTwice(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	d := f.decideAll()
	for name, id := range map[string]domain.ChangeID{"confirmed": d.byHand.ID, "dismissed": d.other.ID} {
		t.Run(name, func(t *testing.T) {
			if err := f.s.Decide(f.ctx, dmCaller, f.campaign, id, confirmed); !errors.Is(err, domain.ErrConflict) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestDecidingNoChangeIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	if err := f.s.Decide(f.ctx, dmCaller, f.campaign, uuid.New(), confirmed); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestAPlayerSeesTheTierTheirOwnPersonalTierAndOnlySharedReasons(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	d := f.decideAll()
	seen := f.named(playerCaller, "The Lantern Watch")
	if seen.Tier != standing.Unfriendly || seen.DM || seen.Faction.Score != 0 || seen.Faction.Notes != "" || seen.Faction.Goals != "" || seen.Faction.Territory != "" {
		t.Fatalf("a Player's Watch = %+v", seen)
	}
	if len(seen.Personal) != 1 || seen.Personal[0].CharacterID != f.tamsin || seen.Personal[0].Tier != standing.Friendly || seen.Personal[0].Score != nil {
		t.Fatalf("a Player's Personal Standing = %+v", seen.Personal)
	}
	if len(seen.Changes) != 4 {
		t.Fatalf("a Player's changes = %+v", seen.Changes)
	}
	for _, ch := range seen.Changes {
		want := map[domain.ChangeID]struct {
			reason string
			rose   bool
		}{d.byHand.ID: {"Returned the stolen seal.", true}, d.byAgent.ID: {"", false}, d.own.ID: {"", true}, d.again.ID: {"", true}}[ch.ID]
		if ch.DM != nil || ch.Status != domain.ChangeConfirmed || ch.Reason != want.reason || ch.Rose != want.rose {
			t.Fatalf("a Player's change = %+v", ch)
		}
	}
}

func TestNothingTheDMKeepsReachesAPlayer(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.decideAll()
	raw, _ := json.Marshal(f.list(playerCaller))
	for _, secret := range []string{"watchman", "bribes", "Old Joris", "insulted", "Keep the peace", "Oakford", "captain's son", "stood a round", "Claude", "mcp", ":-50", ":-20", `"Score":25`, `"Delta"`} {
		t.Run(secret, func(t *testing.T) {
			if strings.Contains(string(raw), secret) {
				t.Fatalf("it reached a Player: %s", raw)
			}
		})
	}
}

func TestTheDMSeesEveryPersonalStandingAndAPlayerOnlyTheirOwn(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.decide(f.suggest(dmCaller, f.guild.Faction.ID, app.Suggestion{Character: &f.joris, Delta: 80, Reason: "An old debt.", ShareReason: true}).ID, confirmed)
	if got := f.named(playerCaller, "Grey Hands"); len(got.Personal) != 0 || len(got.Changes) != 0 {
		t.Fatalf("another Character's Standing reached a Player: %+v", got)
	}
	if got := f.named(dmCaller, "Grey Hands"); len(got.Personal) != 1 || got.Personal[0].Tier != standing.Allied || got.Faction.Score != 0 {
		t.Fatalf("the DM's Grey Hands = %+v", got)
	}
}

func TestAFactionOrChangeOutsideTheRulesIsRefused(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	pending := f.suggest(dmCaller, f.watch.Faction.ID, app.Suggestion{Delta: 5, Reason: "A favour."})
	zero, blank := 0, " "
	create := func(in app.FactionInput) func() error {
		return func() error { _, err := f.s.Create(f.ctx, dmCaller, f.campaign, in); return err }
	}
	propose := func(g app.Suggestion) func() error {
		return func() error { _, err := f.s.Propose(f.ctx, dmCaller, f.campaign, f.watch.Faction.ID, g); return err }
	}
	for name, try := range map[string]func() error{
		"no name":        create(app.FactionInput{Name: " "}),
		"a long name":    create(app.FactionInput{Name: strings.Repeat("a", 81)}),
		"long goals":     create(app.FactionInput{Name: "A", Goals: strings.Repeat("a", 2001)}),
		"long territory": create(app.FactionInput{Name: "A", Territory: strings.Repeat("a", 2001)}),
		"long notes":     create(app.FactionInput{Name: "A", Notes: strings.Repeat("a", 2001)}),
		"no change":      propose(app.Suggestion{Delta: 0, Reason: "Nothing."}),
		"too much":       propose(app.Suggestion{Delta: 101, Reason: "Everything."}),
		"no reason":      propose(app.Suggestion{Delta: 5, Reason: "  "}),
		"a long reason":  propose(app.Suggestion{Delta: 5, Reason: strings.Repeat("a", 501)}),
		"confirmed at nothing": func() error {
			return f.s.Decide(f.ctx, dmCaller, f.campaign, pending.ID, app.Decision{Confirm: true, Delta: &zero})
		},
		"confirmed for no reason": func() error {
			return f.s.Decide(f.ctx, dmCaller, f.campaign, pending.ID, app.Decision{Confirm: true, Reason: &blank})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := try(); !errors.Is(err, domain.ErrInvalid) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAPersonalStandingForNoCharacterIsRefused(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	nobody := domain.CharacterID(uuid.New())
	var rule *apperr.RuleError
	if _, err := f.s.Propose(f.ctx, dmCaller, f.campaign, f.watch.Faction.ID, app.Suggestion{Character: &nobody, Delta: 5, Reason: "Who?"}); !errors.As(err, &rule) {
		t.Fatalf("err = %v", err)
	}
}

func TestAnUnknownFactionArchetypeIsRefused(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	var rule *apperr.RuleError
	if _, err := f.s.Create(f.ctx, dmCaller, f.campaign, app.FactionInput{Name: "Harpers", Archetype: "harpers"}); !errors.As(err, &rule) {
		t.Fatalf("err = %v", err)
	}
}

func TestAChangeForNoFactionIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	if _, err := f.s.Propose(f.ctx, dmCaller, f.campaign, uuid.New(), app.Suggestion{Delta: 5, Reason: "Where?"}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestAPlayerCannotChangeFactions(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	for name, try := range map[string]func() error{
		"create": func() error {
			_, err := f.s.Create(f.ctx, playerCaller, f.campaign, app.FactionInput{Name: "Mine"})
			return err
		},
		"update": func() error {
			return f.s.Update(f.ctx, playerCaller, f.campaign, f.watch.Faction.ID, app.FactionInput{Name: "Mine"})
		},
		"delete": func() error { return f.s.Delete(f.ctx, playerCaller, f.campaign, f.watch.Faction.ID) },
		"propose": func() error {
			_, err := f.s.Propose(f.ctx, playerCaller, f.campaign, f.watch.Faction.ID, app.Suggestion{Delta: 100, Reason: "We are great."})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := try(); !errors.Is(err, domain.ErrForbidden) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAStrangerCannotListFactions(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	if _, err := f.s.List(f.ctx, stranger, f.campaign); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestTheStandingScoreStopsAtItsEnds(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	for _, delta := range []int{100, 100, -100, -100, -100} {
		f.decide(f.suggest(dmCaller, f.guild.Faction.ID, app.Suggestion{Delta: delta, Reason: strings.Repeat("a", 500)}).ID, confirmed)
	}
	if got := f.named(dmCaller, "Grey Hands"); got.Faction.Score != -100 || got.Tier != standing.Hostile {
		t.Fatalf("Grey Hands = %+v", got.Faction)
	}
}

func TestRenamingAFactionKeepsItsStanding(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	f.decide(f.sealReturned().ID, confirmed)
	if err := f.s.Update(f.ctx, dmCaller, f.campaign, f.watch.Faction.ID, app.FactionInput{Name: "The Watch", Archetype: "", Notes: "Honest now."}); err != nil {
		t.Fatal(err)
	}
	if got := f.named(dmCaller, "The Watch"); got.Faction.Notes != "Honest now." || got.Faction.Score != 30 || got.Faction.Archetype != "" {
		t.Fatalf("renamed = %+v", got.Faction)
	}
}

func TestChangingOrRemovingNoFactionIsNotFound(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	for name, try := range map[string]func() error{
		"update": func() error {
			return f.s.Update(f.ctx, dmCaller, f.campaign, uuid.New(), app.FactionInput{Name: "Nobody"})
		},
		"delete": func() error { return f.s.Delete(f.ctx, dmCaller, f.campaign, uuid.New()) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := try(); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestADMRemovesAFaction(t *testing.T) {
	t.Parallel()
	f := newFactionTable(t)
	if err := f.s.Delete(f.ctx, dmCaller, f.campaign, f.guild.Faction.ID); err != nil {
		t.Fatal(err)
	}
	if list := f.list(dmCaller); len(list) != 1 {
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
		t.Run(name, func(t *testing.T) {
			pgtest.EveryFault(t, func(fault *pgtest.Faulty) error {
				err := op(svc(pgstore.NewFaulty(db.Pool(), fault)))
				if err != nil && !errors.Is(err, pgtest.ErrInjected) {
					t.Fatalf("%s: %v", name, err)
				}
				return err
			})
		})
	}
	// A decision that failed part-way left nothing behind: the Standing is what the one confirmed change made it.
	list, err := base.List(ctx, dmCaller, d.ID)
	if err != nil || list[0].Faction.Score%5 != 0 {
		t.Fatalf("after the faults = %+v %v", list, err)
	}
}
