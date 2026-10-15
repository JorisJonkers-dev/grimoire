package designs

import (
	"regexp"
	"slices"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
)

// entries names every SRD 2024 entry a Design file can automate, by the path its file takes.
func entries(snap snapshot.Snapshot) map[string]string {
	out := map[string]string{}
	add := func(doc, p, name string) {
		if doc == Root {
			out[p] = name
		}
	}
	for _, s := range snap.Spells {
		add(s.Document, "spells/"+s.Slug, s.Name)
	}
	for _, c := range snap.Conditions {
		add(c.Document, "conditions/"+c.Slug, c.Name)
	}
	for _, c := range snap.Classes {
		for _, f := range c.Features {
			add(c.Document, "features/"+c.Slug+"/"+f.Slug, f.Name)
		}
	}
	for _, s := range snap.Species {
		for _, t := range s.Traits {
			add(s.Document, "species/"+s.Slug+"/"+slug(t.Name), t.Name)
		}
	}
	for _, f := range snap.Feats {
		add(f.Document, "feats/"+f.Slug, f.Name)
	}
	for _, i := range snap.Items {
		add(i.Document, "items/"+i.Slug, i.Name)
	}
	for _, m := range snap.Monsters {
		add(m.Document, "monsters/"+m.Slug, m.Name)
	}
	return out
}

func slug(s string) string {
	return strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(s), "-"), "-")
}

func sortedPaths[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for p := range m {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}
