// Package changelog reads the features each release added from the release-please CHANGELOG.md, so a
// Release Note can be drafted from them.
package changelog

import (
	"regexp"
	"strings"
)

var (
	heading = regexp.MustCompile(`^##\s+\[?(\d+\.\d+\.\d+[^\]\s]*)\]?`)
	feature = regexp.MustCompile(`^\*\s+(?:\*\*([^*]+):\*\*\s+)?(.+?)(?:\s+\(\[[^)]*\)\))*\s*$`)
	links   = regexp.MustCompile(`\s*\(\[[^\]]*\]\([^)]*\)\)`)
)

// Features lists the features a version added, each as "Scope: what it does", without issue and
// commit links; none when the changelog does not list that version.
func Features(changelog, version string) []string {
	var out []string
	in, features := false, false
	for _, line := range strings.Split(changelog, "\n") {
		if m := heading.FindStringSubmatch(line); m != nil {
			in, features = m[1] == version, false
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			features = strings.TrimSpace(strings.TrimPrefix(line, "### ")) == "Features"
			continue
		}
		if !features {
			continue
		}
		m := feature.FindStringSubmatch(links.ReplaceAllString(line, ""))
		if m == nil {
			continue
		}
		text := strings.ToUpper(m[2][:1]) + m[2][1:]
		if m[1] != "" {
			text = strings.ToUpper(m[1][:1]) + m[1][1:] + ": " + m[2]
		}
		out = append(out, text)
	}
	return out
}
