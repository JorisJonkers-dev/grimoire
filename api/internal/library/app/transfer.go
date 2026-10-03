package app

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/library/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/itembuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/spellbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Report is what an import added, and what needs doing by hand.
type Report struct {
	Entries     []domain.Entry
	Collections []domain.Collection
	Manual      []domain.Manual
}

// Export writes the caller's Library in Grimoire's own schema: everything, one Collection with its
// entries, or one entry.
func (s *Service) Export(ctx context.Context, c caller.Caller, collection, entry *uuid.UUID) (domain.Export, error) {
	out := domain.Export{Entries: []domain.Exported{}, Collections: []domain.ExportedCollection{}}
	var wanted func(uuid.UUID) bool
	switch {
	case entry != nil:
		e, err := s.owned(ctx, c, *entry)
		if err != nil {
			return out, err
		}
		out.Entries = append(out.Entries, exported(e))
		return out, nil
	case collection != nil:
		col, err := s.ownedCollection(ctx, c, *collection)
		if err != nil {
			return out, err
		}
		out.Collections = append(out.Collections, exportedCollection(col))
		wanted = func(id uuid.UUID) bool { return slices.Contains(col.Entries, id) }
	default:
		cols, err := s.Repo.Collections(ctx, c.Subject, nil)
		if err != nil {
			return out, err
		}
		for _, col := range cols {
			out.Collections = append(out.Collections, exportedCollection(col))
		}
		wanted = func(uuid.UUID) bool { return true }
	}
	entries, err := s.Repo.Entries(ctx, c.Subject, "")
	for _, e := range entries {
		if wanted(e.ID) {
			out.Entries = append(out.Entries, exported(e))
		}
	}
	return out, err
}

func exported(e domain.Entry) domain.Exported {
	return domain.Exported{Key: e.ID.String(), Kind: e.Kind, Name: e.Name, Fields: e.Fields, Design: e.Design}
}

func exportedCollection(c domain.Collection) domain.ExportedCollection {
	out := domain.ExportedCollection{Name: c.Name, Description: c.Description, Entries: []string{}}
	for _, e := range c.Entries {
		out.Entries = append(out.Entries, e.String())
	}
	return out
}

// Import adds an export's entries and Collections to the caller's Library as new ones, each entry its
// first Revision. What cannot be taken is reported as Manual, never refused whole.
func (s *Service) Import(ctx context.Context, c caller.Caller, in []domain.Incoming, cols []domain.ExportedCollection) (Report, error) {
	entries, collections, manual := domain.PlanImport(in, cols)
	for i, x := range entries {
		if reason := s.checkDesign(ctx, x); reason != "" {
			manual = append(manual, domain.Manual{Where: fmt.Sprintf("entries %q design", x.Name), Reason: reason})
			entries[i].Design = nil
		}
	}
	r := Report{Entries: []domain.Entry{}, Collections: []domain.Collection{}, Manual: manual}
	now := s.Now()
	err := s.Repo.InTx(ctx, func(repo Repository) error {
		ids := map[string]uuid.UUID{}
		for _, x := range entries {
			e, err := s.importEntry(ctx, repo, c, x, now)
			if err != nil {
				return err
			}
			ids[x.Key] = e.ID
			r.Entries = append(r.Entries, e)
		}
		for _, x := range collections {
			col, err := importCollection(ctx, repo, c, x, ids, now)
			if err != nil {
				return err
			}
			r.Collections = append(r.Collections, col)
		}
		return nil
	})
	return r, err
}

// importEntry adds one imported entry as its first Revision.
func (s *Service) importEntry(ctx context.Context, repo Repository, c caller.Caller, x domain.Exported, now time.Time) (domain.Entry, error) {
	e := domain.Entry{ID: uuid.New(), Owner: c.Subject, Kind: x.Kind, Name: x.Name, Fields: x.Fields, Design: x.Design, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := repo.InsertEntry(ctx, e); err != nil {
		return e, err
	}
	return e, repo.InsertRevision(ctx, e.ID, domain.Revision{No: 1, Name: e.Name, Fields: e.Fields, Design: e.Design, Author: c.Subject, At: now})
}

// importCollection adds one imported Collection holding the imported entries it names.
func importCollection(ctx context.Context, repo Repository, c caller.Caller, x domain.ExportedCollection, ids map[string]uuid.UUID, now time.Time) (domain.Collection, error) {
	col := domain.Collection{ID: uuid.New(), Owner: c.Subject, Name: x.Name, Description: x.Description, Entries: []uuid.UUID{}, CreatedAt: now, UpdatedAt: now}
	for _, key := range x.Entries {
		col.Entries = append(col.Entries, ids[key])
	}
	if err := repo.InsertCollection(ctx, col); err != nil {
		return col, err
	}
	return col, repo.UpdateCollection(ctx, col)
}

// checkDesign says why an imported design cannot be built, or nothing when it can or there is none.
func (s *Service) checkDesign(ctx context.Context, x domain.Exported) string {
	if x.Design == nil {
		return ""
	}
	spell := func(d spellbuild.Design) error {
		_, err := s.build(ctx, uuid.Nil, x.Name, d)
		return err
	}
	checks := map[string]func([]byte) error{
		"spell":      func(raw []byte) error { return checked(raw, spell) },
		"item":       func(raw []byte) error { return checked(raw, itembuild.Check) },
		"subclass":   func(raw []byte) error { return checked(raw, checkSubclass) },
		"class":      func(raw []byte) error { return checked(raw, checkClass) },
		"species":    func(raw []byte) error { return checked(raw, checkSpecies) },
		"background": func(raw []byte) error { return checked(raw, checkBackground) },
		"feat":       func(raw []byte) error { return checked(raw, checkFeat) },
		"condition":  func(raw []byte) error { return checked(raw, checkCondition) },
		"creature":   func(raw []byte) error { return checked(raw, checkMonster) },
	}
	if err := checks[x.Kind](x.Design); err != nil {
		return err.Error()
	}
	return ""
}

// checked reads a design and checks it.
func checked[D any](raw []byte, check func(D) error) error {
	var d D
	if err := json.Unmarshal(raw, &d); err != nil {
		return err
	}
	return check(d)
}
