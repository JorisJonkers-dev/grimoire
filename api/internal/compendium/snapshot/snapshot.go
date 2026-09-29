// Package snapshot is the pinned, license-clean copy of SRD reference data that Grimoire imports.
package snapshot

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
)

// File is the snapshot's name inside the seeds directory; it is gzip-compressed JSON.
const File = "compendium.json.gz"

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

// Named is a titled block of rules text, e.g. a trait or a benefit.
type Named struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Entry is what every non-spell entry shares.
type Entry struct {
	Document    string `json:"document"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ClassFeature is a class feature and the levels it is gained at.
type ClassFeature struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Levels      []int  `json:"levels"`
}

// Class is a class or, with Parent set, a subclass.
type Class struct {
	Entry
	Parent       string         `json:"parent,omitempty"`
	HitDie       int            `json:"hitDie,omitempty"`
	CasterType   string         `json:"casterType,omitempty"`
	SavingThrows []string       `json:"savingThrows"`
	Features     []ClassFeature `json:"features"`
}

// Species is a playable species (2024) or race (2014).
type Species struct {
	Entry
	Subspecies bool    `json:"subspecies"`
	Traits     []Named `json:"traits"`
}

// Background is a character background.
type Background struct {
	Entry
	Benefits []Named `json:"benefits"`
}

// Feat is a feat.
type Feat struct {
	Entry
	Type         string   `json:"type,omitempty"`
	Prerequisite string   `json:"prerequisite,omitempty"`
	Benefits     []string `json:"benefits"`
}

// WeaponProperty is a weapon property or mastery with its detail, e.g. Versatile (1d10).
type WeaponProperty struct {
	Name    string `json:"name"`
	Mastery bool   `json:"mastery"`
	Detail  string `json:"detail,omitempty"`
}

// Weapon is a weapon's combat data.
type Weapon struct {
	Entry
	DamageDice    string           `json:"damageDice"`
	DamageType    string           `json:"damageType"`
	RangeFeet     int              `json:"rangeFeet"`
	LongRangeFeet int              `json:"longRangeFeet"`
	Simple        bool             `json:"simple"`
	Properties    []WeaponProperty `json:"properties"`
}

// Armor is a suit of armour or a shield.
type Armor struct {
	Entry
	Category            string `json:"category"`
	ACBase              int    `json:"acBase"`
	AddDex              bool   `json:"addDex"`
	DexCap              *int   `json:"dexCap,omitempty"`
	StealthDisadvantage bool   `json:"stealthDisadvantage"`
	StrengthRequired    *int   `json:"strengthRequired,omitempty"`
}

// Item is mundane equipment or a magic item.
type Item struct {
	Entry
	Category           string  `json:"category"`
	CostGP             float64 `json:"costGp"`
	WeightLB           float64 `json:"weightLb"`
	Magic              bool    `json:"magic"`
	Rarity             string  `json:"rarity,omitempty"`
	RequiresAttunement bool    `json:"requiresAttunement"`
	AttunementDetail   string  `json:"attunementDetail,omitempty"`
}

// Attack is the structured part of a monster action.
type Attack struct {
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	ToHit         int    `json:"toHit"`
	ReachFeet     int    `json:"reachFeet,omitempty"`
	RangeFeet     int    `json:"rangeFeet,omitempty"`
	LongRangeFeet int    `json:"longRangeFeet,omitempty"`
	DamageDice    string `json:"damageDice,omitempty"`
	DamageBonus   int    `json:"damageBonus,omitempty"`
	DamageType    string `json:"damageType,omitempty"`
	ExtraDice     string `json:"extraDice,omitempty"`
	ExtraType     string `json:"extraType,omitempty"`
}

// Action is a monster action, bonus action, reaction or legendary action.
type Action struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Attacks     []Attack `json:"attacks"`
}

// Monster is a creature's statblock.
type Monster struct {
	Entry
	Size                string         `json:"size"`
	Type                string         `json:"type"`
	Alignment           string         `json:"alignment"`
	ArmorClass          int            `json:"armorClass"`
	ArmorDetail         string         `json:"armorDetail,omitempty"`
	HitPoints           int            `json:"hitPoints"`
	HitDice             string         `json:"hitDice"`
	ChallengeRating     float64        `json:"challengeRating"`
	XP                  int            `json:"xp"`
	Abilities           map[string]int `json:"abilities"`
	Saves               map[string]int `json:"saves"`
	Skills              map[string]int `json:"skills"`
	Speeds              map[string]int `json:"speeds"`
	Senses              map[string]int `json:"senses"`
	PassivePerception   int            `json:"passivePerception"`
	Languages           string         `json:"languages,omitempty"`
	Resistances         []string       `json:"resistances"`
	Immunities          []string       `json:"immunities"`
	Vulnerabilities     []string       `json:"vulnerabilities"`
	ConditionImmunities []string       `json:"conditionImmunities"`
	Traits              []Named        `json:"traits"`
	Actions             []Action       `json:"actions"`
}

// Snapshot is everything imported into the compendium.
type Snapshot struct {
	Source      string       `json:"source"`
	Documents   []Document   `json:"documents"`
	Spells      []Spell      `json:"spells"`
	Conditions  []Condition  `json:"conditions"`
	Classes     []Class      `json:"classes"`
	Species     []Species    `json:"species"`
	Backgrounds []Background `json:"backgrounds"`
	Feats       []Feat       `json:"feats"`
	Weapons     []Weapon     `json:"weapons"`
	Armor       []Armor      `json:"armor"`
	Items       []Item       `json:"items"`
	Monsters    []Monster    `json:"monsters"`
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ErrInvalid wraps every validation failure.
var ErrInvalid = errors.New("snapshot: invalid")

// Load reads and validates the snapshot, returning it with a content hash.
func Load(fsys fs.FS) (Snapshot, string, error) {
	compressed, err := fs.ReadFile(fsys, File)
	if err != nil {
		return Snapshot{}, "", fmt.Errorf("snapshot: read: %w", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return Snapshot{}, "", fmt.Errorf("snapshot: gunzip: %w", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		return Snapshot{}, "", fmt.Errorf("snapshot: gunzip: %w", err)
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
	if err := s.validateConditions(docs); err != nil {
		return err
	}
	return s.validateEntries(docs)
}

func (s Snapshot) entries() []Entry {
	out := []Entry{}
	for _, c := range s.Classes {
		out = append(out, c.Entry)
	}
	for _, x := range s.Species {
		out = append(out, x.Entry)
	}
	for _, x := range s.Backgrounds {
		out = append(out, x.Entry)
	}
	for _, x := range s.Feats {
		out = append(out, x.Entry)
	}
	for _, x := range s.Weapons {
		out = append(out, x.Entry)
	}
	for _, x := range s.Armor {
		out = append(out, x.Entry)
	}
	for _, x := range s.Items {
		out = append(out, x.Entry)
	}
	for _, x := range s.Monsters {
		out = append(out, x.Entry)
	}
	return out
}

func (s Snapshot) validateEntries(docs map[string]bool) error {
	for _, e := range s.entries() {
		if !docs[e.Document] || !slugPattern.MatchString(e.Slug) || e.Name == "" {
			return fmt.Errorf("%w: entry %s/%q is invalid", ErrInvalid, e.Document, e.Slug)
		}
	}
	return nil
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
