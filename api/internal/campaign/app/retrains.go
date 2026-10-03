package app

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// RetrainInput is a rebuilt build: origin and ability scores, class skills, the values of the picks made
// on each level, and the Ability Score Improvements.
type RetrainInput struct {
	Species    string
	Background string
	Method     string
	Base       map[string]int
	Bonus      map[string]int
	Skills     []string
	Picks      []domain.Pick
	Increase   map[string]int
	Reason     string
}

// RequestRetrain asks the DM to rebuild a Character the caller owns. The rebuilt build must follow the
// rules and keep every pick the Character made, with new values allowed.
func (s *Characters) RequestRetrain(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, in RetrainInput) (domain.Retrain, error) {
	sheet, err := s.Get(ctx, c, id, ch)
	if err != nil {
		return domain.Retrain{}, err
	}
	if !sheet.Mine {
		return domain.Retrain{}, domain.ErrForbidden
	}
	reason := ""
	if strings.TrimSpace(in.Reason) != "" {
		if reason, err = cleanText(in.Reason, 500); err != nil {
			return domain.Retrain{}, err
		}
	}
	next := retrained(sheet.Character, snapshotOf(in))
	if _, err := s.checkRetrain(ctx, sheet, next); err != nil {
		return domain.Retrain{}, err
	}
	r := domain.Retrain{
		ID: uuid.New(), CharacterID: ch, Proposed: snapshotOf(in), Status: domain.RetrainPending, Reason: reason,
		RequestedBy: sheet.Owner.DisplayName, DecidedBy: "", CreatedAt: s.Now(), DecidedAt: nil,
	}
	r.Proposed.ID = uuid.New()
	err = s.Repo.InTx(ctx, func(tx Repository) error { return tx.InsertRetrain(ctx, id, r, r.CreatedAt) })
	if errors.Is(err, domain.ErrConflict) {
		return domain.Retrain{}, refuse("a retrain is already waiting for the DM")
	}
	return r, err
}

func snapshotOf(in RetrainInput) domain.Snapshot {
	return domain.Snapshot{
		ID: uuid.Nil, Species: in.Species, Background: in.Background, Method: in.Method, Base: in.Base, Bonus: in.Bonus,
		Increase: in.Increase, Skills: in.Skills, Picks: in.Picks, CreatedAt: time.Time{},
	}
}

// snapshotFrom keeps a Character's current rebuildable choices.
func snapshotFrom(c domain.Character) domain.Snapshot {
	return domain.Snapshot{
		ID: uuid.New(), Species: c.Species, Background: c.Background, Method: c.Method, Base: maps.Clone(c.Base), Bonus: maps.Clone(c.Bonus),
		Increase: maps.Clone(c.Increase), Skills: slices.Clone(c.Skills), Picks: slices.Clone(c.Picks), CreatedAt: time.Time{},
	}
}

// retrained is a Character rebuilt from a snapshot, keeping its classes, level, equipment and story.
func retrained(c domain.Character, sn domain.Snapshot) domain.Character {
	c.Species, c.Background, c.Method = sn.Species, sn.Background, sn.Method
	c.Base, c.Bonus, c.Increase = orEmpty(sn.Base), orEmpty(sn.Bonus), orEmpty(sn.Increase)
	c.Skills, c.Picks = sn.Skills, sn.Picks
	return c
}

func orEmpty(m map[string]int) map[string]int {
	if m == nil {
		return map[string]int{}
	}
	return m
}

// checkRetrain checks a rebuilt Character against the rules and returns it with its hit point maximum
// adjusted for any change to its Constitution modifier.
func (s *Characters) checkRetrain(ctx context.Context, sheet Sheet, next domain.Character) (domain.Character, error) {
	o, err := s.options(ctx, next.CampaignID, next.Ruleset)
	if err != nil {
		return next, err
	}
	built, err := derive(o, next)
	if err != nil {
		return next, err
	}
	if err := samePicks(sheet.Picks, next.Picks); err != nil {
		return next, err
	}
	asis := 0
	for _, p := range next.Picks {
		if p.Value == asi {
			asis++
		}
	}
	unimproved := maps.Clone(built.Scores)
	for a, n := range next.Increase {
		unimproved[rules.Ability(a)] -= n
	}
	if err := rules.CheckIncreases(unimproved, abilityMap(next.Increase), asis); err != nil {
		return next, invalid(err)
	}
	if err := s.checkPicks(ctx, o, next); err != nil {
		return next, err
	}
	conDelta := rules.Modifier(built.Scores[rules.Constitution]) - rules.Modifier(sheet.Scores[rules.Constitution])
	next.HPMax = max(1, sheet.HPMax+conDelta*sheet.Level)
	next.BackgroundSkills = built.BackgroundSkills
	return next, nil
}

// samePicks refuses a retrain that adds or drops a pick: it may change values, not which choices were
// made on which level.
func samePicks(had, want []domain.Pick) error {
	count := func(ps []domain.Pick) map[string]int {
		out := map[string]int{}
		for _, p := range ps {
			out[strings.Join([]string{strconv.Itoa(p.Level), p.Choice}, "/")]++
		}
		return out
	}
	if !maps.Equal(count(had), count(want)) {
		return refuse("a retrain keeps every choice made on each level and changes only what was chosen")
	}
	return nil
}

// checkPicks checks each picked value against the options its choice offers the rebuilt Character, as
// if that one choice were still open.
func (s *Characters) checkPicks(ctx context.Context, o compendium.BuilderOptions, next domain.Character) error {
	s, err := s.within(ctx, next.CampaignID)
	if err != nil {
		return err
	}
	cat, err := s.Compendium.Features(ctx)
	if err != nil {
		return err
	}
	feats := map[string]bool{}
	for i, p := range next.Picks {
		opts, ch, err := s.pickOptions(ctx, o, cat, next, p)
		if err != nil {
			return err
		}
		opt, ok := find(opts, func(x LevelUpOption) bool { return x.Slug == p.Value })
		switch {
		case !ok:
			return refuse(p.Value + " is not an option for " + ch.Name)
		case len(opt.Unmet) > 0:
			return refuse(opt.Name + " needs " + strings.Join(opt.Unmet, ", "))
		case ch.Pool == features.FeatCategory && p.Value != asi && feats[p.Value]:
			return refuse("take each feat once")
		case slices.ContainsFunc(next.Picks[:i], func(x domain.Pick) bool { return x == p }):
			return refuse("choose each option once")
		}
		feats[p.Value] = ch.Pool == features.FeatCategory
	}
	return nil
}

// RetrainChoice is one pick a Character made, with every option it could take instead.
type RetrainChoice struct {
	Level   int
	Choice  string
	Name    string
	Value   string
	Options []LevelUpOption
}

// RetrainChoices lists the picks a Character can change in a retrain, with their options; its owner.
func (s *Characters) RetrainChoices(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) ([]RetrainChoice, error) {
	sheet, err := s.Get(ctx, c, id, ch)
	if err != nil {
		return nil, err
	}
	if !sheet.Mine {
		return nil, domain.ErrForbidden
	}
	s, err = s.within(ctx, id)
	if err != nil {
		return nil, err
	}
	o, err := s.Compendium.BuilderOptions(ctx, sheet.Ruleset)
	if err != nil {
		return nil, err
	}
	cat, err := s.Compendium.Features(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RetrainChoice, 0, len(sheet.Picks))
	for _, p := range sheet.Picks {
		opts, def, err := s.pickOptions(ctx, o, cat, sheet.Character, p)
		if err != nil {
			return nil, err
		}
		out = append(out, RetrainChoice{Level: p.Level, Choice: p.Choice, Name: def.Name, Value: p.Value, Options: opts})
	}
	return out, nil
}

// pickOptions are the options a pick's choice offers the Character, as if that one choice were still open.
func (s *Characters) pickOptions(ctx context.Context, o compendium.BuilderOptions, cat features.Catalog, c domain.Character, p domain.Pick) ([]LevelUpOption, features.Choice, error) {
	def, class, ok := choiceDef(cat, c.Classes, p.Choice)
	if !ok {
		return nil, def, refuse("this Character has no choice " + p.Choice)
	}
	lu, err := s.Compendium.LevelUpOptions(ctx, c.Ruleset, class, 0)
	if err != nil {
		return nil, def, err
	}
	c.Picks = slices.DeleteFunc(slices.Clone(c.Picks), func(x domain.Pick) bool { return x.Level == p.Level && x.Choice == p.Choice })
	open, err := derive(o, c)
	if err != nil {
		return nil, def, err
	}
	open.Level = p.Level - 1
	return optionsFor(cat, lu, open, def), def, nil
}

// choiceDef finds a choice by slug among the Character's classes and subclasses.
func choiceDef(cat features.Catalog, classes []domain.ClassLevel, slug string) (features.Choice, string, bool) {
	for _, x := range classes {
		for _, owner := range []features.Owner{{Kind: "class", Slug: x.Class}, {Kind: "subclass", Slug: x.Subclass}} {
			if c, ok := find(cat.Choices[owner], func(c features.Choice) bool { return c.Slug == slug }); ok {
				return c, x.Class, true
			}
		}
	}
	return features.Choice{}, "", false
}

// Retrains lists a Character's retrains; its owner or a DM.
func (s *Characters) Retrains(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) ([]domain.Retrain, error) {
	if err := s.ownerOrDM(ctx, c, id, ch); err != nil {
		return nil, err
	}
	return s.Repo.Retrains(ctx, id, ch)
}

// CharacterRevisions lists the builds a Character's retrains replaced; its owner or a DM.
func (s *Characters) CharacterRevisions(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) ([]domain.CharacterRevision, error) {
	if err := s.ownerOrDM(ctx, c, id, ch); err != nil {
		return nil, err
	}
	return s.Repo.CharacterRevisions(ctx, id, ch)
}

func (s *Characters) ownerOrDM(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) error {
	sheet, err := s.Get(ctx, c, id, ch)
	if err != nil {
		return err
	}
	me, err := member(ctx, s.Repo, c, id)
	if err != nil {
		return err
	}
	if !sheet.Mine && me.Role != domain.RoleDM {
		return domain.ErrForbidden
	}
	return nil
}

// DecideRetrain approves or declines a pending retrain; DM only. Approving checks the build again against
// the Character as it is now, keeps the old build as a Revision and applies the new one.
func (s *Characters) DecideRetrain(ctx context.Context, c caller.Caller, id domain.CampaignID, retrain uuid.UUID, approve bool) (domain.Retrain, error) {
	me, err := member(ctx, s.Repo, c, id)
	if err != nil {
		return domain.Retrain{}, err
	}
	if me.Role != domain.RoleDM {
		return domain.Retrain{}, domain.ErrForbidden
	}
	r, err := s.Repo.Retrain(ctx, id, retrain)
	if err != nil {
		return domain.Retrain{}, err
	}
	now := s.Now()
	if !approve {
		err = s.Repo.InTx(ctx, func(tx Repository) error {
			return tx.DecideRetrain(ctx, retrain, domain.RetrainDeclined, me.DisplayName, now)
		})
		return s.decided(ctx, id, retrain, err)
	}
	sheet, err := s.Get(ctx, c, id, r.CharacterID)
	if err != nil {
		return domain.Retrain{}, err
	}
	next, err := s.checkRetrain(ctx, sheet, retrained(sheet.Character, r.Proposed))
	if err != nil {
		return domain.Retrain{}, err
	}
	err = s.Repo.InTx(ctx, func(tx Repository) error {
		if err := tx.DecideRetrain(ctx, retrain, domain.RetrainApproved, me.DisplayName, now); err != nil {
			return err
		}
		return tx.ApplyRetrain(ctx, id, snapshotFrom(sheet.Character), next, retrain, c, me.DisplayName, now)
	})
	return s.decided(ctx, id, retrain, err)
}

func (s *Characters) decided(ctx context.Context, id domain.CampaignID, retrain uuid.UUID, err error) (domain.Retrain, error) {
	if errors.Is(err, domain.ErrConflict) {
		return domain.Retrain{}, refuse("this retrain was already decided")
	}
	if err != nil {
		return domain.Retrain{}, err
	}
	return s.Repo.Retrain(ctx, id, retrain)
}
