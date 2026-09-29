// Package pgstore is the Postgres adapter for the compendium.
package pgstore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// Store reads and imports compendium data.
type Store struct {
	pool *pgxpool.Pool
	q    *queries.Queries
}

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: queries.New(pool)}
}

// Import loads the snapshot in one transaction unless this exact snapshot was imported last.
func (s *Store) Import(ctx context.Context, snap snapshot.Snapshot, hash string) (bool, error) {
	last, err := s.q.LatestSnapshotHash(ctx)
	if err != nil {
		return false, fmt.Errorf("compendium: last import: %w", err)
	}
	if last == hash {
		return false, nil
	}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return importInto(ctx, queries.New(tx), snap, hash)
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func importInto(ctx context.Context, q *queries.Queries, snap snapshot.Snapshot, hash string) error {
	docIDs := map[string]int64{}
	for _, d := range snap.Documents {
		id, err := q.UpsertDocument(ctx, queries.UpsertDocumentParams{
			Key: d.Key, Title: d.Title, RulesetYear: int32(d.RulesetYear), Precedence: int32(d.Precedence), //nolint:gosec // small bounded values
			License: d.License, Attribution: d.Attribution, Url: d.URL,
		})
		if err != nil {
			return fmt.Errorf("compendium: document %s: %w", d.Key, err)
		}
		docIDs[d.Key] = id
	}
	lookups := &lookupCache{q: q, schools: map[string]int64{}, damage: map[string]int64{}, abilities: map[string]int64{}}
	for _, sp := range snap.Spells {
		if err := importSpell(ctx, q, lookups, docIDs[sp.Document], sp); err != nil {
			return fmt.Errorf("compendium: spell %s/%s: %w", sp.Document, sp.Slug, err)
		}
	}
	for _, c := range snap.Conditions {
		if err := q.UpsertCondition(ctx, queries.UpsertConditionParams{
			DocumentID: docIDs[c.Document], Slug: c.Slug, Name: c.Name, Description: c.Description,
		}); err != nil {
			return fmt.Errorf("compendium: condition %s: %w", c.Slug, err)
		}
	}
	if err := importEntries(ctx, q, lookups, docIDs, snap); err != nil {
		return err
	}
	_, err := q.RecordCompendiumImport(ctx, hash)
	return err
}

type lookupCache struct {
	q         *queries.Queries
	schools   map[string]int64
	damage    map[string]int64
	abilities map[string]int64
}

func (l *lookupCache) school(ctx context.Context, slug string) (int64, error) {
	if id, ok := l.schools[slug]; ok {
		return id, nil
	}
	id, err := l.q.UpsertMagicSchool(ctx, queries.UpsertMagicSchoolParams{Slug: slug, Name: title(slug)})
	l.schools[slug] = id
	return id, err
}

func (l *lookupCache) damageType(ctx context.Context, slug string) (int64, error) {
	if id, ok := l.damage[slug]; ok {
		return id, nil
	}
	id, err := l.q.UpsertDamageType(ctx, queries.UpsertDamageTypeParams{Slug: slug, Name: title(slug)})
	l.damage[slug] = id
	return id, err
}

func (l *lookupCache) ability(ctx context.Context, slug string) (pgtype.Int8, error) {
	if slug == "" {
		return pgtype.Int8{}, nil
	}
	id, ok := l.abilities[slug]
	if !ok {
		var err error
		if id, err = l.q.AbilityIDBySlug(ctx, slug); err != nil {
			return pgtype.Int8{}, fmt.Errorf("unknown ability %q: %w", slug, err)
		}
		l.abilities[slug] = id
	}
	return pgtype.Int8{Int64: id, Valid: true}, nil
}

func importSpell(ctx context.Context, q *queries.Queries, l *lookupCache, docID int64, sp snapshot.Spell) error {
	schoolID, err := l.school(ctx, sp.School)
	if err != nil {
		return err
	}
	saveID, err := l.ability(ctx, sp.SaveAbility)
	if err != nil {
		return err
	}
	id, err := q.UpsertSpell(ctx, queries.UpsertSpellParams{
		DocumentID: docID, Slug: sp.Slug, Name: sp.Name, Level: int32(sp.Level), SchoolID: schoolID, //nolint:gosec // 0..9
		CastingTime: sp.CastingTime, RangeText: sp.RangeText, RangeFeet: optInt(sp.RangeFeet),
		RequiresVerbal: sp.Verbal, RequiresSomatic: sp.Somatic, RequiresMaterial: sp.Material,
		MaterialText: optText(sp.MaterialText), Ritual: sp.Ritual, Concentration: sp.Concentration,
		Duration: sp.Duration, Description: sp.Description, HigherLevel: optText(sp.HigherLevel),
		SaveAbilityID: saveID, AttackRoll: sp.AttackRoll, DamageRoll: optText(sp.DamageRoll),
	})
	if err != nil {
		return err
	}
	if err := q.ClearSpellChildren(ctx, id); err != nil {
		return err
	}
	for _, c := range sp.Classes {
		if err := q.AddSpellClass(ctx, queries.AddSpellClassParams{SpellID: id, ClassSlug: c}); err != nil {
			return err
		}
	}
	for _, d := range sp.DamageTypes {
		dt, err := l.damageType(ctx, d)
		if err != nil {
			return err
		}
		if err := q.AddSpellDamageType(ctx, queries.AddSpellDamageTypeParams{SpellID: id, DamageTypeID: dt}); err != nil {
			return err
		}
	}
	for _, s := range sp.Scaling {
		if err := q.AddSpellScaling(ctx, queries.AddSpellScalingParams{
			SpellID: id, Kind: s.Kind, AtLevel: int32(s.Level), DamageRoll: s.DamageRoll, //nolint:gosec // 1..20
		}); err != nil {
			return err
		}
	}
	return nil
}

// Version identifies the imported compendium; it changes only when an import changes data.
func (s *Store) Version(ctx context.Context) (int64, error) {
	return s.q.CompendiumVersion(ctx)
}

// ListSources returns every source document with its attribution.
func (s *Store) ListSources(ctx context.Context) ([]compendium.Source, error) {
	rows, err := s.q.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]compendium.Source, 0, len(rows))
	for _, r := range rows {
		out = append(out, compendium.Source{
			Key: r.Key, Title: r.Title, RulesetYear: int(r.RulesetYear), License: r.License, Attribution: r.Attribution, URL: r.Url,
		})
	}
	return out, nil
}

// ListSpells returns one page of spells after the filter's cursor.
func (s *Store) ListSpells(ctx context.Context, f compendium.SpellFilter) ([]compendium.SpellSummary, error) {
	p := queries.ListSpellsParams{
		Query: optText(f.Query), School: optText(f.School), ClassSlug: optText(f.Class), Ruleset: optText(f.Ruleset),
		PageSize: int32(f.PageSize), //nolint:gosec // capped by the API at 100
	}
	if f.Level != nil {
		p.Level = pgtype.Int4{Int32: int32(*f.Level), Valid: true} //nolint:gosec // 0..9
	}
	if f.After != nil {
		p.AfterName = optText(f.After.Name)
		p.AfterSlug = optText(f.After.Slug)
	}
	rows, err := s.q.ListSpells(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]compendium.SpellSummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, compendium.SpellSummary{
			Slug: r.Slug, Name: r.Name, Level: int(r.Level), School: r.School, Ruleset: r.DocumentKey,
			Ritual: r.Ritual, Concentration: r.Concentration,
		})
	}
	return out, nil
}

// GetSpell returns a spell from the ruleset, or the highest-precedence ruleset when empty.
func (s *Store) GetSpell(ctx context.Context, slug, ruleset string) (compendium.Spell, error) {
	r, err := s.q.GetSpell(ctx, queries.GetSpellParams{Slug: slug, Ruleset: optText(ruleset)})
	if errors.Is(err, pgx.ErrNoRows) {
		return compendium.Spell{}, compendium.ErrNotFound
	}
	if err != nil {
		return compendium.Spell{}, err
	}
	sp := compendium.Spell{
		SpellSummary: compendium.SpellSummary{
			Slug: r.Slug, Name: r.Name, Level: int(r.Level), School: r.School, Ruleset: r.DocumentKey,
			Ritual: r.Ritual, Concentration: r.Concentration,
		},
		CastingTime: r.CastingTime, RangeText: r.RangeText, Verbal: r.RequiresVerbal, Somatic: r.RequiresSomatic,
		Material: r.RequiresMaterial, MaterialText: r.MaterialText.String, Duration: r.Duration, Description: r.Description,
		HigherLevel: r.HigherLevel.String, SaveAbility: r.SaveAbility.String, AttackRoll: r.AttackRoll, DamageRoll: r.DamageRoll.String,
	}
	if r.RangeFeet.Valid {
		feet := int(r.RangeFeet.Int32)
		sp.RangeFeet = &feet
	}
	if sp.Classes, err = s.q.SpellClasses(ctx, r.ID); err != nil {
		return compendium.Spell{}, err
	}
	if sp.DamageTypes, err = s.q.SpellDamageTypes(ctx, r.ID); err != nil {
		return compendium.Spell{}, err
	}
	scaling, err := s.q.SpellScaling(ctx, r.ID)
	if err != nil {
		return compendium.Spell{}, err
	}
	sp.Scaling = make([]compendium.Scaling, 0, len(scaling))
	for _, sc := range scaling {
		sp.Scaling = append(sp.Scaling, compendium.Scaling{Kind: sc.Kind, Level: int(sc.AtLevel), DamageRoll: sc.DamageRoll})
	}
	conditions, err := s.q.ConditionsForDocument(ctx, r.DocumentKey)
	if err != nil {
		return compendium.Spell{}, err
	}
	all := make([]compendium.Condition, 0, len(conditions))
	for _, c := range conditions {
		all = append(all, compendium.Condition{Slug: c.Slug, Name: c.Name, Description: c.Description})
	}
	sp.Mentions = compendium.FindMentions(all, sp.Description, sp.HigherLevel)
	return sp, nil
}

func optText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

func optInt(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(*v), Valid: true} //nolint:gosec // ranges fit
}

func title(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
