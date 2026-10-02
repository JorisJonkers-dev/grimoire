package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

func challenged(out domain.SignedIn) *oas.TwoStepChallengeHeaders {
	return &oas.TwoStepChallengeHeaders{Response: oas.TwoStepChallenge{Challenge: out.Challenge}}
}

func (h *Handler) twoStepError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		return problem(http.StatusUnauthorized, "Wrong code", "That code is wrong or was already used.")
	case errors.Is(err, domain.ErrExpired):
		return problem(http.StatusGone, "Gone", "This sign-in expired or had too many wrong codes; sign in again.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Already on", "Two-step is already on; turn it off first to use another app.")
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not started", "Start two-step first.")
	}
	return h.identityError(ctx, op, err)
}

// PassTwoStep answers a sign-in's second step.
func (h *Handler) PassTwoStep(ctx context.Context, req *oas.TwoStepAnswer) (oas.PassTwoStepRes, error) {
	a, token, err := h.Accounts.PassTwoStep(ctx, req.Challenge, req.Code, httpx.UserAgent(ctx))
	if err != nil {
		return h.twoStepError(ctx, "pass two-step", err), nil
	}
	return h.signedInProfile(ctx, a, token), nil
}

// BeginTwoStep makes a new authenticator secret for the signed-in Account.
func (h *Handler) BeginTwoStep(ctx context.Context) (oas.BeginTwoStepRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	setup, err := h.Accounts.BeginTwoStep(ctx, id.Subject)
	if err != nil {
		return h.twoStepError(ctx, "begin two-step", err), nil
	}
	return &oas.TwoStepSetupHeaders{Response: oas.TwoStepSetup{Secret: setup.Secret, URI: setup.URI}}, nil
}

// ConfirmTwoStep turns two-step on and returns the recovery codes.
func (h *Handler) ConfirmTwoStep(ctx context.Context, req *oas.TwoStepCode, params oas.ConfirmTwoStepParams) (oas.ConfirmTwoStepRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	session, _ := params.GrimoireSession.Get()
	codes, err := h.Accounts.ConfirmTwoStep(ctx, id.Subject, req.Code, session)
	if err != nil {
		return h.twoStepError(ctx, "confirm two-step", err), nil
	}
	return &oas.RecoveryCodesHeaders{Response: oas.RecoveryCodes{Codes: codes}}, nil
}

// DisableTwoStep turns two-step off.
func (h *Handler) DisableTwoStep(ctx context.Context, req *oas.TwoStepCode) (oas.DisableTwoStepRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Accounts.DisableTwoStep(ctx, id.Subject, req.Code); err != nil {
		return h.twoStepError(ctx, "disable two-step", err), nil
	}
	return &oas.DisableTwoStepNoContent{}, nil
}

// ResetRecoveryCodes replaces the recovery codes.
func (h *Handler) ResetRecoveryCodes(ctx context.Context, req *oas.TwoStepCode) (oas.ResetRecoveryCodesRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	codes, err := h.Accounts.ResetRecoveryCodes(ctx, id.Subject, req.Code)
	if err != nil {
		return h.twoStepError(ctx, "reset recovery codes", err), nil
	}
	return &oas.RecoveryCodesHeaders{Response: oas.RecoveryCodes{Codes: codes}}, nil
}
