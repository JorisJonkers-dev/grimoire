package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
)

func optSlug(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }

// InsertCharacter stores a new Character with its scores, skills and weapons in one transaction.
func (s *Store) InsertCharacter(ctx context.Context, c domain.Character, now time.Time) (domain.CharacterID, error) {
	var id uuid.UUID
	err := s.InTx(ctx, func(r app.Repository) error {
		q := r.(*Store).q
		owned := uuid.UUID(c.Owned)
		if owned == uuid.Nil {
			owned = uuid.New()
			if err := q.InsertAccountCharacter(ctx, queries.InsertAccountCharacterParams{
				ID: owned, OwnerSubject: c.Owner.Subject, Name: c.Name, Ruleset: c.Ruleset, SpeciesSlug: c.Species,
				ClassSlug: c.Class, BackgroundSlug: c.Background, Appearance: c.Appearance, Backstory: c.Backstory, Now: now,
			}); err != nil {
				return err
			}
		}
		var err error
		id, err = q.InsertCharacter(ctx, queries.InsertCharacterParams{
			CampaignID: uuid.UUID(c.CampaignID), OwnerMemberID: uuid.UUID(c.Owner.ID), CharacterID: pgtype.UUID{Bytes: owned, Valid: true}, Name: c.Name, Ruleset: c.Ruleset,
			SpeciesSlug: c.Species, ClassSlug: c.Class, BackgroundSlug: c.Background, AbilityMethod: c.Method,
			HpMax: int32(c.HPMax), ArmorSlug: optSlug(c.Armor), Shield: c.Shield, Now: now, //nolint:gosec // hit points are small
		})
		if err != nil {
			return conflict(err)
		}
		if err := q.AdoptCharacterIdentity(ctx, id); err != nil {
			return err
		}
		return addBuild(ctx, q, id, c)
	})
	return domain.CharacterID(id), err
}

func addBuild(ctx context.Context, q *queries.Queries, id uuid.UUID, c domain.Character) error {
	for ability, base := range c.Base {
		if err := q.SetCharacterAbility(ctx, queries.SetCharacterAbilityParams{
			CharacterID: id, Ability: ability, Base: int32(base), Bonus: int32(c.Bonus[ability]), //nolint:gosec // 3..20
		}); err != nil {
			return err
		}
	}
	for source, skills := range map[string][]string{"class": c.Skills, "background": c.BackgroundSkills} {
		for _, skill := range skills {
			if err := q.AddCharacterSkill(ctx, queries.AddCharacterSkillParams{CharacterID: id, Skill: skill, Source: source}); err != nil {
				return err
			}
		}
	}
	return addWeapons(ctx, q, id, c.Weapons)
}

func addWeapons(ctx context.Context, q *queries.Queries, id uuid.UUID, weapons []string) error {
	for i, w := range weapons {
		if err := q.AddCharacterWeapon(ctx, queries.AddCharacterWeaponParams{CharacterID: id, WeaponSlug: w, Ordering: int32(i)}); err != nil { //nolint:gosec // at most four
			return err
		}
	}
	return nil
}

// Character reads one Character with its build.
func (s *Store) Character(ctx context.Context, id domain.CampaignID, ch domain.CharacterID) (domain.Character, error) {
	r, err := s.q.GetCharacter(ctx, queries.GetCharacterParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(ch)})
	if err != nil {
		return domain.Character{}, notFound(err)
	}
	c := domain.Character{
		Build: domain.Build{
			Name: r.Name, Species: r.SpeciesSlug, Class: r.ClassSlug, Background: r.BackgroundSlug, Method: r.AbilityMethod,
			Base: map[string]int{}, Bonus: map[string]int{}, Skills: []string{}, Armor: r.ArmorSlug.String, Shield: r.Shield,
		},
		ID: domain.CharacterID(r.ID), Owned: domain.OwnedID(r.CharacterID.Bytes), CampaignID: domain.CampaignID(r.CampaignID),
		Owner:   domain.Member{ID: domain.MemberID(r.OwnerMemberID), CampaignID: domain.CampaignID(r.CampaignID), Subject: r.OwnerSubject, DisplayName: r.OwnerName},
		Ruleset: r.Ruleset, Level: int(r.Level), BackgroundSkills: []string{}, HPMax: int(r.HpMax), HPCurrent: int(r.HpCurrent), TempHP: int(r.TempHp), UpdatedAt: r.UpdatedAt,
		Portrait: image(r.PortraitKey, r.PortraitType), Token: image(r.TokenKey, r.TokenType),
		Increase: map[string]int{}, LevelUpReady: r.LevelUpReady,
	}
	scores, err := s.q.CharacterAbilities(ctx, r.ID)
	if err != nil {
		return domain.Character{}, err
	}
	for _, a := range scores {
		c.Base[a.Ability] = int(a.Base)
		if a.Bonus > 0 {
			c.Bonus[a.Ability] = int(a.Bonus)
		}
		if a.Increase > 0 {
			c.Increase[a.Ability] = int(a.Increase)
		}
	}
	skills, err := s.q.CharacterSkills(ctx, r.ID)
	if err != nil {
		return domain.Character{}, err
	}
	for _, sk := range skills {
		if sk.Source == "class" {
			c.Skills = append(c.Skills, sk.Skill)
		} else {
			c.BackgroundSkills = append(c.BackgroundSkills, sk.Skill)
		}
	}
	if c.Weapons, err = s.q.CharacterWeapons(ctx, r.ID); err != nil {
		return domain.Character{}, err
	}
	return s.progress(ctx, c)
}

// progress reads what a Character chose as it levelled: its classes, picks and spells. One that never
// levelled has every level in its starting class.
func (s *Store) progress(ctx context.Context, c domain.Character) (domain.Character, error) {
	id := uuid.UUID(c.ID)
	classes, err := s.q.CharacterClasses(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}
	for _, x := range classes {
		c.Classes = append(c.Classes, domain.ClassLevel{Class: x.ClassSlug, Subclass: x.SubclassSlug.String, Level: int(x.Level)})
	}
	if len(c.Classes) == 0 {
		c.Classes = []domain.ClassLevel{{Class: c.Class, Subclass: "", Level: max(c.Level, 1)}}
	}
	picks, err := s.q.CharacterPicks(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}
	for _, p := range picks {
		c.Picks = append(c.Picks, domain.Pick{Level: int(p.Level), Choice: p.Choice, Value: p.Value})
	}
	spells, err := s.q.CharacterSpells(ctx, id)
	if err != nil {
		return domain.Character{}, err
	}
	for _, sp := range spells {
		c.Spells = append(c.Spells, domain.LearnedSpell{Class: sp.ClassSlug, Spell: sp.SpellSlug, Level: int(sp.LearnedLevel)})
	}
	return c, nil
}

// LevelUp takes a Character's next level once, while it is unlocked, with everything chosen for it.
//
//nolint:gosec // levels, hit points and increases are bounded by the rules
func (s *Store) LevelUp(ctx context.Context, l domain.LevelUp, now time.Time) error {
	id := uuid.UUID(l.ID)
	n, err := s.q.LevelUpCharacter(ctx, queries.LevelUpCharacterParams{CampaignID: uuid.UUID(l.CampaignID), ID: id, Level: int32(l.From), Gain: int32(l.Gain), Now: now})
	if err != nil {
		return err
	}
	if n == 0 {
		return domain.ErrConflict
	}
	if err := s.q.ClearCharacterClasses(ctx, id); err != nil {
		return err
	}
	for i, x := range l.Classes {
		p := queries.InsertCharacterClassParams{CharacterID: id, ClassSlug: x.Class, SubclassSlug: optSlug(x.Subclass), Level: int32(x.Level), Position: int32(i)}
		if err := s.q.InsertCharacterClass(ctx, p); err != nil {
			return err
		}
	}
	for _, p := range l.Picks {
		if err := s.q.InsertCharacterPick(ctx, queries.InsertCharacterPickParams{CharacterID: id, Level: int32(p.Level), Choice: p.Choice, Value: p.Value}); err != nil {
			return err
		}
	}
	for _, sp := range l.Spells {
		if err := s.q.InsertCharacterSpell(ctx, queries.InsertCharacterSpellParams{CharacterID: id, ClassSlug: sp.Class, SpellSlug: sp.Spell, LearnedLevel: int32(sp.Level)}); err != nil {
			return err
		}
	}
	for ability, inc := range l.Increase {
		if err := s.q.SetAbilityIncrease(ctx, queries.SetAbilityIncreaseParams{CharacterID: id, Ability: ability, Increase: int32(inc)}); err != nil {
			return err
		}
	}
	return nil
}

// SetLevelUpReady unlocks or locks a Character's next level; a level 20 Character has none.
func (s *Store) SetLevelUpReady(ctx context.Context, id domain.CampaignID, ch domain.CharacterID, ready bool, now time.Time) error {
	return s.q.SetLevelUpReady(ctx, queries.SetLevelUpReadyParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(ch), Ready: ready, Now: now})
}

// Characters lists a Campaign's Characters.
func (s *Store) Characters(ctx context.Context, id domain.CampaignID) ([]domain.Character, error) {
	rows, err := s.q.ListCharacters(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Character, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Character{
			Build: domain.Build{Name: r.Name, Species: r.SpeciesSlug, Class: r.ClassSlug},
			ID:    domain.CharacterID(r.ID), Owned: domain.OwnedID(r.CharacterID.Bytes), CampaignID: id,
			Owner:   domain.Member{ID: domain.MemberID(r.OwnerMemberID), Subject: r.OwnerSubject, DisplayName: r.OwnerName},
			Ruleset: r.Ruleset, Level: int(r.Level), HPMax: int(r.HpMax), HPCurrent: int(r.HpCurrent), Token: image(r.TokenKey, pgtype.Text{}),
		})
	}
	return out, nil
}

// UpdateCharacter stores an edited Character.
func (s *Store) UpdateCharacter(ctx context.Context, c domain.Character, now time.Time) error {
	return s.InTx(ctx, func(r app.Repository) error {
		q := r.(*Store).q
		id := uuid.UUID(c.ID)
		if err := q.UpdateCharacter(ctx, queries.UpdateCharacterParams{
			CampaignID: uuid.UUID(c.CampaignID), ID: id, Name: c.Name, HpCurrent: int32(c.HPCurrent), TempHp: int32(c.TempHP), //nolint:gosec // hit points are small
			ArmorSlug: optSlug(c.Armor), Shield: c.Shield, Now: now,
		}); err != nil {
			return err
		}
		if err := q.ClearCharacterWeapons(ctx, id); err != nil {
			return err
		}
		if err := addWeapons(ctx, q, id, c.Weapons); err != nil {
			return err
		}
		return q.FlowCharacterIdentity(ctx, queries.FlowCharacterIdentityParams{ID: id, Now: now})
	})
}

// DeleteCharacter removes a Character.
func (s *Store) DeleteCharacter(ctx context.Context, id domain.CampaignID, ch domain.CharacterID) error {
	return s.q.DeleteCharacter(ctx, queries.DeleteCharacterParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(ch)})
}

func image(key, contentType pgtype.Text) *domain.Image {
	if !key.Valid {
		return nil
	}
	return &domain.Image{Key: key.String, Type: contentType.String}
}

// SetCharacterImage stores or clears a portrait or token icon reference.
func (s *Store) SetCharacterImage(ctx context.Context, id domain.CampaignID, ch domain.CharacterID, kind domain.ImageKind, img *domain.Image, now time.Time) error {
	key, contentType := pgtype.Text{}, pgtype.Text{}
	if img != nil {
		key, contentType = optSlug(img.Key), optSlug(img.Type)
	}
	return s.InTx(ctx, func(r app.Repository) error {
		q := r.(*Store).q
		var err error
		if kind == domain.Portrait {
			err = q.SetCharacterPortrait(ctx, queries.SetCharacterPortraitParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(ch), Key: key, ContentType: contentType, Now: now})
		} else {
			err = q.SetCharacterToken(ctx, queries.SetCharacterTokenParams{CampaignID: uuid.UUID(id), ID: uuid.UUID(ch), Key: key, ContentType: contentType, Now: now})
		}
		if err != nil {
			return err
		}
		return q.FlowCharacterIdentity(ctx, queries.FlowCharacterIdentityParams{ID: uuid.UUID(ch), Now: now})
	})
}
