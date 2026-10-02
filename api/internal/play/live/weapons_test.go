package live_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// armoury gives a Character one attack per weapon in hand, and two more AC behind a shield.
type armoury struct{ bestiary }

func (a armoury) Holding(_ context.Context, _ caller.Caller, _, _ uuid.UUID, weapons []string, shield bool) (domain.Stats, error) {
	st := domain.Stats{AC: 14}
	if shield {
		st.AC += 2
	}
	for _, w := range weapons {
		st.Attacks = append(st.Attacks, domain.Attack{Name: w, ToHit: 5, ReachFt: 5, Damage: "1d8", DamageType: "slashing"})
	}
	return st, nil
}

// A Character swaps between a melee set and a ranged set: out of a fight for free, in one paying the
// equip rules, a shield taking the action; the token fights with the new set and the set is kept.
func TestSwappingWeaponSets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	w.hub.Stats = armoury{bestiary{owner: w.player.ID}}
	stocked(t, w)
	weapon := func(slug, name string) snapshot.Item {
		it := item(slug, name, 3)
		it.Category = "weapon"
		return it
	}
	arms := snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Items:     []snapshot.Item{weapon("longsword", "Longsword"), weapon("longbow", "Longbow"), item("shield", "Shield", 6)},
	}
	if _, err := comppg.New(w.pool).Import(ctx, arms, "arms"); err != nil {
		t.Fatal(err)
	}
	mine := containerNamed(t, look(t, w, join(t, w, w.dm, dmCaller, live.AudienceDM)), "Aria")
	w.hub.Close(w.session.ID)
	aria, box := uuid.MustParse(mine.CharacterID), mine.ID
	for slug, slot := range map[string]string{"longsword": "main_hand", "shield": "off_hand", "longbow": "ranged_main"} {
		if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, equipped_slot, created_at)
			VALUES ($1, $2, $3, 1, true, false, $4, now())`, uuid.New(), box, slug, slot); err != nil {
			t.Fatal(err)
		}
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tb.dmSays(live.Command{Kind: live.CmdPlace, CharacterID: aria.String()})
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "hobgoblin", TokenKind: domain.TokenEnemy, Q: 1})
	ids := map[string]string{}
	for _, tv := range d.View.Tokens {
		ids[tv.Label] = tv.ID
	}
	refuse := func(want string, cmd live.Command) {
		t.Helper()
		w.hub.Submit(tb.player, cmd)
		if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	holds := func(v *live.View, ac int, attack string) {
		t.Helper()
		tv := tokenNamed(t, v, "Aria")
		if tv.AC == nil || *tv.AC != ac || len(tv.Attacks) != 1 || tv.Attacks[0].Name != attack {
			t.Fatalf("Aria holds %s at AC %d = %+v %+v", attack, ac, tv.AC, tv.Attacks)
		}
	}
	set := func() string {
		t.Helper()
		var s string
		if err := w.pool.QueryRow(ctx, "SELECT weapon_set FROM campaign.characters WHERE id = $1", aria).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	refuse("No such creature", live.Command{Kind: live.CmdSwapWeapons, TokenID: uuid.NewString()})
	refuse("not yours", live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Hobgoblin"]})
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Hobgoblin"]})
	if u := next(t, tb.dm); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "no weapon sets") {
		t.Fatalf("a monster has no weapon sets = %+v", u)
	}
	p := tb.playerSays(live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Aria"]})
	holds(p.View, 14, "longbow")
	if set() != "ranged" {
		t.Fatalf("the ranged set is kept, got %s", set())
	}

	d, _ = tb.dmSays(live.Command{Kind: live.CmdStartCombat, Combatants: []live.CombatantSetup{{TokenID: ids["Aria"], SpeedFt: 30}, {TokenID: ids["Hobgoblin"], SpeedFt: 30}}})
	tb.roll(combatant(d.View, "Aria"), w.player, 20)
	tb.roll(combatant(d.View, "Hobgoblin"), w.dm, 2)
	p = tb.playerSays(live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Aria"]})
	holds(p.View, 16, "longsword")
	if a := combatant(p.View, "Aria"); a.Action || !a.Interaction {
		t.Fatalf("taking up a shield costs the action, not the free interaction = %+v", a)
	}
	refuse("no time left", live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Aria"]})
	if u := tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(p.View, "Aria").ID}); u.View == nil {
		t.Fatalf("Aria ends her turn = %+v", u)
	}
	refuse("not Aria's turn", live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Aria"]})
	w.hub.Close(w.session.ID)
	holds(look(t, w, join(t, w, w.dm, dmCaller, live.AudienceDM)), 16, "longsword")
	if set() != "melee" {
		t.Fatalf("the melee set is kept, got %s", set())
	}
	var main string
	if err := w.pool.QueryRow(ctx, "SELECT weapon_slug FROM campaign.character_weapons WHERE character_id = $1", aria).Scan(&main); err != nil || main != "longsword" {
		t.Fatalf("the sheet holds the longsword = %q %v", main, err)
	}
	w.hub.Close(w.session.ID)
	w.hub.Stats = bestiary{owner: w.player.ID}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	tb.dmSays(live.Command{Kind: live.CmdEndCombat})
	refuse("could not be read", live.Command{Kind: live.CmdSwapWeapons, TokenID: ids["Aria"]})
}
