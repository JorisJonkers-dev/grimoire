package live

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/tactics"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// SpawnMonster is one kind of creature spawn_encounter places, and how many.
type SpawnMonster struct {
	Slug  string `json:"monsterSlug"`
	Count int    `json:"count"`
}

// ActionRecord is one Action of the Session, with what undoing it needs.
type ActionRecord struct {
	ID      uuid.UUID
	Kind    string
	Undone  bool
	Token   domain.TokenID
	Hexes   []hex.Coord
	HP      HPChange
	Spawned []domain.TokenID
	Effect  domain.EffectID
}

// maxSpawn caps how many creatures one spawn places.
const maxSpawn = 30

// planSpawn places an encounter's creatures together on the nearest free hexes around the one chosen.
func (r *runtime) planSpawn(cmd Command) (Write, string) {
	at := hex.Coord{Q: cmd.Q, R: cmd.R}
	total := 0
	for _, m := range cmd.Monsters {
		if m.Count < 1 {
			return Write{}, "Spawn at least one of each creature."
		}
		total += m.Count
	}
	switch {
	case total == 0 || total > maxSpawn:
		return Write{}, "Spawn between 1 and 30 creatures."
	case !r.st.onBoard(at):
		return Write{}, "That hex is off the map."
	}
	free := r.st.freeHexes(at, total)
	if len(free) < total {
		return Write{}, "There is no room for that many creatures there."
	}
	w := Write{Kind: domain.ActionEncounterSpawned}
	for _, m := range cmd.Monsters {
		name, stats, err := r.stats.Monster(context.Background(), r.st.session.CampaignID, m.Slug)
		if err != nil {
			return Write{}, "No such monster: " + m.Slug + "."
		}
		for i := range m.Count {
			label := name
			if m.Count > 1 {
				label = name + " " + strconv.Itoa(i+1)
			}
			st, c := stats, free[len(w.Spawned)]
			w.Spawned = append(w.Spawned, domain.Token{
				ID: domain.TokenID(uuid.New()), Label: string([]rune(label)[:min(len([]rune(label)), 40)]), Kind: domain.TokenEnemy,
				Q: c.Q, R: c.R, Hidden: cmd.Hidden, Stats: &st, Tactics: tactics.FromIntelligence, CanShield: st.Shield,
			})
		}
	}
	return w, ""
}

// freeHexes lists up to n hexes on the board, nearest first, with no token and no wall.
func (s *state) freeHexes(at hex.Coord, n int) []hex.Coord {
	taken := map[hex.Coord]bool{}
	for _, t := range s.tokens {
		taken[hex.Coord{Q: t.Q, R: t.R}] = true
	}
	if s.board != nil {
		for c := range s.board.Walls {
			taken[c] = true
		}
	}
	near := hex.Disk(at, 6)
	slices.SortFunc(near, func(a, b hex.Coord) int {
		return cmp.Or(cmp.Compare(hex.Distance(at, a), hex.Distance(at, b)), cmp.Compare(a.Q, b.Q), cmp.Compare(a.R, b.R))
	})
	var out []hex.Coord
	for _, c := range near {
		if len(out) < n && s.onBoard(c) && !taken[c] {
			out = append(out, c)
		}
	}
	return out
}

// planAdjustHP adds to or takes from a creature's hit points by hand, within 0 and its maximum.
func (r *runtime) planAdjustHP(cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	switch {
	case !ok:
		return Write{}, "No such token."
	case t.Stats == nil:
		return Write{}, "That token has no hit points."
	case cmd.HPDelta == 0:
		return Write{}, "Change the hit points by at least one."
	}
	if d, down := r.st.dying[t.ID]; down && d.State.Dead && cmd.HPDelta > 0 {
		return Write{}, t.Label + " is dead; only revival magic brings them back."
	}
	after := max(0, min(t.Stats.HPMax, t.Stats.HP+cmd.HPDelta))
	return Write{Kind: domain.ActionHPAdjusted, Token: t, HP: &HPChange{Token: t.ID, Before: t.Stats.HP, After: after, Raw: -cmd.HPDelta}}, ""
}

// tokenByID finds a token on the board by its id as a client sends it.
func (s *state) tokenByID(raw string) (domain.Token, bool) {
	id, err := uuid.Parse(raw)
	if err != nil {
		return domain.Token{}, false
	}
	t, ok := s.tokens[domain.TokenID(id)]
	return t, ok
}

// undo reverts one Action of the Session by its Action Log sequence. Removing a spawned encounter
// takes one Action per creature, the last of them recorded as the undo.
func (r *runtime) undo(req request) {
	a, err := r.store.Action(context.Background(), r.st.session.ID, req.cmd.Seq)
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		r.reject(req, "No such action in this session.")
		return
	case err != nil:
		r.log.Error("live: load action", "error", err)
		r.reject(req, "The action could not be read.")
		return
	case a.Undone:
		r.reject(req, "That action is already undone.")
		return
	}
	if a.Kind == domain.ActionEncounterSpawned {
		r.unspawn(req, a)
		return
	}
	w, reason := r.inverse(a)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	w.Undoes = a.ID
	r.commit(req, w, req.from.Member, req.from.Caller)
}

// unspawn removes the creatures of a spawned encounter that are still on the board.
func (r *runtime) unspawn(req request, a ActionRecord) {
	var left []domain.Token
	for _, id := range a.Spawned {
		if t, ok := r.st.tokens[id]; ok {
			left = append(left, t)
		}
	}
	if len(left) == 0 {
		r.reject(req, "Those creatures are already gone.")
		return
	}
	for i, t := range left {
		w := Write{Kind: domain.ActionTokenRemoved, Token: t}
		if i < len(left)-1 {
			r.commit(request{}, w, req.from.Member, req.from.Caller)
			continue
		}
		w.Undoes = a.ID
		r.commit(req, w, req.from.Member, req.from.Caller)
	}
}

// inverse is the change that reverts an Action, or why Grimoire cannot revert it.
func (r *runtime) inverse(a ActionRecord) (Write, string) {
	t, present := r.st.tokens[a.Token]
	switch a.Kind {
	case domain.ActionTokenPlaced:
		if !present {
			return Write{}, "That token is already gone."
		}
		return Write{Kind: domain.ActionTokenRemoved, Token: t}, ""
	case domain.ActionHexesRevealed, domain.ActionHexesConcealed:
		if r.st.board == nil {
			return Write{}, "Choose a map first."
		}
		kind := map[bool]string{true: domain.ActionHexesConcealed, false: domain.ActionHexesRevealed}[a.Kind == domain.ActionHexesRevealed]
		return Write{Kind: kind, Hexes: a.Hexes}, ""
	case domain.ActionEffectApplied:
		if !slices.ContainsFunc(r.st.fx.Active, func(e domain.Effect) bool { return e.ID == a.Effect }) {
			return Write{}, "That effect has already ended."
		}
		return Write{Kind: domain.ActionEffectEnded, Token: t, ended: []domain.EffectID{a.Effect}}, ""
	case domain.ActionDamageDealt, domain.ActionHPAdjusted:
		h, ok := r.st.tokens[a.HP.Token]
		if !ok || h.Stats == nil {
			return Write{}, "That token is already gone."
		}
		after := max(0, min(h.Stats.HPMax, h.Stats.HP+a.HP.Before-a.HP.After))
		kind := map[bool]string{true: domain.ActionDamageUndone, false: domain.ActionHPAdjusted}[a.Kind == domain.ActionDamageDealt]
		return Write{Kind: kind, Token: h, HP: &HPChange{Token: h.ID, Before: h.Stats.HP, After: after, Undoes: a.ID}}, ""
	}
	return Write{}, "Grimoire cannot undo that action."
}
