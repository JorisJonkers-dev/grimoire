package live

import (
	"cmp"
	"maps"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/loot"
)

// Claim options besides need and greed: pass takes a claim back.
const claimPass = "pass"

// planClaim records a Character's need or greed call on an item in a loot pile, rolling its d20 the
// first time; pass takes the call back.
func (r *runtime) planClaim(m domain.Member, cmd Command) (Write, string) {
	pile, ok := r.st.container(cmd.FromID)
	if !ok || pile.Kind != domain.ContainerDrop {
		return Write{}, "No such loot pile."
	}
	item, ok := pileItem(pile, cmd)
	if !ok {
		return Write{}, "There is no such item there."
	}
	character, err := uuid.Parse(cmd.CharacterID)
	_, b, carries := r.st.inventory.Carrier(character)
	switch {
	case err != nil || !carries:
		return Write{}, "No such character."
	case !m.DM && b.Owner != m.ID:
		return Write{}, "That character is not yours to claim for."
	case cmd.Option != loot.Need && cmd.Option != loot.Greed && cmd.Option != claimPass:
		return Write{}, "Claim it with need or greed, or pass."
	}
	c := domain.Claim{Container: pile.ID, Character: character, Item: item, Choice: cmd.Option, At: r.now()}
	if i := r.st.claimIndex(c); i >= 0 {
		c.Roll, c.At = r.st.inventory.Claims[i].Roll, r.st.inventory.Claims[i].At
	} else {
		c.Roll = dice.Face(r.source(r.seed()), 20)
	}
	return Write{Kind: domain.ActionLootClaimed, Claim: &c, Unclaim: cmd.Option == claimPass}, ""
}

// pileItem is the claimed item's key: a plain stack's slug or an Item Instance's id.
func pileItem(pile domain.Container, cmd Command) (string, bool) {
	if cmd.InstanceID != "" {
		id := domain.InstanceID(parseID(cmd.InstanceID))
		return uuid.UUID(id).String(), slices.ContainsFunc(pile.Instances, func(in domain.Instance) bool { return in.ID == id })
	}
	return cmd.ItemSlug, pile.Items[cmd.ItemSlug] > 0
}

func (s *state) claimIndex(c domain.Claim) int {
	return slices.IndexFunc(s.inventory.Claims, func(x domain.Claim) bool {
		return x.Container == c.Container && x.Character == c.Character && x.Item == c.Item
	})
}

// planSettle shares a loot pile out: each item to its claimants, need before greed and then by roll,
// a stack dealt one at a time; coins split evenly between the party; whatever nobody claimed, and the
// copper that will not split, goes to the Party Stash.
func (r *runtime) planSettle(cmd Command) (Write, string) {
	pile, ok := r.st.container(cmd.FromID)
	if !ok || pile.Kind != domain.ContainerDrop {
		return Write{}, "No such loot pile."
	}
	inv := r.st.inventory
	st := settling{pile: pile, stash: inv.Containers[slices.IndexFunc(inv.Containers, func(c domain.Container) bool { return c.Kind == domain.ContainerStash })]}
	for _, slug := range slices.Sorted(maps.Keys(pile.Items)) {
		st.stack(r.st, slug)
	}
	for _, in := range pile.Instances {
		id := in.ID
		to := st.stash
		if ranked := loot.Rank(r.st.pileClaims(pile.ID, uuid.UUID(id).String())); len(ranked) > 0 {
			to = r.st.carried(ranked[0].Claimant)
		}
		st.give(to, domain.Move{Instance: &id, Item: in.Slug, Count: in.Quantity})
	}
	st.coins(r.st.sharers())
	return Write{Kind: domain.ActionLootSettled, Moves: st.moves, Gone: &pile.ID}, ""
}

// settling gathers the transfers that share a loot pile out.
type settling struct {
	pile, stash domain.Container
	moves       []domain.Move
}

func (st *settling) give(to domain.Container, mv domain.Move) {
	mv.From, mv.To, mv.FromLabel, mv.ToLabel = st.pile.ID, to.ID, st.pile.Label, to.Label
	st.moves = append(st.moves, mv)
}

// stack deals a plain stack to its ranked claimants; what is left goes to the stash.
func (st *settling) stack(s *state, slug string) {
	left := st.pile.Items[slug]
	ranked := loot.Rank(s.pileClaims(st.pile.ID, slug))
	shares := loot.Share(left, ranked)
	for _, c := range ranked {
		if k := shares[c.Claimant]; k > 0 {
			shares[c.Claimant], left = 0, left-k
			st.give(s.carried(c.Claimant), domain.Move{Item: slug, Count: k})
		}
	}
	if left > 0 {
		st.give(st.stash, domain.Move{Item: slug, Count: left})
	}
}

// coins splits the pile's coins between the party, the dearest coin first; the rest goes to the stash.
func (st *settling) coins(party []domain.Container) {
	each, left := loot.Split(st.pile.Coins, len(party))
	for _, c := range party {
		for _, coin := range slices.Backward(loot.Coins()) {
			if n := each[coin]; n > 0 {
				st.give(c, domain.Move{Coin: coin, Count: n})
			}
		}
	}
	for _, coin := range loot.Coins() {
		if n := left[coin]; n > 0 {
			st.give(st.stash, domain.Move{Coin: coin, Count: n})
		}
	}
}

// pileClaims are the claims on one item of a loot pile, as the rules rank them: Order is how early each came.
func (s *state) pileClaims(pile domain.ContainerID, item string) []loot.Claim {
	var out []loot.Claim
	for i, c := range s.inventory.Claims {
		if c.Container == pile && c.Item == item {
			out = append(out, loot.Claim{Claimant: c.Character.String(), Choice: c.Choice, Roll: c.Roll, Order: i})
		}
	}
	return out
}

// carried is a Character's own Container.
func (s *state) carried(character string) domain.Container {
	c, _, _ := s.inventory.Carrier(uuid.MustParse(character))
	return c
}

// sharers are who share a pile's coins: the Characters with a token on the board, or every Character
// when none has one; in name order.
func (s *state) sharers() []domain.Container {
	var out []domain.Container
	for _, t := range s.tokens {
		if id, ok := characterOf(t); ok {
			if c, _, carries := s.inventory.Carrier(id); carries {
				out = append(out, c)
			}
		}
	}
	if len(out) == 0 {
		for _, b := range s.inventory.Bearers {
			c, _, _ := s.inventory.Carrier(b.CharacterID)
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b domain.Container) int {
		return cmp.Or(cmp.Compare(a.Label, b.Label), cmp.Compare(uuid.UUID(a.ID).String(), uuid.UUID(b.ID).String()))
	})
	return slices.CompactFunc(out, func(a, b domain.Container) bool { return a.ID == b.ID })
}

// applyClaims keeps a claim, or settles a pile: its transfers, then the pile and its claims are gone.
func applyClaims(s *state, w *Write) {
	inv := &s.inventory
	if c := w.Claim; c != nil {
		inv.Claims = slices.Clone(inv.Claims)
		switch i := s.claimIndex(*c); {
		case w.Unclaim && i >= 0:
			inv.Claims = slices.Delete(inv.Claims, i, i+1)
		case w.Unclaim:
		case i >= 0:
			inv.Claims[i].Choice = c.Choice
		default:
			inv.Claims = append(inv.Claims, *c)
		}
		return
	}
	for i := range w.Moves {
		moveOne(inv, &w.Moves[i])
	}
	inv.Containers = slices.DeleteFunc(inv.Containers, func(c domain.Container) bool { return c.ID == *w.Gone })
	inv.Claims = slices.DeleteFunc(slices.Clone(inv.Claims), func(c domain.Claim) bool { return c.Container == *w.Gone })
}
