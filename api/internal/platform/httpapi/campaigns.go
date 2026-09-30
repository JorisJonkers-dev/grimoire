package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Campaigns is what the campaign operations need.
type Campaigns interface {
	Create(ctx context.Context, c caller.Caller, in app.CreateInput) (domain.Detail, error)
	List(ctx context.Context, c caller.Caller, after *domain.ListCursor, pageSize int) ([]domain.Summary, error)
	Get(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Detail, error)
	Update(ctx context.Context, c caller.Caller, id domain.CampaignID, in app.UpdateInput) (domain.Campaign, error)
	SetRole(ctx context.Context, c caller.Caller, id domain.CampaignID, m domain.MemberID, role domain.Role) (domain.Member, error)
	RemoveMember(ctx context.Context, c caller.Caller, id domain.CampaignID, m domain.MemberID) error
	CreateInvite(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.NewInvite, error)
	Invites(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.Invite, error)
	RevokeInvite(ctx context.Context, c caller.Caller, id domain.CampaignID, invite domain.InviteID) error
	PreviewInvite(ctx context.Context, token string) (domain.InvitePreview, error)
	AcceptInvite(ctx context.Context, c caller.Caller, token, displayName string) (domain.CampaignID, error)
}

// campaignProblem maps a use-case error to a problem; anything unexpected is logged and hidden.
func (h *Handler) campaignProblem(ctx context.Context, op string, err error) *oas.ProblemStatusCodeWithHeaders {
	var rule *apperr.RuleError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return problem(http.StatusNotFound, "Not found", "No such campaign, member or invite.")
	case errors.Is(err, domain.ErrForbidden):
		return problem(http.StatusForbidden, "Forbidden", "Only the DM can do that.")
	case errors.Is(err, domain.ErrConflict):
		return problem(http.StatusConflict, "Conflict", "A campaign needs at least one DM.")
	case errors.As(err, &rule):
		return problem(http.StatusUnprocessableEntity, "Not allowed by the rules", rule.Reason)
	case errors.Is(err, domain.ErrInvalid):
		return problem(http.StatusUnprocessableEntity, "Invalid", "Check the values and try again.")
	case errors.Is(err, domain.ErrLocked):
		return problem(http.StatusConflict, "In combat", "Characters cannot be edited during combat.")
	default:
		h.Log.ErrorContext(ctx, op, "error", err)
		return unavailable()
	}
}

func campaignID(id uuid.UUID) domain.CampaignID { return domain.CampaignID(id) }

func uiCaller(ctx context.Context) (caller.Caller, bool) {
	id, ok := auth.FromContext(ctx)
	if !ok {
		return caller.Caller{}, false
	}
	return caller.UI(id.Subject), true
}

func unauthorized() *oas.ProblemStatusCodeWithHeaders {
	return problem(http.StatusUnauthorized, "Unauthorized", "Sign in to continue.")
}

type campaignCursor struct {
	T time.Time `json:"t"`
	I uuid.UUID `json:"i"`
}

func encodeCampaignCursor(s domain.Summary) string {
	raw, _ := json.Marshal(campaignCursor{T: s.CreatedAt, I: uuid.UUID(s.ID)}) //nolint:errchkjson // plain struct
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCampaignCursor(token string) (*domain.ListCursor, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, false
	}
	var c campaignCursor
	if json.Unmarshal(raw, &c) != nil || c.T.IsZero() {
		return nil, false
	}
	return &domain.ListCursor{CreatedAt: c.T, ID: domain.CampaignID(c.I)}, true
}

func memberOut(m domain.Member, subject string) oas.Member {
	return oas.Member{
		ID: oas.ID(m.ID), DisplayName: oas.DisplayName(m.DisplayName), Role: oas.Role(m.Role), JoinedAt: m.JoinedAt.UTC(),
		IsMe: m.Subject == subject,
	}
}

func summaryOut(s domain.Summary) oas.CampaignSummary {
	return oas.CampaignSummary{
		ID: oas.ID(s.ID), Name: oas.CampaignName(s.Name), Ruleset: oas.Ruleset(s.Ruleset), MyRole: oas.Role(s.MyRole),
		MemberCount: int32(s.MemberCount), CreatedAt: s.CreatedAt.UTC(), //nolint:gosec // member counts are small
		ReactionTimeoutS: oas.NewOptReactionTimeout(oas.ReactionTimeout(s.ReactionTimeoutS)), //nolint:gosec // 3 to 120 seconds
	}
}

func detailOut(d domain.Detail) oas.Campaign {
	out := oas.Campaign{
		ID: oas.ID(d.ID), Name: oas.CampaignName(d.Name), Ruleset: oas.Ruleset(d.Ruleset), MyRole: oas.Role(d.Me.Role),
		MemberCount: int32(len(d.Members)), CreatedAt: d.CreatedAt.UTC(), Me: memberOut(d.Me, d.Me.Subject), //nolint:gosec // member counts are small
		ReactionTimeoutS: oas.NewOptReactionTimeout(oas.ReactionTimeout(d.ReactionTimeoutS)), //nolint:gosec // 3 to 120 seconds
		Members:          make([]oas.Member, 0, len(d.Members)),
	}
	for _, m := range d.Members {
		out.Members = append(out.Members, memberOut(m, d.Me.Subject))
	}
	return out
}

func inviteOut(i domain.Invite) oas.Invite {
	return oas.Invite{ID: oas.ID(i.ID), CreatedAt: i.CreatedAt.UTC(), ExpiresAt: i.ExpiresAt.UTC(), CreatedBy: oas.DisplayName(i.CreatedByName)}
}

// ListCampaigns returns the caller's Campaigns, newest first.
func (h *Handler) ListCampaigns(ctx context.Context, p oas.ListCampaignsParams) (oas.ListCampaignsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var after *domain.ListCursor
	if token, set := p.Cursor.Get(); set {
		if after, ok = decodeCampaignCursor(token); !ok {
			return problem(http.StatusBadRequest, "Bad request", "The page cursor is not valid."), nil
		}
	}
	page := int(p.Limit.Or(defaultPageSize))
	list, err := h.Campaigns.List(ctx, c, after, page+1)
	if err != nil {
		return h.campaignProblem(ctx, "list campaigns", err), nil
	}
	out := oas.CampaignPage{Items: make([]oas.CampaignSummary, 0, min(len(list), page))}
	for i, s := range list {
		if i == page {
			out.NextCursor = oas.NewOptString(encodeCampaignCursor(list[page-1]))
			break
		}
		out.Items = append(out.Items, summaryOut(s))
	}
	return &oas.CampaignPageHeaders{Response: out}, nil
}

// CreateCampaign starts a Campaign with the caller as DM.
func (h *Handler) CreateCampaign(ctx context.Context, req *oas.CampaignCreate) (oas.CreateCampaignRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.Campaigns.Create(ctx, c, app.CreateInput{
		Name: string(req.Name), Ruleset: string(req.Ruleset.Or(oas.RulesetSrd2024)), DisplayName: string(req.DisplayName),
	})
	if err != nil {
		return h.campaignProblem(ctx, "create campaign", err), nil
	}
	return &oas.CampaignHeaders{Response: detailOut(d)}, nil
}

// GetCampaign returns a Campaign's home to one of its Members.
func (h *Handler) GetCampaign(ctx context.Context, p oas.GetCampaignParams) (oas.GetCampaignRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	d, err := h.Campaigns.Get(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "get campaign", err), nil
	}
	return &oas.CampaignHeaders{Response: detailOut(d)}, nil
}

// UpdateCampaign changes a Campaign's settings.
func (h *Handler) UpdateCampaign(ctx context.Context, req *oas.CampaignUpdate, p oas.UpdateCampaignParams) (oas.UpdateCampaignRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var in app.UpdateInput
	if v, set := req.Name.Get(); set {
		name := string(v)
		in.Name = &name
	}
	if v, set := req.Ruleset.Get(); set {
		ruleset := string(v)
		in.Ruleset = &ruleset
	}
	if v, set := req.ReactionTimeoutS.Get(); set {
		timeout := int(v)
		in.ReactionTimeoutS = &timeout
	}
	camp, err := h.Campaigns.Update(ctx, c, domain.CampaignID(p.CampaignId), in)
	if err != nil {
		return h.campaignProblem(ctx, "update campaign", err), nil
	}
	d, err := h.Campaigns.Get(ctx, c, camp.ID)
	if err != nil {
		return h.campaignProblem(ctx, "update campaign", err), nil
	}
	return &oas.CampaignSummaryHeaders{Response: summaryOut(domain.Summary{Campaign: camp, MyRole: d.Me.Role, MemberCount: len(d.Members)})}, nil
}

// UpdateMember changes a Member's role.
func (h *Handler) UpdateMember(ctx context.Context, req *oas.MemberUpdate, p oas.UpdateMemberParams) (oas.UpdateMemberRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	m, err := h.Campaigns.SetRole(ctx, c, domain.CampaignID(p.CampaignId), domain.MemberID(p.MemberId), domain.Role(req.Role))
	if err != nil {
		return h.campaignProblem(ctx, "update member", err), nil
	}
	return &oas.MemberHeaders{Response: memberOut(m, c.Subject)}, nil
}

// RemoveMember removes a Member or lets one leave.
func (h *Handler) RemoveMember(ctx context.Context, p oas.RemoveMemberParams) (oas.RemoveMemberRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Campaigns.RemoveMember(ctx, c, domain.CampaignID(p.CampaignId), domain.MemberID(p.MemberId)); err != nil {
		return h.campaignProblem(ctx, "remove member", err), nil
	}
	return &oas.RemoveMemberNoContent{}, nil
}

// ListInvites lists open invite links.
func (h *Handler) ListInvites(ctx context.Context, p oas.ListInvitesParams) (oas.ListInvitesRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	invites, err := h.Campaigns.Invites(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list invites", err), nil
	}
	out := make([]oas.Invite, 0, len(invites))
	for _, i := range invites {
		out = append(out, inviteOut(i))
	}
	return &oas.ListInvitesOKHeaders{Response: out}, nil
}

// CreateInvite opens an invite link.
func (h *Handler) CreateInvite(ctx context.Context, p oas.CreateInviteParams) (oas.CreateInviteRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	inv, err := h.Campaigns.CreateInvite(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "create invite", err), nil
	}
	i := inviteOut(inv.Invite)
	return &oas.NewInviteHeaders{Response: oas.NewInvite{
		ID: i.ID, CreatedAt: i.CreatedAt, ExpiresAt: i.ExpiresAt, CreatedBy: i.CreatedBy, Token: oas.Token(inv.Token),
	}}, nil
}

// RevokeInvite closes an invite link.
func (h *Handler) RevokeInvite(ctx context.Context, p oas.RevokeInviteParams) (oas.RevokeInviteRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Campaigns.RevokeInvite(ctx, c, domain.CampaignID(p.CampaignId), domain.InviteID(p.InviteId)); err != nil {
		return h.campaignProblem(ctx, "revoke invite", err), nil
	}
	return &oas.RevokeInviteNoContent{}, nil
}

// PreviewInvite shows where an invite link leads.
func (h *Handler) PreviewInvite(ctx context.Context, req *oas.InviteToken) (oas.PreviewInviteRes, error) {
	if _, ok := uiCaller(ctx); !ok {
		return unauthorized(), nil
	}
	pv, err := h.Campaigns.PreviewInvite(ctx, string(req.Token))
	if err != nil {
		return h.campaignProblem(ctx, "preview invite", err), nil
	}
	return &oas.InvitePreviewHeaders{Response: oas.InvitePreview{
		CampaignName: oas.CampaignName(pv.CampaignName), InvitedBy: oas.DisplayName(pv.InvitedBy),
	}}, nil
}

// AcceptInvite joins the caller to a Campaign.
func (h *Handler) AcceptInvite(ctx context.Context, req *oas.InviteAccept) (oas.AcceptInviteRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	id, err := h.Campaigns.AcceptInvite(ctx, c, string(req.Token), string(req.DisplayName))
	if err != nil {
		return h.campaignProblem(ctx, "accept invite", err), nil
	}
	return &oas.CampaignRefHeaders{Response: oas.CampaignRef{ID: oas.ID(id)}}, nil
}
