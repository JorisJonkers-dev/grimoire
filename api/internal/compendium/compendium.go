// Package compendium is the read-mostly SRD reference: spells, conditions and their sources.
package compendium

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
)

// ErrNotFound is returned when an entry does not exist in the requested ruleset.
var ErrNotFound = errors.New("compendium: not found")

// SpellFilter narrows a spell listing. Empty fields do not filter.
type SpellFilter struct {
	Query    string
	Level    *int
	School   string
	Class    string
	Ruleset  string
	After    *Cursor
	PageSize int
}

// Cursor marks the last item of a page in name order.
type Cursor struct {
	Name string `json:"n"`
	Slug string `json:"s"`
}

// Encode turns the cursor into an opaque token.
func (c Cursor) Encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodeCursor parses a token produced by Encode; ok is false for anything else.
func DecodeCursor(token string) (Cursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, false
	}
	var c Cursor
	if err := json.Unmarshal(raw, &c); err != nil || c.Slug == "" {
		return Cursor{}, false
	}
	return c, true
}

// SpellSummary is one row of a spell listing.
type SpellSummary struct {
	Slug          string
	Name          string
	Level         int
	School        string
	Ruleset       string
	Ritual        bool
	Concentration bool
}

// Scaling is extra damage at a higher slot or character level.
type Scaling struct {
	Kind       string
	Level      int
	DamageRoll string
}

// Condition is a condition's rules text.
type Condition struct {
	Slug        string
	Name        string
	Description string
}

// Spell is a spell with everything the detail view shows.
type Spell struct {
	SpellSummary
	CastingTime  string
	RangeText    string
	RangeFeet    *int
	Verbal       bool
	Somatic      bool
	Material     bool
	MaterialText string
	Duration     string
	Description  string
	HigherLevel  string
	Classes      []string
	DamageTypes  []string
	SaveAbility  string
	AttackRoll   bool
	DamageRoll   string
	Scaling      []Scaling
	Mentions     []Condition
}

// Source is a document the compendium draws from, with its required attribution.
type Source struct {
	Key         string
	Title       string
	RulesetYear int
	License     string
	Attribution string
	URL         string
}

// FindMentions returns the conditions named in any of the texts, in name order.
func FindMentions(conditions []Condition, texts ...string) []Condition {
	joined := strings.ToLower(strings.Join(texts, "\n"))
	found := []Condition{}
	for _, c := range conditions {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(strings.ToLower(c.Name)) + `\b`)
		if pattern.MatchString(joined) {
			found = append(found, c)
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	return found
}

// EntryFilter narrows a listing of one kind.
type EntryFilter struct {
	Kind     string
	Query    string
	Ruleset  string
	After    *Cursor
	PageSize int
}

// EntrySummary is one row of an entry listing.
type EntrySummary struct {
	Kind     string
	Slug     string
	Name     string
	Subtitle string
	Ruleset  string
}

// Fact is a labelled value in an entry's header, e.g. "Armor Class: 17".
type Fact struct {
	Label string
	Value string
}

// Section is a titled block of rules text.
type Section struct {
	Title string
	Text  string
}

// EntryDetail is an entry rendered for reading.
type EntryDetail struct {
	EntrySummary
	Facts    []Fact
	Sections []Section
	Mentions []Condition
}

// AutomationCount says how much of one kind the rules engine can compute.
type AutomationCount struct {
	Kind    string
	Total   int
	Full    int
	Partial int
	Manual  int
}

// ClassOption is a class a first-level character can take.
type ClassOption struct {
	Slug   string
	Name   string
	HitDie int
	Saves  []string
}

// SpeciesOption is a playable species.
type SpeciesOption struct {
	Slug      string
	Name      string
	SpeedFeet int
}

// BackgroundOption is a background and what it grants. Abilities is empty where the ruleset lets
// the player choose freely.
type BackgroundOption struct {
	Slug      string
	Name      string
	Abilities []string
	Skills    []string
}

// ArmorOption is a suit of armour or a shield. DexCap is negative when uncapped.
type ArmorOption struct {
	Slug             string
	Name             string
	Category         string
	Shield           bool
	ACBase           int
	AddDex           bool
	DexCap           int
	StrengthRequired int
	Stealth          bool
}

// WeaponOption is a weapon a character can start with.
type WeaponOption struct {
	Slug          string
	Name          string
	DamageDice    string
	DamageType    string
	Simple        bool
	RangeFeet     int
	LongRangeFeet int
	Properties    []string
}

// BuilderOptions is everything the character builder offers for one ruleset.
type BuilderOptions struct {
	Ruleset     string
	RulesetYear int
	Classes     []ClassOption
	Species     []SpeciesOption
	Backgrounds []BackgroundOption
	Armor       []ArmorOption
	Weapons     []WeaponOption
}

// ClassLevel is the levels a Character has in a class, and its subclass there, as its traits need them.
type ClassLevel struct {
	Class    string
	Subclass string
	Level    int
}

// Named is a compendium entry by slug and name.
type Named struct {
	Slug string
	Name string
}

// FeatOption is a feat and the category it belongs to: General, Fighting Style, Epic Boon or Origin.
type FeatOption struct {
	Slug        string
	Name        string
	Category    string
	Description string
}

// SpellOption is a cantrip (level 0) or spell on a class's list, whether it can be cast as a ritual,
// and its casting time.
type SpellOption struct {
	Slug        string
	Name        string
	Level       int
	Ritual      bool
	CastingTime string
}

// LevelUpOptions are what a class offers on levelling up: its subclasses, every feat, and the cantrips
// and spells on its list up to a spell level.
type LevelUpOptions struct {
	Subclasses []Named
	Feats      []FeatOption
	Spells     []SpellOption
}

// Trait is a feature or trait a Character has: from its class or subclass at a level, its species, or a
// feat it took.
type Trait struct {
	Name        string
	Source      string
	Level       int
	Description string
}
