package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// CompendiumReader is what the compendium operations need.
type CompendiumReader interface {
	Version(ctx context.Context) (int64, error)
	ListSpells(ctx context.Context, f compendium.SpellFilter) ([]compendium.SpellSummary, error)
	GetSpell(ctx context.Context, slug, ruleset string) (compendium.Spell, error)
	ListSources(ctx context.Context) ([]compendium.Source, error)
	ListEntries(ctx context.Context, f compendium.EntryFilter) ([]compendium.EntrySummary, error)
	GetEntry(ctx context.Context, kind, slug, ruleset string) (compendium.EntryDetail, error)
	AutomationCoverage(ctx context.Context) ([]compendium.AutomationCount, error)
}

const defaultPageSize = 50

func (h *Handler) etag(ctx context.Context) (string, error) {
	v, err := h.Compendium.Version(ctx)
	if err != nil {
		return "", err
	}
	return `"c` + strconv.FormatInt(v, 10) + `"`, nil
}

// ListSpells returns one page of spells.
func (h *Handler) ListSpells(ctx context.Context, p oas.ListSpellsParams) (oas.ListSpellsRes, error) {
	tag, err := h.etag(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "compendium version", "error", err)
		return unavailable(), nil
	}
	if p.IfNoneMatch.Or("") == tag {
		return &oas.ListSpellsNotModified{ETag: oas.NewOptString(tag)}, nil
	}
	f := compendium.SpellFilter{
		Query: p.Q.Or(""), School: string(p.School.Or("")), Class: string(p.Class.Or("")),
		Ruleset: string(p.Ruleset.Or("")), PageSize: int(p.Limit.Or(defaultPageSize)),
	}
	if lvl, ok := p.Level.Get(); ok {
		level := int(lvl)
		f.Level = &level
	}
	if token, ok := p.Cursor.Get(); ok {
		c, ok := compendium.DecodeCursor(token)
		if !ok {
			return problem(http.StatusBadRequest, "Bad request", "The page cursor is not valid."), nil
		}
		f.After = &c
	}
	page := f.PageSize
	f.PageSize++
	spells, err := h.Compendium.ListSpells(ctx, f)
	if err != nil {
		h.Log.ErrorContext(ctx, "list spells", "error", err)
		return unavailable(), nil
	}
	out := oas.SpellPage{Items: make([]oas.SpellSummary, 0, min(len(spells), page))}
	for i, s := range spells {
		if i == page {
			last := spells[page-1]
			out.NextCursor = oas.NewOptString(compendium.Cursor{Name: last.Name, Slug: last.Slug}.Encode())
			break
		}
		out.Items = append(out.Items, summary(s))
	}
	return &oas.SpellPageHeaders{ETag: oas.NewOptString(tag), Response: out}, nil
}

// GetSpell returns one spell with the conditions it mentions.
func (h *Handler) GetSpell(ctx context.Context, p oas.GetSpellParams) (oas.GetSpellRes, error) {
	tag, err := h.etag(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "compendium version", "error", err)
		return unavailable(), nil
	}
	if p.IfNoneMatch.Or("") == tag {
		return &oas.GetSpellNotModified{ETag: oas.NewOptString(tag)}, nil
	}
	sp, err := h.Compendium.GetSpell(ctx, string(p.Slug), string(p.Ruleset.Or("")))
	if errors.Is(err, compendium.ErrNotFound) {
		return problem(http.StatusNotFound, "Not found", "No such spell in this ruleset."), nil
	}
	if err != nil {
		h.Log.ErrorContext(ctx, "get spell", "error", err)
		return unavailable(), nil
	}
	return &oas.SpellHeaders{ETag: oas.NewOptString(tag), Response: spell(sp)}, nil
}

// ListSources returns the compendium's source documents with their attribution.
func (h *Handler) ListSources(ctx context.Context) (oas.ListSourcesRes, error) {
	sources, err := h.Compendium.ListSources(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "list sources", "error", err)
		return unavailable(), nil
	}
	out := make([]oas.Source, 0, len(sources))
	for _, s := range sources {
		out = append(out, oas.Source{
			Key: s.Key, Title: s.Title, RulesetYear: int32(s.RulesetYear), License: s.License, //nolint:gosec // years fit
			Attribution: s.Attribution, URL: s.URL,
		})
	}
	return &oas.ListSourcesOKHeaders{Response: out}, nil
}

func summary(s compendium.SpellSummary) oas.SpellSummary {
	return oas.SpellSummary{
		Slug: oas.Slug(s.Slug), Name: s.Name, Level: int32(s.Level), School: oas.Slug(s.School), //nolint:gosec // 0..9
		Ruleset: oas.Ruleset(s.Ruleset), Ritual: s.Ritual, Concentration: s.Concentration,
	}
}

func spell(s compendium.Spell) oas.Spell {
	out := oas.Spell{
		Slug: oas.Slug(s.Slug), Name: s.Name, Level: int32(s.Level), School: oas.Slug(s.School), //nolint:gosec // 0..9
		Ruleset: oas.Ruleset(s.Ruleset), Ritual: s.Ritual, Concentration: s.Concentration,
		CastingTime: s.CastingTime, RangeText: s.RangeText, Verbal: s.Verbal, Somatic: s.Somatic, Material: s.Material,
		Duration: s.Duration, Description: s.Description, AttackRoll: s.AttackRoll,
		Classes: slugs(s.Classes), DamageTypes: slugs(s.DamageTypes),
		Scaling: make([]oas.SpellScaling, 0, len(s.Scaling)), Mentions: make([]oas.ConditionRef, 0, len(s.Mentions)),
	}
	if s.RangeFeet != nil {
		out.RangeFeet = oas.NewOptInt32(int32(*s.RangeFeet)) //nolint:gosec // ranges fit
	}
	setOpt(&out.MaterialText, s.MaterialText)
	setOpt(&out.HigherLevel, s.HigherLevel)
	setOpt(&out.DamageRoll, s.DamageRoll)
	if s.SaveAbility != "" {
		out.SaveAbility = oas.NewOptSlug(oas.Slug(s.SaveAbility))
	}
	for _, sc := range s.Scaling {
		out.Scaling = append(out.Scaling, oas.SpellScaling{
			Kind: oas.SpellScalingKind(sc.Kind), Level: int32(sc.Level), DamageRoll: sc.DamageRoll, //nolint:gosec // 1..20
		})
	}
	for _, c := range s.Mentions {
		out.Mentions = append(out.Mentions, oas.ConditionRef{Slug: oas.Slug(c.Slug), Name: c.Name, Description: c.Description})
	}
	return out
}

func slugs(in []string) []oas.Slug {
	out := make([]oas.Slug, 0, len(in))
	for _, s := range in {
		out = append(out, oas.Slug(s))
	}
	return out
}

func setOpt(dst *oas.OptString, v string) {
	if v != "" {
		*dst = oas.NewOptString(v)
	}
}
