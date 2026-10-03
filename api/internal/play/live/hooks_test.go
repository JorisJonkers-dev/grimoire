package live_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
)

const fumbleTable = `{"dice":"1d8","results":[{"from":1,"to":2,"text":"You fall flat on your face.","effect":"prone"},
{"from":3,"to":4,"text":"Your weapon slips from your grip."},{"from":5,"to":5,"text":"A rope uncoils from your pack.","item":"rope","quantity":2},
{"from":6,"to":6,"text":"A trinket falls from nowhere.","item":"no-such-item"}]}`

// libraryTable puts a Roll Table in the DM's Library and links it to the Campaign.
func libraryTable(t *testing.T, w world, name, design string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO library.entries (id, owner_subject, kind, name, design, created_at, updated_at) VALUES ($1, $2, 'table', $3, $4, now(), now())`,
		id, dmCaller.Subject, name, design); err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, `INSERT INTO library.campaign_links (campaign_id, entry_id, linked_at, updated_at) VALUES ($1, $2, now(), now())`, w.session.CampaignID, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// hookOn authors a Rule Variant of the Campaign's own, as the DM does outside the Session.
func hookOn(t *testing.T, w world, name, point string, table *uuid.UUID, effect string) {
	t.Helper()
	var fx any
	if effect != "" {
		fx = effect
	}
	if _, err := w.pool.Exec(context.Background(), `INSERT INTO campaign.rule_variant_hooks (id, campaign_id, name, hook, roll_table, effect, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, clock_timestamp())`, uuid.New(), w.session.CampaignID, name, point, table, fx); err != nil {
		t.Fatal(err)
	}
}

type unreadableHooks struct{ live.Store }

func (unreadableHooks) RuleHooks(context.Context, uuid.UUID) ([]live.Hook, error) {
	return []live.Hook{{Name: "Stray", Point: "drop-to-0", Table: nil, Effect: "poisoned"}}, errors.New("no such table")
}

// faults counts what the runtime logs as an error.
type faults struct {
	slog.Handler
	seen atomic.Int32
}

func (f *faults) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		f.seen.Add(1)
	}
	return f.Handler.Handle(ctx, r)
}

type unreadableItems struct{ live.Store }

func (unreadableItems) Items(context.Context, uuid.UUID, []string) (map[string]domain.ItemInfo, error) {
	return map[string]domain.ItemInfo{"rope": {}}, errors.New("no such table")
}

type unreadableTables struct{ live.Store }

func (unreadableTables) RollTables(context.Context, uuid.UUID) ([]live.Table, error) {
	return nil, errors.New("no such table")
}

// A critical fumble table: the DM hooks a Roll Table of the Library to a natural 1 on an attack roll.
// Whoever fumbles rolls on it; the result's words reach every screen that sees them and the Action
// Log, its Effect lands on them, and its Item drops as loot.
func TestACriticalFumbleRollsOnTheCampaignsFumbleTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := magicTable(t)
	if _, err := comppg.New(w.pool).Import(ctx, snapshot.Snapshot{
		Documents: []snapshot.Document{{Key: "srd-2024", Title: "SRD 5.2", RulesetYear: 2024, Precedence: 20, License: "CC-BY-4.0", Attribution: "a", URL: "https://a"}},
		Items:     []snapshot.Item{item("rope", "Rope", 5)},
	}, "fumbles"); err != nil {
		t.Fatal(err)
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Raider", Q: 1})
	ids["Raider"] = token(d.View, "Raider").ID
	fumbles := libraryTable(t, w, "Fumbles", fumbleTable)
	hookOn(t, w, "Critical fumbles", "natural-1", &fumbles, "")
	hookOn(t, w, "A hook to nothing the rules know", "natural-1", nil, "no-such-effect")
	hookOn(t, w, "Overreach", "critical", nil, "prone")
	tb.fight(ids, "Aria", "Raider")

	store := pgstore.New(w.pool)
	fill := func(rollID string, who domain.Member, faces ...int) {
		t.Helper()
		cl := dmCaller
		if !who.DM {
			cl = playerCaller
		}
		for i, f := range faces {
			if _, err := tb.rolls.SetDie(ctx, cl, w.session.CampaignID, domain.RollID(uuid.MustParse(rollID)), i, app.Fill{Value: f}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// open is the Roll Request still waiting whose purpose starts so, if there is one.
	open := func(purpose string) (domain.Roll, bool) {
		t.Helper()
		var id uuid.UUID
		err := w.pool.QueryRow(ctx, "SELECT id FROM play.roll_requests WHERE campaign_id = $1 AND status = 'pending' AND purpose LIKE $2 || '%' ORDER BY created_at DESC, id LIMIT 1", w.session.CampaignID, purpose).Scan(&id)
		if err != nil {
			return domain.Roll{}, false
		}
		roll, err := store.Roll(ctx, w.session.CampaignID, domain.RollID(id))
		if err != nil {
			t.Fatal(err)
		}
		return roll, true
	}
	effects := func(v *live.View, label string) []string {
		out := []string{}
		for _, e := range token(v, label).Effects {
			out = append(out, e.Slug)
		}
		return out
	}
	// attack has a creature attack another and fills the d20 of its attack roll.
	attack := func(sub *live.Subscriber, who domain.Member, attacker, target string, faces ...int) {
		t.Helper()
		cmd := live.Command{Kind: live.CmdAttack, TokenID: ids[attacker], TargetID: ids[target]}
		var v *live.View
		if sub == tb.dm {
			d, _ := tb.dmSays(cmd)
			v = d.View
		} else {
			v = tb.playerSays(cmd).View
		}
		if v == nil || v.Combat == nil || v.Combat.Attack == nil {
			t.Fatalf("%s's attack opened no roll", attacker)
		}
		fill(v.Combat.Attack.RollID, who, faces...)
	}
	endTurn := func(label string) {
		t.Helper()
		v := look(t, w, tb.dm)
		w.hub.Submit(tb.dm, live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, label).ID})
		look(t, w, tb.dm)
		look(t, w, tb.player)
	}

	// Aria rolls a natural 1: she, not the DM, rolls on the Fumbles table, and nothing else is asked of her.
	attack(tb.player, w.player, "Aria", "Raider", 1)
	look(t, w, tb.dm)
	roll, ok := open("Critical fumbles")
	if !ok || roll.Purpose != "Critical fumbles: Fumbles" || roll.Notation != "1d8" || roll.Roller.ID != w.player.ID {
		var kinds []string
		rows, _ := w.pool.Query(ctx, "SELECT kind FROM play.actions WHERE session_id = $1 ORDER BY seq", w.session.ID)
		for rows.Next() {
			var k string
			_ = rows.Scan(&k)
			kinds = append(kinds, k)
		}
		t.Fatalf("the fumble roll = %+v %v after %v", roll.Purpose, ok, kinds)
	}
	if v := look(t, w, tb.player); v.TableResult != nil || len(effects(v, "Aria")) != 0 {
		t.Fatalf("before the roll lands = %+v %v", v.TableResult, effects(v, "Aria"))
	}
	fill(uuid.UUID(roll.ID).String(), w.player, 3)
	for name, sub := range map[string]*live.Subscriber{"the DM": tb.dm, "the party": tb.player} {
		v := look(t, w, sub)
		if r := v.TableResult; r == nil || r.Hook != "Critical fumbles" || r.Table != "Fumbles" || r.Label != "Aria" || r.TokenID != ids["Aria"] || r.Total != 3 || r.Text != "Your weapon slips from your grip." {
			t.Fatalf("%s sees = %+v", name, v.TableResult)
		}
		if got := effects(v, "Aria"); len(got) != 0 {
			t.Fatalf("a result of words alone left %v on Aria", got)
		}
	}
	var fired, rolled int
	var label string
	if err := w.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE a.kind = 'rule_hook_fired'), count(*) FILTER (WHERE a.kind = 'roll_table_rolled'),
		coalesce(max(e.label) FILTER (WHERE a.kind = 'roll_table_rolled'), '')
		FROM play.actions a LEFT JOIN play.action_token_events e ON e.action_id = a.id WHERE a.session_id = $1`, w.session.ID).Scan(&fired, &rolled, &label); err != nil {
		t.Fatal(err)
	}
	if fired != 1 || rolled != 1 || label != "Aria: Your weapon slips from your grip." {
		t.Fatalf("the Action Log: %d fired, %d rolled, %q", fired, rolled, label)
	}
	if _, more := open("Critical fumbles"); more {
		t.Fatal("the fumble roll is still open")
	}

	// The Raider fumbles: the DM rolls for it, and a 1 knocks it Prone. The roll outlives a restart.
	endTurn("Aria")
	attack(tb.dm, w.dm, "Raider", "Aria", 1)
	look(t, w, tb.dm)
	roll, ok = open("Critical fumbles")
	if !ok || roll.Roller.ID != w.dm.ID {
		t.Fatalf("the Raider's fumble roll = %+v %v", roll, ok)
	}
	w.hub.Close(w.session.ID)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	fill(uuid.UUID(roll.ID).String(), w.dm, 1)
	v := look(t, w, tb.player)
	if r := v.TableResult; r == nil || r.Label != "Raider" || r.Text != "You fall flat on your face." || !slices.Equal(effects(v, "Raider"), []string{"prone"}) {
		t.Fatalf("the Raider's fumble = %+v, on it %v", v.TableResult, effects(v, "Raider"))
	}
	// Once the party cannot see the Raider, its result is the DM's alone.
	tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: ids["Raider"], Hidden: true})
	if look(t, w, tb.player).TableResult != nil || look(t, w, tb.dm).TableResult == nil {
		t.Fatal("the result of a creature the party cannot see reached the party, or left the DM")
	}
	tb.dmSays(live.Command{Kind: live.CmdSetHidden, TokenID: ids["Raider"], Hidden: false})
	endTurn("Raider")

	// A natural 20 fires the critical hook and not the fumble one: Overreach knocks Aria Prone at once,
	// with no roll. The Raider is Prone, so she attacks with Advantage.
	attack(tb.player, w.player, "Aria", "Raider", 20, 4)
	if v := look(t, w, tb.dm); !slices.Equal(effects(v, "Aria"), []string{"prone"}) || v.Combat.Attack == nil || !v.Combat.Attack.Critical {
		t.Fatalf("after a natural 20 Aria has %v, and the attack is %+v", effects(v, "Aria"), v.Combat.Attack)
	}
	if _, asked := open("Critical fumbles"); asked {
		t.Fatal("a natural 20 asked for a fumble roll")
	}
	fill(look(t, w, tb.dm).Combat.Attack.RollID, w.player, 1, 1)
	look(t, w, tb.dm)
	endTurn("Aria")

	// A 5 gives two ropes, dropped as loot. Both are Prone now, which cancels out to a plain d20.
	fumble := func(face int) *live.View {
		t.Helper()
		attack(tb.dm, w.dm, "Raider", "Aria", 1)
		look(t, w, tb.dm)
		roll, asked := open("Critical fumbles")
		if !asked {
			t.Fatal("the fumble asked for no roll")
		}
		fill(uuid.UUID(roll.ID).String(), w.dm, face)
		v := look(t, w, tb.dm)
		if v.TableResult == nil || v.TableResult.Total != face {
			t.Fatalf("a %d landed as %+v", face, v.TableResult)
		}
		endTurn("Raider")
		endTurn("Aria")
		return v
	}
	drops := func(v *live.View) int {
		n := 0
		for _, c := range v.Inventory {
			if c.Label == "Fumbles" {
				n++
			}
		}
		return n
	}
	v = fumble(5)
	if drop := containerNamed(t, v, "Fumbles"); v.TableResult.Text != "A rope uncoils from your pack." || drops(v) != 1 || len(drop.Items) != 1 || drop.Items[0].Slug != "rope" || drop.Items[0].Count != 2 {
		t.Fatalf("a 5 = %+v, dropping %+v", v.TableResult, drop)
	}
	// An Item the Campaign does not know is not dropped, though the words are still said; a 7 lands in
	// a gap of the table and nothing happens.
	if v := fumble(6); v.TableResult.Text != "A trinket falls from nowhere." || drops(v) != 1 {
		t.Fatalf("a 6 = %+v, with %d drops", v.TableResult, drops(v))
	}
	if v := fumble(7); v.TableResult.Text != "Nothing happens." || v.TableResult.Table != "Fumbles" || drops(v) != 1 {
		t.Fatalf("a 7 = %+v, with %d drops", v.TableResult, drops(v))
	}
	// When the Items cannot be read nothing is dropped, and the log says why.
	problems := &faults{Handler: slog.NewTextHandler(io.Discard, nil)}
	whole := w.hub.Store
	w.hub.Close(w.session.ID)
	w.hub.Store, w.hub.Log = unreadableItems{Store: whole}, slog.New(problems)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if v := fumble(5); v.TableResult.Text != "A rope uncoils from your pack." || drops(v) != 1 || problems.seen.Load() != 1 {
		t.Fatalf("with the Items unreadable = %+v, %d drops, %d errors logged", v.TableResult, drops(v), problems.seen.Load())
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = whole
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)

	// A roll whose table the Campaign stops seeing before it lands comes to nothing; after that the hook
	// asks for no roll at all.
	attack(tb.dm, w.dm, "Raider", "Aria", 1)
	look(t, w, tb.dm)
	roll, _ = open("Critical fumbles")
	if _, err := w.pool.Exec(ctx, "DELETE FROM library.campaign_links WHERE entry_id = $1", fumbles); err != nil {
		t.Fatal(err)
	}
	fill(uuid.UUID(roll.ID).String(), w.dm, 1)
	if v := look(t, w, tb.dm); v.TableResult == nil || v.TableResult.Text != "Nothing happens." || v.TableResult.Table != "" || v.TableResult.Hook != "Critical fumbles" || len(effects(v, "Raider")) != 1 {
		t.Fatalf("with the table gone = %+v, on the Raider %v", v.TableResult, effects(v, "Raider"))
	}
	endTurn("Raider")
	endTurn("Aria")
	// From here on the runtime's errors are counted afresh: a hook that does nothing is no fault.
	problems = &faults{Handler: slog.NewTextHandler(io.Discard, nil)}
	w.hub.Close(w.session.ID)
	w.hub.Log = slog.New(problems)
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	attack(tb.dm, w.dm, "Raider", "Aria", 1)
	look(t, w, tb.dm)
	if _, asked := open("Critical fumbles"); asked {
		t.Fatal("a hook to a table the Campaign no longer sees asked for a roll")
	}
	if n := problems.seen.Load(); n != 0 {
		t.Fatalf("hooks that do nothing left %d errors in the log", n)
	}
	if !strings.Contains(strings.Join(effects(look(t, w, tb.dm), "Aria"), ","), "prone") {
		t.Fatal("Aria is still Prone from Overreach")
	}
}

// The Campaign's own Rule Variants also hang on a creature dropping to 0 hit points and on a rest
// finished. A roll for a creature nobody controls falls to the DM, whoever's blow set it off. When the
// hooks or the tables cannot be read, play goes on without them.
func TestRuleHooksOnDroppingToZeroAndOnRests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := restingParty(t)
	d, _ := tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Raider", Q: 1})
	raider := token(d.View, "Raider").ID
	words := libraryTable(t, w, "Last words", `{"dice":"1d4","results":[{"from":1,"to":4,"text":"It curses you with its last breath."}]}`)
	hookOn(t, w, "Death throes", "drop-to-0", nil, "poisoned")
	hookOn(t, w, "Last words", "drop-to-0", &words, "")
	hookOn(t, w, "Bad dreams", "rest", nil, "poisoned")
	store := pgstore.New(w.pool)
	waiting := func(purpose string) (domain.Roll, bool) {
		t.Helper()
		var id uuid.UUID
		if err := w.pool.QueryRow(ctx, "SELECT id FROM play.roll_requests WHERE campaign_id = $1 AND status = 'pending' AND purpose LIKE $2 || '%' ORDER BY created_at DESC, id LIMIT 1", w.session.CampaignID, purpose).Scan(&id); err != nil {
			return domain.Roll{}, false
		}
		roll, err := store.Roll(ctx, w.session.CampaignID, domain.RollID(id))
		if err != nil {
			t.Fatal(err)
		}
		return roll, true
	}
	slugs := func(v *live.View, id string) []string {
		out := []string{}
		for _, e := range tokenByID(t, v, id).Effects {
			out = append(out, e.Slug)
		}
		return out
	}

	// A Player's command drops the Raider: its Effect lands at once, and its roll on the table is the DM's.
	hurt := func(sub *live.Subscriber, id string, by int) {
		t.Helper()
		w.hub.Submit(sub, live.Command{Kind: live.CmdAdjustHP, TokenID: id, HPDelta: by})
		look(t, w, tb.dm)
		look(t, w, tb.player)
	}
	hurt(tb.dm, raider, -3)
	if v := look(t, w, tb.dm); len(slugs(v, raider)) != 0 {
		t.Fatalf("a blow that leaves it standing fired a hook: %v", slugs(v, raider))
	}
	hurt(tb.dm, raider, -999)
	roll, ok := waiting("Last words")
	if v := look(t, w, tb.dm); !slices.Contains(slugs(v, raider), "poisoned") || !ok || roll.Purpose != "Last words: Last words" || roll.Roller.ID != w.dm.ID {
		t.Fatalf("the Raider dropping: effects %v, roll %+v %v", slugs(v, raider), roll, ok)
	}
	// The roll is out until it lands: the party cannot split while it is, and can once it has.
	split := func() string {
		t.Helper()
		w.hub.Submit(tb.dm, live.Command{Kind: live.CmdSplitParty, Name: "The tower", TokenIDs: []string{ids["aria-token"]}, MapID: uuid.NewString()})
		return next(t, tb.dm).Reason
	}
	if reason := split(); !strings.Contains(reason, "under way") {
		t.Fatalf("splitting with the roll out = %q", reason)
	}
	if _, err := tb.rolls.SetDie(ctx, dmCaller, w.session.CampaignID, roll.ID, 0, app.Fill{Value: 2}); err != nil {
		t.Fatal(err)
	}
	if v := look(t, w, tb.player); v.TableResult == nil || v.TableResult.Text != "It curses you with its last breath." {
		t.Fatalf("the Raider's last words = %+v", v.TableResult)
	}
	look(t, w, tb.dm)
	if reason := split(); reason == "" || strings.Contains(reason, "under way") {
		t.Fatalf("splitting once the roll has landed = %q", reason)
	}
	// Hurt again at 0, it does not drop a second time.
	hurt(tb.dm, raider, -5)
	if _, again := waiting("Last words"); again {
		t.Fatal("a creature already at 0 dropped again")
	}
	// Aria drops: the roll is her Player's.
	hurt(tb.dm, ids["aria-token"], -*tokenByID(t, look(t, w, tb.dm), ids["aria-token"]).HP)
	if roll, ok := waiting("Last words"); !ok || roll.Roller.ID != w.player.ID || !slices.Contains(slugs(look(t, w, tb.dm), ids["aria-token"]), "poisoned") {
		t.Fatalf("Aria dropping: roll %+v %v", roll, ok)
	}
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["aria-token"], HPDelta: 5})
	for _, e := range tokenByID(t, look(t, w, tb.dm), ids["aria-token"]).Effects {
		if e.Slug == "poisoned" {
			tb.dmSays(live.Command{Kind: live.CmdEndEffect, EffectID: e.ID})
		}
	}

	// A rest finished fires the rest hook for each Character who rested.
	tb.playerSays(live.Command{Kind: live.CmdProposeRest, Rest: live.RestShort})
	tb.dmSays(live.Command{Kind: live.CmdAgreeRest})
	if v := look(t, w, tb.dm); slices.Contains(slugs(v, ids["aria-token"]), "poisoned") {
		t.Fatal("the rest hook fired before the rest finished")
	}
	w.hub.Submit(tb.dm, live.Command{Kind: live.CmdFinishRest})
	v := look(t, w, tb.dm)
	look(t, w, tb.player)
	rested := 0
	for _, tv := range v.Tokens {
		if tv.Kind == domain.TokenParty && slices.ContainsFunc(tv.Effects, func(e live.EffectView) bool { return e.Slug == "poisoned" }) {
			rested++
		}
	}
	if rested != 2 || slices.Contains(slugs(v, raider), "bad-dreams") {
		t.Fatalf("after the rest %d Characters have bad dreams", rested)
	}

	// With the hooks unreadable nothing fires; with the tables unreadable an Effect still lands, no roll
	// is asked for, and a roll already open comes to nothing.
	open, _ := waiting("Last words")
	w.hub.Close(w.session.ID)
	whole := w.hub.Store
	w.hub.Store = unreadableTables{Store: whole}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	if _, err := tb.rolls.SetDie(ctx, playerCaller, w.session.CampaignID, open.ID, 0, app.Fill{Value: 2}); err != nil {
		t.Fatal(err)
	}
	if v := look(t, w, tb.dm); v.TableResult == nil || v.TableResult.Text != "Nothing happens." || v.TableResult.Table != "" || v.TableResult.Hook != "Last words" {
		t.Fatalf("a roll landing with the tables unreadable = %+v", v.TableResult)
	}
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Second", Q: 1, R: 1})
	second := token(d.View, "Second").ID
	hurt(tb.dm, second, -999)
	if _, asked := waiting("Last words"); asked || slices.Contains(slugs(look(t, w, tb.dm), second), "poisoned") {
		t.Fatal("with the tables unreadable a hook still fired")
	}
	w.hub.Close(w.session.ID)
	w.hub.Store = unreadableHooks{Store: whole}
	tb.dm, tb.player = join(t, w, w.dm, dmCaller, live.AudienceDM), join(t, w, w.player, playerCaller, live.AudienceParty)
	d, _ = tb.dmSays(live.Command{Kind: live.CmdPlace, MonsterSlug: "goblin", TokenKind: domain.TokenEnemy, Label: "Third", Q: 2, R: 1})
	third := token(d.View, "Third").ID
	hurt(tb.dm, third, -999)
	if _, asked := waiting("Last words"); asked || len(slugs(look(t, w, tb.dm), third)) != 0 {
		t.Fatal("with the hooks unreadable a hook still fired")
	}
}

// Casting a spell over an area fires the cast hook for its caster. And when a Player's blow drops a
// creature nobody controls, the roll its hook asks for is the DM's to make.
func TestRuleHooksOnCastingAndWhoRolls(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb, ids := magicTable(t)
	words := libraryTable(t, w, "Last words", `{"dice":"1d4","results":[{"from":1,"to":4,"text":"It curses you with its last breath."}]}`)
	hookOn(t, w, "Wild surge", "cast", nil, "poisoned")
	hookOn(t, w, "Last words", "drop-to-0", &words, "")
	tb.fight(ids, "Goblin", "Aria")
	has := func(label, slug string) bool {
		return slices.ContainsFunc(token(look(t, w, tb.dm), label).Effects, func(e live.EffectView) bool { return e.Slug == slug })
	}
	if has("Goblin", "poisoned") {
		t.Fatal("the Goblin is Poisoned before it casts")
	}
	d, _ := tb.dmSays(live.Command{Kind: live.CmdCastArea, TokenID: ids["Goblin"], Effect: "thunderwave", Q: 2, R: 0})
	tb.fill(d.View.Area.DamageRollID, w.dm, 1, 1)
	tb.fill(d.View.Area.Saves[0].RollID, w.player, 20)
	look(t, w, tb.player)
	if !has("Goblin", "poisoned") || has("Aria", "poisoned") {
		t.Fatalf("after the cast: the Goblin Poisoned %v, Aria Poisoned %v", has("Goblin", "poisoned"), has("Aria", "poisoned"))
	}
	// The Goblin is left on 1 hit point; Aria's dart drops it.
	v := look(t, w, tb.dm)
	tb.dmSays(live.Command{Kind: live.CmdAdjustHP, TokenID: ids["Goblin"], HPDelta: 1 - *token(v, "Goblin").HP})
	tb.dmSays(live.Command{Kind: live.CmdEndTurn, CombatantID: combatant(v, "Goblin").ID})
	u := tb.playerSays(live.Command{Kind: live.CmdAttack, TokenID: ids["Aria"], AttackNo: 1, TargetID: ids["Goblin"]})
	if u.View == nil || u.View.Combat.Attack == nil {
		t.Fatalf("the dart = %+v", u)
	}
	tb.fill(u.View.Combat.Attack.RollID, w.player, 19)
	look(t, w, tb.player)
	if a := look(t, w, tb.dm).Combat.Attack; a != nil {
		tb.fill(a.RollID, w.player, 4)
		look(t, w, tb.player)
	}
	if hp := *token(look(t, w, tb.dm), "Goblin").HP; hp != 0 {
		t.Fatalf("the Goblin is on %d", hp)
	}
	var roller uuid.UUID
	if err := w.pool.QueryRow(ctx, "SELECT roller_member_id FROM play.roll_requests WHERE campaign_id = $1 AND purpose = 'Last words: Last words'", w.session.CampaignID).Scan(&roller); err != nil || roller != w.dm.ID {
		t.Fatalf("the Goblin's last words are rolled by %s (the DM is %s, the Player %s): %v", roller, w.dm.ID, w.player.ID, err)
	}
}
