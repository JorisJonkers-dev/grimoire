package live

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
)

// plan validates a command against the live state and turns it into a Write.
func (r *runtime) plan(m domain.Member, cmd Command) (Write, string) {
	switch cmd.Kind {
	case CmdWalk:
		t, path, _, reason := r.route(m, cmd)
		return Write{Kind: domain.ActionTokenWalked, Token: t, Path: path}, reason
	case CmdPlace, CmdMove, CmdSetHidden, CmdRemove:
		return r.planToken(cmd)
	case CmdSetMap:
		return r.planMap(cmd)
	case CmdRevealHexes, CmdSetWalls:
		if r.st.board == nil {
			return Write{}, "Choose a map first."
		}
		return r.planPaint(cmd)
	case CmdPlaceLight, CmdRemoveLight, CmdSetAmbient:
		if r.st.board == nil {
			return Write{}, "Choose a map first."
		}
		return r.planLight(cmd)
	default:
		return Write{}, "Unknown command."
	}
}

func (r *runtime) planToken(cmd Command) (Write, string) {
	if cmd.Kind == CmdPlace {
		return r.planPlace(cmd)
	}
	id, err := uuid.Parse(cmd.TokenID)
	t, ok := r.st.tokens[domain.TokenID(id)]
	if err != nil || !ok {
		return Write{}, "No such token."
	}
	switch cmd.Kind {
	case CmdMove:
		if !r.st.onBoard(hex.Coord{Q: cmd.Q, R: cmd.R}) {
			return Write{}, "That hex is off the map."
		}
		t.Q, t.R = cmd.Q, cmd.R
		return Write{Kind: domain.ActionTokenMoved, Token: t}, ""
	case CmdSetHidden:
		t.Hidden = cmd.Hidden
		kind := domain.ActionTokenRevealed
		if cmd.Hidden {
			kind = domain.ActionTokenHidden
		}
		return Write{Kind: kind, Token: t}, ""
	default:
		return Write{Kind: domain.ActionTokenRemoved, Token: t}, ""
	}
}

func (r *runtime) planPlace(cmd Command) (Write, string) {
	at := hex.Coord{Q: cmd.Q, R: cmd.R}
	label := strings.TrimSpace(cmd.Label)
	switch {
	case label == "" || len([]rune(label)) > 40:
		return Write{}, "Give the token a name of up to 40 characters."
	case cmd.TokenKind != domain.TokenParty && cmd.TokenKind != domain.TokenEnemy && cmd.TokenKind != domain.TokenNPC && cmd.TokenKind != domain.TokenObject:
		return Write{}, "Unknown token kind."
	case cmd.DarkvisionFt < 0 || cmd.DarkvisionFt > 300:
		return Write{}, "Darkvision runs from 0 to 300 feet."
	case !r.st.onBoard(at):
		return Write{}, "That hex is off the map."
	}
	t := domain.Token{ID: domain.TokenID(uuid.New()), Label: label, Kind: cmd.TokenKind, Q: cmd.Q, R: cmd.R, Hidden: cmd.Hidden, DarkvisionFt: cmd.DarkvisionFt}
	if cmd.ControllerID != "" {
		id, err := uuid.Parse(cmd.ControllerID)
		if err != nil {
			return Write{}, "No such member."
		}
		if _, err := r.members.Member(context.Background(), r.st.session.CampaignID, id); err != nil {
			return Write{}, "No such member."
		}
		t.Controller = &id
	}
	return Write{Kind: domain.ActionTokenPlaced, Token: t}, ""
}

func (r *runtime) planMap(cmd Command) (Write, string) {
	if cmd.MapID == "" {
		return Write{Kind: domain.ActionMapSet}, ""
	}
	id, err := uuid.Parse(cmd.MapID)
	if err != nil {
		return Write{}, "No such map."
	}
	board, err := r.store.LoadMap(context.Background(), r.st.session.CampaignID, domain.MapID(id))
	if err != nil {
		return Write{}, "No such map."
	}
	mid := domain.MapID(id)
	return Write{Kind: domain.ActionMapSet, MapID: &mid, board: board}, ""
}

func (r *runtime) boardHexes(hs []Hex) ([]hex.Coord, string) {
	if len(hs) == 0 || len(hs) > 2000 {
		return nil, "Paint between 1 and 2000 hexes at a time."
	}
	out := make([]hex.Coord, 0, len(hs))
	for _, h := range hs {
		c := hex.Coord{Q: h.Q, R: h.R}
		if !r.st.onBoard(c) {
			return nil, "That hex is off the map."
		}
		out = append(out, c)
	}
	return out, ""
}

func (r *runtime) planPaint(cmd Command) (Write, string) {
	hs, reason := r.boardHexes(cmd.Hexes)
	if reason != "" {
		return Write{}, reason
	}
	kinds := map[string][2]string{
		CmdRevealHexes: {domain.ActionHexesConcealed, domain.ActionHexesRevealed},
		CmdSetWalls:    {domain.ActionWallsCleared, domain.ActionWallsSet},
	}[cmd.Kind]
	kind := kinds[0]
	if cmd.On {
		kind = kinds[1]
	}
	return Write{Kind: kind, Hexes: hs}, ""
}

func (r *runtime) planLight(cmd Command) (Write, string) {
	switch cmd.Kind {
	case CmdPlaceLight:
		at := hex.Coord{Q: cmd.Q, R: cmd.R}
		switch {
		case !r.st.onBoard(at):
			return Write{}, "That hex is off the map."
		case cmd.BrightFt < 0 || cmd.DimFt < 0 || cmd.BrightFt > 600 || cmd.DimFt > 600:
			return Write{}, "Light reaches 0 to 600 feet."
		}
		return Write{Kind: domain.ActionLightPlaced, Light: domain.MapLight{ID: domain.LightID(uuid.New()), At: at, BrightFt: cmd.BrightFt, DimFt: cmd.DimFt}}, ""
	case CmdRemoveLight:
		id, _ := uuid.Parse(cmd.LightID)
		for _, l := range r.st.board.Lights {
			if l.ID == domain.LightID(id) {
				return Write{Kind: domain.ActionLightRemoved, Light: l}, ""
			}
		}
		return Write{}, "No such light."
	default:
		if cmd.Ambient != domain.AmbientBright && cmd.Ambient != domain.AmbientDim && cmd.Ambient != domain.AmbientDark {
			return Write{}, "Ambient light is bright, dim or dark."
		}
		return Write{Kind: domain.ActionAmbientSet, Ambient: cmd.Ambient}, ""
	}
}
