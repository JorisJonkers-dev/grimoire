// Package domain holds the Library: an account's reusable entries, linked into Campaigns with a
// Campaign Override and an optional pinned Revision (ADR-0011).
package domain

import (
	"maps"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// Kinds are what a Library keeps.
func Kinds() []string {
	return []string{"creature", "npc", "location", "shop", "item", "spell", "table"}
}

// Limits on an entry's fields.
const (
	MaxFields     = 100
	MaxFieldName  = 60
	MaxFieldValue = 4000
)

// Fields are an entry's named values.
type Fields map[string]string

// Entry is a Library entry's base: its latest Revision.
type Entry struct {
	ID       uuid.UUID
	Owner    string
	Kind     string
	Name     string
	Fields   Fields
	Revision int
	// Shared marks a read-only copy in the Shared Library.
	Shared bool
	// Design is a homebrew spell's design as the Effect builder saves it, nil for any other entry.
	Design    []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SharedOwner owns the Shared Library's copies; no account signs in as it.
const SharedOwner = "grimoire:shared"

// Submission statuses: a DM's request to share an entry, waiting for an Admin or decided.
const (
	SubmissionPending  = "pending"
	SubmissionApproved = "approved"
	SubmissionDeclined = "declined"
)

// Submission is a DM's request to put one Revision of an entry in the Shared Library, and the Admin's
// review: IPClear is the Admin's check that it carries no non-SRD text, nil until reviewed.
type Submission struct {
	ID        uuid.UUID
	Entry     uuid.UUID
	Revision  int
	Draft     Draft
	Design    []byte
	Note      string
	Submitter string
	Status    string
	IPClear   *bool
	IPNote    string
	Message   string
	Reviewer  string
	Shared    *uuid.UUID
	CreatedAt time.Time
	DecidedAt *time.Time
}

// Revision is one saved version of an entry's base.
type Revision struct {
	No     int
	Name   string
	Fields Fields
	Design []byte
	Author string
	At     time.Time
}

// Use is a Campaign an entry is linked into, and the Revision it is pinned to there.
type Use struct {
	CampaignID uuid.UUID
	Campaign   string
	Pinned     *int
}

// Detail is an entry with its Revisions, newest first, and where it is used.
type Detail struct {
	Entry
	Revisions []Revision
	Uses      []Use
}

// Linked is an entry as one Campaign sees it: the base it follows (the pinned Revision, or the latest),
// the Campaign Override on top, and the two resolved together.
type Linked struct {
	Entry    Entry
	Pinned   *int
	Base     Fields
	BaseName string
	Override Fields
	// Direct marks an entry the DM linked themselves; Via names the switched-on Collections that bring it in.
	Direct bool
	Via    []string
}

// Collection is a named group of an account's entries, switched on per Campaign; On says whether it is
// on in the Campaign it was read for.
type Collection struct {
	ID          uuid.UUID
	Owner       string
	Name        string
	Description string
	Entries     []uuid.UUID
	On          bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CleanCollection checks a Collection's name and description.
func CleanCollection(name, description string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || utf8.RuneCountInString(description) > 2000 {
		return name, description, apperr.Refuse("give the Collection a name of up to 80 characters and a description of up to 2000")
	}
	return name, strings.TrimSpace(description), nil
}

// Resolved is the base with the Campaign Override on top.
func (l Linked) Resolved() Fields {
	out := Fields{}
	maps.Copy(out, l.Base)
	maps.Copy(out, l.Override)
	return out
}

// Draft is a new or changed entry.
type Draft struct {
	Kind   string
	Name   string
	Fields Fields
}

// Clean checks a Draft: a known kind, a name, and fields within the limits.
func (d Draft) Clean() (Draft, error) {
	if !slices.Contains(Kinds(), d.Kind) {
		return d, apperr.Refuse("choose a kind the Library keeps: " + strings.Join(Kinds(), ", "))
	}
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || utf8.RuneCountInString(d.Name) > 80 {
		return d, apperr.Refuse("give it a name of up to 80 characters")
	}
	f, err := CleanFields(d.Fields)
	d.Fields = f
	return d, err
}

// CleanFields checks named values: at most MaxFields, each name 1 to MaxFieldName characters and each
// value at most MaxFieldValue.
func CleanFields(in Fields) (Fields, error) {
	if len(in) > MaxFields {
		return nil, apperr.Refuse("keep it to 100 fields")
	}
	out := Fields{}
	for name, value := range in {
		name = strings.TrimSpace(name)
		if name == "" || utf8.RuneCountInString(name) > MaxFieldName || utf8.RuneCountInString(value) > MaxFieldValue {
			return nil, apperr.Refuse("name each field in up to 60 characters, with a value of up to 4000")
		}
		out[name] = value
	}
	return out, nil
}

// Proposal statuses.
const (
	StatusPending          = "pending"
	StatusChangesRequested = "changes_requested"
	StatusApproved         = "approved"
	StatusDeclined         = "declined"
)

// Review actions: what happened to a Proposal.
const (
	ReviewSubmitted        = "submitted"
	ReviewResubmitted      = "resubmitted"
	ReviewChangesRequested = "changes_requested"
	ReviewApproved         = "approved"
	ReviewDeclined         = "declined"
)

// Proposal is a Player's request that the DM accept a new entry into a Campaign, or a change to one
// the Campaign sees (Base). Entry is the Library entry an approval made or changed.
type Proposal struct {
	ID         uuid.UUID
	Campaign   uuid.UUID
	Author     string
	AuthorName string
	Draft      Draft
	Note       string
	Base       *uuid.UUID
	Status     string
	Message    string
	Entry      *uuid.UUID
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Review is one step of a Proposal's history.
type Review struct {
	No      int
	Action  string
	Message string
	By      string
	At      time.Time
}

// ProposalDetail is a Proposal with its history and, for a change, the entry as the Campaign sees it now.
type ProposalDetail struct {
	Proposal
	Reviews []Review
	Current *Linked
}

// CleanMessage checks a note or a DM's message.
func CleanMessage(s string) (string, error) {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > 2000 {
		return s, apperr.Refuse("keep the message to 2000 characters")
	}
	return s, nil
}
