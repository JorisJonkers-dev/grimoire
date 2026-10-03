package pgstore

import (
	"github.com/google/uuid"

	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaigndomain "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// TrackEvents tells the Sessions under way in a Campaign that a Track's score crossed thresholds.
type TrackEvents struct {
	Hub *live.Hub
}

// TrackCrossed passes the thresholds crossed to the Campaign's live Sessions.
func (e TrackEvents) TrackCrossed(campaign campaigndomain.CampaignID, by campaigndomain.Member, c caller.Caller, crossed []campaignapp.TrackCrossing) {
	out := make([]live.Crossing, 0, len(crossed))
	for _, cr := range crossed {
		// Named after its Track alone: a threshold, its label with it, is the DM's to know, and what a
		// Session is told is shown to the party.
		row := live.Crossing{Name: cr.Track, Character: nil, Effect: cr.Threshold.Effect, Table: cr.Threshold.RollTable}
		if cr.Character != nil {
			character := uuid.UUID(*cr.Character)
			row.Character = &character
		}
		out = append(out, row)
	}
	e.Hub.TrackCrossed(uuid.UUID(campaign), member(by), c, out)
}
