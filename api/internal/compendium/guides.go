package compendium

import "github.com/JorisJonkers-dev/grimoire/api/internal/rules/guides"

// GuideSpell is a spell as the spells-by-level guide lists it.
type GuideSpell struct {
	Slug   string
	Name   string
	School string
}

// GuideLevel is the spells of one spell level, by name.
type GuideLevel struct {
	Level  int
	Spells []GuideSpell
}

// GuideChallenge sums up the monsters of one Challenge Rating and their attacks.
type GuideChallenge struct {
	Challenge string
	Monsters  int
	Band      guides.Band
}

// GuideItem is a magic item as the loot guide lists it.
type GuideItem struct {
	Slug string
	Name string
}

// GuideRarity is the magic items of one rarity, by name, and the first tier of play they suit.
type GuideRarity struct {
	Rarity    string
	FirstTier int
	Items     []GuideItem
}

// Guides are the compendium's guides, worked out of its SRD entries: spells by level, the attacks of
// monsters by Challenge Rating, and loot by tier of play.
type Guides struct {
	Spells     []GuideLevel
	Challenges []GuideChallenge
	Tiers      []guides.Tier
	Rarities   []GuideRarity
}
