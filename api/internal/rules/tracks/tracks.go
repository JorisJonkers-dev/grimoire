// Package tracks holds the rules of Tracks: Campaign-specific scores such as sanity or renown, kept
// within bounds, with thresholds that are crossed as the score moves.
package tracks

import "slices"

// Threshold is a score at which something happens: on the way up when it is rising, on the way down
// otherwise.
type Threshold struct {
	At     int
	Rising bool
}

// Clamp keeps a score within a Track's bounds.
func Clamp(lo, hi, score int) int {
	return max(lo, min(hi, score))
}

// Crossed lists which thresholds, by their place in the list, a score crosses moving from one value to
// another, nearest first: a rising one it reaches or passes on the way up, a falling one on the way down.
func Crossed(thresholds []Threshold, from, to int) []int {
	var out []int
	for i, th := range thresholds {
		up := th.Rising && from < th.At && th.At <= to
		down := !th.Rising && from > th.At && th.At >= to
		if up || down {
			out = append(out, i)
		}
	}
	// Nearest first: the one the score comes to soonest.
	far := func(i int) int { return max(thresholds[i].At-from, from-thresholds[i].At) }
	slices.SortStableFunc(out, func(a, b int) int { return far(a) - far(b) })
	return out
}
