package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// SessionService is what the Session operations need.
type SessionService interface {
	Start(ctx context.Context, c caller.Caller, campaign uuid.UUID) (playdomain.Session, error)
	Get(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.SessionID) (playdomain.Session, error)
	List(ctx context.Context, c caller.Caller, campaign uuid.UUID) ([]playdomain.Session, error)
	End(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.SessionID) (playdomain.Session, error)
	Log(ctx context.Context, c caller.Caller, campaign uuid.UUID, id playdomain.SessionID, limit int) ([]playdomain.LoggedAction, error)
}

// LiveHub is what the live socket needs from the runtime.
type LiveHub interface {
	Join(ctx context.Context, id playdomain.SessionID, m playdomain.Member, c caller.Caller, a live.Audience) (*live.Subscriber, error)
	Leave(sub *live.Subscriber)
	Submit(sub *live.Subscriber, cmd live.Command)
}

// LiveMembers resolves who is connecting.
type LiveMembers interface {
	Membership(ctx context.Context, campaign uuid.UUID, subject string) (playdomain.Member, error)
}

func sessionOut(s playdomain.Session) oas.PlaySession {
	out := oas.PlaySession{
		ID: oas.ID(s.ID), Number: int32(s.Number), Status: oas.PlaySessionStatus(s.Status), Seq: int32(s.Seq), //nolint:gosec // small numbers
		GridRadius: int32(s.GridRadius), StartedAt: s.StartedAt.UTC(), //nolint:gosec // 1..60
	}
	if s.Status == playdomain.SessionEnded {
		out.EndedAt = oas.NewOptDateTime(s.EndedAt.UTC())
	}
	return out
}

// ListSessions lists the Campaign's Sessions.
func (h *Handler) ListSessions(ctx context.Context, p oas.ListSessionsParams) (oas.ListSessionsRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Sessions.List(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list sessions", err), nil
	}
	out := make([]oas.PlaySession, 0, len(list))
	for _, s := range list {
		out = append(out, sessionOut(s))
	}
	return &oas.ListSessionsOKHeaders{Response: out}, nil
}

// StartSession opens a live Session.
func (h *Handler) StartSession(ctx context.Context, p oas.StartSessionParams) (oas.StartSessionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Sessions.Start(ctx, c, uuid.UUID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "start session", err), nil
	}
	return &oas.PlaySessionHeaders{Response: sessionOut(s)}, nil
}

// GetSession returns one Session.
func (h *Handler) GetSession(ctx context.Context, p oas.GetSessionParams) (oas.GetSessionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Sessions.Get(ctx, c, uuid.UUID(p.CampaignId), playdomain.SessionID(p.SessionId))
	if err != nil {
		return h.campaignProblem(ctx, "get session", err), nil
	}
	return &oas.PlaySessionHeaders{Response: sessionOut(s)}, nil
}

// EndSession ends a live Session.
func (h *Handler) EndSession(ctx context.Context, p oas.EndSessionParams) (oas.EndSessionRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Sessions.End(ctx, c, uuid.UUID(p.CampaignId), playdomain.SessionID(p.SessionId))
	if err != nil {
		return h.campaignProblem(ctx, "end session", err), nil
	}
	return &oas.PlaySessionHeaders{Response: sessionOut(s)}, nil
}

// LiveSocket serves /api/v1/campaigns/{campaignId}/sessions/{sessionId}/live. It checks identity,
// membership and audience before upgrading, then relays frames between the socket and the runtime.
func (h *Handler) LiveSocket(w http.ResponseWriter, r *http.Request) {
	subject := r.Header.Get(httpx.IdentityHeader)
	if subject == "" {
		httpx.WriteProblem(w, http.StatusUnauthorized, "Unauthorized", "Sign in to continue.")
		return
	}
	campaign, err1 := uuid.Parse(r.PathValue("campaignId"))
	sid, err2 := uuid.Parse(r.PathValue("sessionId"))
	if err1 != nil || err2 != nil {
		httpx.WriteProblem(w, http.StatusNotFound, "Not found", "No such session.")
		return
	}
	c := caller.UI(subject)
	me, err := h.LiveMembers.Membership(r.Context(), campaign, subject)
	if err == nil {
		_, err = h.Sessions.Get(r.Context(), c, campaign, playdomain.SessionID(sid))
	}
	audience := live.Audience(r.URL.Query().Get("audience"))
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		httpx.WriteProblem(w, http.StatusNotFound, "Not found", "No such session.")
		return
	case err != nil:
		h.Log.ErrorContext(r.Context(), "live socket", "error", err)
		httpx.WriteProblem(w, http.StatusServiceUnavailable, "Service unavailable", "Try again shortly.")
		return
	case audience != live.AudienceDM && audience != live.AudienceParty && audience != live.AudienceTable:
		httpx.WriteProblem(w, http.StatusBadRequest, "Bad request", "Choose the dm, party or table audience.")
		return
	case audience == live.AudienceDM && !me.DM:
		httpx.WriteProblem(w, http.StatusForbidden, "Forbidden", "Only the DM can see everything.")
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	h.relay(r.Context(), conn, playdomain.SessionID(sid), me, c, audience)
}

func (h *Handler) relay(ctx context.Context, conn *websocket.Conn, sid playdomain.SessionID, me playdomain.Member, c caller.Caller, a live.Audience) {
	sub, err := h.Hub.Join(ctx, sid, me, c, a)
	var away *live.ElsewhereError
	if errors.As(err, &away) {
		// The party is split and this screen belongs with another group: it is told where, shown nothing,
		// and let go.
		_ = wsjson.Write(ctx, conn, live.Update{Kind: live.UpdRegroup, Group: &live.GroupView{SessionID: uuid.UUID(away.Session).String(), Tokens: []string{}}})
		_ = conn.Close(websocket.StatusNormalClosure, "regroup")
		return
	}
	if err != nil {
		_ = conn.Close(websocket.StatusTryAgainLater, "session unavailable")
		return
	}
	go func() {
		for u := range sub.Out {
			if err := wsjson.Write(ctx, conn, u); err != nil {
				break
			}
		}
		_ = conn.Close(websocket.StatusNormalClosure, "bye")
	}()
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			h.Hub.Leave(sub)
			return
		}
		var cmd live.Command
		if json.Unmarshal(raw, &cmd) == nil {
			h.Hub.Submit(sub, cmd)
		}
	}
}
