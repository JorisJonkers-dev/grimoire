package app

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/classbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/speciesbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/subclassbuild"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// homebrew is the compendium as one Campaign sees it: with the homebrew classes and subclasses its
// Library adds.
type homebrew struct {
	Compendium
	classes []classbuild.Class
	subs    []subclassbuild.Subclass
	species []speciesbuild.Option
}

// class is a homebrew class by slug.
func (h homebrew) class(slug string) (classbuild.Class, bool) {
	return find(h.classes, func(c classbuild.Class) bool { return c.Slug == slug })
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
	classes, err := s.Repo.HomebrewClasses(ctx, id)
	if err != nil {
		return nil, err
	}
	species, err := s.Repo.HomebrewSpecies(ctx, id)
	if err != nil {
		return nil, err
	}
	in := *s
	in.Compendium = homebrew{Compendium: s.Compendium, classes: classes, subs: subs, species: species}
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

// Options are what a character can be built from in a Campaign: its ruleset's options with the
// homebrew classes its Library adds. Its members.
func (s *Characters) Options(ctx context.Context, c caller.Caller, id domain.CampaignID) (compendium.BuilderOptions, error) {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return compendium.BuilderOptions{}, err
	}
	camp, err := s.Repo.GetCampaign(ctx, id)
	if err != nil {
		return compendium.BuilderOptions{}, err
	}
	return s.options(ctx, id, camp.Ruleset)
}

// classList is a casting class's spell list up to the highest level it casts, as a Campaign sees it.
func (s *Characters) classList(ctx context.Context, sheet Sheet, cs ClassSpells) ([]compendium.SpellOption, error) {
	in, err := s.within(ctx, sheet.CampaignID)
	if err != nil {
		return nil, err
	}
	return in.Compendium.ClassSpells(ctx, sheet.Ruleset, cs.Class, cs.MaxLevel)
}

// BuilderOptions adds the homebrew classes.
func (h homebrew) BuilderOptions(ctx context.Context, ruleset string) (compendium.BuilderOptions, error) {
	o, err := h.Compendium.BuilderOptions(ctx, ruleset)
	if err != nil {
		return o, err
	}
	for _, c := range h.classes {
		o.Classes = append(o.Classes, compendium.ClassOption{Slug: c.Slug, Name: c.Name, HitDie: c.HitDie, Saves: c.Saves, Rules: c.Profile})
	}
	for _, sp := range h.species {
		o.Species = append(o.Species, compendium.SpeciesOption{Slug: sp.Slug, Name: sp.Name, SpeedFeet: sp.SpeedFeet})
	}
	return o, nil
}

// Features adds the homebrew classes' choices and the homebrew subclasses' Resources and choices.
func (h homebrew) Features(ctx context.Context) (features.Catalog, error) {
	cat, err := h.Compendium.Features(ctx)
	if err != nil {
		return cat, err
	}
	return classbuild.Merge(subclassbuild.Merge(cat, h.subs), h.classes), nil
}

// ClassSpells reads a homebrew class's spells from the SRD list it casts from.
func (h homebrew) ClassSpells(ctx context.Context, ruleset, class string, maxSpellLevel int) ([]compendium.SpellOption, error) {
	if c, ok := h.class(class); ok {
		class = c.SpellList
	}
	return h.Compendium.ClassSpells(ctx, ruleset, class, maxSpellLevel)
}

// LevelUpOptions offers the class's homebrew subclasses beside its own; a homebrew class has only
// homebrew subclasses and learns from the SRD list it casts from.
func (h homebrew) LevelUpOptions(ctx context.Context, ruleset, class string, maxSpellLevel int) (compendium.LevelUpOptions, error) {
	from := class
	hb, own := h.class(class)
	if own {
		from = hb.SpellList
	}
	lu, err := h.Compendium.LevelUpOptions(ctx, ruleset, from, maxSpellLevel)
	if err != nil {
		return lu, err
	}
	if own {
		lu.Subclasses = []compendium.Named{}
	}
	for _, sc := range h.subs {
		if sc.Class == class {
			lu.Subclasses = append(lu.Subclasses, compendium.Named{Slug: sc.Slug, Name: sc.Name})
		}
	}
	return lu, nil
}

// Traits adds a homebrew class's and subclass's features up to the level in the class.
func (h homebrew) Traits(ctx context.Context, ruleset, species string, classes []compendium.ClassLevel, feats []string) ([]compendium.Trait, error) {
	out, err := h.Compendium.Traits(ctx, ruleset, species, classes, feats)
	if err != nil {
		return nil, err
	}
	if sp, ok := find(h.species, func(o speciesbuild.Option) bool { return o.Slug == species }); ok {
		level := 0
		for _, c := range classes {
			level += c.Level
		}
		out = append(out, gained(level, sp.Traits, func(t speciesbuild.Gained) compendium.Trait {
			return compendium.Trait{Name: t.Name, Source: "species", Level: t.Level, Description: t.Text}
		})...)
	}
	for _, c := range classes {
		if hb, ok := h.class(c.Class); ok {
			out = append(out, gained(c.Level, hb.Traits, func(t classbuild.Trait) compendium.Trait {
				return compendium.Trait{Name: t.Name, Source: "class", Level: t.Level, Description: t.Text}
			})...)
		}
		if sc, ok := find(h.subs, func(sc subclassbuild.Subclass) bool { return sc.Slug == c.Subclass && sc.Class == c.Class }); ok {
			out = append(out, gained(c.Level, sc.Traits, func(t subclassbuild.Trait) compendium.Trait {
				return compendium.Trait{Name: t.Name, Source: "class", Level: t.Level, Description: t.Text}
			})...)
		}
	}
	return out, nil
}

// gained are the traits gained by a class level.
func gained[T any](level int, traits []T, as func(T) compendium.Trait) []compendium.Trait {
	var out []compendium.Trait
	for _, t := range traits {
		if tr := as(t); tr.Level <= level {
			out = append(out, tr)
		}
	}
	return out
}
