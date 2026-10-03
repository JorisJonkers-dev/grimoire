package domain

import "github.com/google/uuid"

// LiveSession is a Session under way in a Campaign the caller is a Member of.
type LiveSession struct {
	Campaign     uuid.UUID
	CampaignName string
	Session      uuid.UUID
	Number       int
	DM           bool
}

// Kinds of thing that need the caller before the next Session.
const (
	NeedLevelUp        = "level_up"
	NeedProposals      = "proposals"
	NeedRevise         = "revise_proposal"
	NeedRolls          = "rolls"
	NeedDowntime       = "downtime"
	NeedFriendRequests = "friend_requests"
)

// Need is one thing that needs the caller, with where to deal with it. Campaign is the Campaign's
// name, empty for what belongs to no Campaign.
type Need struct {
	Kind     string
	Campaign string
	Title    string
	Path     string
}

// Dashboard is what the caller sees first: the Sessions under way, and what needs them.
type Dashboard struct {
	Live  []LiveSession
	Needs []Need
}

// Groups of search results.
const (
	GroupCompendium = "compendium"
	GroupLibrary    = "library"
	GroupCampaigns  = "campaigns"
	GroupPeople     = "people"
)

// Hit is one search result the caller may open, with a line that previews it.
type Hit struct {
	Group   string
	Kind    string
	Title   string
	Preview string
	Path    string
}
