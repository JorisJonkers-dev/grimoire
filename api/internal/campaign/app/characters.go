package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Compendium is the compendium read port the builder needs.
type Compendium interface {
	BuilderOptions(ctx context.Context, ruleset string) (compendium.BuilderOptions, error)
}

// CombatStatus says whether a Character is in an active Combat, which locks its sheet.
type CombatStatus interface {
	InCombat(ctx context.Context, id domain.CharacterID) (bool, error)
}

// NoCombat is the CombatStatus until live Combat exists: nobody is ever in one.
type NoCombat struct{}

// InCombat implements CombatStatus.
func (NoCombat) InCombat(context.Context, domain.CharacterID) (bool, error) { return false, nil }

// Characters runs the character use cases.
type Characters struct {
	Repo       Repository
	Compendium Compendium
	Combat     CombatStatus
	Blobs      Blobs
	Now        func() time.Time
	// Roll rolls six ability scores for a draft; nil means the Campaign cannot roll.
	Roll func() []int
}

// Sheet is a Character with everything the sheet shows derived from the rules.
type Sheet struct {
	domain.Character
	ClassName      string
	SpeciesName    string
	BackgroundName string
	Scores         map[rules.Ability]int
	Derived        rules.Sheet
	Armor          *compendium.ArmorOption
	Weapons        []compendium.WeaponOption
	Mine           bool
	Editable       bool
}

// MaxStartingWeapons caps the weapons a new character carries.
const MaxStartingWeapons = 4

func invalid(err error) error {
	var v *rules.ViolationError
	if errors.As(err, &v) {
		return &RuleError{Reason: v.Reason}
	}
	return err
}

// RuleError is a build the rules refuse, with a reason for the player.
type RuleError = apperr.RuleError

func refuse(reason string) error { return apperr.Refuse(reason) }

func find[T any](list []T, match func(T) bool) (T, bool) {
	for _, x := range list {
		if match(x) {
			return x, true
		}
	}
	var zero T
	return zero, false
}

func abilityMap(m map[string]int) map[rules.Ability]int {
	out := make(map[rules.Ability]int, len(m))
	for k, v := range m {
		out[rules.Ability(k)] = v
	}
	return out
}

func toSkills(in []string) []rules.Skill {
	out := make([]rules.Skill, 0, len(in))
	for _, s := range in {
		out = append(out, rules.Skill(s))
	}
	return out
}

func toAbilities(in []string) []rules.Ability {
	out := make([]rules.Ability, 0, len(in))
	for _, s := range in {
		out = append(out, rules.Ability(s))
	}
	return out
}

// derive validates a build against the ruleset and computes the sheet.
func derive(o compendium.BuilderOptions, c domain.Character) (Sheet, error) {
	class, ok := find(o.Classes, func(x compendium.ClassOption) bool { return x.Slug == c.Class })
	if !ok {
		return Sheet{}, refuse("choose a class from this ruleset")
	}
	species, ok := find(o.Species, func(x compendium.SpeciesOption) bool { return x.Slug == c.Species })
	if !ok {
		return Sheet{}, refuse("choose a species from this ruleset")
	}
	background, ok := find(o.Backgrounds, func(x compendium.BackgroundOption) bool { return x.Slug == c.Background })
	if !ok {
		return Sheet{}, refuse("choose a background from this ruleset")
	}
	scores, err := abilities(c.Build, background, o.RulesetYear)
	if err != nil {
		return Sheet{}, err
	}
	if err := rules.ValidateSkills(c.Class, toSkills(c.Skills), toSkills(background.Skills)); err != nil {
		return Sheet{}, invalid(err)
	}
	armor, shield, weapons, err := equipment(o, c.Build)
	if err != nil {
		return Sheet{}, err
	}
	var worn *rules.Armor
	if armor != nil {
		worn = &rules.Armor{Base: armor.ACBase, AddDex: armor.AddDex, DexCap: armor.DexCap, StrengthRequired: armor.StrengthRequired, Stealth: armor.Stealth}
	}
	derived := rules.BuildSheet(rules.SheetInput{
		Class: c.Class, Level: max(c.Level, 1), HitDie: class.HitDie, Scores: scores,
		SaveProfs: toAbilities(class.Saves), SkillProfs: toSkills(append(slices.Clone(c.Skills), background.Skills...)),
		Armor: worn, ShieldBonus: shield, SpeedFeet: species.SpeedFeet,
	})
	c.BackgroundSkills = background.Skills
	if c.HPMax == 0 {
		c.HPMax = rules.HitPointsAt(class.HitDie, rules.Modifier(scores[rules.Constitution]), max(c.Level, 1))
		c.HPCurrent = c.HPMax
	}
	return Sheet{
		Character: c, ClassName: class.Name, SpeciesName: species.Name, BackgroundName: background.Name, Scores: scores,
		Derived: derived, Armor: armor, Weapons: weapons,
	}, nil
}

func abilities(b domain.Build, background compendium.BackgroundOption, year int) (map[rules.Ability]int, error) {
	if err := rules.ValidateBase(rules.Method(b.Method), abilityMap(b.Base)); err != nil {
		return nil, invalid(err)
	}
	allowed := []rules.Ability{}
	if year >= 2024 {
		allowed = toAbilities(background.Abilities)
	}
	if err := rules.ValidateOriginBonuses(abilityMap(b.Bonus), allowed); err != nil {
		return nil, invalid(err)
	}
	scores, err := rules.FinalScores(abilityMap(b.Base), abilityMap(b.Bonus))
	return scores, invalid(err)
}

func equipment(o compendium.BuilderOptions, b domain.Build) (*compendium.ArmorOption, int, []compendium.WeaponOption, error) {
	var armor *compendium.ArmorOption
	if b.Armor != "" {
		a, ok := find(o.Armor, func(x compendium.ArmorOption) bool { return x.Slug == b.Armor && !x.Shield })
		if !ok {
			return nil, 0, nil, refuse("choose body armour from this ruleset")
		}
		armor = &a
	}
	shield := 0
	if b.Shield {
		s, ok := find(o.Armor, func(x compendium.ArmorOption) bool { return x.Shield })
		if !ok {
			return nil, 0, nil, refuse("this ruleset has no shield")
		}
		shield = s.ACBase
	}
	if len(b.Weapons) > MaxStartingWeapons {
		return nil, 0, nil, refuse("start with at most four weapons")
	}
	weapons := make([]compendium.WeaponOption, 0, len(b.Weapons))
	for _, slug := range b.Weapons {
		w, ok := find(o.Weapons, func(x compendium.WeaponOption) bool { return x.Slug == slug })
		if !ok || slices.ContainsFunc(weapons, func(x compendium.WeaponOption) bool { return x.Slug == slug }) {
			return nil, 0, nil, refuse("choose each weapon once, from this ruleset")
		}
		weapons = append(weapons, w)
	}
	return armor, shield, weapons, nil
}

// prepare checks a build against the Campaign's rules and makes its sheet at the Campaign's starting
// level. A fresh build must also use a method the Campaign allows, and rolled scores the server rolled.
func (s *Characters) prepare(ctx context.Context, c caller.Caller, id domain.CampaignID, b domain.Build, fresh bool) (Sheet, error) {
	me, err := member(ctx, s.Repo, c, id)
	if err != nil {
		return Sheet{}, err
	}
	name, err := cleanText(b.Name, 60)
	if err != nil {
		return Sheet{}, err
	}
	b.Name = name
	if b, err = cleanStory(b); err != nil {
		return Sheet{}, err
	}
	camp, err := s.Repo.GetCampaign(ctx, id)
	if err != nil {
		return Sheet{}, err
	}
	if fresh {
		if err := s.allowed(ctx, camp, c.Subject, b); err != nil {
			return Sheet{}, err
		}
	}
	o, err := s.Compendium.BuilderOptions(ctx, camp.Ruleset)
	if err != nil {
		return Sheet{}, err
	}
	sheet, err := derive(o, domain.Character{Build: b, CampaignID: id, Owner: me, Ruleset: o.Ruleset, Level: max(camp.StartingLevel, 1)})
	if err != nil {
		return Sheet{}, err
	}
	sheet.Mine, sheet.Editable = true, true
	return sheet, nil
}

// allowed checks a new build's ability scores come the way the Campaign allows.
func (s *Characters) allowed(ctx context.Context, camp domain.Campaign, subject string, b domain.Build) error {
	if len(camp.CreationMethods) > 0 && !slices.Contains(camp.CreationMethods, b.Method) {
		return refuse("this Campaign sets ability scores another way")
	}
	if b.Method != string(rules.Rolled) {
		return nil
	}
	draft, err := s.Repo.Draft(ctx, camp.ID, subject)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && draft.Rolled == nil) {
		return refuse("roll your ability scores first")
	}
	if err != nil {
		return err
	}
	return invalid(rules.ValidateRolled(abilityMap(b.Base), draft.Rolled))
}

// cleanStory trims a build's Appearance and Backstory and keeps them to their lengths.
func cleanStory(b domain.Build) (domain.Build, error) {
	b.Appearance, b.Backstory = strings.TrimSpace(b.Appearance), strings.TrimSpace(b.Backstory)
	if utf8.RuneCountInString(b.Appearance) > 2000 || utf8.RuneCountInString(b.Backstory) > 4000 {
		return b, domain.ErrInvalid
	}
	return b, nil
}

// Preview validates a build and shows the sheet it would make, without saving it.
func (s *Characters) Preview(ctx context.Context, c caller.Caller, id domain.CampaignID, b domain.Build) (Sheet, error) {
	return s.prepare(ctx, c, id, b, true)
}

// Create saves a Character owned by the caller at the Campaign's starting level, and drops the draft it
// was made from.
func (s *Characters) Create(ctx context.Context, c caller.Caller, id domain.CampaignID, b domain.Build) (Sheet, error) {
	sheet, err := s.prepare(ctx, c, id, b, true)
	if err != nil {
		return Sheet{}, err
	}
	cid, err := s.Repo.InsertCharacter(ctx, sheet.Character, s.Now())
	if err != nil {
		return Sheet{}, err
	}
	stored, err := s.Repo.Character(ctx, id, cid)
	if err != nil {
		return Sheet{}, err
	}
	sheet.ID, sheet.Owned = cid, stored.Owned
	return sheet, s.Repo.DeleteDraft(ctx, id, c.Subject)
}

// List returns the party's Characters.
func (s *Characters) List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.CharacterSummary, error) {
	if _, err := member(ctx, s.Repo, c, id); err != nil {
		return nil, err
	}
	all, err := s.Repo.Characters(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CharacterSummary, 0, len(all))
	for _, ch := range all {
		out = append(out, domain.CharacterSummary{
			ID: ch.ID, Name: ch.Name, OwnerName: ch.Owner.DisplayName, Mine: ch.Owner.Subject == c.Subject,
			Species: ch.Species, Class: ch.Class, Level: ch.Level, HPCurrent: ch.HPCurrent, HPMax: ch.HPMax, TokenKey: tokenKey(ch),
		})
	}
	return out, nil
}

// Get returns a Character's sheet to any Member of its Campaign.
func (s *Characters) Get(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) (Sheet, error) {
	me, err := member(ctx, s.Repo, c, id)
	if err != nil {
		return Sheet{}, err
	}
	stored, err := s.Repo.Character(ctx, id, ch)
	if err != nil {
		return Sheet{}, err
	}
	o, err := s.Compendium.BuilderOptions(ctx, stored.Ruleset)
	if err != nil {
		return Sheet{}, err
	}
	sheet, err := derive(o, stored)
	if err != nil {
		return Sheet{}, err
	}
	inCombat, err := s.Combat.InCombat(ctx, ch)
	if err != nil {
		return Sheet{}, err
	}
	sheet.Mine = stored.Owner.ID == me.ID
	sheet.Editable = !inCombat && (sheet.Mine || me.Role == domain.RoleDM)
	return sheet, nil
}

// Edit is an out-of-combat change to a Character; nil leaves a field alone.
type Edit struct {
	Name      *string
	HPCurrent *int
	Armor     *string
	Shield    *bool
	Weapons   []string
}

// editable loads a sheet the caller may change right now.
func (s *Characters) editable(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) (Sheet, error) {
	sheet, err := s.Get(ctx, c, id, ch)
	if err != nil {
		return Sheet{}, err
	}
	if !sheet.Editable {
		inCombat, err := s.Combat.InCombat(ctx, ch)
		if err != nil {
			return Sheet{}, err
		}
		if inCombat {
			return Sheet{}, domain.ErrLocked
		}
		return Sheet{}, domain.ErrForbidden
	}
	return sheet, nil
}

// Update applies an Edit. The owner or a DM, and never during Combat.
func (s *Characters) Update(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, e Edit) (Sheet, error) {
	sheet, err := s.editable(ctx, c, id, ch)
	if err != nil {
		return Sheet{}, err
	}
	next := sheet.Character
	if e.Name != nil {
		if next.Name, err = cleanText(*e.Name, 60); err != nil {
			return Sheet{}, err
		}
	}
	if e.HPCurrent != nil {
		if *e.HPCurrent < 0 || *e.HPCurrent > next.HPMax {
			return Sheet{}, refuse("hit points run from 0 to the maximum")
		}
		next.HPCurrent = *e.HPCurrent
	}
	if e.Armor != nil {
		next.Armor = *e.Armor
	}
	if e.Shield != nil {
		next.Shield = *e.Shield
	}
	if e.Weapons != nil {
		next.Weapons = e.Weapons
	}
	o, err := s.Compendium.BuilderOptions(ctx, next.Ruleset)
	if err != nil {
		return Sheet{}, err
	}
	if _, err := derive(o, next); err != nil {
		return Sheet{}, err
	}
	if err := s.Repo.UpdateCharacter(ctx, next, s.Now()); err != nil {
		return Sheet{}, err
	}
	return s.Get(ctx, c, id, ch)
}

// Delete removes a Character. The owner or a DM, and never during Combat.
func (s *Characters) Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) error {
	if _, err := s.editable(ctx, c, id, ch); err != nil {
		return err
	}
	return s.Repo.DeleteCharacter(ctx, id, ch)
}

func tokenKey(c domain.Character) string {
	if c.Token == nil {
		return ""
	}
	return c.Token.Key
}
