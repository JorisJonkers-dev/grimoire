package itembuild_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
)

// ashwood is the Ashwood Longbow: a +1 longbow that casts Light at will and Hunter's Mark for a charge,
// regaining 1d4 charges at dawn.
func ashwood() itembuild.Design {
	return itembuild.Design{
		Kind: "weapon", Base: "longbow", Rarity: "rare", Enchantment: 1, WeightLb: 2, ValueGP: 4000,
		Attunement: &itembuild.Attunement{Kind: "class", Value: "ranger"},
		Weapon:     &itembuild.Weapon{Properties: []string{"ammunition", "heavy", "two-handed"}, Mastery: "slow"},
		Charges:    &itembuild.Charges{Max: 3, On: "dawn", Dice: 1, Faces: 4},
		Properties: []itembuild.Property{
			{Type: "cantrip", Spell: "light", Name: "Light"},
			{Type: "spell", Spell: "hunters-mark", Name: "Hunter's Mark", Level: 1, Cost: 1},
		},
	}
}

func TestTheAshwoodLongbow(t *testing.T) {
	t.Parallel()
	d := ashwood()
	if err := itembuild.Check(d); err != nil {
		t.Fatal(err)
	}
	if got := itembuild.Spells(d); !reflect.DeepEqual(got, []itembuild.Granted{{Spell: "light", Name: "Light"}, {Spell: "hunters-mark", Name: "Hunter's Mark", Level: 1, Cost: 1}}) {
		t.Fatalf("spells = %+v", got)
	}
	want := []string{
		"Ashwood Longbow +1",
		"weapon, rare (requires attunement by a ranger)",
		"You have a +1 bonus to attack and damage rolls made with this magic item.",
		"Properties: ammunition, heavy, two-handed. Mastery: slow.",
		"It has 3 charges and regains 1d4 expended charges daily at dawn.",
		"You can cast Light at will.",
		"You can expend 1 charge to cast Hunter's Mark from it.",
	}
	if got := itembuild.Card("Ashwood Longbow", d, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("card =\n%s", strings.Join(got, "\n"))
	}
	if p := itembuild.Price(d); p.Points != 6 || p.Suggested != "rare" || p.PriceGP != 4000 || !p.Fits || p.Notes != nil {
		t.Fatalf("price = %+v", p)
	}
	if itembuild.Category(d) != "weapon" {
		t.Fatal("category")
	}
}

func TestEveryPropertyReadsOnTheCard(t *testing.T) {
	t.Parallel()
	d := itembuild.Design{
		Kind: "cloak", Rarity: "legendary", Attunement: &itembuild.Attunement{}, Charges: &itembuild.Charges{Max: 7, On: "long_rest"},
		Properties: []itembuild.Property{
			{Type: "skill_boost", Skill: "sleight-of-hand", Mode: "advantage"},
			{Type: "skill_boost", Skill: "stealth", Mode: "d4"},
			{Type: "skill_boost", Skill: "stealth", Mode: "flat", Value: 2},
			{Type: "skill_boost", Skill: "arcana", Mode: "proficiency"},
			{Type: "skill_boost", Skill: "arcana", Mode: "expertise"},
			{Type: "bonus", Target: "spell_dc", Value: 1},
			{Type: "resistance", Damage: "cold"},
			{Type: "extra_damage", Dice: "1d6", Damage: "fire"},
			{Type: "sense", Sense: "darkvision", Feet: 60},
			{Type: "speed", Speed: "fly", Feet: 30},
			{Type: "speed", Speed: "swim", Feet: 30},
			{Type: "spell", Spell: "fly", Name: "Fly", Level: 3, Cost: 2},
			{Type: "spell", Spell: "misty-step", Name: "Misty Step", Level: 2},
			{Type: "light", BrightFt: 10, DimFt: 10},
			{Type: "container", CapacityLb: 500, Weightless: true, OnlyKind: "potion"},
			{Type: "container", CapacityLb: 50},
			{Type: "curse", Text: "You fear the dark.", CannotDrop: true},
			{Type: "curse", Text: "It hums.", Hidden: true},
			{Type: "sentient", Text: "It wants to be worn at court."},
			{Type: "growth", AtLevel: 11, Text: "Its light reaches twice as far."},
			{Type: "set_bonus", Set: "Starweave", Pieces: 3, Text: "+1 AC."},
			{Type: "trigger", Text: "When you are hit, it glows."},
			{Type: "manual", Text: "The DM decides."},
		},
	}
	if err := itembuild.Check(d); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Cloak", "cloak, legendary (requires attunement)",
		"It has 7 charges and regains all its expended charges on a long rest.",
		"You have Advantage on Sleight of Hand checks.", "You add 1d4 to Stealth checks.", "You gain a +2 bonus to Stealth checks.",
		"You have proficiency in Arcana.", "You have Expertise in Arcana.", "You gain a +1 bonus to spell dc.", "You have Resistance to cold damage.",
		"Hits with it deal an extra 1d6 fire damage.", "You have darkvision out to 60 feet.", "You have a fly speed of 30 feet.", "You have a swim speed of 30 feet.",
		"You can expend 2 charges to cast Fly from it.", "You can cast Misty Step from it at will.",
		"It sheds bright light in a 10-foot radius and dim light for an additional 10 feet.",
		"It holds up to 500 pounds of potion items and weighs the same however full it is.", "It holds up to 50 pounds.",
		"Curse. You fear the dark. You can't remove it while attuned unless the curse is broken.",
		"Sentience. It wants to be worn at court.", "At level 11: Its light reaches twice as far.", "Starweave (3 pieces): +1 AC.",
		"When you are hit, it glows.", "The DM decides.",
	}
	if got := itembuild.Card("Cloak", d, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("card =\n%s", strings.Join(got, "\n"))
	}
	if known := itembuild.Card("Cloak", d, true); !strings.Contains(strings.Join(known, "\n"), "Curse. It hums.") {
		t.Fatal("a hidden curse shows once the item is known")
	}
	// 1+1+2+1+2 boosts, 2 bonus, 2 resistance, 1+1 extra damage, 1 sense, 3+1 speeds, 4+3 spells, 1+1 containers,
	// -2-2 curses, 1 sentient, 2 growth, 1 set, (7+2)/3 charges.
	if p := itembuild.Price(d); p.Points != 30 || p.Suggested != "legendary" {
		t.Fatalf("points = %+v", p)
	}
	for _, c := range []struct {
		props []itembuild.Property
		want  string
	}{
		{[]itembuild.Property{{Type: "consumable", Uses: "single"}}, "It is used up when used."},
		{[]itembuild.Property{{Type: "consumable", Uses: "long_rest"}}, "Once used, it can't be used again until the next long rest."},
		{[]itembuild.Property{{Type: "consumable", Uses: "coating", Hits: 1}}, "Applied to a weapon, it lasts for 1 hit."},
		{[]itembuild.Property{{Type: "consumable", Uses: "coating", Hits: 3}}, "Applied to a weapon, it lasts for 3 hits."},
		{[]itembuild.Property{{Type: "firearm", Misfire: 2, Reload: 6, Burst: 3}}, "Firearm: misfires on a 2 or lower, reloads after 6 shots, fires bursts of 3."},
	} {
		got := itembuild.Card("X", itembuild.Design{Kind: "potion", Rarity: "common", Properties: c.props}, true)
		if got[len(got)-1] != c.want {
			t.Errorf("card line = %q, want %q", got[len(got)-1], c.want)
		}
	}
	custom := itembuild.Card("Axe", itembuild.Design{Kind: "weapon", Rarity: "common", Weapon: &itembuild.Weapon{Properties: []string{"heavy"}, Mastery: "custom", Custom: "Knocks down doors"}}, true)
	bare := itembuild.Card("Club", itembuild.Design{Kind: "weapon", Rarity: "common", Weapon: &itembuild.Weapon{Properties: []string{"light"}}, Charges: &itembuild.Charges{Max: 2, On: "short_rest", Dice: 1, Faces: 6, Bonus: 1}}, true)
	if custom[2] != "Properties: heavy. Mastery: Knocks down doors." || bare[2] != "Properties: light." || bare[3] != "It has 2 charges and regains 1d6 + 1 expended charges on a short rest." {
		t.Fatalf("weapon lines = %v %v", custom, bare)
	}
	for kind, want := range map[string]string{"armor": "armor", "helmet": "armor", "amulet": "wondrous-item", "ring": "ring", "ammunition": "weapon", "potion": "potion"} {
		if got := itembuild.Category(itembuild.Design{Kind: kind}); got != want {
			t.Errorf("category of %s = %s", kind, got)
		}
	}
}

func TestDesignsAreChecked(t *testing.T) {
	t.Parallel()
	with := func(change func(*itembuild.Design)) itembuild.Design {
		d := ashwood()
		change(&d)
		return d
	}
	prop := func(p itembuild.Property) func(*itembuild.Design) {
		return func(d *itembuild.Design) { d.Properties = []itembuild.Property{p} }
	}
	many := make([]itembuild.Property, 41)
	for i := range many {
		many[i] = itembuild.Property{Type: "manual", Text: "x"}
	}
	for name, c := range map[string]struct {
		change func(*itembuild.Design)
		want   string
	}{
		"kind":             {func(d *itembuild.Design) { d.Kind = "spoon" }, "choose a kind"},
		"base":             {func(d *itembuild.Design) { d.Base = "Long Bow" }, "base item"},
		"rarity":           {func(d *itembuild.Design) { d.Rarity = "mythic" }, "choose a rarity"},
		"negative plus":    {func(d *itembuild.Design) { d.Enchantment = -1 }, "+0 to +3"},
		"plus four":        {func(d *itembuild.Design) { d.Enchantment = 4 }, "+0 to +3"},
		"negative weight":  {func(d *itembuild.Design) { d.WeightLb = -1 }, "0 to 1000 pounds"},
		"heavy":            {func(d *itembuild.Design) { d.WeightLb = 1001 }, "0 to 1000 pounds"},
		"negative value":   {func(d *itembuild.Design) { d.ValueGP = -1 }, "0 to 10000000 gp"},
		"dear":             {func(d *itembuild.Design) { d.ValueGP = 10000001 }, "0 to 10000000 gp"},
		"attune kind":      {func(d *itembuild.Design) { d.Attunement.Kind = "hat" }, "class, species, background or alignment"},
		"attune blank":     {func(d *itembuild.Design) { d.Attunement.Value = " " }, "up to 40 characters"},
		"attune long":      {func(d *itembuild.Design) { d.Attunement.Value = strings.Repeat("a", 41) }, "up to 40 characters"},
		"weapon on a ring": {func(d *itembuild.Design) { d.Kind = "ring" }, "only a weapon"},
		"mastery":          {func(d *itembuild.Design) { d.Weapon.Mastery = "spin" }, "Weapon Mastery"},
		"custom blank":     {func(d *itembuild.Design) { d.Weapon.Mastery = "custom" }, "describe the custom"},
		"custom long":      {func(d *itembuild.Design) { d.Weapon.Custom = strings.Repeat("a", 301) }, "up to 300 characters"},
		"weapon property":  {func(d *itembuild.Design) { d.Weapon.Properties = []string{"shiny"} }, "weapon properties"},
		"no charges":       {func(d *itembuild.Design) { d.Charges.Max = 0 }, "1 to 100 charges"},
		"many charges":     {func(d *itembuild.Design) { d.Charges.Max = 101 }, "1 to 100 charges"},
		"schedule":         {func(d *itembuild.Design) { d.Charges.On = "noon" }, "dawn, on a long rest"},
		"negative dice":    {func(d *itembuild.Design) { d.Charges.Dice = -1 }, "up to 10 dice"},
		"many dice":        {func(d *itembuild.Design) { d.Charges.Dice = 11 }, "up to 10 dice"},
		"negative bonus":   {func(d *itembuild.Design) { d.Charges.Bonus = -1 }, "up to 10 dice"},
		"big bonus":        {func(d *itembuild.Design) { d.Charges.Bonus = 101 }, "up to 10 dice"},
		"faces":            {func(d *itembuild.Design) { d.Charges.Faces = 5 }, "roll d4"},
		"too many":         {func(d *itembuild.Design) { d.Properties = many }, "40 Item Properties"},
		"property":         {prop(itembuild.Property{Type: "glow"}), "an Item Property is"},
		"boost skill":      {prop(itembuild.Property{Type: "skill_boost", Skill: "juggling", Mode: "advantage"}), "names a skill"},
		"boost mode":       {prop(itembuild.Property{Type: "skill_boost", Skill: "stealth", Mode: "luck"}), "gives advantage"},
		"boost zero":       {prop(itembuild.Property{Type: "skill_boost", Skill: "stealth", Mode: "flat"}), "+1 to +5"},
		"boost six":        {prop(itembuild.Property{Type: "skill_boost", Skill: "stealth", Mode: "flat", Value: 6}), "+1 to +5"},
		"bonus target":     {prop(itembuild.Property{Type: "bonus", Target: "luck", Value: 1}), "a bonus adds"},
		"bonus zero":       {prop(itembuild.Property{Type: "bonus", Target: "ac"}), "a bonus adds"},
		"bonus six":        {prop(itembuild.Property{Type: "bonus", Target: "ac", Value: 6}), "a bonus adds"},
		"resistance":       {prop(itembuild.Property{Type: "resistance", Damage: "sadness"}), "damage type"},
		"extra type":       {prop(itembuild.Property{Type: "extra_damage", Dice: "1d6", Damage: "sadness"}), "damage type"},
		"extra dice":       {prop(itembuild.Property{Type: "extra_damage", Dice: "1d7", Damage: "fire"}), "look like 1d6"},
		"sense":            {prop(itembuild.Property{Type: "sense", Sense: "smell", Feet: 30}), "a sense is"},
		"sense short":      {prop(itembuild.Property{Type: "sense", Sense: "darkvision", Feet: 4}), "a sense is"},
		"sense far":        {prop(itembuild.Property{Type: "sense", Sense: "darkvision", Feet: 301}), "a sense is"},
		"speed":            {prop(itembuild.Property{Type: "speed", Speed: "teleport", Feet: 30}), "a speed is"},
		"speed slow":       {prop(itembuild.Property{Type: "speed", Speed: "fly", Feet: 4}), "a speed is"},
		"speed fast":       {prop(itembuild.Property{Type: "speed", Speed: "fly", Feet: 121}), "a speed is"},
		"spell slug":       {prop(itembuild.Property{Type: "spell", Spell: "Fire Ball", Name: "Fireball", Level: 3}), "names a spell"},
		"spell name":       {prop(itembuild.Property{Type: "spell", Spell: "fireball", Name: " ", Level: 3}), "names a spell"},
		"cantrip level":    {prop(itembuild.Property{Type: "cantrip", Spell: "light", Name: "Light", Level: 1}), "a cantrip is level 0"},
		"cantrip cost":     {prop(itembuild.Property{Type: "cantrip", Spell: "light", Name: "Light", Cost: 1}), "a cantrip is level 0"},
		"spell zero":       {prop(itembuild.Property{Type: "spell", Spell: "fireball", Name: "Fireball"}), "level 1 to 9"},
		"spell ten":        {prop(itembuild.Property{Type: "spell", Spell: "fireball", Name: "Fireball", Level: 10}), "level 1 to 9"},
		"negative cost":    {prop(itembuild.Property{Type: "spell", Spell: "fireball", Name: "Fireball", Level: 3, Cost: -1}), "0 to 100 charges"},
		"big cost":         {prop(itembuild.Property{Type: "spell", Spell: "fireball", Name: "Fireball", Level: 3, Cost: 101}), "0 to 100 charges"},
		"chargeless": {func(d *itembuild.Design) {
			d.Charges, d.Properties = nil, []itembuild.Property{{Type: "spell", Spell: "fireball", Name: "Fireball", Level: 3, Cost: 1}}
		}, "needs an item that holds them"},
		"dark":         {prop(itembuild.Property{Type: "light", BrightFt: 4}), "light reaches"},
		"blinding":     {prop(itembuild.Property{Type: "light", BrightFt: 121}), "light reaches"},
		"negative dim": {prop(itembuild.Property{Type: "light", BrightFt: 5, DimFt: -1}), "light reaches"},
		"wide dim":     {prop(itembuild.Property{Type: "light", BrightFt: 5, DimFt: 121}), "light reaches"},
		"uses":         {prop(itembuild.Property{Type: "consumable", Uses: "twice"}), "a consumable is"},
		"no hits":      {prop(itembuild.Property{Type: "consumable", Uses: "coating"}), "a consumable is"},
		"many hits":    {prop(itembuild.Property{Type: "consumable", Uses: "coating", Hits: 101}), "a consumable is"},
		"empty bag":    {prop(itembuild.Property{Type: "container"}), "a container holds"},
		"huge bag":     {prop(itembuild.Property{Type: "container", CapacityLb: 10001}), "a container holds"},
		"bag kind":     {prop(itembuild.Property{Type: "container", CapacityLb: 5, OnlyKind: "spoon"}), "a container holds"},
		"misfire":      {prop(itembuild.Property{Type: "firearm", Misfire: -1}), "a firearm"},
		"misfire high": {prop(itembuild.Property{Type: "firearm", Misfire: 21}), "a firearm"},
		"reload":       {prop(itembuild.Property{Type: "firearm", Reload: -1}), "a firearm"},
		"reload high":  {prop(itembuild.Property{Type: "firearm", Reload: 21}), "a firearm"},
		"burst":        {prop(itembuild.Property{Type: "firearm", Burst: -1}), "a firearm"},
		"burst high":   {prop(itembuild.Property{Type: "firearm", Burst: 11}), "a firearm"},
		"blank curse":  {prop(itembuild.Property{Type: "curse", Text: " "}), "up to 1000 characters"},
		"long curse":   {prop(itembuild.Property{Type: "curse", Text: strings.Repeat("a", 1001)}), "up to 1000 characters"},
		"growth zero":  {prop(itembuild.Property{Type: "growth", Text: "x"}), "level from 1 to 20"},
		"growth high":  {prop(itembuild.Property{Type: "growth", Text: "x", AtLevel: 21}), "level from 1 to 20"},
		"nameless set": {prop(itembuild.Property{Type: "set_bonus", Text: "x", Pieces: 2}), "set bonus"},
		"lonely set":   {prop(itembuild.Property{Type: "set_bonus", Text: "x", Set: "S", Pieces: 1}), "set bonus"},
		"crowded set":  {prop(itembuild.Property{Type: "set_bonus", Text: "x", Set: "S", Pieces: 11}), "set bonus"},
	} {
		err := itembuild.Check(with(c.change))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want %q", name, err, c.want)
		}
	}
	for name, change := range map[string]func(*itembuild.Design){
		"edges": func(d *itembuild.Design) {
			d.Enchantment, d.WeightLb, d.ValueGP, d.Base, d.Attunement = 3, 1000, 10000000, "", nil
			d.Attunement = &itembuild.Attunement{Kind: "alignment", Value: strings.Repeat("a", 40)}
			d.Weapon = &itembuild.Weapon{Mastery: "custom", Custom: strings.Repeat("a", 300)}
			d.Charges = &itembuild.Charges{Max: 100, On: "short_rest", Dice: 10, Faces: 20, Bonus: 100}
			d.Properties = append(make([]itembuild.Property, 0, 40), d.Properties...)
			d.Properties = append(d.Properties,
				itembuild.Property{Type: "skill_boost", Skill: "stealth", Mode: "flat", Value: 5}, itembuild.Property{Type: "bonus", Target: "ac", Value: 5},
				itembuild.Property{Type: "sense", Sense: "truesight", Feet: 300}, itembuild.Property{Type: "speed", Speed: "burrow", Feet: 120},
				itembuild.Property{Type: "spell", Spell: "wish", Name: "Wish", Level: 9, Cost: 100}, itembuild.Property{Type: "light", BrightFt: 120, DimFt: 120},
				itembuild.Property{Type: "consumable", Uses: "coating", Hits: 100}, itembuild.Property{Type: "container", CapacityLb: 10000},
				itembuild.Property{Type: "firearm", Misfire: 20, Reload: 20, Burst: 10}, itembuild.Property{Type: "growth", Text: strings.Repeat("a", 1000), AtLevel: 20},
				itembuild.Property{Type: "set_bonus", Text: "x", Set: "S", Pieces: 10},
			)
			for len(d.Properties) < 40 {
				d.Properties = append(d.Properties, itembuild.Property{Type: "manual", Text: "x"})
			}
		},
		"least": func(d *itembuild.Design) {
			d.Enchantment, d.WeightLb, d.ValueGP = 0, 0, 0
			d.Charges = &itembuild.Charges{Max: 1, On: "long_rest"}
			d.Properties = []itembuild.Property{
				{Type: "skill_boost", Skill: "stealth", Mode: "flat", Value: 1},
				{Type: "bonus", Target: "ac", Value: 1},
				{Type: "sense", Sense: "darkvision", Feet: 5},
				{Type: "speed", Speed: "walk", Feet: 5},
				{Type: "spell", Spell: "sleep", Name: "Sleep", Level: 1},
				{Type: "light", BrightFt: 5},
				{Type: "consumable", Uses: "coating", Hits: 1},
				{Type: "container", CapacityLb: 1},
				{Type: "growth", Text: "x", AtLevel: 1},
				{Type: "set_bonus", Text: "x", Set: "S", Pieces: 2},
				{Type: "firearm"},
			}
		},
	} {
		if err := itembuild.Check(with(change)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestThePriceCheck(t *testing.T) {
	t.Parallel()
	for points, want := range map[int]string{0: "common", 1: "common", 2: "uncommon", 3: "uncommon", 4: "rare", 6: "rare", 7: "very_rare", 9: "very_rare", 10: "legendary"} {
		if got := itembuild.Suggest(points); got != want {
			t.Errorf("%d points = %s, want %s", points, got, want)
		}
	}
	potion := itembuild.Design{Kind: "potion", Rarity: "uncommon", ValueGP: 200, Properties: []itembuild.Property{
		{Type: "resistance", Damage: "fire"}, {Type: "bonus", Target: "saves", Value: 1}, {Type: "consumable", Uses: "single"},
	}}
	if p := itembuild.Price(potion); p.Points != 2 || !p.Fits {
		t.Fatalf("a consumable is worth half = %+v", p)
	}
	cursed := itembuild.Design{Kind: "ring", Rarity: "common", ValueGP: 100, Properties: []itembuild.Property{{Type: "curse", Text: "x"}}}
	if p := itembuild.Price(cursed); p.Points != 0 || !p.Fits {
		t.Fatalf("a curse never makes an item worth less than nothing = %+v", p)
	}
	for value, fits := range map[int]bool{1999: false, 2000: true, 8000: true, 8001: false} {
		d := ashwood()
		d.ValueGP = value
		if p := itembuild.Price(d); p.Fits != fits {
			t.Errorf("%d gp for a rare item fits = %v", value, p.Fits)
		}
	}
	d := ashwood()
	d.Rarity, d.ValueGP = "uncommon", 99
	p := itembuild.Price(d)
	if p.Fits || len(p.Notes) != 2 || p.Notes[0] != "Its properties point to rare, not uncommon." || !strings.Contains(p.Notes[1], "goes for about 400 gp; 99 gp is far from it") {
		t.Fatalf("an underpriced item = %+v", p)
	}
	d.Properties = []itembuild.Property{{Type: "extra_damage", Dice: "2d6", Damage: "fire"}}
	d.Charges = nil
	if p := itembuild.Price(d); p.Points != 2+1+7/3 {
		t.Fatalf("extra damage weighs its average = %+v", p)
	}
	for want, props := range map[int][]itembuild.Property{
		4: {{Type: "bonus", Target: "ac", Value: 2}},
		3: {{Type: "speed", Speed: "fly", Feet: 30}},
		1: {{Type: "speed", Speed: "swim", Feet: 30}},
	} {
		if p := itembuild.Price(itembuild.Design{Kind: "ring", Rarity: "rare", Properties: props}); p.Points != want {
			t.Errorf("%+v weighs %d, want %d", props, p.Points, want)
		}
	}
	if len(itembuild.Kinds()) != 19 || len(itembuild.Masteries()) != 9 || len(itembuild.Skills()) != 18 {
		t.Fatal("the builder's lists")
	}
}
