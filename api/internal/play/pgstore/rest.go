package pgstore

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	comppg "github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/features"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/surface"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/variants"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Surfaces reads the Surface catalogue from the compendium.
func (s *Store) Surfaces(ctx context.Context) (surface.Catalog, error) {
	return comppg.New(s.pool).Surfaces(ctx)
}

// Features reads what classes, species and feats grant from the compendium.
func (s *Store) Features(ctx context.Context) (features.Catalog, error) {
	return comppg.New(s.pool).Features(ctx)
}

// RuleVariants reads what the Campaign has each Rule Variant at.
func (s *Store) RuleVariants(ctx context.Context, campaign uuid.UUID) (variants.Set, error) {
	rows, err := s.q.ListRuleVariants(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make(variants.Set, len(rows))
	for _, r := range rows {
		out[r.Variant] = r.Value
	}
	return out, nil
}

// ShortRests reads how many Short Rests the party has taken since its last Long Rest.
func (s *Store) ShortRests(ctx context.Context, campaign uuid.UUID) (int, error) {
	n, err := s.q.CampaignShortRests(ctx, campaign)
	return int(n), err
}

// RestSupplies reports whether a Long Rest costs each resting Character a day of Rations.
func (s *Store) RestSupplies(ctx context.Context, campaign uuid.UUID) (bool, error) {
	return s.q.CampaignRestSupplies(ctx, campaign)
}

// RestInfo reads each Character's class, level, Hit Die, Hit Dice left, ability scores and spent Resources.
func (s *Store) RestInfo(ctx context.Context, campaign uuid.UUID, ids []uuid.UUID) ([]domain.Rester, error) {
	rows, err := s.q.RestCharacters(ctx, queries.RestCharactersParams{CampaignID: campaign, Ids: ids})
	if err != nil {
		return nil, err
	}
	abilities, err := s.q.RestAbilities(ctx, ids)
	if err != nil {
		return nil, err
	}
	used, err := s.q.RestResourcesUsed(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Rester, 0, len(rows))
	for _, r := range rows {
		x := domain.Rester{
			CharacterID: r.ID, Name: r.Name, Class: r.ClassSlug, Level: int(r.Level), HitDie: int(r.HitDie),
			HitDiceLeft: max(0, int(r.Level-r.HitDiceSpent)), Abilities: map[string]int{}, Used: map[string]int{},
		}
		for _, a := range abilities {
			if a.CharacterID == r.ID {
				x.Abilities[a.Ability] = int(a.Score)
			}
		}
		for _, u := range used {
			if u.CharacterID == r.ID {
				x.Used[u.ResourceSlug] = int(u.Used)
			}
		}
		out = append(out, x)
	}
	return out, nil
}

// LoadRest reads the rest a Session has under way, its agreements and its resters as they stand now.
func (s *Store) LoadRest(ctx context.Context, campaign uuid.UUID, id domain.SessionID) (*domain.Rest, error) {
	sid := uuid.UUID(id)
	row, err := s.q.GetRest(ctx, sid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no rest under way is not an error
	}
	if err != nil {
		return nil, err
	}
	rest := &domain.Rest{Kind: row.Kind, Status: row.Status, ProposedBy: row.ProposedBy, DMAgreed: row.DmAgreed}
	if rest.Agreed, err = s.q.ListRestAgreements(ctx, sid); err != nil {
		return nil, err
	}
	resters, err := s.q.ListRestResters(ctx, sid)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(resters))
	for _, r := range resters {
		ids = append(ids, r.CharacterID)
	}
	if rest.Resters, err = s.RestInfo(ctx, campaign, ids); err != nil {
		return nil, err
	}
	for i, x := range rest.Resters {
		for _, r := range resters {
			if r.CharacterID != x.CharacterID {
				continue
			}
			rest.Resters[i].TokenID = domain.TokenID(r.TokenID)
			if r.RollID.Valid {
				roll := domain.RollID(r.RollID.Bytes)
				rest.Resters[i].RollID = &roll
			}
		}
	}
	return rest, nil
}

// saveRest writes the rest a write leaves, a spent Hit Die and its roll, then what the rest changed.
func (s *Store) saveRest(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) error {
	sid := uuid.UUID(sess.ID)
	if w.Kind == domain.ActionHitDieSpent {
		if err := s.openRolls(ctx, sess, w.Rolls, actor, c, now); err != nil {
			return err
		}
	}
	if w.Resting != nil || w.RestOver {
		if err := s.writeRest(ctx, sid, w.Resting); err != nil {
			return err
		}
	}
	if w.Kind == domain.ActionHitDieSpent {
		if err := s.q.SpendHitDie(ctx, restingCharacter(w)); err != nil {
			return err
		}
	}
	return s.saveRestOutcome(ctx, sid, w, now)
}

// saveRestOutcome writes the Rations a Long Rest ate, the hit points it gave back, and what the rest
// leaves each Character with, telling a player whose Character may now level up.
//
//nolint:gosec // counts are bounded by the rules
func (s *Store) saveRestOutcome(ctx context.Context, sid uuid.UUID, w live.Write, now time.Time) error {
	for _, sup := range w.Supplies {
		if err := s.setCount(ctx, sup.Container, sup.Item, "", sup.Left); err != nil {
			return err
		}
	}
	for _, h := range w.Healed {
		if err := s.q.SetTokenHP(ctx, queries.SetTokenHPParams{SessionID: sid, ID: uuid.UUID(h.Token), Hp: pgInt(h.After)}); err != nil {
			return err
		}
	}
	for _, res := range w.Results {
		if err := s.saveRestResult(ctx, res, now); err != nil {
			return err
		}
	}
	for _, rc := range w.Recharged {
		if err := s.SetCharges(ctx, rc.Instance, rc.Charges); err != nil {
			return err
		}
	}
	return nil
}

// saveRestResult stores what a rest leaves one Character with.
//
//nolint:gosec // counts are bounded by the rules
func (s *Store) saveRestResult(ctx context.Context, res domain.RestResult, now time.Time) error {
	p := queries.SaveRestResultParams{ID: res.CharacterID, HpCurrent: int32(res.HPCurrent), HitDiceSpent: int32(res.HitDiceSpent), LevelUpReady: res.LevelUpReady}
	if err := s.q.SaveRestResult(ctx, p); err != nil {
		return err
	}
	if res.LevelUpReady {
		if err := s.q.NotifyLevelUp(ctx, queries.NotifyLevelUpParams{CharacterID: res.CharacterID, Now: now}); err != nil {
			return err
		}
	}
	for slug, used := range res.Used {
		if err := s.q.SetResourceUsed(ctx, queries.SetResourceUsedParams{CharacterID: res.CharacterID, ResourceSlug: slug, Used: int32(used)}); err != nil {
			return err
		}
	}
	return nil
}

// restingCharacter is the Character whose token spent a Hit Die.
func restingCharacter(w live.Write) uuid.UUID {
	for _, x := range w.Resting.Resters {
		if x.TokenID == w.Token.ID {
			return x.CharacterID
		}
	}
	return uuid.Nil
}

func (s *Store) writeRest(ctx context.Context, sid uuid.UUID, rest *domain.Rest) error {
	if rest == nil {
		return s.q.ClearRest(ctx, sid)
	}
	p := queries.SaveRestParams{SessionID: sid, Kind: rest.Kind, Status: rest.Status, ProposedBy: rest.ProposedBy, DmAgreed: rest.DMAgreed}
	if err := s.q.SaveRest(ctx, p); err != nil {
		return err
	}
	if err := s.q.ClearRestMembers(ctx, sid); err != nil {
		return err
	}
	for _, m := range rest.Agreed {
		if err := s.q.AddRestAgreement(ctx, queries.AddRestAgreementParams{SessionID: sid, MemberID: m}); err != nil {
			return err
		}
	}
	for _, x := range rest.Resters {
		p := queries.AddRestResterParams{SessionID: sid, CharacterID: x.CharacterID, TokenID: uuid.UUID(x.TokenID)}
		if x.RollID != nil {
			p.RollID = pgtype.UUID{Bytes: *x.RollID, Valid: true}
		}
		if err := s.q.AddRestRester(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
