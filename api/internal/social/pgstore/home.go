package pgstore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/guides"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// LiveSessions reads the Sessions under way in the Campaigns a subject is a Member of, newest first.
func (s *Store) LiveSessions(ctx context.Context, subject string) ([]domain.LiveSession, error) {
	rows, err := s.q.HomeLiveSessions(ctx, subject)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LiveSession, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.LiveSession{Campaign: r.CampaignID, CampaignName: r.Campaign, Session: r.SessionID, Number: int(r.Number), DM: r.Role == "dm"})
	}
	return out, nil
}

func counted(n int64, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.FormatInt(n, 10) + " " + many
}

// Needs reads what needs a subject before the next Session: Characters of theirs that can level up or
// have downtime days to spend, Proposals to review where they are the DM, their own Proposals sent
// back for changes, rolls waiting on them and Friend Requests.
func (s *Store) Needs(ctx context.Context, subject string) ([]domain.Need, error) {
	out := []domain.Need{}
	levels, err := s.q.HomeLevelUps(ctx, subject)
	if err != nil {
		return nil, err
	}
	for _, r := range levels {
		out = append(out, domain.Need{Kind: domain.NeedLevelUp, Campaign: r.Campaign, Title: r.Name + " can level up", Path: fmt.Sprintf("/campaigns/%s/characters/%s/level-up", r.CampaignID, r.CharacterID)})
	}
	reviews, err := s.q.HomeProposalsToReview(ctx, subject)
	if err != nil {
		return nil, err
	}
	for _, r := range reviews {
		out = append(out, domain.Need{Kind: domain.NeedProposals, Campaign: r.Campaign, Title: counted(r.Waiting, "Proposal", "Proposals") + " to review", Path: fmt.Sprintf("/campaigns/%s/proposals", r.CampaignID)})
	}
	revisions, err := s.q.HomeProposalsToRevise(ctx, subject)
	if err != nil {
		return nil, err
	}
	for _, r := range revisions {
		out = append(out, domain.Need{Kind: domain.NeedRevise, Campaign: r.Campaign, Title: "Your Proposal " + r.Name + " needs changes", Path: fmt.Sprintf("/campaigns/%s/proposals/%s", r.CampaignID, r.ProposalID)})
	}
	rolls, err := s.q.HomeRollsWaiting(ctx, subject)
	if err != nil {
		return nil, err
	}
	for _, r := range rolls {
		out = append(out, domain.Need{Kind: domain.NeedRolls, Campaign: r.Campaign, Title: counted(r.Waiting, "roll", "rolls") + " waiting on you", Path: fmt.Sprintf("/campaigns/%s/dice", r.CampaignID)})
	}
	downtime, err := s.q.HomeDowntime(ctx, subject)
	if err != nil {
		return nil, err
	}
	for _, r := range downtime {
		out = append(out, domain.Need{Kind: domain.NeedDowntime, Campaign: r.Campaign, Title: r.Name + " has " + counted(int64(r.DowntimeDays), "downtime day", "downtime days") + " to spend", Path: fmt.Sprintf("/campaigns/%s/downtime", r.CampaignID)})
	}
	requests, err := s.q.HomeFriendRequests(ctx, subject)
	if err != nil {
		return nil, err
	}
	if requests > 0 {
		out = append(out, domain.Need{Kind: domain.NeedFriendRequests, Campaign: "", Title: counted(requests, "Friend Request", "Friend Requests") + " to answer", Path: "/friends"})
	}
	return out, nil
}

// spellLevel names a spell's level the way its own page does.
func spellLevel(level int32) string {
	if level == 0 {
		return "Cantrip"
	}
	return "Level " + strconv.Itoa(int(level))
}

// literal makes what was typed match as itself in a LIKE pattern: none of it is a wildcard.
func literal(typed string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(typed)
}

// sought is what a search looks for: the text itself, to rank by, and the LIKE pattern that holds it.
type sought struct {
	needle, pattern string
	lim             int32
}

// Search finds what a subject may open whose name holds the query.
func (s *Store) Search(ctx context.Context, subject, query string, perKind int) ([]domain.Hit, error) {
	q := sought{needle: query, pattern: "%" + literal(query) + "%", lim: int32(perKind)} //nolint:gosec // a small page size
	out, err := s.searchCompendium(ctx, q)
	if err != nil {
		return nil, err
	}
	entries, err := s.q.SearchLibrary(ctx, queries.SearchLibraryParams{Subject: subject, Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		hit := domain.Hit{Group: domain.GroupLibrary, Kind: e.Kind, Title: e.Name, Preview: "Your Library · " + e.Kind, Path: "/library/" + e.ID.String()}
		if !e.Mine {
			hit.Preview, hit.Path = "Shared Library · "+e.Kind, "/shared-library"
		}
		out = append(out, hit)
	}
	mine, err := s.searchCampaigns(ctx, subject, q)
	if err != nil {
		return nil, err
	}
	out = append(out, mine...)
	friends, err := s.q.SearchFriends(ctx, queries.SearchFriendsParams{Subject: subject, Pattern: q.pattern, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, f := range friends {
		out = append(out, domain.Hit{Group: domain.GroupPeople, Kind: "friend", Title: f.Nickname, Preview: "Friend · @" + f.Username, Path: "/friends"})
	}
	return out, nil
}

func (s *Store) searchCompendium(ctx context.Context, q sought) ([]domain.Hit, error) {
	out := []domain.Hit{}
	spells, err := s.q.SearchSpells(ctx, queries.SearchSpellsParams{Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, r := range spells {
		out = append(out, domain.Hit{Group: domain.GroupCompendium, Kind: "spell", Title: r.Name, Preview: spellLevel(r.Level) + " " + strings.ToLower(r.School) + " spell", Path: "/compendium/spells/" + r.Slug})
	}
	monsters, err := s.q.SearchMonsters(ctx, queries.SearchMonstersParams{Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, r := range monsters {
		out = append(out, domain.Hit{Group: domain.GroupCompendium, Kind: "monster", Title: r.Name, Preview: r.Size + " " + strings.ToLower(r.CreatureType) + " · CR " + guides.Challenge(r.ChallengeRating), Path: "/compendium/monster/" + r.Slug})
	}
	items, err := s.q.SearchItems(ctx, queries.SearchItemsParams{Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, r := range items {
		hit := domain.Hit{Group: domain.GroupCompendium, Kind: "item", Title: r.Name, Preview: "Equipment · " + r.Category, Path: "/compendium/item/" + r.Slug}
		if r.Magic {
			hit.Kind, hit.Preview, hit.Path = "magic-item", "Magic item", "/compendium/magic-item/"+r.Slug
		}
		if r.Magic && r.Rarity != "" {
			hit.Preview += " · " + r.Rarity
		}
		out = append(out, hit)
	}
	return out, nil
}

func (s *Store) searchCampaigns(ctx context.Context, subject string, q sought) ([]domain.Hit, error) {
	out := []domain.Hit{}
	campaigns, err := s.q.SearchCampaigns(ctx, queries.SearchCampaignsParams{Subject: subject, Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, c := range campaigns {
		as := "You play in this Campaign"
		if c.Role == "dm" {
			as = "You are the DM of this Campaign"
		}
		out = append(out, domain.Hit{Group: domain.GroupCampaigns, Kind: "campaign", Title: c.Name, Preview: as, Path: "/campaigns/" + c.ID.String()})
	}
	characters, err := s.q.SearchCharacters(ctx, queries.SearchCharactersParams{Subject: subject, Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, c := range characters {
		out = append(out, domain.Hit{Group: domain.GroupCampaigns, Kind: "character", Title: c.Name, Preview: fmt.Sprintf("Level %d %s in %s", c.Level, c.ClassSlug, c.Campaign), Path: fmt.Sprintf("/campaigns/%s/characters/%s", c.CampaignID, c.ID)})
	}
	npcs, err := s.q.SearchNpcs(ctx, queries.SearchNpcsParams{Subject: subject, Pattern: q.pattern, Needle: q.needle, Lim: q.lim})
	if err != nil {
		return nil, err
	}
	for _, n := range npcs {
		out = append(out, domain.Hit{Group: domain.GroupCampaigns, Kind: "npc", Title: n.Name, Preview: "NPC in " + n.Campaign, Path: fmt.Sprintf("/campaigns/%s/npcs/%s", n.CampaignID, n.ID)})
	}
	return out, nil
}
