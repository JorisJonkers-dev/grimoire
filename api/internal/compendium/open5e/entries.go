package open5e

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

type apiClass struct {
	Key          string `json:"key"`
	Document     ref    `json:"document"`
	Name         string `json:"name"`
	Desc         string `json:"desc"`
	HitDice      string `json:"hit_dice"`
	CasterType   string `json:"caster_type"`
	SubclassOf   *ref   `json:"subclass_of"`
	SavingThrows []ref  `json:"saving_throws"`
	Features     []struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		Desc     string `json:"desc"`
		GainedAt []struct {
			Level int `json:"level"`
		} `json:"gained_at"`
	} `json:"features"`
}

type apiNamed struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

type apiSpecies struct {
	Key          string     `json:"key"`
	Document     ref        `json:"document"`
	Name         string     `json:"name"`
	Desc         string     `json:"desc"`
	IsSubspecies bool       `json:"is_subspecies"`
	Traits       []apiNamed `json:"traits"`
}

type apiBackground struct {
	Key      string     `json:"key"`
	Document ref        `json:"document"`
	Name     string     `json:"name"`
	Desc     string     `json:"desc"`
	Benefits []apiNamed `json:"benefits"`
}

type apiFeat struct {
	Key          string     `json:"key"`
	Document     ref        `json:"document"`
	Name         string     `json:"name"`
	Desc         string     `json:"desc"`
	Type         string     `json:"type"`
	Prerequisite string     `json:"prerequisite"`
	Benefits     []apiNamed `json:"benefits"`
}

type apiWeapon struct {
	Key        string  `json:"key"`
	Document   ref     `json:"document"`
	Name       string  `json:"name"`
	DamageDice string  `json:"damage_dice"`
	DamageType ref     `json:"damage_type"`
	Range      float64 `json:"range"`
	LongRange  float64 `json:"long_range"`
	IsSimple   bool    `json:"is_simple"`
	Properties []struct {
		Property struct {
			Name string  `json:"name"`
			Type *string `json:"type"`
		} `json:"property"`
		Detail *string `json:"detail"`
	} `json:"properties"`
}

type apiArmor struct {
	Key                   string `json:"key"`
	Document              ref    `json:"document"`
	Name                  string `json:"name"`
	Category              string `json:"category"`
	ACBase                int    `json:"ac_base"`
	ACAddDexmod           bool   `json:"ac_add_dexmod"`
	ACCapDexmod           *int   `json:"ac_cap_dexmod"`
	StealthDisadvantage   bool   `json:"grants_stealth_disadvantage"`
	StrengthScoreRequired *int   `json:"strength_score_required"`
}

type apiItem struct {
	Key                string  `json:"key"`
	Document           ref     `json:"document"`
	Name               string  `json:"name"`
	Desc               string  `json:"desc"`
	Category           ref     `json:"category"`
	Cost               string  `json:"cost"`
	Weight             string  `json:"weight"`
	Rarity             *ref    `json:"rarity"`
	RequiresAttunement bool    `json:"requires_attunement"`
	AttunementDetail   *string `json:"attunement_detail"`
}

type apiAttack struct {
	Name          string   `json:"name"`
	AttackType    string   `json:"attack_type"`
	ToHitMod      int      `json:"to_hit_mod"`
	Reach         *float64 `json:"reach"`
	Range         *float64 `json:"range"`
	LongRange     *float64 `json:"long_range"`
	DieCount      *int     `json:"damage_die_count"`
	DieType       *string  `json:"damage_die_type"`
	DamageBonus   *int     `json:"damage_bonus"`
	DamageType    *ref     `json:"damage_type"`
	ExtraDieCount *int     `json:"extra_damage_die_count"`
	ExtraDieType  *string  `json:"extra_damage_die_type"`
	ExtraType     *ref     `json:"extra_damage_type"`
}

type apiCreature struct {
	Key             string          `json:"key"`
	Document        ref             `json:"document"`
	Name            string          `json:"name"`
	Size            ref             `json:"size"`
	Type            ref             `json:"type"`
	Alignment       string          `json:"alignment"`
	ArmorClass      int             `json:"armor_class"`
	ArmorDetail     string          `json:"armor_detail"`
	HitPoints       int             `json:"hit_points"`
	HitDice         string          `json:"hit_dice"`
	ChallengeRating float64         `json:"challenge_rating"`
	XP              int             `json:"experience_points"`
	Abilities       map[string]int  `json:"ability_scores"`
	Saves           map[string]*int `json:"saving_throws"`
	Skills          map[string]*int `json:"skill_bonuses"`
	SpeedAll        map[string]any  `json:"speed_all"`
	Passive         int             `json:"passive_perception"`
	Darkvision      *float64        `json:"darkvision_range"`
	Blindsight      *float64        `json:"blindsight_range"`
	Tremorsense     *float64        `json:"tremorsense_range"`
	Truesight       *float64        `json:"truesight_range"`
	Languages       struct {
		AsString string `json:"as_string"`
	} `json:"languages"`
	Resist struct {
		Immunities          []ref `json:"damage_immunities"`
		Resistances         []ref `json:"damage_resistances"`
		Vulnerabilities     []ref `json:"damage_vulnerabilities"`
		ConditionImmunities []ref `json:"condition_immunities"`
	} `json:"resistances_and_immunities"`
	Traits  []apiNamed `json:"traits"`
	Actions []struct {
		Name       string      `json:"name"`
		Desc       string      `json:"desc"`
		ActionType string      `json:"action_type"`
		Attacks    []apiAttack `json:"attacks"`
	} `json:"actions"`
}

// fetchKind fetches one kind for one document, keeping only that document's entries: some Open5e
// endpoints ignore the document filter.
func fetchKind[T any, S any](ctx context.Context, c Client, path, doc string, docOf func(T) string, mapped func(T) S) ([]S, error) {
	all, err := fetchAll[T](ctx, c, path+"?document__key="+doc+"&limit=100")
	if err != nil {
		return nil, err
	}
	out := []S{}
	for _, x := range all {
		if docOf(x) == doc {
			out = append(out, mapped(x))
		}
	}
	return out, nil
}

// fetchEntries fills every non-spell kind, one document at a time.
func (c Client) fetchEntries(ctx context.Context, snap *snapshot.Snapshot, doc string) error {
	steps := []func() error{
		func() error {
			x, e := fetchKind(ctx, c, "/v2/classes/", doc, func(x apiClass) string { return x.Document.Key }, mapClass)
			snap.Classes = append(snap.Classes, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/species/", doc, func(x apiSpecies) string { return x.Document.Key }, mapSpecies)
			snap.Species = append(snap.Species, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/backgrounds/", doc, func(x apiBackground) string { return x.Document.Key }, mapBackground)
			snap.Backgrounds = append(snap.Backgrounds, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/feats/", doc, func(x apiFeat) string { return x.Document.Key }, mapFeat)
			snap.Feats = append(snap.Feats, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/weapons/", doc, func(x apiWeapon) string { return x.Document.Key }, mapWeapon)
			snap.Weapons = append(snap.Weapons, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/armor/", doc, func(x apiArmor) string { return x.Document.Key }, mapArmor)
			snap.Armor = append(snap.Armor, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/items/", doc, func(x apiItem) string { return x.Document.Key }, func(x apiItem) snapshot.Item { return mapItem(x, false) })
			snap.Items = append(snap.Items, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/magicitems/", doc, func(x apiItem) string { return x.Document.Key }, func(x apiItem) snapshot.Item { return mapItem(x, true) })
			snap.Items = append(snap.Items, x...)
			return e
		},
		func() error {
			x, e := fetchKind(ctx, c, "/v2/creatures/", doc, func(x apiCreature) string { return x.Document.Key }, mapCreature)
			snap.Monsters = append(snap.Monsters, x...)
			return e
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func mapSpecies(x apiSpecies) snapshot.Species {
	return snapshot.Species{Entry: entry(x.Document, x.Key, x.Name, x.Desc), Subspecies: x.IsSubspecies, Traits: named(x.Traits)}
}

func mapBackground(x apiBackground) snapshot.Background {
	return snapshot.Background{Entry: entry(x.Document, x.Key, x.Name, x.Desc), Benefits: named(x.Benefits)}
}

func mapFeat(x apiFeat) snapshot.Feat {
	f := snapshot.Feat{Entry: entry(x.Document, x.Key, x.Name, x.Desc), Type: x.Type, Prerequisite: x.Prerequisite, Benefits: []string{}}
	for _, b := range x.Benefits {
		f.Benefits = append(f.Benefits, strings.TrimSpace(b.Desc))
	}
	return f
}

func mapArmor(x apiArmor) snapshot.Armor {
	return snapshot.Armor{
		Entry: entry(x.Document, x.Key, x.Name, ""), Category: x.Category, ACBase: x.ACBase, AddDex: x.ACAddDexmod,
		DexCap: x.ACCapDexmod, StealthDisadvantage: x.StealthDisadvantage, StrengthRequired: x.StrengthScoreRequired,
	}
}

func entry(doc ref, key, name, desc string) snapshot.Entry {
	return snapshot.Entry{Document: doc.Key, Slug: unprefix(key), Name: name, Description: strings.TrimSpace(desc)}
}

func named(in []apiNamed) []snapshot.Named {
	out := make([]snapshot.Named, 0, len(in))
	for _, n := range in {
		out = append(out, snapshot.Named{Name: n.Name, Description: strings.TrimSpace(n.Desc)})
	}
	return out
}

func mapClass(x apiClass) snapshot.Class {
	c := snapshot.Class{
		Entry: entry(x.Document, x.Key, x.Name, x.Desc), HitDie: dieSize(x.HitDice), CasterType: strings.ToLower(x.CasterType),
		SavingThrows: []string{}, Features: []snapshot.ClassFeature{},
	}
	if x.SubclassOf != nil {
		c.Parent = unprefix(x.SubclassOf.Key)
	}
	for _, s := range x.SavingThrows {
		c.SavingThrows = append(c.SavingThrows, strings.ToLower(s.Name))
	}
	for _, f := range x.Features {
		feature := snapshot.ClassFeature{Slug: slugAfterClass(f.Key), Name: f.Name, Description: strings.TrimSpace(f.Desc), Levels: []int{}}
		for _, g := range f.GainedAt {
			feature.Levels = append(feature.Levels, g.Level)
		}
		c.Features = append(c.Features, feature)
	}
	return c
}

func mapWeapon(x apiWeapon) snapshot.Weapon {
	w := snapshot.Weapon{
		Entry: entry(x.Document, x.Key, x.Name, ""), DamageDice: x.DamageDice, DamageType: x.DamageType.Key,
		RangeFeet: int(x.Range), LongRangeFeet: int(x.LongRange), Simple: x.IsSimple, Properties: []snapshot.WeaponProperty{},
	}
	for _, p := range x.Properties {
		prop := snapshot.WeaponProperty{Name: p.Property.Name, Mastery: p.Property.Type != nil && *p.Property.Type == "Mastery"}
		if p.Detail != nil {
			prop.Detail = *p.Detail
		}
		w.Properties = append(w.Properties, prop)
	}
	return w
}

func mapItem(x apiItem, magic bool) snapshot.Item {
	it := snapshot.Item{
		Entry: entry(x.Document, x.Key, x.Name, x.Desc), Category: x.Category.Key, CostGP: number(x.Cost), WeightLB: number(x.Weight),
		Magic: magic, RequiresAttunement: x.RequiresAttunement,
	}
	if x.Rarity != nil {
		it.Rarity = x.Rarity.Key
	}
	if x.AttunementDetail != nil {
		it.AttunementDetail = *x.AttunementDetail
	}
	return it
}

func mapCreature(x apiCreature) snapshot.Monster {
	m := snapshot.Monster{
		Entry: entry(x.Document, x.Key, x.Name, ""), Size: x.Size.Key, Type: x.Type.Key, Alignment: x.Alignment,
		ArmorClass: x.ArmorClass, ArmorDetail: x.ArmorDetail, HitPoints: x.HitPoints, HitDice: x.HitDice,
		ChallengeRating: x.ChallengeRating, XP: x.XP, Abilities: map[string]int{}, Saves: map[string]int{}, Skills: map[string]int{},
		Speeds: map[string]int{}, Senses: map[string]int{}, PassivePerception: x.Passive, Languages: x.Languages.AsString,
		Resistances: keys(x.Resist.Resistances), Immunities: keys(x.Resist.Immunities), Vulnerabilities: keys(x.Resist.Vulnerabilities),
		ConditionImmunities: keys(x.Resist.ConditionImmunities), Traits: named(x.Traits), Actions: []snapshot.Action{},
	}
	mapCreatureStats(x, &m)
	for _, a := range x.Actions {
		action := snapshot.Action{Name: a.Name, Description: strings.TrimSpace(a.Desc), Type: strings.ToLower(a.ActionType), Attacks: []snapshot.Attack{}}
		for _, at := range a.Attacks {
			action.Attacks = append(action.Attacks, mapAttack(at))
		}
		m.Actions = append(m.Actions, action)
	}
	return m
}

func mapCreatureStats(x apiCreature, m *snapshot.Monster) {
	for k, v := range x.Abilities {
		m.Abilities[k] = v
	}
	for k, v := range x.Saves {
		if v != nil {
			m.Saves[k] = *v
		}
	}
	for k, v := range x.Skills {
		if v != nil {
			m.Skills[strings.ReplaceAll(k, "_", "-")] = *v
		}
	}
	for k, v := range x.SpeedAll {
		if f, ok := v.(float64); ok && f > 0 {
			m.Speeds[k] = int(f)
		}
	}
	for name, v := range map[string]*float64{"darkvision": x.Darkvision, "blindsight": x.Blindsight, "tremorsense": x.Tremorsense, "truesight": x.Truesight} {
		if v != nil && *v > 0 {
			m.Senses[name] = int(*v)
		}
	}
}

func mapAttack(a apiAttack) snapshot.Attack {
	out := snapshot.Attack{
		Name: a.Name, Kind: strings.ToLower(a.AttackType), ToHit: a.ToHitMod,
		ReachFeet: feetOf(a.Reach), RangeFeet: feetOf(a.Range), LongRangeFeet: feetOf(a.LongRange),
		DamageDice: dice(a.DieCount, a.DieType), ExtraDice: dice(a.ExtraDieCount, a.ExtraDieType),
	}
	if a.DamageBonus != nil {
		out.DamageBonus = *a.DamageBonus
	}
	if a.DamageType != nil {
		out.DamageType = a.DamageType.Key
	}
	if a.ExtraType != nil {
		out.ExtraType = a.ExtraType.Key
		if out.DamageType == "" && out.ExtraDice == "" {
			out.DamageType, out.ExtraType = out.ExtraType, ""
		}
	}
	return out
}

func dice(count *int, die *string) string {
	if count == nil || die == nil || *count == 0 {
		return ""
	}
	return strconv.Itoa(*count) + strings.ToLower(*die)
}

func feetOf(v *float64) int {
	if v == nil {
		return 0
	}
	return int(*v)
}

func dieSize(hitDice string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(strings.ToUpper(hitDice), "D"))
	return n
}

func number(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) {
		return 0
	}
	return f
}

func keys(refs []ref) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.Key)
	}
	return out
}

// slugAfterClass turns "srd-2024_barbarian_rage" into "rage".
func slugAfterClass(key string) string {
	rest := unprefix(key)
	if _, after, found := strings.Cut(rest, "_"); found {
		return after
	}
	return rest
}
