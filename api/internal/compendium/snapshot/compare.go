package snapshot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Kind names one collection of entries in the snapshot JSON, e.g. "spell" in "spells".
type Kind struct {
	Name       string
	Collection string
}

// Kinds lists every entry collection of a snapshot.
func Kinds() []Kind {
	return []Kind{
		{"spell", "spells"},
		{"condition", "conditions"},
		{"class", "classes"},
		{"species", "species"},
		{"background", "backgrounds"},
		{"feat", "feats"},
		{"weapon", "weapons"},
		{"armor", "armor"},
		{"item", "items"},
		{"monster", "monsters"},
	}
}

// CollectionOf returns the JSON collection holding a kind's entries.
func CollectionOf(kind string) (string, bool) {
	for _, k := range Kinds() {
		if k.Name == kind {
			return k.Collection, true
		}
	}
	return "", false
}

// Ref names one entry.
type Ref struct {
	Kind     string
	Document string
	Slug     string
}

func (r Ref) String() string { return r.Kind + " " + r.Document + "/" + r.Slug }

// Changed is an entry present in both snapshots whose fields differ.
type Changed struct {
	Ref
	Fields []string
}

// Count is the number of entries of one kind before and after.
type Count struct {
	Kind   string
	Before int
	After  int
}

// Changes is what a snapshot refresh changed.
type Changes struct {
	Counts  []Count
	Added   []Ref
	Removed []Ref
	Changed []Changed
}

// Compare reports what changed from before to after, entry by entry.
func Compare(before, after Snapshot) (Changes, error) {
	old, err := before.Tree()
	if err != nil {
		return Changes{}, err
	}
	next, err := after.Tree()
	if err != nil {
		return Changes{}, err
	}
	c := Changes{Counts: []Count{}, Added: []Ref{}, Removed: []Ref{}, Changed: []Changed{}}
	for _, k := range Kinds() {
		c.compare(k.Name, Entries(old, k.Collection), Entries(next, k.Collection))
	}
	return c, nil
}

func (c *Changes) compare(kind string, was, is []map[string]any) {
	c.Counts = append(c.Counts, Count{Kind: kind, Before: len(was), After: len(is)})
	wasBy, isBy := byIdentity(was), byIdentity(is)
	for _, id := range sortedKeys(isBy) {
		ref := Ref{Kind: kind, Document: id[0], Slug: id[1]}
		prev, ok := wasBy[id]
		if !ok {
			c.Added = append(c.Added, ref)
			continue
		}
		if fields := changedFields(prev, isBy[id]); len(fields) > 0 {
			c.Changed = append(c.Changed, Changed{Ref: ref, Fields: fields})
		}
	}
	for _, id := range sortedKeys(wasBy) {
		if _, ok := isBy[id]; !ok {
			c.Removed = append(c.Removed, Ref{Kind: kind, Document: id[0], Slug: id[1]})
		}
	}
}

// Text renders the changes for a reviewer.
func (c Changes) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Snapshot refresh: %d added, %d removed, %d changed\n\nEntries per kind:\n", len(c.Added), len(c.Removed), len(c.Changed))
	for _, n := range c.Counts {
		fmt.Fprintf(&b, "  %s: %d -> %d\n", n.Kind, n.Before, n.After)
	}
	for _, r := range c.Added {
		fmt.Fprintf(&b, "+ %s\n", r)
	}
	for _, r := range c.Removed {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	for _, ch := range c.Changed {
		fmt.Fprintf(&b, "~ %s: %s\n", ch.Ref, strings.Join(ch.Fields, ", "))
	}
	return b.String()
}

// Tree is the snapshot as decoded JSON, the form corrections are applied to.
func (s Snapshot) Tree() (map[string]any, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("snapshot: encode: %w", err)
	}
	var tree map[string]any
	return tree, json.Unmarshal(raw, &tree)
}

// FromTree decodes a tree strictly: a field the snapshot types do not have is refused.
func FromTree(tree map[string]any) (Snapshot, error) {
	raw, err := json.Marshal(tree)
	if err != nil {
		return Snapshot{}, fmt.Errorf("snapshot: encode: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var s Snapshot
	if err := dec.Decode(&s); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return s, nil
}

// CanonicalEntry decodes one entry of a kind strictly and returns it as the snapshot writes it.
func CanonicalEntry(kind string, entry map[string]any) (map[string]any, error) {
	collection, ok := CollectionOf(kind)
	if !ok {
		return nil, fmt.Errorf("%w: unknown kind %q", ErrInvalid, kind)
	}
	s, err := FromTree(map[string]any{collection: []any{entry}})
	if err != nil {
		return nil, err
	}
	tree, err := s.Tree()
	if err != nil {
		return nil, err
	}
	return Entries(tree, collection)[0], nil
}

// Entries returns a collection's entries from a tree.
func Entries(tree map[string]any, collection string) []map[string]any {
	list, _ := tree[collection].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, e := range list {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func byIdentity(entries []map[string]any) map[[2]string]map[string]any {
	out := map[[2]string]map[string]any{}
	for _, e := range entries {
		doc, _ := e["document"].(string)
		slug, _ := e["slug"].(string)
		out[[2]string{doc, slug}] = e
	}
	return out
}

func sortedKeys(m map[[2]string]map[string]any) [][2]string {
	out := make([][2]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0]+"/"+out[i][1] < out[j][0]+"/"+out[j][1] })
	return out
}

func changedFields(before, after map[string]any) []string {
	fields := []string{}
	for k, v := range after {
		if !reflect.DeepEqual(before[k], v) {
			fields = append(fields, k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			fields = append(fields, k)
		}
	}
	sort.Strings(fields)
	return fields
}
