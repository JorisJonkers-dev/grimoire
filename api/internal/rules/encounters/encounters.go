// Package encounters holds the random-encounter rules: the 2024 XP budget, weighted draws from an
// Encounter Table, and filling an Encounter Pool up to a budget.
package encounters

import "github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"

// Difficulty is how hard a generated encounter aims to be.
type Difficulty int

// Difficulties of the 2024 rules.
const (
	Low Difficulty = iota
	Moderate
	High
)

// ParseDifficulty reads a difficulty by name.
func ParseDifficulty(s string) (Difficulty, bool) {
	switch s {
	case "low":
		return Low, true
	case "moderate":
		return Moderate, true
	case "high":
		return High, true
	}
	return Moderate, false
}

// String names the difficulty.
func (d Difficulty) String() string {
	return [...]string{"low", "moderate", "high"}[d]
}

// perCharacter is the 2024 XP budget per character by level (1–20) and difficulty.
func perCharacter() [20][3]int {
	return [20][3]int{
		{50, 75, 100},
		{100, 150, 200},
		{150, 225, 400},
		{250, 375, 500},
		{500, 750, 1100},
		{600, 1000, 1400},
		{750, 1300, 1700},
		{1000, 1700, 2100},
		{1300, 2000, 2600},
		{1600, 2300, 3100},
		{1900, 2900, 4100},
		{2200, 3700, 4700},
		{2600, 4200, 5400},
		{2900, 4900, 6200},
		{3300, 5400, 7800},
		{3800, 6100, 9800},
		{4500, 7200, 11700},
		{5000, 8700, 14200},
		{5500, 10700, 17200},
		{6400, 13200, 22000},
	}
}

// Budget is the XP an encounter may spend against a party of these character levels.
func Budget(levels []int, d Difficulty) int {
	table := perCharacter()
	total := 0
	for _, l := range levels {
		total += table[min(max(l, 1), 20)-1][d]
	}
	return total
}

// Triggered reports whether a percentile roll sets off an encounter at the table's chance.
func Triggered(roll, chancePct int) bool {
	return roll <= chancePct
}

// Draw picks an index with a chance proportional to its weight, or -1 when no weight is positive.
func Draw(src dice.Source, weights []int) int {
	total := 0
	for _, w := range weights {
		total += max(w, 0)
	}
	if total == 0 {
		return -1
	}
	n := src.IntN(total)
	for i := 0; ; i++ {
		w := max(weights[i], 0)
		if n < w {
			return i
		}
		n -= w
	}
}

// Member is a creature an Encounter Pool can field: its XP, how often it is drawn, and how many of it
// an encounter holds at least and at most.
type Member struct {
	Slug   string
	XP     int
	Weight int
	Min    int
	Max    int
}

// Pick is how many of one creature an encounter holds.
type Pick struct {
	Slug  string
	Count int
}

// Fill builds an encounter from a Pool: every member's minimum first, then weighted draws among the
// members still under their maximum that fit what is left of the budget, until none does.
func Fill(src dice.Source, budget int, members []Member) []Pick {
	counts := make([]int, len(members))
	spent := 0
	for i, m := range members {
		counts[i] = m.Min
		spent += m.Min * m.XP
	}
	for {
		weights := make([]int, len(members))
		for i, m := range members {
			if counts[i] < m.Max && spent+m.XP <= budget {
				weights[i] = m.Weight
			}
		}
		i := Draw(src, weights)
		if i < 0 {
			break
		}
		counts[i]++
		spent += members[i].XP
	}
	var out []Pick
	for i, m := range members {
		if counts[i] > 0 {
			out = append(out, Pick{Slug: m.Slug, Count: counts[i]})
		}
	}
	return out
}
