package domain

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"
)

// The export's own schema.
const (
	ExportFormat  = "grimoire-library"
	ExportVersion = 1
)

// Exported is one entry in an export, keyed so Collections can name it.
type Exported struct {
	Key    string
	Kind   string
	Name   string
	Fields Fields
}

// ExportedCollection is one Collection in an export, naming its entries by key.
type ExportedCollection struct {
	Name        string
	Description string
	Entries     []string
}

// Export is Homebrew in Grimoire's own schema.
type Export struct {
	Entries     []Exported
	Collections []ExportedCollection
}

// Incoming is an entry to import as it arrived: its fields and parts not yet checked.
type Incoming struct {
	Key    string
	Kind   string
	Name   string
	Fields map[string]any
	Parts  []map[string]any
}

// Manual is a part of an import Grimoire could not take, to redo by hand.
type Manual struct {
	Where  string
	Reason string
}

// PlanImport checks an import: what can be taken as entries and Collections, and what must be done by
// hand. Nothing is refused whole for one bad part.
func PlanImport(in []Incoming, cols []ExportedCollection) ([]Exported, []ExportedCollection, []Manual) {
	var manual []Manual
	note := func(where, reason string) { manual = append(manual, Manual{Where: where, Reason: reason}) }
	keys := map[string]bool{}
	var entries []Exported
	for i, e := range in {
		if x, ok := planEntry(i, e, keys, note); ok {
			entries = append(entries, x)
		}
	}
	var collections []ExportedCollection
	for k, c := range cols {
		if col, ok := planCollection(k, c, keys, note); ok {
			collections = append(collections, col)
		}
	}
	return entries, collections, manual
}

// planEntry checks one imported entry; its parts are all noted, since no builder runs them yet.
func planEntry(i int, e Incoming, keys map[string]bool, note func(string, string)) (Exported, bool) {
	name := strings.TrimSpace(e.Name)
	where := fmt.Sprintf("entries[%d] %q", i, name)
	switch {
	case !slices.Contains(Kinds(), e.Kind):
		note(where, "the Library keeps no "+e.Kind+" entries")
		return Exported{}, false
	case name == "" || utf8.RuneCountInString(name) > 80:
		note(where, "a name needs 1 to 80 characters")
		return Exported{}, false
	case keys[e.Key]:
		note(where, "another entry already has the key "+e.Key)
		return Exported{}, false
	}
	keys[e.Key] = true
	x := Exported{Key: e.Key, Kind: e.Kind, Name: name, Fields: takeFields(e.Fields, where, note)}
	for j, p := range e.Parts {
		kind, _ := p["type"].(string)
		if kind == "" {
			kind = "untyped"
		}
		note(fmt.Sprintf("%s parts[%d]", where, j), kind+" parts are not supported yet")
	}
	return x, true
}

// planCollection checks one imported Collection, keeping the entries it names that were imported.
func planCollection(k int, c ExportedCollection, keys map[string]bool, note func(string, string)) (ExportedCollection, bool) {
	where := fmt.Sprintf("collections[%d] %q", k, c.Name)
	name, description, err := CleanCollection(c.Name, c.Description)
	if err != nil {
		note(where, "a Collection needs a name of up to 80 characters and a description of up to 2000")
		return ExportedCollection{}, false
	}
	col := ExportedCollection{Name: name, Description: description, Entries: []string{}}
	for _, key := range c.Entries {
		if !keys[key] {
			note(where, "no imported entry has the key "+key)
			continue
		}
		col.Entries = append(col.Entries, key)
	}
	return col, true
}

// takeFields keeps the fields that are named text within the limits and notes the rest.
func takeFields(in map[string]any, where string, note func(string, string)) Fields {
	out := Fields{}
	for _, name := range slices.Sorted(maps.Keys(in)) {
		value, text := in[name].(string)
		clean := strings.TrimSpace(name)
		switch {
		case !text:
			note(where+" fields."+name, "only text values are kept")
		case clean == "" || utf8.RuneCountInString(clean) > MaxFieldName || utf8.RuneCountInString(value) > MaxFieldValue:
			note(where+" fields."+name, "a field needs a name of up to 60 characters and a value of up to 4000")
		case len(out) == MaxFields:
			note(where+" fields."+name, "an entry keeps at most 100 fields")
		default:
			out[clean] = value
		}
	}
	return out
}
