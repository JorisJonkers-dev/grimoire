package app

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/tracks"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Limits on Tracks.
const (
	MaxTracks          = 20
	MaxTrackThresholds = 20
	MaxTrackScore      = 1000
)

// TrackRepository keeps a Campaign's Tracks and where each stands.
type TrackRepository interface {
	Membership(ctx context.Context, id domain.CampaignID, subject string) (domain.Member, error)
	Tracks(ctx context.Context, id domain.CampaignID) ([]domain.Track, error)
	TrackValues(ctx context.Context, id domain.CampaignID) ([]domain.TrackValue, error)
	TrackCharacters(ctx context.Context, id domain.CampaignID) ([]domain.TrackCharacter, error)
	CampaignRollTables(ctx context.Context, id domain.CampaignID) ([]domain.RollTableRef, error)
	// InsertTrack adds a Track with its thresholds, all or nothing.
	InsertTrack(ctx context.Context, t domain.Track) error
	// DeleteTrack reports ErrNotFound for a Track the Campaign does not have.
	DeleteTrack(ctx context.Context, id domain.CampaignID, track domain.TrackID) error
	SetTrackValue(ctx context.Context, v domain.TrackValue) error
}

// TrackCrossing is a threshold a score crossed, for a Character or for the party.
type TrackCrossing struct {
	Track     string
	Character *domain.CharacterID
	Threshold domain.TrackThreshold
}

// TrackEvents is told when a Track's score crosses thresholds, so that a Session under way can apply
// what they trigger.
type TrackEvents interface {
	TrackCrossed(campaign domain.CampaignID, by domain.Member, c caller.Caller, crossed []TrackCrossing)
}

// Tracks runs the Track use cases. The DM keeps the Tracks and moves their scores; a Player sees the
// Tracks and where their own Characters and the party stand, and nothing of the thresholds.
type Tracks struct {
	Repo   TrackRepository
	Events TrackEvents
	Now    func() time.Time
}

// TrackStanding is where a Track stands for one Character, or for the party when Character is nil.
type TrackStanding struct {
	Character *domain.CharacterID
	Name      string
	Value     int
}

// TrackView is a Track as the caller may see it, with where it stands.
type TrackView struct {
	Track     domain.Track
	Standings []TrackStanding
}

// TracksView is a Campaign's Tracks as the caller may see them.
type TracksView struct {
	DM     bool
	Tracks []TrackView
}

// scoreOf is where a Track stands for a Character or the party: its start until it has moved.
func scoreOf(t domain.Track, values []domain.TrackValue, character *domain.CharacterID) int {
	for _, v := range values {
		same := (v.Character == nil && character == nil) || (v.Character != nil && character != nil && *v.Character == *character)
		if v.Track == t.ID && same {
			return v.Value
		}
	}
	return t.Start
}

// List shows a Member the Campaign's Tracks. The DM sees every Character's score and the thresholds; a
// Player sees the party's scores and their own Characters', and no threshold.
func (s *Tracks) List(ctx context.Context, c caller.Caller, id domain.CampaignID) (TracksView, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err != nil {
		return TracksView{}, err
	}
	list, err := s.Repo.Tracks(ctx, id)
	if err != nil {
		return TracksView{}, err
	}
	values, err := s.Repo.TrackValues(ctx, id)
	if err != nil {
		return TracksView{}, err
	}
	characters, err := s.Repo.TrackCharacters(ctx, id)
	if err != nil {
		return TracksView{}, err
	}
	v := TracksView{DM: me.Role == domain.RoleDM, Tracks: make([]TrackView, 0, len(list))}
	for _, t := range list {
		row := TrackView{Track: t, Standings: standings(t, values, characters, me)}
		if !v.DM {
			row.Track.Thresholds = nil
		}
		v.Tracks = append(v.Tracks, row)
	}
	return v, nil
}

// standings is where a Track stands as a Member may see it: the party's score, or the score of every
// Character for the DM and of their own Characters for a Player.
func standings(t domain.Track, values []domain.TrackValue, characters []domain.TrackCharacter, me domain.Member) []TrackStanding {
	if t.Scope == domain.TrackParty {
		return []TrackStanding{{Character: nil, Name: "", Value: scoreOf(t, values, nil)}}
	}
	out := []TrackStanding{}
	for _, ch := range characters {
		if me.Role == domain.RoleDM || ch.Owner == me.ID {
			out = append(out, TrackStanding{Character: &ch.ID, Name: ch.Name, Value: scoreOf(t, values, &ch.ID)})
		}
	}
	return out
}

// TrackInput is a new Track.
type TrackInput struct {
	Name       string
	Scope      string
	Min        int
	Max        int
	Start      int
	Thresholds []domain.TrackThreshold
}

func (s *Tracks) dm(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Member, error) {
	me, err := s.Repo.Membership(ctx, id, c.Subject)
	if err == nil && me.Role != domain.RoleDM {
		err = domain.ErrForbidden
	}
	return me, err
}

func checkTrack(in TrackInput, name string) error {
	bounded := in.Min >= -MaxTrackScore && in.Max <= MaxTrackScore && in.Min < in.Max && in.Start >= in.Min && in.Start <= in.Max
	if name == "" || utf8.RuneCountInString(name) > 80 || (in.Scope != domain.TrackPerCharacter && in.Scope != domain.TrackParty) || !bounded || len(in.Thresholds) > MaxTrackThresholds {
		return domain.ErrInvalid
	}
	for _, th := range in.Thresholds {
		label := strings.TrimSpace(th.Label)
		if label == "" || utf8.RuneCountInString(label) > 80 || th.At < in.Min || th.At > in.Max || len(th.Effect) > 80 || (th.Effect != "" && th.RollTable != nil) {
			return domain.ErrInvalid
		}
	}
	return nil
}

// Create adds a Track with its thresholds. A threshold may apply an Effect or roll on a Roll Table the
// Campaign sees, never both. DM only.
func (s *Tracks) Create(ctx context.Context, c caller.Caller, id domain.CampaignID, in TrackInput) (domain.Track, error) {
	if _, err := s.dm(ctx, c, id); err != nil {
		return domain.Track{}, err
	}
	name := strings.TrimSpace(in.Name)
	if err := checkTrack(in, name); err != nil {
		return domain.Track{}, err
	}
	t := domain.Track{ID: uuid.New(), CampaignID: id, Name: name, Scope: in.Scope, Min: in.Min, Max: in.Max, Start: in.Start, Thresholds: make([]domain.TrackThreshold, 0, len(in.Thresholds)), CreatedAt: s.Now()}
	tables, err := s.Repo.CampaignRollTables(ctx, id)
	if err != nil {
		return domain.Track{}, err
	}
	for _, th := range in.Thresholds {
		if th.RollTable != nil && !slices.ContainsFunc(tables, func(x domain.RollTableRef) bool { return x.ID == *th.RollTable }) {
			return domain.Track{}, refuse("that Roll Table is not one this Campaign sees")
		}
		t.Thresholds = append(t.Thresholds, domain.TrackThreshold{At: th.At, Rising: th.Rising, Label: strings.TrimSpace(th.Label), Effect: strings.TrimSpace(th.Effect), RollTable: th.RollTable})
	}
	existing, err := s.Repo.Tracks(ctx, id)
	if err != nil {
		return domain.Track{}, err
	}
	if len(existing) >= MaxTracks {
		return domain.Track{}, refuse("a Campaign keeps up to 20 Tracks")
	}
	return t, s.Repo.InsertTrack(ctx, t)
}

// Delete removes a Track with its thresholds and scores. DM only.
func (s *Tracks) Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, track domain.TrackID) error {
	if _, err := s.dm(ctx, c, id); err != nil {
		return err
	}
	return s.Repo.DeleteTrack(ctx, id, track)
}

// moved finds the Track whose score is to move, for a Character of the Campaign when it is kept for
// each Character and for nobody when it is the party's.
func (s *Tracks) moved(ctx context.Context, id domain.CampaignID, track domain.TrackID, character *domain.CharacterID) (domain.Track, error) {
	list, err := s.Repo.Tracks(ctx, id)
	if err != nil {
		return domain.Track{}, err
	}
	i := slices.IndexFunc(list, func(t domain.Track) bool { return t.ID == track })
	if i < 0 {
		return domain.Track{}, domain.ErrNotFound
	}
	if (list[i].Scope == domain.TrackParty) != (character == nil) {
		return domain.Track{}, domain.ErrInvalid
	}
	if character == nil {
		return list[i], nil
	}
	characters, err := s.Repo.TrackCharacters(ctx, id)
	if err != nil {
		return domain.Track{}, err
	}
	if !slices.ContainsFunc(characters, func(ch domain.TrackCharacter) bool { return ch.ID == *character }) {
		return domain.Track{}, domain.ErrNotFound
	}
	return list[i], nil
}

// TrackAdjusted is where a score stands after it was moved, and the thresholds it crossed on the way.
type TrackAdjusted struct {
	Value   int
	Crossed []domain.TrackThreshold
}

// Adjust moves a Track's score for one Character, or for the party, within the Track's bounds. The
// thresholds it crosses are told to the Session under way, which applies what they trigger. DM only.
func (s *Tracks) Adjust(ctx context.Context, c caller.Caller, id domain.CampaignID, track domain.TrackID, character *domain.CharacterID, delta int) (TrackAdjusted, error) {
	me, err := s.dm(ctx, c, id)
	if err != nil {
		return TrackAdjusted{}, err
	}
	if delta == 0 || delta < -2*MaxTrackScore || delta > 2*MaxTrackScore {
		return TrackAdjusted{}, domain.ErrInvalid
	}
	t, err := s.moved(ctx, id, track, character)
	if err != nil {
		return TrackAdjusted{}, err
	}
	values, err := s.Repo.TrackValues(ctx, id)
	if err != nil {
		return TrackAdjusted{}, err
	}
	from := scoreOf(t, values, character)
	to := tracks.Clamp(t.Min, t.Max, from+delta)
	if err := s.Repo.SetTrackValue(ctx, domain.TrackValue{Track: t.ID, Character: character, Value: to}); err != nil {
		return TrackAdjusted{}, err
	}
	out := TrackAdjusted{Value: to, Crossed: []domain.TrackThreshold{}}
	rules := make([]tracks.Threshold, 0, len(t.Thresholds))
	for _, th := range t.Thresholds {
		rules = append(rules, tracks.Threshold{At: th.At, Rising: th.Rising})
	}
	var crossed []TrackCrossing
	for _, n := range tracks.Crossed(rules, from, to) {
		out.Crossed = append(out.Crossed, t.Thresholds[n])
		crossed = append(crossed, TrackCrossing{Track: t.Name, Character: character, Threshold: t.Thresholds[n]})
	}
	if len(crossed) > 0 {
		s.Events.TrackCrossed(id, me, c, crossed)
	}
	return out, nil
}
