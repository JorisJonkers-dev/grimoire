package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
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
	}
	for name, call := range calls {
		res, err := call()
		p, ok := res.(*oas.ProblemStatusCodeWithHeaders)
		if err != nil || !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s = %#v, %v", name, res, err)
		}
	}
}
