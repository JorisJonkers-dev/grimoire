// Package surface is terrain an effect leaves on hexes: what it does to movement and to creatures in
// it, and how damage turns one surface into another.
package surface

// Kind is a Surface.
type Kind string

// Surfaces. None is bare ground.
const (
	None        Kind = ""
	Fire        Kind = "fire"
	Grease      Kind = "grease"
	Water       Kind = "water"
	Ice         Kind = "ice"
	Web         Kind = "web"
	Electrified Kind = "electrified"
)

// Kinds lists every Surface.
func Kinds() []Kind {
	return []Kind{Fire, Grease, Water, Ice, Web, Electrified}
}

// Valid reports whether a kind is a Surface.
func Valid(k Kind) bool {
	for _, x := range Kinds() {
		if x == k {
			return true
		}
	}
	return false
}

// Difficult reports whether a Surface doubles the cost of moving through it.
func Difficult(k Kind) bool {
	return k == Grease || k == Ice || k == Web
}

// Hazard is the damage a creature takes entering a Surface or starting its turn in it.
func Hazard(k Kind) (dice, damageType string, ok bool) {
	switch k {
	case Fire:
		return "1d4", "fire", true
	case Electrified:
		return "1d4", "lightning", true
	case None, Grease, Water, Ice, Web:
	}
	return "", "", false
}

// React is what damage of a type does to a Surface: grease and web catch fire or burn away, ice
// melts, water freezes or carries lightning, and cold puts fire out.
func React(k Kind, damageType string) Kind {
	switch {
	case damageType == "fire" && k == Grease:
		return Fire
	case damageType == "fire" && k == Web:
		return None
	case damageType == "fire" && k == Ice:
		return Water
	case damageType == "cold" && k == Water:
		return Ice
	case damageType == "cold" && k == Fire:
		return None
	case damageType == "lightning" && k == Water:
		return Electrified
	}
	return k
}
