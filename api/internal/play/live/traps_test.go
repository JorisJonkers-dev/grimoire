package live_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// dungeonTable puts Aria, who carries an iron key, at the start of a dungeon map, with a goblin two
// hexes east.
func dungeonTable(t *testing.T) (world, *table, map[string]string) {
	t.Helper()
	ctx := context.Background()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	aria, bag := uuid.New(), uuid.New()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO campaign.characters (id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug, ability_method, hp_max, hp_current)
			VALUES ($1, $2, $3, 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 12, 12)`, []any{aria, w.session.CampaignID, w.player.ID}},
		{`INSERT INTO campaign.containers (id, campaign_id, kind, character_id, label, created_at) VALUES ($1, $2, 'character', $3, 'Aria', now())`, []any{bag, w.session.CampaignID, aria}},
		{`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at) VALUES ($1, $2, 'iron-key', 1, true, false, now())`, []any{uuid.New(), bag}},
	} {
		if _, err := w.pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(w.dungeon(t).ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: aria.String(), Q: 0})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	return w, tb, ids
}

// A passive Perception of 13 finds a DC 12 needle trap on approach but not a DC 14 pit, which springs
// when Aria steps on it; a found trap disarms on a success and springs on a failure by 5 or more.
func TestTrapsDetectTriggerAndDisarm(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	place := func(name string, q, r, detect int) string {
		d, _ := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "trap", ObjectName: name, Q: q, R: r, Secret: true, DetectDC: detect, DisarmDC: 15, Effect: "prone"})
		return object(d.View, name).ID
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 4, R: 0})
	pit := place("Pit", 2, 0, 14)
	needle := place("Needle", 0, 1, 12)
	gas := place("Gas vent", 1, 0, 0)
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdPlaceObject, ObjectKind: "trap", Q: 1, DetectDC: 41})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "DCs run from 1 to 40") {
		t.Fatalf("a DC of 41 = %+v", u)
	}
	if v := look(t, w, tb.player); len(v.Objects) != 0 {
		t.Fatalf("hidden traps = %+v", v.Objects)
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Goblin"], Q: 3, R: 0})
	next(t, tb.dm)
	next(t, tb.player)
	v := look(t, w, tb.player)
	if object(v, "Needle") == nil || object(v, "Pit") != nil {
		t.Fatalf("passive Perception 13 finds the needle only = %+v", v.Objects)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdDisarm, TokenID: ids["Aria"], ObjectID: pit}); u.Reason != "No such object." {
		t.Fatalf("an unfound pit = %+v", u)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdDisarm, TokenID: ids["Aria"], ObjectID: needle}); u.Kind != live.UpdView {
		t.Fatalf("disarm the needle = %+v", u)
	}
	tb.fill(uuid.UUID(lastRoll(t, w, "Disarm the needle").ID).String(), w.player, 13)
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	if n := object(look(t, w, tb.dm), "Needle"); n.Armed {
		t.Fatalf("13 + 2 disarms the needle = %+v", n)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdDisarm, TokenID: ids["Aria"], ObjectID: needle}); !strings.Contains(u.Reason, "no armed trap") {
		t.Fatalf("a disarmed needle = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdFindObject, ObjectID: gas})
	if u := tb.playerSays(live.Command{Kind: live.CmdDisarm, TokenID: ids["Aria"], ObjectID: gas}); u.Kind != live.UpdView {
		t.Fatalf("disarm the vent = %+v", u)
	}
	tb.fill(uuid.UUID(lastRoll(t, w, "Disarm the gas vent").ID).String(), w.player, 1)
	for range 3 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v = look(t, w, tb.dm)
	if g := object(v, "Gas vent"); g.Armed || effect(token(v, "Aria"), "Prone") == nil {
		t.Fatalf("a disarm failed by 12 sets the vent off = %+v %+v", g, token(v, "Aria").Effects)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: effect(token(v, "Aria"), "Prone").ID})
	tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: 2, R: 0})
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	v = look(t, w, tb.player)
	if p := object(v, "Pit"); p == nil || effect(token(v, "Aria"), "Prone") == nil {
		t.Fatalf("stepping on the pit springs it = %+v %+v", v.Objects, token(v, "Aria"))
	}
	if p := object(look(t, w, tb.dm), "Pit"); p.Armed || p.Secret {
		t.Fatalf("a sprung pit is spent and found = %+v", p)
	}
}

// A locked door holds until its key, picked or forced lock, or Knock opens it.
func TestLocks(t *testing.T) {
	t.Parallel()
	w, tb, ids := dungeonTable(t)
	door := func(name string, q, r int, key string) string {
		d, _ := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "door", ObjectName: name, Q: q, R: r, LockDC: 15, Key: key})
		return object(d.View, name).ID
	}
	vault := door("Vault", 1, 0, "iron-key")
	cell := door("Cell", 0, 1, "brass-key")
	gate := door("Gate", 1, 1, "")
	unlock := func(id, method string) live.Command {
		return live.Command{Kind: live.CmdUnlock, TokenID: ids["Aria"], ObjectID: id, Method: method}
	}
	for want, cmd := range map[string]live.Command{
		"is locked":           {Kind: live.CmdUseObject, TokenID: ids["Aria"], ObjectID: vault},
		"carries no key":      unlock(cell, "key"),
		"Unlock with the key": unlock(cell, "spell"),
		"not yours to play":   {Kind: live.CmdUnlock, TokenID: ids["Goblin"], ObjectID: cell, Method: "key"},
		"No such creature":    {Kind: live.CmdUnlock, TokenID: "nope", ObjectID: cell, Method: "key"},
		"no armed trap":       {Kind: live.CmdDisarm, TokenID: ids["Aria"], ObjectID: cell},
		"No such object":      {Kind: live.CmdUnlock, TokenID: ids["Aria"], ObjectID: uuid.NewString(), Method: "key"},
	} {
		if u := tb.playerSays(cmd); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	if u := tb.playerSays(unlock(vault, "key")); object(u.View, "Vault").Locked {
		t.Fatal("the iron key opens the vault")
	}
	if u := tb.playerSays(unlock(vault, "key")); !strings.Contains(u.Reason, "not locked") {
		t.Fatalf("unlocked twice = %+v", u)
	}
	tb.playerSays(unlock(cell, "tools"))
	tb.fill(uuid.UUID(lastRoll(t, w, "Pick the lock of the cell").ID).String(), w.player, 13)
	for range 2 {
		next(t, tb.player)
		next(t, tb.dm)
	}
	if object(look(t, w, tb.player), "Cell").Locked {
		t.Fatal("13 + 2 picks the cell")
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 0, R: 2})
	if u := tb.playerSays(unlock(gate, "force")); u.Kind != live.UpdView {
		t.Fatalf("force the gate = %+v", u)
	}
	tb.fill(uuid.UUID(lastRoll(t, w, "Force the gate").ID).String(), w.player, 2)
	next(t, tb.player)
	next(t, tb.dm)
	if !object(look(t, w, tb.player), "Gate").Locked {
		t.Fatal("a weak shove leaves the gate locked")
	}
	tb.dmSays(live.Command{Kind: live.CmdMove, TokenID: ids["Aria"], Q: 4, R: 0})
	if u := tb.playerSays(unlock(gate, "knock")); u.View == nil || object(u.View, "Gate").Locked {
		t.Fatalf("Knock opens the gate = %+v", u)
	}
}
