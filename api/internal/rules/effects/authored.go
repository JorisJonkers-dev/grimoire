package effects

import (
	"encoding/json"
	"slices"
	"strconv"
)

// Authored is a Part as its author writes it, before its kind checks it: the kind's slug, its values by
// field name, the Parts beneath it in named groups, and how its fields scale. It is the one shape every
// Part takes on the wire.
type Authored struct {
	Kind    string            `json:"kind"`
	Values  Values            `json:"values"`
	Groups  []AuthoredGroup   `json:"groups"`
	Scaling []AuthoredScaling `json:"scaling,omitempty"`
}

// AuthoredGroup is one named group of authored Parts beneath a Part.
type AuthoredGroup struct {
	Name  string     `json:"name"`
	Parts []Authored `json:"parts"`
}

// MarshalJSON writes no values and no groups as empty rather than null, as the wire requires.
func (a Authored) MarshalJSON() ([]byte, error) {
	type plain Authored
	if a.Values == nil {
		a.Values = Values{}
	}
	if a.Groups == nil {
		a.Groups = []AuthoredGroup{}
	}
	return json.Marshal(plain(a))
}

// MarshalJSON writes a group of no Parts as empty rather than null.
func (g AuthoredGroup) MarshalJSON() ([]byte, error) {
	type plain AuthoredGroup
	if g.Parts == nil {
		g.Parts = []Authored{}
	}
	return json.Marshal(plain(g))
}

// PartError is an authored Part its kind refuses, with where it sits.
type PartError struct {
	At  string
	Err error
}

func (e PartError) Error() string { return e.At + ": " + e.Err.Error() }

func (e PartError) Unwrap() error { return e.Err }

// maxDepth is how deep Parts may nest beneath the Effect's own.
const maxDepth = 4

// KindOf is the kind a slug names.
func KindOf(slug string) (Kind, bool) {
	for _, k := range Kinds() {
		if k.Describe().Slug == slug {
			return k, true
		}
	}
	return nil, false
}

// Decode checks authored Parts against their kinds' definitions and makes them, the Parts beneath each
// first.
func Decode(in []Authored) ([]Part, error) {
	return decode(in, 0)
}

func decode(in []Authored, depth int) ([]Part, error) {
	out := make([]Part, 0, len(in))
	for i, a := range in {
		p, err := a.decode(depth)
		if err != nil {
			return nil, PartError{At: "part " + strconv.Itoa(i+1), Err: err}
		}
		out = append(out, p)
	}
	return out, nil
}

func (a Authored) decode(depth int) (Part, error) {
	k, ok := KindOf(a.Kind)
	if !ok {
		return nil, refuse("kind", "choose a kind the rules know")
	}
	d := k.Describe()
	for name := range a.Values {
		if !slices.ContainsFunc(d.Fields, func(f Field) bool { return f.Name == name }) {
			return nil, refuse(name, d.Label+" has no such field")
		}
	}
	groups, err := a.beneath(d, depth)
	if err != nil {
		return nil, err
	}
	return k.Decode(a.Values, groups)
}

// beneath decodes the groups of Parts beneath a Part, each one its kind holds and named once.
func (a Authored) beneath(d Description, depth int) (Groups, error) {
	if len(a.Groups) > 0 && depth == maxDepth {
		return nil, refuse("groups", "nest Parts at most "+strconv.Itoa(maxDepth)+" deep")
	}
	out := make(Groups, 0, len(a.Groups))
	for _, g := range a.Groups {
		if !d.OpenGroups && !slices.Contains(d.Groups, g.Name) {
			return nil, refuse("groups", d.Label+" holds no group named "+g.Name)
		}
		if slices.ContainsFunc(out, func(o Group) bool { return o.Name == g.Name }) {
			return nil, refuse("groups", "name each group once")
		}
		parts, err := decode(g.Parts, depth+1)
		if err != nil {
			return nil, PartError{At: g.Name, Err: err}
		}
		out = append(out, Group{Name: g.Name, Parts: parts})
	}
	return out, nil
}
