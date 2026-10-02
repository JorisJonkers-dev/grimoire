package live

import (
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/objects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// objectBlocks reports whether a Map Object in a hex stops sight and movement.
func (s *state) objectBlocks(c hex.Coord) (bool, bool) {
	sight, move := false, false
	for _, o := range s.board.Objects {
		if o.At == c {
			sees, moves := objects.State{Kind: objects.Kind(o.Kind), Open: o.Open, Broken: o.Broken}.Blocks()
			sight, move = sight || sees, move || moves
		}
	}
	return sight, move
}

// trigger is an Effect a Map Object sets off: on whoever used it, or on every creature within RadiusFt.
type trigger struct {
	effect   string
	at       hex.Coord
	radiusFt int
	user     *domain.TokenID
}

// planObject places, removes, damages or finds a Map Object for the DM.
func (r *runtime) planObject(cmd Command) (Write, string) {
	if r.st.board == nil {
		return Write{}, "Map Objects need a map."
	}
	if cmd.Kind == CmdPlaceObject {
		return r.placeObject(cmd)
	}
	id, _ := uuid.Parse(cmd.ObjectID)
	o, ok := r.st.board.Objects[id]
	if !ok {
		return Write{}, "No such object."
	}
	switch cmd.Kind {
	case CmdRemoveObject:
		return Write{Kind: domain.ActionObjectRemoved, Object: o.ID}, ""
	case CmdFindObject:
		o.Secret = false
		return objectWrite(domain.ActionObjectFound, o), ""
	}
	return r.damageObject(o, cmd.HPDelta)
}

func objectWrite(kind string, changed ...domain.MapObject) Write {
	w := Write{Kind: kind, changedObjects: map[domain.ObjectID]domain.MapObject{}}
	for _, o := range changed {
		w.changedObjects[o.ID] = o
		w.Objects = append(w.Objects, o.ID)
	}
	return w
}

// placeObject puts a Map Object on a hex of the map, with the SRD's Armor Class and hit points for its
// kind unless others are given.
func (r *runtime) placeObject(cmd Command) (Write, string) {
	kind := objects.Kind(cmd.ObjectKind)
	at := hex.Coord{Q: cmd.Q, R: cmd.R}
	name := strings.TrimSpace(cmd.ObjectName)
	if name == "" {
		name = strings.ToUpper(string(kind)[:min(1, len(kind))]) + string(kind)[min(1, len(kind)):]
	}
	ac, hp := objects.Defaults(kind)
	if cmd.ArmorClass > 0 {
		ac = cmd.ArmorClass
	}
	if cmd.HPMax > 0 {
		hp = cmd.HPMax
	}
	switch {
	case !objects.Valid(kind):
		return Write{}, "Choose a door, lever, chest, barrel, curtain, destructible object or trap."
	case !r.st.onBoard(at):
		return Write{}, "That hex is off the map."
	case len([]rune(name)) > 40 || ac > 30 || hp > 1000:
		return Write{}, "Name it in 40 characters; Armor Class goes up to 30, hit points to 1000."
	case cmd.RadiusFt < 0 || cmd.RadiusFt > 60 || len(cmd.Effect) > 80 || cmd.TriggerFt < 0 || cmd.TriggerFt > 60:
		return Write{}, "A trigger reaches up to 60 feet."
	case min(cmd.DetectDC, cmd.DisarmDC, cmd.LockDC) < 0 || max(cmd.DetectDC, cmd.DisarmDC, cmd.LockDC) > 40 || len(cmd.Key) > 80:
		return Write{}, "DCs run from 1 to 40."
	}
	o := domain.MapObject{
		ID: uuid.New(), Kind: string(kind), Name: name, At: at, AC: ac, HP: hp, HPMax: hp, Secret: cmd.Secret,
		Effect: strings.ToLower(strings.TrimSpace(cmd.Effect)), RadiusFt: cmd.RadiusFt,
		Armed: kind == objects.Trap, DetectDC: cmd.DetectDC, DisarmDC: cmd.DisarmDC, TriggerFt: cmd.TriggerFt,
		Locked: cmd.LockDC > 0, LockDC: cmd.LockDC, Key: strings.ToLower(strings.TrimSpace(cmd.Key)),
	}
	for _, l := range cmd.Links {
		id, _ := uuid.Parse(l)
		if _, ok := r.st.board.Objects[id]; !ok {
			return Write{}, "Link only to objects on this map."
		}
		o.Links = append(o.Links, id)
	}
	return objectWrite(domain.ActionObjectPlaced, o), ""
}

// damageObject takes hit points off an object, or mends it with a positive change; at 0 it breaks and
// sets off its trigger.
func (r *runtime) damageObject(o domain.MapObject, delta int) (Write, string) {
	if delta == 0 {
		return Write{}, "Damage or mend the object by at least 1 hit point."
	}
	if delta > 0 {
		o.HP = min(o.HPMax, o.HP+delta)
		o.Broken = false
		return objectWrite(domain.ActionObjectDamaged, o), ""
	}
	wasWhole := !o.Broken
	left, broken := objects.Hit(o.HP, -delta)
	o.HP, o.Broken = left, o.Broken || broken
	w := objectWrite(domain.ActionObjectDamaged, o)
	if wasWhole && o.Broken && o.Effect != "" {
		w.trigger = &trigger{effect: o.Effect, at: o.At, radiusFt: o.RadiusFt, user: nil}
	}
	return w, ""
}

// planUse has a creature next to a Map Object open or close it, or pull it. A lever works every object
// it is linked to. In a fight it takes the turn's free object interaction.
func (r *runtime) planUse(m domain.Member, cmd Command) (Write, string) {
	t, ok := r.st.tokenByID(cmd.TokenID)
	id, _ := uuid.Parse(cmd.ObjectID)
	var o domain.MapObject
	known := false
	if r.st.board != nil {
		o, known = r.st.board.Objects[id]
	}
	switch {
	case !ok || t.Stats == nil:
		return Write{}, "No such creature."
	case !m.DM && (t.Controller == nil || *t.Controller != m.ID):
		return Write{}, "That token is not yours to play."
	case !known || (!m.DM && !r.st.objectShown(o, r.st.vision())):
		return Write{}, "No such object."
	case !(objects.State{Kind: objects.Kind(o.Kind), Open: o.Open, Broken: o.Broken}).Usable():
		return Write{}, "The " + strings.ToLower(o.Name) + " cannot be used."
	case o.Locked:
		return Write{}, "The " + strings.ToLower(o.Name) + " is locked."
	case hex.Distance(hex.Coord{Q: t.Q, R: t.R}, o.At) > 1:
		return Write{}, t.Label + " must stand next to the " + strings.ToLower(o.Name) + "."
	case r.st.catalog.Incapacitated(r.st.actives(t.ID)):
		return Write{}, t.Label + " can't act while Incapacitated."
	}
	o.Open = !o.Open
	changed := []domain.MapObject{o}
	for _, l := range o.Links {
		if linked, ok := r.st.board.Objects[l]; ok && !linked.Broken {
			linked.Open = !linked.Open
			changed = append(changed, linked)
		}
	}
	w := objectWrite(domain.ActionObjectToggled, changed...)
	w.Token = t
	if o.Effect != "" {
		w.trigger = &trigger{effect: o.Effect, at: o.At, radiusFt: o.RadiusFt, user: &t.ID}
	}
	return w, r.st.interacting(&w, t)
}

// interacting takes the turn's free object interaction for a write in a fight; it says why it cannot.
func (s *state) interacting(w *Write, t domain.Token) string {
	c := s.combat
	if c == nil || c.Status != domain.CombatActive {
		return ""
	}
	x, acting := s.combatantOf(t.ID)
	switch {
	case !acting:
		return "It is not " + t.Label + "'s turn."
	case !x.Economy.Interaction:
		return t.Label + " has used their free object interaction; another takes the Utilize action."
	}
	w.Combatant = x.ID
	return ""
}

// applyObject changes the Map Objects and spends a free object interaction on a use in a fight.
func applyObject(s *state, w *Write) {
	if w.Kind == domain.ActionObjectRemoved {
		delete(s.board.Objects, w.Object)
		for id, o := range s.board.Objects {
			if slices.Contains(o.Links, w.Object) {
				o.Links = slices.DeleteFunc(slices.Clone(o.Links), func(l domain.ObjectID) bool { return l == w.Object })
				s.board.Objects[id] = o
				w.Objects = append(w.Objects, id)
			}
		}
		return
	}
	if s.board.Objects == nil {
		s.board.Objects = map[domain.ObjectID]domain.MapObject{}
	}
	for id, o := range w.changedObjects {
		s.board.Objects[id] = o
	}
	if w.taken != nil {
		applyAction(s, w)
	}
	if w.Kind == domain.ActionObjectToggled && w.Combatant != (domain.CombatantID{}) {
		applyInteraction(s, w)
	}
}

// fire sets off a Map Object's trigger: its Effect on whoever used it, or on every creature within its
// radius, each as if the DM applied it.
func (r *runtime) fire(t *trigger, actor domain.Member, c caller.Caller) {
	var targets []domain.TokenID
	if t.radiusFt == 0 && t.user != nil {
		targets = append(targets, *t.user)
	}
	if t.radiusFt > 0 {
		for _, tok := range r.st.tokens {
			if tok.Stats != nil && hex.Distance(t.at, hex.Coord{Q: tok.Q, R: tok.R})*hex.FeetPerHex <= t.radiusFt {
				targets = append(targets, tok.ID)
			}
		}
	}
	slices.SortFunc(targets, func(a, b domain.TokenID) int { return strings.Compare(r.st.tokens[a].Label, r.st.tokens[b].Label) })
	sys := caller.Caller{Subject: c.Subject, Origin: caller.OriginSystem, Client: ""}
	for _, id := range targets {
		w, reason := r.planApply(Command{Kind: CmdApplyEffect, TargetID: uuid.UUID(id).String(), Effect: t.effect})
		if reason == "" {
			r.commit(request{}, w, actor, sys)
		}
	}
}

// objectShown reports whether the party knows a Map Object: found, and in a hex it sees or remembers.
func (s *state) objectShown(o domain.MapObject, seen map[hex.Coord]bool) bool {
	return !o.Secret && (seen[o.At] || s.board.Reveals[o.At])
}

// ObjectView is a Map Object as an audience sees it; its numbers and secrecy go to the DM only.
type ObjectView struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Q        int    `json:"q"`
	R        int    `json:"r"`
	Open     bool   `json:"open"`
	Broken   bool   `json:"broken"`
	Secret   bool   `json:"secret,omitempty"`
	AC       *int   `json:"ac,omitempty"`
	HP       *int   `json:"hp,omitempty"`
	HPMax    *int   `json:"hpMax,omitempty"`
	Effect   string `json:"effect,omitempty"`
	RadiusFt int    `json:"radiusFt,omitempty"`
	// Locked shows to everyone who knows the object; the trap and lock numbers go to the DM only.
	Locked    bool   `json:"locked,omitempty"`
	Armed     bool   `json:"armed,omitempty"`
	DetectDC  int    `json:"detectDc,omitempty"`
	DisarmDC  int    `json:"disarmDc,omitempty"`
	TriggerFt int    `json:"triggerFt,omitempty"`
	LockDC    int    `json:"lockDc,omitempty"`
	Key       string `json:"key,omitempty"`
}

// objectViews lists the Map Objects an audience knows.
func (s *state) objectViews(a Audience, seen map[hex.Coord]bool) []ObjectView {
	var out []ObjectView
	for _, o := range s.board.Objects {
		if a != AudienceDM && !s.objectShown(o, seen) {
			continue
		}
		v := ObjectView{ID: o.ID.String(), Kind: o.Kind, Name: o.Name, Q: o.At.Q, R: o.At.R, Open: o.Open, Broken: o.Broken, Locked: o.Locked}
		if a == AudienceDM {
			ac, hp, most := o.AC, o.HP, o.HPMax
			v.AC, v.HP, v.HPMax, v.Secret, v.Effect, v.RadiusFt = &ac, &hp, &most, o.Secret, o.Effect, o.RadiusFt
			v.Armed, v.DetectDC, v.DisarmDC, v.TriggerFt, v.LockDC, v.Key = o.Armed, o.DetectDC, o.DisarmDC, o.TriggerFt, o.LockDC, o.Key
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b ObjectView) int { return strings.Compare(a.Name+a.ID, b.Name+b.ID) })
	return out
}
