package changelog_test

import (
	"os"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/social/changelog"
)

const sample = `# Changelog

## [1.1.0](https://github.com/x/y/compare/v1.0.0...v1.1.0) (2026-10-05)


### Features

* **social:** Friends with requests ([#110](https://github.com/x/y/issues/110)) ([9ef3caf](https://github.com/x/y/commit/9ef3caf))
* one image for everything ([#36](https://github.com/x/y/issues/36)) ([ceaa8ee](https://github.com/x/y/commit/ceaa8ee))


### Bug Fixes

* **play:** a fix that is not a feature ([#1](https://github.com/x/y/issues/1))

## 1.0.0 (2026-10-01)


### Features

* **apps:** installable PWA ([#70](https://github.com/x/y/issues/70)) ([20078da](https://github.com/x/y/commit/20078da))
`

// A release's features read as plain lines, scoped, without links; fixes and other releases stay out.
func TestFeatures(t *testing.T) {
	t.Parallel()
	got := changelog.Features(sample, "1.1.0")
	if strings.Join(got, "|") != "Social: Friends with requests|One image for everything" {
		t.Fatalf("1.1.0 = %q", got)
	}
	if got := changelog.Features(sample, "1.0.0"); len(got) != 1 || got[0] != "Apps: installable PWA" {
		t.Fatalf("1.0.0 = %q", got)
	}
	if got := changelog.Features(sample, "9.9.9"); got != nil {
		t.Fatalf("an unknown version = %q", got)
	}
}

// The real changelog parses.
func TestTheRepositoryChangelog(t *testing.T) {
	t.Parallel()
	body, err := os.ReadFile("../../../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := changelog.Features(string(body), "1.0.0"); len(got) < 10 || !strings.HasPrefix(got[0], "Apps: ") {
		t.Fatalf("1.0.0 = %q", got)
	}
}
