package pgstore

import "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"

func intPtr(n int) *int { return &n }

// AddSampleEntries adds one of every entry kind to a snapshot with srd-2024 and srd-2014 documents.
func AddSampleEntries(s *snapshot.Snapshot) {
	e := func(doc, slug, name, desc string) snapshot.Entry {
		return snapshot.Entry{Document: doc, Slug: slug, Name: name, Description: desc}
	}
	s.Classes = []snapshot.Class{
		{
			Entry: e("srd-2024", "fighter", "Fighter", "A master of arms."), HitDie: 10, CasterType: "none",
			SavingThrows: []string{"strength", "constitution"},
			Features: []snapshot.ClassFeature{
				{Slug: "second-wind", Name: "Second Wind", Description: "Regain hit points.", Levels: []int{1}},
				{Slug: "table", Name: "Fighter Table", Description: "Levels.", Levels: []int{}},
			},
		},
		{Entry: e("srd-2024", "champion", "Champion", ""), Parent: "fighter", CasterType: "third", SavingThrows: []string{}, Features: []snapshot.ClassFeature{}},
		{Entry: e("srd-2014", "fighter", "Fighter", "Old fighter."), HitDie: 10, SavingThrows: []string{}, Features: []snapshot.ClassFeature{}},
	}
	s.Species = []snapshot.Species{{Entry: e("srd-2024", "dwarf", "Dwarf", "Stout."), Traits: []snapshot.Named{{Name: "Darkvision", Description: "See in the dark."}}}}
	s.Backgrounds = []snapshot.Background{{Entry: e("srd-2024", "sage", "Sage", "Scholar."), Benefits: []snapshot.Named{{Name: "Skills", Description: "Arcana."}}}}
	s.Feats = []snapshot.Feat{
		{Entry: e("srd-2024", "grappler", "Grappler", ""), Type: "General", Prerequisite: "Level 4+", Benefits: []string{"Knock a target prone.", "Hold on."}},
		{Entry: e("srd-2014", "alert", "Alert", "Wary."), Benefits: []string{}},
	}
	s.Weapons = []snapshot.Weapon{
		{
			Entry: e("srd-2024", "longbow", "Longbow", ""), DamageDice: "1d8", DamageType: "fire", RangeFeet: 150, LongRangeFeet: 600,
			Properties: []snapshot.WeaponProperty{{Name: "Ammunition", Detail: "arrow"}, {Name: "Heavy"}, {Name: "Slow", Mastery: true}},
		},
		{Entry: e("srd-2024", "club", "Club", ""), DamageDice: "1d4", Simple: true, Properties: []snapshot.WeaponProperty{}},
	}
	s.Armor = []snapshot.Armor{
		{Entry: e("srd-2024", "plate-armor", "Plate Armor", ""), Category: "heavy", ACBase: 18, StealthDisadvantage: true, StrengthRequired: intPtr(15)},
		{Entry: e("srd-2024", "half-plate", "Half Plate", ""), Category: "medium", ACBase: 15, AddDex: true, DexCap: intPtr(2)},
		{Entry: e("srd-2024", "leather", "Leather", ""), Category: "light", ACBase: 11, AddDex: true},
	}
	s.Items = []snapshot.Item{
		{Entry: e("srd-2024", "rope", "Rope", "Hemp."), CostGP: 1, WeightLB: 5},
		{
			Entry: e("srd-2024", "bag-of-holding", "Bag of Holding", "Holds a lot."), Category: "wondrous-item", Magic: true,
			Rarity: "uncommon", RequiresAttunement: true, AttunementDetail: "by a wizard",
		},
	}
	s.Monsters = []snapshot.Monster{{
		Entry: e("srd-2024", "goblin", "Goblin", ""), Size: "small", Type: "humanoid", Alignment: "chaotic neutral", ArmorClass: 15,
		ArmorDetail: "leather", HitPoints: 7, HitDice: "2d6", ChallengeRating: 0.25, XP: 50,
		Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Saves:     map[string]int{"dexterity": 4}, Skills: map[string]int{"stealth": 6}, Speeds: map[string]int{"walk": 30},
		Senses: map[string]int{"darkvision": 60}, PassivePerception: 9, Languages: "Common, Goblin",
		Resistances: []string{"fire"}, Immunities: []string{"poison"}, Vulnerabilities: []string{"cold"}, ConditionImmunities: []string{"charmed"},
		Traits: []snapshot.Named{{Name: "Nimble Escape", Description: "Disengage as a bonus action."}},
		Actions: []snapshot.Action{
			{Name: "Scimitar", Description: "Slash. The target is knocked prone.", Type: "action", Attacks: []snapshot.Attack{
				{Name: "Scimitar", Kind: "weapon", ToHit: 4, ReachFeet: 5, DamageDice: "1d6", DamageBonus: 2, DamageType: "slashing", ExtraDice: "1d4", ExtraType: "poison"},
			}},
			{Name: "Hide", Description: "Hides.", Type: "bonus_action", Attacks: []snapshot.Attack{}},
		},
	}}
}
