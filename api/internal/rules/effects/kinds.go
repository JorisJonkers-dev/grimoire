package effects

// Kinds is every kind defined in a file of its own, in slug order. A new kind adds its file and one
// line here; the linter bans package globals, so kinds cannot register themselves.
func Kinds() []Kind {
	return []Kind{
		bonusDieKind{},
		branchKind{},
		saveDamageKind{},
	}
}
