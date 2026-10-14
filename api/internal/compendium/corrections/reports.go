package corrections

import (
	"fmt"
	"strings"
)

// Unfiled lists the drafted upstream reports not filed yet, for the maintainer to file by hand, and
// the corrections with no report drafted.
func Unfiled(cs []Correction) string {
	var drafts, none []Correction
	for _, c := range cs {
		switch {
		case c.Upstream == nil:
			none = append(none, c)
		case c.Upstream.Filed == "":
			drafts = append(drafts, c)
		}
	}
	if len(drafts)+len(none) == 0 {
		return "Every upstream report is filed.\n"
	}
	var b strings.Builder
	if len(drafts) > 0 {
		fmt.Fprintf(&b, "Upstream reports to file (%d). Set `filed` to the issue URL once filed.\n", len(drafts))
	}
	for _, c := range drafts {
		fmt.Fprintf(&b, "\n## %s\n%s %s/%s, %s p. %d\n\nTitle: %s\n\n%s\n",
			c.File, c.Kind, c.Document, c.Entry, c.Source.Document, c.Source.Page, c.Upstream.Title, strings.TrimSpace(c.Upstream.Body))
	}
	if len(none) > 0 {
		fmt.Fprintf(&b, "\nNo report drafted (%d):\n", len(none))
	}
	for _, c := range none {
		fmt.Fprintf(&b, "  %s\n", c.File)
	}
	return b.String()
}
