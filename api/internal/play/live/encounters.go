package live

import (
	"context"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/encounters"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Rests the party takes.
const (
	RestShort = "short"
	RestLong  = "long"
)

func tableByID(tables []prep.Table, id string) (prep.Table, bool) {
	tid := prep.TableID(parseID(id))
	i := slices.IndexFunc(tables, func(t prep.Table) bool { return t.ID == tid })
	if i < 0 {
		return prep.Table{}, false
	}
	return tables[i], true
}

// planEncounter takes a rest, schedules a check, or runs one now.
func (r *runtime) planEncounter(m domain.Member, cmd Command) (Write, string) {
	if cmd.Kind == CmdRest {
		if cmd.Rest != RestShort && cmd.Rest != RestLong {
			return Write{}, "A rest is short or long."
		}
		return Write{Kind: domain.ActionRestTaken, Rest: cmd.Rest}, ""
	}
	p, err := r.store.LoadPrep(context.Background(), r.campaign)
	if err != nil {
		r.log.Error("live: load prep", "error", err)
		return Write{}, "The encounter tables could not be read."
	}
	t, ok := tableByID(p.Tables, cmd.TableID)
	if !ok {
		return Write{}, "No such encounter table."
	}
	if cmd.Kind == CmdScheduleCheck {
		if cmd.Due != prep.DueNextRest && cmd.Due != prep.DueNextTravel {
			return Write{}, "Schedule a check for the next rest or the next Travel Leg."
		}
		return Write{Kind: domain.ActionCheckScheduled, Schedule: &prep.Scheduled{ID: uuid.New(), TableID: t.ID, Due: cmd.Due}}, ""
	}
	switch {
	case cmd.Mode != prep.ModeNormal && cmd.Mode != prep.ModeForce && cmd.Mode != prep.ModePick:
		return Write{}, "A check rolls normally, forces an encounter, or takes the entry you pick."
	case cmd.Mode == prep.ModePick && (cmd.Entry < 0 || cmd.Entry >= len(t.Entries)):
		return Write{}, "Pick one of the table's entries."
	}
	return r.check(m, p, t, prep.TriggerDM, cmd.Mode, cmd.Entry), ""
}

// check runs an Encounter Check. A normal check on an open table opens a percentile Roll Request the
// Table Display shows; every other check rolls and draws at once from its seed.
func (r *runtime) check(dm domain.Member, p prep.Prep, t prep.Table, trigger, mode string, pick int) Write {
	seed := r.seed()
	sid := uuid.UUID(r.st.session.ID)
	c := prep.Check{
		ID: prep.CheckID(uuid.New()), SessionID: &sid, TableID: &t.ID, TableName: t.Name, Trigger: trigger, Mode: mode, Visibility: t.Visibility,
		Seed: int64(seed), ChancePct: t.ChancePct, Status: prep.CheckPending, Monsters: []prep.EntryMonster{}, CreatedAt: r.now(), //nolint:gosec // a seed is any 64 bits
	}
	if mode == prep.ModeNormal && t.Visibility == prep.Open {
		roll := r.request(dm, domain.Token{}, "Encounter check", "1d100")
		id := uuid.UUID(roll.ID)
		c.RollID = &id
		return Write{Kind: domain.ActionEncounterChecked, Check: &c, Rolls: []domain.Roll{roll}}
	}
	src := r.source(seed)
	chance := 0
	if mode == prep.ModeNormal {
		chance = dice.Face(src, 100)
	}
	resolveCheck(&c, p, t, src, chance, pick)
	return Write{Kind: domain.ActionEncounterChecked, Check: &c}
}

// resolveCheck rolls the chance, draws an entry by weight (never Nothing when forced), and fills a Pool
// draw to the party's XP budget.
func resolveCheck(c *prep.Check, p prep.Prep, t prep.Table, src dice.Source, chance, pick int) {
	c.Status, c.Outcome = prep.CheckResolved, prep.OutcomeNothing
	if c.Mode == prep.ModeNormal {
		c.ChanceRoll = chance
		if !encounters.Triggered(chance, c.ChancePct) {
			return
		}
	}
	i := pick
	if c.Mode != prep.ModePick {
		weights := make([]int, len(t.Entries))
		for j, e := range t.Entries {
			if c.Mode != prep.ModeForce || e.Kind != prep.EntryNothing {
				weights[j] = e.Weight
			}
		}
		if i = encounters.Draw(src, weights); i < 0 {
			return
		}
	}
	e := t.Entries[i]
	c.EntryLabel = e.Label
	monsters := e.Monsters
	if e.Kind == prep.EntryPool {
		monsters = fillPool(p, *e.PoolID, src, &c.EntryLabel)
	}
	if len(monsters) > 0 {
		c.Outcome, c.Monsters = prep.OutcomeFight, monsters
	}
}

// fillPool draws a Pool's creatures to the party's budget, unless the party is outside its level band.
func fillPool(p prep.Prep, id prep.PoolID, src dice.Source, label *string) []prep.EntryMonster {
	i := slices.IndexFunc(p.Pools, func(x prep.Pool) bool { return x.ID == id })
	pool := p.Pools[i]
	if *label == "" {
		*label = pool.Name
	}
	total := 0
	for _, l := range p.Levels {
		total += l
	}
	level := 1
	if len(p.Levels) > 0 {
		level = total / len(p.Levels)
	}
	if level < pool.LevelMin || level > pool.LevelMax {
		*label += " (the party is outside its levels)"
		return nil
	}
	difficulty, _ := encounters.ParseDifficulty(pool.Difficulty)
	members := make([]encounters.Member, 0, len(pool.Members))
	for _, m := range pool.Members {
		members = append(members, encounters.Member{Slug: m.Slug, XP: p.XP[m.Slug], Weight: m.Weight, Min: m.Min, Max: m.Max})
	}
	var out []prep.EntryMonster
	for _, pick := range encounters.Fill(src, encounters.Budget(p.Levels, difficulty), members) {
		out = append(out, prep.EntryMonster{Slug: pick.Slug, Count: pick.Count})
	}
	return out
}

// pendingCheck finds the open check waiting on a Roll Request.
func (s *state) pendingCheck(id domain.RollID) (prep.Check, bool) {
	i := slices.IndexFunc(s.checks, func(c prep.Check) bool {
		return c.Status == prep.CheckPending && c.RollID != nil && *c.RollID == uuid.UUID(id)
	})
	if i < 0 {
		return prep.Check{}, false
	}
	return s.checks[i], true
}

// checkRolled resolves an open check once its percentile roll is in. A table deleted meanwhile draws Nothing.
func (r *runtime) checkRolled(c prep.Check) {
	roll, err := r.store.Roll(context.Background(), r.campaign, domain.RollID(*c.RollID))
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	p, err := r.store.LoadPrep(context.Background(), r.campaign)
	if err != nil {
		r.log.Error("live: load prep", "error", err)
		return
	}
	t, _ := tableByID(p.Tables, uuid.UUID(*c.TableID).String())
	resolveCheck(&c, p, t, r.source(uint64(c.Seed)), roll.Total, -1) //nolint:gosec // the seed round-trips its 64 bits
	r.commit(request{}, Write{Kind: domain.ActionEncounterResolved, Check: &c}, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem})
}

// encounterChecks follows a rest or a Travel Leg: the checks the DM scheduled for it, then the check of
// the Table for where the party stands, or of the Table that applies everywhere.
func (r *runtime) encounterChecks(trigger, due string, actor domain.Member, c caller.Caller) {
	p, err := r.store.LoadPrep(context.Background(), r.campaign)
	if err != nil {
		r.log.Error("live: load prep", "error", err)
		return
	}
	for _, s := range p.Scheduled {
		if s.Due != due {
			continue
		}
		t, _ := tableByID(p.Tables, uuid.UUID(s.TableID).String())
		w := r.check(actor, p, t, trigger, prep.ModeNormal, -1)
		w.Unschedule = s.ID
		r.commit(request{}, w, actor, c)
	}
	if t, ok := r.regionTable(p.Tables); ok {
		r.commit(request{}, r.check(actor, p, t, trigger, prep.ModeNormal, -1), actor, c)
	}
}

// regionTable is the Table of the location the party stands at on the world map, or else one without a Region.
func (r *runtime) regionTable(tables []prep.Table) (prep.Table, bool) {
	var here *uuid.UUID
	if w := r.st.world; w != nil && w.Party != nil {
		id := uuid.UUID(*w.Party)
		here = &id
	}
	for _, t := range tables {
		if here != nil && t.RegionID != nil && *t.RegionID == *here {
			return t, true
		}
	}
	i := slices.IndexFunc(tables, func(t prep.Table) bool { return t.RegionID == nil })
	if i < 0 {
		return prep.Table{}, false
	}
	return tables[i], true
}

// applyCheck keeps a check the write ran or resolved.
func applyCheck(s *state, c prep.Check) {
	i := slices.IndexFunc(s.checks, func(x prep.Check) bool { return x.ID == c.ID })
	if i < 0 {
		s.checks = append(s.checks, c)
		return
	}
	s.checks[i] = c
}

// checkViews shows the Session's latest checks. The DM sees everything; everyone else sees an open
// check's roll and every check's outcome, never its table, entry or creatures.
func (s *state) checkViews(a Audience) []CheckView {
	var out []CheckView
	for _, c := range s.checks[max(0, len(s.checks)-10):] {
		v := CheckView{ID: uuid.UUID(c.ID).String(), Trigger: c.Trigger, Visibility: c.Visibility, Status: c.Status, Outcome: c.Outcome}
		if a == AudienceDM || c.Visibility == prep.Open {
			v.ChancePct, v.ChanceRoll = c.ChancePct, c.ChanceRoll
			if c.RollID != nil {
				v.RollID = c.RollID.String()
			}
		}
		if a == AudienceDM {
			v.TableName, v.Mode, v.EntryLabel, v.Seed = c.TableName, c.Mode, c.EntryLabel, strconv.FormatInt(c.Seed, 10)
			for _, m := range c.Monsters {
				v.Monsters = append(v.Monsters, CheckMonsterView{Slug: m.Slug, Count: m.Count})
			}
		}
		out = append(out, v)
	}
	return out
}
