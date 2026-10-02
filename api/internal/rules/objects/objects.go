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
)

// Kinds lists every kind.
func Kinds() []Kind {
	return []Kind{Door, Lever, Chest, Barrel, Curtain, Destructible}
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
	case Lever:
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
	case Lever:
	}
	return 19, 5
}

// Hit takes damage off an object's hit points; at 0 it breaks.
func Hit(hp, amount int) (int, bool) {
	left := max(0, hp-max(0, amount))
	return left, left == 0
}
