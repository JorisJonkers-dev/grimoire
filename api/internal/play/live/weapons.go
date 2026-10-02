package live

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// WeaponSwap is a Character taking up its other weapon set, and what it then wears and holds.
type WeaponSwap struct {
	Character uuid.UUID
	Set       string
	Armor     string
	Shield    bool
	Weapons   []string
	economy   *combat.Economy
}

// planSwapWeapons puts a Character's weapons away and draws its other set; in a fight each weapon pays
// the equip rules, and a shield takes the Utilize action.
func (r *runtime) planSwapWeapons(m domain.Member, c caller.Caller, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	}
	id, ok := characterOf(t)
	inv := r.st.inventory
	mine, b, carries := inv.Carrier(id)
	if !ok || !carries {
		return Write{}, t.Label + " has no weapon sets."
	}
	set := inventory.OtherSet(b.WeaponSet)
	armor, shield, weapons := inv.Held(mine, set)
	_, held, stowed := inv.Held(mine, b.WeaponSet)
	swap := &WeaponSwap{Character: id, Set: set, Armor: armor, Shield: shield, Weapons: weapons}
	w := Write{
		Kind: domain.ActionObjectUsed, Swap: swap,
		manuals: []domain.ManualPrompt{{ID: uuid.New(), Text: t.Label + " takes up their " + set + " weapons."}},
	}
	if fight := r.st.combat; fight != nil && fight.Status == domain.CombatActive {
		x, acting := r.st.combatantOf(t.ID)
		if !acting {
			return Write{}, "It is not " + t.Label + "'s turn."
		}
		e, paid := x.Economy.Swap(len(weapons)+len(stowed), shield || held)
		if !paid {
			return Write{}, t.Label + " has no time left this turn to change weapons."
		}
		w.Combatant, swap.economy = x.ID, &e
	}
	stats, err := r.stats.Holding(context.Background(), c, r.st.session.CampaignID, id, weapons, shield)
	if err != nil {
		return Write{}, "That Character's weapons could not be read."
	}
	next := *t.Stats
	next.Attacks, next.AC = stats.Attacks, stats.AC
	t.Stats = &next
	w.Token = t
	return w, ""
}

// applySwap gives the token its new attacks and Armor Class, records the set held, and spends the turn's
// economy.
func applySwap(s *state, w *Write) {
	s.tokens[w.Token.ID] = w.Token
	if i := slices.IndexFunc(s.inventory.Bearers, func(b domain.Bearer) bool { return b.CharacterID == w.Swap.Character }); i >= 0 {
		s.inventory.Bearers = slices.Clone(s.inventory.Bearers)
		s.inventory.Bearers[i].WeaponSet = w.Swap.Set
	}
	if w.Swap.economy == nil {
		return
	}
	i := slices.IndexFunc(s.combat.Combatants, func(x domain.Combatant) bool { return x.ID == w.Combatant })
	s.combat.Combatants[i].Economy = *w.Swap.economy
	w.Combat = s.combat
}
