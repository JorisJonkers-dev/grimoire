package designs

import (
	"slices"
	"strings"
)

// exception is an entry on the approved Manual exception list: the parts of it left to the DM, on which
// ground, and why.
type exception struct {
	Entry  string   `yaml:"entry"`
	Ground string   `yaml:"ground"`
	Parts  []string `yaml:"parts"`
	Reason string   `yaml:"reason"`
}

// approved reads the exception list only the maintainer changes, by entry.
func (r *reader) approved(body []byte) map[string]exception {
	const file = "_exceptions.yaml"
	out := map[string]exception{}
	var list []exception
	if err := strict(body, &list); err != nil && len(body) > 0 {
		r.bad(file, "%v", err)
		return out
	}
	for _, e := range list {
		switch {
		case r.seed[e.Entry] == "":
			r.bad(file, "%s: no such entry in the SRD 2024 seed", e.Entry)
		case !slices.Contains([]string{"dm_knowledge", "on_the_spot", "free_form"}, e.Ground):
			r.bad(file, "%s: ground is dm_knowledge, on_the_spot or free_form, not %s", e.Entry, e.Ground)
		case len(e.Parts) == 0 || e.Reason == "":
			r.bad(file, "%s: state the parts left to the DM and the reason", e.Entry)
		case out[e.Entry].Entry != "":
			r.bad(file, "%s: list each entry once", e.Entry)
		default:
			out[e.Entry] = e
		}
	}
	return out
}

// listed reads a folder's entries that need no Effect.
func (r *reader) listed(file, folder string, body []byte) {
	if folder != "features" && folder != "items" {
		r.bad(file, "only features and items list entries with no Effect")
		return
	}
	var list []NoEffect
	if err := strict(body, &list); err != nil {
		r.bad(file, "%v", err)
		return
	}
	seen := map[string]bool{}
	for _, n := range list {
		switch {
		case !strings.HasPrefix(n.Entry, folder+"/") || r.seed[n.Entry] == "":
			r.bad(file, "%s: no such entry in the SRD 2024 seed", n.Entry)
		case !slices.Contains([]string{"table_column", "choice", "sheet_value", "plain_gear"}, n.Reason):
			r.bad(file, "%s: reason is table_column, choice, sheet_value or plain_gear, not %s", n.Entry, n.Reason)
		case seen[n.Entry]:
			r.bad(file, "%s: list each entry once", n.Entry)
		default:
			seen[n.Entry] = true
			r.noEffect = append(r.noEffect, n)
		}
	}
}
