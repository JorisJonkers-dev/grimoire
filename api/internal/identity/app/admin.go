package app

import (
	"context"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/domain"
)

// AdminRepository reads every Account and Invite, records history, and holds the Admin controls.
type AdminRepository interface {
	ListAccounts(ctx context.Context) ([]domain.Listed, error)
	UnusedInvites(ctx context.Context) ([]domain.Invite, error)
	InsertEvent(ctx context.Context, account domain.AccountID, e domain.Event) error
	Events(ctx context.Context, account domain.AccountID) ([]domain.Event, error)
	SetDisabled(ctx context.Context, account domain.AccountID, disabled bool) error
	RevokeEverything(ctx context.Context, account domain.AccountID, now time.Time) error
	LiveCounts(ctx context.Context, account domain.AccountID, now time.Time) (int, int, error)
}

// record writes one line of an Account's history.
func (s *Service) record(ctx context.Context, r Repository, account domain.AccountID, actor, action, detail string) error {
	return r.InsertEvent(ctx, account, domain.Event{At: s.Now(), Actor: actor, ActorName: "", Action: action, Detail: detail})
}

// AdminAccounts lists every Account and every Invite not yet used, for an Admin.
func (s *Service) AdminAccounts(ctx context.Context, actor string) ([]domain.Listed, []domain.ListedInvite, error) {
	if !s.IsAdmin(ctx, actor) {
		return nil, nil, domain.ErrForbidden
	}
	accounts, err := s.Repo.ListAccounts(ctx)
	if err != nil {
		return nil, nil, err
	}
	invites, err := s.Repo.UnusedInvites(ctx)
	if err != nil {
		return nil, nil, err
	}
	now := s.Now()
	out := make([]domain.ListedInvite, 0, len(invites))
	for _, inv := range invites {
		out = append(out, domain.ListedInvite{Invite: inv, Open: inv.Open(now)})
	}
	return accounts, out, nil
}

// AdminAccount reads one Account's page for an Admin: how it signs in, what it has live, its history.
func (s *Service) AdminAccount(ctx context.Context, actor string, id domain.AccountID) (domain.Detail, error) {
	if !s.IsAdmin(ctx, actor) {
		return domain.Detail{}, domain.ErrForbidden
	}
	a, err := s.Repo.AccountByID(ctx, id)
	if err != nil {
		return domain.Detail{}, err
	}
	p, err := s.profile(ctx, a)
	if err != nil {
		return domain.Detail{}, err
	}
	p.AdminPowers = false
	out := domain.Detail{Profile: p, Sessions: 0, Tokens: 0, Events: nil}
	if out.Sessions, out.Tokens, err = s.Repo.LiveCounts(ctx, id, s.Now()); err != nil {
		return out, err
	}
	out.Events, err = s.Repo.Events(ctx, id)
	return out, err
}

// adminControl runs an Admin's change to an Account in one transaction and records it.
func (s *Service) adminControl(ctx context.Context, actor string, id domain.AccountID, action string, change func(Repository, domain.Account) error) error {
	if !s.IsAdmin(ctx, actor) {
		return domain.ErrForbidden
	}
	return s.Repo.InTx(ctx, func(r Repository) error {
		a, err := r.AccountByID(ctx, id)
		if err != nil {
			return err
		}
		if err := change(r, a); err != nil {
			return err
		}
		return s.record(ctx, r, id, actor, action, "")
	})
}

// SendSignInLink emails an Account's holder a sign-in link, for an Admin.
func (s *Service) SendSignInLink(ctx context.Context, actor string, id domain.AccountID) error {
	return s.adminControl(ctx, actor, id, domain.EventSignInLinkSent, func(r Repository, a domain.Account) error {
		if a.Disabled {
			return domain.ErrConflict
		}
		return s.mailLink(ctx, r, a)
	})
}

// SetAdmin makes an Account an Admin or not, for an Admin; nobody removes their own Admin role.
func (s *Service) SetAdmin(ctx context.Context, actor string, id domain.AccountID, admin bool) error {
	action := map[bool]string{true: domain.EventAdminGranted, false: domain.EventAdminRevoked}[admin]
	return s.adminControl(ctx, actor, id, action, func(r Repository, a domain.Account) error {
		if a.Subject == actor && !admin {
			return domain.ErrConflict
		}
		return r.SetAdmin(ctx, id, admin)
	})
}

// SetDisabled disables an Account or enables it again, for an Admin. Disabling ends every session and
// Access Token it has; nobody disables themselves.
func (s *Service) SetDisabled(ctx context.Context, actor string, id domain.AccountID, disabled bool) error {
	action := map[bool]string{true: domain.EventDisabled, false: domain.EventEnabled}[disabled]
	return s.adminControl(ctx, actor, id, action, func(r Repository, a domain.Account) error {
		if a.Subject == actor {
			return domain.ErrConflict
		}
		if err := r.SetDisabled(ctx, id, disabled); err != nil || !disabled {
			return err
		}
		return r.RevokeEverything(ctx, id, s.Now())
	})
}

// ResetTwoStep turns an Account's two-step off for a holder who lost their phone and recovery codes,
// for an Admin.
func (s *Service) ResetTwoStep(ctx context.Context, actor string, id domain.AccountID) error {
	return s.adminControl(ctx, actor, id, domain.EventTwoStepReset, func(r Repository, _ domain.Account) error {
		return r.DeleteTOTP(ctx, id)
	})
}

// History reads the signed-in Account's own history.
func (s *Service) History(ctx context.Context, subject string) ([]domain.Event, error) {
	a, err := s.Repo.AccountBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	return s.Repo.Events(ctx, a.ID)
}
