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
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/conditionbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/effects"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
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
	// Resources are Character Resources an Effect spent or gave back as it landed; Dismissed are summoned
	// tokens that left because the Effect keeping them ended.
	Resources []ResourceDelta
	Dismissed []domain.TokenID
	// Objects are the Map Objects a change touched, for the store to save; Object the one it removed.
	Objects []domain.ObjectID
	Object  domain.ObjectID
	// Sneak is the party's sneaking after a change when SaveSneak, nil once it stops; Explore the same
	// for exploration turns.
	Sneak       *domain.Sneak
	SaveSneak   bool
	Explore     *domain.Exploration
	SaveExplore bool
	// Unveiled marks a write whose token lost Visibility Qualities to a Reveal.
	Unveiled bool
	// Formed is a token as it takes a form; Reverted are tokens whose form ended.
	Formed   *domain.Token
	Reverted []domain.TokenID
	// Observers saw a ranged attack's damage; each remembers it against the attacker.
	Observers []domain.TokenID
	attack    *domain.PendingAttack
	summons   []domain.Combatant
	commanded domain.CombatantID
	// changedObjects are Map Objects as a change leaves them; trigger an Effect one sets off.
	changedObjects map[domain.ObjectID]domain.MapObject
	trigger        *trigger
	// grounded are Effects Surfaces put on creatures that entered them or started a turn in them;
	// forced marks a change that may drop creatures from a height.
	grounded []grounding
	forced   bool
	sneak    *domain.Sneak
	explore  *domain.Exploration
	board    *domain.MapState
	frames   []*state
	prompt   *domain.ReactionPrompt
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
	// Zone is the Encounter Zone after the change; Revealed the hidden creatures it showed everyone.
	Zone     *domain.Zone
	Revealed []domain.Token
	// Rest is the rest a rest_taken took; Check the Encounter Check a write ran or resolved; Schedule a
	// check the DM asked for later, and Unschedule the scheduled check this one used up.
	Rest       string
	Check      *prep.Check
	Schedule   *prep.Scheduled
	Unschedule uuid.UUID
	// Drop is a new drop of loot; Move a transfer between Containers; Gone the drop it emptied.
	Drop *domain.Container
	Move *domain.Move
	// Moves are the transfers a settled loot pile makes, in order; Claim a call on one of its items,
	// taken back when Unclaim.
	Moves   []domain.Move
	Claim   *domain.Claim
	Unclaim bool
	// Swap is a Character changing weapon sets.
	Swap      *WeaponSwap
	Gone      *domain.ContainerID
	items     map[string]domain.ItemInfo
	lootTable string
	// Shop opens a Shop; Trade buys or sells there; Restock is a Shop with fresh Stock; Day the in-game day after the change.
	Shop    *domain.OpenShop
	Trade   *domain.Trade
	Restock *prep.Shop
	Day     *int
	haggle  *haggleChange
	// Resting is the rest a write leaves under way; RestOver ends it. Supplies are the Rations a Long
	// Rest ate; Results what a finished rest leaves each Character with; Healed the hit points it gave back.
	Resting  *domain.Rest
	RestOver bool
	Supplies []domain.Supply
	// Recharged are the charges a rest gave back to magic items.
	Recharged []domain.Recharge
	Results   []domain.RestResult
	Healed    []HPChange
	// Pending is a Hide, Grapple or Shove waiting on its roll; Settled the roll of one that resolved.
	// Pushed is a shoved creature where it lands; Dragged a grappled creature pulled along a walk.
	Pending *domain.PendingAction
	Settled domain.RollID
	Pushed  *domain.Token
	Dragged *domain.Token
	taken   *takenAction
	cleave  *domain.TokenID
	// Dying is a Character's death saves after the change; Undying the one that woke or was revived.
	Dying   *domain.Dying
	Undying *domain.TokenID
	// Spawned are the creatures an encounter_spawned places; Undoes is the Action an undo reverts.
	Spawned []domain.Token
	Undoes  uuid.UUID
	price   *prep.ItemPrice
	// Trades are the sales and purchases of a trade, in order; prices what the Shop learnt sold items are worth.
	Trades []domain.Trade
	prices map[string]*prep.ItemPrice
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

// loaded is what the rules keep between a Session's runtimes besides tokens, Combat and Effects.
type loaded struct {
	catalog  effects.Catalog
	surfaces surface.Catalog
	looks    map[string]look
	rest     *domain.Rest
	pending  []domain.PendingAction
	dying    map[domain.TokenID]domain.Dying
	sneak    *domain.Sneak
	explore  *domain.Exploration
}

// loadRules reads the Effect catalogue, the rest the Session has under way, the Hides, Grapples and
// Shoves waiting on rolls, and the Characters at 0 hit points.
func (h *Hub) loadRules(ctx context.Context, s domain.Session) (loaded, error) {
	var out loaded
	var err error
	if out.catalog, err = h.Store.Effects(ctx); err != nil {
		return out, err
	}
	if out.surfaces, err = h.Store.Surfaces(ctx); err != nil {
		return out, err
	}
	brew, err := h.Store.Homebrew(ctx, s.CampaignID)
	if err != nil {
		return out, err
	}
	out.catalog, out.surfaces = withHomebrew(out.catalog, out.surfaces, brew)
	out.looks = looksOf(brew.Conditions)
	if out.rest, err = h.Store.LoadRest(ctx, s.CampaignID, s.ID); err != nil {
		return out, err
	}
	if out.pending, err = h.Store.LoadPendingActions(ctx, s.ID); err != nil {
		return out, err
	}
	if out.dying, err = h.Store.LoadDying(ctx, s.ID); err != nil {
		return out, err
	}
	if out.sneak, err = h.Store.LoadSneak(ctx, s.ID); err != nil {
		return out, err
	}
	out.explore, err = h.Store.LoadExploration(ctx, s.ID)
	return out, err
}

// Brew is what a Campaign adds to the rules: its homebrew spells and conditions, and the exhaustion it
// plays with.
type Brew struct {
	Spells     []spellbuild.Built
	Conditions []conditionbuild.Condition
	Exhaustion effects.Exhausting
}

// withHomebrew adds a Campaign's homebrew spells, the Surfaces their start-of-turn damage lies on, and
// its homebrew conditions to copies of the shared catalogues, and makes exhaustion its variant.
func withHomebrew(cat effects.Catalog, ground surface.Catalog, brew Brew) (effects.Catalog, surface.Catalog) {
	cat, ground = maps.Clone(cat), maps.Clone(ground)
	for _, b := range brew.Spells {
		cat[b.Definition.Slug] = b.Definition
		if g := b.Surface; g != nil {
			ground[surface.Kind(g.Slug)] = surface.Definition{Kind: surface.Kind(g.Slug), Name: g.Name, Cost: 1, HazardDice: g.Dice, HazardType: g.Type}
		}
	}
	for _, c := range brew.Conditions {
		cat[c.Definition.Slug] = c.Definition
	}
	if ex, ok := cat["exhaustion"]; ok {
		parts := slices.Clone(ex.Components)
		for i, c := range parts {
			if _, stacks := c.(effects.Exhausting); stacks {
				parts[i] = brew.Exhaustion
			}
		}
		ex.Components = parts
		cat["exhaustion"] = ex
	}
	return cat, ground
}

// look is how a homebrew condition shows on a token.
type look struct {
	name, icon, color string
}

func looksOf(conditions []conditionbuild.Condition) map[string]look {
	out := make(map[string]look, len(conditions))
	for _, c := range conditions {
		out[c.Definition.Slug] = look{name: c.Definition.Name, icon: c.Icon, color: c.Color}
	}
	return out
}

// Store is the runtime's persistence port.
type Store interface {
	// Homebrew builds the homebrew spells and conditions a Campaign sees, and its exhaustion.
	Homebrew(ctx context.Context, campaign uuid.UUID) (Brew, error)
	Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error)
	LoadMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (*domain.MapState, error)
	LoadCombat(ctx context.Context, id domain.SessionID) (*domain.Combat, error)
	LoadEffects(ctx context.Context, id domain.SessionID) (domain.Effects, error)
	// LoadRest reads the rest a Session has under way, with each rester's Hit Die roll still out; nil when none.
	LoadRest(ctx context.Context, campaign uuid.UUID, id domain.SessionID) (*domain.Rest, error)
	// LoadPendingActions reads the Hides, Grapples and Shoves waiting on rolls.
	LoadPendingActions(ctx context.Context, id domain.SessionID) ([]domain.PendingAction, error)
	// LoadDying reads the Characters at 0 hit points.
	LoadDying(ctx context.Context, id domain.SessionID) (map[domain.TokenID]domain.Dying, error)
	// RestInfo reads what a rest needs of each Character; RestSupplies whether a Long Rest costs Rations.
	RestInfo(ctx context.Context, campaign uuid.UUID, characters []uuid.UUID) ([]domain.Rester, error)
	RestSupplies(ctx context.Context, campaign uuid.UUID) (bool, error)
	// Features reads what classes, species and feats grant.
	Features(ctx context.Context) (features.Catalog, error)
	// Effects reads the Effect catalogue the rules resolve against.
	Effects(ctx context.Context) (effects.Catalog, error)
	Surfaces(ctx context.Context) (surface.Catalog, error)
	LoadSneak(ctx context.Context, id domain.SessionID) (*domain.Sneak, error)
	CampaignInitiative(ctx context.Context, campaign uuid.UUID) (string, bool, error)
	LoadExploration(ctx context.Context, id domain.SessionID) (*domain.Exploration, error)
	LoadTerrain(ctx context.Context, id domain.SessionID) (map[hex.Coord]domain.Surface, *domain.AreaCast, error)
	LoadTable(ctx context.Context, id domain.SessionID) (domain.TableDisplay, error)
	// LoadWorld reads a world map with its locations, routes and the party, and the Session's Travel Legs on it.
	LoadWorld(ctx context.Context, campaign uuid.UUID, sid domain.SessionID, id domain.MapID) (*domain.World, error)
	LoadZones(ctx context.Context, id domain.SessionID) ([]domain.Zone, error)
	// LoadPrep reads the Campaign's Encounter Tables, Pools, creature XP, party levels and scheduled checks.
	LoadPrep(ctx context.Context, campaign uuid.UUID) (prep.Prep, error)
	LoadChecks(ctx context.Context, campaign uuid.UUID, sid domain.SessionID) ([]prep.Check, error)
	// LoadInventory reads every Container of the Campaign, making the Party Stash and each Character's Inventory first.
	LoadInventory(ctx context.Context, campaign uuid.UUID) (domain.Inventory, error)
	LoadLoot(ctx context.Context, campaign uuid.UUID) ([]prep.LootTable, error)
	Items(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]domain.ItemInfo, error)
	// LoadShop reads a Shop to open; LoadOpenShop the one a Session has open, if any.
	LoadShop(ctx context.Context, campaign uuid.UUID, id prep.ShopID) (*domain.OpenShop, error)
	LoadOpenShop(ctx context.Context, campaign uuid.UUID, sid domain.SessionID) (*domain.OpenShop, error)
	LoadShops(ctx context.Context, campaign uuid.UUID) ([]prep.Shop, error)
	// Stock rolls fresh Stock for a Shop.
	Stock(ctx context.Context, campaign uuid.UUID, shop prep.Shop, src dice.Source) ([]prep.StockItem, error)
	ItemPrices(ctx context.Context, campaign uuid.UUID, slugs []string) (map[string]prep.ItemPrice, error)
	// TradeBonus is a Character's Persuasion bonus.
	TradeBonus(ctx context.Context, campaign, character uuid.UUID) (int, error)
	GameDay(ctx context.Context, campaign uuid.UUID) (int, error)
	// HighGround reports whether the Campaign uses the high-ground optional rule.
	HighGround(ctx context.Context, campaign uuid.UUID) (bool, error)
	// Observations is how much damage each creature has seen each other creature deal from range.
	Observations(ctx context.Context, id domain.SessionID) (map[domain.TokenID]map[domain.TokenID]int, error)
	Roll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (domain.Roll, error)
	// ReactionTimeout is how many seconds the Campaign's Reaction Prompts wait.
	ReactionTimeout(ctx context.Context, campaign uuid.UUID) (int, error)
	// LastDamage is the Session's latest damage not undone yet; its Undoes names that damage's Action.
	LastDamage(ctx context.Context, id domain.SessionID) (HPChange, bool, error)
	Commit(ctx context.Context, s domain.Session, board *domain.MapState, w Write, actor domain.Member, c caller.Caller, now time.Time) (Committed, error)
	// Action reads one Action of the Session by its Action Log sequence.
	Action(ctx context.Context, id domain.SessionID, seq int64) (ActionRecord, error)
}

// Committed is where a write landed: the Session's sequence and the Action Log's.
type Committed struct {
	Seq    int64
	Action int64
}

// Statblocks copies fighting stats onto new tokens.
type Statblocks interface {
	Monster(ctx context.Context, campaign uuid.UUID, slug string) (string, domain.Stats, error)
	Character(ctx context.Context, c caller.Caller, campaign, id uuid.UUID) (string, uuid.UUID, domain.Stats, error)
	// Holding is a Character's stats with these weapons and shield in hand.
	Holding(ctx context.Context, c caller.Caller, campaign, id uuid.UUID, weapons []string, shield bool) (domain.Stats, error)
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
	seed     func() uint64
	source   func(seed uint64) dice.Source
	notify   Notifier
	// dm is the DM last seen on this Session; an ambush opens its creatures' rolls for them.
	dm    *domain.Member
	subs  map[*Subscriber]struct{}
	join  chan *Subscriber
	leave chan *Subscriber
	cmds  chan request
	stop  chan struct{}
	done  chan struct{}
}

// Hub starts, finds and stops Session runtimes.
type Hub struct {
	Store   Store
	Members Members
	Stats   Statblocks
	Owner   Owner
	Now     func() time.Time
	Log     *slog.Logger
	// Seed and Source drive Encounter Checks: each check keeps its seed so its draw can be replayed.
	Seed   func() uint64
	Source func(seed uint64) dice.Source
	// Notify reaches players' devices when their turn starts or a Reaction Prompt waits; nil leaves them be.
	Notify Notifier

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
	kept, err := h.loadRules(ctx, s)
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
	zones, err := h.Store.LoadZones(ctx, id)
	if err != nil {
		release()
		return nil, err
	}
	checks, err := h.Store.LoadChecks(ctx, s.CampaignID, id)
	if err != nil {
		release()
		return nil, err
	}
	trade, err := h.loadTrade(ctx, s)
	if err != nil {
		release()
		return nil, err
	}
	st := &state{
		session: s, tokens: map[domain.TokenID]domain.Token{}, combat: fight, observed: seen, now: h.Now, fx: fx, catalog: kept.catalog, looks: kept.looks, terrainKinds: kept.surfaces, sneak: kept.sneak, explore: kept.explore, rest: kept.rest, pending: kept.pending, dying: kept.dying, surfaces: ground, cast: cast, table: table,
		tableMap: tableMap, zones: zones, checks: checks, inventory: trade.inventory, shop: trade.shop, day: trade.day,
	}
	for _, t := range tokens {
		st.tokens[t.ID] = t
	}
	st.setBoard(board)
	st.setWorld(world)
	rt := &runtime{
		store: h.Store, campaign: s.CampaignID, seed: h.Seed, source: h.Source, members: h.Members, stats: h.Stats, notify: h.Notify, now: h.Now, log: h.Log, release: release, st: st, subs: map[*Subscriber]struct{}{},
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

type trading struct {
	inventory domain.Inventory
	shop      *domain.OpenShop
	day       int
}

// loadTrade reads the Campaign's Containers, the Shop the Session has open and the in-game day.
func (h *Hub) loadTrade(ctx context.Context, s domain.Session) (trading, error) {
	var out trading
	var err error
	if out.inventory, err = h.Store.LoadInventory(ctx, s.CampaignID); err != nil {
		return out, err
	}
	if out.shop, err = h.Store.LoadOpenShop(ctx, s.CampaignID, s.ID); err != nil {
		return out, err
	}
	if err = priceCarried(ctx, h.Store, s.CampaignID, out.inventory, out.shop); err != nil {
		return out, err
	}
	out.day, err = h.Store.GameDay(ctx, s.CampaignID)
	return out, err
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
			r.seeDM(sub)
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

// seeDM remembers a DM who joined; a zone that settled while no DM was here starts its fight now.
func (r *runtime) seeDM(sub *Subscriber) {
	if !sub.Member.DM {
		return
	}
	first := r.dm == nil
	m := sub.Member
	r.dm = &m
	if first && r.st.combat == nil {
		r.ambush(sub.Caller)
	}
}

// send never blocks the runtime: a connection that cannot keep up is dropped and will resync.
func (r *runtime) send(sub *Subscriber, u Update) {
	if _, ok := r.subs[sub]; !ok {
		return
	}
	if sub.Audience == AudienceParty {
		u = private(u, sub.Member)
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
	case req.cmd.Kind == CmdUndo:
		r.undo(req)
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
	case CmdWalk, CmdEndTurn, CmdSpend, CmdAttack, CmdReact, CmdCastArea, CmdMoveItem, CmdMoveCoins, CmdClaimLoot, CmdBuy, CmdSell, CmdHaggle, CmdTrade,
		CmdProposeRest, CmdAgreeRest, CmdSpendHitDie, CmdTakeAction, CmdUnarmed, CmdInteract, CmdSwapWeapons, CmdSetReaction, CmdStabilise, CmdRevive, CmdTeleport, CmdSummon, CmdCommand, CmdUseObject, CmdUnlock, CmdDisarm, CmdJump, CmdThrow, CmdSneak, CmdPassTurn:
		return true
	}
	return false
}

// commit applies a planned write to a copy of the state, writes it through, then broadcasts it.
func (r *runtime) commit(req request, w Write, actor domain.Member, c caller.Caller) {
	next := r.st.clone()
	apply(next, &w)
	done, err := r.store.Commit(context.Background(), next.session, next.board, w, actor, c, r.now())
	if err != nil {
		r.log.Error("live: commit", "error", err)
		r.reject(req, "That change could not be saved.")
		return
	}
	seq := done.Seq
	next.session.Seq = seq
	prev := r.st
	r.st = next
	r.nudge(prev, next)
	views := map[Audience]Update{}
	for sub := range r.subs {
		u, ok := views[sub.Audience]
		if !ok {
			u = r.viewUpdate(sub.Audience, seq, &w)
			views[sub.Audience] = u
		}
		if sub == req.from {
			u.Nonce, u.ActionSeq = req.cmd.Nonce, done.Action
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
	var heights map[domain.TokenID]int
	if w.forced || w.Pushed != nil {
		heights = s.heights()
	}
	round := 0
	if s.combat != nil {
		round = s.combat.Round
	}
	change(s, w)
	if w.Day != nil {
		s.day = *w.Day
	}
	applyZones(s, w)
	if w.Kind == domain.ActionRestTaken {
		w.ended = append(w.ended, s.restEnded()...)
	}
	fell := heights != nil && s.falls(w, heights)
	s.formBroken(w)
	changed := w.effect != nil || len(w.ended)+len(w.manuals)+len(w.newSaves) > 0 || w.resolved != uuid.Nil || w.saved != domain.RollID{}
	applyEffects(s, w)
	started := map[domain.TokenID]bool{}
	for id := range s.acting() {
		started[id] = !before[id]
	}
	changed = s.tick(started) || changed
	changed = s.hazards(started, w) || changed
	settleTerrain(s, w, round)
	changed = s.aftermath(w) || changed || fell
	if changed {
		fx := cloneEffects(s.fx)
		w.Effects = &fx
	}
}

// aftermath settles what a change leaves behind: concentration broken by damage, the Effects of a
// removed token, summons whose Effect ended and forms that reverted. It reports whether the Effects
// changed.
func (s *state) aftermath(w *Write) bool {
	changed := false
	if w.Kind == domain.ActionDamageDealt {
		changed = s.concentrate(*w.HP)
	}
	if w.Kind == domain.ActionTokenRemoved {
		s.forget(w.Token.ID)
		s.forgetChecks(w.Token.ID)
		changed = true
	}
	changed = s.dismiss(w) || changed
	return s.revert(w) || changed
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
	case domain.ActionAreaCast, domain.ActionAreaResolved, domain.ActionTokenRemoved, domain.ActionCountered:
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
	case domain.ActionTaken, domain.ActionConcentrationChecked:
		applyAction(s, w)
		applyDying(s, w)
		return
	case domain.ActionDowned, domain.ActionDyingChanged, domain.ActionRevived:
		applyDying(s, w)
		return
	case domain.ActionObjectUsed:
		applyInteraction(s, w)
		return
	case domain.ActionTeleported:
		applyTeleport(s, w)
	case domain.ActionSummoned, domain.ActionCommanded:
		applySummon(s, w)
		return
	case domain.ActionJumped:
		applyJump(s, w)
	case domain.ActionSneakStarted, domain.ActionSneakEnded, domain.ActionStealthRolled, domain.ActionPartyNoticed:
		s.sneak, w.Sneak, w.SaveSneak = w.sneak, w.sneak, true
		return
	case domain.ActionExplorationStarted, domain.ActionExplorationTurn, domain.ActionExplorationEnded:
		s.explore, w.Explore, w.SaveExplore = w.explore, w.explore, true
		return
	case domain.ActionThrown:
		applyThrow(s, w)
		return
	case domain.ActionObjectPlaced, domain.ActionObjectRemoved, domain.ActionObjectToggled, domain.ActionObjectDamaged, domain.ActionObjectFound,
		domain.ActionObjectUnlocked, domain.ActionTrapDisarmed, domain.ActionTrapSprung:
		applyObject(s, w)
	case domain.ActionMasteryUsed:
		applyMastery(s, w)
		return
	case domain.ActionReactionSet:
		s.tokens[w.Token.ID] = w.Token
		return
	case domain.ActionUnarmed:
		applyAction(s, w)
		applyResolved(s, w)
		return
	case domain.ActionResolved:
		applyResolved(s, w)
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
	case domain.ActionReactionOffered, domain.ActionReactionUsed, domain.ActionReactionDeclined, domain.ActionCountered:
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
	case domain.ActionZoneAdded, domain.ActionZoneRemoved, domain.ActionZoneHeld, domain.ActionZoneSprung, domain.ActionPerceptionRolled,
		domain.ActionCheckScheduled:
		return
	case domain.ActionRestTaken, domain.ActionRestProposed, domain.ActionRestAgreed, domain.ActionRestStarted, domain.ActionHitDieSpent,
		domain.ActionHitDieHealed, domain.ActionRestInterrupted:
		applyRest(s, w)
		return
	case domain.ActionEncounterChecked, domain.ActionEncounterResolved:
		applyCheck(s, *w.Check)
		return
	case domain.ActionLootDropped, domain.ActionItemMoved, domain.ActionCoinsMoved:
		applyInventory(s, w)
		return
	case domain.ActionLootClaimed, domain.ActionLootSettled:
		applyClaims(s, w)
		return
	case domain.ActionShopOpened, domain.ActionShopClosed, domain.ActionItemBought, domain.ActionItemSold, domain.ActionHaggleStarted,
		domain.ActionHaggled, domain.ActionStockRolled, domain.ActionTradeMade:
		applyShop(s, w)
		return
	case domain.ActionWorldSet, domain.ActionNodeAdded, domain.ActionNodeRemoved, domain.ActionRouteAdded, domain.ActionRouteRemoved,
		domain.ActionPartyPlaced, domain.ActionTravelLeg:
		applyWorld(s, w)
		return
	case domain.ActionHPAdjusted:
		s.setHP(*w.HP)
		return
	case domain.ActionEncounterSpawned:
		for _, t := range w.Spawned {
			s.tokens[t.ID] = t
		}
	case domain.ActionTokenRemoved:
		delete(s.tokens, w.Token.ID)
		dropCombatant(s, w)
	case domain.ActionTokenPlaced, domain.ActionTokenMoved, domain.ActionTokenHidden, domain.ActionTokenRevealed, domain.ActionVisibilitySet:
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
	if e := s.explore; e != nil && s.combat == nil && e.Order[e.Turn] == w.Token.ID {
		e.MovedFt += w.CostFt
		w.Explore, w.SaveExplore = e, true
	}
	for _, c := range w.Path[1:] {
		w.Token.Q, w.Token.R = c.Q, c.R
		s.tokens[w.Token.ID] = w.Token
		s.reveal(w)
		w.frames = append(w.frames, s.clone())
	}
	if w.Dragged != nil {
		s.tokens[w.Dragged.ID] = *w.Dragged
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
