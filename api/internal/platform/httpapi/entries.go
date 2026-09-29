package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// ListEntries returns one page of one entry kind.
func (h *Handler) ListEntries(ctx context.Context, p oas.ListEntriesParams) (oas.ListEntriesRes, error) {
	tag, err := h.etag(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "compendium version", "error", err)
		return unavailable(), nil
	}
	if p.IfNoneMatch.Or("") == tag {
		return &oas.ListEntriesNotModified{ETag: oas.NewOptString(tag)}, nil
	}
	f := compendium.EntryFilter{
		Kind: string(p.Kind), Query: p.Q.Or(""), Ruleset: string(p.Ruleset.Or("")), PageSize: int(p.Limit.Or(defaultPageSize)),
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
	entries, err := h.Compendium.ListEntries(ctx, f)
	if err != nil {
		h.Log.ErrorContext(ctx, "list entries", "error", err)
		return unavailable(), nil
	}
	out := oas.EntryPage{Items: make([]oas.EntrySummary, 0, min(len(entries), page))}
	for i, e := range entries {
		if i == page {
			last := entries[page-1]
			out.NextCursor = oas.NewOptString(compendium.Cursor{Name: last.Name, Slug: last.Slug}.Encode())
			break
		}
		out.Items = append(out.Items, entrySummary(e))
	}
	return &oas.EntryPageHeaders{ETag: oas.NewOptString(tag), Response: out}, nil
}

// GetEntry returns one entry rendered for reading.
func (h *Handler) GetEntry(ctx context.Context, p oas.GetEntryParams) (oas.GetEntryRes, error) {
	tag, err := h.etag(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "compendium version", "error", err)
		return unavailable(), nil
	}
	if p.IfNoneMatch.Or("") == tag {
		return &oas.GetEntryNotModified{ETag: oas.NewOptString(tag)}, nil
	}
	e, err := h.Compendium.GetEntry(ctx, string(p.Kind), string(p.Slug), string(p.Ruleset.Or("")))
	if errors.Is(err, compendium.ErrNotFound) {
		return problem(http.StatusNotFound, "Not found", "No such entry in this ruleset."), nil
	}
	if err != nil {
		h.Log.ErrorContext(ctx, "get entry", "error", err)
		return unavailable(), nil
	}
	out := oas.Entry{
		Kind: oas.EntryKind(e.Kind), Slug: oas.Slug(e.Slug), Name: e.Name, Subtitle: e.Subtitle, Ruleset: oas.Ruleset(e.Ruleset),
		Facts: make([]oas.EntryFact, 0, len(e.Facts)), Sections: make([]oas.EntrySection, 0, len(e.Sections)),
		Mentions: make([]oas.ConditionRef, 0, len(e.Mentions)),
	}
	for _, f := range e.Facts {
		out.Facts = append(out.Facts, oas.EntryFact{Label: f.Label, Value: f.Value})
	}
	for _, s := range e.Sections {
		out.Sections = append(out.Sections, oas.EntrySection{Title: s.Title, Text: s.Text})
	}
	for _, c := range e.Mentions {
		out.Mentions = append(out.Mentions, oas.ConditionRef{Slug: oas.Slug(c.Slug), Name: c.Name, Description: c.Description})
	}
	return &oas.EntryHeaders{ETag: oas.NewOptString(tag), Response: out}, nil
}

// GetAutomationCoverage reports how much of each kind the rules engine computes.
func (h *Handler) GetAutomationCoverage(ctx context.Context, p oas.GetAutomationCoverageParams) (oas.GetAutomationCoverageRes, error) {
	tag, err := h.etag(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "compendium version", "error", err)
		return unavailable(), nil
	}
	if p.IfNoneMatch.Or("") == tag {
		return &oas.GetAutomationCoverageNotModified{ETag: oas.NewOptString(tag)}, nil
	}
	counts, err := h.Compendium.AutomationCoverage(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "automation coverage", "error", err)
		return unavailable(), nil
	}
	out := make([]oas.AutomationCount, 0, len(counts))
	for _, c := range counts {
		out = append(out, oas.AutomationCount{
			Kind: oas.AutomationCountKind(c.Kind), Total: int32(c.Total), Full: int32(c.Full), //nolint:gosec // counts fit
			Partial: int32(c.Partial), Manual: int32(c.Manual), //nolint:gosec // counts fit
		})
	}
	return &oas.GetAutomationCoverageOKHeaders{ETag: oas.NewOptString(tag), Response: out}, nil
}

func entrySummary(e compendium.EntrySummary) oas.EntrySummary {
	return oas.EntrySummary{
		Kind: oas.EntryKind(e.Kind), Slug: oas.Slug(e.Slug), Name: e.Name, Subtitle: e.Subtitle, Ruleset: oas.Ruleset(e.Ruleset),
	}
}
