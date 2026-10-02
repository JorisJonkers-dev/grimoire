package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// NotificationService is the bell and the channels each kind reaches.
type NotificationService interface {
	Notifications(ctx context.Context, subject string) ([]domain.Notification, int, error)
	ReadNotification(ctx context.Context, subject string, id uuid.UUID) error
	ReadAll(ctx context.Context, subject string) error
	Preferences(ctx context.Context, subject string) ([]domain.Preference, error)
	SetPreferences(ctx context.Context, subject string, in []domain.Preference) ([]domain.Preference, error)
}

func preferencesOut(in []domain.Preference) oas.NotificationPreferences {
	out := oas.NotificationPreferences{Items: make([]oas.NotificationPreference, 0, len(in))}
	for _, p := range in {
		out.Items = append(out.Items, oas.NotificationPreference{Kind: oas.NotificationKind(p.Kind), InApp: p.InApp, Push: p.Push, Email: p.Email})
	}
	return out
}

// ListNotifications reads the caller's bell.
func (h *Handler) ListNotifications(ctx context.Context) (oas.ListNotificationsRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, unread, err := h.Notifications.Notifications(ctx, id.Subject)
	if err != nil {
		return h.friendError(ctx, "list notifications", err), nil
	}
	out := oas.NotificationList{Items: make([]oas.NotificationEntry, 0, len(list)), Unread: int32(unread)} //nolint:gosec // a count
	for _, n := range list {
		out.Items = append(out.Items, oas.NotificationEntry{
			ID: oas.ID(n.ID), Kind: oas.NotificationKind(n.Kind), Title: n.Title, Body: n.Body, ActionLabel: n.ActionLabel,
			ActionPath: n.ActionPath, At: n.At.UTC(), Read: n.ReadAt != nil,
		})
	}
	return &oas.NotificationListHeaders{Response: out}, nil
}

// ReadAllNotifications clears the caller's bell.
func (h *Handler) ReadAllNotifications(ctx context.Context) (oas.ReadAllNotificationsRes, error) {
	if bad := h.friendCall(ctx, "read all notifications", func(s string) error { return h.Notifications.ReadAll(ctx, s) }); bad != nil {
		return bad, nil
	}
	return &oas.ReadAllNotificationsNoContent{}, nil
}

// ReadNotification marks one Notification read.
func (h *Handler) ReadNotification(ctx context.Context, p oas.ReadNotificationParams) (oas.ReadNotificationRes, error) {
	if bad := h.friendCall(ctx, "read notification", func(s string) error {
		return h.Notifications.ReadNotification(ctx, s, uuid.UUID(p.NotificationId))
	}); bad != nil {
		return bad, nil
	}
	return &oas.ReadNotificationNoContent{}, nil
}

// GetNotificationPreferences reads the caller's channels per kind.
func (h *Handler) GetNotificationPreferences(ctx context.Context) (oas.GetNotificationPreferencesRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	prefs, err := h.Notifications.Preferences(ctx, id.Subject)
	if err != nil {
		return h.friendError(ctx, "notification preferences", err), nil
	}
	return &oas.NotificationPreferencesHeaders{Response: preferencesOut(prefs)}, nil
}

// SetNotificationPreferences changes the caller's channels per kind.
func (h *Handler) SetNotificationPreferences(ctx context.Context, req *oas.NotificationPreferences) (oas.SetNotificationPreferencesRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	in := make([]domain.Preference, 0, len(req.Items))
	for _, p := range req.Items {
		in = append(in, domain.Preference{Kind: string(p.Kind), InApp: p.InApp, Push: p.Push, Email: p.Email})
	}
	prefs, err := h.Notifications.SetPreferences(ctx, id.Subject, in)
	if err != nil {
		return h.friendError(ctx, "set notification preferences", err), nil
	}
	return &oas.NotificationPreferencesHeaders{Response: preferencesOut(prefs)}, nil
}
