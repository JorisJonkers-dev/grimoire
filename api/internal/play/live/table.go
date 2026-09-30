package live

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// planTable changes what the Table Display shows: its camera, its scene or its blackout.
func (r *runtime) planTable(cmd Command) (Write, string) {
	next := r.st.table
	world := r.st.world
	switch cmd.Kind {
	case CmdTableCamera:
		switch {
		case cmd.Camera != domain.CameraFollowTurn && cmd.Camera != domain.CameraShowParty && cmd.Camera != domain.CameraFree:
			return Write{}, "The camera follows the turn, shows the party, or stays where you put it."
		case cmd.ZoomPct < 50 || cmd.ZoomPct > 300:
			return Write{}, "Zoom runs from 50 to 300 percent."
		}
		next.Camera, next.Q, next.R, next.ZoomPct = cmd.Camera, cmd.Q, cmd.R, cmd.ZoomPct
	case CmdTableScene:
		title, body := strings.TrimSpace(cmd.Title), strings.TrimSpace(cmd.Body)
		switch {
		case cmd.Scene != domain.SceneLocal && cmd.Scene != domain.SceneWorld && cmd.Scene != domain.SceneHandout && cmd.Scene != domain.SceneTitle:
			return Write{}, "Scenes are the local map, the world map, a handout or a title card."
		case utf8.RuneCountInString(title) > 80 || utf8.RuneCountInString(body) > 1000:
			return Write{}, "Titles run to 80 characters and handouts to 1000."
		}
		next.Scene, next.Title, next.Body, next.MapID, world = cmd.Scene, title, body, nil, nil
		if cmd.Scene == domain.SceneWorld {
			id, _ := uuid.Parse(cmd.MapID)
			board, err := r.store.LoadMap(context.Background(), r.st.session.CampaignID, domain.MapID(id))
			if err != nil {
				return Write{}, "Choose a map for the world scene."
			}
			next.MapID, world = &board.Map.ID, &board.Map
		}
	default:
		next.Blackout = cmd.On
	}
	return Write{Kind: domain.ActionTableSet, Table: &next, world: world}, ""
}

// ping flashes a hex on every screen; it changes nothing, so it is neither saved nor sequenced.
func (r *runtime) ping(req request) {
	c := Hex{Q: req.cmd.Q, R: req.cmd.R}
	for sub := range r.subs {
		r.send(sub, Update{Kind: UpdPing, Seq: r.st.session.Seq, Ping: &c})
	}
}

// tableView shows the Table Display's settings to every audience.
func (s *state) tableView() *TableView {
	t := s.table
	v := &TableView{Camera: t.Camera, Q: t.Q, R: t.R, ZoomPct: t.ZoomPct, Scene: t.Scene, Title: t.Title, Body: t.Body, Blackout: t.Blackout}
	if w := s.world; w != nil && t.Scene == domain.SceneWorld {
		v.WorldMap = &MapView{
			ID: uuid.UUID(w.ID).String(), Name: w.Name, Width: w.Width, Height: w.Height, HexSizePx: w.HexSize, OriginX: w.OriginX, OriginY: w.OriginY,
			ImageURL: "/api/v1/campaigns/" + w.CampaignID.String() + "/maps/" + uuid.UUID(w.ID).String() + "/image",
		}
	}
	return v
}
