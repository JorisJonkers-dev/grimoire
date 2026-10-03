package monsterbuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/monsterbuild"
)

// bogKing is a legendary marsh tyrant with a lair, a second phase and a damage threshold.
func bogKing() monsterbuild.Design {
	return monsterbuild.Design{
		Size: "large", CreatureType: "monstrosity", AC: 16, HP: 120, SpeedFt: 30, Challenge: 8,
		Abilities: map[string]int{"strength": 20, "dexterity": 12, "constitution": 18, "intelligence": 8, "wisdom": 14, "charisma": 16},
		Saves:     []string{"constitution", "wisdom"}, Senses: []monsterbuild.Measure{{Kind: "darkvision", Feet: 60}},
		Resistances: []string{"poison"}, Immunities: []string{"acid"}, Vulnerabilities: []string{"fire"}, Threshold: 5, Swarm: false,
		Traits:      []monsterbuild.Trait{{Name: "Bog Stride", Text: "Mud costs it no extra movement."}},
		Aura:        &monsterbuild.Aura{Name: "Stench", Feet: 10, Text: "A creature starting its turn here is Poisoned."},
		Multiattack: 2,
		Actions: []monsterbuild.Action{
			{Name: "Claw", Kind: "melee", ToHit: 8, ReachFt: 10, Damage: "2d8", DamageBonus: 5, DamageType: "slashing"},
			{Name: "Mud Bolt", Kind: "ranged", ToHit: 6, RangeFt: 60, LongRangeFt: 120, Damage: "3d6", DamageType: "acid"},
			{Name: "Bog Breath", Kind: "save", SaveAbility: "constitution", DC: 15, Damage: "6d6", DamageType: "poison", Recharge: 5, Text: "15-foot cone; half damage on a success."},
			{Name: "Croak", Kind: "special", Text: "Every frog within a mile answers."},
		},
		Legendary: &monsterbuild.Legendary{Uses: 3, Resistance: 2, Actions: []monsterbuild.LegendaryAction{{Name: "Tail Sweep", Cost: 1, Text: "One Claw attack."}, {Name: "Sink", Cost: 2, Text: "It sinks into the mire and moves 30 feet."}}},
		Lair:      &monsterbuild.Lair{Actions: []monsterbuild.Trait{{Name: "Rising Water", Text: "The water rises a foot."}}, Regional: []string{"Fogs never lift within a mile."}},
		Phases:    []monsterbuild.Phase{{Name: "Drowned King", HP: 60, Text: "It rises from the water, its wounds closed."}},
	}
}

func TestTheBogKingCompiles(t *testing.T) {
	t.Parallel()
	d := bogKing()
	if err := monsterbuild.Check(d); err != nil {
		t.Fatal(err)
	}
	m := monsterbuild.Compile("hb-0190c7a80006", "Bog King", d)
	if m.Slug != "hb-0190c7a80006" || m.Name != "Bog King" || m.AC != 16 || m.HP != 120 || m.Proficiency != 3 || m.AttacksPerAction != 2 || m.Initiative != 1 || m.Perception != 2 || m.Stealth != 1 {
		t.Fatalf("monster = %+v", m)
	}
	if !reflect.DeepEqual(m.Saves, map[string]int{"strength": 5, "dexterity": 1, "constitution": 7, "intelligence": -1, "wisdom": 5, "charisma": 3}) {
		t.Fatalf("saves = %v", m.Saves)
	}
	if !reflect.DeepEqual(m.Attacks, []monsterbuild.Attack{
		{Name: "Claw", ToHit: 8, ReachFt: 10, RangeFt: 0, LongRangeFt: 0, Damage: "2d8", DamageBonus: 5, DamageType: "slashing"},
		{Name: "Mud Bolt", ToHit: 6, ReachFt: 0, RangeFt: 60, LongRangeFt: 120, Damage: "3d6", DamageBonus: 0, DamageType: "acid"},
	}) {
		t.Fatalf("attacks = %+v", m.Attacks)
	}
	if m.Senses["darkvision"] != 60 || m.Threshold != 5 || m.Legendary.Uses != 3 || len(m.Lair.Actions) != 1 || len(m.Phases) != 1 {
		t.Fatalf("legend = %+v", m)
	}
	if est := monsterbuild.Estimate(m); est != "11" {
		t.Errorf("estimated Challenge = %s", est)
	}
	lines := strings.Join(monsterbuild.Lines(m), "\n")
	for _, w := range []string{
		"Large Monstrosity", "AC 16 · HP 120 · Speed 30 ft.", "STR 20 (+5) · DEX 12 (+1) · CON 18 (+4) · INT 8 (−1) · WIS 14 (+2) · CHA 16 (+3)",
		"Saving Throws: Constitution +7, Wisdom +5", "Senses: Darkvision 60 ft.", "Resistances: Poison", "Immunities: Acid", "Vulnerabilities: Fire",
		"Damage Threshold 5", "Challenge 8 (estimated 11) · Proficiency Bonus +3",
		"Bog Stride. Mud costs it no extra movement.", "Stench (aura, 10 ft.). A creature starting its turn here is Poisoned.",
		"Multiattack. It makes 2 attacks.", "Claw. Melee Attack Roll: +8, reach 10 ft. Hit: 2d8 + 5 slashing damage.",
		"Mud Bolt. Ranged Attack Roll: +6, range 60/120 ft. Hit: 3d6 acid damage.",
		"Bog Breath (Recharge 5–6). Constitution Saving Throw: DC 15. 6d6 poison damage. 15-foot cone; half damage on a success.",
		"Croak. Every frog within a mile answers.",
		"Legendary Actions (3 a round). Legendary Resistance 2 a day.", "Tail Sweep. One Claw attack.", "Sink (costs 2). It sinks into the mire and moves 30 feet.",
		"Lair Actions (initiative 20). Rising Water. The water rises a foot.", "Regional Effect. Fogs never lift within a mile.",
		"Phase 2: Drowned King (60 HP). It rises from the water, its wounds closed.",
	} {
		if !strings.Contains(lines, w) {
			t.Errorf("lines miss %q:\n%s", w, lines)
		}
	}
}

func TestEstimatedChallenges(t *testing.T) {
	t.Parallel()
	weak := monsterbuild.Design{
		Size: "small", CreatureType: "humanoid", AC: 12, HP: 7, SpeedFt: 30, Challenge: 0.25,
		Abilities: map[string]int{"strength": 8, "dexterity": 14, "constitution": 10, "intelligence": 10, "wisdom": 8, "charisma": 8},
		Actions:   []monsterbuild.Action{{Name: "Scimitar", Kind: "melee", ToHit: 4, ReachFt: 5, Damage: "1d6", DamageBonus: 2, DamageType: "slashing"}},
	}
	if err := monsterbuild.Check(weak); err != nil {
		t.Fatal(err)
	}
	if got := monsterbuild.Estimate(monsterbuild.Compile("x", "X", weak)); got != "1/2" {
		t.Errorf("a scimitar goblin = %s", got)
	}
	unarmed := weak
	unarmed.Actions = []monsterbuild.Action{{Name: "Shriek", Kind: "special", Text: "It shrieks."}}
	for hp, want := range map[int]string{1: "0", 7: "1/8", 14: "1/4", 20: "1/2", 60: "2"} {
		w := unarmed
		w.HP = hp
		if got := monsterbuild.Estimate(monsterbuild.Compile("x", "X", w)); got != want {
			t.Errorf("HP %d = %s, want %s", hp, got, want)
		}
	}
	huge := bogKing()
	huge.HP, huge.AC = 1000, 30
	if got := monsterbuild.Estimate(monsterbuild.Compile("x", "X", huge)); got != "30" {
		t.Errorf("a towering monster = %s", got)
	}
	plain := weak
	plain.Actions = nil
	m := monsterbuild.Compile("x", "Plain", plain)
	if lines := monsterbuild.Lines(m); lines[len(lines)-1] != "Challenge 1/4 (estimated 1/8) · Proficiency Bonus +2" {
		t.Fatalf("a monster with no actions = %v", lines)
	}
	swarm := weak
	swarm.Swarm, swarm.Size = true, "medium"
	if lines := strings.Join(monsterbuild.Lines(monsterbuild.Compile("x", "Rats", swarm)), "\n"); !strings.Contains(lines, "Swarm. It can occupy another creature's space") {
		t.Fatalf("a swarm = %s", lines)
	}
}

func TestMonsterDesignsThatDoNotBuild(t *testing.T) {
	t.Parallel()
	for want, change := range map[string]func(*monsterbuild.Design){
		"choose a size":                       func(d *monsterbuild.Design) { d.Size = "titanic" },
		"choose a creature type":              func(d *monsterbuild.Design) { d.CreatureType = "robot" },
		"AC of 5 to 30":                       func(d *monsterbuild.Design) { d.AC = 31 },
		"1 to 1000 hit points":                func(d *monsterbuild.Design) { d.HP = 0 },
		"Speed of 0 to 120":                   func(d *monsterbuild.Design) { d.SpeedFt = 125 },
		"Challenge from 0 to 30":              func(d *monsterbuild.Design) { d.Challenge = 0.3 },
		"each ability from 1 to 30":           func(d *monsterbuild.Design) { d.Abilities["strength"] = 31 },
		"all six abilities":                   func(d *monsterbuild.Design) { delete(d.Abilities, "charisma") },
		"proficient saves":                    func(d *monsterbuild.Design) { d.Saves = []string{"luck"} },
		"darkvision, blindsight":              func(d *monsterbuild.Design) { d.Senses[0].Kind = "sonar" },
		"each sense 5 to 300":                 func(d *monsterbuild.Design) { d.Senses[0].Feet = 0 },
		"real damage types":                   func(d *monsterbuild.Design) { d.Immunities = []string{"cheese"} },
		"threshold of 0 to 50":                func(d *monsterbuild.Design) { d.Threshold = 51 },
		"name each trait":                     func(d *monsterbuild.Design) { d.Traits[0].Name = "" },
		"an aura of 5 to 60 feet":             func(d *monsterbuild.Design) { d.Aura.Feet = 0 },
		"0 or 2 to 4 attacks":                 func(d *monsterbuild.Design) { d.Multiattack = 1 },
		"1 to 20 actions":                     func(d *monsterbuild.Design) { d.Actions = nil },
		"melee, ranged, save or special":      func(d *monsterbuild.Design) { d.Actions[0].Kind = "psychic" },
		"name each action":                    func(d *monsterbuild.Design) { d.Actions[0].Name = "" },
		"to-hit bonus of −5 to +20":           func(d *monsterbuild.Design) { d.Actions[0].ToHit = 21 },
		"reach of 5 to 30 feet":               func(d *monsterbuild.Design) { d.Actions[0].ReachFt = 0 },
		"range of 5 to 600 feet":              func(d *monsterbuild.Design) { d.Actions[1].RangeFt = 0 },
		"long range no shorter":               func(d *monsterbuild.Design) { d.Actions[1].LongRangeFt = 30 },
		"damage dice like 2d6":                func(d *monsterbuild.Design) { d.Actions[0].Damage = "lots" },
		"a damage type":                       func(d *monsterbuild.Design) { d.Actions[0].DamageType = "cheese" },
		"the save and its DC":                 func(d *monsterbuild.Design) { d.Actions[2].DC = 5 },
		"recharge on 4, 5 or 6":               func(d *monsterbuild.Design) { d.Actions[2].Recharge = 3 },
		"describe Croak":                      func(d *monsterbuild.Design) { d.Actions[3].Text = "" },
		"1 to 5 legendary actions":            func(d *monsterbuild.Design) { d.Legendary.Uses = 0 },
		"0 to 5 Legendary Resistances":        func(d *monsterbuild.Design) { d.Legendary.Resistance = 6 },
		"cost 1 to 3":                         func(d *monsterbuild.Design) { d.Legendary.Actions[0].Cost = 4 },
		"name and describe each legendary":    func(d *monsterbuild.Design) { d.Legendary.Actions[0].Text = "" },
		"1 to 10 legendary actions to choose": func(d *monsterbuild.Design) { d.Legendary.Actions = nil },
		"name and describe each lair":         func(d *monsterbuild.Design) { d.Lair.Actions[0].Name = "" },
		"describe each regional effect":       func(d *monsterbuild.Design) { d.Lair.Regional = []string{""} },
		"up to 3 phases":                      func(d *monsterbuild.Design) { d.Phases = make([]monsterbuild.Phase, 4) },
		"each phase 1 to 1000 hit points":     func(d *monsterbuild.Design) { d.Phases[0].HP = 0 },
		"name each phase":                     func(d *monsterbuild.Design) { d.Phases[0].Name = "" },
	} {
		d := bogKing()
		d.Abilities = map[string]int{"strength": 20, "dexterity": 12, "constitution": 18, "intelligence": 8, "wisdom": 14, "charisma": 16}
		change(&d)
		if err := monsterbuild.Check(d); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", want, err)
		}
	}
}

func TestTheRarerShapesOfAStatBlock(t *testing.T) {
	t.Parallel()
	d := bogKing()
	d.Actions = append(d.Actions,
		monsterbuild.Action{Name: "Gaze", Kind: "save", SaveAbility: "wisdom", DC: 14, Recharge: 6, Text: "The target is Frightened."},
	)
	for cr, want := range map[float64]string{0.125: "Challenge 1/8 ", 0.5: "Challenge 1/2 "} {
		d.Challenge = cr
		if err := monsterbuild.Check(d); err != nil {
			t.Fatal(err)
		}
		lines := strings.Join(monsterbuild.Lines(monsterbuild.Compile("x", "X", d)), "\n")
		if !strings.Contains(lines, want) || !strings.Contains(lines, "Gaze (Recharge 6). Wisdom Saving Throw: DC 14. The target is Frightened.") {
			t.Errorf("challenge %v:\n%s", cr, lines)
		}
	}
	many := bogKing()
	many.Traits = make([]monsterbuild.Trait, 21)
	if err := monsterbuild.Check(many); err == nil || !strings.Contains(err.Error(), "up to 20 traits") {
		t.Errorf("21 traits: %v", err)
	}
	long := bogKing()
	long.Actions[0].Text = strings.Repeat("x", 2001)
	if err := monsterbuild.Check(long); err == nil || !strings.Contains(err.Error(), "in up to 2000 characters") {
		t.Errorf("a long action: %v", err)
	}
}

func TestDesignsAtTheirLimitsAndExactEstimates(t *testing.T) {
	t.Parallel()
	d := bogKing()
	d.Challenge = 30
	d.Traits = make([]monsterbuild.Trait, 20)
	for i := range d.Traits {
		d.Traits[i] = monsterbuild.Trait{Name: strings.Repeat("n", 60), Text: strings.Repeat("x", 2000)}
	}
	for len(d.Actions) < 20 {
		d.Actions = append(d.Actions, monsterbuild.Action{Name: "Hum", Kind: "special", Text: "It hums."})
	}
	d.Phases = []monsterbuild.Phase{{Name: "A", HP: 1, Text: strings.Repeat("x", 2000)}, {Name: "B", HP: 1}, {Name: "C", HP: 1}}
	if err := monsterbuild.Check(d); err != nil {
		t.Fatalf("a design at its limits: %v", err)
	}
	quiet := monsterbuild.Design{
		Size: "small", CreatureType: "beast", AC: 13, HP: 1, SpeedFt: 30, Challenge: 0,
		Abilities: map[string]int{"strength": 10, "dexterity": 10, "constitution": 10, "intelligence": 10, "wisdom": 10, "charisma": 10},
		Actions:   []monsterbuild.Action{{Name: "Bite", Kind: "melee", ToHit: 5, ReachFt: 5, Damage: "2d8", DamageType: "piercing"}},
	}
	if got := monsterbuild.Estimate(monsterbuild.Compile("x", "X", quiet)); got != "1" {
		t.Errorf("a 2d8 bite = %s", got)
	}
	quiet.Actions = []monsterbuild.Action{{Name: "Spit", Kind: "save", SaveAbility: "dexterity", DC: 13, Damage: "4d6", DamageType: "acid"}}
	if got := monsterbuild.Estimate(monsterbuild.Compile("x", "X", quiet)); got != "1" {
		t.Errorf("a 4d6 spit that does not recharge = %s", got)
	}
	lines := strings.Join(monsterbuild.Lines(monsterbuild.Compile("x", "X", quiet)), "\n")
	for _, absent := range []string{"Saving Throws", "Senses", "Resistances", "Immunities", "Vulnerabilities", "Damage Threshold"} {
		if strings.Contains(lines, absent) {
			t.Errorf("a plain beast lists %s:\n%s", absent, lines)
		}
	}
	quiet.Actions = append(quiet.Actions, monsterbuild.Action{Name: "Pebble", Kind: "ranged", ToHit: 2, RangeFt: 30, Damage: "1d4", DamageType: "bludgeoning"})
	lines = strings.Join(monsterbuild.Lines(monsterbuild.Compile("x", "X", quiet)), "\n")
	if !strings.Contains(lines, "Pebble. Ranged Attack Roll: +2, range 30 ft. Hit:") {
		t.Errorf("a ranged attack without a long range:\n%s", lines)
	}
	if !strings.Contains(lines, "CON 10 (+0)") {
		t.Errorf("a modifier of 0 reads +0:\n%s", lines)
	}
}
