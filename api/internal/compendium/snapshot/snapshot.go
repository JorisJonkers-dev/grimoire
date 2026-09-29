// Package snapshot is the pinned, license-clean copy of SRD reference data that Grimoire imports.
package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
)

// File is the snapshot's name inside the seeds directory.
const File = "compendium.json"

// Document is one source of rules content, e.g. SRD 5.2.
type Document struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	RulesetYear int    `json:"rulesetYear"`
	Precedence  int    `json:"precedence"`
	License     string `json:"license"`
	Attribution string `json:"attribution"`
	URL         string `json:"url"`
}

// Scaling is extra damage when a spell is cast with a higher slot or at a higher character level.
type Scaling struct {
	Kind       string `json:"kind"`
	Level      int    `json:"level"`
	DamageRoll string `json:"damageRoll"`
}

// Spell is a spell as the SRD defines it.
type Spell struct {
	Document      string    `json:"document"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Level         int       `json:"level"`
	School        string    `json:"school"`
	CastingTime   string    `json:"castingTime"`
	RangeText     string    `json:"rangeText"`
	RangeFeet     *int      `json:"rangeFeet,omitempty"`
	Verbal        bool      `json:"verbal"`
	Somatic       bool      `json:"somatic"`
	Material      bool      `json:"material"`
	MaterialText  string    `json:"materialText,omitempty"`
	Ritual        bool      `json:"ritual"`
	Concentration bool      `json:"concentration"`
	Duration      string    `json:"duration"`
	Description   string    `json:"description"`
	HigherLevel   string    `json:"higherLevel,omitempty"`
	Classes       []string  `json:"classes"`
	SaveAbility   string    `json:"saveAbility,omitempty"`
	AttackRoll    bool      `json:"attackRoll"`
	DamageRoll    string    `json:"damageRoll,omitempty"`
	DamageTypes   []string  `json:"damageTypes"`
	Scaling       []Scaling `json:"scaling"`
}

// Condition is a condition's rules text in one document.
type Condition struct {
	Document    string `json:"document"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Snapshot is everything imported into the compendium.
type Snapshot struct {
	Source     string      `json:"source"`
	Documents  []Document  `json:"documents"`
	Spells     []Spell     `json:"spells"`
	Conditions []Condition `json:"conditions"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ErrInvalid wraps every validation failure.
var ErrInvalid = errors.New("snapshot: invalid")

// Load reads and validates the snapshot, returning it with a content hash.
func Load(fsys fs.FS) (Snapshot, string, error) {
	raw, err := fs.ReadFile(fsys, File)
	if err != nil {
		return Snapshot{}, "", fmt.Errorf("snapshot: read: %w", err)
	}
	var s Snapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return Snapshot{}, "", fmt.Errorf("snapshot: decode: %w", err)
	}
	if err := s.Validate(); err != nil {
		return Snapshot{}, "", err
	}
	sum := sha256.Sum256(raw)
	return s, hex.EncodeToString(sum[:]), nil
}

// Validate checks the invariants the database also enforces, so a bad snapshot fails before import.
func (s Snapshot) Validate() error {
	docs, err := s.validateDocuments()
	if err != nil {
		return err
	}
	if err := s.validateSpells(docs); err != nil {
		return err
	}
	return s.validateConditions(docs)
}

func (s Snapshot) validateDocuments() (map[string]bool, error) {
	docs := map[string]bool{}
	for _, d := range s.Documents {
		if d.Key == "" || d.Title == "" || d.License == "" || d.Attribution == "" {
			return nil, fmt.Errorf("%w: document %q is incomplete", ErrInvalid, d.Key)
		}
		docs[d.Key] = true
	}
	return docs, nil
}

func (s Snapshot) validateSpells(docs map[string]bool) error {
	for _, sp := range s.Spells {
		if !docs[sp.Document] {
			return fmt.Errorf("%w: spell %q references unknown document %q", ErrInvalid, sp.Slug, sp.Document)
		}
		if !slugPattern.MatchString(sp.Slug) || sp.Name == "" || sp.School == "" {
			return fmt.Errorf("%w: spell %q has a bad slug, name or school", ErrInvalid, sp.Slug)
		}
		if sp.Level < 0 || sp.Level > 9 {
			return fmt.Errorf("%w: spell %q has level %d", ErrInvalid, sp.Slug, sp.Level)
		}
	}
	return nil
}

func (s Snapshot) validateConditions(docs map[string]bool) error {
	for _, c := range s.Conditions {
		if !docs[c.Document] || !slugPattern.MatchString(c.Slug) || c.Description == "" {
			return fmt.Errorf("%w: condition %q is invalid", ErrInvalid, c.Slug)
		}
	}
	return nil
}
