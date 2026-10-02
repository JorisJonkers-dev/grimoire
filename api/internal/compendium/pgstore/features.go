package pgstore

import (
	"context"
	"fmt"

	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
)

// Features reads every Resource, scaling value, choice and prerequisite into a Catalog: one query per
// table, assembled in memory.
func (s *Store) Features(ctx context.Context) (features.Catalog, error) {
	cat := features.Catalog{
		Resources: map[string]features.Resource{}, Scales: map[string]features.Named{},
		Choices: map[features.Owner][]features.Choice{}, Prerequisites: map[features.Owner][]features.Requirement{},
	}
	if err := s.resources(ctx, cat); err != nil {
		return cat, fmt.Errorf("compendium: features: %w", err)
	}
	if err := s.scales(ctx, cat); err != nil {
		return cat, fmt.Errorf("compendium: features: %w", err)
	}
	if err := s.choicesAndPrerequisites(ctx, cat); err != nil {
		return cat, fmt.Errorf("compendium: features: %w", err)
	}
	return cat, nil
}

func (s *Store) resources(ctx context.Context, cat features.Catalog) error {
	rows, err := s.q.ListResources(ctx)
	if err != nil {
		return err
	}
	byID := map[int64]*features.Resource{}
	for _, r := range rows {
		byID[r.ID] = &features.Resource{
			Slug: r.Slug, Name: r.Name, Owner: features.Owner{Kind: r.OwnerKind, Slug: r.OwnerSlug}, Basis: features.Basis(r.Basis),
			Multiplier: int(r.Multiplier), Ability: r.Ability.String, FromLevel: int(r.FromLevel),
		}
	}
	maxima, err := s.q.ListResourceMaxima(ctx)
	if err != nil {
		return err
	}
	for _, m := range maxima {
		r := byID[m.ResourceID]
		r.Table = append(r.Table, features.Step[int]{Level: int(m.Level), Value: int(m.Maximum)})
	}
	dice, err := s.q.ListResourceDice(ctx)
	if err != nil {
		return err
	}
	for _, d := range dice {
		r := byID[d.ResourceID]
		r.Die = append(r.Die, features.Step[string]{Level: int(d.Level), Value: d.Die})
	}
	recharges, err := s.q.ListResourceRecharges(ctx)
	if err != nil {
		return err
	}
	for _, rc := range recharges {
		r := byID[rc.ResourceID]
		r.Recharges = append(r.Recharges, features.Recharge{
			On: features.Event(rc.Event), FromLevel: int(rc.FromLevel), Amount: int(rc.Amount.Int32), RollAtLeast: int(rc.RollAtLeast.Int32),
		})
	}
	for _, r := range byID {
		cat.Resources[r.Slug] = *r
	}
	return nil
}

func (s *Store) scales(ctx context.Context, cat features.Catalog) error {
	rows, err := s.q.ListScales(ctx)
	if err != nil {
		return err
	}
	byID := map[int64]*features.Named{}
	for _, r := range rows {
		byID[r.ID] = &features.Named{Slug: r.Slug, Name: r.Name, Owner: features.Owner{Kind: r.OwnerKind, Slug: r.OwnerSlug}}
	}
	steps, err := s.q.ListScaleSteps(ctx)
	if err != nil {
		return err
	}
	for _, st := range steps {
		n := byID[st.ScaleID]
		n.Steps = append(n.Steps, features.Step[string]{Level: int(st.Level), Value: st.Value})
	}
	for _, n := range byID {
		cat.Scales[n.Slug] = *n
	}
	return nil
}

func (s *Store) choicesAndPrerequisites(ctx context.Context, cat features.Catalog) error {
	choices, err := s.q.ListChoices(ctx)
	if err != nil {
		return err
	}
	for _, c := range choices {
		o := features.Owner{Kind: c.OwnerKind, Slug: c.OwnerSlug}
		cat.Choices[o] = append(cat.Choices[o], features.Choice{
			Slug: c.Slug, Name: c.Name, Level: int(c.Level), Count: int(c.Count), Pool: features.Pool(c.Pool), From: c.PoolFrom,
		})
	}
	reqs, err := s.q.ListPrerequisites(ctx)
	if err != nil {
		return err
	}
	for _, p := range reqs {
		o := features.Owner{Kind: p.OwnerKind, Slug: p.OwnerSlug}
		cat.Prerequisites[o] = append(cat.Prerequisites[o], features.Requirement{
			Kind: features.Kind(p.Kind), Ability: p.Ability.String, Minimum: int(p.Minimum), Slug: p.RefSlug.String, Group: int(p.GroupNo),
		})
	}
	return nil
}
