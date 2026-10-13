package kindproto

// Kinds is every kind the engine knows. This sorted list is the only shared line a new kind adds.
func Kinds() []Kind {
	return []Kind{
		areaKind{},
		bonusDieKind{},
		choiceKind{},
		healKind{},
		saveDamageKind{},
	}
}
