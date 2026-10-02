package live_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func countOf(c live.ContainerView, slug string) int {
	for _, it := range c.Items {
		if it.Slug == slug {
			return it.Count
		}
	}
	return 0
}

func coinsOf(c live.ContainerView) map[string]int {
	out := map[string]int{}
	for _, k := range c.Coins {
		out[k.Coin] = k.Count
	}
	return out
}

// Two players claim from a loot pile: need beats greed, the higher roll takes more of a stack, a pass
// takes a claim back; the DM settles it, the coins split exactly and the rest goes to the Party Stash.
func TestTwoPlayersClaimALootPile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, tb := ambushTable(t)
	stocked(t, w)
	second := caller.UI("player-2")
	camp := campaignapp.NewService(campaignpg.New(w.pool))
	invite, err := camp.CreateInvite(ctx, dmCaller, campaigndomain.CampaignID(w.session.CampaignID))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := camp.AcceptInvite(ctx, second, invite.Token, "Bea"); err != nil {
		t.Fatal(err)
	}
	bea, err := pgstore.CampaignMembers{Store: campaignpg.New(w.pool)}.Membership(ctx, w.session.CampaignID, second.Subject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.pool.Exec(ctx, "UPDATE campaign.characters SET owner_member_id = $1 WHERE campaign_id = $2 AND name = 'Brom'", bea.ID, w.session.CampaignID); err != nil {
		t.Fatal(err)
	}
	pile, blade := uuid.New(), uuid.New()
	for _, q := range []string{
		`INSERT INTO campaign.containers (id, campaign_id, kind, label, created_at) VALUES ($1, $2, 'loot_drop', 'Loot: Ogre', now())`,
		`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at) VALUES (gen_random_uuid(), $1, 'anvil', 3, true, false, now())`,
		`INSERT INTO campaign.item_instances (id, container_id, item_slug, quantity, identified, attuned, created_at) VALUES (gen_random_uuid(), $1, 'rope', 1, true, false, now())`,
		`INSERT INTO campaign.item_instances (id, container_id, item_slug, custom_name, quantity, identified, attuned, created_at) VALUES ($2, $1, 'rope', 'Moonrope', 1, true, false, now())`,
		`INSERT INTO campaign.container_coins (container_id, coin, amount) VALUES ($1, 'gp', 10), ($1, 'cp', 1)`,
	} {
		args := []any{pile}
		switch {
		case strings.Contains(q, "Moonrope"):
			args = append(args, blade)
		case strings.Contains(q, "$2"):
			args = append(args, w.session.CampaignID)
		}
		if _, err := w.pool.Exec(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	tb.dm = join(t, w, w.dm, dmCaller, live.AudienceDM)
	tb.player = join(t, w, w.player, playerCaller, live.AudienceParty)
	other := join(t, w, bea, second, live.AudienceParty)
	subs := []*live.Subscriber{tb.dm, tb.player, other}
	say := func(sub *live.Subscriber, cmd live.Command) live.Update {
		t.Helper()
		w.hub.Submit(sub, cmd)
		u := next(t, sub)
		if u.Kind == live.UpdView {
			for _, s := range subs {
				if s != sub {
					next(t, s)
				}
			}
		}
		return u
	}
	refuse := func(sub *live.Subscriber, want string, cmd live.Command) {
		t.Helper()
		if u := say(sub, cmd); u.Kind != live.UpdRejected || !strings.Contains(u.Reason, want) {
			t.Fatalf("%s = %+v", want, u)
		}
	}
	v := look(t, w, tb.dm)
	aria, brom := containerNamed(t, v, "Aria").CharacterID, containerNamed(t, v, "Brom").CharacterID
	for _, id := range []string{aria, brom} {
		say(tb.dm, live.Command{Kind: live.CmdPlace, CharacterID: id})
	}
	from := pile.String()
	claim := func(sub *live.Subscriber, character, choice string, item ...string) live.Update {
		t.Helper()
		cmd := live.Command{Kind: live.CmdClaimLoot, FromID: from, CharacterID: character, Option: choice, ItemSlug: item[0]}
		if item[0] == "" {
			cmd.InstanceID = blade.String()
		}
		u := say(sub, cmd)
		if u.View == nil {
			t.Fatalf("%s claims %v: %+v", character, item, u)
		}
		return u
	}

	refuse(tb.player, "No such loot pile", live.Command{Kind: live.CmdClaimLoot, FromID: uuid.NewString(), CharacterID: aria, Option: "need", ItemSlug: "anvil"})
	refuse(tb.player, "no such item", live.Command{Kind: live.CmdClaimLoot, FromID: from, CharacterID: aria, Option: "need", ItemSlug: "sword"})
	refuse(tb.player, "no such item", live.Command{Kind: live.CmdClaimLoot, FromID: from, CharacterID: aria, Option: "need", InstanceID: uuid.NewString()})
	refuse(tb.player, "No such character", live.Command{Kind: live.CmdClaimLoot, FromID: from, CharacterID: uuid.NewString(), Option: "need", ItemSlug: "anvil"})
	refuse(tb.player, "not yours to claim for", live.Command{Kind: live.CmdClaimLoot, FromID: from, CharacterID: brom, Option: "need", ItemSlug: "anvil"})
	refuse(tb.player, "need or greed", live.Command{Kind: live.CmdClaimLoot, FromID: from, CharacterID: aria, Option: "want", ItemSlug: "anvil"})
	refuse(tb.player, "Only the DM", live.Command{Kind: live.CmdSettleLoot, FromID: from})
	refuse(tb.dm, "No such loot pile", live.Command{Kind: live.CmdSettleLoot, FromID: uuid.NewString()})

	claim(tb.player, aria, "greed", "anvil")
	claim(tb.player, aria, "need", "anvil")
	claim(other, brom, "need", "anvil")
	claim(tb.player, aria, "greed", "")
	claim(other, brom, "need", "")
	claim(other, brom, "greed", "rope")
	u := claim(other, brom, "pass", "rope")
	claims := containerNamed(t, u.View, "Loot: Ogre").Claims
	rolls := map[string]int{}
	for _, c := range claims {
		if c.Item == "anvil" {
			rolls[c.Name] = c.Roll
		}
		if c.Item == "rope" {
			t.Fatalf("a pass takes the claim back = %+v", claims)
		}
	}
	if len(claims) != 4 || claims[0].Name != "Aria" || claims[0].Choice != "need" || rolls["Aria"] < 1 || rolls["Brom"] < 1 {
		t.Fatalf("claims, earliest first, keeping the first roll = %+v", claims)
	}
	more, fewer := "Aria", "Brom"
	if rolls["Brom"] > rolls["Aria"] {
		more, fewer = fewer, more
	}

	d := say(tb.dm, live.Command{Kind: live.CmdSettleLoot, FromID: from})
	if d.View == nil {
		t.Fatalf("settle = %+v", d)
	}
	for _, c := range d.View.Inventory {
		if c.Kind == "loot_drop" {
			t.Fatalf("the pile is gone = %+v", c)
		}
	}
	check := func(v *live.View) {
		t.Helper()
		got := map[string]live.ContainerView{"Aria": containerNamed(t, v, "Aria"), "Brom": containerNamed(t, v, "Brom"), "Stash": containerNamed(t, v, "Party Stash")}
		if countOf(got[more], "anvil") != 2 || countOf(got[fewer], "anvil") != 1 {
			t.Fatalf("both need the anvils; %s rolled higher and takes two = %+v", more, got)
		}
		if instanceNamed(t, got["Brom"], "Moonrope").Slug != "rope" {
			t.Fatalf("need beats greed for the Moonrope = %+v", got["Brom"])
		}
		for _, name := range []string{"Aria", "Brom"} {
			if coins := coinsOf(got[name]); coins["gp"] != 5 || coins["cp"] != 0 {
				t.Fatalf("%s's share of the coins = %v", name, coins)
			}
		}
		if countOf(got["Stash"], "rope") != 1 || coinsOf(got["Stash"])["cp"] != 1 {
			t.Fatalf("the unclaimed rope and the copper that will not split go to the stash = %+v", got["Stash"])
		}
	}
	check(d.View)
	w.hub.Close(w.session.ID)
	check(look(t, w, join(t, w, w.dm, dmCaller, live.AudienceDM)))
	var claimsLeft int
	if err := w.pool.QueryRow(ctx, "SELECT count(*) FROM campaign.loot_claims").Scan(&claimsLeft); err != nil || claimsLeft != 0 {
		t.Fatalf("the claims go with the pile = %d %v", claimsLeft, err)
	}
}
