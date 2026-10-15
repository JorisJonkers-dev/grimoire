package effects

// PartsIn are the Parts that count for an Effect applied in a mode: its own, with each choice
// replaced by the chosen mode's. PartsIn beneath a Holder count only once it lands.
func (d Definition) PartsIn(mode string) []Part {
	return parts(d.Parts, mode)
}

func parts(cs []Part, mode string) []Part {
	var out []Part
	for _, c := range cs {
		if ch, ok := c.(Chooser); ok {
			out = append(out, parts(ch.Chosen(mode), mode)...)
			continue
		}
		if _, ok := c.(Holder); !ok {
			out = append(out, c)
		}
	}
	return out
}

// Modes lists the names of an Effect's modes, empty when it offers no choice.
func (d Definition) Modes() []string {
	var out []string
	for _, c := range d.Parts {
		if ch, ok := c.(Chooser); ok {
			for _, g := range ch.Groups() {
				out = append(out, g.Name)
			}
		}
	}
	return out
}

// Branches lists the Parts an Effect adds for a creature in a situation, in order.
func (d Definition) Branches(mode string, s Situation) []Part {
	return branches(d.Parts, mode, s)
}

func branches(cs []Part, mode string, s Situation) []Part {
	var out []Part
	for _, c := range cs {
		if h, ok := c.(Holder); ok {
			then := h.Landed(s)
			out = append(out, parts(then, mode)...)
			out = append(out, branches(then, mode, s)...)
		}
		if ch, ok := c.(Chooser); ok {
			out = append(out, branches(ch.Chosen(mode), mode, s)...)
		}
	}
	return out
}

// walk calls visit on every Part of an Effect, in modes and branches too.
func walk(cs []Part, visit func(Part)) {
	for _, c := range cs {
		visit(c)
		if g, ok := c.(Grouper); ok {
			for _, group := range g.Groups() {
				walk(group.Parts, visit)
			}
		}
	}
}

// DurationKind is how long an Effect lasts.
type DurationKind string

// Duration kinds.
const (
	Instant        DurationKind = "instant"
	Rounds         DurationKind = "rounds"
	Minutes        DurationKind = "minutes"
	Hours          DurationKind = "hours"
	UntilDispelled DurationKind = "until_dispelled"
	UntilRest      DurationKind = "until_rest"
	Permanent      DurationKind = "permanent"
	EndOfNextTurn  DurationKind = "end_of_next_turn"
	// UntilCured is a lingering injury: it lasts until its own cure is applied.
	UntilCured DurationKind = "until_cured"
)

// Duration is how long an Effect lasts, and the save its bearer repeats to end it early. The zero
// Duration leaves the length to whoever applies the Effect.
type Duration struct {
	Kind   DurationKind
	Amount int
	// RepeatSave is the ability the bearer saves with to end the Effect, at the end of each of its turns.
	RepeatSave string
}

// RoundsPerMinute is how many six-second rounds make a minute.
const RoundsPerMinute = 10

// Rounds is how many rounds an Effect lasts; zero when its end is not counted in rounds.
func (d Duration) Rounds() int {
	switch d.Kind {
	case Rounds:
		return d.Amount
	case Minutes:
		return d.Amount * RoundsPerMinute
	case Hours:
		return d.Amount * RoundsPerMinute * 60
	case EndOfNextTurn:
		return 1
	case Instant, UntilDispelled, UntilRest, Permanent, UntilCured:
	}
	return 0
}

// EndsOnRest reports whether finishing a rest ends the Effect.
func (d Duration) EndsOnRest() bool {
	return d.Kind == UntilRest
}

// Lingers reports whether the Effect lasts until its own cure: no rest ends it, and it goes with its
// bearer from one Session to the next.
func (d Duration) Lingers() bool {
	return d.Kind == UntilCured
}
