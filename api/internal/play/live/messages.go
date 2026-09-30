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
	CmdResync        = "resync"
	CmdPlace         = "place_token"
	CmdMove          = "move_token"
	CmdSetHidden     = "set_token_hidden"
	CmdRemove        = "remove_token"
	CmdSetMap        = "set_map"
	CmdRevealHexes   = "reveal_hexes"
	CmdSetWalls      = "set_walls"
	CmdPlaceLight    = "place_light"
	CmdRemoveLight   = "remove_light"
	CmdSetAmbient    = "set_ambient"
	CmdPlanWalk      = "plan_walk"
	CmdWalk          = "walk"
	CmdStartCombat   = "start_combat"
	CmdEndTurn       = "end_turn"
	CmdSpend         = "spend"
	CmdEndCombat     = "end_combat"
	CmdPreviewAttack = "preview_attack"
	CmdAttack        = "attack"
	CmdUndoDamage    = "undo_damage"
	CmdSetTactics    = "set_tactics"
	CmdReact         = "react"
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
	promptID     uuid.UUID
	rollID       domain.RollID
}

// Update kinds. A snapshot answers a join or resync; a view follows every change.
const (
	UpdSnapshot      = "snapshot"
	UpdView          = "view"
	UpdRejected      = "rejected"
	UpdPath          = "path"
	UpdAttackPreview = "attack_preview"
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
	Health  string       `json:"health,omitempty"`
	Attacks []AttackView `json:"attacks,omitempty"`
	Shield  bool         `json:"shield,omitempty"`
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
	// Tactics and Suggestion go to the DM only.
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
	Kind    string       `json:"kind"`
	Seq     int64        `json:"seq"`
	Nonce   string       `json:"nonce,omitempty"`
	Reason  string       `json:"reason,omitempty"`
	Session *SessionView `json:"session,omitempty"`
	View    *View        `json:"view,omitempty"`
	// Steps are the views along a walk before its final View, for clients to play back at walking pace.
	Steps   []View         `json:"steps,omitempty"`
	Path    *PathView      `json:"path,omitempty"`
	Preview *AttackPreview `json:"preview,omitempty"`
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
		v.AC, v.HP, v.HPMax, v.Attacks, v.Shield = &s.AC, &s.HP, &s.HPMax, []AttackView{}, t.CanShield
		for _, x := range s.Attacks {
			v.Attacks = append(v.Attacks, AttackView{
				Name: x.Name, ToHit: x.ToHit, ReachFt: x.ReachFt, RangeFt: x.RangeFt, LongRangeFt: x.LongRangeFt, Damage: x.Damage,
				DamageBonus: x.DamageBonus, DamageType: x.DamageType,
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
