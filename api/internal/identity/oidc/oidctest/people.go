package oidctest

import (
	"regexp"
	"slices"
	"strings"
)

// People are the jorisjonkers.dev identities the fake provider's sign-in page knows, one of each kind
// an e2e journey needs. A new kind is one more Login here; its Username is what a test types.
//
// Signing in as "<username>+<tag>" gives a fresh identity of the same kind: the subject and email carry
// the tag and the preferred username becomes "<username>-<tag>". Tests that run in parallel, or again
// against the same database, tag each sign-in so none of them meets an Account another one made.
func People() []Login {
	return []Login{
		// An estate user holding the Grimoire grant.
		{Subject: "fake-player", Email: "player@example.org", Username: "player", Name: "Rowan Player", Roles: []string{GrantRole}},
		// An estate user without the Grimoire grant, who holds some other service's.
		{Subject: "fake-outsider", Email: "outsider@example.org", Username: "outsider", Name: "Ash Outsider", Roles: []string{"SERVICE_OTHER"}},
		// An estate admin, who holds the Grimoire grant too.
		{Subject: "fake-admin", Email: "admin@example.org", Username: "admin", Name: "Sage Admin", Roles: []string{GrantRole, AdminRole}},
	}
}

var tagPattern = regexp.MustCompile(`^[a-z0-9-]{1,24}$`)

// person finds whom a typed username names among people, a tag making a fresh identity of that kind.
func person(people []Login, typed string) (Login, bool) {
	base, tag, tagged := strings.Cut(strings.ToLower(strings.TrimSpace(typed)), "+")
	if tagged && !tagPattern.MatchString(tag) {
		return Login{}, false
	}
	i := slices.IndexFunc(people, func(l Login) bool { return l.Username == base })
	if i < 0 {
		return Login{}, false
	}
	l := people[i]
	l.Roles = slices.Clone(l.Roles)
	if tagged {
		local, domain, _ := strings.Cut(l.Email, "@")
		l.Subject, l.Email, l.Username = l.Subject+"+"+tag, local+"+"+tag+"@"+domain, l.Username+"-"+tag
	}
	return l, true
}
