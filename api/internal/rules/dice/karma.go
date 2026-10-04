package dice

// Karmic dice smooth a roller's luck on dice the server rolls: after a run of low d20s the next one
// leans high, and after a run of high ones it leans low. A die a player throws is never one of them.
const (
	// KarmaRun is how many of a roller's latest automatic d20s make a run.
	KarmaRun = 2
	// KarmaLow and KarmaHigh are the faces a low run stays at or under and a high run at or over.
	KarmaLow  = 5
	KarmaHigh = 16
)

// Karma is which way a roller's next automatic d20 leans, from its latest automatic d20s, newest
// first: up after a run of low faces, down after a run of high ones, and not at all otherwise.
func Karma(recent []int) int {
	if len(recent) < KarmaRun {
		return 0
	}
	low, high := 0, 0
	for _, face := range recent[:KarmaRun] {
		if face <= KarmaLow {
			low++
		}
		if face >= KarmaHigh {
			high++
		}
	}
	if low == KarmaRun {
		return 1
	}
	if high == KarmaRun {
		return -1
	}
	return 0
}

// KarmicFace rolls one die that leans: it rolls twice and keeps the higher face when it leans up,
// the lower when it leans down, and says which face it let go. A die that does not lean is rolled
// once, exactly as Face rolls it, and lets nothing go.
func KarmicFace(src Source, faces, lean int) (kept, dropped int) {
	first := Face(src, faces)
	if lean > 0 {
		second := Face(src, faces)
		return max(first, second), min(first, second)
	}
	if lean < 0 {
		second := Face(src, faces)
		return min(first, second), max(first, second)
	}
	return first, 0
}
