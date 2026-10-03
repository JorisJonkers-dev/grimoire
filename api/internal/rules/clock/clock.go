// Package clock holds the rules of the Game Clock: how in-world time moves on, when dawn comes and
// how long a journey takes.
package clock

const (
	// DayMinutes is how long a game day is.
	DayMinutes = 24 * 60
	// DawnMinute is when dawn comes: six in the morning.
	DawnMinute = 6 * 60
	// ShortRest and LongRest are how long each rest takes, in minutes.
	ShortRest = 60
	LongRest  = 8 * 60
	// campMinutes is the night's camp between two days of travel.
	campMinutes = 16 * 60
)

// Time is a moment on a Campaign's Game Clock: its game day and the minutes after midnight.
type Time struct {
	Day    int
	Minute int
}

func (t Time) minutes() int {
	return t.Day*DayMinutes + t.Minute
}

// Add moves the time on by some minutes, or back by a negative number; never back past the start of
// the first day.
func (t Time) Add(minutes int) Time {
	total := max(0, t.minutes()+minutes)
	return Time{Day: total / DayMinutes, Minute: total % DayMinutes}
}

// Reached reports whether a moment has come: this time is at it or after it.
func (t Time) Reached(when Time) bool {
	return t.minutes() >= when.minutes()
}

// Dawns counts the dawns between two times: each one after from, up to and including to.
func Dawns(from, to Time) int {
	// The dawns up to a moment: one for every day whose sixth hour it has reached.
	upTo := func(t Time) int { return (t.minutes() - DawnMinute + DayMinutes) / DayMinutes }
	return max(0, upTo(to)-upTo(from))
}

// Journey is how long a journey takes on the clock: its minutes on the road, and a night's camp
// between its travel days.
func Journey(minutes, days int) int {
	return minutes + max(0, days-1)*campMinutes
}
