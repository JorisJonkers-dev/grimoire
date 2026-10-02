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
		ID: r.ID, Owner: r.OwnerSubject, Kind: r.Kind, Name: r.Name, Fields: decode(r.Fields), Revision: int(r.Revision),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// InsertEntry adds an entry.
func (s *Store) InsertEntry(ctx context.Context, e domain.Entry) error {
	return s.q.InsertLibraryEntry(ctx, queries.InsertLibraryEntryParams{
		ID: e.ID, OwnerSubject: e.Owner, Kind: e.Kind, Name: e.Name, Fields: encode(e.Fields), Now: e.CreatedAt,
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

// SetOverride replaces a link's Campaign Override.
func (s *Store) SetOverride(ctx context.Context, campaign, entry uuid.UUID, override domain.Fields, now time.Time) (bool, error) {
	n, err := s.q.SetLibraryOverride(ctx, queries.SetLibraryOverrideParams{CampaignID: campaign, EntryID: entry, Override: encode(override), Now: now})
	return n > 0, err
}

// Pin holds a link to a Revision, or nil for the latest.
func (s *Store) Pin(ctx context.Context, campaign, entry uuid.UUID, revision *int, now time.Time) (bool, error) {
	p := queries.PinLibraryRevisionParams{CampaignID: campaign, EntryID: entry, Now: now}
	if revision != nil {
		p.PinnedRevision = pgtype.Int4{Int32: int32(*revision), Valid: true} //nolint:gosec // checked against stored numbers
	}
	n, err := s.q.PinLibraryRevision(ctx, p)
	return n > 0, err
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
		})
		l := domain.Linked{Entry: e, Pinned: pinned(r.PinnedRevision), Base: e.Fields, BaseName: e.Name, Override: decode(r.Override)}
		if l.Pinned != nil {
			l.Base, l.BaseName = decode(r.PinnedFields), r.PinnedName.String
		}
		out = append(out, l)
	}
	return out, err
}
