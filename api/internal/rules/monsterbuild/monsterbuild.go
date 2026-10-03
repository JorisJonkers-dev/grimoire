// Package monsterbuild turns a homebrew creature's design, its statistics, traits, actions with
// multiattack and recharge, legendary actions and resistance, lair actions, regional effects, mythic
// phases, auras, swarms and damage thresholds, into the stat block play reads, its rules text, and an
// estimated Challenge.
package monsterbuild

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
)

// DesignError is why a design cannot be built, fit to show its author.
type DesignError string

func (e DesignError) Error() string { return string(e) }

// Design is a homebrew creature as its author builds it. Challenge is the author's own rating.
type Design struct {
	Size            string         `json:"size"`
	CreatureType    string         `json:"creatureType"`
	AC              int            `json:"ac"`
	HP              int            `json:"hp"`
	SpeedFt         int            `json:"speedFt"`
	Challenge       float64        `json:"challenge"`
	Abilities       map[string]int `json:"abilities"`
	Saves           []string       `json:"saves"`
	Senses          []Measure      `json:"senses"`
	Resistances     []string       `json:"resistances"`
	Immunities      []string       `json:"immunities"`
	Vulnerabilities []string       `json:"vulnerabilities"`
	Threshold       int            `json:"threshold"`
	Swarm           bool           `json:"swarm,omitempty"`
	Traits          []Trait        `json:"traits"`
	Aura            *Aura          `json:"aura,omitempty"`
	Multiattack     int            `json:"multiattack"`
	Actions         []Action       `json:"actions"`
	Legendary       *Legendary     `json:"legendary,omitempty"`
	Lair            *Lair          `json:"lair,omitempty"`
	Phases          []Phase        `json:"phases"`
}

// Measure is a sense and its range.
type Measure struct {
	Kind string `json:"kind"`
	Feet int    `json:"feet"`
}

// Trait is a named rule of the creature's.
type Trait struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// Aura is an effect around the creature, out to Feet.
type Aura struct {
	Name string `json:"name"`
	Feet int    `json:"feet"`
	Text string `json:"text"`
}

// Action is something the creature does with its action: a melee or ranged attack, a save it forces,
// or anything else as text. Recharge is the lowest d6 that brings it back, 0 when it always can.
type Action struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	ToHit       int    `json:"toHit,omitempty"`
	ReachFt     int    `json:"reachFt,omitempty"`
	RangeFt     int    `json:"rangeFt,omitempty"`
	LongRangeFt int    `json:"longRangeFt,omitempty"`
	Damage      string `json:"damage,omitempty"`
	DamageBonus int    `json:"damageBonus,omitempty"`
	DamageType  string `json:"damageType,omitempty"`
	SaveAbility string `json:"saveAbility,omitempty"`
	DC          int    `json:"dc,omitempty"`
	Recharge    int    `json:"recharge,omitempty"`
	Text        string `json:"text,omitempty"`
}

// Legendary is how many legendary actions the creature takes a round, how many Legendary Resistances it
// has a day, and the actions it chooses from.
type Legendary struct {
	Uses       int               `json:"uses"`
	Resistance int               `json:"resistance"`
	Actions    []LegendaryAction `json:"actions"`
}

// LegendaryAction costs some of the round's legendary actions.
type LegendaryAction struct {
	Name string `json:"name"`
	Cost int    `json:"cost"`
	Text string `json:"text"`
}

// Lair is what the creature's lair does on initiative count 20, and its regional effects.
type Lair struct {
	Actions  []Trait  `json:"actions"`
	Regional []string `json:"regional"`
}

// Phase is a mythic form the creature takes when it drops to 0 hit points, with fresh hit points.
type Phase struct {
	Name string `json:"name"`
	HP   int    `json:"hp"`
	Text string `json:"text"`
}

// Attack is an attack on the creature's hotbar.
type Attack struct {
	Name        string
	ToHit       int
	ReachFt     int
	RangeFt     int
	LongRangeFt int
	Damage      string
	DamageBonus int
	DamageType  string
}

// Monster is a built design: the stat block play reads, and the design it reads back from.
type Monster struct {
	Slug             string
	Name             string
	AC               int
	HP               int
	SpeedFt          int
	Proficiency      int
	Saves            map[string]int
	Senses           map[string]int
	Attacks          []Attack
	AttacksPerAction int
	Initiative       int
	Perception       int
	Stealth          int
	Threshold        int
	Legendary        *Legendary
	Lair             *Lair
	Phases           []Phase
	Design           Design
}

var dicePattern = regexp.MustCompile(`^[1-9][0-9]?d(4|6|8|10|12|20)$`)

func abilities() []string {
	return []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}
}

func sizes() []string { return []string{"tiny", "small", "medium", "large", "huge", "gargantuan"} }

func creatureTypes() []string {
	return []string{"aberration", "beast", "celestial", "construct", "dragon", "elemental", "fey", "fiend", "giant", "humanoid", "monstrosity", "ooze", "plant", "undead"}
}

func damageTypes() []string {
	return []string{"acid", "bludgeoning", "cold", "fire", "force", "lightning", "necrotic", "piercing", "poison", "psychic", "radiant", "slashing", "thunder"}
}

func challenges() []float64 {
	out := []float64{0, 0.125, 0.25, 0.5}
	for cr := range 30 {
		out = append(out, float64(cr+1))
	}
	return out
}

func sized(s string, most int) bool {
	n := len([]rune(strings.TrimSpace(s)))
	return n > 0 && n <= most
}

func within(n, lo, hi int) bool { return n >= lo && n <= hi }

// Check validates a design.
func Check(d Design) error {
	for _, check := range []func(Design) error{body, defences, traits, actions, legend} {
		if err := check(d); err != nil {
			return err
		}
	}
	return nil
}

func body(d Design) error {
	switch {
	case !slices.Contains(sizes(), d.Size):
		return DesignError("choose a size: " + strings.Join(sizes(), ", "))
	case !slices.Contains(creatureTypes(), d.CreatureType):
		return DesignError("choose a creature type: " + strings.Join(creatureTypes(), ", "))
	case !within(d.AC, 5, 30):
		return DesignError("give it an AC of 5 to 30")
	case !within(d.HP, 1, 1000):
		return DesignError("give it 1 to 1000 hit points")
	case !within(d.SpeedFt, 0, 120):
		return DesignError("give it a Speed of 0 to 120 feet")
	case !slices.Contains(challenges(), d.Challenge):
		return DesignError("rate its Challenge from 0 to 30")
	case len(d.Abilities) != 6 || slices.ContainsFunc(abilities(), func(a string) bool { _, ok := d.Abilities[a]; return !ok }):
		return DesignError("give it all six abilities")
	}
	for _, a := range abilities() {
		if !within(d.Abilities[a], 1, 30) {
			return DesignError("score each ability from 1 to 30")
		}
	}
	if slices.ContainsFunc(d.Saves, func(s string) bool { return !slices.Contains(abilities(), s) }) {
		return DesignError("choose its proficient saves from the six abilities")
	}
	return nil
}

func defences(d Design) error {
	for _, s := range d.Senses {
		switch {
		case !slices.Contains([]string{"darkvision", "blindsight", "tremorsense", "truesight"}, s.Kind):
			return DesignError("give it darkvision, blindsight, tremorsense or truesight")
		case !within(s.Feet, 5, 300):
			return DesignError("make each sense 5 to 300 feet")
		}
	}
	for _, list := range [][]string{d.Resistances, d.Immunities, d.Vulnerabilities} {
		if slices.ContainsFunc(list, func(t string) bool { return !slices.Contains(damageTypes(), t) }) {
			return DesignError("name real damage types: " + strings.Join(damageTypes(), ", "))
		}
	}
	if !within(d.Threshold, 0, 50) {
		return DesignError("give it a damage threshold of 0 to 50")
	}
	return nil
}

func traits(d Design) error {
	if len(d.Traits) > 20 {
		return DesignError("keep it to up to 20 traits")
	}
	for _, t := range d.Traits {
		if !sized(t.Name, 60) || len([]rune(t.Text)) > 2000 {
			return DesignError("name each trait in up to 60 characters, and describe it in up to 2000")
		}
	}
	if a := d.Aura; a != nil && (!within(a.Feet, 5, 60) || !sized(a.Name, 60) || !sized(a.Text, 2000)) {
		return DesignError("give it an aura of 5 to 60 feet, named and described")
	}
	if d.Multiattack != 0 && !within(d.Multiattack, 2, 4) {
		return DesignError("make 0 or 2 to 4 attacks with Multiattack")
	}
	return nil
}

func actions(d Design) error {
	if len(d.Actions) < 1 || len(d.Actions) > 20 {
		return DesignError("give it 1 to 20 actions")
	}
	for _, a := range d.Actions {
		if err := action(a); err != nil {
			return err
		}
	}
	return nil
}

func action(a Action) error {
	switch {
	case !slices.Contains([]string{"melee", "ranged", "save", "special"}, a.Kind):
		return DesignError("make each action a melee, ranged, save or special one")
	case !sized(a.Name, 60):
		return DesignError("name each action in up to 60 characters")
	case a.Recharge != 0 && !within(a.Recharge, 4, 6):
		return DesignError("recharge on 4, 5 or 6, or not at all")
	case len([]rune(a.Text)) > 2000:
		return DesignError("describe " + a.Name + " in up to 2000 characters")
	}
	switch a.Kind {
	case "melee", "ranged":
		return attack(a)
	case "save":
		if !slices.Contains(abilities(), a.SaveAbility) || !within(a.DC, 8, 30) {
			return DesignError("give " + a.Name + " the save and its DC, 8 to 30")
		}
		return damageOf(a, true)
	}
	if strings.TrimSpace(a.Text) == "" {
		return DesignError("describe " + a.Name)
	}
	return nil
}

func attack(a Action) error {
	switch {
	case !within(a.ToHit, -5, 20):
		return DesignError("give each attack a to-hit bonus of −5 to +20")
	case a.Kind == "melee" && !within(a.ReachFt, 5, 30):
		return DesignError("give each melee attack a reach of 5 to 30 feet")
	case a.Kind == "ranged" && !within(a.RangeFt, 5, 600):
		return DesignError("give each ranged attack a range of 5 to 600 feet")
	case a.Kind == "ranged" && a.LongRangeFt != 0 && !within(a.LongRangeFt, a.RangeFt, 1200):
		return DesignError("make its long range no shorter than its range")
	}
	return damageOf(a, false)
}

func damageOf(a Action, optional bool) error {
	switch {
	case optional && a.Damage == "":
		return nil
	case !dicePattern.MatchString(a.Damage) || !within(a.DamageBonus, -5, 30):
		return DesignError("give " + a.Name + " damage dice like 2d6 and a bonus of −5 to +30")
	case !slices.Contains(damageTypes(), a.DamageType):
		return DesignError("give " + a.Name + " a damage type")
	}
	return nil
}

func legend(d Design) error {
	for _, check := range []func(Design) error{legendary, lair, phases} {
		if err := check(d); err != nil {
			return err
		}
	}
	return nil
}

func legendary(d Design) error {
	l := d.Legendary
	switch {
	case l == nil:
		return nil
	case !within(l.Uses, 1, 5):
		return DesignError("give it 1 to 5 legendary actions a round")
	case !within(l.Resistance, 0, 5):
		return DesignError("give it 0 to 5 Legendary Resistances a day")
	case len(l.Actions) < 1 || len(l.Actions) > 10:
		return DesignError("give it 1 to 10 legendary actions to choose from")
	}
	for _, a := range l.Actions {
		if !within(a.Cost, 1, 3) {
			return DesignError("make each legendary action cost 1 to 3")
		}
		if !sized(a.Name, 60) || !sized(a.Text, 2000) {
			return DesignError("name and describe each legendary action")
		}
	}
	return nil
}

func lair(d Design) error {
	if d.Lair == nil {
		return nil
	}
	for _, a := range d.Lair.Actions {
		if !sized(a.Name, 60) || !sized(a.Text, 2000) {
			return DesignError("name and describe each lair action")
		}
	}
	for _, r := range d.Lair.Regional {
		if !sized(r, 2000) {
			return DesignError("describe each regional effect in up to 2000 characters")
		}
	}
	return nil
}

func phases(d Design) error {
	if len(d.Phases) > 3 {
		return DesignError("give it up to 3 phases")
	}
	for _, p := range d.Phases {
		if !within(p.HP, 1, 1000) {
			return DesignError("give each phase 1 to 1000 hit points")
		}
		if !sized(p.Name, 60) || len([]rune(p.Text)) > 2000 {
			return DesignError("name each phase, and describe it in up to 2000 characters")
		}
	}
	return nil
}

// Compile builds a checked design into the creature a slug names.
func Compile(slug, name string, d Design) Monster {
	pb := rules.ProficiencyByChallenge(d.Challenge)
	mod := func(a string) int { return rules.Modifier(d.Abilities[a]) }
	m := Monster{
		Slug: slug, Name: name, AC: d.AC, HP: d.HP, SpeedFt: d.SpeedFt, Proficiency: pb, Saves: map[string]int{}, Senses: map[string]int{},
		Attacks: []Attack{}, AttacksPerAction: max(1, d.Multiattack), Initiative: mod("dexterity"), Perception: mod("wisdom"), Stealth: mod("dexterity"),
		Threshold: d.Threshold, Legendary: d.Legendary, Lair: d.Lair, Phases: d.Phases, Design: d,
	}
	for _, a := range abilities() {
		m.Saves[a] = mod(a)
		if slices.Contains(d.Saves, a) {
			m.Saves[a] += pb
		}
	}
	for _, s := range d.Senses {
		m.Senses[s.Kind] = s.Feet
	}
	for _, a := range d.Actions {
		if a.Kind == "melee" || a.Kind == "ranged" {
			m.Attacks = append(m.Attacks, Attack{
				Name: a.Name, ToHit: a.ToHit, ReachFt: a.ReachFt, RangeFt: a.RangeFt, LongRangeFt: a.LongRangeFt, Damage: a.Damage, DamageBonus: a.DamageBonus, DamageType: a.DamageType,
			})
		}
	}
	return m
}

// average is a dice expression's average roll plus a bonus.
func average(dice string, bonus int) float64 {
	if dice == "" {
		return 0
	}
	count, faces, _ := strings.Cut(dice, "d")
	n, _ := strconv.Atoi(count)
	f, _ := strconv.Atoi(faces)
	return float64(n)*float64(f+1)/2 + float64(bonus)
}

// Estimate is a rough Challenge from the stat block: a defensive rating from its effective hit points
// and AC, an offensive one from its best round of damage and its best attack bonus or save DC, averaged,
// and one more for legendary actions. It is a guide for the author, not a rule.
func Estimate(m Monster) string {
	d := m.Design
	hp := float64(d.HP)
	for _, p := range d.Phases {
		hp += float64(p.HP)
	}
	hp *= min(1.25, 1+0.05*float64(len(d.Resistances)+2*len(d.Immunities)))
	defence := hp/15 + float64(d.AC-13)/4
	best, hit := 0.0, 5
	for _, a := range d.Actions {
		switch a.Kind {
		case "melee", "ranged":
			best, hit = max(best, average(a.Damage, a.DamageBonus)*float64(m.AttacksPerAction)), max(hit, a.ToHit)
		case "save":
			once := average(a.Damage, a.DamageBonus)
			if a.Recharge > 0 {
				once /= 2
			}
			best, hit = max(best, once), max(hit, a.DC-8)
		}
	}
	raw := (defence + best/6 + float64(hit-5)/4) / 2
	if d.Legendary != nil {
		raw++
	}
	switch {
	case raw < 0.0625:
		return "0"
	case raw < 0.1875:
		return "1/8"
	case raw < 0.375:
		return "1/4"
	case raw < 0.75:
		return "1/2"
	}
	return strconv.Itoa(min(30, int(math.Round(raw))))
}

func challengeText(cr float64) string {
	switch cr {
	case 0.125:
		return "1/8"
	case 0.25:
		return "1/4"
	case 0.5:
		return "1/2"
	}
	return strconv.Itoa(int(cr))
}

func title(s string) string { return strings.ToUpper(s[:1]) + s[1:] }

func signed(n int) string {
	if n < 0 {
		return "−" + strconv.Itoa(-n)
	}
	return "+" + strconv.Itoa(n)
}

func titles(xs []string) string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, title(x))
	}
	return strings.Join(out, ", ")
}

// Lines are the creature as its stat block reads.
func Lines(m Monster) []string {
	d := m.Design
	out := []string{title(d.Size) + " " + title(d.CreatureType), "AC " + strconv.Itoa(d.AC) + " · HP " + strconv.Itoa(d.HP) + " · Speed " + strconv.Itoa(d.SpeedFt) + " ft."}
	scores := make([]string, 0, 6)
	for _, a := range abilities() {
		scores = append(scores, strings.ToUpper(a[:3])+" "+strconv.Itoa(d.Abilities[a])+" ("+signed(rules.Modifier(d.Abilities[a]))+")")
	}
	out = append(out, strings.Join(scores, " · "))
	out = append(out, defenceLines(m)...)
	out = append(out, "Challenge "+challengeText(d.Challenge)+" (estimated "+Estimate(m)+") · Proficiency Bonus +"+strconv.Itoa(m.Proficiency))
	return append(out, actionLines(m)...)
}

func defenceLines(m Monster) []string {
	d := m.Design
	var out []string
	if len(d.Saves) > 0 {
		saves := make([]string, 0, len(d.Saves))
		for _, s := range d.Saves {
			saves = append(saves, title(s)+" "+signed(m.Saves[s]))
		}
		out = append(out, "Saving Throws: "+strings.Join(saves, ", "))
	}
	if len(d.Senses) > 0 {
		senses := make([]string, 0, len(d.Senses))
		for _, s := range d.Senses {
			senses = append(senses, title(s.Kind)+" "+strconv.Itoa(s.Feet)+" ft.")
		}
		out = append(out, "Senses: "+strings.Join(senses, ", "))
	}
	for _, l := range []struct {
		label string
		list  []string
	}{{"Resistances", d.Resistances}, {"Immunities", d.Immunities}, {"Vulnerabilities", d.Vulnerabilities}} {
		if len(l.list) > 0 {
			out = append(out, l.label+": "+titles(l.list))
		}
	}
	if d.Threshold > 0 {
		out = append(out, "Damage Threshold "+strconv.Itoa(d.Threshold))
	}
	return out
}

func actionLines(m Monster) []string {
	d := m.Design
	var out []string
	if d.Swarm {
		out = append(out, "Swarm. It can occupy another creature's space and vice versa, and it can move through any opening large enough for a Tiny creature.")
	}
	for _, t := range d.Traits {
		out = append(out, t.Name+". "+t.Text)
	}
	if a := d.Aura; a != nil {
		out = append(out, a.Name+" (aura, "+strconv.Itoa(a.Feet)+" ft.). "+a.Text)
	}
	if d.Multiattack > 0 {
		out = append(out, "Multiattack. It makes "+strconv.Itoa(d.Multiattack)+" attacks.")
	}
	for _, a := range d.Actions {
		out = append(out, actionLine(a))
	}
	out = append(out, legendLines(d)...)
	for i, p := range d.Phases {
		out = append(out, "Phase "+strconv.Itoa(i+2)+": "+p.Name+" ("+strconv.Itoa(p.HP)+" HP). "+p.Text)
	}
	return out
}

func legendLines(d Design) []string {
	var out []string
	if l := d.Legendary; l != nil {
		out = append(out, "Legendary Actions ("+strconv.Itoa(l.Uses)+" a round). Legendary Resistance "+strconv.Itoa(l.Resistance)+" a day.")
		for _, a := range l.Actions {
			name := a.Name
			if a.Cost > 1 {
				name += " (costs " + strconv.Itoa(a.Cost) + ")"
			}
			out = append(out, name+". "+a.Text)
		}
	}
	if l := d.Lair; l != nil {
		for _, a := range l.Actions {
			out = append(out, "Lair Actions (initiative 20). "+a.Name+". "+a.Text)
		}
		for _, r := range l.Regional {
			out = append(out, "Regional Effect. "+r)
		}
	}
	return out
}

func actionLine(a Action) string {
	name := a.Name
	if a.Recharge > 0 {
		name += " (Recharge " + strconv.Itoa(a.Recharge) + "–6)"
		if a.Recharge == 6 {
			name = a.Name + " (Recharge 6)"
		}
	}
	hit := a.Damage
	if a.DamageBonus != 0 {
		hit += " " + strings.Replace(signed(a.DamageBonus), "+", "+ ", 1)
	}
	hit = strings.Replace(hit, "−", "− ", 1)
	text := strings.TrimSpace(" " + a.Text)
	switch a.Kind {
	case "melee":
		return strings.TrimSpace(name + ". Melee Attack Roll: " + signed(a.ToHit) + ", reach " + strconv.Itoa(a.ReachFt) + " ft. Hit: " + hit + " " + a.DamageType + " damage. " + text)
	case "ranged":
		reach := strconv.Itoa(a.RangeFt)
		if a.LongRangeFt > 0 {
			reach += "/" + strconv.Itoa(a.LongRangeFt)
		}
		return strings.TrimSpace(name + ". Ranged Attack Roll: " + signed(a.ToHit) + ", range " + reach + " ft. Hit: " + hit + " " + a.DamageType + " damage. " + text)
	case "save":
		line := name + ". " + title(a.SaveAbility) + " Saving Throw: DC " + strconv.Itoa(a.DC) + "."
		if a.Damage != "" {
			line += " " + hit + " " + a.DamageType + " damage."
		}
		return strings.TrimSpace(line + " " + text)
	}
	return name + ". " + text
}
