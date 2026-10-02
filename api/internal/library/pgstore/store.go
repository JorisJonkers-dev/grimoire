// Package pgstore is the Postgres adapter for the Library.
package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// Store implements app.Repository.
type Store struct {
	pool *pgxpool.Pool
	q    *queries.Queries
	wrap func(queries.DBTX) queries.DBTX
}

var _ app.Repository = (*Store)(nil)

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return newWrapped(pool, func(db queries.DBTX) queries.DBTX { return db })
}

func newWrapped(pool *pgxpool.Pool, wrap func(queries.DBTX) queries.DBTX) *Store {
	return &Store{pool: pool, q: queries.New(wrap(pool)), wrap: wrap}
}

// InTx runs fn against a Store bound to one transaction.
func (s *Store) InTx(ctx context.Context, fn func(app.Repository) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, q: queries.New(s.wrap(tx)), wrap: s.wrap})
	})
}

func encode(f domain.Fields) []byte {
	if f == nil {
		f = domain.Fields{}
	}
	raw, _ := json.Marshal(f) //nolint:errchkjson // a map of strings always marshals
	return raw
}

func decode(raw []byte) domain.Fields {
	out := domain.Fields{}
	_ = json.Unmarshal(raw, &out) // the column only ever holds an object of strings
	return out
}

func entryOf(r queries.LibraryEntry) domain.Entry {
	return domain.Entry{
		ID: r.ID, Owner: r.OwnerSubject, Kind: r.Kind, Name: r.Name, Fields: decode(r.Fields), Revision: int(r.Revision), Shared: r.Shared,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// InsertEntry adds an entry.
func (s *Store) InsertEntry(ctx context.Context, e domain.Entry) error {
	return s.q.InsertLibraryEntry(ctx, queries.InsertLibraryEntryParams{
		ID: e.ID, OwnerSubject: e.Owner, Kind: e.Kind, Name: e.Name, Fields: encode(e.Fields), Now: e.CreatedAt, Shared: e.Shared,
	})
}

// UpdateEntry saves a new base and returns its Revision number.
func (s *Store) UpdateEntry(ctx context.Context, id uuid.UUID, name string, fields domain.Fields, now time.Time) (int, error) {
	no, err := s.q.UpdateLibraryEntry(ctx, queries.UpdateLibraryEntryParams{ID: id, Name: name, Fields: encode(fields), Now: now})
	return int(no), err
}

// InsertRevision records a Revision of an entry's base.
func (s *Store) InsertRevision(ctx context.Context, entry uuid.UUID, r domain.Revision) error {
	return s.q.InsertLibraryRevision(ctx, queries.InsertLibraryRevisionParams{
		EntryID: entry, No: int32(r.No), Name: r.Name, Fields: encode(r.Fields), AuthorSubject: r.Author, Now: r.At, //nolint:gosec // revision numbers are small
	})
}

// Entry reads one entry.
func (s *Store) Entry(ctx context.Context, id uuid.UUID) (domain.Entry, error) {
	r, err := s.q.LibraryEntry(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Entry{}, apperr.ErrNotFound
	}
	return entryOf(r), err
}

// Entries lists an owner's entries, of one kind when kind is set.
func (s *Store) Entries(ctx context.Context, owner, kind string) ([]domain.Entry, error) {
	rows, err := s.q.LibraryEntries(ctx, queries.LibraryEntriesParams{OwnerSubject: owner, Kind: pgtype.Text{String: kind, Valid: kind != ""}})
	out := make([]domain.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, entryOf(r))
	}
	return out, err
}

// Revisions lists an entry's Revisions, newest first.
func (s *Store) Revisions(ctx context.Context, entry uuid.UUID) ([]domain.Revision, error) {
	rows, err := s.q.LibraryRevisions(ctx, entry)
	out := make([]domain.Revision, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Revision{No: int(r.No), Name: r.Name, Fields: decode(r.Fields), Author: r.AuthorSubject, At: r.CreatedAt})
	}
	return out, err
}

// RevisionExists reports whether an entry has a Revision.
func (s *Store) RevisionExists(ctx context.Context, entry uuid.UUID, no int) (bool, error) {
	return s.q.LibraryRevisionExists(ctx, queries.LibraryRevisionExistsParams{EntryID: entry, No: int32(no)}) //nolint:gosec // checked against stored numbers
}

func pinned(p pgtype.Int4) *int {
	if !p.Valid {
		return nil
	}
	n := int(p.Int32)
	return &n
}

// Uses lists the Campaigns an entry is linked into.
func (s *Store) Uses(ctx context.Context, entry uuid.UUID) ([]domain.Use, error) {
	rows, err := s.q.LibraryEntryUses(ctx, entry)
	out := make([]domain.Use, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Use{CampaignID: r.ID, Campaign: r.Name, Pinned: pinned(r.PinnedRevision)})
	}
	return out, err
}

// Link links an entry into a Campaign.
func (s *Store) Link(ctx context.Context, campaign, entry uuid.UUID, now time.Time) error {
	return s.q.LinkLibraryEntry(ctx, queries.LinkLibraryEntryParams{CampaignID: campaign, EntryID: entry, Now: now})
}

// Unlink takes an entry out of a Campaign.
func (s *Store) Unlink(ctx context.Context, campaign, entry uuid.UUID) (bool, error) {
	n, err := s.q.UnlinkLibraryEntry(ctx, queries.UnlinkLibraryEntryParams{CampaignID: campaign, EntryID: entry})
	return n > 0, err
}

// SetOverride replaces the Campaign Override of an entry the Campaign sees.
func (s *Store) SetOverride(ctx context.Context, campaign, entry uuid.UUID, override domain.Fields, now time.Time) error {
	return s.q.SetLibraryOverride(ctx, queries.SetLibraryOverrideParams{CampaignID: campaign, EntryID: entry, Override: encode(override), Now: now})
}

// Pin holds a Campaign to a Revision of an entry it sees, or nil for the latest.
func (s *Store) Pin(ctx context.Context, campaign, entry uuid.UUID, revision *int, now time.Time) error {
	p := queries.PinLibraryRevisionParams{CampaignID: campaign, EntryID: entry, Now: now}
	if revision != nil {
		p.PinnedRevision = pgtype.Int4{Int32: int32(*revision), Valid: true} //nolint:gosec // checked against stored numbers
	}
	return s.q.PinLibraryRevision(ctx, p)
}

// Linked reads a Campaign's links as it sees them, or one when entry is set.
func (s *Store) Linked(ctx context.Context, campaign uuid.UUID, entry *uuid.UUID) ([]domain.Linked, error) {
	p := queries.CampaignLibraryLinksParams{CampaignID: campaign}
	if entry != nil {
		p.EntryID = pgtype.UUID{Bytes: *entry, Valid: true}
	}
	rows, err := s.q.CampaignLibraryLinks(ctx, p)
	out := make([]domain.Linked, 0, len(rows))
	for _, r := range rows {
		e := entryOf(queries.LibraryEntry{
			ID: r.ID, OwnerSubject: r.OwnerSubject, Kind: r.Kind, Name: r.Name, Fields: r.Fields, Revision: r.Revision, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			Shared: r.Shared,
		})
		l := domain.Linked{Entry: e, Pinned: pinned(r.PinnedRevision), Base: e.Fields, BaseName: e.Name, Override: decode(r.Override), Direct: r.Direct, Via: r.Via}
		if l.Pinned != nil {
			l.Base, l.BaseName = decode(r.PinnedFields), r.PinnedName.String
		}
		out = append(out, l)
	}
	return out, err
}

func collectionOf(id uuid.UUID, owner, name, description string, created, updated time.Time, entries []uuid.UUID, on bool) domain.Collection {
	return domain.Collection{ID: id, Owner: owner, Name: name, Description: description, Entries: entries, On: on, CreatedAt: created, UpdatedAt: updated}
}

// InsertCollection adds an empty Collection.
func (s *Store) InsertCollection(ctx context.Context, c domain.Collection) error {
	return s.q.InsertLibraryCollection(ctx, queries.InsertLibraryCollectionParams{
		ID: c.ID, OwnerSubject: c.Owner, Name: c.Name, Description: c.Description, Now: c.CreatedAt,
	})
}

// UpdateCollection saves a Collection's name, description and entries.
func (s *Store) UpdateCollection(ctx context.Context, c domain.Collection) error {
	if err := s.q.UpdateLibraryCollection(ctx, queries.UpdateLibraryCollectionParams{ID: c.ID, Name: c.Name, Description: c.Description, Now: c.UpdatedAt}); err != nil {
		return err
	}
	if err := s.q.ClearLibraryCollection(ctx, c.ID); err != nil {
		return err
	}
	for _, e := range c.Entries {
		if err := s.q.AddToLibraryCollection(ctx, queries.AddToLibraryCollectionParams{CollectionID: c.ID, EntryID: e}); err != nil {
			return err
		}
	}
	return nil
}

// Collection reads one Collection.
func (s *Store) Collection(ctx context.Context, id uuid.UUID) (domain.Collection, error) {
	r, err := s.q.LibraryCollection(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Collection{}, apperr.ErrNotFound
	}
	return collectionOf(r.ID, r.OwnerSubject, r.Name, r.Description, r.CreatedAt, r.UpdatedAt, r.EntryIds, false), err
}

// Collections lists an owner's Collections and, with a Campaign, every other one switched on there.
func (s *Store) Collections(ctx context.Context, owner string, campaign *uuid.UUID) ([]domain.Collection, error) {
	p := queries.CampaignLibraryCollectionsParams{OwnerSubject: owner}
	if campaign != nil {
		p.CampaignID = pgtype.UUID{Bytes: *campaign, Valid: true}
	}
	rows, err := s.q.CampaignLibraryCollections(ctx, p)
	out := make([]domain.Collection, 0, len(rows))
	for _, r := range rows {
		out = append(out, collectionOf(r.ID, r.OwnerSubject, r.Name, r.Description, r.CreatedAt, r.UpdatedAt, r.EntryIds, r.SwitchedOn))
	}
	return out, err
}

// Switch turns a Collection on or off in a Campaign.
func (s *Store) Switch(ctx context.Context, campaign, collection uuid.UUID, on bool, now time.Time) error {
	if on {
		return s.q.SwitchOnLibraryCollection(ctx, queries.SwitchOnLibraryCollectionParams{CampaignID: campaign, CollectionID: collection, Now: now})
	}
	return s.q.SwitchOffLibraryCollection(ctx, queries.SwitchOffLibraryCollectionParams{CampaignID: campaign, CollectionID: collection})
}

// AddToCollection puts an entry in a Collection.
func (s *Store) AddToCollection(ctx context.Context, collection, entry uuid.UUID) error {
	return s.q.AddToLibraryCollection(ctx, queries.AddToLibraryCollectionParams{CollectionID: collection, EntryID: entry})
}

// CampaignHome reads the Campaign Collection's id.
func (s *Store) CampaignHome(ctx context.Context, campaign uuid.UUID) (uuid.UUID, error) {
	id, err := s.q.CampaignHome(ctx, campaign)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, apperr.ErrNotFound
	}
	return id, err
}

// InsertCampaignHome makes a Collection the Campaign Collection.
func (s *Store) InsertCampaignHome(ctx context.Context, campaign, collection uuid.UUID) error {
	return s.q.InsertCampaignHome(ctx, queries.InsertCampaignHomeParams{CampaignID: campaign, CollectionID: collection})
}

func optUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func uuidOf(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

// InsertProposal adds a pending Proposal.
func (s *Store) InsertProposal(ctx context.Context, p domain.Proposal) error {
	return s.q.InsertProposal(ctx, queries.InsertProposalParams{
		ID: p.ID, CampaignID: p.Campaign, AuthorSubject: p.Author, AuthorName: p.AuthorName, Kind: p.Draft.Kind, Name: p.Draft.Name,
		Fields: encode(p.Draft.Fields), Note: p.Note, BaseEntryID: optUUID(p.Base), Now: p.CreatedAt,
	})
}

// UpdateProposal saves a Proposal's draft, status, message and resulting entry.
func (s *Store) UpdateProposal(ctx context.Context, p domain.Proposal) error {
	return s.q.UpdateProposal(ctx, queries.UpdateProposalParams{
		ID: p.ID, Name: p.Draft.Name, Fields: encode(p.Draft.Fields), Note: p.Note, Status: p.Status, Message: p.Message,
		EntryID: optUUID(p.Entry), Now: p.UpdatedAt,
	})
}

func proposalOf(r queries.LibraryProposal) domain.Proposal {
	return domain.Proposal{
		ID: r.ID, Campaign: r.CampaignID, Author: r.AuthorSubject, AuthorName: r.AuthorName,
		Draft: domain.Draft{Kind: r.Kind, Name: r.Name, Fields: decode(r.Fields)}, Note: r.Note, Base: uuidOf(r.BaseEntryID),
		Status: r.Status, Message: r.Message, Entry: uuidOf(r.EntryID), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// Proposal reads one Proposal.
func (s *Store) Proposal(ctx context.Context, id uuid.UUID) (domain.Proposal, error) {
	r, err := s.q.Proposal(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Proposal{}, apperr.ErrNotFound
	}
	return proposalOf(r), err
}

// Proposals lists a Campaign's Proposals, newest first; only one author's when author is set.
func (s *Store) Proposals(ctx context.Context, campaign uuid.UUID, author string) ([]domain.Proposal, error) {
	rows, err := s.q.CampaignProposals(ctx, queries.CampaignProposalsParams{CampaignID: campaign, AuthorSubject: pgtype.Text{String: author, Valid: author != ""}})
	out := make([]domain.Proposal, 0, len(rows))
	for _, r := range rows {
		out = append(out, proposalOf(r))
	}
	return out, err
}

// InsertReview records the next step of a Proposal's history.
func (s *Store) InsertReview(ctx context.Context, proposal uuid.UUID, r domain.Review) error {
	return s.q.InsertProposalReview(ctx, queries.InsertProposalReviewParams{ProposalID: proposal, Action: r.Action, Message: r.Message, ByName: r.By, Now: r.At})
}

// Reviews lists a Proposal's history in order.
func (s *Store) Reviews(ctx context.Context, proposal uuid.UUID) ([]domain.Review, error) {
	rows, err := s.q.ProposalReviews(ctx, proposal)
	out := make([]domain.Review, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Review{No: int(r.No), Action: r.Action, Message: r.Message, By: r.ByName, At: r.CreatedAt})
	}
	return out, err
}

// InsertSubmission records a DM's request to share an entry.
func (s *Store) InsertSubmission(ctx context.Context, x domain.Submission) error {
	return s.q.InsertSharedSubmission(ctx, queries.InsertSharedSubmissionParams{
		ID: x.ID, EntryID: x.Entry, Revision: int32(x.Revision), Kind: x.Draft.Kind, Name: x.Draft.Name, Fields: encode(x.Draft.Fields), //nolint:gosec // revision numbers are small
		Note: x.Note, SubmitterSubject: x.Submitter, Now: x.CreatedAt,
	})
}

// DecideSubmission records an Admin's review.
func (s *Store) DecideSubmission(ctx context.Context, x domain.Submission) error {
	p := queries.DecideSharedSubmissionParams{
		ID: x.ID, Status: x.Status, IpNote: x.IPNote, Message: x.Message, ReviewerSubject: pgtype.Text{String: x.Reviewer, Valid: true},
		SharedEntryID: optUUID(x.Shared),
	}
	if x.IPClear != nil {
		p.IpClear = pgtype.Bool{Bool: *x.IPClear, Valid: true}
	}
	if x.DecidedAt != nil {
		p.Now = pgtype.Timestamptz{Time: *x.DecidedAt, Valid: true}
	}
	return s.q.DecideSharedSubmission(ctx, p)
}

func submissionOf(r queries.LibrarySharedSubmission) domain.Submission {
	x := domain.Submission{
		ID: r.ID, Entry: r.EntryID, Revision: int(r.Revision), Draft: domain.Draft{Kind: r.Kind, Name: r.Name, Fields: decode(r.Fields)},
		Note: r.Note, Submitter: r.SubmitterSubject, Status: r.Status, IPNote: r.IpNote, Message: r.Message, Reviewer: r.ReviewerSubject.String,
		Shared: uuidOf(r.SharedEntryID), CreatedAt: r.CreatedAt,
	}
	if r.IpClear.Valid {
		x.IPClear = &r.IpClear.Bool
	}
	if r.DecidedAt.Valid {
		x.DecidedAt = &r.DecidedAt.Time
	}
	return x
}

// Submission reads one request to share.
func (s *Store) Submission(ctx context.Context, id uuid.UUID) (domain.Submission, error) {
	r, err := s.q.SharedSubmission(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Submission{}, apperr.ErrNotFound
	}
	return submissionOf(r), err
}

// Submissions lists requests to share, pending first; only one submitter's when submitter is set.
func (s *Store) Submissions(ctx context.Context, submitter string) ([]domain.Submission, error) {
	rows, err := s.q.SharedSubmissions(ctx, pgtype.Text{String: submitter, Valid: submitter != ""})
	out := make([]domain.Submission, 0, len(rows))
	for _, r := range rows {
		out = append(out, submissionOf(r))
	}
	return out, err
}

// PendingSubmission reports whether an entry already waits for an Admin.
func (s *Store) PendingSubmission(ctx context.Context, entry uuid.UUID) (bool, error) {
	return s.q.PendingSharedSubmission(ctx, entry)
}
