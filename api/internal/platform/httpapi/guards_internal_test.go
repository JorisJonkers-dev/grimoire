package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// Every Account handler refuses a call that reaches it without an identity, whatever the router let
// through.
func TestAccountHandlersNeedAnIdentity(t *testing.T) {
	t.Parallel()
	ctx, h := context.Background(), &Handler{}
	calls := map[string]func() (any, error){
		"invite":        func() (any, error) { return h.CreateAccountInvite(ctx, &oas.AccountInviteRequest{}) },
		"account":       func() (any, error) { return h.GetAccount(ctx) },
		"password":      func() (any, error) { return h.SetAccountPassword(ctx, &oas.PasswordChange{}) },
		"profile":       func() (any, error) { return h.UpdateAccount(ctx, &oas.AccountChange{}) },
		"link":          func() (any, error) { return h.StartOidcLink(ctx) },
		"unlink":        func() (any, error) { return h.UnlinkOidc(ctx) },
		"begin":         func() (any, error) { return h.BeginTwoStep(ctx) },
		"confirm":       func() (any, error) { return h.ConfirmTwoStep(ctx, &oas.TwoStepCode{}, oas.ConfirmTwoStepParams{}) },
		"disable":       func() (any, error) { return h.DisableTwoStep(ctx, &oas.TwoStepCode{}) },
		"codes":         func() (any, error) { return h.ResetRecoveryCodes(ctx, &oas.TwoStepCode{}) },
		"tokens":        func() (any, error) { return h.ListAccessTokens(ctx) },
		"mint":          func() (any, error) { return h.CreateAccessToken(ctx, &oas.AccessTokenRequest{}) },
		"revoke":        func() (any, error) { return h.RevokeAccessToken(ctx, oas.RevokeAccessTokenParams{}) },
		"admin list":    func() (any, error) { return h.ListAdminAccounts(ctx) },
		"admin account": func() (any, error) { return h.GetAdminAccount(ctx, oas.GetAdminAccountParams{}) },
		"admin link":    func() (any, error) { return h.SendAdminSignInLink(ctx, oas.SendAdminSignInLinkParams{}) },
		"history":       func() (any, error) { return h.GetAccountHistory(ctx) },
		"friends":       func() (any, error) { return h.ListFriends(ctx) },
		"befriend":      func() (any, error) { return h.SendFriendRequest(ctx, &oas.FriendRequestCreate{}) },
		"accept friend": func() (any, error) { return h.AcceptFriendRequest(ctx, oas.AcceptFriendRequestParams{}) },
		"conversations": func() (any, error) { return h.ListConversations(ctx) },
		"start":         func() (any, error) { return h.StartConversation(ctx, &oas.ConversationStart{}) },
		"messages":      func() (any, error) { return h.ListMessages(ctx, oas.ListMessagesParams{}) },
		"send":          func() (any, error) { return h.SendMessage(ctx, &oas.MessageSend{}, oas.SendMessageParams{}) },
		"mentionables":  func() (any, error) { return h.ListMentionables(ctx, oas.ListMentionablesParams{}) },
	}
	for name, call := range calls {
		res, err := call()
		p, ok := res.(*oas.ProblemStatusCodeWithHeaders)
		if err != nil || !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s = %#v, %v", name, res, err)
		}
	}
}

type brokenFriends struct{}

var errFriends = errors.New("database gone")

func (brokenFriends) Request(context.Context, string, string) error            { return errFriends }
func (brokenFriends) Accept(context.Context, string, uuid.UUID) error          { return errFriends }
func (brokenFriends) Decline(context.Context, string, uuid.UUID, bool) error   { return errFriends }
func (brokenFriends) Cancel(context.Context, string, uuid.UUID) error          { return errFriends }
func (brokenFriends) Unfriend(context.Context, string, domain.AccountID) error { return errFriends }
func (brokenFriends) Unblock(context.Context, string, domain.AccountID) error  { return errFriends }
func (brokenFriends) Friends(context.Context, string) (domain.Friends, error) {
	return domain.Friends{}, errFriends
}

func (brokenFriends) StartConversation(context.Context, string, string, []domain.AccountID) (uuid.UUID, error) {
	return uuid.Nil, errFriends
}

func (brokenFriends) Conversations(context.Context, string) ([]domain.Conversation, error) {
	return nil, errFriends
}

func (brokenFriends) Send(context.Context, string, uuid.UUID, string, []domain.Mention) (domain.Message, error) {
	return domain.Message{}, errFriends
}

func (brokenFriends) Messages(context.Context, string, uuid.UUID, *time.Time) ([]domain.Message, error) {
	return nil, errFriends
}

func (brokenFriends) Mentionable(context.Context, string, string) ([]domain.Mentionable, error) {
	return nil, errFriends
}

// When the Friends store fails, every call answers 503 without saying why.
func TestFriendsWhenTheStoreFails(t *testing.T) {
	t.Parallel()
	ctx := auth.WithIdentity(context.Background(), auth.Identity{Subject: "aria"})
	h := &Handler{Friends: brokenFriends{}, Conversations: brokenFriends{}, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	calls := map[string]func() (any, error){
		"list":    func() (any, error) { return h.ListFriends(ctx) },
		"request": func() (any, error) { return h.SendFriendRequest(ctx, &oas.FriendRequestCreate{}) },
		"accept":  func() (any, error) { return h.AcceptFriendRequest(ctx, oas.AcceptFriendRequestParams{}) },
		"decline": func() (any, error) {
			return h.DeclineFriendRequest(ctx, &oas.FriendRequestDecline{}, oas.DeclineFriendRequestParams{})
		},
		"cancel":             func() (any, error) { return h.CancelFriendRequest(ctx, oas.CancelFriendRequestParams{}) },
		"unfriend":           func() (any, error) { return h.Unfriend(ctx, oas.UnfriendParams{}) },
		"unblock":            func() (any, error) { return h.Unblock(ctx, oas.UnblockParams{}) },
		"list conversations": func() (any, error) { return h.ListConversations(ctx) },
		"start":              func() (any, error) { return h.StartConversation(ctx, &oas.ConversationStart{}) },
		"messages": func() (any, error) {
			return h.ListMessages(ctx, oas.ListMessagesParams{Before: oas.NewOptDateTime(time.Now())})
		},
		"send":         func() (any, error) { return h.SendMessage(ctx, &oas.MessageSend{}, oas.SendMessageParams{}) },
		"mentionables": func() (any, error) { return h.ListMentionables(ctx, oas.ListMentionablesParams{}) },
	}
	for name, call := range calls {
		res, err := call()
		p, ok := res.(*oas.ProblemStatusCodeWithHeaders)
		if err != nil || !ok || p.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("%s = %#v, %v", name, res, err)
		}
	}
}
