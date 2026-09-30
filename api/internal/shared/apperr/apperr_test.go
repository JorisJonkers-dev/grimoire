package apperr_test

import (
	"errors"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

func TestRuleErrorIsInvalid(t *testing.T) {
	t.Parallel()
	err := apperr.Refuse("no")
	var rule *apperr.RuleError
	if !errors.Is(err, apperr.ErrInvalid) || !errors.As(err, &rule) || err.Error() != "no" {
		t.Fatal(err)
	}
}
