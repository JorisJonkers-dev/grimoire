package live

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surprise"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// statsOf is what a token fights and hides with; a token without a statblock has no bonuses and walks 30 ft.
func statsOf(t domain.Token) domain.Stats {
	if t.Stats == nil {
		return domain.Stats{SpeedFt: 30}
	}
	return *t.Stats
}

func (s *state) zone(id string) (domain.Zone, bool) {
	zid := domain.ZoneID(parseID(id))
	i := slices.IndexFunc(s.zones, func(z domain.Zone) bool { return z.ID == zid })
	if i < 0 {
		return domain.Zone{}, false
	}
	return cloneZone(s.zones[i]), true
}

func cloneZone(z domain.Zone) domain.Zone {
	z.Checks, z.Creatures = slices.Clone(z.Checks), slices.Clone(z.Creatures)
	return z
}

// creatures are the tokens still on the map of those a sprung zone held.
func (s *state) creatures(z domain.Zone) []domain.Token {
	var out []domain.Token
	for _, id := range z.Creatures {
		if t, ok := s.tokens[id]; ok {
			out = append(out, t)
		}
	}
	return out
}

// lurkers are the hidden creatures waiting in a zone.
func (s *state) lurkers(z domain.Zone) []domain.Token {
	var out []domain.Token
	for _, t := range s.tokens {
		if t.Hidden && t.Kind != domain.TokenParty && z.Covers(hex.Coord{Q: t.Q, R: t.R}) {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b domain.Token) int {
		return strings.Compare(uuid.UUID(a.ID).String(), uuid.UUID(b.ID).String())
	})
	return out
}

func (s *state) party() []domain.Token {
	var out []domain.Token
	for _, t := range s.tokens {
		if t.Kind == domain.TokenParty {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b domain.Token) int {
		return strings.Compare(uuid.UUID(a.ID).String(), uuid.UUID(b.ID).String())
	})
	return out
}

// planZone places, removes, holds or springs an Encounter Zone.
func (r *runtime) planZone(m domain.Member, cmd Command) (Write, string) {
	if cmd.Kind == CmdAddZone {
		name, at := strings.TrimSpace(cmd.Label), hex.Coord{Q: cmd.Q, R: cmd.R}
		switch {
		case name == "" || utf8.RuneCountInString(name) > 40:
			return Write{}, "Give the zone a name of up to 40 characters."
		case cmd.RadiusHexes < 1 || cmd.RadiusHexes > 20:
			return Write{}, "A zone reaches 1 to 20 hexes."
		case !r.st.onBoard(at):
			return Write{}, "That hex is off the map."
		}
		z := domain.Zone{ID: domain.ZoneID(uuid.New()), Name: name, At: at, RadiusHexes: cmd.RadiusHexes, DMOnly: cmd.DMOnly, Status: domain.ZoneArmed, Checks: []domain.ZoneCheck{}}
		return Write{Kind: domain.ActionZoneAdded, Zone: &z}, ""
	}
	z, ok := r.st.zone(cmd.ZoneID)
	switch {
	case !ok:
		return Write{}, "No such zone."
	case cmd.Kind == CmdRemoveZone:
		return Write{Kind: domain.ActionZoneRemoved, Zone: &z}, ""
	case z.Status != domain.ZoneArmed:
		return Write{}, "That zone has already sprung."
	case cmd.Kind == CmdHoldZone:
		z.Held = cmd.On
		return Write{Kind: domain.ActionZoneHeld, Zone: &z}, ""
	}
	return r.spring(m, z)
}

// spring sets off a zone: party members whose passive Perception meets the hidden creatures' Stealth
// notice them at once, and everyone else rolls Perception against a DC they never see.
func (r *runtime) spring(dm domain.Member, z domain.Zone) (Write, string) {
	lurkers := r.st.lurkers(z)
	switch {
	case r.st.combat != nil:
		return Write{}, "A fight is already on."
	case len(lurkers) == 0:
		return Write{}, "No hidden creatures wait in that zone."
	}
	stealth := make([]int, 0, len(lurkers))
	for _, t := range lurkers {
		stealth = append(stealth, statsOf(t).Stealth)
		z.Creatures = append(z.Creatures, t.ID)
	}
	z.DC, z.Status, z.Checks = surprise.DC(stealth), domain.ZoneSpotting, []domain.ZoneCheck{}
	w := Write{Kind: domain.ActionZoneSprung}
	for _, t := range r.st.party() {
		bonus := statsOf(t).Perception
		check := domain.ZoneCheck{Token: t.ID}
		if surprise.Notices(surprise.Passive(bonus), z.DC) {
			yes := true
			check.Noticed = &yes
		} else {
			roll := r.request(dm, t, "Perception for "+t.Label, "1d20", domain.Modifier{Label: "Perception", Value: bonus})
			check.RollID = &roll.ID
			w.Rolls = append(w.Rolls, roll)
		}
		z.Checks = append(z.Checks, check)
	}
	w.Zone = &z
	w.Revealed = r.noticed(z, lurkers)
	return w, ""
}

// noticed reveals the zone's creatures to everyone once any party member noticed them.
func (r *runtime) noticed(z domain.Zone, lurkers []domain.Token) []domain.Token {
	if !slices.ContainsFunc(z.Checks, func(c domain.ZoneCheck) bool { return c.Noticed != nil && *c.Noticed }) {
		return nil
	}
	return shown(lurkers)
}

func shown(ts []domain.Token) []domain.Token {
	out := make([]domain.Token, 0, len(ts))
	for _, t := range ts {
		t.Hidden = false
		out = append(out, t)
	}
	return out
}

// perceived takes in a party member's Perception roll against a sprung zone.
func (r *runtime) perceived(z domain.Zone, id domain.RollID) {
	roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, id)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	i := slices.IndexFunc(z.Checks, func(c domain.ZoneCheck) bool { return c.RollID != nil && *c.RollID == id })
	yes := surprise.Notices(roll.Total, z.DC)
	z.Checks[i].Noticed = &yes
	w := Write{Kind: domain.ActionPerceptionRolled, Zone: &z, Token: r.st.tokens[z.Checks[i].Token]}
	w.Revealed = r.noticed(z, r.st.creatures(z))
	r.commit(request{}, w, roll.Roller, caller.Caller{Subject: roll.Roller.Subject, Origin: caller.OriginSystem})
}

// zoneRoll finds the sprung zone waiting on a Perception roll.
func (s *state) zoneRoll(id domain.RollID) (domain.Zone, bool) {
	for _, z := range s.zones {
		if z.Status == domain.ZoneSpotting && slices.ContainsFunc(z.Checks, func(c domain.ZoneCheck) bool { return c.RollID != nil && *c.RollID == id && c.Noticed == nil }) {
			return cloneZone(z), true
		}
	}
	return domain.Zone{}, false
}

// ambush follows a change outside Combat: a zone whose Perception is settled starts the fight, and an
// armed zone springs when a party member comes within it.
func (r *runtime) ambush(c caller.Caller) {
	if r.dm == nil {
		return
	}
	for _, z := range r.st.zones {
		switch {
		case z.Status == domain.ZoneSpotting && z.Decided():
			r.commit(request{}, r.fight(*r.dm, cloneZone(z)), *r.dm, c)
			return
		case z.Status == domain.ZoneArmed && !z.Held && !z.DMOnly && slices.ContainsFunc(r.st.party(), func(t domain.Token) bool { return z.Covers(hex.Coord{Q: t.Q, R: t.R}) }):
			if w, reason := r.spring(*r.dm, cloneZone(z)); reason == "" {
				r.commit(request{}, w, *r.dm, c)
			}
			return
		}
	}
}

// fight starts the Combat a sprung zone leads to: its creatures and the party, with everyone who never
// noticed the ambush rolling initiative at disadvantage.
func (r *runtime) fight(dm domain.Member, z domain.Zone) Write {
	lurkers := r.st.creatures(z)
	unaware := map[domain.TokenID]bool{}
	var party []domain.Token
	for _, c := range z.Checks {
		if t, ok := r.st.tokens[c.Token]; ok {
			unaware[t.ID] = !*c.Noticed
			party = append(party, t)
		}
	}
	f := &domain.Combat{ID: domain.CombatID(uuid.New()), Status: domain.CombatRolling, StartedAt: r.now()}
	var rolls []domain.Roll
	for _, t := range append(slices.Clone(lurkers), party...) {
		st := statsOf(t)
		label := "Initiative for " + t.Label
		if unaware[t.ID] {
			label += " (surprised)"
		}
		roll := r.request(dm, t, label, surprise.Initiative(unaware[t.ID]), domain.Modifier{Label: "Initiative", Value: st.Initiative})
		rolls = append(rolls, roll)
		f.Combatants = append(f.Combatants, domain.Combatant{
			ID: domain.CombatantID(uuid.New()), TokenID: t.ID, RollID: roll.ID, InitiativeBonus: st.Initiative, SpeedFt: st.SpeedFt, Surprised: unaware[t.ID],
		})
	}
	z.Status = domain.ZoneSprung
	return Write{Kind: domain.ActionCombatStarted, Combat: f, Rolls: rolls, Zone: &z, Revealed: shown(lurkers)}
}

// applyZones keeps the zone a write changed and shows the creatures it revealed.
func applyZones(s *state, w *Write) {
	for _, t := range w.Revealed {
		s.tokens[t.ID] = t
	}
	if w.Zone == nil {
		return
	}
	i := slices.IndexFunc(s.zones, func(z domain.Zone) bool { return z.ID == w.Zone.ID })
	switch {
	case w.Kind == domain.ActionZoneRemoved:
		s.zones = slices.Delete(s.zones, i, i+1)
	case i < 0:
		s.zones = append(s.zones, cloneZone(*w.Zone))
	default:
		s.zones[i] = cloneZone(*w.Zone)
	}
}

// forgetChecks drops a removed token from every zone's Perception.
func (s *state) forgetChecks(id domain.TokenID) {
	for i, z := range s.zones {
		s.zones[i].Checks = slices.DeleteFunc(slices.Clone(z.Checks), func(c domain.ZoneCheck) bool { return c.Token == id })
		s.zones[i].Creatures = slices.DeleteFunc(slices.Clone(z.Creatures), func(c domain.TokenID) bool { return c == id })
	}
}

// zoneViews shows the DM every zone.
func (s *state) zoneViews() []ZoneView {
	out := []ZoneView{}
	for _, z := range s.zones {
		v := ZoneView{
			ID: uuid.UUID(z.ID).String(), Name: z.Name, Q: z.At.Q, R: z.At.R, RadiusHexes: z.RadiusHexes, DMOnly: z.DMOnly, Held: z.Held, Status: z.Status,
			DC: z.DC, Creatures: len(s.creatures(z)), Checks: []ZoneCheckView{},
		}
		if z.Status == domain.ZoneArmed {
			v.Creatures = len(s.lurkers(z))
		}
		for _, c := range z.Checks {
			v.Checks = append(v.Checks, ZoneCheckView{TokenID: uuid.UUID(c.Token).String(), Noticed: c.Noticed})
		}
		out = append(out, v)
	}
	return out
}

// perceptionViews lists the Perception Roll Cards still open, for every audience.
func (s *state) perceptionViews() []PerceptionView {
	var out []PerceptionView
	for _, z := range s.zones {
		for _, c := range z.Checks {
			if z.Status == domain.ZoneSpotting && c.RollID != nil && c.Noticed == nil {
				out = append(out, PerceptionView{RollID: uuid.UUID(*c.RollID).String(), TokenID: uuid.UUID(c.Token).String()})
			}
		}
	}
	return out
}
