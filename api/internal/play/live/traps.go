package live

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/objects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Pending checks against Map Objects.
const (
	disarming = "disarm"
	picking   = "pick"
	forcing   = "force"
)

// reachObject finds a Map Object a member's creature works on: one the member knows, next to the
// creature unless anywhere will do.
func (r *runtime) reachObject(m domain.Member, cmd Command, t domain.Token, anywhere bool) (domain.MapObject, string) {
	id, _ := uuid.Parse(cmd.ObjectID)
	var o domain.MapObject
	known := false
	if r.st.board != nil {
		o, known = r.st.board.Objects[id]
	}
	switch {
	case !known || (!m.DM && !r.st.objectShown(o, r.st.vision())):
		return o, "No such object."
	case !anywhere && hex.Distance(hex.Coord{Q: t.Q, R: t.R}, o.At) > 1:
		return o, t.Label + " must stand next to the " + strings.ToLower(o.Name) + "."
	}
	return o, ""
}

// planUnlock gets through a lock: with its key, as a free object interaction; by picking it with
// thieves' tools or forcing it, as a check; or with Knock from up to 60 feet, as a Magic action.
func (r *runtime) planUnlock(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	}
	o, reason := r.reachObject(m, cmd, t, cmd.Method == "knock")
	switch {
	case reason != "":
		return Write{}, reason
	case !o.Locked:
		return Write{}, "The " + strings.ToLower(o.Name) + " is not locked."
	}
	opened := o
	opened.Locked = false
	switch cmd.Method {
	case "key":
		if o.Key == "" || !r.st.carries(t, o.Key) {
			return Write{}, t.Label + " carries no key to the " + strings.ToLower(o.Name) + "."
		}
		w := objectWrite(domain.ActionObjectUnlocked, opened)
		w.Token = t
		return w, r.st.interacting(&w, t)
	case "knock":
		if hex.Distance(hex.Coord{Q: t.Q, R: t.R}, o.At)*hex.FeetPerHex > objects.KnockRangeFt {
			return Write{}, "Knock reaches 60 feet."
		}
		_, x, reason := r.actor(m, cmd.TokenID, false)
		w := objectWrite(domain.ActionObjectUnlocked, opened)
		w.Token, w.taken = t, &takenAction{action: actions.Magic, readied: nil}
		if x != nil {
			w.Combatant = x.ID
		}
		return w, reason
	case "tools":
		return r.objectCheck(m, t, o, picking, o.LockDC)
	case "force":
		return r.objectCheck(m, t, o, forcing, o.LockDC)
	}
	return Write{}, "Unlock with the key, thieves' tools, force or Knock."
}

// planDisarm works on a known, armed trap next to the creature with thieves' tools.
func (r *runtime) planDisarm(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	if !ok || t.Stats == nil {
		return Write{}, "No such creature."
	}
	o, reason := r.reachObject(m, cmd, t, false)
	switch {
	case reason != "":
		return Write{}, reason
	case o.Kind != string(objects.Trap) || !o.Armed:
		return Write{}, "There is no armed trap there."
	}
	return r.objectCheck(m, t, o, disarming, o.DisarmDC)
}

// objectCheck opens the check a creature makes on an object with the Utilize action: Dexterity with
// thieves' tools to disarm or pick, its Initiative bonus standing in for Dexterity; Strength to force,
// its Strength save standing in for the check.
func (r *runtime) objectCheck(m domain.Member, t domain.Token, o domain.MapObject, action string, dc int) (Write, string) {
	_, x, reason := r.actor(m, uuid.UUID(t.ID).String(), false)
	if reason != "" {
		return Write{}, reason
	}
	label, bonus := "Dexterity (Thieves' Tools)", t.Stats.Initiative
	if action == forcing {
		label, bonus = "Strength (Athletics)", t.Stats.Saves["strength"]
	}
	purpose := map[string]string{disarming: "Disarm the ", picking: "Pick the lock of the ", forcing: "Force the "}[action] + strings.ToLower(o.Name)
	roll := r.request(m, t, purpose+": "+label+" check", "1d20", domain.Modifier{Label: label, Value: bonus})
	w := Write{Kind: domain.ActionTaken, Token: t, taken: &takenAction{action: actions.Utilize, readied: nil}, Rolls: []domain.Roll{roll}}
	w.Pending = &domain.PendingAction{RollID: roll.ID, Actor: t.ID, Action: action, DC: max(1, dc), Object: &o.ID}
	if x != nil {
		w.Combatant = x.ID
	}
	return w, ""
}

// objectChecked settles a check on an object: a disarmed trap is safe and one failed by 5 or more
// springs; a picked or forced lock opens.
func (r *runtime) objectChecked(p domain.PendingAction, total int, actor domain.Member, c caller.Caller) {
	o, ok := r.st.board.Objects[*p.Object]
	if !ok {
		return
	}
	switch p.Action {
	case disarming:
		disarmed, sprung := objects.Disarm(total, p.DC)
		if disarmed {
			o.Armed = false
			r.commit(request{}, objectWrite(domain.ActionTrapDisarmed, o), actor, c)
		}
		if sprung {
			r.springTrap(o, r.st.tokens[p.Actor], actor, c)
		}
	default:
		if objects.Opens(total, p.DC) {
			o.Locked = false
			r.commit(request{}, objectWrite(domain.ActionObjectUnlocked, o), actor, c)
		}
	}
}

// springTrap sets a trap off: it is spent and found, and its Effect lands on whoever set it off, or on
// every creature in its reach.
func (r *runtime) springTrap(o domain.MapObject, by domain.Token, actor domain.Member, c caller.Caller) {
	o.Armed, o.Secret = false, false
	w := objectWrite(domain.ActionTrapSprung, o)
	w.Token = by
	if o.Effect != "" {
		w.trigger = &trigger{effect: o.Effect, at: o.At, radiusFt: o.RadiusFt, user: &by.ID}
	}
	r.commit(request{}, w, actor, c)
}

// approach follows a creature moving: an armed trap whose trigger reach any step entered springs, and
// any party member whose passive Perception reaches a hidden object within 30 feet finds it.
func (r *runtime) approach(w Write, actor domain.Member, c caller.Caller) {
	switch w.Kind {
	case domain.ActionTokenWalked, domain.ActionTokenMoved, domain.ActionTokenPlaced, domain.ActionTeleported:
	default:
		return
	}
	if r.st.board == nil {
		return
	}
	sys := caller.Caller{Subject: c.Subject, Origin: caller.OriginSystem, Client: ""}
	for _, o := range r.st.sortedObjects() {
		if r.st.objectFound(o) {
			found := o
			found.Secret = false
			r.commit(request{}, objectWrite(domain.ActionObjectFound, found), actor, sys)
		}
	}
	mover, ok := r.st.tokens[w.Token.ID]
	if !ok || !standing(mover) {
		return
	}
	steps := append(slices.Clone(w.Path), hex.Coord{Q: mover.Q, R: mover.R})
	for _, o := range r.st.sortedObjects() {
		if !o.Armed || o.Kind != string(objects.Trap) {
			continue
		}
		if slices.ContainsFunc(steps, func(s hex.Coord) bool { return objects.Springs(hex.Distance(s, o.At)*hex.FeetPerHex, o.TriggerFt) }) {
			r.springTrap(r.st.board.Objects[o.ID], mover, actor, sys)
		}
	}
}

// objectFound reports whether a party member's passive Perception finds a hidden object nearby.
func (s *state) objectFound(o domain.MapObject) bool {
	if !o.Secret || o.DetectDC == 0 {
		return false
	}
	for _, t := range s.tokens {
		near := hex.Distance(hex.Coord{Q: t.Q, R: t.R}, o.At)*hex.FeetPerHex <= objects.ApproachFt
		if t.Kind == domain.TokenParty && standing(t) && near && objects.Notices(objects.Passive(t.Stats.Perception), o.DetectDC) {
			return true
		}
	}
	return false
}

func (s *state) sortedObjects() []domain.MapObject {
	out := make([]domain.MapObject, 0, len(s.board.Objects))
	for _, o := range s.board.Objects {
		out = append(out, o)
	}
	slices.SortFunc(out, func(a, b domain.MapObject) int { return strings.Compare(a.Name+a.ID.String(), b.Name+b.ID.String()) })
	return out
}

// carries reports whether a creature's Character holds an item, in any of its Containers and bags.
func (s *state) carries(t domain.Token, slug string) bool {
	owner, ok := characterOf(t)
	if !ok {
		return false
	}
	mine := map[domain.ContainerID]bool{}
	for range s.inventory.Containers {
		for _, c := range s.inventory.Containers {
			if (c.CharacterID != nil && *c.CharacterID == owner) || (c.ParentID != nil && mine[*c.ParentID]) {
				mine[c.ID] = true
			}
		}
	}
	for _, c := range s.inventory.Containers {
		if mine[c.ID] && (c.Items[slug] > 0 || slices.ContainsFunc(c.Instances, func(i domain.Instance) bool { return i.Slug == slug })) {
			return true
		}
	}
	return false
}
