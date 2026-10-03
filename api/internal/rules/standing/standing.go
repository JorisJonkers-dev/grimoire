// Package standing holds the rules of how a Faction regards the party: the hidden score, the five
// tiers it reads as, and the catalogue of generic Faction Archetypes.
package standing

import "slices"

// Tier is how a Faction regards the party, as the table sees it.
type Tier string

// The five tiers, from worst to best.
const (
	Hostile    Tier = "hostile"
	Unfriendly Tier = "unfriendly"
	Neutral    Tier = "neutral"
	Friendly   Tier = "friendly"
	Allied     Tier = "allied"
)

// Min and Max bound the score a Standing is kept as. Only the DM ever sees it.
const (
	Min = -100
	Max = 100
)

// TierOf reads a score as a tier.
func TierOf(score int) Tier {
	switch {
	case score <= -60:
		return Hostile
	case score <= -20:
		return Unfriendly
	case score < 20:
		return Neutral
	case score < 60:
		return Friendly
	}
	return Allied
}

// Apply moves a score by a Standing Change, never past either end.
func Apply(score, delta int) int {
	return min(Max, max(Min, score+delta))
}

// ValidDelta reports whether a Standing Change moves the score at all, and by no more than its range.
func ValidDelta(delta int) bool {
	return delta != 0 && delta >= Min && delta <= Max
}

// ArchetypeInfo is a generic Faction a DM copies and names.
type ArchetypeInfo struct {
	Slug        string
	Name        string
	Description string
	Goals       string
}

// Archetypes is the catalogue, by slug. None is a named faction of a published setting.
func Archetypes() []ArchetypeInfo {
	return slices.Clone(catalogue)
}

// Archetype finds one by its slug.
func Archetype(slug string) (ArchetypeInfo, bool) {
	i := slices.IndexFunc(catalogue, func(a ArchetypeInfo) bool { return a.Slug == slug })
	if i < 0 {
		return ArchetypeInfo{Slug: "", Name: "", Description: "", Goals: ""}, false
	}
	return catalogue[i], true
}

//nolint:gochecknoglobals // a fixed table
var catalogue = []ArchetypeInfo{
	{"arcane-college", "Arcane college", "Scholars and mages who teach, hoard and argue over magic behind well-warded doors.", "Gather lore and relics, guard dangerous knowledge, and keep magic in learned hands."},
	{"city-watch", "City watch", "The sworn keepers of order in a town or city: patrols, gaolers and the officers over them.", "Keep the peace, enforce the law of the place, and answer to whoever pays their wage."},
	{"cult", "Cult", "A hidden circle bound to a forbidden power, a doomed prophecy or a leader who promises both.", "Grow in secret, serve what they worship, and bring about what it has promised them."},
	{"druid-circle", "Druid circle", "Wardens of a stretch of wild land who speak for its beasts, its trees and its seasons.", "Protect their land from those who would spoil it, and keep the old balance there."},
	{"knightly-order", "Knightly order", "Sworn companions bound by an oath, a code and a long roll of honoured dead.", "Uphold their oath, defend those in their charge, and win renown worthy of the order."},
	{"mercenary-company", "Mercenary company", "Soldiers for hire under a captain and a contract, loyal to their pay and to each other.", "Find paying work, finish what they are paid for, and keep the company whole."},
	{"merchant-league", "Merchant league", "Trading houses in common cause, with caravans, warehouses and a long reach along the roads.", "Open markets, keep the roads safe for trade, and settle disputes in coin rather than blood."},
	{"noble-house", "Noble house", "An old family with land, a name and retainers, and rivals of the same kind.", "Raise the family's standing, hold its lands, and outlast the houses that oppose it."},
	{"smuggling-ring", "Smuggling ring", "Boatmen, carters and fixers who move what the law forbids past those who would tax or seize it.", "Keep their routes open and unseen, and their cargo moving at a profit."},
	{"temple", "Temple", "The clergy and lay folk of a faith, with a house of worship and a place in the life around it.", "Serve their faith, tend to the faithful, and spread or defend what they hold sacred."},
	{"thieves-guild", "Thieves' guild", "Burglars, cutpurses and fences under one roof, with rules about whose streets are whose.", "Take their share of what moves through the city, and keep outsiders off their ground."},
}
