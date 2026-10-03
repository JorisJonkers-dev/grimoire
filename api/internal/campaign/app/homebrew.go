package app

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
)

// homebrew is the compendium as one Campaign sees it: with the homebrew subclasses its Library adds.
type homebrew struct {
	Compendium
	subs []subclassbuild.Subclass
}

// within is the use cases over the compendium as a Campaign sees it.
func (s *Characters) within(ctx context.Context, id domain.CampaignID) (*Characters, error) {
	if _, done := s.Compendium.(homebrew); done {
		return s, nil
	}
	subs, err := s.Repo.HomebrewSubclasses(ctx, id)
	if err != nil {
		return nil, err
	}
	in := *s
	in.Compendium = homebrew{Compendium: s.Compendium, subs: subs}
	return &in, nil
}

// options are the builder options as a Campaign sees them.
func (s *Characters) options(ctx context.Context, id domain.CampaignID, ruleset string) (compendium.BuilderOptions, error) {
	in, err := s.within(ctx, id)
	if err != nil {
		return compendium.BuilderOptions{}, err
	}
	return in.Compendium.BuilderOptions(ctx, ruleset)
}

// classList is a casting class's spell list up to the highest level it casts, as a Campaign sees it.
func (s *Characters) classList(ctx context.Context, sheet Sheet, cs ClassSpells) ([]compendium.SpellOption, error) {
	in, err := s.within(ctx, sheet.CampaignID)
	if err != nil {
		return nil, err
	}
	return in.Compendium.ClassSpells(ctx, sheet.Ruleset, cs.Class, cs.MaxLevel)
}

// Features adds the homebrew subclasses' Resources and choices.
func (h homebrew) Features(ctx context.Context) (features.Catalog, error) {
	cat, err := h.Compendium.Features(ctx)
	if err != nil {
		return cat, err
	}
	return subclassbuild.Merge(cat, h.subs), nil
}

// LevelUpOptions offers the class's homebrew subclasses beside its own.
func (h homebrew) LevelUpOptions(ctx context.Context, ruleset, class string, maxSpellLevel int) (compendium.LevelUpOptions, error) {
	lu, err := h.Compendium.LevelUpOptions(ctx, ruleset, class, maxSpellLevel)
	if err != nil {
		return lu, err
	}
	for _, sc := range h.subs {
		if sc.Class == class {
			lu.Subclasses = append(lu.Subclasses, compendium.Named{Slug: sc.Slug, Name: sc.Name})
		}
	}
	return lu, nil
}

// Traits adds a homebrew subclass's features up to the level in its class.
func (h homebrew) Traits(ctx context.Context, ruleset, species string, classes []compendium.ClassLevel, feats []string) ([]compendium.Trait, error) {
	out, err := h.Compendium.Traits(ctx, ruleset, species, classes, feats)
	if err != nil {
		return nil, err
	}
	for _, c := range classes {
		for _, sc := range h.subs {
			if sc.Slug != c.Subclass || sc.Class != c.Class {
				continue
			}
			for _, t := range sc.Traits {
				if t.Level <= c.Level {
					out = append(out, compendium.Trait{Name: t.Name, Source: "class", Level: t.Level, Description: t.Text})
				}
			}
		}
	}
	return out, nil
}
