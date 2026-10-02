// Package objects describes Map Objects: what each kind blocks while it stands closed or whole, what
// using one does, and how damage breaks it.
package objects

// Kind is what a Map Object is.
type Kind string

// Map Object kinds.
const (
	Door         Kind = "door"
	Lever        Kind = "lever"
	Chest        Kind = "chest"
	Barrel       Kind = "barrel"
	Curtain      Kind = "curtain"
	Destructible Kind = "destructible"
	Trap         Kind = "trap"
)

// Kinds lists every kind.
func Kinds() []Kind {
	return []Kind{Door, Lever, Chest, Barrel, Curtain, Destructible, Trap}
}

// Valid reports whether a kind exists.
func Valid(k Kind) bool {
	for _, x := range Kinds() {
		if x == k {
			return true
		}
	}
	return false
}

// State is how an object stands: open (or pulled, for a lever) and broken.
type State struct {
	Kind   Kind
	Open   bool
	Broken bool
}

// Blocks reports whether an object stops sight and movement through its hex. A closed door or an
// unbroken destructible stops both; a drawn curtain only sight; a chest or barrel only movement; a
// lever neither. A broken object stops nothing.
func (s State) Blocks() (sight, move bool) {
	if s.Broken {
		return false, false
	}
	switch s.Kind {
	case Door:
		return !s.Open, !s.Open
	case Curtain:
		return !s.Open, false
	case Chest, Barrel:
		return false, true
	case Destructible:
		return true, true
	case Lever, Trap:
	}
	return false, false
}

// Usable reports whether a creature can use the object: open or close it, or pull it.
func (s State) Usable() bool {
	return !s.Broken && (s.Kind == Door || s.Kind == Curtain || s.Kind == Chest || s.Kind == Lever)
}

// Defaults are an object's Armor Class and Hit Points when its author gives none, after the SRD's
// object guidance: wood is AC 15, a lever's iron AC 19, cloth AC 11; hit points follow size.
func Defaults(k Kind) (ac, hp int) {
	switch k {
	case Door, Destructible:
		return 15, 18
	case Chest:
		return 15, 13
	case Barrel:
		return 15, 9
	case Curtain:
		return 11, 2
	case Lever, Trap:
	}
	return 19, 5
}

// Hit takes damage off an object's hit points; at 0 it breaks.
func Hit(hp, amount int) (int, bool) {
	left := max(0, hp-max(0, amount))
	return left, left == 0
}

// Passive is what a passive check scores without a roll: 10 plus the bonus.
func Passive(bonus int) int {
	return 10 + bonus
}

// Notices reports whether a creature's passive Perception finds a hidden object of a detection DC.
// An object without a DC is found only by looking for it.
func Notices(passivePerception, dc int) bool {
	return dc > 0 && passivePerception >= dc
}

// ApproachFt is how close a creature comes before its passive Perception can find a hidden object.
const ApproachFt = 30

// Springs reports whether a creature this far from an armed trap sets it off.
func Springs(distanceFt, triggerFt int) bool {
	return distanceFt <= triggerFt
}

// SpringMargin is how far a failed disarm must fall short to set the trap off.
const SpringMargin = 5

// Disarm reads a disarm check: the trap is made safe on a success, and set off on a failure by 5 or more.
func Disarm(total, dc int) (disarmed, sprung bool) {
	if total >= dc {
		return true, false
	}
	return false, dc-total >= SpringMargin
}

// Opens reports whether a check gets through a lock: picking it with thieves' tools, or forcing it.
func Opens(total, dc int) bool {
	return total >= dc
}

// KnockRangeFt is how far the Knock spell reaches.
const KnockRangeFt = 60
