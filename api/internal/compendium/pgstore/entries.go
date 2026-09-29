package pgstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

// ListEntries returns one page of a kind, blended so the newest ruleset leads unless one is chosen.
func (s *Store) ListEntries(ctx context.Context, f compendium.EntryFilter) ([]compendium.EntrySummary, error) {
	p := queries.ListEntriesParams{
		Kind: f.Kind, Query: optText(f.Query), Ruleset: optText(f.Ruleset),
		PageSize: int32(f.PageSize), //nolint:gosec // capped by the API at 100
	}
	if f.After != nil {
		p.AfterName = optText(f.After.Name)
		p.AfterSlug = optText(f.After.Slug)
	}
	rows, err := s.q.ListEntries(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]compendium.EntrySummary, 0, len(rows))
	for _, r := range rows {
		out = append(out, compendium.EntrySummary{Kind: r.Kind, Slug: r.Slug, Name: r.Name, Subtitle: r.Subtitle, Ruleset: r.DocumentKey})
	}
	return out, nil
}

type presenter func(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error

func presenters() map[string]presenter {
	return map[string]presenter{
		"class": presentClass, "species": presentSpecies, "background": presentBackground, "feat": presentFeat,
		"weapon": presentWeapon, "armor": presentArmor, "item": presentItem, "magic-item": presentItem,
		"monster": presentMonster, "condition": presentCondition,
	}
}

// GetEntry renders one entry for reading, with the conditions its text mentions.
func (s *Store) GetEntry(ctx context.Context, kind, slug, ruleset string) (compendium.EntryDetail, error) {
	present, ok := presenters()[kind]
	if !ok {
		return compendium.EntryDetail{}, compendium.ErrNotFound
	}
	r, err := s.q.FindEntry(ctx, queries.FindEntryParams{Kind: kind, Slug: slug, Ruleset: optText(ruleset)})
	if errors.Is(err, pgx.ErrNoRows) {
		return compendium.EntryDetail{}, compendium.ErrNotFound
	}
	if err != nil {
		return compendium.EntryDetail{}, err
	}
	e := compendium.EntryDetail{
		EntrySummary: compendium.EntrySummary{Kind: kind, Slug: slug, Name: r.Name, Subtitle: r.Subtitle, Ruleset: r.DocumentKey},
		Facts:        []compendium.Fact{}, Sections: []compendium.Section{}, Mentions: []compendium.Condition{},
	}
	if err := present(ctx, s.q, r.ID, &e); err != nil {
		return compendium.EntryDetail{}, fmt.Errorf("compendium: %s %s: %w", kind, slug, err)
	}
	if kind == "condition" {
		return e, nil
	}
	conditions, err := s.q.ConditionsForDocument(ctx, r.DocumentKey)
	if err != nil {
		return compendium.EntryDetail{}, err
	}
	all := make([]compendium.Condition, 0, len(conditions))
	for _, c := range conditions {
		all = append(all, compendium.Condition{Slug: c.Slug, Name: c.Name, Description: c.Description})
	}
	texts := make([]string, 0, len(e.Sections))
	for _, sec := range e.Sections {
		texts = append(texts, sec.Text)
	}
	e.Mentions = compendium.FindMentions(all, texts...)
	return e, nil
}

// AutomationCoverage counts every kind by Automation Level. Nothing is computed by the rules engine yet.
func (s *Store) AutomationCoverage(ctx context.Context) ([]compendium.AutomationCount, error) {
	rows, err := s.q.CountEntriesByKind(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]compendium.AutomationCount, 0, len(rows))
	for _, r := range rows {
		total := int(r.Total)
		out = append(out, compendium.AutomationCount{Kind: r.Kind, Total: total, Manual: total})
	}
	return out, nil
}

func addFact(e *compendium.EntryDetail, label, value string) {
	if value != "" {
		e.Facts = append(e.Facts, compendium.Fact{Label: label, Value: value})
	}
}

func addSection(e *compendium.EntryDetail, title, text string) {
	if text != "" {
		e.Sections = append(e.Sections, compendium.Section{Title: title, Text: text})
	}
}

func presentClass(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	c, err := q.GetClassDetail(ctx, id)
	if err != nil {
		return err
	}
	if c.HitDie.Valid {
		addFact(e, "Hit Die", "d"+strconv.Itoa(int(c.HitDie.Int32)))
	}
	addFact(e, "Subclass of", title(c.ParentSlug.String))
	if c.CasterType != "none" {
		addFact(e, "Spellcasting", title(c.CasterType))
	}
	saves, err := q.ClassSaves(ctx, id)
	if err != nil {
		return err
	}
	for i, s := range saves {
		saves[i] = title(s)
	}
	addFact(e, "Saving Throws", strings.Join(saves, ", "))
	addSection(e, "Overview", c.Description)
	features, err := q.ClassFeatures(ctx, id)
	if err != nil {
		return err
	}
	for _, f := range features {
		heading := f.Name
		if len(f.Levels) > 0 {
			heading += " (level " + strconv.Itoa(int(f.Levels[0])) + ")"
		}
		addSection(e, heading, f.Description)
	}
	return nil
}

func presentSpecies(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	s, err := q.GetSpeciesDetail(ctx, id)
	if err != nil {
		return err
	}
	addSection(e, "Overview", s.Description)
	traits, err := q.SpeciesTraits(ctx, id)
	if err != nil {
		return err
	}
	for _, t := range traits {
		addSection(e, t.Name, t.Description)
	}
	return nil
}

func presentBackground(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	desc, err := q.GetBackgroundDetail(ctx, id)
	if err != nil {
		return err
	}
	addSection(e, "Overview", desc)
	benefits, err := q.BackgroundBenefits(ctx, id)
	if err != nil {
		return err
	}
	for _, b := range benefits {
		addSection(e, b.Name, b.Description)
	}
	return nil
}

func presentFeat(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	f, err := q.GetFeatDetail(ctx, id)
	if err != nil {
		return err
	}
	addFact(e, "Type", f.FeatType)
	addFact(e, "Prerequisite", f.Prerequisite)
	addSection(e, "Overview", f.Description)
	benefits, err := q.FeatBenefits(ctx, id)
	if err != nil {
		return err
	}
	addSection(e, "Benefits", strings.Join(benefits, "\n\n"))
	return nil
}

func presentWeapon(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	w, err := q.GetWeaponDetail(ctx, id)
	if err != nil {
		return err
	}
	addFact(e, "Damage", strings.TrimSpace(w.DamageDice+" "+w.DamageType))
	addFact(e, "Category", map[bool]string{true: "Simple", false: "Martial"}[w.Simple])
	if w.LongRangeFeet > 0 {
		addFact(e, "Range", fmt.Sprintf("%d/%d ft", w.RangeFeet, w.LongRangeFeet))
	}
	props, err := q.WeaponProperties(ctx, id)
	if err != nil {
		return err
	}
	names := []string{}
	for _, p := range props {
		name := p.Name
		if p.Detail.String != "" {
			name += " (" + p.Detail.String + ")"
		}
		if p.Mastery {
			addFact(e, "Mastery", name)
			continue
		}
		names = append(names, name)
	}
	addFact(e, "Properties", strings.Join(names, ", "))
	return nil
}

func presentArmor(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	a, err := q.GetArmorDetail(ctx, id)
	if err != nil {
		return err
	}
	ac := strconv.Itoa(int(a.AcBase))
	switch {
	case a.AddDex && a.DexCap.Valid:
		ac += " + Dex (max " + strconv.Itoa(int(a.DexCap.Int32)) + ")"
	case a.AddDex:
		ac += " + Dex"
	}
	addFact(e, "Category", title(a.Category))
	addFact(e, "Armor Class", ac)
	if a.StrengthRequired.Valid {
		addFact(e, "Strength", strconv.Itoa(int(a.StrengthRequired.Int32)))
	}
	if a.StealthDisadvantage {
		addFact(e, "Stealth", "Disadvantage")
	}
	return nil
}

func presentItem(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	it, err := q.GetItemDetail(ctx, id)
	if err != nil {
		return err
	}
	addFact(e, "Category", title(it.Category))
	addFact(e, "Rarity", title(it.Rarity.String))
	if it.RequiresAttunement {
		addFact(e, "Attunement", strings.TrimSpace("Required "+it.AttunementDetail.String))
	}
	if it.CostGp > 0 {
		addFact(e, "Cost", strconv.FormatFloat(it.CostGp, 'f', -1, 64)+" gp")
	}
	if it.WeightLb > 0 {
		addFact(e, "Weight", strconv.FormatFloat(it.WeightLb, 'f', -1, 64)+" lb")
	}
	addSection(e, "Description", it.Description)
	return nil
}

func presentCondition(ctx context.Context, q *queries.Queries, id int64, e *compendium.EntryDetail) error {
	desc, err := q.GetConditionDetail(ctx, id)
	if err != nil {
		return err
	}
	addSection(e, "Effect", desc)
	return nil
}
