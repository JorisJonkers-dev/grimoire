// Package corrections restores SRD text Open5e gets wrong. One YAML file per corrected entry, under
// <kind>/<entry>.yaml, is applied when the snapshot is built.
package corrections

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

// ErrInvalid wraps every malformed or misfitting correction.
var ErrInvalid = errors.New("corrections: invalid")

// Source is where the corrected value is printed.
type Source struct {
	Document string `yaml:"document"`
	Page     int    `yaml:"page"`
}

// Report is the upstream report drafted for Open5e; Filed holds its URL once the maintainer filed it.
type Report struct {
	Title string `yaml:"title"`
	Body  string `yaml:"body"`
	Filed string `yaml:"filed"`
}

// Field sets one field of the entry, addressed by its path in the snapshot JSON, e.g.
// "description" or "actions[name=Parry].name".
type Field struct {
	Field string
	Value any
}

// Correction is one correction file: field corrections to an upstream entry, or a whole added entry.
type Correction struct {
	File     string
	Kind     string
	Entry    string
	Document string
	Source   Source
	Fields   []Field
	Add      map[string]any
	Upstream *Report
}

// Warning is a correction upstream has caught up with; Delete means the file can go.
type Warning struct {
	File    string
	Message string
	Delete  bool
}

type file struct {
	Entry    string         `yaml:"entry"`
	Document string         `yaml:"document"`
	Source   Source         `yaml:"source"`
	Fields   []fileField    `yaml:"fields"`
	Add      map[string]any `yaml:"add"`
	Upstream *Report        `yaml:"upstream"`
}

type fileField struct {
	Field string    `yaml:"field"`
	Value yaml.Node `yaml:"value"`
}

// Load reads every correction file, refusing unknown fields; a missing directory holds none.
func Load(fsys fs.FS) ([]Correction, error) {
	out := []Correction{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		c, err := read(fsys, p)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrInvalid, p, err)
		}
		out = append(out, c)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return []Correction{}, nil
	}
	return out, err
}

func read(fsys fs.FS, p string) (Correction, error) {
	kind, name, ok := strings.Cut(p, "/")
	if !ok || strings.Contains(name, "/") || path.Ext(name) != ".yaml" {
		return Correction{}, errors.New("want <kind>/<entry>.yaml")
	}
	if _, known := snapshot.CollectionOf(kind); !known {
		return Correction{}, fmt.Errorf("unknown kind %q", kind)
	}
	raw, err := fs.ReadFile(fsys, p)
	if err != nil {
		return Correction{}, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var f file
	if err := dec.Decode(&f); err != nil {
		return Correction{}, err
	}
	var more any
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return Correction{}, errors.New("one YAML document per file")
	}
	if err := f.check(strings.TrimSuffix(name, ".yaml")); err != nil {
		return Correction{}, err
	}
	return f.correction(p, kind)
}

func (f file) check(entry string) error {
	_, addsDocument := f.Add["document"]
	_, addsSlug := f.Add["slug"]
	switch {
	case f.Entry != entry:
		return fmt.Errorf("entry %q does not match the file name", f.Entry)
	case f.Document == "":
		return errors.New("no document")
	case f.Source.Document == "" || f.Source.Page < 1:
		return errors.New("the source needs a document and a page")
	case (len(f.Fields) > 0) == (f.Add != nil):
		return errors.New("want fields or add, not both")
	case addsDocument || addsSlug:
		return errors.New("an added entry takes its document and slug from the file")
	case f.Upstream != nil && (f.Upstream.Title == "" || f.Upstream.Body == ""):
		return errors.New("an upstream report needs a title and a body")
	}
	return nil
}

func (f file) correction(p, kind string) (Correction, error) {
	c := Correction{File: p, Kind: kind, Entry: f.Entry, Document: f.Document, Source: f.Source, Upstream: f.Upstream}
	if f.Add != nil {
		add, err := jsonValue(f.Add)
		if err != nil {
			return Correction{}, err
		}
		c.Add, _ = add.(map[string]any)
	}
	for _, ff := range f.Fields {
		if _, err := parsePath(ff.Field); err != nil {
			return Correction{}, err
		}
		if ff.Value.Kind == 0 {
			return Correction{}, fmt.Errorf("field %q has no value", ff.Field)
		}
		var v any
		if err := ff.Value.Decode(&v); err != nil {
			return Correction{}, err
		}
		value, err := jsonValue(v)
		if err != nil {
			return Correction{}, err
		}
		c.Fields = append(c.Fields, Field{Field: ff.Field, Value: value})
	}
	return c, nil
}

// jsonValue turns a YAML value into the form the snapshot tree holds, trimming text as the importer does.
func jsonValue(v any) (any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return trim(out), nil
}

func trim(v any) any {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case map[string]any:
		for k, e := range x {
			x[k] = trim(e)
		}
	case []any:
		for i, e := range x {
			x[i] = trim(e)
		}
	}
	return v
}
