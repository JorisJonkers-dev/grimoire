package live_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/difficulty"
)

// preset sets the Campaign's difficulty, as the DM does outside the Session.
func preset(t *testing.T, w world, slug string) {
	t.Helper()
	if _, err := w.pool.Exec(context.Background(), "UPDATE campaign.campaigns SET difficulty = $2 WHERE id = $1", w.session.CampaignID, slug); err != nil {
		t.Fatal(err)
	}
}

type unreadableDifficulty struct{ live.Store }

func (unreadableDifficulty) Difficulty(context.Context, uuid.UUID) (string, error) {
	return difficulty.Hard, errors.New("no such column")
}

// A difficulty preset changes the hit points an enemy comes onto the map with and what its attacks
// roll at; allies, NPCs and Characters play by the rules as written. The Session under way follows
// the preset as the DM changes it, and a creature keeps the hit points it came with.
func TestADifficultyPresetChangesEnemiesHitPointsAndAttacks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := magicTable(t)
	hp := func(v *live.View, label string) [2]int {
		t.Helper()
		tv := token(v, label)
		if tv == nil || tv.HP == nil || tv.HPMax == nil {
			t.Fatalf("%s = %+v", label, tv)
		}
		return [2]int{*tv.HP, *tv.HPMax}
	}
	place := func(c live.Command) *live.View {
		t.Helper()
		c.Kind = live.CmdPlace
		d, _ := tb.dmSays(c)
		ids[c.Label] = token(d.View, c.Label).ID
		return d.View
	}
	preview := func(sub *live.Subscriber, attacker, target string) *live.AttackPreview {
		t.Helper()
		w.hub.Submit(sub, live.Command{Kind: live.CmdPreviewAttack, TokenID: ids[attacker], AttackNo: 0, TargetID: ids[target]})
		// Views of what went before may still be on their way; the preview comes after them.
		for {
			u := next(t, sub)
			if u.Kind == live.UpdRejected {
				t.Fatalf("%s at %s: %+v", attacker, target, u)
			}
			if u.Preview != nil {
				return u.Preview
			}
		}
	}
	rolled := func(id string) domain.Roll {
		t.Helper()
		roll, err := pgstore.New(w.pool).Roll(ctx, w.session.CampaignID, domain.RollID(uuid.MustParse(id)))
		if err != nil {
			t.Fatal(err)
		}
		return roll
	}
	added := func(roll domain.Roll) domain.Modifier {
		for _, m := range roll.Modifiers {
			if strings.HasSuffix(m.Label, " difficulty") {
				return m
			}
		}
		return domain.Modifier{Label: "", Value: 0}
	}

	// Standard, as a Campaign starts: a goblin comes with its 7 hit points.
	v := place(live.Command{MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Raider", Q: 1})
	if got := hp(v, "Raider"); got != [2]int{7, 7} {
		t.Fatalf("standard = %v", got)
	}
	preset(t, w, difficulty.Hard)
	v = place(live.Command{MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Brute", Q: -2})
	if got := hp(v, "Brute"); got != [2]int{8, 8} {
		t.Fatalf("hard = %v", got)
	}
	// The goblin that was there keeps what it came with; an ally and an NPC are no enemies.
	if got := hp(v, "Raider"); got != [2]int{7, 7} {
		t.Fatalf("placed before = %v", got)
	}
	v = place(live.Command{MonsterSlug: "goblin", TokenKind: domain.TokenParty, Label: "Ally", Q: 1, R: -1})
	if got := hp(v, "Ally"); got != [2]int{7, 7} {
		t.Fatalf("an ally = %v", got)
	}
	v = place(live.Command{MonsterSlug: "goblin", TokenKind: domain.TokenNPC, Label: "Innkeeper", Q: -2, R: 2})
	if got := hp(v, "Innkeeper"); got != [2]int{7, 7} {
		t.Fatalf("an NPC = %v", got)
	}
	// An encounter spawned comes with the preset's hit points too.
	d, _ := tb.dmSays(live.Command{Kind: live.CmdSpawnEncounter, Q: 2, R: -2, Monsters: []live.SpawnMonster{{Slug: "hobgoblin", Count: 1}}})
	if got := hp(d.View, "Hobgoblin"); got != [2]int{13, 13} {
		t.Fatalf("spawned hard = %v", got)
	}
	preset(t, w, difficulty.Story)
	v = place(live.Command{MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Runt", Q: 2, R: 0})
	if got := hp(v, "Runt"); got != [2]int{5, 5} {
		t.Fatalf("story = %v", got)
	}

	// In a fight the preset is what an enemy's attack rolls at, and the Roll Card says so.
	preset(t, w, difficulty.Standard)
	tb.fight(ids, "Raider", "Aria", "Ally")
	plain := preview(tb.dm, "Raider", "Aria")
	if len(plain.Reasons) != 1 {
		t.Fatalf("standard = %v", plain.Reasons)
	}
	preset(t, w, difficulty.Hard)
	hard := preview(tb.dm, "Raider", "Aria")
	if !slices.Contains(hard.Reasons, "Hard difficulty: +2 to hit") || hard.HitChance != plain.HitChance+10 || len(hard.Reasons) != 2 {
		t.Fatalf("hard preview = %+v, standard %d", hard, plain.HitChance)
	}
	preset(t, w, difficulty.Story)
	story := preview(tb.dm, "Raider", "Aria")
	if !slices.Contains(story.Reasons, "Story difficulty: -2 to hit") || story.HitChance != plain.HitChance-10 {
		t.Fatalf("story preview = %+v, standard %d", story, plain.HitChance)
	}
	struck, _ := tb.dmSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Raider"], TargetID: ids["Aria"]})
	view := struck.View
	if m := added(rolled(view.Combat.Attack.RollID)); m.Label != "Story difficulty" || m.Value != -2 {
		t.Fatalf("story on the Roll Card = %+v", m)
	}
	tb.fill(view.Combat.Attack.RollID, w.dm, 1)
	next(t, tb.dm)
	next(t, tb.player)

	// A Character's attack and an ally's are as written, whatever the preset.
	preset(t, w, difficulty.Hard)
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(view, "Raider").ID})
	if p := preview(tb.player, "Aria", "Raider"); len(p.Reasons) != 1 {
		t.Fatalf("a Character's attack = %v", p.Reasons)
	}
	// An enemy's opportunity attack rolls at the preset as well.
	u := tb.playerSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Aria"], Q: -1, R: 0})
	if u.View == nil || u.View.Combat.Prompt == nil {
		t.Fatalf("leaving the Raider's reach = %+v", u)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdReact, Use: true})
	if m := added(rolled(d.View.Combat.Attack.RollID)); m.Label != "Hard difficulty" || m.Value != 2 {
		t.Fatalf("the opportunity attack = %+v", m)
	}
	tb.fill(d.View.Combat.Attack.RollID, w.dm, 1)
	next(t, tb.dm)
	next(t, tb.player)
	tb.playerSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(view, "Aria").ID})
	if p := preview(tb.dm, "Ally", "Raider"); len(p.Reasons) != 1 {
		t.Fatalf("an ally's attack = %v", p.Reasons)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(view, "Ally").ID})
	// Aria walked off: the Raider closes in again.
	tb.dmSays(live.Command{Kind: live.CmdWalk, TokenID: ids["Raider"], Q: 0, R: 0})

	// Every roll live play opened, initiative and attacks alike, was asked of its roller: karmic dice go by those.
	var asked, all int
	if err := w.pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE asked), count(*) FROM play.roll_requests WHERE campaign_id = $1", w.session.CampaignID).Scan(&asked, &all); err != nil || all < 5 || asked != all {
		t.Fatalf("asked rolls = %d of %d %v", asked, all, err)
	}

	// A preset that cannot be read plays by the rules as written.
	w.hub.Close(w.session.ID)
	w.hub.Store = unreadableDifficulty{Store: w.hub.Store}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	v = place(live.Command{MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Stray", Q: 2, R: 1})
	if got := hp(v, "Stray"); got != [2]int{7, 7} {
		t.Fatalf("unreadable = %v", got)
	}
	if p := preview(tb.dm, "Raider", "Aria"); p.HitChance != plain.HitChance || len(p.Reasons) != 1 {
		t.Fatalf("unreadable preview = %+v", p)
	}
}
