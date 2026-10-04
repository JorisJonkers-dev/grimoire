// Package difficulty holds the difficulty presets a Campaign chooses from, and what each changes.
package difficulty

// The presets.
const (
	Story    = "story"
	Standard = "standard"
	Hard     = "hard"
)

// Preset is how hard a Campaign's enemies are: the share of their hit points they come onto the map
// with, and what is added to their attack rolls.
type Preset struct {
	Slug        string
	Name        string
	Description string
	HPPercent   int
	ToHit       int
}

// Presets lists the presets, easiest first.
func Presets() []Preset {
	return []Preset{
		{Story, "Story", "Enemies come with three quarters of their hit points and attack at −2.", 75, -2},
		{Standard, "Standard", "The rules as written.", 100, 0},
		{Hard, "Hard", "Enemies come with a quarter more hit points and attack at +2.", 125, 2},
	}
}

// Valid reports whether a preset exists.
func Valid(slug string) bool {
	for _, p := range Presets() {
		if p.Slug == slug {
			return true
		}
	}
	return false
}

// Of is a preset by its slug; a preset nobody knows plays by the rules as written.
func Of(slug string) Preset {
	for _, p := range Presets() {
		if p.Slug == slug {
			return p
		}
	}
	return Presets()[1]
}

// Changes reports whether the preset plays differently from the rules as written.
func (p Preset) Changes() bool {
	return p.HPPercent != 100 || p.ToHit != 0
}

// HitPoints is what an enemy with that many hit points comes onto the map with: never fewer than 1.
func (p Preset) HitPoints(hp int) int {
	return max(hp*p.HPPercent/100, min(hp, 1))
}
