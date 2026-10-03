package live

import (
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/clock"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/inventory"
)

// maxGameDay is the last day the Game Clock counts to.
const maxGameDay = 1_000_000

// gameTime is the time on the Campaign's Game Clock.
func (s *state) gameTime() clock.Time {
	return clock.Time{Day: s.day, Minute: s.minute}
}

// pass moves the Game Clock to a time, with what every dawn passed on the way gives back.
func (r *runtime) pass(w *Write, to clock.Time) {
	day := to.Day
	w.Day, w.Minute = &day, to.Minute
	w.Dawned = r.dawned(clock.Dawns(r.st.gameTime(), to))
}

// dawned rolls what some dawns give back to every charged item the Campaign holds whose charges come
// back at dawn.
func (r *runtime) dawned(dawns int) []domain.Recharge {
	var out []domain.Recharge
	if dawns == 0 {
		return out
	}
	for _, c := range r.st.inventory.Containers {
		for _, in := range c.Instances {
			if held, changed := r.atDawn(in, dawns); changed {
				out = append(out, domain.Recharge{Container: c.ID, Instance: in.ID, Charges: held})
			}
		}
	}
	return out
}

// atDawn is the charges an Item Instance holds after some dawns, rolled once for each, and whether
// that is more than it held.
func (r *runtime) atDawn(in domain.Instance, dawns int) (int, bool) {
	info := r.st.itemInfo(in.Slug)
	if info.MaxCharges == 0 || in.Charges == nil {
		return 0, false
	}
	schedule := inventory.Charges{Max: info.MaxCharges, Dice: info.RegainDice, Faces: info.RegainFaces, Bonus: info.RegainBonus, On: info.RechargeOn}
	held := *in.Charges
	// A dawn that gives anything gives at least one charge, so no more dawns than the item holds matter.
	for range min(dawns, schedule.Max) {
		src, rolled := r.source(r.seed()), 0
		for range schedule.Dice {
			rolled += dice.Face(src, schedule.Faces)
		}
		held = schedule.AtDawn(held, rolled)
	}
	return held, held != *in.Charges
}

// planClock is the DM setting the Game Clock: forward, with every dawn on the way, or back.
func (r *runtime) planClock(cmd Command) (Write, string) {
	if cmd.GameDay < 0 || cmd.GameDay > maxGameDay || cmd.GameMinute < 0 || cmd.GameMinute >= clock.DayMinutes {
		return Write{}, "The Game Clock runs from day 0 to 1000000, by the minutes of each day."
	}
	w := Write{Kind: domain.ActionClockSet}
	r.pass(&w, clock.Time{Day: cmd.GameDay, Minute: cmd.GameMinute})
	return w, ""
}
