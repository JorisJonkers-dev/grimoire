// Package designs reads the SRD Design files: one strict YAML file per entry stating its Effects in the
// engine's own Part kinds, checked against the Compendium seed before anything is loaded.
package designs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
)

// Root is the folder of the SRD 2024 Design files, named after the seed document they automate.
const Root = "srd-2024"

// Set is every Effect the Design files state, the entries listed as having none, and a hash over every
// file so an unchanged set is not loaded again.
type Set struct {
	Effects  []Loaded
	NoEffect []NoEffect
	Hash     string
}

// Loaded is one Effect and the entry that owns it.
type Loaded struct {
	Owner      effects.Owner
	Definition effects.Definition
}

// NoEffect is an entry that needs no Effect, with the reason.
type NoEffect struct {
	Entry  string `yaml:"entry"`
	Reason string `yaml:"reason"`
}

// Read reads every Design file under Root and checks it against the seed; it reports every problem at
// once, each with the file it is in.
func Read(fsys fs.FS, snap snapshot.Snapshot) (Set, error) {
	r := &reader{fsys: fsys, seed: entries(snap), owners: map[string]string{}}
	raw, err := r.walk()
	if err != nil {
		return Set{}, err
	}
	r.exceptions = r.approved(raw["_exceptions.yaml"])
	for _, p := range sortedPaths(raw) {
		r.file(p, raw[p])
	}
	if len(r.problems) > 0 {
		return Set{}, errors.Join(r.problems...)
	}
	return Set{Effects: r.effects, NoEffect: r.noEffect, Hash: hash(raw)}, nil
}

type reader struct {
	fsys       fs.FS
	seed       map[string]string
	exceptions map[string]exception
	owners     map[string]string
	effects    []Loaded
	noEffect   []NoEffect
	problems   []error
}

func (r *reader) bad(file, format string, a ...any) {
	r.problems = append(r.problems, fmt.Errorf("%s: %s", file, fmt.Sprintf(format, a...)))
}

// walk reads every file under Root by its path below it.
func (r *reader) walk() (map[string][]byte, error) {
	raw := map[string][]byte{}
	err := fs.WalkDir(r.fsys, Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(r.fsys, p)
		raw[strings.TrimPrefix(p, Root+"/")] = body
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("designs: %w", err)
	}
	return raw, nil
}

// file reads one file by what its path says it is.
func (r *reader) file(p string, body []byte) {
	folder, _, _ := strings.Cut(p, "/")
	switch {
	case path.Base(p) == "README.md", p == "_exceptions.yaml":
	case path.Base(p) == "_no-effect.yaml":
		r.listed(p, folder, body)
	case !strings.HasSuffix(p, ".yaml"):
		r.bad(p, "a Design file is YAML, named <entry>.yaml")
	case pending(folder) != "":
		r.bad(p, "%s load once their link lands (%s)", folder, pending(folder))
	case owner(folder) == "":
		r.bad(p, "no kind of entry is kept in %s", folder)
	default:
		r.design(p, body)
	}
}

// owner is what owns the Effects of the files in a folder that loads.
func owner(folder string) effects.OwnerKind {
	switch folder {
	case "spells":
		return effects.OwnedBySpell
	case "conditions":
		return effects.OwnedByCondition
	}
	return ""
}

// pending is the ticket that links the entries of a folder that does not load yet.
func pending(folder string) string {
	switch folder {
	case "features", "species", "feats":
		return "#262"
	case "items":
		return "#263"
	case "monsters", "traits":
		return "#264"
	}
	return ""
}

// strict decodes YAML refusing any field the shape does not name.
func strict(body []byte, v any) error {
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("does not read: %w", err)
	}
	return nil
}

func hash(raw map[string][]byte) string {
	h := sha256.New()
	for _, p := range sortedPaths(raw) {
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(raw[p])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
