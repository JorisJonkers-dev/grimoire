package httpapi_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// failingLibrary fails as a broken database would.
type failingLibrary struct{ httpapi.LibraryService }

func (failingLibrary) Entries(context.Context, caller.Caller, string) ([]domain.Entry, error) {
	return nil, errors.New("library down")
}

func TestABrokenLibraryAnswersUnavailable(t *testing.T) {
	t.Parallel()
	h := campaignServer(t, nil, httpapi.LibraryService(failingLibrary{}))
	if rec := call(h, http.MethodGet, "/api/v1/library", "dm", ""); rec.Code != http.StatusServiceUnavailable || strings.Contains(rec.Body.String(), "down") {
		t.Fatalf("a broken Library: %d %s", rec.Code, rec.Body.String())
	}
	if rec := call(h, http.MethodGet, "/api/v1/library", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("signed out: %d", rec.Code)
	}
}
