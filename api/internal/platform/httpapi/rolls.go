package httpapi

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// RollService is what the roll and Action Log operations need.
type RollService interface {
	Create(ctx context.Context, c caller.Caller, campaign uuid.UUID, in playapp.RollInput) (playdomain.Roll, error)
	Get(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.RollID) (playdomain.Roll, error)
	List(ctx context.Context, c caller.Caller, campaign uuid.UUID, limit int) ([]playdomain.Roll, error)
	Log(ctx context.Context, c caller.Caller, campaign uuid.UUID, limit int) ([]playdomain.Action, error)
	SetDie(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.RollID, no int, f playapp.Fill) (playdomain.Roll, error)
	RollRest(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.RollID) (playdomain.Roll, error)
}

const defaultLogPage = 30

//nolint:gosec // every narrowing conversion here is of small dice values
func rollOut(r playdomain.Roll, c caller.Caller, dm bool) oas.RollRequest {
	out := oas.RollRequest{
		ID: oas.ID(r.ID), Purpose: r.Purpose, Notation: r.Notation, RequestedBy: oas.DisplayName(r.RequestedBy),
		Roller: oas.MemberRef{ID: oas.ID(r.Roller.ID), Name: oas.DisplayName(r.Roller.Name)}, Mine: r.Roller.Subject == c.Subject,
		Status: oas.RollRequestStatus(r.Status), Groups: []oas.DiceGroup{}, Dice: make([]oas.RollDie, 0, len(r.Dice)),
		Modifiers: make([]oas.RollModifier, 0, len(r.Modifiers)), CreatedAt: r.CreatedAt.UTC(),
	}
	out.CanRoll = out.Mine || dm
	if r.Status == playdomain.StatusResolved {
		out.Total = oas.NewOptInt32(int32(r.Total))
		out.ResolvedAt = oas.NewOptDateTime(r.ResolvedAt.UTC())
	}
	spec, _ := dice.Parse(r.Notation) // stored notation was validated when the request was made
	for i, g := range spec.Groups {
		group := oas.DiceGroup{Index: int32(i), Count: int32(g.Count), Faces: int32(g.Faces), Sign: int32(g.Sign)}
		setOpt(&group.Label, r.Labels[i])
		if g.Keep != dice.KeepAll {
			group.Keep = oas.NewOptDiceGroupKeep(map[dice.Keep]oas.DiceGroupKeep{dice.KeepHighest: oas.DiceGroupKeepHighest, dice.KeepLowest: oas.DiceGroupKeepLowest}[g.Keep])
			group.KeepCount = oas.NewOptInt32(int32(g.KeepN))
		}
		out.Groups = append(out.Groups, group)
	}
	for _, d := range r.Dice {
		die := oas.RollDie{No: int32(d.No), Group: int32(d.Group), Faces: int32(d.Faces), Kept: d.Kept}
		if d.Value > 0 {
			die.Value = oas.NewOptInt32(int32(d.Value))
			die.Mode = oas.NewOptRollDieMode(oas.RollDieMode(d.Mode))
		}
		out.Dice = append(out.Dice, die)
	}
	for _, m := range r.Modifiers {
		out.Modifiers = append(out.Modifiers, oas.RollModifier{Label: m.Label, Value: int32(m.Value)})
	}
	return out
}

// isDM reports whether the caller runs the Campaign; it only shapes the canRoll hint.
func (h *Handler) isDM(ctx context.Context, c caller.Caller, campaign uuid.UUID) bool {
	d, err := h.Campaigns.Get(ctx, c, campaignID(campaign))
	return err == nil && d.Me.Role == "dm"
}

// ListRolls lists recent Roll Requests.
func (h *Handler) ListRolls(ctx context.Context, p oas.ListRollsParams) (oas.ListRollsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	campaign := uuid.UUID(p.CampaignId)
	rolls, err := h.Rolls.List(ctx, c, campaign, int(p.Limit.Or(defaultLogPage)))
	if err != nil {
		return h.campaignProblem(ctx, "list rolls", err), nil
	}
	dm := h.isDM(ctx, c, campaign)
	out := make([]oas.RollRequest, 0, len(rolls))
	for _, r := range rolls {
		out = append(out, rollOut(r, c, dm))
	}
	return &oas.ListRollsOKHeaders{Response: out}, nil
}

// CreateRoll opens a Roll Request.
func (h *Handler) CreateRoll(ctx context.Context, req *oas.RollCreate, p oas.CreateRollParams) (oas.CreateRollRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	in := playapp.RollInput{Purpose: req.Purpose, Notation: req.Notation, Labels: map[int]string{}}
	for _, l := range req.Labels {
		in.Labels[int(l.Group)] = l.Label
	}
	for _, m := range req.Modifiers {
		in.Modifiers = append(in.Modifiers, playdomain.Modifier{Label: m.Label, Value: int(m.Value)})
	}
	if v, set := req.RollerId.Get(); set {
		roller := uuid.UUID(v)
		in.Roller = &roller
	}
	campaign := uuid.UUID(p.CampaignId)
	r, err := h.Rolls.Create(ctx, c, campaign, in)
	if err != nil {
		return h.campaignProblem(ctx, "create roll", err), nil
	}
	return &oas.RollRequestHeaders{Response: rollOut(r, c, h.isDM(ctx, c, campaign))}, nil
}

// GetRoll returns one Roll Request.
func (h *Handler) GetRoll(ctx context.Context, p oas.GetRollParams) (oas.GetRollRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	campaign := uuid.UUID(p.CampaignId)
	r, err := h.Rolls.Get(ctx, c, campaign, playdomain.RollID(p.RollId))
	if err != nil {
		return h.campaignProblem(ctx, "get roll", err), nil
	}
	return &oas.RollRequestHeaders{Response: rollOut(r, c, h.isDM(ctx, c, campaign))}, nil
}

// SetDie rolls or enters one die.
func (h *Handler) SetDie(ctx context.Context, req *oas.DieFill, p oas.SetDieParams) (oas.SetDieRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	campaign := uuid.UUID(p.CampaignId)
	fill := playapp.Fill{Auto: req.Mode == oas.DieFillModeAuto, Value: int(req.Value.Or(0))}
	r, err := h.Rolls.SetDie(ctx, c, campaign, playdomain.RollID(p.RollId), int(p.DieNo), fill)
	if err != nil {
		return h.campaignProblem(ctx, "set die", err), nil
	}
	return &oas.RollRequestHeaders{Response: rollOut(r, c, h.isDM(ctx, c, campaign))}, nil
}

// RollRest rolls every empty die.
func (h *Handler) RollRest(ctx context.Context, p oas.RollRestParams) (oas.RollRestRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	campaign := uuid.UUID(p.CampaignId)
	r, err := h.Rolls.RollRest(ctx, c, campaign, playdomain.RollID(p.RollId))
	if err != nil {
		return h.campaignProblem(ctx, "roll rest", err), nil
	}
	return &oas.RollRequestHeaders{Response: rollOut(r, c, h.isDM(ctx, c, campaign))}, nil
}

// GetActionLog returns the Campaign's recent Actions.
func (h *Handler) GetActionLog(ctx context.Context, p oas.GetActionLogParams) (oas.GetActionLogRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	actions, err := h.Rolls.Log(ctx, c, uuid.UUID(p.CampaignId), int(p.Limit.Or(defaultLogPage)))
	if err != nil {
		return h.campaignProblem(ctx, "action log", err), nil
	}
	out := make([]oas.ActionEntry, 0, len(actions))
	for _, a := range actions {
		e := oas.ActionEntry{
			Seq: a.Seq, Kind: oas.ActionEntryKind(a.Kind), Actor: oas.DisplayName(a.Actor), Origin: oas.ActionEntryOrigin(a.Origin),
			Value: int32(a.Value), CreatedAt: a.CreatedAt.UTC(), //nolint:gosec // dice values
		}
		setOpt(&e.Client, a.Client)
		if a.Seed != nil {
			e.Seed = oas.NewOptString(strconv.FormatUint(*a.Seed, 10))
		}
		if a.RollID != nil {
			e.RollId = oas.NewOptID(oas.ID(*a.RollID))
		}
		if a.DieNo != nil {
			e.DieNo = oas.NewOptInt32(int32(*a.DieNo)) //nolint:gosec // bounded
		}
		out = append(out, e)
	}
	return &oas.GetActionLogOKHeaders{Response: out}, nil
}
