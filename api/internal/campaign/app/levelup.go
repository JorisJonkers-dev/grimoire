package app

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// asi is the feat that raises ability scores instead of granting a feature.
const asi = "ability-score-improvement"

// LevelUpOption is one option a choice offers, and what the Character lacks to take it.
type LevelUpOption struct {
	Slug  string
	Name  string
	Unmet []string
}

// LevelUpChoice is a pick the next level asks for, from the class's Feature data.
type LevelUpChoice struct {
	Slug    string
	Name    string
	Pool    features.Pool
	Count   int
	Options []LevelUpOption
}

// LevelUpClass is a class the next level can go to: one the Character has, or a new one when it meets
// the multiclass prerequisites.
type LevelUpClass struct {
	Slug   string
	Name   string
	HitDie int
	Level  int
	Unmet  []string
}

// LevelUpPlan is everything the next level in one class offers.
type LevelUpPlan struct {
	Ready      bool
	Held       bool
	Level      int
	Classes    []LevelUpClass
	Class      string
	ClassLevel int
	HitDie     int
	ConMod     int
	Average    int
	Choices    []LevelUpChoice
	Cantrips   int
	Spells     int
	SpellList  []compendium.SpellOption
}

// LevelUpRequest is what a player chose for the next level. Roll rolls the Hit Die instead of taking its
// average; Increase is the Ability Score Improvement when that feat is picked.
type LevelUpRequest struct {
	Class    string
	Roll     bool
	Picks    map[string][]string
	Increase map[string]int
	Spells   []string
}

// featCategories maps a choice's feat category to the feats' own.
func featCategories() map[string]string {
	return map[string]string{"general": "General", "fighting-style": "Fighting Style", "epic-boon": "Epic Boon", "origin": "Origin"}
}

// PlanLevelUp is what the next level in a class offers a Character; an empty class means its starting
// class. The owner or a DM, out of combat.
func (s *Characters) PlanLevelUp(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, class string) (LevelUpPlan, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return LevelUpPlan{}, err
	}
	return s.plan(ctx, sheet, class)
}

func (s *Characters) plan(ctx context.Context, sheet Sheet, class string) (LevelUpPlan, error) {
	o, err := s.Compendium.BuilderOptions(ctx, sheet.Ruleset)
	if err != nil {
		return LevelUpPlan{}, err
	}
	camp, err := s.Repo.GetCampaign(ctx, sheet.CampaignID)
	if err != nil {
		return LevelUpPlan{}, err
	}
	if class == "" {
		class = sheet.Classes[0].Class
	}
	p := LevelUpPlan{Ready: sheet.LevelUpReady && sheet.Level < 20, Held: camp.HoldLevelUps, Level: sheet.Level + 1, Class: class}
	p.Classes = classOffers(o, sheet)
	offer, ok := find(p.Classes, func(x LevelUpClass) bool { return x.Slug == class })
	if !ok {
		return LevelUpPlan{}, refuse("choose a class from this ruleset")
	}
	if len(offer.Unmet) > 0 {
		return LevelUpPlan{}, refuse("multiclassing into " + offer.Name + " needs " + strings.Join(offer.Unmet, ", "))
	}
	p.ClassLevel, p.HitDie = offer.Level+1, offer.HitDie
	p.ConMod = rules.Modifier(sheet.Scores[rules.Constitution])
	p.Average = rules.HitPointGain(p.HitDie, p.ConMod, 0)
	p.Cantrips, p.Spells = newSpells(class, offer.Level)
	lu, err := s.Compendium.LevelUpOptions(ctx, sheet.Ruleset, class, rules.MaxSpellLevel(class, p.ClassLevel))
	if err != nil {
		return LevelUpPlan{}, err
	}
	cat, err := s.Compendium.Features(ctx)
	if err != nil {
		return LevelUpPlan{}, err
	}
	p.Choices = choicesFor(cat, lu, sheet, class, p.ClassLevel, offer.Level == 0)
	p.SpellList = spellList(lu.Spells, sheet.Spells, p.Cantrips, p.Spells)
	return p, nil
}

// newSpells are how many cantrips and spells a class's next level adds, from one it has at a level; a
// new class grants its whole first level. A wizard writes its new spells into its spellbook: six at first
// level and two each level after.
func newSpells(class string, level int) (int, int) {
	cantrips := rules.CantripsKnown(class, level+1) - rules.CantripsKnown(class, level)
	if level == 0 {
		cantrips = rules.CantripsKnown(class, 1)
	}
	switch {
	case rules.KeepsSpellbook(class):
		return cantrips, rules.SpellbookAllotment(level+1) - rules.SpellbookAllotment(level)*min(level, 1)
	case level == 0:
		return cantrips, rules.PreparedSpells(class, 1)
	default:
		return cantrips, rules.PreparedSpells(class, level+1) - rules.PreparedSpells(class, level)
	}
}

// preparedCount is how many spells of level 1 and up a Character has prepared through a class.
func preparedCount(spells []domain.LearnedSpell, class string) int {
	n := 0
	for _, s := range spells {
		if s.Class == class && s.Prepared && s.Spellbook {
			n++
		}
	}
	return n
}

// spellList is the class's list the level can learn from: cantrips when it grants some, spells when it
// grants some, without any already learned.
func spellList(list []compendium.SpellOption, learned []domain.LearnedSpell, cantrips, spells int) []compendium.SpellOption {
	var out []compendium.SpellOption
	for _, sp := range list {
		known := slices.ContainsFunc(learned, func(x domain.LearnedSpell) bool { return x.Spell == sp.Slug })
		if !known && ((sp.Level == 0 && cantrips > 0) || (sp.Level > 0 && spells > 0)) {
			out = append(out, sp)
		}
	}
	return out
}

// classOffers are the classes the next level can go to, with what each new one still needs.
func classOffers(o compendium.BuilderOptions, sheet Sheet) []LevelUpClass {
	have := make([]string, 0, len(sheet.Classes))
	for _, x := range sheet.Classes {
		have = append(have, x.Class)
	}
	out := make([]LevelUpClass, 0, len(o.Classes))
	for _, cl := range o.Classes {
		level := 0
		if x, ok := find(sheet.Classes, func(x domain.ClassLevel) bool { return x.Class == cl.Slug }); ok {
			level = x.Level
		}
		out = append(out, LevelUpClass{Slug: cl.Slug, Name: cl.Name, HitDie: cl.HitDie, Level: level, Unmet: rules.MulticlassUnmet(sheet.Scores, have, cl.Slug)})
	}
	return out
}

// choicesFor are the picks a class level asks for, from its Feature data and its subclass's. Weapon
// Mastery follows the weapons carried, so it is not picked here; a new class grants a skill only where
// the multiclass rules give one.
func choicesFor(cat features.Catalog, lu compendium.LevelUpOptions, sheet Sheet, class string, level int, multiclass bool) []LevelUpChoice {
	raw := features.ChoicesAt(cat.Choices[features.Owner{Kind: "class", Slug: class}], level)
	if x, ok := find(sheet.Classes, func(x domain.ClassLevel) bool { return x.Class == class }); ok && x.Subclass != "" {
		raw = append(raw, features.ChoicesAt(cat.Choices[features.Owner{Kind: "subclass", Slug: x.Subclass}], level)...)
	}
	var out []LevelUpChoice
	for _, ch := range raw {
		if ch.Pool == features.WeaponKind {
			continue
		}
		if multiclass && ch.Pool == features.Skill {
			if !slices.Contains([]string{"bard", "ranger", "rogue"}, class) {
				continue
			}
			ch.Count = 1
		}
		opts := optionsFor(cat, lu, sheet, ch)
		if len(opts) == 0 {
			continue
		}
		out = append(out, LevelUpChoice{Slug: ch.Slug, Name: ch.Name, Pool: ch.Pool, Count: ch.Count, Options: opts})
	}
	return out
}

func optionsFor(cat features.Catalog, lu compendium.LevelUpOptions, sheet Sheet, ch features.Choice) []LevelUpOption {
	var out []LevelUpOption
	switch ch.Pool {
	case features.FeatCategory:
		out = featOptions(cat, lu.Feats, sheet, ch)
	case features.Subclass:
		for _, x := range lu.Subclasses {
			out = append(out, LevelUpOption{Slug: x.Slug, Name: x.Name, Unmet: nil})
		}
	case features.Skill, features.Expertise:
		for _, sk := range sheet.Derived.Skills {
			if (ch.Pool == features.Skill && !sk.Proficient) || (ch.Pool == features.Expertise && sk.Proficient && !sk.Expertise) {
				out = append(out, LevelUpOption{Slug: string(sk.Skill), Name: titleCase(string(sk.Skill)), Unmet: nil})
			}
		}
	case features.Listed, features.WeaponKind:
		// No class level lists its own options in the SRD, and Weapon Mastery follows the weapons carried.
	}
	return out
}

// featOptions are a category's feats the Character has not taken, Ability Score Improvement as often as
// it likes, each with the prerequisites it misses.
func featOptions(cat features.Catalog, feats []compendium.FeatOption, sheet Sheet, ch features.Choice) []LevelUpOption {
	taken := pickValues(sheet.Picks)
	who := features.Candidate{
		Level: sheet.Level + 1, Abilities: scoreMap(sheet.Scores), Spellcasting: rules.CasterFor(sheet.Class) != rules.NoCaster,
		Feats: taken, Features: append(slices.Clone(taken), ch.Slug),
	}
	var out []LevelUpOption
	for _, f := range feats {
		if f.Category == featCategories()[ch.From] && (f.Slug == asi || !slices.Contains(taken, f.Slug)) {
			out = append(out, LevelUpOption{Slug: f.Slug, Name: f.Name, Unmet: features.Unmet(cat.Prerequisites[features.Owner{Kind: "feat", Slug: f.Slug}], who)})
		}
	}
	return out
}

func pickValues(picks []domain.Pick) []string {
	out := make([]string, 0, len(picks))
	for _, p := range picks {
		out = append(out, p.Value)
	}
	return out
}

func scoreMap(in map[rules.Ability]int) map[string]int {
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[string(k)] = v
	}
	return out
}

func titleCase(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// LevelUp takes a Character's next level with the choices made for it, once the level is unlocked.
func (s *Characters) LevelUp(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, req LevelUpRequest) (Sheet, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return Sheet{}, err
	}
	p, err := s.plan(ctx, sheet, req.Class)
	if err != nil {
		return Sheet{}, err
	}
	if !p.Ready {
		return Sheet{}, refuse("the next level is not unlocked yet")
	}
	l := domain.LevelUp{CampaignID: id, ID: ch, From: sheet.Level, Classes: slices.Clone(sheet.Classes), Increase: map[string]int{}}
	if l.Picks, err = picksFor(p, req.Picks); err != nil {
		return Sheet{}, err
	}
	scores, err := improvement(sheet, l.Picks, req.Increase)
	if err != nil {
		return Sheet{}, err
	}
	if l.Spells, err = spellsFor(p, req.Spells, preparedCount(sheet.Spells, p.Class)); err != nil {
		return Sheet{}, err
	}
	l.Classes = advance(l.Classes, p, l.Picks)
	l.Picks = slices.DeleteFunc(l.Picks, func(x domain.Pick) bool { return x.Choice == "subclass" })
	for a, n := range sheet.Increase {
		l.Increase[a] = n
	}
	for a, n := range req.Increase {
		l.Increase[a] += n
	}
	l.Gain = s.hitPoints(p, sheet, scores, req.Roll)
	if err := s.Repo.InTx(ctx, func(r Repository) error { return r.LevelUp(ctx, l, s.Now()) }); err != nil {
		return Sheet{}, err
	}
	return s.Get(ctx, c, id, ch)
}

// hitPoints is what the level adds: the Hit Die rolled or its average with the new Constitution modifier,
// and one point per earlier level for each point the modifier rose.
func (s *Characters) hitPoints(p LevelUpPlan, sheet Sheet, scores map[rules.Ability]int, roll bool) int {
	conMod := rules.Modifier(scores[rules.Constitution])
	rolled := 0
	if roll && s.Die != nil {
		rolled = s.Die(p.HitDie)
	}
	return rules.HitPointGain(p.HitDie, conMod, rolled) + (conMod-p.ConMod)*sheet.Level
}

// picksFor checks the picks against the plan's choices: each choice answered with its count of distinct
// options the Character can take, and nothing else.
func picksFor(p LevelUpPlan, picks map[string][]string) ([]domain.Pick, error) {
	var out []domain.Pick
	for _, ch := range p.Choices {
		values := picks[ch.Slug]
		if len(values) != ch.Count {
			return nil, refuse("choose " + strconv.Itoa(ch.Count) + " for " + ch.Name)
		}
		for i, v := range values {
			opt, ok := find(ch.Options, func(o LevelUpOption) bool { return o.Slug == v })
			if !ok || slices.Contains(values[:i], v) {
				return nil, refuse("choose each " + ch.Name + " once, from the options")
			}
			if len(opt.Unmet) > 0 {
				return nil, refuse(opt.Name + " needs " + strings.Join(opt.Unmet, ", "))
			}
			out = append(out, domain.Pick{Level: p.Level, Choice: ch.Slug, Value: v})
		}
	}
	for slug := range picks {
		if !slices.ContainsFunc(p.Choices, func(c LevelUpChoice) bool { return c.Slug == slug }) {
			return nil, refuse("this level has no choice " + slug)
		}
	}
	return out, nil
}

// improvement applies the Ability Score Improvement when that feat is picked; an increase without it is
// refused.
func improvement(sheet Sheet, picks []domain.Pick, increase map[string]int) (map[rules.Ability]int, error) {
	if !slices.ContainsFunc(picks, func(x domain.Pick) bool { return x.Value == asi }) {
		if len(increase) > 0 {
			return nil, refuse("raise ability scores by picking Ability Score Improvement")
		}
		return sheet.Scores, nil
	}
	scores, err := rules.ImproveAbilities(sheet.Scores, abilityMap(increase))
	return scores, invalid(err)
}

// spellsFor checks the cantrips and spells learned: as many of each as the level grants, distinct, from
// the class's list.
//
// Cantrips and the spells of most classes are prepared at once; a wizard's go into its spellbook and are
// prepared while it has room.
func spellsFor(p LevelUpPlan, spells []string, prepared int) ([]domain.LearnedSpell, error) {
	limit := rules.PreparedSpells(p.Class, p.ClassLevel)
	book := rules.KeepsSpellbook(p.Class)
	cantrips, leveled := 0, 0
	out := make([]domain.LearnedSpell, 0, len(spells))
	for i, slug := range spells {
		sp, ok := find(p.SpellList, func(x compendium.SpellOption) bool { return x.Slug == slug })
		if !ok || slices.Contains(spells[:i], slug) {
			return nil, refuse("choose each spell once, from the class's list")
		}
		learned := domain.LearnedSpell{Class: p.Class, Spell: slug, Level: p.Level, Prepared: true, Spellbook: false}
		if sp.Level == 0 {
			cantrips++
		} else {
			leveled++
			learned.Spellbook = book
			learned.Prepared = !book || prepared < limit
			if learned.Prepared {
				prepared++
			}
		}
		out = append(out, learned)
	}
	if cantrips != p.Cantrips || leveled != p.Spells {
		return nil, refuse("learn " + strconv.Itoa(p.Cantrips) + " cantrips and " + strconv.Itoa(p.Spells) + " spells")
	}
	return out, nil
}

// advance adds the level to its class, a new class last, with the subclass picked for it.
func advance(classes []domain.ClassLevel, p LevelUpPlan, picks []domain.Pick) []domain.ClassLevel {
	i := slices.IndexFunc(classes, func(x domain.ClassLevel) bool { return x.Class == p.Class })
	if i < 0 {
		classes = append(classes, domain.ClassLevel{Class: p.Class, Subclass: "", Level: 0})
		i = len(classes) - 1
	}
	classes[i].Level++
	for _, x := range picks {
		if x.Choice == "subclass" {
			classes[i].Subclass = x.Value
		}
	}
	return classes
}
