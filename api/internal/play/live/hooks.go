package live

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/rolltable"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Hook is a Rule Variant the DM authored: at its hook point it applies an Effect to whoever it
// happened to, or has them roll on a Roll Table.
type Hook struct {
	Name   string
	Point  string
	Table  *uuid.UUID
	Effect string
}

// Table is a Roll Table of the Library a Campaign sees.
type Table struct {
	ID     uuid.UUID
	Name   string
	Design rolltable.Design
}

// TableResult is what a roll on a Roll Table landed on, and for whom.
type TableResult struct {
	Hook  string
	Table string
	Token domain.TokenID
	Label string
	Total int
	Text  string
}

// TableResultView is the last Roll Table result, as every screen that sees the creature reads it.
type TableResultView struct {
	Hook    string `json:"hook"`
	Table   string `json:"table"`
	TokenID string `json:"tokenId"`
	Label   string `json:"label"`
	Total   int    `json:"total"`
	Text    string `json:"text"`
}

// tableRoll is the pending roll on a Roll Table.
const tableRoll = "roll_table"

// nothing is what a roll says when it lands in a gap of its table, or the table is gone.
const nothing = "Nothing happens."

// ruleHooks fires the Campaign's own Rule Variants a change sets off: an attack's natural 1 or 20,
// a creature dropping to 0 hit points, a rest finished, a spell cast over an area.
func (r *runtime) ruleHooks(w Write, actor domain.Member, c caller.Caller) {
	switch {
	case w.hook != "":
		r.hooked(w.hook, w.Token.ID, actor, c)
	case w.Kind == domain.ActionRestTaken:
		for _, res := range w.Results {
			r.hooked(variants.HookRest, res.Token, actor, c)
		}
	case w.Kind == domain.ActionAreaCast:
		r.hooked(variants.HookCast, w.Token.ID, actor, c)
	}
	if h := w.HP; h != nil && h.Before > 0 && h.After == 0 {
		r.hooked(variants.HookDropTo0, h.Token, actor, c)
	}
}

// hooked runs every Rule Variant of the Campaign's own hung on a hook point, for the creature it
// happened to. They are read as it happens, so a Session under way follows the DM's changes. A hook
// whose Roll Table the Campaign no longer sees, or whose Effect the rules do not know, does nothing.
func (r *runtime) hooked(point string, id domain.TokenID, actor domain.Member, c caller.Caller) {
	ctx := context.Background()
	hooks, err := r.store.RuleHooks(ctx, r.campaign)
	if err != nil {
		r.log.Error("live: rule hooks", "error", err)
		return
	}
	hooks = slices.DeleteFunc(hooks, func(h Hook) bool { return h.Point != point })
	if len(hooks) == 0 {
		return
	}
	tables, err := r.store.RollTables(ctx, r.campaign)
	if err != nil {
		r.log.Error("live: roll tables", "error", err)
		return
	}
	for _, h := range hooks {
		t := r.st.tokens[id]
		w := Write{Kind: domain.ActionHookFired, Token: t, Note: h.Name}
		if h.Table == nil {
			def, known := r.st.catalog.Lookup(h.Effect)
			if !known {
				continue
			}
			w.effect = &domain.Effect{ID: domain.EffectID(uuid.New()), Target: t.ID, Slug: h.Effect, Name: def.Name, Level: 1}
		} else {
			i := slices.IndexFunc(tables, func(x Table) bool { return x.ID == *h.Table })
			if i < 0 {
				continue
			}
			roll := r.request(r.rollerFor(actor), t, h.Name+": "+tables[i].Name, tables[i].Design.Notation())
			// The roll is the creature's own: it is its target as well as its actor.
			w.Rolls, w.Pending = []domain.Roll{roll}, &domain.PendingAction{RollID: roll.ID, Actor: t.ID, Target: &t.ID, Action: tableRoll, Table: h.Table, Hook: h.Name}
		}
		r.commit(request{}, w, actor, c)
	}
}

// rollerFor is who asks for a creature's roll: the DM, not the Player whose action set it off, so that
// a roll for a creature nobody controls falls to the DM. A creature somebody controls rolls for itself.
func (r *runtime) rollerFor(actor domain.Member) domain.Member {
	if !actor.DM && r.dm != nil {
		return *r.dm
	}
	return actor
}

// tableRolled lands a roll on its Roll Table: the result's words go to every screen that sees the
// creature and to the Action Log, its Effect lands on the creature, and its Item drops as loot.
func (r *runtime) tableRolled(w Write, p domain.PendingAction, roll domain.Roll) {
	ctx := context.Background()
	sys := caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem, Client: ""}
	a := w.Token
	w.Kind = domain.ActionTableRolled
	shown := TableResult{Hook: p.Hook, Table: "", Token: a.ID, Label: a.Label, Total: roll.Total, Text: nothing}
	var landed rolltable.Result
	tables, err := r.store.RollTables(ctx, r.campaign)
	if err != nil {
		r.log.Error("live: roll tables", "error", err)
	}
	if i := slices.IndexFunc(tables, func(x Table) bool { return x.ID == *p.Table }); i >= 0 {
		shown.Table = tables[i].Name
		if res, ok := tables[i].Design.Roll(roll.Total); ok {
			landed, shown.Text = res, res.Text
		}
	}
	if def, known := r.st.catalog.Lookup(landed.Effect); known {
		w.effect = &domain.Effect{ID: domain.EffectID(uuid.New()), Target: a.ID, Slug: landed.Effect, Name: def.Name, Level: 1}
	}
	w.Note, w.tableResult = shown.Text, &shown
	r.commit(request{}, w, roll.Roller, sys)
	if landed.Item == "" {
		return
	}
	items, err := r.store.Items(ctx, r.campaign, []string{landed.Item})
	if err != nil {
		r.log.Error("live: item info", "error", err)
		return
	}
	if _, known := items[landed.Item]; !known {
		return
	}
	drop := domain.Container{
		ID: domain.ContainerID(uuid.New()), Kind: domain.ContainerDrop, Label: shown.Table, Items: map[string]int{landed.Item: landed.Gives()}, Coins: map[string]int{}, CreatedAt: r.now(),
	}
	r.commit(request{}, Write{Kind: domain.ActionLootDropped, Drop: &drop, items: items}, roll.Roller, sys)
}

// applyHook keeps a roll on a Roll Table that is still open, settles the one that landed and keeps
// its result to show.
func applyHook(s *state, w *Write) {
	if w.Pending != nil {
		s.pending = append(s.pending, *w.Pending)
	}
	s.pending = slices.DeleteFunc(s.pending, func(p domain.PendingAction) bool { return p.RollID == w.Settled })
	if w.tableResult != nil {
		s.tableResult = w.tableResult
	}
}

// tableResultView shows the last Roll Table result to the DM, and to every other screen that sees the
// creature it was for.
func (s *state) tableResultView(a Audience, seen map[hex.Coord]bool) *TableResultView {
	res := s.tableResult
	if res == nil {
		return nil
	}
	if t, here := s.tokens[res.Token]; a != AudienceDM && (!here || !s.shows(t, seen)) {
		return nil
	}
	return &TableResultView{Hook: res.Hook, Table: res.Table, TokenID: uuid.UUID(res.Token).String(), Label: res.Label, Total: res.Total, Text: res.Text}
}
