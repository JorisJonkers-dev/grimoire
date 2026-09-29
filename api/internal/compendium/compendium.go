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
