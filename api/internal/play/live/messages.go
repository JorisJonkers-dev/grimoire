// Package live runs Sessions: one goroutine per Session owns its state, writes every change through
// to Postgres, and sends each audience only what it may see.
package live

import (
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
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
	CmdResync    = "resync"
	CmdPlace     = "place_token"
	CmdMove      = "move_token"
	CmdSetHidden = "set_token_hidden"
	CmdRemove    = "remove_token"
)

// Command is what a client asks for.
type Command struct {
	Nonce     string `json:"nonce"`
	Kind      string `json:"kind"`
	TokenID   string `json:"tokenId,omitempty"`
	Label     string `json:"label,omitempty"`
	TokenKind string `json:"tokenKind,omitempty"`
	Q         int    `json:"q"`
	R         int    `json:"r"`
	Hidden    bool   `json:"hidden"`
}

// Update kinds.
const (
	UpdSnapshot     = "snapshot"
	UpdToken        = "token"
	UpdTokenRemoved = "token_removed"
	UpdTick         = "tick"
	UpdRejected     = "rejected"
	UpdEnded        = "ended"
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
	ID     string `json:"id"`
	Label  string `json:"label"`
	Kind   string `json:"kind"`
	Q      int    `json:"q"`
	R      int    `json:"r"`
	Hidden bool   `json:"hidden"`
}

// Update is what the server sends. Every Update carries the Session sequence; a client that sees a
// gap asks for a resync. Audiences that see nothing of a change get a tick, so their sequence stays whole.
type Update struct {
	Kind    string       `json:"kind"`
	Seq     int64        `json:"seq"`
	Nonce   string       `json:"nonce,omitempty"`
	Reason  string       `json:"reason,omitempty"`
	Session *SessionView `json:"session,omitempty"`
	Tokens  []TokenView  `json:"tokens,omitempty"`
	Token   *TokenView   `json:"token,omitempty"`
	TokenID string       `json:"tokenId,omitempty"`
}

func view(t domain.Token) TokenView {
	return TokenView{ID: uuid.UUID(t.ID).String(), Label: t.Label, Kind: t.Kind, Q: t.Q, R: t.R, Hidden: t.Hidden}
}

// sees reports whether an audience may see a token. This is the security boundary for hidden tokens.
func sees(a Audience, t domain.Token) bool {
	return a == AudienceDM || !t.Hidden
}
