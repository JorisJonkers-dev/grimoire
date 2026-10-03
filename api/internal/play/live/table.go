package live

import (
	"context"
	"strings"
	"unicode/utf16"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
)

// What every screen is sent has to fit what the screens accept, counted as they count it: the caption
// is kept, and the last roll is sent again with every snapshot, so one that did not fit would leave no
// screen able to read a snapshot at all.
const (
	maxCaption      = 200
	maxRollPurpose  = 120
	maxRollerName   = 60
	maxRollDice     = 100
	maxRollDieFaces = 1000
	maxRollNumber   = 100000
)

// wireLen is a string's length as the screens count it, in UTF-16 code units: a character outside the
// basic plane, such as an emoji, counts twice.
func wireLen(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// fits reports whether a roll is within what the screens accept.
func (r RollShown) fits() bool {
	ok := wireLen(r.Purpose) <= maxRollPurpose && wireLen(r.Roller) <= maxRollerName && r.Roller != "" && len(r.Dice) <= maxRollDice &&
		abs(r.Modifier) <= maxRollNumber && abs(r.Total) <= maxRollNumber
	for _, d := range r.Dice {
		ok = ok && d.Faces >= 2 && d.Faces <= maxRollDieFaces && d.Value >= 0 && d.Value <= maxRollDieFaces
	}
	return ok
}

func abs(n int) int {
	return max(n, -n)
}

// planTable changes what the Table Display shows: its camera, its scene, its caption or its blackout.
func (r *runtime) planTable(cmd Command) (Write, string) {
	next := r.st.table
	shown := r.st.tableMap
	switch cmd.Kind {
	case CmdTableCamera:
		if reason := cameraProblem(cmd); reason != "" {
			return Write{}, reason
		}
		next.Camera, next.Q, next.R, next.ZoomPct = cmd.Camera, cmd.Q, cmd.R, cmd.ZoomPct
	case CmdTableScene:
		title, body := strings.TrimSpace(cmd.Title), strings.TrimSpace(cmd.Body)
		switch {
		case cmd.Scene != domain.SceneLocal && cmd.Scene != domain.SceneWorld && cmd.Scene != domain.SceneHandout && cmd.Scene != domain.SceneTitle:
			return Write{}, "Scenes are the local map, the world map, a handout or a title card."
		case wireLen(title) > 80 || wireLen(body) > 1000:
			return Write{}, "Titles run to 80 characters and handouts to 1000."
		}
		next.Scene, next.Title, next.Body, next.MapID, shown = cmd.Scene, title, body, nil, nil
		if cmd.Scene == domain.SceneWorld {
			id, _ := uuid.Parse(cmd.MapID)
			board, err := r.store.LoadMap(context.Background(), r.st.session.CampaignID, domain.MapID(id))
			if err != nil || board.Map.Kind != domain.MapWorld {
				return Write{}, "Choose a world map for the world scene."
			}
			next.MapID, shown = &board.Map.ID, &board.Map
		}
	case CmdTableCaption:
		caption := strings.TrimSpace(cmd.Caption)
		if wireLen(caption) > maxCaption {
			return Write{}, "Captions run to 200 characters."
		}
		next.Caption = caption
	default:
		next.Blackout = cmd.On
	}
	return Write{Kind: domain.ActionTableSet, Table: &next, tableMap: shown}, ""
}

// cameraProblem says why the Table Display's camera cannot be pointed as asked, or nothing when it can.
func cameraProblem(cmd Command) string {
	switch {
	case cmd.Camera != domain.CameraFollowTurn && cmd.Camera != domain.CameraShowParty && cmd.Camera != domain.CameraFree:
		return "The camera follows the turn, shows the party, or stays where you put it."
	case cmd.ZoomPct < 50 || cmd.ZoomPct > 300:
		return "Zoom runs from 50 to 300 percent."
	}
	return ""
}

// ping flashes a hex on every screen; it changes nothing, so it is neither saved nor sequenced.
func (r *runtime) ping(req request) {
	c := Hex{Q: req.cmd.Q, R: req.cmd.R}
	for sub := range r.subs {
		r.send(sub, Update{Kind: UpdPing, Seq: r.st.session.Seq, Ping: &c})
	}
}

// shareRoll shows a roll a player made on every screen as it resolves. A DM's roll stays the DM's: it
// may be for a creature the party cannot see. Like a ping it changes nothing, so it is not sequenced.
func (r *runtime) shareRoll(id domain.RollID) {
	roll, err := r.store.Roll(context.Background(), r.st.session.CampaignID, id)
	if err != nil || roll.Status != domain.StatusResolved {
		return
	}
	// A roller who cannot be found is treated as the DM: when in doubt the roll stays off the screens.
	who, err := r.members.Member(context.Background(), r.st.session.CampaignID, roll.Roller.ID)
	if err != nil || who.DM {
		return
	}
	shown := RollShown{ID: uuid.UUID(roll.ID).String(), Roller: roll.Roller.Name, Purpose: roll.Purpose, Dice: make([]RollDie, 0, len(roll.Dice)), Modifier: roll.Total, Total: roll.Total}
	kept := keptDice(roll)
	for i, d := range roll.Dice {
		shown.Dice = append(shown.Dice, RollDie{Faces: d.Faces, Value: d.Value, Kept: kept[i]})
		if kept[i] {
			shown.Modifier -= d.Value
		}
	}
	if !shown.fits() {
		return
	}
	if r.dice != nil {
		shown.Look = r.dice.DiceLook(context.Background(), who.Subject)
	}
	r.lastRoll = &shown
	for sub := range r.subs {
		r.send(sub, Update{Kind: UpdRoll, Seq: r.st.session.Seq, Roll: &shown})
	}
}

// keptDice marks the dice of a resolved roll that count: all of a plain group, the higher or lower of
// one rolled with advantage or disadvantage.
func keptDice(roll domain.Roll) []bool {
	out := make([]bool, len(roll.Dice))
	spec, err := dice.Parse(roll.Notation)
	if err != nil {
		return out
	}
	for g, group := range spec.Groups {
		var at, faces []int
		for i, d := range roll.Dice {
			if d.Group == g {
				at, faces = append(at, i), append(faces, d.Value)
			}
		}
		for j, k := range group.Kept(faces) {
			out[at[j]] = k
		}
	}
	return out
}

// tableView shows the Table Display's settings to every audience.
func (s *state) tableView() *TableView {
	t := s.table
	v := &TableView{Camera: t.Camera, Q: t.Q, R: t.R, ZoomPct: t.ZoomPct, Scene: t.Scene, Title: t.Title, Body: t.Body, Blackout: t.Blackout, Caption: t.Caption}
	if w := s.tableMap; w != nil && t.Scene == domain.SceneWorld {
		v.WorldMap = &MapView{
			ID: uuid.UUID(w.ID).String(), Name: w.Name, Width: w.Width, Height: w.Height, HexSizePx: w.HexSize, OriginX: w.OriginX, OriginY: w.OriginY,
			ImageURL: "/api/v1/campaigns/" + w.CampaignID.String() + "/maps/" + uuid.UUID(w.ID).String() + "/image",
		}
	}
	return v
}
