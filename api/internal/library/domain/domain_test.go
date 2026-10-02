package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

func TestDraftsCollectionsAndMessagesAreChecked(t *testing.T) {
	t.Parallel()
	many := domain.Fields{}
	for i := range domain.MaxFields + 1 {
		many[strings.Repeat("x", i+1)] = ""
	}
	for name, err := range map[string]error{
		"unknown kind":     second(domain.Draft{Kind: "vehicle", Name: "Cart"}.Clean()),
		"long name":        second(domain.Draft{Kind: "npc", Name: strings.Repeat("a", 81)}.Clean()),
		"too many fields":  second(domain.CleanFields(many)),
		"long value":       second(domain.CleanFields(domain.Fields{"Lore": strings.Repeat("a", domain.MaxFieldValue+1)})),
		"long message":     second(domain.CleanMessage(strings.Repeat("a", 2001))),
		"nameless group":   third(domain.CleanCollection(" ", "")),
		"long description": third(domain.CleanCollection("Fey", strings.Repeat("a", 2001))),
	} {
		if !errors.Is(err, apperr.ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	d, err := domain.Draft{Kind: "npc", Name: "  Odo  ", Fields: domain.Fields{" Mood ": "cheery"}}.Clean()
	if err != nil || d.Name != "Odo" || d.Fields["Mood"] != "cheery" {
		t.Fatalf("a clean draft = %+v %v", d, err)
	}
	l := domain.Linked{Base: domain.Fields{"HP": "52", "AC": "17"}, Override: domain.Fields{"HP": "30"}}
	if got := l.Resolved(); got["HP"] != "30" || got["AC"] != "17" || l.Base["HP"] != "52" {
		t.Fatalf("the override wins without touching the base = %v %v", got, l.Base)
	}
}

func second[T any](_ T, err error) error { return err }

func third[A, B any](_ A, _ B, err error) error { return err }
