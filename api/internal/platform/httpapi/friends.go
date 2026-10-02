package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// FriendService is Friends between Accounts.
type FriendService interface {
	Request(ctx context.Context, subject, username string) error
	Accept(ctx context.Context, subject string, id uuid.UUID) error
	Decline(ctx context.Context, subject string, id uuid.UUID, block bool) error
	Cancel(ctx context.Context, subject string, id uuid.UUID) error
	Unfriend(ctx context.Context, subject string, friend domain.AccountID) error
	Unblock(ctx context.Context, subject string, blocked domain.AccountID) error
	Friends(ctx context.Context, subject string) (domain.Friends, error)
}

var (
	_ FriendService       = (*app.Service)(nil)
	_ ConversationService = (*app.Service)(nil)
	_ NotificationService = (*app.Service)(nil)
)

func (h *Handler) friendError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrNoAccount):
		return problem(http.StatusForbidden, "No Account", "Friends need a Grimoire Account; sign in with one first.")
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "There is no such Account, Friend, Friend request or Conversation.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Already Friends", "You are already Friends.")
	case errors.Is(err, domain.ErrInvalid):
		return problem(http.StatusUnprocessableEntity, "Invalid", "That is not allowed: yourself as a Friend, an empty or too long message, a Mention you cannot open, or turning security Notifications off.")
	}
	h.Log.ErrorContext(ctx, op, "error", err)
	return unavailable()
}

func personOut(p domain.Person) oas.Person {
	return oas.Person{ID: oas.ID(p.ID), Username: oas.Username(p.Username), Nickname: p.Nickname}
}

func requestsOut(in []domain.Request) []oas.FriendRequestEntry {
	out := make([]oas.FriendRequestEntry, 0, len(in))
	for _, r := range in {
		out = append(out, oas.FriendRequestEntry{ID: oas.ID(r.ID), Person: personOut(r.Person), At: r.At.UTC()})
	}
	return out
}

// friendCall runs a Friends change as the caller.
func (h *Handler) friendCall(ctx context.Context, op string, run func(subject string) error) *oas.ProblemStatusCodeWithHeaders {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized()
	}
	if err := run(id.Subject); err != nil {
		return h.friendError(ctx, op, err)
	}
	return nil
}

// ListFriends reads the caller's Friends page.
func (h *Handler) ListFriends(ctx context.Context) (oas.ListFriendsRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	f, err := h.Friends.Friends(ctx, id.Subject)
	if err != nil {
		return h.friendError(ctx, "list friends", err), nil
	}
	out := oas.FriendsPage{Friends: make([]oas.FriendEntry, 0, len(f.Friends)), Incoming: requestsOut(f.Incoming), Outgoing: requestsOut(f.Outgoing), Blocked: requestsOut(f.Blocked)}
	for _, fr := range f.Friends {
		out.Friends = append(out.Friends, oas.FriendEntry{Person: personOut(fr.Person), Since: fr.Since.UTC()})
	}
	return &oas.FriendsPageHeaders{Response: out}, nil
}

// SendFriendRequest asks an Account to be Friends.
func (h *Handler) SendFriendRequest(ctx context.Context, req *oas.FriendRequestCreate) (oas.SendFriendRequestRes, error) {
	if bad := h.friendCall(ctx, "send friend request", func(s string) error { return h.Friends.Request(ctx, s, req.Username) }); bad != nil {
		return bad, nil
	}
	return &oas.SendFriendRequestAccepted{}, nil
}

// AcceptFriendRequest accepts a Friend request.
func (h *Handler) AcceptFriendRequest(ctx context.Context, p oas.AcceptFriendRequestParams) (oas.AcceptFriendRequestRes, error) {
	if bad := h.friendCall(ctx, "accept friend request", func(s string) error { return h.Friends.Accept(ctx, s, uuid.UUID(p.RequestId)) }); bad != nil {
		return bad, nil
	}
	return &oas.AcceptFriendRequestNoContent{}, nil
}

// DeclineFriendRequest declines a Friend request, and may block its sender.
func (h *Handler) DeclineFriendRequest(ctx context.Context, req *oas.FriendRequestDecline, p oas.DeclineFriendRequestParams) (oas.DeclineFriendRequestRes, error) {
	if bad := h.friendCall(ctx, "decline friend request", func(s string) error {
		return h.Friends.Decline(ctx, s, uuid.UUID(p.RequestId), req.Block.Or(false))
	}); bad != nil {
		return bad, nil
	}
	return &oas.DeclineFriendRequestNoContent{}, nil
}

// CancelFriendRequest withdraws a Friend request.
func (h *Handler) CancelFriendRequest(ctx context.Context, p oas.CancelFriendRequestParams) (oas.CancelFriendRequestRes, error) {
	if bad := h.friendCall(ctx, "cancel friend request", func(s string) error { return h.Friends.Cancel(ctx, s, uuid.UUID(p.RequestId)) }); bad != nil {
		return bad, nil
	}
	return &oas.CancelFriendRequestNoContent{}, nil
}

// Unfriend ends a friendship.
func (h *Handler) Unfriend(ctx context.Context, p oas.UnfriendParams) (oas.UnfriendRes, error) {
	if bad := h.friendCall(ctx, "unfriend", func(s string) error { return h.Friends.Unfriend(ctx, s, uuid.UUID(p.AccountId)) }); bad != nil {
		return bad, nil
	}
	return &oas.UnfriendNoContent{}, nil
}

// Unblock lifts a block.
func (h *Handler) Unblock(ctx context.Context, p oas.UnblockParams) (oas.UnblockRes, error) {
	if bad := h.friendCall(ctx, "unblock", func(s string) error { return h.Friends.Unblock(ctx, s, uuid.UUID(p.AccountId)) }); bad != nil {
		return bad, nil
	}
	return &oas.UnblockNoContent{}, nil
}
