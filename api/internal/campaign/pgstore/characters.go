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
		Ruleset: r.Ruleset, Level: int(r.Level), BackgroundSkills: []string{}, HPMax: int(r.HpMax), HPCurrent: int(r.HpCurrent), UpdatedAt: r.UpdatedAt,
		Portrait: image(r.PortraitKey, r.PortraitType), Token: image(r.TokenKey, r.TokenType),
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
	return c, nil
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
			CampaignID: uuid.UUID(c.CampaignID), ID: id, Name: c.Name, HpCurrent: int32(c.HPCurrent), //nolint:gosec // hit points are small
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
