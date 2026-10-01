// Package live runs Sessions: one goroutine per Session owns its state, writes every change through
// to Postgres, and sends each audience only what it may see.
package live

import (
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// Audience is who a connection sees the Session as.
type Audience string

// Audiences.
const (
	AudienceDM    Audience = "dm"
	AudienceParty Audience = "party"
	AudienceTable Audience = "table"
)

// Command kinds.
const (
	CmdResync         = "resync"
	CmdPlace          = "place_token"
	CmdMove           = "move_token"
	CmdSetHidden      = "set_token_hidden"
	CmdRemove         = "remove_token"
	CmdSetMap         = "set_map"
	CmdRevealHexes    = "reveal_hexes"
	CmdSetWalls       = "set_walls"
	CmdPlaceLight     = "place_light"
	CmdRemoveLight    = "remove_light"
	CmdSetAmbient     = "set_ambient"
	CmdPlanWalk       = "plan_walk"
	CmdWalk           = "walk"
	CmdStartCombat    = "start_combat"
	CmdEndTurn        = "end_turn"
	CmdSpend          = "spend"
	CmdEndCombat      = "end_combat"
	CmdPreviewAttack  = "preview_attack"
	CmdAttack         = "attack"
	CmdUndoDamage     = "undo_damage"
	CmdSpawnEncounter = "spawn_encounter"
	CmdAdjustHP       = "adjust_hp"
	CmdUndo           = "undo"
	CmdSetTactics     = "set_tactics"
	CmdReact          = "react"
	CmdApplyEffect    = "apply_effect"
	CmdEndEffect      = "end_effect"
	CmdResolveManual  = "resolve_manual"
	CmdPreviewArea    = "preview_area"
	CmdCastArea       = "cast_area"
	CmdPaintSurface   = "paint_surface"
	CmdSetElevation   = "set_elevation"
	CmdTableCamera    = "table_camera"
	CmdTableScene     = "table_scene"
	CmdTableBlackout  = "table_blackout"
	CmdPing           = "ping"
	CmdSetWorld       = "set_world"
	CmdAddNode        = "add_node"
	CmdAddRoute       = "add_route"
	CmdRemoveNode     = "remove_node"
	CmdRemoveRoute    = "remove_route"
	CmdPlaceParty     = "place_party"
	CmdTravel         = "travel"
	CmdAddZone        = "add_zone"
	CmdRemoveZone     = "remove_zone"
	CmdHoldZone       = "hold_zone"
	CmdSpringZone     = "spring_zone"
	CmdRest           = "rest"
	CmdEncounterCheck = "encounter_check"
	CmdScheduleCheck  = "schedule_check"
	CmdRollLoot       = "roll_loot"
	CmdMoveItem       = "move_item"
	CmdMoveCoins      = "move_coins"
	CmdOpenShop       = "open_shop"
	CmdCloseShop      = "close_shop"
	CmdBuy            = "buy"
	CmdSell           = "sell"
	CmdHaggle         = "haggle"
	CmdProposeRest    = "propose_rest"
	CmdAgreeRest      = "agree_rest"
	CmdSpendHitDie    = "spend_hit_die"
	CmdFinishRest     = "finish_rest"
	CmdInterruptRest  = "interrupt_rest"
	CmdTakeAction     = "take_action"
	CmdUnarmed        = "unarmed"
	CmdInteract       = "interact"
	CmdTeleport       = "teleport"
	CmdSetReaction    = "set_reaction"
	CmdStabilise      = "stabilise"
	CmdRevive         = "revive"
	// cmdPromptTimeout declines a Reaction Prompt nobody answered in time.
	cmdPromptTimeout = "prompt_timeout"
	// cmdRollResolved comes from the rolls service, never from a client.
	cmdRollResolved = "roll_resolved"
)

// Economy resources a spend command names.
const (
	ResourceAction      = "action"
	ResourceBonusAction = "bonus_action"
	ResourceReaction    = "reaction"
)

// CombatantSetup is one Token joining a Combat.
type CombatantSetup struct {
	TokenID         string `json:"tokenId"`
	InitiativeBonus int    `json:"initiativeBonus"`
	SpeedFt         int    `json:"speedFt"`
}

// Hex is an axial coordinate on the wire.
type Hex struct {
	Q int `json:"q"`
	R int `json:"r"`
}

// Command is what a client asks for.
type Command struct {
	Nonce        string           `json:"nonce"`
	Kind         string           `json:"kind"`
	TokenID      string           `json:"tokenId,omitempty"`
	Label        string           `json:"label,omitempty"`
	TokenKind    string           `json:"tokenKind,omitempty"`
	Q            int              `json:"q"`
	R            int              `json:"r"`
	Hidden       bool             `json:"hidden"`
	DarkvisionFt int              `json:"darkvisionFt,omitempty"`
	MapID        string           `json:"mapId,omitempty"`
	Hexes        []Hex            `json:"hexes,omitempty"`
	On           bool             `json:"on,omitempty"`
	LightID      string           `json:"lightId,omitempty"`
	BrightFt     int              `json:"brightFt,omitempty"`
	DimFt        int              `json:"dimFt,omitempty"`
	Ambient      string           `json:"ambient,omitempty"`
	ControllerID string           `json:"controllerId,omitempty"`
	Combatants   []CombatantSetup `json:"combatants,omitempty"`
	CombatantID  string           `json:"combatantId,omitempty"`
	Resource     string           `json:"resource,omitempty"`
	MonsterSlug  string           `json:"monsterSlug,omitempty"`
	CharacterID  string           `json:"characterId,omitempty"`
	TargetID     string           `json:"targetId,omitempty"`
	AttackNo     int              `json:"attackNo,omitempty"`
	Tactics      string           `json:"tactics,omitempty"`
	Use          bool             `json:"use,omitempty"`
	Shield       bool             `json:"shield,omitempty"`
	Effect       string           `json:"effect,omitempty"`
	EffectName   string           `json:"effectName,omitempty"`
	SourceID     string           `json:"sourceId,omitempty"`
	Rounds       int              `json:"rounds,omitempty"`
	SaveAbility  string           `json:"saveAbility,omitempty"`
	SaveDC       int              `json:"saveDc,omitempty"`
	EffectID     string           `json:"effectId,omitempty"`
	ManualID     string           `json:"manualId,omitempty"`
	Surface      string           `json:"surface,omitempty"`
	ElevationFt  int              `json:"elevationFt,omitempty"`
	Camera       string           `json:"camera,omitempty"`
	ZoomPct      int              `json:"zoomPct,omitempty"`
	Scene        string           `json:"scene,omitempty"`
	Title        string           `json:"title,omitempty"`
	Body         string           `json:"body,omitempty"`
	NodeID       string           `json:"nodeId,omitempty"`
	ToNodeID     string           `json:"toNodeId,omitempty"`
	RouteID      string           `json:"routeId,omitempty"`
	DistanceMi   int              `json:"distanceMi,omitempty"`
	Pace         string           `json:"pace,omitempty"`
	ZoneID       string           `json:"zoneId,omitempty"`
	RadiusHexes  int              `json:"radiusHexes,omitempty"`
	DMOnly       bool             `json:"dmOnly,omitempty"`
	Rest         string           `json:"rest,omitempty"`
	TableID      string           `json:"tableId,omitempty"`
	Mode         string           `json:"mode,omitempty"`
	Entry        int              `json:"entry,omitempty"`
	Due          string           `json:"due,omitempty"`
	LootTableID  string           `json:"lootTableId,omitempty"`
	FromID       string           `json:"fromId,omitempty"`
	ToID         string           `json:"toId,omitempty"`
	ItemSlug     string           `json:"itemSlug,omitempty"`
	InstanceID   string           `json:"instanceId,omitempty"`
	// Action is the 2024 action take_action takes, with Detail for what Help, Magic or Utilize does;
	// Trigger sets off a readied attack; Option is a Grapple or Shove.
	Action  string `json:"action,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Trigger string `json:"trigger,omitempty"`
	Option  string `json:"option,omitempty"`
	// OffHand makes an attack the off-hand attack of a Light weapon.
	OffHand bool `json:"offHand,omitempty"`
	// Cleave makes an attack the second attack of a Cleave hit.
	Cleave bool `json:"cleave,omitempty"`
	// ReactionKind, ReactionMode and Condition are a reaction setting set_reaction stores.
	ReactionKind string `json:"reactionKind,omitempty"`
	ReactionMode string `json:"reactionMode,omitempty"`
	Condition    string `json:"condition,omitempty"`
	Coin         string `json:"coin,omitempty"`
	Count        int    `json:"count,omitempty"`
	ShopID       string `json:"shopId,omitempty"`
	// Monsters are what spawn_encounter places; HPDelta is what adjust_hp adds; Seq is the Action undo reverts.
	Monsters []SpawnMonster `json:"monsters,omitempty"`
	HPDelta  int            `json:"hpDelta,omitempty"`
	Seq      int64          `json:"seq,omitempty"`
	promptID uuid.UUID
	rollID   domain.RollID
}

// Update kinds. A snapshot answers a join or resync; a view follows every change.
const (
	UpdSnapshot      = "snapshot"
	UpdView          = "view"
	UpdRejected      = "rejected"
	UpdPath          = "path"
	UpdAttackPreview = "attack_preview"
	UpdAreaPreview   = "area_preview"
	UpdPing          = "ping"
	UpdEnded         = "ended"
)

// SessionView is the Session as a client sees it.
type SessionView struct {
	ID         string   `json:"id"`
	Number     int      `json:"number"`
	GridRadius int      `json:"gridRadius"`
	Audience   Audience `json:"audience"`
}

// TokenView is a Token as a client sees it.
type TokenView struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`
	Q            int    `json:"q"`
	R            int    `json:"r"`
	Hidden       bool   `json:"hidden"`
	DarkvisionFt int    `json:"darkvisionFt"`
	ControllerID string `json:"controllerId,omitempty"`
	// AC, HP and attacks go to the DM, and to everyone for party tokens; others only show their health.
	AC      *int         `json:"ac,omitempty"`
	HP      *int         `json:"hp,omitempty"`
	HPMax   *int         `json:"hpMax,omitempty"`
	TempHP  int          `json:"tempHp,omitempty"`
	Health  string       `json:"health,omitempty"`
	Attacks []AttackView `json:"attacks,omitempty"`
	Shield  bool         `json:"shield,omitempty"`
	Effects []EffectView `json:"effects,omitempty"`
	// Reactions are the Controller's reaction settings, shown to the DM and for the party's tokens;
	// Dying is a Character's death saves at 0 hit points.
	Reactions []ReactionSettingView `json:"reactions,omitempty"`
	Dying     *DyingView            `json:"dying,omitempty"`
}

// EffectView is an Effect on a token, which everyone who sees the token sees.
type EffectView struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	SourceID      string `json:"sourceId,omitempty"`
	Concentration bool   `json:"concentration"`
	RoundsLeft    int    `json:"roundsLeft,omitempty"`
	// Level is how many levels of a stacking Effect (exhaustion) the token has; Hexes the area an
	// emanation covers around the token where it stands.
	Level int   `json:"level,omitempty"`
	Hexes []Hex `json:"hexes,omitempty"`
}

// ManualView is part of an Effect the DM resolves by hand.
type ManualView struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// AttackView is one attack on a hotbar.
type AttackView struct {
	Name        string `json:"name"`
	ToHit       int    `json:"toHit"`
	ReachFt     int    `json:"reachFt"`
	RangeFt     int    `json:"rangeFt"`
	LongRangeFt int    `json:"longRangeFt"`
	Damage      string `json:"damage,omitempty"`
	DamageBonus int    `json:"damageBonus"`
	DamageType  string `json:"damageType,omitempty"`
	// Light weapons open the off-hand attack; Mastery is the weapon's mastery, when it is mastered.
	Light   bool   `json:"light,omitempty"`
	Mastery string `json:"mastery,omitempty"`
}

// AttackPreview is what an attack would do, sent only to whoever asked: the chance to hit, the damage
// range and every reason behind them.
type AttackPreview struct {
	TokenID   string   `json:"tokenId"`
	TargetID  string   `json:"targetId"`
	AttackNo  int      `json:"attackNo"`
	Name      string   `json:"name"`
	HitChance int      `json:"hitChance"`
	Mode      string   `json:"mode"`
	DamageMin int      `json:"damageMin"`
	DamageMax int      `json:"damageMax"`
	CritMax   int      `json:"critMax"`
	Reasons   []string `json:"reasons"`
}

// PendingAttackView is an attack waiting on a Roll Card.
type PendingAttackView struct {
	AttackerID string `json:"attackerId"`
	TargetID   string `json:"targetId"`
	Name       string `json:"name"`
	Stage      string `json:"stage"`
	RollID     string `json:"rollId"`
	Critical   bool   `json:"critical"`
}

// PathView is the route a walk would take and what it costs, sent only to whoever asked.
type PathView struct {
	TokenID string `json:"tokenId"`
	Hexes   []Hex  `json:"hexes"`
	CostFt  int    `json:"costFt"`
}

// MapView is the active Map's geometry and picture.
type MapView struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ImageURL     string  `json:"imageUrl"`
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	HexSizePx    float64 `json:"hexSizePx"`
	OriginX      float64 `json:"originX"`
	OriginY      float64 `json:"originY"`
	ImageVersion int     `json:"imageVersion"`
}

// LightView is a light, shown to the DM only.
type LightView struct {
	ID       string `json:"id"`
	Q        int    `json:"q"`
	R        int    `json:"r"`
	BrightFt int    `json:"brightFt"`
	DimFt    int    `json:"dimFt"`
}

// View is everything an audience may see right now. With a Map, hexes are visible now, remembered, or
// absent: a hex the party never saw appears in no list, and nothing in it is sent.
type View struct {
	Tokens     []TokenView `json:"tokens"`
	Map        *MapView    `json:"map,omitempty"`
	Fog        bool        `json:"fog"`
	Visible    []Hex       `json:"visible"`
	Remembered []Hex       `json:"remembered"`
	Walls      []Hex       `json:"walls,omitempty"`
	Lights     []LightView `json:"lights,omitempty"`
	Ambient    string      `json:"ambient,omitempty"`
	Combat     *CombatView `json:"combat,omitempty"`
	// Manual goes to the DM; everyone else only learns that something is being resolved.
	Manual    []ManualView    `json:"manual,omitempty"`
	Resolving bool            `json:"resolving,omitempty"`
	Saves     []SaveView      `json:"saves,omitempty"`
	Surfaces  []SurfaceView   `json:"surfaces,omitempty"`
	Elevation []ElevationView `json:"elevation,omitempty"`
	Area      *AreaView       `json:"area,omitempty"`
	Table     *TableView      `json:"table,omitempty"`
	World     *WorldView      `json:"world,omitempty"`
	// Zones go to the DM only; Perception lists the party's Perception Roll Cards, never the DC.
	Zones      []ZoneView       `json:"zones,omitempty"`
	Perception []PerceptionView `json:"perception,omitempty"`
	Checks     []CheckView      `json:"checks,omitempty"`
	Inventory  []ContainerView  `json:"inventory,omitempty"`
	Shop       *ShopView        `json:"shop,omitempty"`
	Rest       *RestView        `json:"rest,omitempty"`
	GameDay    int              `json:"gameDay"`
}

// RestView is the rest the party proposed or is taking: who agreed, who the rest still waits on, and
// each resting Character's Hit Dice.
type RestView struct {
	Kind        string       `json:"kind"`
	Status      string       `json:"status"`
	ProposedBy  string       `json:"proposedBy"`
	Agreed      []string     `json:"agreed"`
	Waiting     []string     `json:"waiting"`
	WaitingOnDM bool         `json:"waitingOnDm"`
	Resters     []ResterView `json:"resters"`
}

// ResterView is a resting Character: their Hit Die, how many are left, and the roll of one being spent.
type ResterView struct {
	CharacterID string `json:"characterId"`
	TokenID     string `json:"tokenId"`
	Name        string `json:"name"`
	HitDie      string `json:"hitDie"`
	HitDiceLeft int    `json:"hitDiceLeft"`
	RollID      string `json:"rollId,omitempty"`
}

// ContainerView is a Character's Inventory, the Party Stash or a drop of loot, with what it weighs.
// A Character's carries its owner and how much they can carry.
type ContainerView struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	Label       string         `json:"label"`
	CharacterID string         `json:"characterId,omitempty"`
	OwnerID     string         `json:"ownerId,omitempty"`
	ParentID    string         `json:"parentId,omitempty"`
	Items       []ItemView     `json:"items"`
	Instances   []InstanceView `json:"instances"`
	Coins       []CoinView     `json:"coins"`
	WeightLb    float64        `json:"weightLb"`
	CapacityLb  float64        `json:"capacityLb,omitempty"`
	Encumbered  bool           `json:"encumbered,omitempty"`
}

// ItemView is a stack of one item in a Container.
type ItemView struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	WeightLb float64 `json:"weightLb"`
}

// InstanceView is one Item Instance. Only the DM sees what an unidentified item really is: the party
// sees the base item, without its own name or Charges.
type InstanceView struct {
	ID         string  `json:"id"`
	Slug       string  `json:"slug"`
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	Charges    *int    `json:"charges,omitempty"`
	Identified bool    `json:"identified"`
	Attuned    bool    `json:"attuned,omitempty"`
	Slot       string  `json:"slot,omitempty"`
	WeightLb   float64 `json:"weightLb"`
}

// CoinView is how many coins of one kind a Container holds.
type CoinView struct {
	Coin  string `json:"coin"`
	Count int    `json:"count"`
}

// CheckView is an Encounter Check. Everyone sees what set it off and its outcome, and an open check's
// roll; only the DM sees its table, mode, seed, entry and creatures.
type CheckView struct {
	ID         string             `json:"id"`
	Trigger    string             `json:"trigger"`
	Visibility string             `json:"visibility"`
	Status     string             `json:"status"`
	Outcome    string             `json:"outcome,omitempty"`
	ChancePct  int                `json:"chancePct,omitempty"`
	ChanceRoll int                `json:"chanceRoll,omitempty"`
	RollID     string             `json:"rollId,omitempty"`
	TableName  string             `json:"tableName,omitempty"`
	Mode       string             `json:"mode,omitempty"`
	Seed       string             `json:"seed,omitempty"`
	EntryLabel string             `json:"entryLabel,omitempty"`
	Monsters   []CheckMonsterView `json:"monsters,omitempty"`
}

// CheckMonsterView is how many of one creature a check produced.
type CheckMonsterView struct {
	Slug  string `json:"slug"`
	Count int    `json:"count"`
}

// ZoneView is an Encounter Zone as the DM sees it.
type ZoneView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Q           int             `json:"q"`
	R           int             `json:"r"`
	RadiusHexes int             `json:"radiusHexes"`
	DMOnly      bool            `json:"dmOnly"`
	Held        bool            `json:"held"`
	Status      string          `json:"status"`
	DC          int             `json:"dc,omitempty"`
	Creatures   int             `json:"creatures"`
	Checks      []ZoneCheckView `json:"checks"`
}

// ZoneCheckView is whether one party member noticed a sprung zone; Noticed is absent while they roll.
type ZoneCheckView struct {
	TokenID string `json:"tokenId"`
	Noticed *bool  `json:"noticed,omitempty"`
}

// PerceptionView is a party member's Perception Roll Card after a zone springs.
type PerceptionView struct {
	RollID  string `json:"rollId"`
	TokenID string `json:"tokenId"`
}

// WorldView is the world map the party travels: the DM sees it all, everyone else only the locations
// the party has seen or can reach from where it stands, and the routes between them.
type WorldView struct {
	Map         MapView     `json:"map"`
	Revealed    []Hex       `json:"revealed"`
	Nodes       []NodeView  `json:"nodes"`
	Routes      []RouteView `json:"routes"`
	PartyNodeID string      `json:"partyNodeId,omitempty"`
	Legs        []LegView   `json:"legs"`
}

// NodeView is a location on the world map.
type NodeView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Q    int    `json:"q"`
	R    int    `json:"r"`
}

// RouteView is a route with how long it takes at each pace.
type RouteView struct {
	ID         string     `json:"id"`
	FromNodeID string     `json:"fromNodeId"`
	ToNodeID   string     `json:"toNodeId"`
	DistanceMi int        `json:"distanceMi"`
	Plans      []PlanView `json:"plans"`
}

// PlanView is how long a route takes at one pace.
type PlanView struct {
	Pace    string `json:"pace"`
	Minutes int    `json:"minutes"`
	Days    int    `json:"days"`
}

// LegView is one Travel Leg the party made this Session.
type LegView struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Pace       string `json:"pace"`
	DistanceMi int    `json:"distanceMi"`
	Minutes    int    `json:"minutes"`
	Days       int    `json:"days"`
}

// SaveView is a saving throw waiting on its Roll Card to end an Effect.
type SaveView struct {
	RollID  string `json:"rollId"`
	TokenID string `json:"tokenId"`
	Effect  string `json:"effect"`
	DC      int    `json:"dc"`
}

// CombatView is the running Combat: its round and every Combatant the audience can see, in turn order.
type CombatView struct {
	Status     string             `json:"status"`
	Round      int                `json:"round"`
	Combatants []CombatantView    `json:"combatants"`
	Attack     *PendingAttackView `json:"attack,omitempty"`
	Prompt     *PromptView        `json:"prompt,omitempty"`
}

// PromptView is a Reaction Prompt: who may react, to what, what it would do, and how long is left.
type PromptView struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	ReactorID   string `json:"reactorId"`
	TriggerID   string `json:"triggerId"`
	Effect      string `json:"effect"`
	SecondsLeft int    `json:"secondsLeft"`
}

// CombatantView is one Combatant in the initiative rail. Tied initiatives share a rank and act together.
type CombatantView struct {
	ID           string `json:"id"`
	TokenID      string `json:"tokenId"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`
	ControllerID string `json:"controllerId,omitempty"`
	RollID       string `json:"rollId"`
	Initiative   *int   `json:"initiative,omitempty"`
	Rank         int    `json:"rank,omitempty"`
	Acting       bool   `json:"acting"`
	Done         bool   `json:"done"`
	Action       bool   `json:"action"`
	BonusAction  bool   `json:"bonusAction"`
	Reaction     bool   `json:"reaction"`
	MovementFt   int    `json:"movementFt"`
	SpeedFt      int    `json:"speedFt"`
	// Disengaged movement provokes no opportunity attacks; a Readied attack is shown to the DM and,
	// for the party's own Combatants, to the party.
	Disengaged bool `json:"disengaged,omitempty"`
	Readied    bool `json:"readied,omitempty"`
	// AttacksLeft are the attacks of an Attack action already begun; OffHand is set while the off-hand
	// attack is open; Interaction while the free object interaction is unused.
	AttacksLeft int  `json:"attacksLeft,omitempty"`
	OffHand     bool `json:"offHand,omitempty"`
	Interaction bool `json:"interaction,omitempty"`
	// Cleave is set while a Cleave hit leaves a second attack open.
	Cleave bool `json:"cleave,omitempty"`
	// Tactics and Suggestion go to the DM only.
	Surprised  bool            `json:"surprised,omitempty"`
	Tactics    string          `json:"tactics,omitempty"`
	Suggestion *SuggestionView `json:"suggestion,omitempty"`
}

// SuggestionView is a creature's Suggested Action; without AttackNo nothing reaches yet and it should close in.
type SuggestionView struct {
	AttackNo *int   `json:"attackNo,omitempty"`
	TargetID string `json:"targetId"`
	Reason   string `json:"reason"`
}

// Update is what the server sends. Every Update carries the Session sequence; a view whose sequence is
// not the next one means the client missed something and must resync.
type Update struct {
	Kind  string `json:"kind"`
	Seq   int64  `json:"seq"`
	Nonce string `json:"nonce,omitempty"`
	// ActionSeq is the Action Log sequence of the change that answers the sender's command.
	ActionSeq int64        `json:"actionSeq,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	Session   *SessionView `json:"session,omitempty"`
	View      *View        `json:"view,omitempty"`
	// Steps are the views along a walk before its final View, for clients to play back at walking pace.
	Steps   []View         `json:"steps,omitempty"`
	Path    *PathView      `json:"path,omitempty"`
	Preview *AttackPreview `json:"preview,omitempty"`
	Area    *AreaPreview   `json:"area,omitempty"`
	Ping    *Hex           `json:"ping,omitempty"`
}

// TableView is what the Table Display shows: its camera, its scene and whether it is blacked out.
type TableView struct {
	Camera   string   `json:"camera"`
	Q        int      `json:"q"`
	R        int      `json:"r"`
	ZoomPct  int      `json:"zoomPct"`
	Scene    string   `json:"scene"`
	Title    string   `json:"title,omitempty"`
	Body     string   `json:"body,omitempty"`
	WorldMap *MapView `json:"worldMap,omitempty"`
	Blackout bool     `json:"blackout"`
}

// AreaPreview is an area spell's template and who it would catch, allies flagged, sent only to whoever asked.
type AreaPreview struct {
	TokenID string       `json:"tokenId"`
	Effect  string       `json:"effect"`
	Name    string       `json:"name"`
	DC      int          `json:"dc"`
	Hexes   []Hex        `json:"hexes"`
	Targets []AreaTarget `json:"targets"`
	Allies  int          `json:"allies"`
	// Ends names the Effects the caster concentrates on that casting this one would end.
	Ends []string `json:"ends,omitempty"`
}

// AreaTarget is a creature an area catches.
type AreaTarget struct {
	TokenID string `json:"tokenId"`
	Ally    bool   `json:"ally"`
}

// AreaView is an area spell waiting on its rolls.
type AreaView struct {
	CasterID     string     `json:"casterId"`
	Name         string     `json:"name"`
	Hexes        []Hex      `json:"hexes"`
	DamageRollID string     `json:"damageRollId,omitempty"`
	Saves        []AreaSave `json:"saves"`
}

// AreaSave is one target's saving throw against an area.
type AreaSave struct {
	TokenID string `json:"tokenId"`
	RollID  string `json:"rollId,omitempty"`
}

// SurfaceView is a Surface on a hex.
type SurfaceView struct {
	Q          int    `json:"q"`
	R          int    `json:"r"`
	Kind       string `json:"kind"`
	RoundsLeft int    `json:"roundsLeft,omitempty"`
}

// ElevationView is a raised or sunken hex.
type ElevationView struct {
	Q           int `json:"q"`
	R           int `json:"r"`
	ElevationFt int `json:"elevationFt"`
}

func tokenView(t domain.Token, a Audience) TokenView {
	v := TokenView{ID: uuid.UUID(t.ID).String(), Label: t.Label, Kind: t.Kind, Q: t.Q, R: t.R, Hidden: t.Hidden, DarkvisionFt: t.DarkvisionFt}
	if t.Controller != nil {
		v.ControllerID = t.Controller.String()
	}
	s := t.Stats
	switch {
	case s == nil:
	case a == AudienceDM || t.Kind == domain.TokenParty:
		v.AC, v.HP, v.HPMax, v.TempHP, v.Attacks, v.Shield, v.Reactions = &s.AC, &s.HP, &s.HPMax, s.TempHP, []AttackView{}, t.CanShield, reactionViews(t)
		for _, x := range s.Attacks {
			v.Attacks = append(v.Attacks, AttackView{
				Name: x.Name, ToHit: x.ToHit, ReachFt: x.ReachFt, RangeFt: x.RangeFt, LongRangeFt: x.LongRangeFt, Damage: x.Damage,
				DamageBonus: x.DamageBonus, DamageType: x.DamageType, Light: x.Light, Mastery: x.Mastery,
			})
		}
	default:
		v.Health = health(s.HP, s.HPMax)
	}
	return v
}

// health is what anyone can tell by looking: unhurt, hurt, bloodied at half or less, or down.
func health(hp, most int) string {
	switch {
	case hp <= 0:
		return "down"
	case hp*2 <= most:
		return "bloodied"
	case hp < most:
		return "hurt"
	}
	return "unhurt"
}

func wireHexes(cs []hex.Coord) []Hex {
	out := make([]Hex, 0, len(cs))
	for _, c := range cs {
		out = append(out, Hex{Q: c.Q, R: c.R})
	}
	return out
}

func hexes(set map[hex.Coord]bool) []Hex {
	out := make([]Hex, 0, len(set))
	for c := range set {
		out = append(out, Hex{Q: c.Q, R: c.R})
	}
	sortHexes(out)
	return out
}

// ShopView is the Shop open in the Session: its Stock with asking prices in copper, and each
// Character's haggling there, settled or waiting on its roll.
type ShopView struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Kind       string       `json:"kind"`
	Settlement string       `json:"settlement"`
	Owner      string       `json:"owner,omitempty"`
	Stock      []StockView  `json:"stock"`
	Haggles    []HaggleView `json:"haggles"`
}

// StockView is one item the open Shop sells.
type StockView struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	PriceCP  int     `json:"priceCp"`
	WeightLb float64 `json:"weightLb"`
}

// HaggleView is a Character's haggling: its roll while out, then its price adjustment in percent.
type HaggleView struct {
	CharacterID string `json:"characterId"`
	RollID      string `json:"rollId,omitempty"`
	AdjustPct   *int   `json:"adjustPct,omitempty"`
}
