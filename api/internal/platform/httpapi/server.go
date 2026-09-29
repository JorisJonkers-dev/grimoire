package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/ogen-go/ogen/ogenerrors"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// Options configures the HTTP surface.
type Options struct {
	Handler    *Handler
	DevSubject string
	RateLimit  int
	Now        func() time.Time
}

// New builds the full HTTP handler: generated router, security, rate limiting and dev identity.
func New(o Options) (http.Handler, error) {
	srv, err := oas.NewServer(o.Handler, auth.ForwardAuth{}, oas.WithErrorHandler(errorHandler(o.Handler.Log)))
	if err != nil {
		return nil, err
	}
	limiter := &httpx.RateLimiter{
		Limit:  o.RateLimit,
		Window: time.Minute,
		Now:    o.Now,
		Exempt: map[string]bool{"/healthz": true, "/readyz": true},
	}
	h := http.Handler(limiter.Wrap(srv))
	if o.DevSubject != "" {
		h = httpx.DevIdentity(o.DevSubject, h)
	}
	return h, nil
}

func errorHandler(log *slog.Logger) ogenerrors.ErrorHandler {
	return func(ctx context.Context, w http.ResponseWriter, _ *http.Request, err error) {
		var secErr *ogenerrors.SecurityError
		switch {
		case errors.As(err, &secErr):
			httpx.WriteProblem(w, http.StatusUnauthorized, "Unauthorized", "Sign in to continue.")
		case errors.Is(err, ogenerrors.ErrSecurityRequirementIsNotSatisfied):
			httpx.WriteProblem(w, http.StatusUnauthorized, "Unauthorized", "Sign in to continue.")
		default:
			code := ogenerrors.ErrorCode(err)
			if code >= http.StatusInternalServerError {
				log.ErrorContext(ctx, "request failed", "error", err)
				httpx.WriteProblem(w, code, "Something went wrong", "Try again shortly.")
				return
			}
			httpx.WriteProblem(w, code, http.StatusText(code), "The request could not be processed.")
		}
	}
}
