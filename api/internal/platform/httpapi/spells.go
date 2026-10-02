package httpapi

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// GetSpellcasting shows a Character's spells.
func (h *Handler) GetSpellcasting(ctx context.Context, p oas.GetSpellcastingParams) (oas.GetSpellcastingRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	sc, err := h.Characters.Spells(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "spells", err), nil
	}
	return &oas.SpellcastingHeaders{Response: spellcastingOut(sc)}, nil
}

// PrepareSpells sets the spells prepared through a class.
func (h *Handler) PrepareSpells(ctx context.Context, req *oas.SpellPreparation, p oas.PrepareSpellsParams) (oas.PrepareSpellsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	sc, err := h.Characters.Prepare(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), string(req.Class), slugList(req.Spells))
	if err != nil {
		return h.campaignProblem(ctx, "prepare spells", err), nil
	}
	return &oas.SpellcastingHeaders{Response: spellcastingOut(sc)}, nil
}

// CastRitual casts a ritual out of combat.
func (h *Handler) CastRitual(ctx context.Context, req *oas.SpellChoice, p oas.CastRitualParams) (oas.CastRitualRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	r, err := h.Characters.CastRitual(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), string(req.Spell))
	if err != nil {
		return h.campaignProblem(ctx, "cast ritual", err), nil
	}
	return &oas.RitualCastHeaders{Response: oas.RitualCast{Spell: spellOut(r.Spell), Minutes: int32(r.Minutes), Clock: clockOut(r.Clock)}}, nil //nolint:gosec // minutes are bounded
}

// CopySpell writes a spell into a wizard's spellbook.
func (h *Handler) CopySpell(ctx context.Context, req *oas.SpellChoice, p oas.CopySpellParams) (oas.CopySpellRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	sc, err := h.Characters.CopySpell(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), string(req.Spell))
	if err != nil {
		return h.campaignProblem(ctx, "copy spell", err), nil
	}
	return &oas.SpellcastingHeaders{Response: spellcastingOut(sc)}, nil
}

//nolint:gosec // levels, limits and coins are bounded by the rules
func spellcastingOut(sc app.Spellcasting) oas.Spellcasting {
	out := oas.Spellcasting{CanPrepare: sc.CanPrepare, Classes: make([]oas.ClassSpells, 0, len(sc.Classes)), Purse: []oas.LiveCoins{}, Clock: clockOut(sc.Clock)}
	for _, cs := range sc.Classes {
		out.Classes = append(out.Classes, oas.ClassSpells{
			Class: oas.Slug(cs.Class), Name: cs.Name, Level: int32(cs.Level), Limit: int32(cs.Limit), MaxLevel: int32(cs.MaxLevel),
			KeepsSpellbook: cs.Book, Allotment: int32(cs.Allotment), Cantrips: spellsOut(cs.Cantrips), Prepared: spellsOut(cs.Prepared),
			Always: spellsOut(cs.Always), Spellbook: spellsOut(cs.Spellbook), Options: spellsOut(cs.Options), Copyable: spellsOut(cs.Copyable),
		})
	}
	for _, coin := range []string{"pp", "gp", "ep", "sp", "cp"} {
		if n := sc.Purse[coin]; n > 0 {
			out.Purse = append(out.Purse, oas.LiveCoins{Coin: oas.Coin(coin), Count: int32(n)})
		}
	}
	return out
}

func spellsOut(in []compendium.SpellOption) oas.SpellPickList {
	out := make(oas.SpellPickList, 0, len(in))
	for _, sp := range in {
		out = append(out, spellOut(sp))
	}
	return out
}

func spellOut(sp compendium.SpellOption) oas.SpellPick {
	return oas.SpellPick{Slug: oas.Slug(sp.Slug), Name: sp.Name, Level: int32(sp.Level), Ritual: oas.NewOptBool(sp.Ritual)} //nolint:gosec // 0 to 9
}

//nolint:gosec // days and minutes are bounded by their constraints
func clockOut(c domain.Clock) oas.GameClock {
	return oas.GameClock{Day: int32(c.Day), Minute: int32(c.Minute)}
}
