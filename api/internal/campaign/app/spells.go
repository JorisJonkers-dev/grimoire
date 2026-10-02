package app

import (
	"context"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/shops"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// ClassSpells is what a Character casts through one class: the cantrips it knows, the spells it has
// prepared against its limit, those always prepared, and what it may prepare from. A wizard prepares
// from its spellbook and copies new spells into it, free up to its allotment.
type ClassSpells struct {
	Class     string
	Name      string
	Level     int
	Limit     int
	MaxLevel  int
	Book      bool
	Allotment int
	Cantrips  []compendium.SpellOption
	Prepared  []compendium.SpellOption
	Always    []compendium.SpellOption
	Spellbook []compendium.SpellOption
	Options   []compendium.SpellOption
	// Copyable are the wizard spells it could copy into its spellbook.
	Copyable []compendium.SpellOption
}

// Spellcasting is a Character's spells in every class it casts through, whether it may change what it
// has prepared now, its coins and the Campaign's Game Clock.
type Spellcasting struct {
	CanPrepare bool
	Classes    []ClassSpells
	Purse      map[string]int
	Clock      domain.Clock
}

// Ritual is a ritual cast: the spell, how long it took, and the Game Clock after it.
type Ritual struct {
	Spell   compendium.SpellOption
	Minutes int
	Clock   domain.Clock
}

// Spells shows a Character's spellcasting. The owner or a DM, out of combat.
func (s *Characters) Spells(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) (Spellcasting, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return Spellcasting{}, err
	}
	return s.spellcasting(ctx, sheet)
}

func (s *Characters) spellcasting(ctx context.Context, sheet Sheet) (Spellcasting, error) {
	out := Spellcasting{CanPrepare: sheet.CanPrepare, Classes: []ClassSpells{}}
	for _, x := range sheet.Classes {
		if rules.CasterFor(x.Class) == rules.NoCaster {
			continue
		}
		cs, err := s.classSpells(ctx, sheet, x)
		if err != nil {
			return Spellcasting{}, err
		}
		out.Classes = append(out.Classes, cs)
	}
	purse, err := s.Repo.Purse(ctx, sheet.ID)
	if err != nil {
		return Spellcasting{}, err
	}
	out.Purse = purse.Coins
	out.Clock, err = s.Repo.Clock(ctx, sheet.CampaignID)
	return out, err
}

func (s *Characters) classSpells(ctx context.Context, sheet Sheet, x domain.ClassLevel) (ClassSpells, error) {
	maxLevel := rules.MaxSpellLevel(x.Class, x.Level)
	list, err := s.Compendium.ClassSpells(ctx, sheet.Ruleset, x.Class, maxLevel)
	if err != nil {
		return ClassSpells{}, err
	}
	always, err := s.Compendium.AlwaysPrepared(ctx, sheet.Ruleset, x.Class, x.Subclass, x.Level)
	if err != nil {
		return ClassSpells{}, err
	}
	cs := ClassSpells{
		Class: x.Class, Name: sheet.ClassNames[x.Class], Level: x.Level, Limit: rules.PreparedSpells(x.Class, x.Level), MaxLevel: maxLevel,
		Book: rules.KeepsSpellbook(x.Class), Allotment: rules.SpellbookAllotment(x.Level), Always: always,
	}
	for _, sp := range list {
		cs.sort(sp, sheet.Spells, always)
	}
	if !cs.Book {
		cs.Allotment = 0
	}
	return cs, nil
}

// sort files one spell of the class's list: a known cantrip, a prepared spell, a spellbook entry, and
// whether it can be prepared (from the spellbook for a wizard, the whole list otherwise).
func (cs *ClassSpells) sort(sp compendium.SpellOption, learned []domain.LearnedSpell, always []compendium.SpellOption) {
	l, ok := find(learned, func(l domain.LearnedSpell) bool { return l.Class == cs.Class && l.Spell == sp.Slug })
	switch {
	case ok && sp.Level == 0:
		cs.Cantrips = append(cs.Cantrips, sp)
	case ok && l.Prepared:
		cs.Prepared = append(cs.Prepared, sp)
	}
	inBook := ok && l.Spellbook
	if inBook {
		cs.Spellbook = append(cs.Spellbook, sp)
	} else if cs.Book && sp.Level > 0 {
		cs.Copyable = append(cs.Copyable, sp)
	}
	alwaysPrepared := slices.ContainsFunc(always, func(a compendium.SpellOption) bool { return a.Slug == sp.Slug })
	if sp.Level > 0 && (!cs.Book || inBook) && !alwaysPrepared {
		cs.Options = append(cs.Options, sp)
	}
}

// Prepare sets the spells a Character has prepared through a class, from what the class may prepare,
// within its limit; it may change them after a long rest or a new level.
func (s *Characters) Prepare(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, class string, spells []string) (Spellcasting, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return Spellcasting{}, err
	}
	cs, err := s.casterClass(ctx, sheet, class)
	if err != nil {
		return Spellcasting{}, err
	}
	if !sheet.CanPrepare {
		return Spellcasting{}, refuse("prepare spells again after a long rest or a new level")
	}
	previous := make([]string, 0, len(cs.Prepared))
	for _, sp := range cs.Prepared {
		previous = append(previous, sp.Slug)
	}
	if err := rules.CheckPreparation(class, cs.Limit, previous, spells); err != nil {
		return Spellcasting{}, invalid(err)
	}
	for _, slug := range spells {
		if !slices.ContainsFunc(cs.Options, func(o compendium.SpellOption) bool { return o.Slug == slug }) {
			return Spellcasting{}, refuse("prepare spells from your class's list up to level " + strconv.Itoa(cs.MaxLevel))
		}
	}
	rows := preparedRows(sheet, cs, spells)
	if err := s.Repo.InTx(ctx, func(r Repository) error { return r.ReplaceClassSpells(ctx, ch, class, rows, false) }); err != nil {
		return Spellcasting{}, err
	}
	return s.Spells(ctx, c, id, ch)
}

// preparedRows are a class's learned spells once a new list is prepared: its cantrips, its spellbook
// with the new list marked prepared, or for other classes just the new list.
func preparedRows(sheet Sheet, cs ClassSpells, spells []string) []domain.LearnedSpell {
	var rows []domain.LearnedSpell
	for _, l := range sheet.Spells {
		if l.Class != cs.Class {
			continue
		}
		cantrip := slices.ContainsFunc(cs.Cantrips, func(o compendium.SpellOption) bool { return o.Slug == l.Spell })
		if cantrip || l.Spellbook {
			l.Prepared = cantrip || slices.Contains(spells, l.Spell)
			rows = append(rows, l)
		}
	}
	for _, slug := range spells {
		if !slices.ContainsFunc(rows, func(l domain.LearnedSpell) bool { return l.Spell == slug }) {
			rows = append(rows, domain.LearnedSpell{Class: cs.Class, Spell: slug, Level: sheet.Level, Prepared: true, Spellbook: false})
		}
	}
	return rows
}

func (s *Characters) casterClass(ctx context.Context, sheet Sheet, class string) (ClassSpells, error) {
	x, ok := find(sheet.Classes, func(x domain.ClassLevel) bool { return x.Class == class })
	if !ok || rules.CasterFor(class) == rules.NoCaster {
		return ClassSpells{}, refuse("this Character casts no spells as " + class)
	}
	return s.classSpells(ctx, sheet, x)
}

// CastRitual casts a ritual spell out of combat: no slot, and its casting time plus 10 minutes on the
// Game Clock. It must be prepared, always prepared, or for a wizard in its spellbook.
func (s *Characters) CastRitual(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, spell string) (Ritual, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return Ritual{}, err
	}
	sc, err := s.spellcasting(ctx, sheet)
	if err != nil {
		return Ritual{}, err
	}
	sp, ok := castable(sc, spell)
	if !ok {
		return Ritual{}, refuse("cast a ritual you have prepared, or one in your spellbook")
	}
	if !sp.Ritual {
		return Ritual{}, refuse(sp.Name + " is not a ritual")
	}
	out := Ritual{Spell: sp, Minutes: rules.RitualMinutes(sp.CastingTime), Clock: sc.Clock}
	out.Clock.Day, out.Clock.Minute = rules.AdvanceClock(sc.Clock.Day, sc.Clock.Minute, out.Minutes)
	return out, s.Repo.InTx(ctx, func(r Repository) error { return r.SetClock(ctx, id, out.Clock) })
}

func castable(sc Spellcasting, spell string) (compendium.SpellOption, bool) {
	for _, cs := range sc.Classes {
		for _, list := range [][]compendium.SpellOption{cs.Prepared, cs.Always, cs.Spellbook} {
			if sp, ok := find(list, func(o compendium.SpellOption) bool { return o.Slug == spell }); ok {
				return sp, true
			}
		}
	}
	return compendium.SpellOption{}, false
}

// CopySpell writes a wizard spell into the spellbook: free while the book holds fewer than its allotment,
// otherwise 50 gold pieces and 2 hours on the Game Clock per spell level, paid from the Character's coins.
func (s *Characters) CopySpell(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, spell string) (Spellcasting, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return Spellcasting{}, err
	}
	sc, err := s.spellcasting(ctx, sheet)
	if err != nil {
		return Spellcasting{}, err
	}
	i := slices.IndexFunc(sc.Classes, func(x ClassSpells) bool { return x.Book })
	if i < 0 {
		return Spellcasting{}, refuse("only a wizard keeps a spellbook")
	}
	cs := sc.Classes[i]
	list, err := s.Compendium.ClassSpells(ctx, sheet.Ruleset, cs.Class, cs.MaxLevel)
	if err != nil {
		return Spellcasting{}, err
	}
	sp, ok := find(list, func(o compendium.SpellOption) bool { return o.Slug == spell && o.Level > 0 })
	if !ok || slices.ContainsFunc(cs.Spellbook, func(o compendium.SpellOption) bool { return o.Slug == spell }) {
		return Spellcasting{}, refuse("copy a wizard spell you can cast that is not in your spellbook yet")
	}
	purse, clock, err := s.copyCost(ctx, sheet, sc, cs, sp)
	if err != nil {
		return Spellcasting{}, err
	}
	rows := append(preparedRows(sheet, cs, slugsOf(cs.Prepared)), domain.LearnedSpell{Class: cs.Class, Spell: sp.Slug, Level: sheet.Level, Prepared: false, Spellbook: true})
	err = s.Repo.InTx(ctx, func(r Repository) error {
		if err := r.ReplaceClassSpells(ctx, ch, cs.Class, rows, sheet.CanPrepare); err != nil {
			return err
		}
		if purse.Container == uuid.Nil {
			return nil
		}
		if err := r.SetPurse(ctx, purse); err != nil {
			return err
		}
		return r.SetClock(ctx, id, clock)
	})
	if err != nil {
		return Spellcasting{}, err
	}
	return s.Spells(ctx, c, id, ch)
}

// copyCost is the purse and Game Clock after copying a spell: unchanged while the book has free room,
// otherwise less the gold and later by the hours. A purse without a container is left alone.
func (s *Characters) copyCost(ctx context.Context, sheet Sheet, sc Spellcasting, cs ClassSpells, sp compendium.SpellOption) (domain.Purse, domain.Clock, error) {
	purse, clock := domain.Purse{Container: uuid.Nil, Coins: sc.Purse}, sc.Clock
	if len(cs.Spellbook) < cs.Allotment {
		return purse, clock, nil
	}
	gp, minutes := rules.CopyCost(sp.Level)
	coins, paid := shops.Pay(sc.Purse, gp*100)
	if !paid {
		return purse, clock, refuse("copying " + sp.Name + " costs " + strconv.Itoa(gp) + " gold pieces")
	}
	purse, err := s.Repo.Purse(ctx, sheet.ID)
	purse.Coins = coins
	clock.Day, clock.Minute = rules.AdvanceClock(clock.Day, clock.Minute, minutes)
	return purse, clock, err
}

func slugsOf(spells []compendium.SpellOption) []string {
	out := make([]string, 0, len(spells))
	for _, sp := range spells {
		out = append(out, sp.Slug)
	}
	return out
}
