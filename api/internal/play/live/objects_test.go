package live_test

import (
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

func object(v *live.View, name string) *live.ObjectView {
	for i := range v.Objects {
		if v.Objects[i].Name == name {
			return &v.Objects[i]
		}
	}
	return nil
}

// Doors block sight until opened, a found lever works the door it is linked to and trips its user,
// a barrel breaks and knocks down whoever stands next to it, and all of it survives a restart.
func TestMapObjectsOpenBreakAndTrigger(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 1})
	if u := next(t, tb.dm); u.Reason != "Map Objects need a map." {
		t.Fatalf("without a map = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(w.dungeon(t).ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	for want, cmd := range map[string]live.Command{
		"Choose a door":         {Kind: live.CmdPlaceObject, ObjectKind: "statue", Q: 1},
		"off the map":           {Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 99},
		"Armor Class goes up":   {Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 1, ArmorClass: 31},
		"reaches up to 60 feet": {Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 1, RadiusFt: 65},
		"Link only to objects":  {Kind: live.CmdPlaceObject, ObjectKind: "lever", Q: 1, Links: []string{uuid.NewString()}},
		"No such object":        {Kind: live.CmdRemoveObject, ObjectID: uuid.NewString()},
	} {
		w.hub.Submit(tb.dm, cmd)
		if u := next(t, tb.dm); !strings.Contains(u.Reason, want) {
			t.Errorf("%s = %+v", want, u)
		}
	}
	d, p := tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "door", Q: 1})
	door := object(d.View, "Door")
	if door == nil || *door.AC != 15 || *door.HP != 18 || door.Open || token(p.View, "Goblin") != nil || object(p.View, "Door") == nil || object(p.View, "Door").AC != nil {
		t.Fatalf("a closed door hides the goblin = %+v %+v", door, p.View.Tokens)
	}
	use := func(id string) live.Command {
		return live.Command{Kind: live.CmdUseObject, TokenID: ids["Aria"], ObjectID: id}
	}
	if u := tb.playerSays(use(door.ID)); !object(u.View, "Door").Open || token(u.View, "Goblin") == nil {
		t.Fatalf("opened = %+v", u.View.Objects)
	}
	d, p = tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "lever", ObjectName: "Rusty lever", R: 1, Secret: true, Effect: "prone", Links: []string{door.ID}})
	lever := object(d.View, "Rusty lever")
	if object(p.View, "Rusty lever") != nil || !lever.Secret || lever.Effect != "prone" {
		t.Fatalf("a secret lever = %+v %+v", lever, p.View.Objects)
	}
	if u := tb.playerSays(use(lever.ID)); u.Reason != "No such object." {
		t.Fatalf("an unfound lever = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdFindObject, ObjectID: lever.ID})
	tb.playerSays(use(lever.ID))
	next(t, tb.player)
	next(t, tb.dm)
	v := look(t, w, tb.player)
	if !object(v, "Rusty lever").Open || object(v, "Door").Open || token(v, "Goblin") != nil || effect(token(v, "Aria"), "Prone") == nil {
		t.Fatalf("the lever shuts the door and trips Aria = %+v %+v", v.Objects, token(v, "Aria"))
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "barrel", Q: 1, R: 1, Effect: "prone", RadiusFt: 5})
	barrel := object(d.View, "Barrel")
	if u := tb.playerSays(use(barrel.ID)); !strings.Contains(u.Reason, "cannot be used") {
		t.Fatalf("a barrel is not opened = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdDamageObject, ObjectID: barrel.ID})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "at least 1 hit point") {
		t.Fatalf("no damage = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdDamageObject, ObjectID: barrel.ID, HPDelta: -4})
	tb.dmSays(live.Command{Kind: live.CmdDamageObject, ObjectID: barrel.ID, HPDelta: -20})
	next(t, tb.dm)
	next(t, tb.player)
	v = look(t, w, tb.dm)
	if b := object(v, "Barrel"); !b.Broken || *b.HP != 0 || effect(token(v, "Goblin"), "Prone") == nil {
		t.Fatalf("the barrel bursts and knocks the goblin down = %+v %+v", b, token(v, "Goblin"))
	}
	tb.dmSays(live.Command{Kind: live.CmdDamageObject, ObjectID: barrel.ID, HPDelta: 3})
	w.hub.Close(w.session.ID)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	v = look(t, w, tb.dm)
	if b := object(v, "Barrel"); b.Broken || *b.HP != 3 || !object(v, "Rusty lever").Open || object(v, "Door").Open {
		t.Fatalf("after a restart = %+v", v.Objects)
	}
	if u := tb.playerSays(live.Command{Kind: live.CmdUseObject, TokenID: ids["Goblin"], ObjectID: door.ID}); !strings.Contains(u.Reason, "not yours") {
		t.Fatalf("the goblin is not Aria's = %+v", u)
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdUseObject, TokenID: ids["Goblin"], ObjectID: lever.ID})
	if u := next(t, tb.dm); !strings.Contains(u.Reason, "must stand next to the rusty lever") {
		t.Fatalf("too far = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdRemoveObject, ObjectID: door.ID})
	if v := look(t, w, tb.dm); object(v, "Door") != nil || len(v.Objects) != 2 {
		t.Fatalf("removed = %+v", v.Objects)
	}
}

// In a fight, using an object takes the free object interaction, once a turn and only on one's turn.
func TestUsingObjectsInAFight(t *testing.T) {
	t.Parallel()
	w := setup(t)
	w.hub.Stats = bestiary{owner: w.player.ID}
	rolls := &app.Rolls{
		Repo: pgstore.New(w.pool), Members: pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}, Seed: func() uint64 { return 7 },
		Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: w.hub.RollResolved,
	}
	tb := &table{t: t, w: w, rolls: rolls, dm: join(t, w, w.dm, dmCaller, live.AudienceDM), player: join(t, w, w.player, playerCaller, live.AudienceParty)}
	tb.dmSays(live.Command{Kind: live.CmdSetMap, MapID: uuid.UUID(w.dungeon(t).ID).String()})
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: uuid.NewString(), Q: 0})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Q: 2})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlaceObject, ObjectKind: "curtain", Q: 1})
	curtain := object(d.View, "Curtain").ID
	tb.fight(ids, "Goblin", "Aria")
	use := live.Command{Kind: live.CmdUseObject, TokenID: ids["Aria"], ObjectID: curtain}
	if u := tb.playerSays(use); !strings.Contains(u.Reason, "not Aria's turn") {
		t.Fatalf("off turn = %+v", u)
	}
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(look(t, w, tb.dm), "Goblin").ID})
	if u := tb.playerSays(use); !object(u.View, "Curtain").Open || combatant(u.View, "Aria").Interaction {
		t.Fatalf("drawn = %+v", u.View.Objects)
	}
	if u := tb.playerSays(use); !strings.Contains(u.Reason, "free object interaction") {
		t.Fatalf("a second use = %+v", u)
	}
}
