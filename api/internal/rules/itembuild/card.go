package itembuild

import (
	"strconv"
	"strings"
)

// Card is an item as players will read it: a heading line, then one line per part. Hidden properties
// show only when known is true, once the item is identified or attuned.
func Card(name string, d Design, known bool) []string {
	head := words(d.Kind) + ", " + words(d.Rarity)
	if a := d.Attunement; a != nil {
		head += " (requires attunement"
		if a.Kind != "" {
			head += " by a " + a.Value
		}
		head += ")"
	}
	title := name
	if d.Enchantment > 0 {
		title += " +" + strconv.Itoa(d.Enchantment)
	}
	out := []string{title, head}
	if d.Enchantment > 0 {
		out = append(out, "You have a +"+strconv.Itoa(d.Enchantment)+" bonus to attack and damage rolls made with this magic item.")
	}
	if w := d.Weapon; w != nil {
		out = append(out, weaponLine(w))
	}
	if c := d.Charges; c != nil {
		out = append(out, chargesLine(c))
	}
	for _, p := range d.Properties {
		if p.Hidden && !known {
			continue
		}
		out = append(out, p.line())
	}
	return out
}

func weaponLine(w *Weapon) string {
	text := "Properties: " + strings.Join(w.Properties, ", ")
	switch {
	case w.Mastery == "custom":
		text += ". Mastery: " + w.Custom
	case w.Mastery != "":
		text += ". Mastery: " + w.Mastery
	}
	return text + "."
}

func chargesLine(c *Charges) string {
	when := map[string]string{"dawn": "daily at dawn", "long_rest": "on a long rest", "short_rest": "on a short rest"}[c.On]
	back := "all its expended charges"
	if c.Dice > 0 {
		back = strconv.Itoa(c.Dice) + "d" + strconv.Itoa(c.Faces)
		if c.Bonus > 0 {
			back += " + " + strconv.Itoa(c.Bonus)
		}
		back += " expended charges"
	}
	return "It has " + strconv.Itoa(c.Max) + " charges and regains " + back + " " + when + "."
}

func (p Property) line() string {
	switch p.Type {
	case SkillBoost:
		return boostLine(p)
	case Bonus:
		return "You gain a +" + strconv.Itoa(p.Value) + " bonus to " + words(p.Target) + "."
	case Resistance:
		return "You have Resistance to " + p.Damage + " damage."
	case ExtraDamage:
		return "Hits with it deal an extra " + p.Dice + " " + p.Damage + " damage."
	case SenseRow:
		return "You have " + p.Sense + " out to " + strconv.Itoa(p.Feet) + " feet."
	case SpeedRow:
		return "You have a " + p.Speed + " speed of " + strconv.Itoa(p.Feet) + " feet."
	case Cantrip:
		return "You can cast " + p.Name + " at will."
	case SpellRow:
		return spellLine(p)
	case LightRow:
		return "It sheds bright light in a " + strconv.Itoa(p.BrightFt) + "-foot radius and dim light for an additional " + strconv.Itoa(p.DimFt) + " feet."
	default:
		return moreLine(p)
	}
}

func boostLine(p Property) string {
	skill := skillName(p.Skill)
	switch p.Mode {
	case "advantage":
		return "You have Advantage on " + skill + " checks."
	case "d4":
		return "You add 1d4 to " + skill + " checks."
	case "flat":
		return "You gain a +" + strconv.Itoa(p.Value) + " bonus to " + skill + " checks."
	case "proficiency":
		return "You have proficiency in " + skill + "."
	}
	return "You have Expertise in " + skill + "."
}

func spellLine(p Property) string {
	if p.Cost == 0 {
		return "You can cast " + p.Name + " from it at will."
	}
	return "You can expend " + strconv.Itoa(p.Cost) + " " + plural(p.Cost, "charge") + " to cast " + p.Name + " from it."
}

func moreLine(p Property) string {
	switch p.Type {
	case Consumable:
		return map[string]string{
			"single": "It is used up when used.", "long_rest": "Once used, it can't be used again until the next long rest.",
			"coating": "Applied to a weapon, it lasts for " + strconv.Itoa(p.Hits) + " " + plural(p.Hits, "hit") + ".",
		}[p.Uses]
	case Container:
		text := "It holds up to " + strconv.Itoa(p.CapacityLb) + " pounds"
		if p.OnlyKind != "" {
			text += " of " + words(p.OnlyKind) + " items"
		}
		if p.Weightless {
			text += " and weighs the same however full it is"
		}
		return text + "."
	case Firearm:
		return "Firearm: misfires on a " + strconv.Itoa(p.Misfire) + " or lower, reloads after " + strconv.Itoa(p.Reload) + " shots, fires bursts of " + strconv.Itoa(p.Burst) + "."
	case Curse:
		if p.CannotDrop {
			return "Curse. " + p.Text + " You can't remove it while attuned unless the curse is broken."
		}
		return "Curse. " + p.Text
	case Sentient:
		return "Sentience. " + p.Text
	case Growth:
		return "At level " + strconv.Itoa(p.AtLevel) + ": " + p.Text
	case SetBonus:
		return p.Set + " (" + strconv.Itoa(p.Pieces) + " pieces): " + p.Text
	}
	return p.Text
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// skillName spells a skill slug as the rules write it: sleight-of-hand is Sleight of Hand.
func skillName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, w := range parts {
		if w != "of" && w != "" {
			parts[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(parts, " ")
}
