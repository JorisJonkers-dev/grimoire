// Package httpapi implements the generated OpenAPI server interface.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// StatusSource answers the questions the operations endpoints ask of the database.
type StatusSource interface {
	Ping(ctx context.Context) error
	InstanceCreatedAt(ctx context.Context) (time.Time, error)
}

// Handler implements oas.Handler.
type Handler struct {
	Version string
	Store   StatusSource
	Log     *slog.Logger
}

var _ oas.Handler = (*Handler)(nil)

// GetHealth reports liveness.
func (h *Handler) GetHealth(context.Context) (oas.GetHealthRes, error) {
	return &oas.HealthStatus{Status: "ok"}, nil
}

// GetReadiness reports whether the database answers.
func (h *Handler) GetReadiness(ctx context.Context) (oas.GetReadinessRes, error) {
	if err := h.Store.Ping(ctx); err != nil {
		h.Log.WarnContext(ctx, "readiness: database unreachable", "error", err)
		return unavailable(), nil
	}
	return &oas.HealthStatus{Status: "ok"}, nil
}

// GetStatus reports the running version and the database state.
func (h *Handler) GetStatus(ctx context.Context) (oas.GetStatusRes, error) {
	created, err := h.Store.InstanceCreatedAt(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "status: database query failed", "error", err)
		return unavailable(), nil
	}
	return &oas.StatusHeaders{Response: oas.Status{
		Service:   "grimoire",
		Version:   h.Version,
		Database:  oas.StatusDatabaseUp,
		StartedAt: created.UTC(),
	}}, nil
}

func unavailable() *oas.ProblemStatusCodeWithHeaders {
	return &oas.ProblemStatusCodeWithHeaders{
		StatusCode: http.StatusServiceUnavailable,
		Response: oas.Problem{
			Type:   "about:blank",
			Title:  "Service unavailable",
			Status: http.StatusServiceUnavailable,
			Detail: oas.NewOptString("Try again shortly."),
		},
	}
}
