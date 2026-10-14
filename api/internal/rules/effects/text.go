package effects

import (
	"strconv"
	"strings"
)

// Text writes an Effect as rules text, one sentence per part, in the order the parts come: what it
// does, how long it lasts, and how it scales.
func (d Definition) Text() []string {
	out := sentences(d.Parts)
	if line := d.Duration.Text(d.Concentration); line != "" {
		out = append(out, line)
	}
	if d.Duration.RepeatSave != "" {
		out = append(out, "The target repeats the "+title(d.Duration.RepeatSave)+" saving throw at the end of each of its turns, ending the effect on a success.")
	}
	if d.Scaling != nil {
		out = append(out, d.Scaling.Text())
	}
	return out
}

func sentences(cs []Part) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Text())
	}
	return out
}

// Text writes a Duration as a rules line; empty when the Effect leaves its length open.
func (d Duration) Text(concentration bool) string {
	length := map[DurationKind]string{
		Instant: "Instantaneous", Rounds: plural(d.Amount, "round"), Minutes: plural(d.Amount, "minute"), Hours: plural(d.Amount, "hour"),
		UntilDispelled: "Until dispelled", UntilRest: "Until the target finishes a Short or Long Rest", Permanent: "Permanent",
		EndOfNextTurn: "Until the end of the target's next turn", UntilCured: "Until cured",
	}[d.Kind]
	switch {
	case length == "":
		return ""
	case concentration:
		return "Duration: Concentration, up to " + length + "."
	}
	return "Duration: " + length + "."
}

// Text writes a Scaling as a rules line.
func (s Scaling) Text() string {
	switch s.Axis {
	case SlotLevel:
		return "Using a Higher-Level Spell Slot: the damage increases by " + s.Dice + " for each slot level above " + strconv.Itoa(s.Base) + "."
	case CharacterLevel:
		return "Cantrip Upgrade: the damage becomes " + steps(s.Steps, "level") + "."
	case ClassLevel:
		return "The damage becomes " + steps(s.Steps, title(s.Class)+" level") + "."
	case TableColumn:
	}
	return "The damage equals the " + s.Column + " column of the " + title(s.Class) + " table."
}

func steps(st []Step, at string) string {
	out := make([]string, 0, len(st))
	for _, s := range st {
		out = append(out, s.Dice+" at "+at+" "+strconv.Itoa(s.At))
	}
	return strings.Join(out, ", ")
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.Itoa(n) + " " + unit + "s"
}

// title capitalises each word of a slug or name: hunters-mark becomes Hunters Mark.
func title(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == ' ' })
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func lower(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func article(word string) string {
	if word != "" && strings.ContainsRune("aeiouAEIOU", rune(word[0])) {
		return "an"
	}
	return "a"
}
