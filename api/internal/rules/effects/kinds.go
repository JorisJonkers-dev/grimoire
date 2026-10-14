package effects

// Kinds is every kind of Part, each defined in a file of its own (kind_<slug>.go), in slug order. A new kind adds its
// file and one line here; the linter bans package globals, so kinds cannot register themselves.
func Kinds() []Kind {
	return []Kind{
		areaKind{},
		areaSaveKind{},
		bonusDieKind{},
		branchKind{},
		choiceKind{},
		counterKind{},
		createSurfaceKind{},
		critWithinKind{},
		dispelKind{},
		edgeKind{},
		exhaustingKind{},
		extraDamageKind{},
		forcedMoveKind{},
		formKind{},
		grantFeatureKind{},
		immobileKind{},
		incapacitatedKind{},
		lightKind{},
		manualKind{},
		moveCostKind{},
		reactsKind{},
		resourceChangeKind{},
		revealKind{},
		saveConditionKind{},
		saveDamageKind{},
		saveEdgeKind{},
		speedPenaltyKind{},
		summonKind{},
		teleportKind{},
		tempHPKind{},
	}
}
