package corrections

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

// Apply corrects the snapshot and warns for each correction upstream now matches.
func Apply(snap snapshot.Snapshot, cs []Correction) (snapshot.Snapshot, []Warning, error) {
	tree, err := snap.Tree()
	if err != nil {
		return snapshot.Snapshot{}, nil, err
	}
	warnings := []Warning{}
	for _, c := range cs {
		w, err := apply(tree, c)
		if err != nil {
			return snapshot.Snapshot{}, nil, fmt.Errorf("corrections: %s: %w", c.File, err)
		}
		warnings = append(warnings, w...)
	}
	out, err := snapshot.FromTree(tree)
	if err != nil {
		return snapshot.Snapshot{}, nil, err
	}
	if err := out.Validate(); err != nil {
		return snapshot.Snapshot{}, nil, fmt.Errorf("corrections: %w", err)
	}
	return out, warnings, nil
}

func apply(tree map[string]any, c Correction) ([]Warning, error) {
	collection, _ := snapshot.CollectionOf(c.Kind)
	list, _ := tree[collection].([]any)
	i := indexOf(list, c.Document, c.Entry)
	if c.Add != nil {
		return add(tree, collection, list, i, c)
	}
	if i < 0 {
		return nil, fmt.Errorf("%w: upstream has no %s %s/%s", ErrInvalid, c.Kind, c.Document, c.Entry)
	}
	entry, _ := list[i].(map[string]any)
	matched := []string{}
	for _, f := range c.Fields {
		steps, err := parsePath(f.Field)
		if err != nil {
			return nil, err
		}
		same, err := set(entry, f.Field, steps, f.Value)
		if err != nil {
			return nil, err
		}
		if same {
			matched = append(matched, f.Field)
		}
	}
	canonical, err := snapshot.CanonicalEntry(c.Kind, entry)
	if err != nil {
		return nil, err
	}
	list[i] = canonical
	switch {
	case len(matched) == 0:
		return nil, nil
	case len(matched) == len(c.Fields):
		return []Warning{{File: c.File, Message: "upstream now matches every corrected field; delete the file", Delete: true}}, nil
	default:
		return []Warning{{File: c.File, Message: "upstream now matches " + strings.Join(matched, ", ") + "; drop those fields from the file"}}, nil
	}
}

func add(tree map[string]any, collection string, list []any, i int, c Correction) ([]Warning, error) {
	entry := map[string]any{"document": c.Document, "slug": c.Entry}
	for k, v := range c.Add {
		entry[k] = v
	}
	canonical, err := snapshot.CanonicalEntry(c.Kind, entry)
	if err != nil {
		return nil, err
	}
	if i < 0 {
		tree[collection] = append(list, canonical)
		return nil, nil
	}
	same := reflect.DeepEqual(list[i], canonical)
	list[i] = canonical
	if same {
		return []Warning{{File: c.File, Message: "upstream now carries this entry as corrected; delete the file", Delete: true}}, nil
	}
	return []Warning{{File: c.File, Message: "upstream now carries this entry, with other values; compare them, then turn the file into field corrections or delete it"}}, nil
}

func indexOf(list []any, document, slug string) int {
	for i, e := range list {
		if m, ok := e.(map[string]any); ok && m["document"] == document && m["slug"] == slug {
			return i
		}
	}
	return -1
}

type step struct {
	key, matchKey, matchValue string
}

var stepPattern = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]*)(?:\[([A-Za-z][A-Za-z0-9]*)=([^\]]+)\])?$`)

// parsePath reads "a.b[key=value].c": object keys, and list items picked by one field's value.
func parsePath(p string) ([]step, error) {
	var parts []string
	depth, start := 0, 0
	for i, r := range p {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		case '.':
			if depth == 0 {
				parts, start = append(parts, p[start:i]), i+1
			}
		}
	}
	steps := []step{}
	for _, part := range append(parts, p[start:]) {
		m := stepPattern.FindStringSubmatch(part)
		if m == nil {
			return nil, fmt.Errorf("%w: field %q: bad step %q", ErrInvalid, p, part)
		}
		steps = append(steps, step{key: m[1], matchKey: m[2], matchValue: m[3]})
	}
	return steps, nil
}

// set writes value at the path and reports whether upstream already held it.
func set(cur any, p string, steps []step, value any) (bool, error) {
	obj, ok := cur.(map[string]any)
	if !ok {
		return false, fmt.Errorf("%w: %s: %s is inside something that is not an object", ErrInvalid, p, steps[0].key)
	}
	st, rest := steps[0], steps[1:]
	if st.matchKey == "" {
		old, had := obj[st.key]
		if len(rest) == 0 {
			obj[st.key] = value
			return had && reflect.DeepEqual(old, value), nil
		}
		if !had {
			return false, fmt.Errorf("%w: %s: no %s", ErrInvalid, p, st.key)
		}
		return set(old, p, rest, value)
	}
	list, ok := obj[st.key].([]any)
	if !ok {
		return false, fmt.Errorf("%w: %s: %s is not a list", ErrInvalid, p, st.key)
	}
	found := matches(list, st.matchKey, st.matchValue)
	switch {
	case len(found) == 0 && renamed(list, st, rest, value):
		return true, nil
	case len(found) != 1:
		return false, fmt.Errorf("%w: %s: %d items in %s have %s %q, want one", ErrInvalid, p, len(found), st.key, st.matchKey, st.matchValue)
	case len(rest) == 0:
		same := reflect.DeepEqual(list[found[0]], value)
		list[found[0]] = value
		return same, nil
	}
	return set(list[found[0]], p, rest, value)
}

// renamed is a correction of the very field an item is picked by, which upstream has made already.
func renamed(list []any, st step, rest []step, value any) bool {
	name, ok := value.(string)
	return ok && len(rest) == 1 && rest[0].matchKey == "" && rest[0].key == st.matchKey && len(matches(list, st.matchKey, name)) == 1
}

func matches(list []any, key, want string) []int {
	out := []int{}
	for i, e := range list {
		if m, ok := e.(map[string]any); ok && fmt.Sprint(m[key]) == want {
			out = append(out, i)
		}
	}
	return out
}
