package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	playdomain "github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// liveTimeout is how long a REST call waits for its live Session to answer.
func (h *Handler) liveTimeout() time.Duration {
	if h.LiveTimeout > 0 {
		return h.LiveTimeout
	}
	return 10 * time.Second
}

// joinLive connects the caller to a live Session as their live connection would, DM or party.
func (h *Handler) joinLive(ctx context.Context, c caller.Caller, campaign, sid uuid.UUID) (*live.Subscriber, *oas.ProblemStatusCodeWithHeaders) {
	me, err := h.LiveMembers.Membership(ctx, campaign, c.Subject)
	var s playdomain.Session
	if err == nil {
		s, err = h.Sessions.Get(ctx, c, campaign, playdomain.SessionID(sid))
	}
	if err != nil {
		return nil, h.campaignProblem(ctx, "join session", err)
	}
	if s.Status == playdomain.SessionEnded {
		return nil, problem(http.StatusConflict, "Session ended", "That session has ended.")
	}
	audience := live.AudienceParty
	if me.DM {
		audience = live.AudienceDM
	}
	sub, err := h.Hub.Join(ctx, playdomain.SessionID(sid), me, c, audience)
	var away *live.ElsewhereError
	if errors.As(err, &away) {
		return nil, problem(http.StatusConflict, "Another group", "The party is split and you play with another group, in session "+uuid.UUID(away.Session).String()+".")
	}
	if err != nil {
		return nil, h.campaignProblem(ctx, "join session", err)
	}
	return sub, nil
}

// next waits for the Session's next Update, or reports that none came in time.
func next(ctx context.Context, sub *live.Subscriber) (live.Update, bool) {
	select {
	case u, ok := <-sub.Out:
		return u, ok
	case <-ctx.Done():
		return live.Update{}, false
	}
}

var errSilent = problem(http.StatusServiceUnavailable, "Service unavailable", "The session did not answer. Try again shortly.") //nolint:gochecknoglobals // a fixed problem

// GetSessionView shows what the caller may see of a live Session now.
func (h *Handler) GetSessionView(ctx context.Context, p oas.GetSessionViewParams) (oas.GetSessionViewRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	sub, prob := h.joinLive(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.SessionId))
	if prob != nil {
		return prob, nil
	}
	defer h.Hub.Leave(sub)
	ctx, cancel := context.WithTimeout(ctx, h.liveTimeout())
	defer cancel()
	u, ok := next(ctx, sub)
	if !ok || u.View == nil {
		return errSilent, nil
	}
	var out oas.LiveView
	convert(u.View, &out)
	return &oas.LiveViewHeaders{Response: out}, nil
}

// SendLiveCommand runs one command in a live Session as the caller and answers with the result.
func (h *Handler) SendLiveCommand(ctx context.Context, req *oas.LiveCommand, p oas.SendLiveCommandParams) (oas.SendLiveCommandRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	var cmd live.Command
	convert(req, &cmd)
	cmd.Nonce = uuid.NewString()
	sub, prob := h.joinLive(ctx, c, uuid.UUID(p.CampaignId), uuid.UUID(p.SessionId))
	if prob != nil {
		return prob, nil
	}
	defer h.Hub.Leave(sub)
	ctx, cancel := context.WithTimeout(ctx, h.liveTimeout())
	defer cancel()
	if _, ok := next(ctx, sub); !ok {
		return errSilent, nil
	}
	h.Hub.Submit(sub, cmd)
	for {
		u, ok := next(ctx, sub)
		switch {
		case !ok:
			return errSilent, nil
		case u.Kind == live.UpdRejected && u.Nonce == cmd.Nonce:
			return problem(http.StatusUnprocessableEntity, "Not allowed by the rules", u.Reason), nil
		case u.Nonce == cmd.Nonce, cmd.Kind == live.CmdPing && u.Kind == live.UpdPing, cmd.Kind == live.CmdResync && u.Kind == live.UpdSnapshot:
			var out oas.LiveCommandResult
			convert(struct {
				Seq       int64               `json:"seq"`
				ActionSeq int64               `json:"actionSeq,omitempty"`
				Path      *live.PathView      `json:"path,omitempty"`
				Preview   *live.AttackPreview `json:"preview,omitempty"`
				Area      *live.AreaPreview   `json:"area,omitempty"`
			}{u.Seq, u.ActionSeq, u.Path, u.Preview, u.Area}, &out)
			return &oas.LiveCommandResultHeaders{Response: out}, nil
		}
	}
}

// GetSessionLog lists a Session's latest Actions.
func (h *Handler) GetSessionLog(ctx context.Context, p oas.GetSessionLogParams) (oas.GetSessionLogRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Sessions.Log(ctx, c, uuid.UUID(p.CampaignId), playdomain.SessionID(p.SessionId), int(p.Limit.Or(30)))
	if err != nil {
		return h.campaignProblem(ctx, "session log", err), nil
	}
	out := make([]oas.SessionAction, 0, len(list))
	for _, a := range list {
		x := oas.SessionAction{
			Seq: int32(a.Seq), Kind: a.Kind, Actor: oas.DisplayName(a.Actor), Origin: oas.SessionActionOrigin(a.Origin), Label: a.Label, //nolint:gosec // sequences stay within int32
			Undoable: a.Undoable(), CreatedAt: a.At.UTC(),
		}
		if a.Client != "" {
			x.Client = oas.NewOptString(a.Client)
		}
		if a.Token != nil {
			x.TokenId = oas.NewOptID(oas.ID(*a.Token))
		}
		out = append(out, x)
	}
	return &oas.GetSessionLogOKHeaders{Response: out}, nil
}

// convert moves a value between the live wire types and the generated ones through their JSON; the
// contract test keeps both on one schema.
func convert(from, to any) {
	raw, _ := json.Marshal(from)
	_ = json.Unmarshal(raw, to)
}
