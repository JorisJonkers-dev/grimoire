package httpapi

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// PushService keeps devices' push subscriptions.
type PushService interface {
	Key() string
	Subscribe(ctx context.Context, subject, endpoint, p256dh, auth string) (uuid.UUID, error)
	Unsubscribe(ctx context.Context, subject string, id uuid.UUID) error
}

func pushOff() *oas.ProblemStatusCodeWithHeaders {
	return problem(http.StatusNotFound, "Not found", "This server sends no notifications.")
}

// GetPushKey gives the key a device subscribes to notifications with.
func (h *Handler) GetPushKey(ctx context.Context) (oas.GetPushKeyRes, error) {
	if _, ok := uiCaller(ctx); !ok {
		return unauthorized(), nil
	}
	if h.Push == nil {
		return pushOff(), nil
	}
	return &oas.PushKeyHeaders{Response: oas.PushKey{PublicKey: h.Push.Key()}}, nil
}

// CreatePushSubscription subscribes one of the caller's devices.
func (h *Handler) CreatePushSubscription(ctx context.Context, req *oas.PushSubscriptionInput) (oas.CreatePushSubscriptionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if h.Push == nil {
		return pushOff(), nil
	}
	id, err := h.Push.Subscribe(ctx, c.Subject, req.Endpoint.String(), req.Keys.P256dh, req.Keys.Auth)
	if err != nil {
		return h.campaignProblem(ctx, "subscribe", err), nil
	}
	return &oas.PushSubscriptionHeaders{Response: oas.PushSubscription{ID: oas.ID(id)}}, nil
}

// DeletePushSubscription unsubscribes one of the caller's devices.
func (h *Handler) DeletePushSubscription(ctx context.Context, p oas.DeletePushSubscriptionParams) (oas.DeletePushSubscriptionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if h.Push == nil {
		return pushOff(), nil
	}
	if err := h.Push.Unsubscribe(ctx, c.Subject, uuid.UUID(p.SubscriptionId)); err != nil {
		return h.campaignProblem(ctx, "unsubscribe", err), nil
	}
	return &oas.DeletePushSubscriptionNoContent{}, nil
}
