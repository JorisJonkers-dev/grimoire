// Package domain holds the Library: an account's reusable entries, linked into Campaigns with a
// Campaign Override and an optional pinned Revision (ADR-0011).
package domain

import (
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// Kinds are what a Library keeps.
func Kinds() []string {
	return []string{"creature", "npc", "location", "shop", "item", "spell", "table"}
}

// Limits on an entry's fields.
const (
	MaxFields     = 100
	MaxFieldName  = 60
	MaxFieldValue = 4000
)

// Fields are an entry's named values.
type Fields map[string]string

// Entry is a Library entry's base: its latest Revision.
type Entry struct {
	ID        uuid.UUID
	Owner     string
	Kind      string
	Name      string
	Fields    Fields
	Revision  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Revision is one saved version of an entry's base.
type Revision struct {
	No     int
	Name   string
	Fields Fields
	Author string
	At     time.Time
}

// Use is a Campaign an entry is linked into, and the Revision it is pinned to there.
type Use struct {
	CampaignID uuid.UUID
	Campaign   string
	Pinned     *int
}

// Detail is an entry with its Revisions, newest first, and where it is used.
type Detail struct {
	Entry
	Revisions []Revision
	Uses      []Use
}

// Linked is an entry as one Campaign sees it: the base it follows (the pinned Revision, or the latest),
// the Campaign Override on top, and the two resolved together.
type Linked struct {
	Entry    Entry
	Pinned   *int
	Base     Fields
	BaseName string
	Override Fields
}

// Resolved is the base with the Campaign Override on top.
func (l Linked) Resolved() Fields {
	out := maps.Clone(l.Base)
	if out == nil {
		out = Fields{}
	}
	maps.Copy(out, l.Override)
	return out
}

// Draft is a new or changed entry.
type Draft struct {
	Kind   string
	Name   string
	Fields Fields
}

// Clean checks a Draft: a known kind, a name, and fields within the limits.
func (d Draft) Clean() (Draft, error) {
	if !slices.Contains(Kinds(), d.Kind) {
		return d, apperr.Refuse("choose a kind the Library keeps: " + strings.Join(Kinds(), ", "))
	}
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || utf8.RuneCountInString(d.Name) > 80 {
		return d, apperr.Refuse("give it a name of up to 80 characters")
	}
	f, err := CleanFields(d.Fields)
	d.Fields = f
	return d, err
}

// CleanFields checks named values: at most MaxFields, each name 1 to MaxFieldName characters and each
// value at most MaxFieldValue.
func CleanFields(in Fields) (Fields, error) {
	if len(in) > MaxFields {
		return nil, apperr.Refuse("keep it to 100 fields")
	}
	out := Fields{}
	for name, value := range in {
		name = strings.TrimSpace(name)
		if name == "" || utf8.RuneCountInString(name) > MaxFieldName || utf8.RuneCountInString(value) > MaxFieldValue {
			return nil, apperr.Refuse("name each field in up to 60 characters, with a value of up to 4000")
		}
		out[name] = value
	}
	return out, nil
}
