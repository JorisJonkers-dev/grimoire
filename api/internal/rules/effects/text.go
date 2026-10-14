package effects

import (
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// Text writes an Effect as rules text, one sentence per part, in the order the parts come: what it
// does, how long it lasts, and how it scales.
func (d Definition) Text() []string {
	out := sentences(d.Components)
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

func sentences(cs []Component) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, sentence(c))
	}
	return out
}

func sentence(c Component) string {
	switch c := c.(type) {
	case Part:
		return c.Text()
	case Manual:
		return c.Instruction
	case Area:
		return areaText(c)
	case SaveCondition:
		return "On a failed " + title(c.Ability) + " saving throw, a creature has the " + title(c.Slug) + " condition."
	case CreateSurface:
		return "The area is covered in " + string(c.Kind) + " for " + plural(c.Rounds, "round") + "."
	case ForcedMove:
		if c.Toward {
			return "A creature that fails is pulled up to " + strconv.Itoa(c.Ft) + " feet straight toward the caster."
		}
		return "A creature that fails is pushed up to " + strconv.Itoa(c.Ft) + " feet straight away from the caster."
	default:
		return lastSentence(c)
	}
}

func lastSentence(c Component) string {
	switch c := c.(type) {
	case Summon:
		return summonText(c)
	case Form:
		return formText(c)
	case Reveal:
		names := make([]string, 0, len(c.Qualities))
		for _, q := range c.Qualities {
			names = append(names, title(q))
		}
		return "Every creature and object in the area loses " + strings.Join(names, " and ") + ", whatever anyone's senses."
	case AreaSave:
		return "Each creature in the area makes a " + title(c.Ability) + " saving throw."
	case Light:
		return "Bright light fills " + strconv.Itoa(c.BrightFt) + " feet around where it lands, and dim light another " + strconv.Itoa(c.DimFt) + " feet."
	default:
		ch, _ := c.(Choice)
		modes := make([]string, 0, len(ch.Modes))
		for _, m := range ch.Modes {
			modes = append(modes, m.Name+": "+strings.Join(sentences(m.Components), " "))
		}
		return "Choose one: " + strings.Join(modes, " Or ")
	}
}

func summonText(s Summon) string {
	who, acts, rolls, it, them := strconv.Itoa(s.Count)+" "+title(s.Monster)+" creatures", "act", "roll their", "They take", "them"
	if s.Count == 1 {
		who, acts, rolls, it, them = article(s.Monster)+" "+title(s.Monster), "acts", "rolls its", "It takes", "it"
	}
	text := "The caster summons " + who + ", which " + rolls + " own Initiative."
	if s.Shares {
		text = "The caster summons " + who + ", which " + acts + " on the caster's turn."
	}
	if s.NeedsCommand {
		text += " " + it + " the Dodge action unless the caster commands " + them + " with a Bonus Action."
	}
	return text
}

func formText(f Form) string {
	what := "a creature of the caster's choice"
	if f.Monster != "" {
		what = article(f.Monster) + " " + title(f.Monster)
	}
	pool := "Temporary Hit Points equal to that creature's Hit Point maximum"
	if f.TempHP > 0 {
		pool = strconv.Itoa(f.TempHP) + " Temporary Hit Points"
	}
	return "The target takes the form of " + what + ", using its statistics and gaining " + pool +
		"; it reverts when they are gone or the effect ends, and any damage left over carries to its own Hit Points."
}

func areaText(a Area) string {
	size := strconv.Itoa(a.SizeFt) + "-foot " + title(string(a.Shape))
	switch {
	case a.Shape == hex.EmanationArea:
		return "A " + size + " surrounds the caster and moves with it."
	case a.RangeFt == 0:
		return "A " + size + " extends from the caster."
	}
	return "A " + size + " appears at a point within " + strconv.Itoa(a.RangeFt) + " feet."
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
