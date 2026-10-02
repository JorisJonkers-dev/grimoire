package httpapi

import (
	"context"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// CharacterService is what the character operations need.
type CharacterService interface {
	Preview(ctx context.Context, c caller.Caller, id domain.CampaignID, b domain.Build) (app.Sheet, error)
	Create(ctx context.Context, c caller.Caller, id domain.CampaignID, b domain.Build) (app.Sheet, error)
	List(ctx context.Context, c caller.Caller, id domain.CampaignID) ([]domain.CharacterSummary, error)
	Get(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) (app.Sheet, error)
	Update(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, e app.Edit) (app.Sheet, error)
	Delete(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) error
	SetImage(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind, data []byte) error
	ClearToken(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) error
	Image(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind) (domain.Image, []byte, error)
	Mine(ctx context.Context, c caller.Caller) ([]domain.OwnedCharacter, error)
	Owned(ctx context.Context, c caller.Caller, id domain.OwnedID) (domain.OwnedCharacter, error)
	UpdateOwned(ctx context.Context, c caller.Caller, id domain.OwnedID, name, backstory string) (domain.OwnedCharacter, error)
	Join(ctx context.Context, c caller.Caller, id domain.OwnedID, campaign domain.CampaignID) (app.Sheet, error)
	Draft(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Draft, error)
	SaveDraft(ctx context.Context, c caller.Caller, id domain.CampaignID, step int, build []byte) (domain.Draft, error)
	DiscardDraft(ctx context.Context, c caller.Caller, id domain.CampaignID) error
	RollScores(ctx context.Context, c caller.Caller, id domain.CampaignID) (domain.Draft, error)
	PlanLevelUp(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, class string) (app.LevelUpPlan, error)
	LevelUp(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, req app.LevelUpRequest) (app.Sheet, error)
	Spells(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) (app.Spellcasting, error)
	Prepare(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, class string, spells []string) (app.Spellcasting, error)
	CastRitual(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, spell string) (app.Ritual, error)
	CopySpell(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, spell string) (app.Spellcasting, error)
	PassInspiration(ctx context.Context, c caller.Caller, id domain.CampaignID, ch, to domain.CharacterID) (app.Sheet, error)
	RequestRetrain(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID, in app.RetrainInput) (domain.Retrain, error)
	Retrains(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) ([]domain.Retrain, error)
	RetrainChoices(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) ([]app.RetrainChoice, error)
	CharacterRevisions(ctx context.Context, c caller.Caller, id domain.CampaignID, ch domain.CharacterID) ([]domain.CharacterRevision, error)
	DecideRetrain(ctx context.Context, c caller.Caller, id domain.CampaignID, retrain uuid.UUID, approve bool) (domain.Retrain, error)
}

func baseMap(b oas.AbilityBase) map[string]int {
	return map[string]int{
		"strength": int(b.Strength), "dexterity": int(b.Dexterity), "constitution": int(b.Constitution),
		"intelligence": int(b.Intelligence), "wisdom": int(b.Wisdom), "charisma": int(b.Charisma),
	}
}

func bonusMap(b oas.AbilityBonus) map[string]int {
	out := map[string]int{}
	for name, v := range map[string]oas.OptInt32{
		"strength": b.Strength, "dexterity": b.Dexterity, "constitution": b.Constitution,
		"intelligence": b.Intelligence, "wisdom": b.Wisdom, "charisma": b.Charisma,
	} {
		if n, ok := v.Get(); ok {
			out[name] = int(n)
		}
	}
	return out
}

func slugList(in []oas.Slug) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, string(s))
	}
	return out
}

func buildIn(b *oas.CharacterBuild) domain.Build {
	return domain.Build{
		Name: string(b.Name), Species: string(b.Species), Class: string(b.Class), Background: string(b.Background),
		Method: string(b.Method), Base: baseMap(b.Base), Bonus: bonusMap(b.Bonus), Skills: slugList(b.Skills),
		Armor: string(b.Armor.Or("")), Shield: b.Shield, Weapons: slugList(b.Weapons),
		Appearance: b.Appearance.Or(""), Backstory: b.Backstory.Or(""),
	}
}

//nolint:gosec // every narrowing conversion here is of small, bounded game values
func sheetOut(s app.Sheet) oas.CharacterSheet {
	out := oas.CharacterSheet{
		Name: oas.CharacterName(s.Name), Ruleset: oas.Ruleset(s.Ruleset), Level: int32(max(s.Level, 1)),
		OwnerName: oas.DisplayName(s.Owner.DisplayName), Mine: s.Mine, Editable: s.Editable,
		Species:    oas.NamedRef{Slug: oas.Slug(s.Species), Name: s.SpeciesName},
		Class:      oas.NamedRef{Slug: oas.Slug(s.Class), Name: s.ClassName},
		Background: oas.NamedRef{Slug: oas.Slug(s.Background), Name: s.BackgroundName},
		Method:     oas.CharacterSheetMethod(s.Method), Base: oas.AbilityBase{
			Strength: int32(s.Base["strength"]), Dexterity: int32(s.Base["dexterity"]), Constitution: int32(s.Base["constitution"]),
			Intelligence: int32(s.Base["intelligence"]), Wisdom: int32(s.Base["wisdom"]), Charisma: int32(s.Base["charisma"]),
		},
		Abilities: make([]oas.AbilityLine, 0, 6), Skills: make([]oas.SkillLine, 0, 18),
		ClassSkills: slugsOf(s.Skills), BackgroundSkills: slugsOf(s.BackgroundSkills),
		HpCurrent: int32(s.HPCurrent), HpMax: int32(s.HPMax), TempHp: oas.NewOptInt32(int32(s.TempHP)), ArmorClass: int32(s.Derived.ArmorClass),
		Initiative: int32(s.Derived.Initiative), SpeedFeet: int32(s.Derived.SpeedFeet), ProficiencyBonus: int32(s.Derived.ProficiencyBonus),
		PassivePerception: int32(s.Derived.PassivePerception), Shield: s.Shield, Weapons: make([]oas.WeaponLine, 0, len(s.Weapons)),
		Resources: make([]oas.ResourcePool, 0, len(s.Derived.Resources)), Effects: []oas.ActiveEffect{}, Warnings: s.Derived.Warnings,
	}
	if s.ID != (domain.CharacterID{}) {
		out.ID = oas.NewOptID(oas.ID(s.ID))
		if s.Portrait != nil {
			out.PortraitUrl = oas.NewOptAssetUrl(assetURL(s.CampaignID, s.ID, domain.Portrait, s.Portrait.Key))
		}
		if s.Token != nil {
			out.TokenUrl = oas.NewOptAssetUrl(assetURL(s.CampaignID, s.ID, domain.TokenIcon, s.Token.Key))
		}
	}
	out.Bonus = bonusOut(s.Bonus)
	for _, sv := range s.Derived.Saves {
		out.Abilities = append(out.Abilities, oas.AbilityLine{
			Ability: oas.Ability(sv.Ability), Score: int32(sv.Score), Modifier: int32(sv.Modifier), Save: int32(sv.Bonus), SaveProficient: sv.Proficient,
		})
	}
	for _, sk := range s.Derived.Skills {
		out.Skills = append(out.Skills, oas.SkillLine{Skill: oas.Slug(sk.Skill), Ability: oas.Ability(sk.Ability), Bonus: int32(sk.Bonus), Proficient: sk.Proficient, Expertise: sk.Expertise})
	}
	if s.Armor != nil {
		out.Armor = oas.NewOptNamedRef(oas.NamedRef{Slug: oas.Slug(s.Armor.Slug), Name: s.Armor.Name})
	}
	for _, w := range s.Weapons {
		out.Weapons = append(out.Weapons, weaponOut(w))
	}
	for _, r := range s.Derived.Resources {
		out.Resources = append(out.Resources, oas.ResourcePool{Key: oas.Slug(r.Key), Label: r.Label, Current: int32(r.Current), Max: int32(r.Max)})
	}
	extrasOut(&out, s)
	return out
}

// extrasOut adds what only a saved Character's sheet carries: its own Character, attacks, traits and
// training.
//
//nolint:gosec // values are bounded by the rules
func extrasOut(out *oas.CharacterSheet, s app.Sheet) {
	if s.Owned != (domain.OwnedID{}) {
		out.CharacterId = oas.NewOptID(oas.ID(s.Owned))
	}
	out.LevelUpReady = oas.NewOptBool(s.LevelUpReady && s.Level < 20)
	out.HeroicInspiration = oas.NewOptBool(s.HeroicInspiration)
	out.Increase = oas.NewOptAbilityIncrease(increaseOut(s.Increase))
	for _, x := range s.Classes {
		line := oas.ClassLine{Slug: oas.Slug(x.Class), Name: s.ClassNames[x.Class], Level: int32(x.Level)}
		if x.Subclass != "" {
			line.Subclass = oas.NewOptSlug(oas.Slug(x.Subclass))
		}
		out.Classes = append(out.Classes, line)
	}
	for _, sp := range s.Spells {
		out.Spells = append(out.Spells, oas.LearnedSpellLine{Slug: oas.Slug(sp.Spell), Class: oas.Slug(sp.Class)})
	}
	for _, a := range s.Attacks {
		line := oas.AttackLine{
			Name: a.Name, ToHit: int32(a.ToHit), Damage: a.Damage, DamageType: a.DamageType,
			ReachFeet: int32(a.ReachFt), RangeFeet: int32(a.RangeFt), LongRangeFeet: int32(a.LongRangeFt),
		}
		if a.Mastery != "" {
			line.Mastery = oas.NewOptString(a.Mastery)
		}
		out.Attacks = append(out.Attacks, line)
	}
	for _, t := range s.Traits {
		out.Traits = append(out.Traits, oas.TraitLine{Name: t.Name, Source: oas.TraitLineSource(t.Source), Level: int32(t.Level), Description: t.Description})
	}
	if s.Proficiencies.Armor != nil || s.Proficiencies.Weapons != nil {
		out.Proficiencies = oas.NewOptProficiencies(oas.Proficiencies{Armor: nonNil(s.Proficiencies.Armor), Weapons: nonNil(s.Proficiencies.Weapons)})
	}
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func bonusOut(m map[string]int) oas.AbilityBonus {
	var b oas.AbilityBonus
	fields := map[string]*oas.OptInt32{
		"strength": &b.Strength, "dexterity": &b.Dexterity, "constitution": &b.Constitution,
		"intelligence": &b.Intelligence, "wisdom": &b.Wisdom, "charisma": &b.Charisma,
	}
	for name, field := range fields {
		if v, ok := m[name]; ok {
			*field = oas.NewOptInt32(int32(v)) //nolint:gosec // 1..2
		}
	}
	return b
}

//nolint:gosec // ranges are small
func weaponOut(w compendium.WeaponOption) oas.WeaponLine {
	return oas.WeaponLine{
		Slug: oas.Slug(w.Slug), Name: w.Name, DamageDice: w.DamageDice, DamageType: w.DamageType,
		RangeFeet: int32(w.RangeFeet), LongRangeFeet: int32(w.LongRangeFeet),
	}
}

func optInt(v oas.OptInt32) *int {
	n, set := v.Get()
	if !set {
		return nil
	}
	i := int(n)
	return &i
}

func slugsOf(in []string) []oas.Slug {
	out := make([]oas.Slug, 0, len(in))
	for _, s := range in {
		out = append(out, oas.Slug(s))
	}
	return out
}

// GetBuilderOptions lists what a first-level character can choose.
func (h *Handler) GetBuilderOptions(ctx context.Context, p oas.GetBuilderOptionsParams) (oas.GetBuilderOptionsRes, error) {
	tag, err := h.etag(ctx)
	if err != nil {
		h.Log.ErrorContext(ctx, "compendium version", "error", err)
		return unavailable(), nil
	}
	if p.IfNoneMatch.Or("") == tag {
		return &oas.GetBuilderOptionsNotModified{ETag: oas.NewOptString(tag)}, nil
	}
	o, err := h.Compendium.BuilderOptions(ctx, string(p.Ruleset))
	if err != nil {
		h.Log.ErrorContext(ctx, "builder options", "error", err)
		return unavailable(), nil
	}
	return &oas.BuilderOptionsHeaders{ETag: oas.NewOptString(tag), Response: builderOut(o)}, nil
}

//nolint:gosec // every narrowing conversion here is of small, bounded game values
func builderOut(o compendium.BuilderOptions) oas.BuilderOptions {
	out := oas.BuilderOptions{
		Ruleset: oas.Ruleset(o.Ruleset), RulesetYear: int32(o.RulesetYear), PointBuyBudget: rules.PointBuyBudget,
		Classes: make([]oas.ClassChoice, 0, len(o.Classes)), Species: make([]oas.SpeciesChoice, 0, len(o.Species)),
		Backgrounds: make([]oas.BackgroundChoice, 0, len(o.Backgrounds)), Armor: make([]oas.ArmorOptionItem, 0, len(o.Armor)),
		Weapons: make([]oas.WeaponLine, 0, len(o.Weapons)), Skills: make([]oas.SkillChoice, 0, 18),
	}
	for _, c := range o.Classes {
		saves := make([]oas.Ability, 0, len(c.Saves))
		for _, a := range c.Saves {
			saves = append(saves, oas.Ability(a))
		}
		primary := make([]oas.Ability, 0, 2)
		for _, a := range rules.PrimaryAbilities(c.Slug) {
			primary = append(primary, oas.Ability(a))
		}
		out.Classes = append(out.Classes, oas.ClassChoice{
			Slug: oas.Slug(c.Slug), Name: c.Name, HitDie: int32(c.HitDie), Saves: saves, SkillChoices: int32(rules.ClassSkillCount(c.Slug)),
			PrimaryAbilities: primary, Caster: oas.NewOptClassChoiceCaster(oas.ClassChoiceCaster(rules.CasterFor(c.Slug))),
		})
	}
	for _, s := range o.Species {
		out.Species = append(out.Species, oas.SpeciesChoice{Slug: oas.Slug(s.Slug), Name: s.Name, SpeedFeet: int32(s.SpeedFeet)})
	}
	for _, b := range o.Backgrounds {
		abilities := make([]oas.Ability, 0, len(b.Abilities))
		for _, a := range b.Abilities {
			abilities = append(abilities, oas.Ability(a))
		}
		out.Backgrounds = append(out.Backgrounds, oas.BackgroundChoice{Slug: oas.Slug(b.Slug), Name: b.Name, Abilities: abilities, Skills: slugsOf(b.Skills)})
	}
	for _, a := range o.Armor {
		item := oas.ArmorOptionItem{
			Slug: oas.Slug(a.Slug), Name: a.Name, Category: a.Category, Shield: a.Shield, AcBase: int32(a.ACBase), AddDex: a.AddDex,
			StrengthRequired: int32(a.StrengthRequired), StealthDisadvantage: a.Stealth,
		}
		if a.DexCap >= 0 {
			item.DexCap = oas.NewOptInt32(int32(a.DexCap))
		}
		out.Armor = append(out.Armor, item)
	}
	for _, w := range o.Weapons {
		out.Weapons = append(out.Weapons, weaponOut(w))
	}
	for _, sk := range rules.Skills() {
		out.Skills = append(out.Skills, oas.SkillChoice{Skill: oas.Slug(sk.Skill), Ability: oas.Ability(sk.Ability)})
	}
	return out
}

// ListCharacters lists the party.
func (h *Handler) ListCharacters(ctx context.Context, p oas.ListCharactersParams) (oas.ListCharactersRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	list, err := h.Characters.List(ctx, c, domain.CampaignID(p.CampaignId))
	if err != nil {
		return h.campaignProblem(ctx, "list characters", err), nil
	}
	out := make([]oas.CharacterSummary, 0, len(list))
	for _, ch := range list {
		var token oas.OptAssetUrl
		if ch.TokenKey != "" {
			token = oas.NewOptAssetUrl(assetURL(domain.CampaignID(p.CampaignId), ch.ID, domain.TokenIcon, ch.TokenKey))
		}
		out = append(out, oas.CharacterSummary{
			TokenUrl: token,
			ID:       oas.ID(ch.ID), Name: oas.CharacterName(ch.Name), OwnerName: oas.DisplayName(ch.OwnerName), Mine: ch.Mine,
			Species: oas.Slug(ch.Species), Class: oas.Slug(ch.Class), Level: int32(ch.Level), //nolint:gosec // 1..20
			HpCurrent: int32(ch.HPCurrent), HpMax: int32(ch.HPMax), //nolint:gosec // hit points are small
			HeroicInspiration: oas.NewOptBool(ch.HeroicInspiration),
		})
	}
	return &oas.ListCharactersOKHeaders{Response: out}, nil
}

// PreviewCharacter validates a build without saving it.
func (h *Handler) PreviewCharacter(ctx context.Context, req *oas.CharacterBuild, p oas.PreviewCharacterParams) (oas.PreviewCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Characters.Preview(ctx, c, domain.CampaignID(p.CampaignId), buildIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "preview character", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}

// CreateCharacter saves a new Character.
func (h *Handler) CreateCharacter(ctx context.Context, req *oas.CharacterBuild, p oas.CreateCharacterParams) (oas.CreateCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Characters.Create(ctx, c, domain.CampaignID(p.CampaignId), buildIn(req))
	if err != nil {
		return h.campaignProblem(ctx, "create character", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}

// GetCharacter returns a sheet.
func (h *Handler) GetCharacter(ctx context.Context, p oas.GetCharacterParams) (oas.GetCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Characters.Get(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId))
	if err != nil {
		return h.campaignProblem(ctx, "get character", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}

// UpdateCharacter applies an out-of-combat edit.
func (h *Handler) UpdateCharacter(ctx context.Context, req *oas.CharacterEdit, p oas.UpdateCharacterParams) (oas.UpdateCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	e := app.Edit{HPCurrent: optInt(req.HpCurrent), Damage: optInt(req.Damage), Heal: optInt(req.Heal), TempHP: optInt(req.TempHp)}
	if v, set := req.LevelUpReady.Get(); set {
		e.LevelUpReady = &v
	}
	if v, set := req.HeroicInspiration.Get(); set {
		e.HeroicInspiration = &v
	}
	if v, set := req.Name.Get(); set {
		name := string(v)
		e.Name = &name
	}
	if v, set := req.Armor.Get(); set {
		armor := string(v)
		e.Armor = &armor
	}
	if v, set := req.Shield.Get(); set {
		e.Shield = &v
	}
	if req.Weapons != nil {
		e.Weapons = slugList(req.Weapons)
	}
	s, err := h.Characters.Update(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), e)
	if err != nil {
		return h.campaignProblem(ctx, "update character", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}

// DeleteCharacter removes a Character.
func (h *Handler) DeleteCharacter(ctx context.Context, p oas.DeleteCharacterParams) (oas.DeleteCharacterRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	if err := h.Characters.Delete(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId)); err != nil {
		return h.campaignProblem(ctx, "delete character", err), nil
	}
	return &oas.DeleteCharacterNoContent{}, nil
}

// PassInspiration gives a Character's Heroic Inspiration to another Character.
func (h *Handler) PassInspiration(ctx context.Context, req *oas.InspirationPass, p oas.PassInspirationParams) (oas.PassInspirationRes, error) {
	c, ok := uiCaller(ctx)
	if !ok {
		return unauthorized(), nil
	}
	s, err := h.Characters.PassInspiration(ctx, c, domain.CampaignID(p.CampaignId), domain.CharacterID(p.CharacterId), domain.CharacterID(req.To))
	if err != nil {
		return h.campaignProblem(ctx, "pass inspiration", err), nil
	}
	return &oas.CharacterSheetHeaders{Response: sheetOut(s)}, nil
}
