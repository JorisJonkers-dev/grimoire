// Package httpapi implements the generated OpenAPI server interface.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// StatusSource answers the questions the operations endpoints ask of the database.
type StatusSource interface {
	Ping(ctx context.Context) error
	InstanceCreatedAt(ctx context.Context) (time.Time, error)
}

// Handler implements oas.Handler.
type Handler struct {
	Version    string
	Store      StatusSource
	Compendium CompendiumReader
	Campaigns  Campaigns
	Characters CharacterService
	NPCs       NPCService
	Rolls      RollService
	Log        *slog.Logger
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

// GetMe returns the identity the platform authenticated.
func (h *Handler) GetMe(ctx context.Context) (oas.GetMeRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return problem(http.StatusUnauthorized, "Unauthorized", "Sign in to continue."), nil
	}
	return &oas.MeHeaders{Response: oas.Me{Subject: id.Subject}}, nil
}

func unavailable() *oas.ProblemStatusCodeWithHeaders {
	return problem(http.StatusServiceUnavailable, "Service unavailable", "Try again shortly.")
}

func problem(status int, title, detail string) *oas.ProblemStatusCodeWithHeaders {
	return &oas.ProblemStatusCodeWithHeaders{
		StatusCode: status,
		Response: oas.Problem{
			Type:   "about:blank",
			Title:  title,
			Status: int32(status), //nolint:gosec // HTTP status codes fit in int32
			Detail: oas.NewOptString(detail),
		},
	}
}
