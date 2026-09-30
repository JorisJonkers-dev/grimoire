package live

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
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
	HP        *HPChange
	// Observers saw a ranged attack's damage; each remembers it against the attacker.
	Observers []domain.TokenID
	attack    *domain.PendingAttack
	board     *domain.MapState
	frames    []*state
	prompt    *domain.ReactionPrompt
	// Effects is the Session's Effects after the change, for the store to save; nil when unchanged.
	Effects *domain.Effects
	// Surfaces is the Session's Surfaces after the change when SaveSurfaces; Cast the area spell when SaveCast.
	Surfaces     map[hex.Coord]domain.Surface
	SaveSurfaces bool
	Cast         *domain.AreaCast
	SaveCast     bool
	// Table is the Table Display after a table_set.
	Table    *domain.TableDisplay
	tableMap *domain.Map
	// WorldMapID is the world map a world_set chooses; WorldMap the map every other world change touches.
	WorldMapID *domain.MapID
	WorldMap   domain.MapID
	Node       domain.WorldNode
	Route      domain.WorldRoute
	Leg        *domain.TravelLeg
	// WorldReveal lists the world hexes the party sees for the first time on arriving somewhere.
	WorldReveal []hex.Coord
	world       *domain.World
	// ElevationFt is the height set on Hexes by an elevation_set.
	ElevationFt int
	cast        *domain.AreaCast
	terrain     bool
	damageType  string
	created     effects.CreateSurface
	elevation   int
	effect      *domain.Effect
	ended       []domain.EffectID
	manuals     []domain.ManualPrompt
	newSaves    []domain.PendingSave
	resolved    uuid.UUID
	saved       domain.RollID
	resume      *domain.Resume
	answered    *domain.ReactionPrompt
	total       int
	resource    combat.Resource
}

// Store is the runtime's persistence port.
type Store interface {
	Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error)
	LoadMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (*domain.MapState, error)
	LoadCombat(ctx context.Context, id domain.SessionID) (*domain.Combat, error)
	LoadEffects(ctx context.Context, id domain.SessionID) (domain.Effects, error)
	LoadTerrain(ctx context.Context, id domain.SessionID) (map[hex.Coord]domain.Surface, *domain.AreaCast, error)
	LoadTable(ctx context.Context, id domain.SessionID) (domain.TableDisplay, error)
	// LoadWorld reads a world map with its locations, routes and the party, and the Session's Travel Legs on it.
	LoadWorld(ctx context.Context, campaign uuid.UUID, sid domain.SessionID, id domain.MapID) (*domain.World, error)
	// HighGround reports whether the Campaign uses the high-ground optional rule.
	HighGround(ctx context.Context, campaign uuid.UUID) (bool, error)
	// Observations is how much damage each creature has seen each other creature deal from range.
	Observations(ctx context.Context, id domain.SessionID) (map[domain.TokenID]map[domain.TokenID]int, error)
	Roll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (domain.Roll, error)
	// ReactionTimeout is how many seconds the Campaign's Reaction Prompts wait.
	ReactionTimeout(ctx context.Context, campaign uuid.UUID) (int, error)
	// LastDamage is the Session's latest damage not undone yet; its Undoes names that damage's Action.
	LastDamage(ctx context.Context, id domain.SessionID) (HPChange, bool, error)
	Commit(ctx context.Context, s domain.Session, board *domain.MapState, w Write, actor domain.Member, c caller.Caller, now time.Time) (int64, error)
}

// Statblocks copies fighting stats onto new tokens.
type Statblocks interface {
	Monster(ctx context.Context, campaign uuid.UUID, slug string) (string, domain.Stats, error)
	Character(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (string, uuid.UUID, domain.Stats, error)
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
	store Store
	// campaign never changes, so the hub may read it from other goroutines.
	campaign uuid.UUID
	armed    uuid.UUID
	members  Members
	stats    Statblocks
	now      func() time.Time
	log      *slog.Logger
	release  func()
	st       *state
	subs     map[*Subscriber]struct{}
	join     chan *Subscriber
	leave    chan *Subscriber
	cmds     chan request
	stop     chan struct{}
	done     chan struct{}
}

// Hub starts, finds and stops Session runtimes.
type Hub struct {
	Store   Store
	Members Members
	Stats   Statblocks
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
	seen, err := h.Store.Observations(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	fx, err := h.Store.LoadEffects(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	ground, cast, err := h.Store.LoadTerrain(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	table, tableMap, err := h.loadTable(ctx, s)
	if err != nil {
		release()
		return nil, err
	}
	world, err := h.loadWorld(ctx, s)
	if err != nil {
		release()
		return nil, err
	}
	st := &state{session: s, tokens: map[domain.TokenID]domain.Token{}, combat: fight, observed: seen, now: h.Now, fx: fx, surfaces: ground, cast: cast, table: table, tableMap: tableMap}
	for _, t := range tokens {
		st.tokens[t.ID] = t
	}
	st.setBoard(board)
	st.setWorld(world)
	rt := &runtime{
		store: h.Store, campaign: s.CampaignID, members: h.Members, stats: h.Stats, now: h.Now, log: h.Log, release: release, st: st, subs: map[*Subscriber]struct{}{},
		join: make(chan *Subscriber), leave: make(chan *Subscriber), cmds: make(chan request), stop: make(chan struct{}), done: make(chan struct{}),
	}
	if h.runtimes == nil {
		h.runtimes = map[domain.SessionID]*runtime{}
	}
	h.runtimes[id] = rt
	go rt.run()
	return rt, nil
}

// loadTable reads the Table Display and the world map it shows, if any.
func (h *Hub) loadTable(ctx context.Context, s domain.Session) (domain.TableDisplay, *domain.Map, error) {
	t, err := h.Store.LoadTable(ctx, s.ID)
	if err != nil || t.MapID == nil {
		return t, nil, err
	}
	board, err := h.Store.LoadMap(ctx, s.CampaignID, *t.MapID)
	if err != nil {
		return t, nil, err
	}
	return t, &board.Map, nil
}

// loadWorld reads the world map the Session travels, if any.
func (h *Hub) loadWorld(ctx context.Context, s domain.Session) (*domain.World, error) {
	if s.WorldMapID == nil {
		return nil, nil //nolint:nilnil // a Session without a world map is not an error
	}
	return h.Store.LoadWorld(ctx, s.CampaignID, s.ID, *s.WorldMapID)
}

// RollResolved tells the Campaign's running Sessions that a Roll Request resolved, so an initiative
// rolled on a Roll Card reaches its Combat.
func (h *Hub) RollResolved(campaign uuid.UUID, id domain.RollID) {
	h.mu.Lock()
	var targets []*runtime
	for _, rt := range h.runtimes {
		if rt.campaign == campaign {
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
	r.arm()
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
	case req.from == nil && req.cmd.Kind == cmdPromptTimeout:
		r.timedOut(req.cmd.promptID)
		return
	case req.from == nil:
		r.rolled(req)
		return
	case req.cmd.Kind == CmdResync:
		r.send(req.from, r.snapshot(req.from.Audience))
		return
	case req.cmd.Kind == CmdPlanWalk:
		r.previewWalk(req)
		return
	case req.cmd.Kind == CmdPreviewAttack:
		r.previewAttack(req)
		return
	case req.cmd.Kind == CmdPreviewArea:
		r.previewArea(req)
		return
	case req.cmd.Kind == CmdPing && req.from.Member.DM:
		r.ping(req)
		return
	case !req.from.Member.DM && !playerMay(req.cmd.Kind):
		r.reject(req, "Only the DM can change the table.")
		return
	}
	w, reason := r.plan(req)
	if reason != "" {
		r.reject(req, reason)
		return
	}
	r.commit(req, w, req.from.Member, req.from.Caller)
}

// playerMay lists the changes a Player may ask for; each is checked against what they control.
func playerMay(kind string) bool {
	switch kind {
	case CmdWalk, CmdEndTurn, CmdSpend, CmdAttack, CmdReact, CmdCastArea:
		return true
	}
	return false
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
	r.arm()
	r.follow(w, actor, c)
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

// apply changes a copy of the state, then its Effects: those the write adds or ends, those that count
// down as turns start, concentration broken by damage, and those of a removed token.
func apply(s *state, w *Write) {
	before := s.acting()
	round := 0
	if s.combat != nil {
		round = s.combat.Round
	}
	change(s, w)
	changed := w.effect != nil || len(w.ended)+len(w.manuals)+len(w.newSaves) > 0 || w.resolved != uuid.Nil || w.saved != domain.RollID{}
	applyEffects(s, w)
	started := map[domain.TokenID]bool{}
	for id := range s.acting() {
		started[id] = !before[id]
	}
	changed = s.tick(started) || changed
	changed = s.hazards(started, w) || changed
	settleTerrain(s, w, round)
	if w.Kind == domain.ActionDamageDealt {
		changed = s.concentrate(*w.HP) || changed
	}
	if w.Kind == domain.ActionTokenRemoved {
		s.forget(w.Token.ID)
		changed = true
	}
	if changed {
		fx := cloneEffects(s.fx)
		w.Effects = &fx
	}
}

// settleTerrain ages Surfaces as a round starts, drops a removed token from the waiting area spell, and
// marks what the store must write.
func settleTerrain(s *state, w *Write, round int) {
	if s.combat != nil && s.combat.Round > round && round > 0 {
		w.terrain = s.weather() || w.terrain
	}
	if w.Kind == domain.ActionTokenRemoved && s.cast != nil {
		s.cast.Targets = slices.DeleteFunc(s.cast.Targets, func(t domain.AreaTarget) bool { return t.Token == w.Token.ID })
		if s.cast.Caster == w.Token.ID {
			s.cast = nil
		}
	}
	switch w.Kind {
	case domain.ActionAreaCast, domain.ActionAreaResolved, domain.ActionTokenRemoved:
		w.Cast, w.SaveCast = s.cast, true
	}
	if w.terrain {
		w.Surfaces, w.SaveSurfaces = maps.Clone(s.surfaces), true
	}
}

// change applies the write itself and records the hexes the party now sees for the first time.
func change(s *state, w *Write) {
	switch w.Kind {
	case domain.ActionTokenWalked:
		walk(s, w)
		return
	case domain.ActionCombatStarted, domain.ActionInitiativeRolled, domain.ActionTurnEnded, domain.ActionResourceSpent, domain.ActionCombatEnded:
		applyCombat(s, w)
		return
	case domain.ActionAttackDeclared, domain.ActionAttackHit, domain.ActionAttackMissed, domain.ActionDamageDealt, domain.ActionDamageUndone:
		applyAttack(s, w)
		return
	case domain.ActionTacticsSet:
		s.tokens[w.Token.ID] = w.Token
		return
	case domain.ActionReactionOffered, domain.ActionReactionUsed, domain.ActionReactionDeclined:
		applyReaction(s, w)
		return
	case domain.ActionEffectApplied, domain.ActionEffectEnded, domain.ActionSavePassed, domain.ActionSaveFailed, domain.ActionManualResolved:
		return
	case domain.ActionAreaCast, domain.ActionAreaResolved, domain.ActionSurfacesSet, domain.ActionElevationSet:
		applyTerrain(s, w)
		return
	case domain.ActionTableSet:
		s.table, s.tableMap = *w.Table, w.tableMap
		return
	case domain.ActionWorldSet, domain.ActionNodeAdded, domain.ActionNodeRemoved, domain.ActionRouteAdded, domain.ActionRouteRemoved,
		domain.ActionPartyPlaced, domain.ActionTravelLeg:
		applyWorld(s, w)
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
			}
		}
		s.combat.Prompt, s.combat.Resume, w.Combat = w.prompt, w.resume, s.combat
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
