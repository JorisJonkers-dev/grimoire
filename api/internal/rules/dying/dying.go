// Package dying is what happens at 0 hit points: massive damage that kills outright, death saving
// throws, damage while down, stabilising, waking on healing, and how long revival magic has to work.
package dying

// MassiveDamage reports whether damage that drops a creature to 0 hit points kills it outright: what is
// left after reaching 0 is at least its hit point maximum.
func MassiveDamage(hpBefore, damage, hpMax int) bool {
	return damage-hpBefore >= hpMax
}

// State is a creature at 0 hit points: its death save successes and failures, and whether it is stable
// or dead.
type State struct {
	Successes int
	Failures  int
	Stable    bool
	Dead      bool
}

// Save applies one death saving throw by its natural d20 and its total with bonuses (Bless): a natural
// 20 wakes the creature with 1 hit point (woke), a natural 1 counts as two failures, a total of 10 or
// more succeeds; three successes stabilise (and wipe the count), three failures kill.
func (s State) Save(natural, total int) (State, bool) {
	switch {
	case natural == 20:
		return State{Successes: 0, Failures: 0, Stable: false, Dead: false}, true
	case natural == 1:
		s.Failures += 2
	case total >= 10:
		s.Successes++
	default:
		s.Failures++
	}
	return s.settle(), false
}

// Hurt applies damage taken at 0 hit points: a failure, two from a Critical Hit, and death outright
// when the damage is at least the hit point maximum. A stable creature starts dying again.
func (s State) Hurt(damage, hpMax int, critical bool) State {
	s.Stable = false
	if damage >= hpMax {
		s.Dead = true
		return s
	}
	s.Failures++
	if critical {
		s.Failures++
	}
	return s.settle()
}

func (s State) settle() State {
	switch {
	case s.Failures >= 3:
		s.Failures, s.Dead = 3, true
	case s.Successes >= 3:
		s = State{Successes: 0, Failures: 0, Stable: true, Dead: false}
	}
	return s
}

// Rolls reports whether the creature still makes death saving throws.
func (s State) Rolls() bool {
	return !s.Stable && !s.Dead
}

// StabiliseDC is the Wisdom (Medicine) check that stabilises a dying creature.
const StabiliseDC = 10

// Stabilise makes a dying creature stable, its death saves wiped.
func (s State) Stabilise() State {
	if s.Dead {
		return s
	}
	return State{Successes: 0, Failures: 0, Stable: true, Dead: false}
}

// Spell is revival magic, by how long after death it still works.
type Spell string

// Revival spells.
const (
	Revivify     Spell = "revivify"
	RaiseDead    Spell = "raise_dead"
	Resurrection Spell = "resurrection"
)

// Died is when a creature died: the fight and round it died in (empty outside one) and the game day.
type Died struct {
	Fight string
	Round int
	Day   int
}

// Now is when revival is tried: the fight and round under way (empty outside one) and the game day.
type Now struct {
	Fight string
	Round int
	Day   int
}

// Revives reports whether a spell can still bring back a creature: Revivify within a minute (ten rounds
// of the same fight), Raise Dead within ten days, Resurrection within a century.
func Revives(s Spell, died Died, now Now) bool {
	switch s {
	case Revivify:
		return died.Fight != "" && died.Fight == now.Fight && now.Round-died.Round <= 10
	case RaiseDead:
		return now.Day-died.Day <= 10
	case Resurrection:
		return now.Day-died.Day <= 36500
	}
	return false
}
