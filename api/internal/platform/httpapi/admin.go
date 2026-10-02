package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

func (h *Handler) adminError(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "There is no such Account.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Not allowed", "You cannot do that to your own Account, to a disabled one, or email one without an email.")
	}
	return h.identityError(ctx, op, err)
}

func status(disabled bool) string {
	if disabled {
		return "disabled"
	}
	return "active"
}

func eventsOut(events []domain.Event) []oas.AccountEvent {
	out := make([]oas.AccountEvent, 0, len(events))
	for _, e := range events {
		actor := e.ActorName
		if actor == "" {
			actor = e.Actor
		}
		out = append(out, oas.AccountEvent{At: e.At.UTC(), Actor: actor, Action: oas.AccountEventAction(e.Action), Detail: e.Detail})
	}
	return out
}

// ListAdminAccounts lists every Account and unused Invite for an Admin.
func (h *Handler) ListAdminAccounts(ctx context.Context) (oas.ListAdminAccountsRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	accounts, invites, err := h.Accounts.AdminAccounts(ctx, id.Subject)
	if err != nil {
		return h.adminError(ctx, "list accounts", err), nil
	}
	out := oas.AdminAccountList{Accounts: make([]oas.AdminAccountRow, 0, len(accounts)), Invites: make([]oas.AdminInviteRow, 0, len(invites))}
	for _, l := range accounts {
		a := l.Account
		row := oas.AdminAccountRow{
			ID: oas.ID(a.ID), Username: oas.Username(a.Username), Nickname: a.Nickname, Email: a.Email, Admin: a.Admin,
			Status: oas.AdminAccountRowStatus(status(a.Disabled)), CreatedAt: a.CreatedAt.UTC(), LastSeenAt: oas.OptDateTime{},
		}
		if l.LastSeenAt != nil {
			row.LastSeenAt = oas.NewOptDateTime(l.LastSeenAt.UTC())
		}
		out.Accounts = append(out.Accounts, row)
	}
	for _, l := range invites {
		inv, st := l.Invite, oas.AdminInviteRowStatusInvited
		if !l.Open {
			st = oas.AdminInviteRowStatusExpired
		}
		out.Invites = append(out.Invites, oas.AdminInviteRow{ID: oas.ID(inv.ID), Admin: inv.Admin, Status: st, CreatedAt: inv.CreatedAt.UTC(), ExpiresAt: inv.ExpiresAt.UTC()})
	}
	return &oas.AdminAccountListHeaders{Response: out}, nil
}

// GetAdminAccount reads one Account's page for an Admin.
func (h *Handler) GetAdminAccount(ctx context.Context, params oas.GetAdminAccountParams) (oas.GetAdminAccountRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.Accounts.AdminAccount(ctx, id.Subject, uuid.UUID(params.AccountId))
	if err != nil {
		return h.adminError(ctx, "get account", err), nil
	}
	a := d.Profile.Account
	out := oas.AdminAccountDetail{
		Account: profileOut(d.Profile), Status: oas.AdminAccountDetailStatus(status(a.Disabled)), CreatedAt: a.CreatedAt.UTC(),
		Sessions: int32(d.Sessions), Tokens: int32(d.Tokens), Campaigns: []oas.AdminCampaign{}, History: eventsOut(d.Events), //nolint:gosec // counts are small
	}
	if h.Campaigns != nil {
		list, err := h.Campaigns.List(ctx, caller.UI(a.Subject), nil, 200)
		if err != nil {
			return h.adminError(ctx, "account campaigns", err), nil
		}
		for _, c := range list {
			out.Campaigns = append(out.Campaigns, oas.AdminCampaign{ID: oas.ID(c.ID), Name: c.Name, Role: string(c.MyRole)})
		}
	}
	return &oas.AdminAccountDetailHeaders{Response: out}, nil
}

func (h *Handler) adminControl(ctx context.Context, op string, run func(subject string) error) *oas.ProblemStatusCodeWithHeaders {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized()
	}
	if err := run(id.Subject); err != nil {
		return h.adminError(ctx, op, err)
	}
	return nil
}

// SendAdminSignInLink emails an Account a sign-in link, for an Admin.
func (h *Handler) SendAdminSignInLink(ctx context.Context, params oas.SendAdminSignInLinkParams) (oas.SendAdminSignInLinkRes, error) {
	if bad := h.adminControl(ctx, "send sign-in link", func(by string) error {
		return h.Accounts.SendSignInLink(ctx, by, uuid.UUID(params.AccountId))
	}); bad != nil {
		return bad, nil
	}
	return &oas.SendAdminSignInLinkAccepted{}, nil
}

// SetAdminRole makes an Account an Admin or not.
func (h *Handler) SetAdminRole(ctx context.Context, req *oas.Toggle, params oas.SetAdminRoleParams) (oas.SetAdminRoleRes, error) {
	if bad := h.adminControl(ctx, "set admin", func(by string) error {
		return h.Accounts.SetAdmin(ctx, by, uuid.UUID(params.AccountId), req.Value)
	}); bad != nil {
		return bad, nil
	}
	return &oas.SetAdminRoleNoContent{}, nil
}

// SetAccountDisabled disables an Account or enables it.
func (h *Handler) SetAccountDisabled(ctx context.Context, req *oas.Toggle, params oas.SetAccountDisabledParams) (oas.SetAccountDisabledRes, error) {
	if bad := h.adminControl(ctx, "set disabled", func(by string) error {
		return h.Accounts.SetDisabled(ctx, by, uuid.UUID(params.AccountId), req.Value)
	}); bad != nil {
		return bad, nil
	}
	return &oas.SetAccountDisabledNoContent{}, nil
}

// ResetAccountTwoStep turns an Account's two-step off, for an Admin.
func (h *Handler) ResetAccountTwoStep(ctx context.Context, params oas.ResetAccountTwoStepParams) (oas.ResetAccountTwoStepRes, error) {
	if bad := h.adminControl(ctx, "reset two-step", func(by string) error {
		return h.Accounts.ResetTwoStep(ctx, by, uuid.UUID(params.AccountId))
	}); bad != nil {
		return bad, nil
	}
	return &oas.ResetAccountTwoStepNoContent{}, nil
}

// GetAccountHistory reads the signed-in Account's history.
func (h *Handler) GetAccountHistory(ctx context.Context) (oas.GetAccountHistoryRes, error) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return unauthorized(), nil
	}
	events, err := h.Accounts.History(ctx, id.Subject)
	if err != nil {
		return h.adminError(ctx, "account history", err), nil
	}
	return &oas.AccountEventListHeaders{Response: oas.AccountEventList{Items: eventsOut(events)}}, nil
}
