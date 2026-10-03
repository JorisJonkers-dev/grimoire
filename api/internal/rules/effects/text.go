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
	case BonusDie:
		return "Add " + c.Dice + " to " + rolls(c.On) + "."
	case Edge:
		return edgeText(c)
	case ExtraDamage:
		return "The source deals an extra " + c.Dice + " damage whenever it hits the target."
	case MoveCost:
		return "Every foot of movement costs " + strconv.Itoa(c.Multiplier) + " feet."
	case Manual:
		return c.Instruction
	case Area:
		return areaText(c)
	case SaveDamage:
		text := "Each creature in the area makes a " + title(c.Ability) + " saving throw, taking " + c.Dice + " " + c.Type + " damage on a failed save"
		if c.Half {
			return text + " or half as much damage on a successful one."
		}
		return text + "."
	case SaveCondition:
		return "On a failed " + title(c.Ability) + " saving throw, a creature has the " + title(c.Slug) + " condition."
	case CreateSurface:
		return "The area is covered in " + string(c.Kind) + " for " + plural(c.Rounds, "round") + "."
	default:
		return moreSentence(c)
	}
}

func moreSentence(c Component) string {
	switch c := c.(type) {
	case Incapacitated:
		return "The target has the Incapacitated condition."
	case Immobile:
		return "The target's Speed is 0."
	case SaveEdge:
		return saveEdgeText(c)
	case CritWithin:
		return "Any hit against the target from within " + strconv.Itoa(c.Feet) + " feet is a Critical Hit."
	case Exhausting:
		return stackingText(c)
	case SpeedPenalty:
		return "The target's Speed is reduced by " + strconv.Itoa(c.Ft) + " feet."
	case Reacts:
		return "When the target is " + c.Trigger + ", it can take a Reaction: " + c.Instruction
	case TempHP:
		return "The target gains " + strconv.Itoa(c.Amount) + " Temporary Hit Points."
	case Teleport:
		return "The caster teleports up to " + strconv.Itoa(c.RangeFt) + " feet to an unoccupied space it can see."
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
	case Dispel:
		return "Every spell on the target ends."
	case Counter:
		return "As a Reaction, the caster interrupts a creature it can see casting a spell within " + strconv.Itoa(c.RangeFt) + " feet; the spell fails."
	case GrantFeature:
		return "The target gains " + c.Name + "."
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
	case ResourceChange:
		if c.Delta > 0 {
			return "The target regains " + plural(c.Delta, "use") + " of " + title(c.Resource) + "."
		}
		return "The target expends " + plural(-c.Delta, "use") + " of " + title(c.Resource) + "."
	case AreaSave:
		return "Each creature in the area makes a " + title(c.Ability) + " saving throw."
	case Light:
		return "Bright light fills " + strconv.Itoa(c.BrightFt) + " feet around where it lands, and dim light another " + strconv.Itoa(c.DimFt) + " feet."
	case Choice:
		modes := make([]string, 0, len(c.Modes))
		for _, m := range c.Modes {
			modes = append(modes, m.Name+": "+strings.Join(sentences(m.Components), " "))
		}
		return "Choose one: " + strings.Join(modes, " Or ")
	default:
		b, _ := c.(Branch)
		return b.When.Text() + ", " + lower(strings.Join(sentences(b.Then), " "))
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

func rolls(on []Roll) string {
	names := make([]string, 0, len(on))
	for _, r := range on {
		names = append(names, map[Roll]string{AttackRolls: "attack rolls", SavingThrows: "saving throws"}[r])
	}
	return strings.Join(names, " and ")
}

func edgeText(e Edge) string {
	kind := map[bool]string{true: "Advantage", false: "Disadvantage"}[e.Advantage]
	text := "The target has " + kind + " on attack rolls"
	if e.Against {
		text = "Attack rolls against the target have " + kind
		if e.SourceOnly {
			text = "The source's attack rolls against the target have " + kind
		}
	}
	switch e.Range {
	case WithinFive:
		text += " from within 5 feet"
	case BeyondFive:
		text += " from more than 5 feet away"
	case AnyRange:
	}
	return text + "."
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

func saveEdgeText(s SaveEdge) string {
	switch s.Mode {
	case SaveFails:
		return "The target automatically fails " + title(s.Ability) + " saving throws."
	case SaveAdvantage:
		return "The target has Advantage on " + title(s.Ability) + " saving throws."
	case SaveDisadvantage:
	}
	return "The target has Disadvantage on " + title(s.Ability) + " saving throws."
}

// Text writes a Branch's condition as the start of a sentence.
func (c Condition) Text() string {
	switch c.Kind {
	case FailsBy:
		return "If a creature fails the save by " + strconv.Itoa(c.N) + " or more"
	case HPAtMost:
		return "If a creature has " + strconv.Itoa(c.N) + " Hit Points or fewer"
	case FirstEachTurn:
		return "The first time on a turn the effect touches a creature"
	case CreatureIs:
	}
	return "If the creature is " + article(c.Type) + " " + c.Type
}

// Text writes a Duration as a rules line; empty when the Effect leaves its length open.
func (d Duration) Text(concentration bool) string {
	length := map[DurationKind]string{
		Instant: "Instantaneous", Rounds: plural(d.Amount, "round"), Minutes: plural(d.Amount, "minute"), Hours: plural(d.Amount, "hour"),
		UntilDispelled: "Until dispelled", UntilRest: "Until the target finishes a Short or Long Rest", Permanent: "Permanent",
		EndOfNextTurn: "Until the end of the target's next turn",
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

// stackingText reads a stacking Effect: what each level takes, when it kills, and how high it rises.
func stackingText(c Exhausting) string {
	var each []string
	if c.D20PerLevel > 0 {
		each = append(each, "gives a −"+strconv.Itoa(c.D20PerLevel)+" penalty to D20 Tests")
	}
	if c.SpeedFtPerLevel > 0 {
		each = append(each, "reduces Speed by "+strconv.Itoa(c.SpeedFtPerLevel)+" feet")
	}
	text := "It stacks"
	if len(each) > 0 {
		text = "Each level " + strings.Join(each, " and ")
	}
	if c.DeathAt > 0 {
		text += "; a creature dies at level " + strconv.Itoa(c.DeathAt)
	}
	if c.MaxLevel > 0 {
		text += "; it rises to level " + strconv.Itoa(c.MaxLevel) + " at most"
	}
	return text + "."
}
