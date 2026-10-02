package live_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
)

func instanceNamed(t *testing.T, c live.ContainerView, name string) live.InstanceView {
	t.Helper()
	for _, in := range c.Instances {
		if in.Name == name {
			return in
		}
	}
	t.Fatalf("no %s in %+v", name, c.Instances)
	return live.InstanceView{}
}

// look resyncs a subscriber and returns its snapshot, skipping updates still queued before it.
func look(t *testing.T, w world, sub *live.Subscriber) *live.View {
	t.Helper()
	w.hub.Submit(sub, live.Command{Kind: live.CmdResync})
	for {
		if u := next(t, sub); u.Kind == live.UpdSnapshot {
			return u.View
		}
	}
}

// Item Instances show beside plain stacks: each with its own name, Charges and state, a bag nested in
// its bearer's Inventory, and an unidentified item kept a mystery from the party.
func TestItemInstancesShowBesideStacks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	stocked(t, w)
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	aria := containerNamed(t, look(t, w, tb.dm), "Aria")
	w.hub.Close(w.session.ID)
	bag := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, parent_id, label, created_at) VALUES ($1, $2, 'bag', $3, 'Backpack', $4)`,
		bag, w.session.CampaignID, aria.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	put := func(container any, slug string, name any, qty int, charges any, identified, attuned bool, slot any) {
		t.Helper()
		if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, charges, identified, attuned, equipped_slot, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())`, uuid.New(), container, slug, name, qty, charges, identified, attuned, slot); err != nil {
			t.Fatal(err)
		}
	}
	brom := containerNamed(t, look(t, w, join(t, w, w.dm, dmCaller, live.AudienceDM)), "Brom")
	w.hub.Close(w.session.ID)
	sack := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, parent_id, label, created_at) VALUES ($1, $2, 'bag', $3, 'Sack', $4)`,
		sack, w.session.CampaignID, brom.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	put(aria.ID, "rope", "Climbing Line", 1, 3, true, true, "neck")
	put(aria.ID, "anvil", "Anvil of Storms", 1, 7, false, false, nil)
	put(bag, "rope", nil, 3, nil, true, false, nil)
	put(sack, "rope", nil, 2, nil, true, false, nil)

	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	table := join(t, w, w.player, playerCaller, live.AudienceTable)
	d, p, tv := look(t, w, tb.dm), look(t, w, tb.player), look(t, w, table)

	ariaDM, packDM := containerNamed(t, d, "Aria"), containerNamed(t, d, "Backpack")
	line := instanceNamed(t, ariaDM, "Climbing Line")
	if line.Slug != "rope" || line.Count != 1 || line.Charges == nil || *line.Charges != 3 || !line.Attuned || !line.Identified || line.Slot != "neck" || line.WeightLb != 5 {
		t.Fatalf("the named rope = %+v", line)
	}
	if storms := instanceNamed(t, ariaDM, "Anvil of Storms"); storms.Identified || storms.Charges == nil {
		t.Fatalf("the DM sees an unidentified item for what it is = %+v", storms)
	}
	if packDM.Kind != domain.ContainerBag || packDM.ParentID != ariaDM.ID || len(packDM.Instances) != 0 || len(packDM.Items) != 1 || packDM.Items[0].Count != 3 {
		t.Fatalf("the backpack = %+v", packDM)
	}
	if ariaDM.WeightLb != 5+100+15 {
		t.Fatalf("Aria carries her backpack too = %v", ariaDM.WeightLb)
	}
	if mystery := instanceNamed(t, containerNamed(t, p, "Aria"), "Anvil"); mystery.Identified || mystery.Charges != nil {
		t.Fatalf("the party sees only an anvil = %+v", mystery)
	}
	if pack := containerNamed(t, p, "Backpack"); pack.OwnerID != w.player.ID.String() || pack.CharacterID != "" {
		t.Fatalf("a bag names its bearer's owner = %+v", pack)
	}
	stash := containerNamed(t, p, "Party Stash").ID
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdMoveItem, FromID: containerNamed(t, p, "Sack").ID, ToID: stash, ItemSlug: "rope", Count: 1})
	if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "not yours") {
		t.Fatalf("taking from a bag in Brom's pack = %+v", u)
	}
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdMoveItem, FromID: containerNamed(t, p, "Backpack").ID, ToID: containerNamed(t, p, "Sack").ID, ItemSlug: "rope", Count: 1})
	if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "not yours") {
		t.Fatalf("putting into Brom's sack = %+v", u)
	}
	moved := tb.playerSays(live.Command{Kind: live.CmdMoveItem, FromID: containerNamed(t, p, "Backpack").ID, ToID: stash, ItemSlug: "rope", Count: 1})
	if pack := containerNamed(t, moved.View, "Backpack"); len(pack.Items) != 1 || pack.Items[0].Count != 2 {
		t.Fatalf("taking from her own backpack = %+v", pack)
	}
	w.hub.Submit(tb.player, live.Command{Kind: live.CmdMoveItem, FromID: ariaDM.ID, ToID: stash, InstanceID: uuid.NewString()})
	if u := next(t, tb.player); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, "no such item") {
		t.Fatalf("moving an instance that is not there = %+v", u)
	}
	moved = tb.playerSays(live.Command{Kind: live.CmdMoveItem, FromID: ariaDM.ID, ToID: packDM.ID, InstanceID: line.ID})
	if got := instanceNamed(t, containerNamed(t, moved.View, "Backpack"), "Climbing Line"); got.Slot != "" || !got.Attuned || got.Charges == nil {
		t.Fatalf("the rope comes off her neck into the pack, still attuned = %+v", got)
	}
	if len(containerNamed(t, moved.View, "Aria").Instances) != 1 {
		t.Fatalf("Aria keeps only the anvil = %+v", containerNamed(t, moved.View, "Aria"))
	}
	w.hub.Close(w.session.ID)
	again := look(t, w, join(t, w, w.dm, dmCaller, live.AudienceDM))
	if got := instanceNamed(t, containerNamed(t, again, "Backpack"), "Climbing Line"); got.Slot != "" || containerNamed(t, again, "Backpack").Items[0].Count != 2 {
		t.Fatalf("the move is stored = %+v", containerNamed(t, again, "Backpack"))
	}
	for _, c := range tv.Inventory {
		if c.Kind != domain.ContainerStash {
			t.Fatalf("the Table sees the stash only, not bags in Inventories = %+v", tv.Inventory)
		}
	}
}

// A Character's Inventory is private to its owner and the DM: another player sees whose pack it is and
// what it weighs, never what is in it.
func TestAnotherPlayerCannotReadAPrivateInventory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	stocked(t, w)
	brom := containerNamed(t, look(t, w, join(t, w, w.dm, dmCaller, live.AudienceDM)), "Brom")
	w.hub.Close(w.session.ID)
	sack := uuid.New()
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.containers (id, campaign_id, kind, parent_id, label, created_at) VALUES ($1, $2, 'bag', $3, 'Sack', now())`,
		sack, w.session.CampaignID, brom.ID); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{brom.ID, sack.String()} {
		if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at)
			VALUES ($1, $2, 'rope', 'Secret Rope', 1, true, false, now())`, uuid.New(), c); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.container_coins (container_id, coin, amount) VALUES ($1, 'gp', 7)`, brom.ID); err != nil {
		t.Fatal(err)
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	if c := containerNamed(t, look(t, w, tb.dm), "Brom"); len(c.Instances) != 1 || len(c.Coins) != 1 {
		t.Fatalf("the DM sees into Brom's pack = %+v", c)
	}
	views := []*live.View{look(t, w, tb.player)}
	d, p := tb.dmSays(live.Command{Kind: live.CmdPlace, Label: "Goblin", TokenKind: domain.TokenEnemy})
	views = append(views, p.View)
	if c := containerNamed(t, d.View, "Sack"); len(c.Instances) != 1 {
		t.Fatalf("the DM sees into Brom's sack = %+v", c)
	}
	for _, v := range views {
		for _, label := range []string{"Brom", "Sack"} {
			c := containerNamed(t, v, label)
			if len(c.Instances)+len(c.Items)+len(c.Coins) != 0 || c.WeightLb == 0 || c.OwnerID != w.dm.ID.String() {
				t.Fatalf("Aria's player reads %s = %+v", label, c)
			}
		}
		if containerNamed(t, v, "Aria").Items == nil {
			t.Fatal("her own pack is open to her")
		}
	}
}
