package app

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// MaxRuleHooks is how many Rule Variants of its own a Campaign keeps.
const MaxRuleHooks = 50

// RuleHookRepository keeps the Rule Variants a DM authors, and knows the Roll Tables a Campaign sees.
type RuleHookRepository interface {
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	RuleHooks(ctx context.Context, id domain.CampaignID) ([]domain.RuleHook, error)
	CampaignRollTables(ctx context.Context, id domain.CampaignID) ([]domain.RollTableRef, error)
	InsertRuleHook(ctx context.Context, h domain.RuleHook) error
	// DeleteRuleHook reports ErrNotFound for a hook the Campaign does not have.
	DeleteRuleHook(ctx context.Context, id domain.CampaignID, hook domain.HookID) error
}

// RuleHooks runs the use cases of the Rule Variants a DM authors from hook points.
type RuleHooks struct {
	Repo RuleHookRepository
	Now  func() time.Time
}

// RuleHookView is a hook with the name of the Roll Table it rolls on; the name is empty for a table
// the Campaign no longer sees.
type RuleHookView struct {
	Hook      domain.RuleHook
	TableName string
}

// RuleHooksView is a Campaign's own Rule Variants, the hook points there are, and for the DM the Roll
// Tables to choose from.
type RuleHooksView struct {
	DM     bool
	Hooks  []RuleHookView
	Points []variants.HookPoint
	Tables []domain.RollTableRef
}

// List shows a Member the Campaign's own Rule Variants.
func (s *RuleHooks) List(ctx context.Context, c caller.Caller, id domain.CampaignID) (RuleHooksView, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return RuleHooksView{}, err
	}
	hooks, err := s.Repo.RuleHooks(ctx, id)
	if err != nil {
		return RuleHooksView{}, err
	}
	tables, err := s.Repo.CampaignRollTables(ctx, id)
	if err != nil {
		return RuleHooksView{}, err
	}
	v := RuleHooksView{DM: me.Role == domain.RoleDM, Hooks: make([]RuleHookView, 0, len(hooks)), Points: variants.HookPoints(), Tables: []domain.RollTableRef{}}
	for _, h := range hooks {
		row := RuleHookView{Hook: h, TableName: ""}
		if i := slices.IndexFunc(tables, func(t domain.RollTableRef) bool { return h.RollTable != nil && t.ID == *h.RollTable }); i >= 0 {
			row.TableName = tables[i].Name
		}
		v.Hooks = append(v.Hooks, row)
	}
	if v.DM {
		v.Tables = tables
	}
	return v, nil
}

// RuleHookInput is a new hook: its name, its hook point, and either a Roll Table or an Effect.
type RuleHookInput struct {
	Name      string
	Hook      string
	RollTable *uuid.UUID
	Effect    string
}

// Create adds a Rule Variant of the Campaign's own. It names exactly one outcome: a Roll Table the
// Campaign sees, or an Effect. DM only.
func (s *RuleHooks) Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in RuleHookInput) (domain.RuleHook, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return domain.RuleHook{}, err
	}
	if me.Role != domain.RoleDM {
		return domain.RuleHook{}, domain.ErrForbidden
	}
	name, effect := strings.TrimSpace(in.Name), strings.TrimSpace(in.Effect)
	if name == "" || utf8.RuneCountInString(name) > 80 || !variants.ValidHook(in.Hook) || len(effect) > 80 || (in.RollTable == nil) == (effect == "") {
		return domain.RuleHook{}, domain.ErrInvalid
	}
	if in.RollTable != nil {
		tables, err := s.Repo.CampaignRollTables(ctx, id)
		if err != nil {
			return domain.RuleHook{}, err
		}
		if !slices.ContainsFunc(tables, func(t domain.RollTableRef) bool { return t.ID == *in.RollTable }) {
			return domain.RuleHook{}, refuse("that Roll Table is not one this Campaign sees")
		}
	}
	existing, err := s.Repo.RuleHooks(ctx, id)
	if err != nil {
		return domain.RuleHook{}, err
	}
	if len(existing) >= MaxRuleHooks {
		return domain.RuleHook{}, refuse("a Campaign keeps up to 50 Rule Variants of its own")
	}
	h := domain.RuleHook{ID: uuid.New(), CampaignID: id, Name: name, Hook: in.Hook, RollTable: in.RollTable, Effect: effect, CreatedAt: s.Now()}
	return h, s.Repo.InsertRuleHook(ctx, h)
}

// Delete removes one of the Campaign's own Rule Variants. DM only.
func (s *RuleHooks) Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, hook domain.HookID) error {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return err
	}
	if me.Role != domain.RoleDM {
		return domain.ErrForbidden
	}
	return s.Repo.DeleteRuleHook(ctx, id, hook)
}
