package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ogen-go/ogen/ogenerrors"
)

func TestErrorHandlerMapsErrorsToProblems(t *testing.T) {
	t.Parallel()
	h := errorHandler(slog.New(slog.NewTextHandler(io.Discard, nil)))
	cases := []struct {
		err  error
		code int
	}{
		{&ogenerrors.SecurityError{OperationContext: ogenerrors.OperationContext{}, Security: "forwardAuth", Err: errors.New("x")}, 401},
		{ogenerrors.ErrSecurityRequirementIsNotSatisfied, 401},
		{&ogenerrors.DecodeRequestError{Err: errors.New("bad")}, 400},
		{errors.New("unexpected"), 500},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h(context.Background(), rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), c.err)
		var p map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		if rec.Code != c.code || p["status"] != float64(c.code) {
			t.Fatalf("%v: got %d %v", c.err, rec.Code, p)
		}
		if detail, _ := p["detail"].(string); detail == "" || detail == c.err.Error() {
			t.Fatalf("detail must be generic, got %q", detail)
		}
	}
}
