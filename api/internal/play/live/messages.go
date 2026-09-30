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
	CmdResync      = "resync"
	CmdPlace       = "place_token"
	CmdMove        = "move_token"
	CmdSetHidden   = "set_token_hidden"
	CmdRemove      = "remove_token"
	CmdSetMap      = "set_map"
	CmdRevealHexes = "reveal_hexes"
	CmdSetWalls    = "set_walls"
	CmdPlaceLight  = "place_light"
	CmdRemoveLight = "remove_light"
	CmdSetAmbient  = "set_ambient"
	CmdPlanWalk    = "plan_walk"
	CmdWalk        = "walk"
	CmdStartCombat = "start_combat"
	CmdEndTurn     = "end_turn"
	CmdSpend       = "spend"
	CmdEndCombat   = "end_combat"
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
	rollID       domain.RollID
}

// Update kinds. A snapshot answers a join or resync; a view follows every change.
const (
	UpdSnapshot = "snapshot"
	UpdView     = "view"
	UpdRejected = "rejected"
	UpdPath     = "path"
	UpdEnded    = "ended"
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
}

// CombatView is the running Combat: its round and every Combatant the audience can see, in turn order.
type CombatView struct {
	Status     string          `json:"status"`
	Round      int             `json:"round"`
	Combatants []CombatantView `json:"combatants"`
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
}

// Update is what the server sends. Every Update carries the Session sequence; a view whose sequence is
// not the next one means the client missed something and must resync.
type Update struct {
	Kind    string       `json:"kind"`
	Seq     int64        `json:"seq"`
	Nonce   string       `json:"nonce,omitempty"`
	Reason  string       `json:"reason,omitempty"`
	Session *SessionView `json:"session,omitempty"`
	View    *View        `json:"view,omitempty"`
	// Steps are the views along a walk before its final View, for clients to play back at walking pace.
	Steps []View    `json:"steps,omitempty"`
	Path  *PathView `json:"path,omitempty"`
}

func tokenView(t domain.Token) TokenView {
	v := TokenView{ID: uuid.UUID(t.ID).String(), Label: t.Label, Kind: t.Kind, Q: t.Q, R: t.R, Hidden: t.Hidden, DarkvisionFt: t.DarkvisionFt}
	if t.Controller != nil {
		v.ControllerID = t.Controller.String()
	}
	return v
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
