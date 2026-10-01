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

func look(t *testing.T, w world, sub *live.Subscriber) *live.View {
	t.Helper()
	w.hub.Submit(sub, live.Command{Kind: live.CmdResync})
	return next(t, sub).View
}

// Item Instances show beside the old stacks: each with its own name, Charges and state, a bag nested in
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
	for _, c := range []uuid.UUID{bag, sack} {
		if _, err := w.pool.Exec(ctx, `INSERT INTO campaign.container_items (container_id, item_slug, quantity) VALUES ($1, 'rope', 2)`, c); err != nil {
			t.Fatal(err)
		}
	}
	put(aria.ID, "rope", "Climbing Line", 1, 3, true, true, "neck")
	put(aria.ID, "anvil", "Anvil of Storms", 1, 7, false, false, nil)
	put(bag, "rope", nil, 3, nil, true, false, nil)

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
	if packDM.Kind != domain.ContainerBag || packDM.ParentID != ariaDM.ID || len(packDM.Instances) != 1 || packDM.Instances[0].Name != "Rope" || packDM.Instances[0].Count != 3 {
		t.Fatalf("the backpack = %+v", packDM)
	}
	if ariaDM.WeightLb != 5+100+15+10 {
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
	if pack := containerNamed(t, moved.View, "Backpack"); len(pack.Items) != 1 || pack.Items[0].Count != 1 {
		t.Fatalf("taking from her own backpack = %+v", pack)
	}
	for _, c := range tv.Inventory {
		if c.Kind != domain.ContainerStash {
			t.Fatalf("the Table sees the stash only, not bags in Inventories = %+v", tv.Inventory)
		}
	}
}
