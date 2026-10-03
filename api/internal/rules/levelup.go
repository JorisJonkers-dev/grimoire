package rules

import (
	"slices"
	"strconv"
	"strings"
)

// MulticlassMinimum is the score each primary ability needs to take a level in another class.
const MulticlassMinimum = 13

// MulticlassUnmet names what a Character lacks to add a level in a class it does not have yet: 13 in the
// primary abilities of every class it has and of the new one. A class that takes any one of them (the
// fighter's Strength or Dexterity) needs only one; otherwise each. Nil when it qualifies or already has
// the class.
func MulticlassUnmet(scores map[Ability]int, classes []Class, target Class) []string {
	if slices.ContainsFunc(classes, func(c Class) bool { return c.Slug == target.Slug }) {
		return nil
	}
	var out []string
	var seen []string
	for _, class := range append(slices.Clone(classes), target) {
		if slices.Contains(seen, class.Slug) {
			continue
		}
		seen = append(seen, class.Slug)
		out = append(out, unmetFor(scores, class)...)
	}
	return out
}

func unmetFor(scores map[Ability]int, class Class) []string {
	need := func(a Ability) string { return abilityName(a) + " " + strconv.Itoa(MulticlassMinimum) + "+" }
	var out, either []string
	for _, a := range class.Primary {
		if class.AnyPrimary && scores[a] >= MulticlassMinimum {
			return nil
		}
		if scores[a] < MulticlassMinimum {
			out = append(out, need(a)+" ("+class.Name+")")
		}
		either = append(either, need(a))
	}
	if class.AnyPrimary && len(either) > 0 {
		return []string{strings.Join(either, " or ") + " (" + class.Name + ")"}
	}
	return out
}

func abilityName(a Ability) string {
	return strings.ToUpper(string(a[:1])) + string(a[1:])
}

// HitPointGain is the hit points a level adds: a roll of the Hit Die, or its fixed average when roll is
// 0, plus the Constitution modifier, and always at least 1.
func HitPointGain(hitDie, conMod, roll int) int {
	if roll == 0 {
		roll = hitDie/2 + 1
	}
	return max(1, roll+conMod)
}

// AbilityCap is the highest an Ability Score Improvement raises a score.
const AbilityCap = 20

// ImproveAbilities applies an Ability Score Improvement, +2 to one ability or +1 to two, none past 20,
// and returns the new scores without changing the old.
func ImproveAbilities(scores map[Ability]int, increase map[Ability]int) (map[Ability]int, error) {
	total := 0
	for a, n := range increase {
		if !a.Valid() || n < 1 || n > 2 {
			return nil, violation("raise one ability by 2 or two abilities by 1")
		}
		total += n
	}
	if total != 2 {
		return nil, violation("raise one ability by 2 or two abilities by 1")
	}
	out := make(map[Ability]int, len(scores))
	for a, s := range scores {
		out[a] = s + increase[a]
		if out[a] > AbilityCap && increase[a] > 0 {
			return nil, violation("%s cannot go past %d", abilityName(a), AbilityCap)
		}
	}
	return out, nil
}

// preparedTables are how many spells each SRD class prepares at each class level (SRD 5.2).
func preparedTables() map[string][20]int {
	full := [20]int{4, 5, 6, 7, 9, 10, 11, 12, 14, 15, 16, 16, 17, 17, 18, 18, 19, 20, 21, 22}
	half := [20]int{2, 3, 4, 5, 6, 6, 7, 7, 9, 9, 10, 10, 11, 11, 12, 12, 14, 14, 15, 15}
	sorcerer := full
	sorcerer[0], sorcerer[1] = 2, 4
	return map[string][20]int{
		"bard": full, "cleric": full, "druid": full, "sorcerer": sorcerer,
		"wizard":  {4, 5, 6, 7, 9, 10, 11, 12, 14, 15, 16, 16, 17, 18, 19, 21, 22, 23, 24, 25},
		"warlock": {2, 3, 4, 5, 6, 7, 8, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15},
		"paladin": half, "ranger": half,
	}
}

// ClassLevel is the levels a Character has in one class, with that class's Hit Die.
type ClassLevel struct {
	Class  Class
	Level  int
	HitDie int
}

// MulticlassResources are the pools of a Character with levels in several classes: one Hit Die per level
// of each class, spell slots from the combined spellcaster level (full casters' levels plus half of each
// half caster's, rounded up) and, beside other spellcasting, Pact Magic slots of their own. A homebrew
// slot table or spell-point pool stays the class's own. One class reads its own table.
func MulticlassResources(classes []ClassLevel) []Resource {
	if len(classes) == 1 {
		c := classes[0]
		return ResourcesAt(c.Class, c.HitDie, c.Level)
	}
	dice := map[int]int{}
	total, casterLevel, pactLevel := 0, 0, 0
	var own []Resource
	for _, c := range classes {
		dice[c.HitDie] += c.Level
		total += c.Level
		switch c.Class.Casting.Kind {
		case FullCaster:
			casterLevel += c.Level
		case HalfCaster:
			casterLevel += (c.Level + 1) / 2
		case PactCaster:
			pactLevel += c.Level
		case CustomSlots, SpellPoints:
			own = append(own, ownPool(c)...)
		case NoCaster:
		}
	}
	out := []Resource{hitDice(dice, total)}
	if casterLevel > 0 {
		for i, n := range fullCasterSlots[min(casterLevel, 20)-1] {
			if n > 0 {
				spell := strconv.Itoa(i + 1)
				out = append(out, Resource{Key: "spell-slots-" + spell, Label: "Level " + spell + " spell slots", Current: n, Max: n})
			}
		}
	}
	if pactLevel > 0 {
		key, label := "spell-slots-", "Level %s spell slots"
		if casterLevel > 0 {
			key, label = "pact-slots-", "Pact Magic slots (level %s)"
		}
		spell, n := strconv.Itoa(min((pactLevel+1)/2, 5)), pactSlots(pactLevel)
		out = append(out, Resource{Key: key + spell, Label: strings.Replace(label, "%s", spell, 1), Current: n, Max: n})
	}
	return append(out, own...)
}

func hitDice(dice map[int]int, total int) Resource {
	sizes := make([]int, 0, len(dice))
	for d := range dice {
		sizes = append(sizes, d)
	}
	slices.Sort(sizes)
	slices.Reverse(sizes)
	parts := make([]string, 0, len(sizes))
	for _, d := range sizes {
		parts = append(parts, strconv.Itoa(dice[d])+"d"+strconv.Itoa(d))
	}
	return Resource{Key: "hit-dice", Label: "Hit Dice (" + strings.Join(parts, ", ") + ")", Current: total, Max: total}
}

// ownPool is a homebrew class's own slots or spell points beside other spellcasting.
func ownPool(c ClassLevel) []Resource {
	if c.Class.Casting.Kind == SpellPoints {
		n := c.Class.Casting.Points[level20(c.Level)-1]
		return []Resource{{Key: c.Class.Slug + "-points", Label: c.Class.Name + " spell points", Current: n, Max: n}}
	}
	var out []Resource
	for i, n := range c.Class.SlotsAt(c.Level) {
		if n > 0 {
			spell := strconv.Itoa(i + 1)
			out = append(out, Resource{Key: c.Class.Slug + "-slots-" + spell, Label: c.Class.Name + " level " + spell + " slots", Current: n, Max: n})
		}
	}
	return out
}

// CheckIncreases checks a retrained set of Ability Score Improvements: two points for each improvement
// taken, none negative, and no score past 20.
func CheckIncreases(scores, increase map[Ability]int, improvements int) error {
	total := 0
	for a, n := range increase {
		if !a.Valid() || n < 0 {
			return violation("raise abilities only, by whole points")
		}
		if scores[a]+n > AbilityCap {
			return violation("%s cannot go past %d", abilityName(a), AbilityCap)
		}
		total += n
	}
	if total != 2*improvements {
		return violation("spend %d points on Ability Score Improvements", 2*improvements)
	}
	return nil
}
