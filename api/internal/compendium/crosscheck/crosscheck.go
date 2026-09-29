// Package crosscheck compares a compendium snapshot with the 5e-bits SRD API and reports where they disagree.
// Like the snapshot command it runs offline of the request path.
package crosscheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

// Client talks to a 5e-bits compatible API.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Workers int
}

// Finding is one difference between the two sources.
type Finding struct {
	Ruleset string
	Kind    string
	Slug    string
	Field   string
	Ours    string
	Theirs  string
}

// Report is the outcome of a cross-check.
type Report struct {
	Compared      []string
	Unavailable   []string
	Missing       []Finding
	Extra         []Finding
	Disagreements []Finding
}

var errNotFound = errors.New("not found")

type resource struct {
	kind, path string
	ours       func(snapshot.Snapshot, string) []string
}

// pathFor handles the one resource 5e-bits renamed between rulesets.
func (r resource) pathFor(doc string) string {
	if r.path == "races" && doc == "srd-2024" {
		return "species"
	}
	return r.path
}

func resources() []resource {
	return []resource{
		{"spell", "spells", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Spells, doc, func(x snapshot.Spell) (string, string, bool) { return x.Document, x.Slug, true })
		}},
		{"class", "classes", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Classes, doc, func(x snapshot.Class) (string, string, bool) { return x.Document, x.Slug, x.Parent == "" })
		}},
		{"species", "races", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Species, doc, func(x snapshot.Species) (string, string, bool) { return x.Document, x.Slug, !x.Subspecies })
		}},
		{"background", "backgrounds", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Backgrounds, doc, func(x snapshot.Background) (string, string, bool) { return x.Document, x.Slug, true })
		}},
		{"feat", "feats", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Feats, doc, func(x snapshot.Feat) (string, string, bool) { return x.Document, x.Slug, true })
		}},
		{"condition", "conditions", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Conditions, doc, func(x snapshot.Condition) (string, string, bool) { return x.Document, x.Slug, true })
		}},
		{"magic-item", "magic-items", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Items, doc, func(x snapshot.Item) (string, string, bool) { return x.Document, x.Slug, x.Magic })
		}},
		{"monster", "monsters", func(s snapshot.Snapshot, doc string) []string {
			return slugsOf(s.Monsters, doc, func(x snapshot.Monster) (string, string, bool) { return x.Document, x.Slug, true })
		}},
	}
}

func slugsOf[T any](items []T, doc string, key func(T) (string, string, bool)) []string {
	out := []string{}
	for _, x := range items {
		if d, slug, ok := key(x); ok && d == doc {
			out = append(out, slug)
		}
	}
	return out
}

func prefix(doc string) string {
	return "/api/" + strings.TrimPrefix(doc, "srd-")
}

// Run compares every ruleset and kind, then the fields both sources carry for spells and monsters.
func (c Client) Run(ctx context.Context, snap snapshot.Snapshot) (Report, error) {
	var r Report
	for _, d := range snap.Documents {
		for _, res := range resources() {
			if err := c.compare(ctx, snap, d.Key, res, &r); err != nil {
				return Report{}, err
			}
		}
	}
	return r, nil
}

func (c Client) compare(ctx context.Context, snap snapshot.Snapshot, doc string, res resource, r *Report) error {
	label := doc + " " + res.kind
	theirs, err := c.index(ctx, prefix(doc)+"/"+res.pathFor(doc))
	if errors.Is(err, errNotFound) {
		r.Unavailable = append(r.Unavailable, label)
		return nil
	}
	if err != nil {
		return err
	}
	r.Compared = append(r.Compared, label)
	ours := res.ours(snap, doc)
	r.Missing = append(r.Missing, absent(theirs, ours, doc, res.kind)...)
	r.Extra = append(r.Extra, absent(ours, theirs, doc, res.kind)...)
	found, err := c.compareFields(ctx, ourFields(snap, doc, res.kind), doc, res.kind, theirs)
	if err != nil {
		return err
	}
	r.Disagreements = append(r.Disagreements, found...)
	return nil
}

// absent lists the slugs in from that are not in other.
func absent(from, other []string, doc, kind string) []Finding {
	var out []Finding
	for _, slug := range from {
		if !slices.Contains(other, slug) {
			out = append(out, Finding{Ruleset: doc, Kind: kind, Slug: slug})
		}
	}
	return out
}

type listing struct {
	Results []struct {
		Index string `json:"index"`
	} `json:"results"`
}

func (c Client) index(ctx context.Context, path string) ([]string, error) {
	var l listing
	if err := c.get(ctx, path, &l); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(l.Results))
	for _, x := range l.Results {
		out = append(out, x.Index)
	}
	return out, nil
}

func (c Client) get(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return fmt.Errorf("crosscheck: request: %w", err)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("crosscheck: get %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("crosscheck: get %s: status %d", path, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(into); err != nil {
		return fmt.Errorf("crosscheck: decode %s: %w", path, err)
	}
	return nil
}

type detail struct {
	Level           *int     `json:"level"`
	ChallengeRating *float64 `json:"challenge_rating"`
	XP              *int     `json:"xp"`
	HitPoints       *int     `json:"hit_points"`
	ArmorClass      []struct {
		Value int `json:"value"`
	} `json:"armor_class"`
}

func ourFields(snap snapshot.Snapshot, doc, kind string) map[string]map[string]string {
	ours := map[string]map[string]string{}
	switch kind {
	case "spell":
		for _, s := range snap.Spells {
			if s.Document == doc {
				ours[s.Slug] = map[string]string{"level": strconv.Itoa(s.Level)}
			}
		}
	case "monster":
		for _, m := range snap.Monsters {
			if m.Document == doc {
				ours[m.Slug] = map[string]string{
					"challenge rating": strconv.FormatFloat(m.ChallengeRating, 'f', -1, 64), "xp": strconv.Itoa(m.XP),
					"hit points": strconv.Itoa(m.HitPoints), "armor class": strconv.Itoa(m.ArmorClass),
				}
			}
		}
	}
	return ours
}

func (c Client) compareFields(ctx context.Context, ours map[string]map[string]string, doc, kind string, theirs []string) ([]Finding, error) {
	shared := []string{}
	for _, slug := range theirs {
		if _, ok := ours[slug]; ok {
			shared = append(shared, slug)
		}
	}
	if len(shared) == 0 {
		return nil, nil
	}
	details, err := c.details(ctx, prefix(doc)+"/"+kind+"s/", shared)
	if err != nil {
		return nil, err
	}
	var out []Finding
	for i, slug := range shared {
		for field, theirValue := range details[i].fields() {
			if ours[slug][field] != theirValue {
				out = append(out, Finding{Ruleset: doc, Kind: kind, Slug: slug, Field: field, Ours: ours[slug][field], Theirs: theirValue})
			}
		}
	}
	slices.SortFunc(out, func(a, b Finding) int { return strings.Compare(a.Slug+a.Field, b.Slug+b.Field) })
	return out, nil
}

func (d detail) fields() map[string]string {
	out := map[string]string{}
	if d.Level != nil {
		out["level"] = strconv.Itoa(*d.Level)
	}
	if d.ChallengeRating != nil {
		out["challenge rating"] = strconv.FormatFloat(*d.ChallengeRating, 'f', -1, 64)
	}
	if d.XP != nil {
		out["xp"] = strconv.Itoa(*d.XP)
	}
	if d.HitPoints != nil {
		out["hit points"] = strconv.Itoa(*d.HitPoints)
	}
	if len(d.ArmorClass) > 0 {
		out["armor class"] = strconv.Itoa(d.ArmorClass[0].Value)
	}
	return out
}

func (c Client) details(ctx context.Context, base string, slugs []string) ([]detail, error) {
	out := make([]detail, len(slugs))
	errs := make([]error, len(slugs))
	sem := make(chan struct{}, max(c.Workers, 1))
	var wg sync.WaitGroup
	for i, slug := range slugs {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			errs[i] = c.get(ctx, base+slug, &out[i])
		})
	}
	wg.Wait()
	return out, errors.Join(errs...)
}

// Markdown renders the report for a pull request or a docs page.
func (r Report) Markdown() string {
	var b strings.Builder
	b.WriteString("# Compendium cross-check against 5e-bits\n\n")
	fmt.Fprintf(&b, "Compared: %s.\n\n", orNone(r.Compared))
	if len(r.Unavailable) > 0 {
		fmt.Fprintf(&b, "Not offered by 5e-bits: %s.\n\n", strings.Join(r.Unavailable, ", "))
	}
	section(&b, "Only in 5e-bits", r.Missing, false)
	section(&b, "Only in Grimoire", r.Extra, false)
	section(&b, "Disagreements", r.Disagreements, true)
	return b.String()
}

func orNone(list []string) string {
	if len(list) == 0 {
		return "nothing"
	}
	return strings.Join(list, ", ")
}

func section(b *strings.Builder, title string, findings []Finding, values bool) {
	fmt.Fprintf(b, "## %s (%d)\n\n", title, len(findings))
	if len(findings) == 0 {
		b.WriteString("None.\n\n")
		return
	}
	if values {
		b.WriteString("| Ruleset | Kind | Entry | Field | Grimoire | 5e-bits |\n|---|---|---|---|---|---|\n")
		for _, f := range findings {
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s |\n", f.Ruleset, f.Kind, f.Slug, f.Field, f.Ours, f.Theirs)
		}
		b.WriteString("\n")
		return
	}
	b.WriteString("| Ruleset | Kind | Entry |\n|---|---|---|\n")
	for _, f := range findings {
		fmt.Fprintf(b, "| %s | %s | %s |\n", f.Ruleset, f.Kind, f.Slug)
	}
	b.WriteString("\n")
}
