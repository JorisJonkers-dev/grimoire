package live

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Write is one change the runtime commits: the action kind plus what it touches. AutoReveal lists the
// hexes the party sees for the first time because of it; they are remembered from then on.
type Write struct {
	Kind       string
	Token      domain.Token
	Hexes      []hex.Coord
	Light      domain.MapLight
	Ambient    string
	MapID      *domain.MapID
	Path       []hex.Coord
	CostFt     int
	AutoReveal []hex.Coord
	// Combat is the Combat after the change, for the store to save; Rolls are Roll Requests it opens.
	Combat    *domain.Combat
	Rolls     []domain.Roll
	Combatant domain.CombatantID
	board     *domain.MapState
	frames    []*state
	total     int
	resource  combat.Resource
}

// Store is the runtime's persistence port.
type Store interface {
	Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error)
	LoadMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (*domain.MapState, error)
	LoadCombat(ctx context.Context, id domain.SessionID) (*domain.Combat, error)
	Roll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (domain.Roll, error)
	Commit(ctx context.Context, s domain.Session, board *domain.MapState, w Write, actor domain.Member, c caller.Caller, now time.Time) (int64, error)
}

// Members finds a Campaign's members.
type Members interface {
	Member(ctx context.Context, campaign, id uuid.UUID) (domain.Member, error)
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
	members Members
	now     func() time.Time
	log     *slog.Logger
	release func()
	st      *state
	subs    map[*Subscriber]struct{}
	join    chan *Subscriber
	leave   chan *Subscriber
	cmds    chan request
	stop    chan struct{}
	done    chan struct{}
}

// Hub starts, finds and stops Session runtimes.
type Hub struct {
	Store   Store
	Members Members
	Owner   Owner
	Now     func() time.Time
	Log     *slog.Logger

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
	s, tokens, board, err := h.Store.Load(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	if s.Status != domain.SessionLive {
		release()
		return nil, ErrClosed
	}
	fight, err := h.Store.LoadCombat(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	st := &state{session: s, tokens: map[domain.TokenID]domain.Token{}, combat: fight}
	for _, t := range tokens {
		st.tokens[t.ID] = t
	}
	st.setBoard(board)
	rt := &runtime{
		store: h.Store, members: h.Members, now: h.Now, log: h.Log, release: release, st: st, subs: map[*Subscriber]struct{}{},
		join: make(chan *Subscriber), leave: make(chan *Subscriber), cmds: make(chan request), stop: make(chan struct{}), done: make(chan struct{}),
	}
	if h.runtimes == nil {
		h.runtimes = map[domain.SessionID]*runtime{}
	}
	h.runtimes[id] = rt
	go rt.run()
	return rt, nil
}

// RollResolved tells the Campaign's running Sessions that a Roll Request resolved, so an initiative
// rolled on a Roll Card reaches its Combat.
func (h *Hub) RollResolved(campaign uuid.UUID, id domain.RollID) {
	h.mu.Lock()
	var targets []*runtime
	for _, rt := range h.runtimes {
		if rt.st.session.CampaignID == campaign {
			targets = append(targets, rt)
		}
	}
	h.mu.Unlock()
	for _, rt := range targets {
		select {
		case rt.cmds <- request{cmd: Command{Kind: cmdRollResolved, rollID: id}}:
		case <-rt.done:
		}
	}
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
	r.catchUp()
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
				r.send(sub, Update{Kind: UpdEnded, Seq: r.st.session.Seq})
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
	v := r.st.project(a)
	s := r.st.session
	return Update{
		Kind: UpdSnapshot, Seq: s.Seq, View: &v,
		Session: &SessionView{ID: uuid.UUID(s.ID).String(), Number: s.Number, GridRadius: s.GridRadius, Audience: a},
	}
}

func (r *runtime) reject(req request, reason string) {
	r.send(req.from, Update{Kind: UpdRejected, Seq: r.st.session.Seq, Nonce: req.cmd.Nonce, Reason: reason})
}

func (r *runtime) handle(req request) {
	switch {
	case req.from == nil:
		r.rolled(req)
		return
	case req.cmd.Kind == CmdResync:
		r.send(req.from, r.snapshot(req.from.Audience))
		return
	case req.cmd.Kind == CmdPlanWalk:
		r.previewWalk(req)
		return
	case !req.from.Member.DM && req.cmd.Kind != CmdWalk && req.cmd.Kind != CmdEndTurn && req.cmd.Kind != CmdSpend:
		r.reject(req, "Only the DM can change the table.")
		return
	}
	w, reason := r.plan(req.from.Member, req.cmd)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	r.commit(req, w, req.from.Member, req.from.Caller)
}

// commit applies a planned write to a copy of the state, writes it through, then broadcasts it.
func (r *runtime) commit(req request, w Write, actor domain.Member, c caller.Caller) {
	next := r.st.clone()
	apply(next, &w)
	seq, err := r.store.Commit(context.Background(), next.session, next.board, w, actor, c, r.now())
	if err != nil {
		r.log.Error("live: commit", "error", err)
		r.reject(req, "That change could not be saved.")
		return
	}
	next.session.Seq = seq
	r.st = next
	views := map[Audience]Update{}
	for sub := range r.subs {
		u, ok := views[sub.Audience]
		if !ok {
			u = r.viewUpdate(sub.Audience, seq, &w)
			views[sub.Audience] = u
		}
		if sub == req.from {
			u.Nonce = req.cmd.Nonce
		}
		r.send(sub, u)
	}
}

// viewUpdate projects a change for one audience, with a view per step when a token walked.
func (r *runtime) viewUpdate(a Audience, seq int64, w *Write) Update {
	v := r.st.project(a)
	u := Update{Kind: UpdView, Seq: seq, View: &v}
	if len(w.frames) > 1 {
		for _, f := range w.frames[:len(w.frames)-1] {
			u.Steps = append(u.Steps, f.project(a))
		}
	}
	return u
}

// apply changes a copy of the state and records the hexes the party now sees for the first time.
func apply(s *state, w *Write) {
	switch w.Kind {
	case domain.ActionTokenWalked:
		walk(s, w)
		return
	case domain.ActionCombatStarted, domain.ActionInitiativeRolled, domain.ActionTurnEnded, domain.ActionResourceSpent, domain.ActionCombatEnded:
		applyCombat(s, w)
		return
	case domain.ActionTokenRemoved:
		delete(s.tokens, w.Token.ID)
		dropCombatant(s, w)
	case domain.ActionTokenPlaced, domain.ActionTokenMoved, domain.ActionTokenHidden, domain.ActionTokenRevealed:
		s.tokens[w.Token.ID] = w.Token
	case domain.ActionMapSet:
		s.session.MapID = w.MapID
		s.setBoard(w.board)
	default:
		applyBoard(s.board, w)
	}
	s.reveal(w)
}

// walk moves a token hex by hex, keeping the state after each step, and spends Combat movement.
func walk(s *state, w *Write) {
	if s.combat != nil {
		for i, x := range s.combat.Combatants {
			if x.TokenID == w.Token.ID {
				s.combat.Combatants[i].Economy, _ = x.Economy.Move(w.CostFt)
				w.Combat = s.combat
			}
		}
	}
	for _, c := range w.Path[1:] {
		w.Token.Q, w.Token.R = c.Q, c.R
		s.tokens[w.Token.ID] = w.Token
		s.reveal(w)
		w.frames = append(w.frames, s.clone())
	}
}

// reveal remembers every hex the party sees now and records the ones it sees for the first time.
func (s *state) reveal(w *Write) {
	if s.board == nil {
		return
	}
	for c := range s.vision() {
		if !s.board.Reveals[c] {
			s.board.Reveals[c] = true
			w.AutoReveal = append(w.AutoReveal, c)
		}
	}
}

func applyBoard(b *domain.MapState, w *Write) {
	switch w.Kind {
	case domain.ActionHexesRevealed, domain.ActionWallsSet:
		set := map[bool]map[hex.Coord]bool{true: b.Reveals, false: b.Walls}[w.Kind == domain.ActionHexesRevealed]
		for _, c := range w.Hexes {
			set[c] = true
		}
	case domain.ActionHexesConcealed, domain.ActionWallsCleared:
		set := map[bool]map[hex.Coord]bool{true: b.Reveals, false: b.Walls}[w.Kind == domain.ActionHexesConcealed]
		for _, c := range w.Hexes {
			delete(set, c)
		}
	case domain.ActionLightPlaced:
		b.Lights = append(b.Lights, w.Light)
	case domain.ActionLightRemoved:
		kept := b.Lights[:0]
		for _, l := range b.Lights {
			if l.ID != w.Light.ID {
				kept = append(kept, l)
			}
		}
		b.Lights = kept
	default:
		b.Map.Ambient = w.Ambient
	}
}
