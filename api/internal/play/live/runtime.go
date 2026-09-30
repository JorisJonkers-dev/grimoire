package live

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Change is one token change the runtime asks the store to write.
type Change struct {
	Kind  string
	Token domain.Token
}

// Store is the runtime's persistence port.
type Store interface {
	Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, error)
	Apply(ctx context.Context, s domain.Session, ch Change, actor domain.Member, c caller.Caller, now time.Time) (int64, domain.Token, error)
}

// Owner guarantees one runtime per Session across processes.
type Owner interface {
	Acquire(ctx context.Context, id domain.SessionID) (func(), error)
}

// ErrClosed is returned when joining a Session whose runtime has shut down.
var ErrClosed = errors.New("live: session closed")

// OutboxSize is how many Updates a slow connection may fall behind before it is dropped.
const OutboxSize = 64

// Subscriber is one connection to a Session.
type Subscriber struct {
	Member   domain.Member
	Caller   caller.Caller
	Audience Audience
	// Out delivers Updates; it is closed when the connection must end.
	Out chan Update
	rt  *runtime
}

type request struct {
	from *Subscriber
	cmd  Command
}

type runtime struct {
	store   Store
	now     func() time.Time
	log     *slog.Logger
	release func()
	session domain.Session
	tokens  map[domain.TokenID]domain.Token
	subs    map[*Subscriber]struct{}
	join    chan *Subscriber
	leave   chan *Subscriber
	cmds    chan request
	stop    chan struct{}
	done    chan struct{}
}

// Hub starts, finds and stops Session runtimes.
type Hub struct {
	Store Store
	Owner Owner
	Now   func() time.Time
	Log   *slog.Logger

	mu       sync.Mutex
	runtimes map[domain.SessionID]*runtime
}

// Join connects a subscriber, starting the Session's runtime if it is not running yet.
func (h *Hub) Join(ctx context.Context, id domain.SessionID, m domain.Member, c caller.Caller, a Audience) (*Subscriber, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rt, ok := h.runtimes[id]
	if !ok {
		var err error
		if rt, err = h.start(ctx, id); err != nil {
			return nil, err
		}
	}
	sub := &Subscriber{Member: m, Caller: c, Audience: a, Out: make(chan Update, OutboxSize), rt: rt}
	// Close removes a runtime under the same lock before stopping it, so this runtime is still running.
	rt.join <- sub
	return sub, nil
}

func (h *Hub) start(ctx context.Context, id domain.SessionID) (*runtime, error) {
	release, err := h.Owner.Acquire(ctx, id)
	if err != nil {
		return nil, err
	}
	s, tokens, err := h.Store.Load(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	if s.Status != domain.SessionLive {
		release()
		return nil, ErrClosed
	}
	rt := &runtime{
		store: h.Store, now: h.Now, log: h.Log, release: release, session: s, tokens: map[domain.TokenID]domain.Token{},
		subs: map[*Subscriber]struct{}{}, join: make(chan *Subscriber), leave: make(chan *Subscriber), cmds: make(chan request),
		stop: make(chan struct{}), done: make(chan struct{}),
	}
	for _, t := range tokens {
		rt.tokens[t.ID] = t
	}
	if h.runtimes == nil {
		h.runtimes = map[domain.SessionID]*runtime{}
	}
	h.runtimes[id] = rt
	go rt.run()
	return rt, nil
}

// Leave disconnects a subscriber.
func (h *Hub) Leave(sub *Subscriber) {
	select {
	case sub.rt.leave <- sub:
	case <-sub.rt.done:
	}
}

// Submit hands a command to the subscriber's Session.
func (h *Hub) Submit(sub *Subscriber, cmd Command) {
	select {
	case sub.rt.cmds <- request{from: sub, cmd: cmd}:
	case <-sub.rt.done:
	}
}

// Close ends a Session's runtime: everyone gets an ended Update and is disconnected.
func (h *Hub) Close(id domain.SessionID) {
	h.mu.Lock()
	rt, ok := h.runtimes[id]
	delete(h.runtimes, id)
	h.mu.Unlock()
	if ok {
		close(rt.stop)
		<-rt.done
	}
}

// Shutdown closes every runtime.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	ids := make([]domain.SessionID, 0, len(h.runtimes))
	for id := range h.runtimes {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	for _, id := range ids {
		h.Close(id)
	}
}

func (r *runtime) run() {
	defer close(r.done)
	defer r.release()
	for {
		select {
		case sub := <-r.join:
			r.subs[sub] = struct{}{}
			r.send(sub, r.snapshot(sub.Audience))
		case sub := <-r.leave:
			r.drop(sub)
		case req := <-r.cmds:
			r.handle(req)
		case <-r.stop:
			for sub := range r.subs {
				r.send(sub, Update{Kind: UpdEnded, Seq: r.session.Seq})
				r.drop(sub)
			}
			return
		}
	}
}

// send never blocks the runtime: a connection that cannot keep up is dropped and will resync.
func (r *runtime) send(sub *Subscriber, u Update) {
	if _, ok := r.subs[sub]; !ok {
		return
	}
	select {
	case sub.Out <- u:
	default:
		r.drop(sub)
	}
}

func (r *runtime) drop(sub *Subscriber) {
	if _, ok := r.subs[sub]; ok {
		delete(r.subs, sub)
		close(sub.Out)
	}
}

func (r *runtime) snapshot(a Audience) Update {
	u := Update{
		Kind: UpdSnapshot, Seq: r.session.Seq, Tokens: []TokenView{},
		Session: &SessionView{ID: uuid.UUID(r.session.ID).String(), Number: r.session.Number, GridRadius: r.session.GridRadius, Audience: a},
	}
	for _, t := range r.tokens {
		if sees(a, t) {
			u.Tokens = append(u.Tokens, view(t))
		}
	}
	return u
}

func (r *runtime) reject(req request, reason string) {
	r.send(req.from, Update{Kind: UpdRejected, Seq: r.session.Seq, Nonce: req.cmd.Nonce, Reason: reason})
}

func (r *runtime) handle(req request) {
	if req.cmd.Kind == CmdResync {
		r.send(req.from, r.snapshot(req.from.Audience))
		return
	}
	if !req.from.Member.DM {
		r.reject(req, "Only the DM can change tokens.")
		return
	}
	ch, reason := r.change(req.cmd)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	before, existed := r.tokens[ch.Token.ID]
	seq, after, err := r.store.Apply(context.Background(), r.session, ch, req.from.Member, req.from.Caller, r.now())
	if err != nil {
		r.log.Error("live: apply", "error", err)
		r.reject(req, "That change could not be saved.")
		return
	}
	r.session.Seq = seq
	if ch.Kind == domain.ActionTokenRemoved {
		delete(r.tokens, after.ID)
	} else {
		r.tokens[after.ID] = after
	}
	for sub := range r.subs {
		r.send(sub, r.project(sub.Audience, before, existed, after, ch.Kind != domain.ActionTokenRemoved, seq, req))
	}
}

// project turns one change into what an audience may learn from it.
func (r *runtime) project(a Audience, before domain.Token, existed bool, after domain.Token, exists bool, seq int64, req request) Update {
	u := Update{Kind: UpdTick, Seq: seq}
	was, is := existed && sees(a, before), exists && sees(a, after)
	switch {
	case is:
		v := view(after)
		u.Kind, u.Token = UpdToken, &v
	case was:
		u.Kind, u.TokenID = UpdTokenRemoved, uuid.UUID(after.ID).String()
	}
	if a == AudienceDM && req.from.Audience == AudienceDM {
		u.Nonce = req.cmd.Nonce
	}
	return u
}

// change validates a command against the Session and builds the token change it asks for.
func (r *runtime) change(cmd Command) (Change, string) {
	inside := hex.Distance(hex.Coord{Q: 0, R: 0}, hex.Coord{Q: cmd.Q, R: cmd.R}) <= r.session.GridRadius
	if cmd.Kind == CmdPlace {
		label := strings.TrimSpace(cmd.Label)
		switch {
		case label == "" || len([]rune(label)) > 40:
			return Change{}, "Give the token a name of up to 40 characters."
		case cmd.TokenKind != domain.TokenParty && cmd.TokenKind != domain.TokenEnemy && cmd.TokenKind != domain.TokenNPC && cmd.TokenKind != domain.TokenObject:
			return Change{}, "Unknown token kind."
		case !inside:
			return Change{}, "That hex is off the map."
		}
		return Change{Kind: domain.ActionTokenPlaced, Token: domain.Token{Label: label, Kind: cmd.TokenKind, Q: cmd.Q, R: cmd.R, Hidden: cmd.Hidden}}, ""
	}
	id, err := uuid.Parse(cmd.TokenID)
	t, ok := r.tokens[domain.TokenID(id)]
	if err != nil || !ok {
		return Change{}, "No such token."
	}
	switch cmd.Kind {
	case CmdMove:
		if !inside {
			return Change{}, "That hex is off the map."
		}
		t.Q, t.R = cmd.Q, cmd.R
		return Change{Kind: domain.ActionTokenMoved, Token: t}, ""
	case CmdSetHidden:
		t.Hidden = cmd.Hidden
		kind := domain.ActionTokenRevealed
		if cmd.Hidden {
			kind = domain.ActionTokenHidden
		}
		return Change{Kind: kind, Token: t}, ""
	case CmdRemove:
		return Change{Kind: domain.ActionTokenRemoved, Token: t}, ""
	default:
		return Change{}, "Unknown command."
	}
}
