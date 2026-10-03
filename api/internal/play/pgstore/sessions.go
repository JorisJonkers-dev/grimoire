package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/actions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/attack"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/combat"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/hex"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/reactions"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

var (
	_ app.SessionRepository = (*Store)(nil)
	_ live.Store            = (*Store)(nil)
)

func session(s queries.PlaySession) domain.Session {
	out := domain.Session{
		ID: domain.SessionID(s.ID), CampaignID: s.CampaignID, Number: int(s.Number), Status: s.Status, Seq: s.Seq,
		GridRadius: int(s.GridRadius), StartedAt: s.StartedAt, EndedAt: s.EndedAt.Time,
	}
	if s.MapID.Valid {
		id := domain.MapID(s.MapID.Bytes)
		out.MapID = &id
	}
	if s.WorldMapID.Valid {
		id := domain.MapID(s.WorldMapID.Bytes)
		out.WorldMapID = &id
	}
	return out
}

// sessionAction appends a Session Action with the next campaign sequence; call it inside a transaction.
func (s *Store) sessionAction(ctx context.Context, campaign uuid.UUID, sid domain.SessionID, kind string, actor domain.Member, c caller.Caller, now time.Time) (uuid.UUID, error) {
	id, _, err := s.loggedAction(ctx, campaign, sid, kind, actor, c, now)
	return id, err
}

// loggedAction appends an Action to the Campaign's log and returns its id and sequence.
func (s *Store) loggedAction(ctx context.Context, campaign uuid.UUID, sid domain.SessionID, kind string, actor domain.Member, c caller.Caller, now time.Time) (uuid.UUID, int64, error) {
	if err := s.q.LockCampaignLog(ctx, "log:"+campaign.String()); err != nil {
		return uuid.UUID{}, 0, err
	}
	seq, err := s.q.NextActionSeq(ctx, campaign)
	if err != nil {
		return uuid.UUID{}, 0, err
	}
	id, err := s.q.InsertSessionAction(ctx, queries.InsertSessionActionParams{
		CampaignID: campaign, SessionID: pgtype.UUID{Bytes: sid, Valid: true}, Seq: int64(seq), Kind: kind,
		ActorMemberID: actor.ID, ActorName: actor.Name, Origin: string(c.Origin), Client: c.Client, Now: now,
	})
	return id, int64(seq), err
}

// CreateSession opens the Campaign's next Session.
func (s *Store) CreateSession(ctx context.Context, campaign uuid.UUID, actor domain.Member, c caller.Caller, now time.Time) (domain.Session, error) {
	var out domain.Session
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		if err := tx.q.LockCampaignLog(ctx, "sessions:"+campaign.String()); err != nil {
			return err
		}
		n, err := tx.q.NextSessionNumber(ctx, campaign)
		if err != nil {
			return err
		}
		row, err := tx.q.InsertSession(ctx, queries.InsertSessionParams{CampaignID: campaign, Number: n, Now: now})
		if err != nil {
			return err
		}
		out = session(row)
		_, err = tx.sessionAction(ctx, campaign, out.ID, domain.ActionSessionStarted, actor, c, now)
		return err
	})
	return out, err
}

// Session reads one Session of a Campaign.
func (s *Store) Session(ctx context.Context, campaign uuid.UUID, id domain.SessionID) (domain.Session, error) {
	row, err := s.q.GetSession(ctx, queries.GetSessionParams{CampaignID: campaign, ID: uuid.UUID(id)})
	if err != nil {
		return domain.Session{}, notFound(err)
	}
	return session(row), nil
}

// Sessions lists a Campaign's Sessions, newest first.
func (s *Store) Sessions(ctx context.Context, campaign uuid.UUID) ([]domain.Session, error) {
	rows, err := s.q.ListSessions(ctx, campaign)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Session, 0, len(rows))
	for _, r := range rows {
		out = append(out, session(r))
	}
	return out, nil
}

// EndSession ends a live Session.
func (s *Store) EndSession(ctx context.Context, campaign uuid.UUID, id domain.SessionID, actor domain.Member, c caller.Caller, now time.Time) error {
	return s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		n, err := tx.q.EndSession(ctx, queries.EndSessionParams{CampaignID: campaign, ID: uuid.UUID(id), Now: pgtypeTime(now)})
		if err != nil {
			return err
		}
		if n == 0 {
			return apperr.ErrNotFound
		}
		_, err = tx.sessionAction(ctx, campaign, id, domain.ActionSessionEnded, actor, c, now)
		return err
	})
}

// Load reads a Session, its Tokens and its active Map for the live runtime.
func (s *Store) Load(ctx context.Context, id domain.SessionID) (domain.Session, []domain.Token, *domain.MapState, error) {
	row, err := s.q.SessionByID(ctx, uuid.UUID(id))
	if err != nil {
		return domain.Session{}, nil, nil, notFound(err)
	}
	rows, err := s.q.SessionTokens(ctx, row.ID)
	if err != nil {
		return domain.Session{}, nil, nil, err
	}
	tokens := make([]domain.Token, 0, len(rows))
	for _, t := range rows {
		tokens = append(tokens, tokenFrom(t))
	}
	saves, err := s.q.SessionTokenSaves(ctx, row.ID)
	if err != nil {
		return domain.Session{}, nil, nil, err
	}
	for _, sv := range saves {
		i := slices.IndexFunc(tokens, func(t domain.Token) bool { return uuid.UUID(t.ID) == sv.TokenID })
		if tokens[i].Stats.Saves == nil {
			tokens[i].Stats.Saves = map[string]int{}
		}
		tokens[i].Stats.Saves[sv.Ability] = int(sv.Bonus)
	}
	attacks, err := s.q.SessionTokenAttacks(ctx, row.ID)
	if err != nil {
		return domain.Session{}, nil, nil, err
	}
	for _, a := range attacks {
		i := slices.IndexFunc(tokens, func(t domain.Token) bool { return uuid.UUID(t.ID) == a.TokenID })
		tokens[i].Stats.Attacks = append(tokens[i].Stats.Attacks, domain.Attack{
			Name: a.Name, ToHit: int(a.ToHit), ReachFt: int(a.ReachFt), RangeFt: int(a.RangeFt), LongRangeFt: int(a.LongRangeFt),
			Damage: a.DamageDice, DamageBonus: int(a.DamageBonus), DamageType: a.DamageType, Light: a.Light, DamageMod: int(a.DamageMod),
			Mastery: a.Mastery.String,
		})
	}
	if err := s.loadReactionSettings(ctx, row.ID, tokens); err != nil {
		return domain.Session{}, nil, nil, err
	}
	if err := s.loadVisibility(ctx, row.ID, tokens); err != nil {
		return domain.Session{}, nil, nil, err
	}
	if err := s.loadForms(ctx, row.ID, tokens); err != nil {
		return domain.Session{}, nil, nil, err
	}
	sess := session(row)
	if sess.MapID == nil {
		return sess, tokens, nil, nil
	}
	board, err := s.LoadMap(ctx, sess.CampaignID, *sess.MapID)
	return sess, tokens, board, err
}

// Commit writes one change, the hexes it reveals, the next Session sequence and its Action in one transaction.
func (s *Store) Commit(ctx context.Context, sess domain.Session, board *domain.MapState, w live.Write, actor domain.Member, c caller.Caller, now time.Time) (live.Committed, error) {
	var done live.Committed
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		sid := uuid.UUID(sess.ID)
		var err error
		if done.Seq, err = tx.q.BumpSessionSeq(ctx, sid); err != nil {
			return err
		}
		steps := []func() error{
			func() error { return tx.saveLegends(ctx, sid, w.Legends) }, // a mythic phase raises the maximum before the hit points
			func() error { return tx.write(ctx, sid, board, w, now) },
			func() error { return tx.dismiss(ctx, sid, w.Dismissed) },
			func() error { return tx.saveForms(ctx, sid, w) },
			func() error { return tx.saveObjects(ctx, board, w) },
			func() error { return tx.saveCombat(ctx, sess, w, actor, c, now) },
			func() error { return tx.saveEffects(ctx, sid, w.Effects) },
			func() error { return tx.saveTerrain(ctx, sid, board, w) },
			func() error { return tx.saveTable(ctx, sid, w.Table) },
			func() error { return tx.saveZone(ctx, sess, w, actor, c, now) },
			func() error { return tx.saveCheck(ctx, sess, w, actor, c, now) },
			func() error { return tx.saveInventory(ctx, sess, w, now) },
			func() error { return tx.saveSwap(ctx, sid, w) },
			func() error { return tx.saveShop(ctx, sess, w, actor, c, now) },
			func() error { return tx.saveRest(ctx, sess, w, actor, c, now) },
			func() error { return tx.saveActions(ctx, sess, w, actor, c, now) },
			func() error {
				if board == nil {
					return nil
				}
				return tx.addReveals(ctx, board.Map.ID, w.AutoReveal)
			},
		}
		for _, step := range steps {
			if err := step(); err != nil {
				return err
			}
		}
		done.Action, err = tx.record(ctx, sess, w, actor, c, now)
		return err
	})
	return done, err
}

// record appends the change's Action with what it touched, and returns its sequence.
func (s *Store) record(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) (int64, error) {
	actionID, seq, err := s.loggedAction(ctx, sess.CampaignID, sess.ID, w.Kind, actor, c, now)
	if err != nil {
		return 0, err
	}
	return seq, s.logged(ctx, actionID, sess, w)
}

// logged records what an Action touched.
func (s *Store) logged(ctx context.Context, actionID uuid.UUID, sess domain.Session, w live.Write) error {
	if err := s.logTools(ctx, actionID, w); err != nil {
		return err
	}
	if w.Leg != nil {
		if err := s.logLeg(ctx, actionID, uuid.UUID(sess.ID), w); err != nil {
			return err
		}
	}
	if err := s.logItems(ctx, actionID, w); err != nil {
		return err
	}
	return s.logWrite(ctx, actionID, w)
}

//nolint:gosec // coordinates and ranges are bounded by the map
func (s *Store) write(ctx context.Context, sid uuid.UUID, board *domain.MapState, w live.Write, now time.Time) error {
	t := w.Token
	switch w.Kind {
	case domain.ActionTokenPlaced:
		return s.insertToken(ctx, sid, t)
	case domain.ActionTokenRemoved:
		return s.q.DeleteToken(ctx, queries.DeleteTokenParams{SessionID: sid, ID: uuid.UUID(t.ID)})
	case domain.ActionTokenMoved, domain.ActionTokenWalked, domain.ActionTokenHidden, domain.ActionTokenRevealed, domain.ActionTeleported, domain.ActionJumped:
		return s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden})
	case domain.ActionEffectApplied:
		return s.writeLanding(ctx, sid, w)
	case domain.ActionCombatStarted, domain.ActionInitiativeRolled, domain.ActionTurnEnded, domain.ActionResourceSpent, domain.ActionCombatEnded,
		domain.ActionAttackDeclared, domain.ActionAttackHit, domain.ActionAttackMissed, domain.ActionReactionOffered, domain.ActionReactionUsed,
		domain.ActionReactionDeclined, domain.ActionEffectEnded, domain.ActionSavePassed, domain.ActionSaveFailed,
		domain.ActionManualResolved, domain.ActionAreaCast, domain.ActionAreaResolved, domain.ActionSurfacesSet, domain.ActionElevationSet, domain.ActionTableSet,
		domain.ActionZoneAdded, domain.ActionZoneRemoved, domain.ActionZoneHeld, domain.ActionZoneSprung, domain.ActionPerceptionRolled,
		domain.ActionRestTaken, domain.ActionCheckScheduled, domain.ActionEncounterChecked, domain.ActionEncounterResolved,
		domain.ActionLootDropped, domain.ActionItemMoved, domain.ActionCoinsMoved, domain.ActionLootClaimed, domain.ActionLootSettled, domain.ActionTradeMade, domain.ActionShopOpened, domain.ActionShopClosed,
		domain.ActionItemBought, domain.ActionItemSold, domain.ActionHaggleStarted, domain.ActionHaggled, domain.ActionStockRolled,
		domain.ActionRestProposed, domain.ActionRestAgreed, domain.ActionRestStarted, domain.ActionHitDieSpent, domain.ActionRestInterrupted,
		domain.ActionTaken, domain.ActionUnarmed, domain.ActionResolved, domain.ActionObjectUsed, domain.ActionMasteryUsed,
		domain.ActionConcentrationChecked, domain.ActionDowned, domain.ActionCountered, domain.ActionCommanded,
		domain.ActionObjectPlaced, domain.ActionObjectRemoved, domain.ActionObjectToggled, domain.ActionObjectDamaged, domain.ActionObjectFound,
		domain.ActionObjectUnlocked, domain.ActionTrapDisarmed, domain.ActionTrapSprung, domain.ActionThrown,
		domain.ActionSneakStarted, domain.ActionSneakEnded, domain.ActionStealthRolled, domain.ActionPartyNoticed,
		domain.ActionExplorationStarted, domain.ActionExplorationTurn, domain.ActionExplorationEnded,
		domain.ActionLegendaryAction, domain.ActionLairAction, domain.ActionLegendaryResistance:
		return s.writeThrown(ctx, sid, w)
	case domain.ActionDyingChanged, domain.ActionRevived:
		if w.HP == nil {
			return nil
		}
		return s.setHP(ctx, sid, *w.HP)
	case domain.ActionReactionSet:
		return s.saveReactionSettings(ctx, w.Token)
	case domain.ActionVisibilitySet:
		return s.saveVisibility(ctx, sid, w.Token)
	case domain.ActionEncounterSpawned, domain.ActionSummoned:
		for _, t := range w.Spawned {
			if err := s.insertToken(ctx, sid, t); err != nil {
				return err
			}
		}
		return nil
	case domain.ActionDamageDealt, domain.ActionDamageUndone, domain.ActionHPAdjusted, domain.ActionHitDieHealed:
		return s.writeHP(ctx, sid, w)
	case domain.ActionWorldSet, domain.ActionNodeAdded, domain.ActionNodeRemoved, domain.ActionRouteAdded, domain.ActionRouteRemoved,
		domain.ActionPartyPlaced, domain.ActionTravelLeg:
		return s.writeWorld(ctx, sid, w)
	case domain.ActionTacticsSet:
		return s.q.SetTokenTactics(ctx, queries.SetTokenTacticsParams{SessionID: sid, ID: uuid.UUID(w.Token.ID), Tactics: w.Token.Tactics})
	case domain.ActionMapSet:
		p := queries.SetSessionMapParams{ID: sid}
		if w.MapID != nil {
			p.MapID = pgtype.UUID{Bytes: *w.MapID, Valid: true}
		}
		return s.q.SetSessionMap(ctx, p)
	default:
		return s.writeBoard(ctx, board, w, now)
	}
}

//nolint:gosec // coordinates and stats are bounded by the rules
func (s *Store) insertToken(ctx context.Context, sid uuid.UUID, t domain.Token) error {
	p := queries.InsertTokenParams{
		ID: uuid.UUID(t.ID), SessionID: sid, Label: t.Label, Kind: t.Kind, Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden, DarkvisionFt: int32(t.DarkvisionFt),
		CanShield: t.CanShield,
	}
	if t.Controller != nil {
		p.ControllerMemberID = pgtype.UUID{Bytes: *t.Controller, Valid: true}
	}
	if t.Summon != nil {
		p.SummonEffectID = pgtype.UUID{Bytes: *t.Summon, Valid: true}
	}
	p.Strength = 10
	if t.Stats == nil {
		p.SpeedFt, p.UnarmedDc, p.AttacksPerAction = 30, 10, 1
		if err := s.q.InsertToken(ctx, p); err != nil {
			return err
		}
		return s.saveVisibility(ctx, sid, t)
	}
	st := t.Stats
	p.StatSource = pgtype.Text{String: st.Source, Valid: true}
	p.ArmorClass, p.Hp, p.HpMax = pgInt(st.AC), pgInt(st.HP), pgInt(st.HPMax)
	p.Stealth, p.Perception, p.Initiative, p.SpeedFt = int32(st.Stealth), int32(st.Perception), int32(st.Initiative), int32(st.SpeedFt)
	p.UnarmedDc = int32(max(1, min(40, st.UnarmedDC)))
	if st.UnarmedDC == 0 {
		p.UnarmedDc = 10
	}
	p.AttacksPerAction = int32(max(1, min(4, st.AttacksPerAction)))
	if st.Legend != nil {
		p.Legend, _ = json.Marshal(st.Legend) //nolint:errchkjson // a Legend is plain data
	}
	p.CreatureType = st.CreatureType
	if st.Strength > 0 {
		p.Strength = int32(min(30, st.Strength))
	}
	if st.SpellDC > 0 {
		p.SpellDc = pgInt(st.SpellDC)
	}
	if st.Intelligence > 0 {
		p.Intelligence = pgInt(st.Intelligence)
	}
	if err := s.q.InsertToken(ctx, p); err != nil {
		return err
	}
	if err := s.insertStatRows(ctx, t.ID, st); err != nil {
		return err
	}
	return s.saveVisibility(ctx, sid, t)
}

// insertAttacks writes a token's attacks in order.
//
//nolint:gosec // statblock numbers are bounded by the rules
func (s *Store) insertAttacks(ctx context.Context, id domain.TokenID, attacks []domain.Attack) error {
	for i, a := range attacks {
		if err := s.q.InsertTokenAttack(ctx, queries.InsertTokenAttackParams{
			TokenID: uuid.UUID(id), Ordering: int32(i), Name: a.Name, ToHit: int32(a.ToHit), ReachFt: int32(a.ReachFt), RangeFt: int32(a.RangeFt),
			LongRangeFt: int32(a.LongRangeFt), DamageDice: a.Damage, DamageBonus: int32(a.DamageBonus), DamageType: a.DamageType,
			Light: a.Light, DamageMod: int32(a.DamageMod), Mastery: pgtype.Text{String: a.Mastery, Valid: a.Mastery != ""},
		}); err != nil {
			return err
		}
	}
	return nil
}

// saveSwap gives a token the attacks and Armor Class of the weapon set its Character took up, and keeps
// the set and what it holds on the Character.
func (s *Store) saveSwap(ctx context.Context, sid uuid.UUID, w live.Write) error {
	sw := w.Swap
	if sw == nil {
		return nil
	}
	t := w.Token
	if err := s.q.DeleteTokenAttacks(ctx, uuid.UUID(t.ID)); err != nil {
		return err
	}
	if err := s.insertAttacks(ctx, t.ID, t.Stats.Attacks); err != nil {
		return err
	}
	if err := s.q.SetTokenArmorClass(ctx, queries.SetTokenArmorClassParams{SessionID: sid, ID: uuid.UUID(t.ID), ArmorClass: pgInt(t.Stats.AC)}); err != nil {
		return err
	}
	if err := s.SetWeaponSet(ctx, sw.Character, sw.Set); err != nil {
		return err
	}
	return s.SyncEquipment(ctx, sw.Character, sw.Armor, sw.Shield, sw.Weapons)
}

// insertStatRows writes a token's attacks, saves and Senses.
//
//nolint:gosec // statblock numbers are bounded by the rules
func (s *Store) insertStatRows(ctx context.Context, id domain.TokenID, st *domain.Stats) error {
	if err := s.insertAttacks(ctx, id, st.Attacks); err != nil {
		return err
	}
	for ability, bonus := range st.Saves {
		if err := s.q.InsertTokenSave(ctx, queries.InsertTokenSaveParams{TokenID: uuid.UUID(id), Ability: ability, Bonus: int32(bonus)}); err != nil {
			return err
		}
	}
	for sense, ft := range st.Senses {
		if err := s.q.AddTokenSense(ctx, queries.AddTokenSenseParams{TokenID: uuid.UUID(id), Sense: sense, RangeFt: int32(ft)}); err != nil {
			return err
		}
	}
	return nil
}

// saveVisibility writes a token's Visibility Qualities, which the party has seen through, and its disguise.
func (s *Store) saveVisibility(ctx context.Context, sid uuid.UUID, t domain.Token) error {
	p := queries.SetTokenDisguiseParams{SessionID: sid, ID: uuid.UUID(t.ID), Disguise: pgtype.Text{String: t.Disguise, Valid: t.Disguise != ""}}
	if err := s.q.SetTokenDisguise(ctx, p); err != nil {
		return err
	}
	if err := s.q.ClearTokenQualities(ctx, uuid.UUID(t.ID)); err != nil {
		return err
	}
	for q, through := range t.Qualities {
		if err := s.q.AddTokenQuality(ctx, queries.AddTokenQualityParams{TokenID: uuid.UUID(t.ID), Quality: q, SeenThrough: through}); err != nil {
			return err
		}
	}
	return nil
}

// loadVisibility reads every token's Visibility Qualities and Senses.
func (s *Store) loadVisibility(ctx context.Context, sid uuid.UUID, tokens []domain.Token) error {
	qualities, err := s.q.SessionTokenQualities(ctx, sid)
	if err != nil {
		return err
	}
	for _, q := range qualities {
		i := slices.IndexFunc(tokens, func(t domain.Token) bool { return uuid.UUID(t.ID) == q.TokenID })
		if tokens[i].Qualities == nil {
			tokens[i].Qualities = map[string]bool{}
		}
		tokens[i].Qualities[q.Quality] = q.SeenThrough
	}
	senses, err := s.q.SessionTokenSenses(ctx, sid)
	if err != nil {
		return err
	}
	for _, sn := range senses {
		i := slices.IndexFunc(tokens, func(t domain.Token) bool { return uuid.UUID(t.ID) == sn.TokenID && t.Stats != nil })
		if i < 0 {
			continue
		}
		if tokens[i].Stats.Senses == nil {
			tokens[i].Stats.Senses = map[string]int{}
		}
		tokens[i].Stats.Senses[sn.Sense] = int(sn.RangeFt)
	}
	return nil
}

// writeLanding writes what an Effect changed as it landed: hit points and Resources.
//
//nolint:gosec // deltas are bounded by a check
func (s *Store) writeLanding(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if w.HP != nil {
		if err := s.setHP(ctx, sid, *w.HP); err != nil {
			return err
		}
	}
	if w.Unveiled {
		if err := s.saveVisibility(ctx, sid, w.Token); err != nil {
			return err
		}
	}
	for _, r := range w.Resources {
		if err := s.q.ChangeResourceUsed(ctx, queries.ChangeResourceUsedParams{CharacterID: r.Character, ResourceSlug: r.Resource, Delta: int32(r.Delta)}); err != nil {
			return err
		}
	}
	return nil
}

// loadForms lays each stored form over its token. The token's row keeps its own statistics; its hit
// points are its own and its temporary hit points the form's.
func (s *Store) loadForms(ctx context.Context, sid uuid.UUID, tokens []domain.Token) error {
	forms, err := s.q.SessionTokenForms(ctx, sid)
	if err != nil {
		return err
	}
	for _, f := range forms {
		i := slices.IndexFunc(tokens, func(t domain.Token) bool { return uuid.UUID(t.ID) == f.TokenID })
		if i < 0 || tokens[i].Stats == nil {
			continue
		}
		var shaped domain.Stats
		if err := json.Unmarshal(f.Stats, &shaped); err != nil {
			return err
		}
		own := *tokens[i].Stats
		shaped.HP, shaped.HPMax, shaped.TempHP = own.HP, own.HPMax, own.TempHP
		own.TempHP = 0
		tokens[i].Stats, tokens[i].Form = &shaped, &domain.Form{Effect: domain.EffectID(f.EffectID), Name: f.Name, Own: own}
	}
	return nil
}

// saveForms writes a form a token takes and drops the forms of tokens that reverted.
func (s *Store) saveForms(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if t := w.Formed; t != nil {
		stats, err := json.Marshal(t.Stats)
		if err != nil {
			return err
		}
		p := queries.SaveTokenFormParams{TokenID: uuid.UUID(t.ID), EffectID: uuid.UUID(t.Form.Effect), Name: t.Form.Name, Stats: stats}
		if err := s.q.SaveTokenForm(ctx, p); err != nil {
			return err
		}
	}
	for _, id := range w.Reverted {
		if err := s.q.DeleteTokenForm(ctx, uuid.UUID(id)); err != nil {
			return err
		}
		if err := s.q.SetTokenTempHP(ctx, queries.SetTokenTempHPParams{SessionID: sid, ID: uuid.UUID(id), TempHp: 0}); err != nil {
			return err
		}
	}
	return nil
}

// writeThrown moves a thrown creature; every other Map Object change is written with the objects.
func (s *Store) writeThrown(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if w.Kind != domain.ActionThrown || w.Pushed == nil {
		return nil
	}
	t := w.Pushed
	return s.q.UpdateToken(ctx, queries.UpdateTokenParams{SessionID: sid, ID: uuid.UUID(t.ID), Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden}) //nolint:gosec // map coordinates
}

// dismiss deletes summoned tokens whose Effect ended; their Combatants go with them.
func (s *Store) dismiss(ctx context.Context, sid uuid.UUID, ids []domain.TokenID) error {
	for _, id := range ids {
		if err := s.q.DeleteToken(ctx, queries.DeleteTokenParams{SessionID: sid, ID: uuid.UUID(id)}); err != nil {
			return err
		}
	}
	return nil
}

// setHP writes a token's hit points, and its temporary hit points when the change touches them.
func (s *Store) setHP(ctx context.Context, sid uuid.UUID, h live.HPChange) error {
	if err := s.q.SetTokenHP(ctx, queries.SetTokenHPParams{SessionID: sid, ID: uuid.UUID(h.Token), Hp: pgInt(h.After)}); err != nil || h.Temp == nil {
		return err
	}
	return s.q.SetTokenTempHP(ctx, queries.SetTokenTempHPParams{SessionID: sid, ID: uuid.UUID(h.Token), TempHp: int32(*h.Temp)}) //nolint:gosec // bounded by a check
}

// writeHP sets a token's hit points and records who watched a ranged attacker deal the damage.
//
//nolint:gosec // hit points are bounded by the rules
func (s *Store) writeHP(ctx context.Context, sid uuid.UUID, w live.Write) error {
	if err := s.setHP(ctx, sid, *w.HP); err != nil {
		return err
	}
	for _, o := range w.Observers {
		if err := s.q.ObserveDamage(ctx, queries.ObserveDamageParams{Observer: uuid.UUID(o), Attacker: uuid.UUID(w.Token.ID), Amount: int32(w.HP.Lost())}); err != nil {
			return err
		}
	}
	return nil
}

//nolint:gosec // coordinates and ranges are bounded by the map
func (s *Store) writeBoard(ctx context.Context, board *domain.MapState, w live.Write, now time.Time) error {
	switch w.Kind {
	case domain.ActionHexesRevealed:
		return s.addReveals(ctx, board.Map.ID, w.Hexes)
	case domain.ActionHexesConcealed:
		for _, c := range w.Hexes {
			if err := s.q.RemoveReveal(ctx, queries.RemoveRevealParams{MapID: uuid.UUID(board.Map.ID), Q: int32(c.Q), R: int32(c.R)}); err != nil {
				return err
			}
		}
		return nil
	case domain.ActionWallsSet, domain.ActionWallsCleared:
		for _, c := range w.Hexes {
			p := queries.AddWallParams{MapID: uuid.UUID(board.Map.ID), Q: int32(c.Q), R: int32(c.R)}
			err := s.q.AddWall(ctx, p)
			if w.Kind == domain.ActionWallsCleared {
				err = s.q.RemoveWall(ctx, queries.RemoveWallParams(p))
			}
			if err != nil {
				return err
			}
		}
		return nil
	case domain.ActionLightPlaced:
		l := w.Light
		return s.q.InsertLight(ctx, queries.InsertLightParams{
			ID: uuid.UUID(l.ID), MapID: uuid.UUID(board.Map.ID), Q: int32(l.At.Q), R: int32(l.At.R), BrightFt: int32(l.BrightFt), DimFt: int32(l.DimFt),
		})
	case domain.ActionLightRemoved:
		return s.q.DeleteLight(ctx, queries.DeleteLightParams{MapID: uuid.UUID(board.Map.ID), ID: uuid.UUID(w.Light.ID)})
	default:
		return s.q.SetMapAmbient(ctx, queries.SetMapAmbientParams{ID: uuid.UUID(board.Map.ID), Ambient: w.Ambient, Now: now})
	}
}

func (s *Store) addReveals(ctx context.Context, id domain.MapID, hs []hex.Coord) error {
	for _, c := range hs {
		if err := s.q.AddReveal(ctx, queries.AddRevealParams{MapID: uuid.UUID(id), Q: int32(c.Q), R: int32(c.R)}); err != nil { //nolint:gosec // map coordinates
			return err
		}
	}
	return nil
}

//nolint:gosec // coordinates are bounded by the map
func (s *Store) logWrite(ctx context.Context, actionID uuid.UUID, w live.Write) error {
	switch w.Kind {
	case domain.ActionTokenPlaced, domain.ActionTokenMoved, domain.ActionTokenWalked, domain.ActionTokenHidden, domain.ActionTokenRevealed, domain.ActionTokenRemoved,
		domain.ActionInitiativeRolled, domain.ActionTurnEnded, domain.ActionResourceSpent, domain.ActionAttackDeclared, domain.ActionAttackHit,
		domain.ActionAttackMissed, domain.ActionTacticsSet, domain.ActionReactionOffered, domain.ActionReactionUsed, domain.ActionReactionDeclined,
		domain.ActionEffectApplied, domain.ActionEffectEnded, domain.ActionSavePassed, domain.ActionSaveFailed, domain.ActionAreaCast,
		domain.ActionAreaResolved, domain.ActionTaken, domain.ActionUnarmed, domain.ActionResolved, domain.ActionMasteryUsed, domain.ActionReactionSet, domain.ActionConcentrationChecked,
		domain.ActionDowned, domain.ActionDyingChanged, domain.ActionRevived, domain.ActionTeleported, domain.ActionCountered, domain.ActionVisibilitySet,
		domain.ActionJumped, domain.ActionThrown:
		t := w.Token
		return s.q.InsertTokenEvent(ctx, queries.InsertTokenEventParams{
			ActionID: actionID, TokenID: uuid.UUID(t.ID), Label: t.Label, Q: int32(t.Q), R: int32(t.R), Hidden: t.Hidden,
		})
	}
	if h := w.HP; h != nil {
		p := queries.InsertHPEventParams{ActionID: actionID, TokenID: uuid.UUID(h.Token), HpBefore: int32(h.Before), HpAfter: int32(h.After)}
		if h.Undoes != (uuid.UUID{}) {
			p.UndoesActionID = pgtype.UUID{Bytes: h.Undoes, Valid: true}
		}
		return s.q.InsertHPEvent(ctx, p)
	}
	hs := append(append([]hex.Coord{}, w.Hexes...), w.AutoReveal...)
	if w.Kind == domain.ActionLightPlaced || w.Kind == domain.ActionLightRemoved {
		hs = append(hs, w.Light.At)
	}
	for _, c := range hs {
		if err := s.q.InsertHexEvent(ctx, queries.InsertHexEventParams{ActionID: actionID, Q: int32(c.Q), R: int32(c.R)}); err != nil {
			return err
		}
	}
	return nil
}

// saveCombat opens the Roll Requests a Combat starts with, then writes the Combat and its Combatants.
//
//nolint:gosec // rounds, counts and feet are bounded by the rules
func (s *Store) saveCombat(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) error {
	if w.Combat == nil {
		return nil
	}
	for _, r := range w.Rolls {
		if _, err := s.InsertRoll(ctx, r, now); err != nil {
			return err
		}
		if err := s.Append(ctx, sess.CampaignID, app.LogEntry{Kind: domain.ActionRollRequested, Actor: actor, Caller: c, RollID: r.ID, At: now}); err != nil {
			return err
		}
	}
	f := w.Combat
	p := queries.SaveCombatParams{ID: uuid.UUID(f.ID), SessionID: uuid.UUID(sess.ID), Status: f.Status, Round: int32(f.Round), StartedAt: f.StartedAt}
	if f.Status == domain.CombatActive {
		p.TurnCount = pgtype.Int4{Int32: int32(f.Turn), Valid: true}
	}
	if !f.EndedAt.IsZero() {
		p.EndedAt = pgtypeTime(f.EndedAt)
	}
	if f.Resume != nil {
		p.ResumeTokenID, p.ResumeCostFt = pgtype.UUID{Bytes: f.Resume.Token, Valid: true}, pgInt(f.Resume.CostFt)
	}
	if err := s.q.SaveCombat(ctx, p); err != nil {
		return err
	}
	if err := s.saveReactions(ctx, f); err != nil {
		return err
	}
	if err := s.saveAttack(ctx, f); err != nil {
		return err
	}
	return s.saveCombatants(ctx, f)
}

//nolint:gosec // rounds, counts and feet are bounded by the rules
func (s *Store) saveCombatants(ctx context.Context, f *domain.Combat) error {
	for _, x := range f.Combatants {
		cp := queries.SaveCombatantParams{
			ID: uuid.UUID(x.ID), CombatID: uuid.UUID(f.ID), TokenID: uuid.UUID(x.TokenID), RollID: uuid.UUID(x.RollID),
			InitiativeBonus: int32(x.InitiativeBonus), SpeedFt: int32(x.SpeedFt), Done: x.Done, HasAction: x.Economy.Action,
			HasBonusAction: x.Economy.BonusAction, HasReaction: x.Economy.Reaction, MovementFt: int32(x.Economy.MovementFt), Shielded: x.Shielded,
			AttacksLeft: int32(x.Economy.AttacksLeft), LightAttack: x.Economy.LightAttack, OffHand: x.Economy.OffHand, Interaction: x.Economy.Interaction, Equips: int32(x.Economy.Equips),
			Cleaved:   x.Cleaved,
			Surprised: x.Surprised, Disengaged: x.Disengaged,
		}
		if x.CleaveFrom != nil {
			cp.CleaveFrom = pgtype.UUID{Bytes: *x.CleaveFrom, Valid: true}
		}
		if x.Owner != nil {
			cp.OwnerCombatantID = pgtype.UUID{Bytes: *x.Owner, Valid: true}
		}
		cp.Commanded = x.Commanded
		if r := x.Readied; r != nil {
			cp.ReadiedTrigger, cp.ReadiedAttack = pgtype.Text{String: string(r.Trigger.Kind), Valid: true}, pgInt(r.AttackNo)
			if who, err := uuid.Parse(r.Trigger.Who); err == nil {
				cp.ReadiedWho = pgtype.UUID{Bytes: who, Valid: true}
			}
		}
		if x.Initiative != nil {
			cp.Initiative = pgtype.Int4{Int32: int32(*x.Initiative), Valid: true}
		}
		if err := s.q.SaveCombatant(ctx, cp); err != nil {
			return err
		}
	}
	return nil
}

// saveReactions writes the open Reaction Prompt and the rest of an interrupted walk, or clears them.
//
//nolint:gosec // coordinates and attack numbers are bounded by the rules
func (s *Store) saveReactions(ctx context.Context, f *domain.Combat) error {
	id := uuid.UUID(f.ID)
	if err := s.q.ClearPrompts(ctx, id); err != nil {
		return err
	}
	if p := f.Prompt; p != nil {
		if err := s.q.SavePrompt(ctx, queries.SavePromptParams{
			ID: p.ID, CombatID: id, Kind: p.Kind, ReactorTokenID: uuid.UUID(p.Reactor), TriggerTokenID: uuid.UUID(p.Trigger),
			AttackNo: int32(p.AttackNo), Effect: p.Effect, Deadline: p.Deadline,
		}); err != nil {
			return err
		}
	}
	if err := s.q.ClearResumePath(ctx, id); err != nil {
		return err
	}
	if f.Resume == nil {
		return nil
	}
	for i, c := range f.Resume.Path {
		if err := s.q.AddResumeHex(ctx, queries.AddResumeHexParams{CombatID: id, Ordering: int32(i), Q: int32(c.Q), R: int32(c.R)}); err != nil {
			return err
		}
	}
	return nil
}

//nolint:gosec // attack numbers and cover are bounded by the rules
func (s *Store) saveAttack(ctx context.Context, f *domain.Combat) error {
	a := f.Attack
	if a == nil {
		return s.q.ClearAttacks(ctx, uuid.UUID(f.ID))
	}
	return s.q.SaveAttack(ctx, queries.SaveAttackParams{
		ID: a.ID, CombatID: uuid.UUID(f.ID), AttackerTokenID: uuid.UUID(a.Attacker), TargetTokenID: uuid.UUID(a.Target), AttackNo: int32(a.AttackNo),
		Mode: a.Mode.String(), CoverBonus: int32(a.CoverBonus), Stage: a.Stage, Critical: a.Critical, RollID: uuid.UUID(a.RollID), Ranged: a.Ranged,
		Total: pgtype.Int4{Int32: int32(a.Total), Valid: a.Stage == domain.StageReaction}, Opportunity: a.Opportunity, OffHand: a.OffHand, Cleave: a.Cleave,
	})
}

// Observations reads the ranged damage each creature of a Session has seen each other creature deal.
func (s *Store) Observations(ctx context.Context, id domain.SessionID) (map[domain.TokenID]map[domain.TokenID]int, error) {
	rows, err := s.q.SessionObservations(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := map[domain.TokenID]map[domain.TokenID]int{}
	for _, r := range rows {
		o := domain.TokenID(r.ObserverTokenID)
		if out[o] == nil {
			out[o] = map[domain.TokenID]int{}
		}
		out[o][domain.TokenID(r.AttackerTokenID)] = int(r.RangedDamage)
	}
	return out, nil
}

// saveEffects rewrites a Session's Effects, Manual prompts and pending saves after a change to them.
//
//nolint:gosec // rounds, DCs and orderings are bounded by the rules
func (s *Store) saveEffects(ctx context.Context, sid uuid.UUID, fx *domain.Effects) error {
	if fx == nil {
		return nil
	}
	for _, clear := range []func(context.Context, uuid.UUID) error{s.q.ClearPendingSaves, s.q.ClearEffects, s.q.ClearManuals} {
		if err := clear(ctx, sid); err != nil {
			return err
		}
	}
	for _, e := range fx.Active {
		if err := s.insertEffect(ctx, sid, e); err != nil {
			return err
		}
	}
	for i, m := range fx.Manual {
		if err := s.q.InsertManual(ctx, queries.InsertManualParams{ID: m.ID, SessionID: sid, Ordering: int32(i), Text: m.Text}); err != nil {
			return err
		}
	}
	for _, p := range fx.Saves {
		if err := s.q.InsertPendingSave(ctx, queries.InsertPendingSaveParams{RollID: uuid.UUID(p.RollID), EffectID: uuid.UUID(p.Effect), SessionID: sid, Dc: int32(p.DC)}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) insertEffect(ctx context.Context, sid uuid.UUID, e domain.Effect) error {
	p := queries.InsertEffectParams{
		ID: uuid.UUID(e.ID), SessionID: sid, TargetTokenID: uuid.UUID(e.Target), Slug: e.Slug, Name: e.Name, Concentration: e.Concentration,
		Level: int32(max(1, e.Level)), //nolint:gosec // at most ten levels
	}
	if e.Source != nil {
		p.SourceTokenID = pgtype.UUID{Bytes: *e.Source, Valid: true}
	}
	if e.RoundsLeft > 0 {
		p.RoundsLeft = pgInt(e.RoundsLeft)
	}
	if e.SaveAbility != "" {
		p.SaveAbility, p.SaveDc = pgtype.Text{String: e.SaveAbility, Valid: true}, pgInt(e.SaveDC)
	}
	p.Mode = pgtype.Text{String: e.Mode, Valid: e.Mode != ""}
	return s.q.InsertEffect(ctx, p)
}

// LoadEffects reads a Session's Effects, Manual prompts and pending saves.
func (s *Store) LoadEffects(ctx context.Context, id domain.SessionID) (domain.Effects, error) {
	sid := uuid.UUID(id)
	var out domain.Effects
	rows, err := s.q.SessionEffects(ctx, sid)
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		e := domain.Effect{
			ID: domain.EffectID(r.ID), Target: domain.TokenID(r.TargetTokenID), Slug: r.Slug, Name: r.Name, Concentration: r.Concentration,
			RoundsLeft: int(r.RoundsLeft.Int32), SaveAbility: r.SaveAbility.String, SaveDC: int(r.SaveDc.Int32), Level: int(r.Level),
			Mode: r.Mode.String,
		}
		if r.SourceTokenID.Valid {
			src := domain.TokenID(r.SourceTokenID.Bytes)
			e.Source = &src
		}
		out.Active = append(out.Active, e)
	}
	manuals, err := s.q.SessionManuals(ctx, sid)
	if err != nil {
		return out, err
	}
	for _, m := range manuals {
		out.Manual = append(out.Manual, domain.ManualPrompt{ID: m.ID, Text: m.Text})
	}
	saves, err := s.q.SessionPendingSaves(ctx, sid)
	if err != nil {
		return out, err
	}
	for _, p := range saves {
		out.Saves = append(out.Saves, domain.PendingSave{RollID: domain.RollID(p.RollID), Effect: domain.EffectID(p.EffectID), DC: int(p.Dc)})
	}
	return out, nil
}

// LastDamage finds the Session's latest damage that no undo has reverted.
func (s *Store) LastDamage(ctx context.Context, id domain.SessionID) (live.HPChange, bool, error) {
	row, err := s.q.LastDamage(ctx, pgtype.UUID{Bytes: id, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		return live.HPChange{}, false, nil
	}
	if err != nil {
		return live.HPChange{}, false, err
	}
	return live.HPChange{Token: domain.TokenID(row.TokenID), Before: int(row.HpBefore), After: int(row.HpAfter), Undoes: row.ID}, true, nil
}

func pgInt(n int) pgtype.Int4 { return pgtype.Int4{Int32: int32(n), Valid: true} } //nolint:gosec // bounded by the rules

// loadReactions reads a Combat's open Reaction Prompt and interrupted walk.
func (s *Store) loadReactions(ctx context.Context, row queries.PlayCombat, out *domain.Combat) error {
	if row.ResumeTokenID.Valid {
		hexes, err := s.q.ResumePath(ctx, row.ID)
		if err != nil {
			return err
		}
		out.Resume = &domain.Resume{Token: domain.TokenID(row.ResumeTokenID.Bytes), CostFt: int(row.ResumeCostFt.Int32)}
		for _, h := range hexes {
			out.Resume.Path = append(out.Resume.Path, hex.Coord{Q: int(h.Q), R: int(h.R)})
		}
	}
	p, err := s.q.CombatPrompt(ctx, row.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	out.Prompt = &domain.ReactionPrompt{
		ID: p.ID, Kind: p.Kind, Reactor: domain.TokenID(p.ReactorTokenID), Trigger: domain.TokenID(p.TriggerTokenID), AttackNo: int(p.AttackNo),
		Effect: p.Effect, Deadline: p.Deadline,
	}
	return nil
}

// ReactionTimeout reads how many seconds a Campaign's Reaction Prompts wait.
func (s *Store) ReactionTimeout(ctx context.Context, campaign uuid.UUID) (int, error) {
	n, err := s.q.CampaignReactionTimeout(ctx, campaign)
	return int(n), err
}

// LoadCombat reads a Session's running Combat, or nil when there is none.
func (s *Store) LoadCombat(ctx context.Context, id domain.SessionID) (*domain.Combat, error) {
	row, err := s.q.RunningCombat(ctx, uuid.UUID(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no running Combat is not an error
	}
	if err != nil {
		return nil, err
	}
	out := &domain.Combat{ID: domain.CombatID(row.ID), Status: row.Status, Round: int(row.Round), Turn: int(row.TurnCount.Int32), StartedAt: row.StartedAt}
	rows, err := s.q.CombatCombatants(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for _, x := range rows {
		out.Combatants = append(out.Combatants, combatantFrom(x))
	}
	if err := s.loadReactions(ctx, row, out); err != nil {
		return nil, err
	}
	a, err := s.q.CombatAttack(ctx, row.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	mode := slices.IndexFunc([]string{"normal", "advantage", "disadvantage"}, func(m string) bool { return m == a.Mode })
	out.Attack = &domain.PendingAttack{
		ID: a.ID, Attacker: domain.TokenID(a.AttackerTokenID), Target: domain.TokenID(a.TargetTokenID), AttackNo: int(a.AttackNo),
		Mode: attack.Mode(mode), CoverBonus: int(a.CoverBonus), Stage: a.Stage, Critical: a.Critical, RollID: domain.RollID(a.RollID),
		Ranged: a.Ranged, Total: int(a.Total.Int32), Opportunity: a.Opportunity, OffHand: a.OffHand, Cleave: a.Cleave,
	}
	return out, nil
}

// LoadMap reads a Map with its walls, lights and remembered hexes.
func (s *Store) LoadMap(ctx context.Context, campaign uuid.UUID, id domain.MapID) (*domain.MapState, error) {
	m, err := s.q.GetMap(ctx, queries.GetMapParams{CampaignID: campaign, ID: uuid.UUID(id)})
	if err != nil {
		return nil, notFound(err)
	}
	out := &domain.MapState{Map: mapRow(m), Walls: map[hex.Coord]bool{}, Lights: []domain.MapLight{}, Reveals: map[hex.Coord]bool{}, Elevation: map[hex.Coord]int{}}
	heights, err := s.q.MapElevations(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, h := range heights {
		out.Elevation[hex.Coord{Q: int(h.Q), R: int(h.R)}] = int(h.ElevationFt)
	}
	walls, err := s.q.MapWalls(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, w := range walls {
		out.Walls[hex.Coord{Q: int(w.Q), R: int(w.R)}] = true
	}
	lights, err := s.q.MapLights(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, l := range lights {
		out.Lights = append(out.Lights, domain.MapLight{ID: domain.LightID(l.ID), At: hex.Coord{Q: int(l.Q), R: int(l.R)}, BrightFt: int(l.BrightFt), DimFt: int(l.DimFt)})
	}
	reveals, err := s.q.MapReveals(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	for _, rv := range reveals {
		out.Reveals[hex.Coord{Q: int(rv.Q), R: int(rv.R)}] = true
	}
	return out, s.loadObjects(ctx, m.ID, out)
}

// loadObjects reads a Map's objects and the links between them.
func (s *Store) loadObjects(ctx context.Context, mapID uuid.UUID, out *domain.MapState) error {
	rows, err := s.q.MapObjects(ctx, mapID)
	if err != nil {
		return err
	}
	out.Objects = map[domain.ObjectID]domain.MapObject{}
	for _, o := range rows {
		out.Objects[o.ID] = domain.MapObject{
			ID: o.ID, Kind: o.Kind, Name: o.Name, At: hex.Coord{Q: int(o.Q), R: int(o.R)}, AC: int(o.ArmorClass), HP: int(o.Hp), HPMax: int(o.HpMax),
			Open: o.Open, Broken: o.Broken, Secret: o.Secret, Effect: o.EffectSlug.String, RadiusFt: int(o.RadiusFt),
			Armed: o.Armed, DetectDC: int(o.DetectDc), DisarmDC: int(o.DisarmDc), TriggerFt: int(o.TriggerFt), Locked: o.Locked, LockDC: int(o.LockDc), Key: o.KeySlug.String,
		}
	}
	links, err := s.q.MapObjectLinks(ctx, mapID)
	if err != nil {
		return err
	}
	for _, l := range links {
		o := out.Objects[l.ObjectID]
		o.Links = append(o.Links, l.TargetID)
		out.Objects[l.ObjectID] = o
	}
	return nil
}

// saveObjects writes the Map Objects a change touched, or deletes a removed one.
//
//nolint:gosec // object numbers are bounded by checks
func (s *Store) saveObjects(ctx context.Context, board *domain.MapState, w live.Write) error {
	if board == nil {
		return nil
	}
	mapID := uuid.UUID(board.Map.ID)
	if w.Kind == domain.ActionObjectRemoved {
		return s.q.DeleteMapObject(ctx, queries.DeleteMapObjectParams{ID: w.Object, MapID: mapID})
	}
	for _, id := range w.Objects {
		o := board.Objects[id]
		p := queries.SaveMapObjectParams{
			ID: o.ID, MapID: mapID, Kind: o.Kind, Name: o.Name, Q: int32(o.At.Q), R: int32(o.At.R), ArmorClass: int32(o.AC), Hp: int32(o.HP), HpMax: int32(o.HPMax),
			Open: o.Open, Broken: o.Broken, Secret: o.Secret, EffectSlug: pgtype.Text{String: o.Effect, Valid: o.Effect != ""}, RadiusFt: int32(o.RadiusFt),
			DetectDc: int32(o.DetectDC), DisarmDc: int32(o.DisarmDC), TriggerFt: int32(o.TriggerFt), Armed: o.Armed, Locked: o.Locked, LockDc: int32(o.LockDC),
			KeySlug: pgtype.Text{String: o.Key, Valid: o.Key != ""},
		}
		if err := s.q.SaveMapObject(ctx, p); err != nil {
			return err
		}
		if err := s.q.ClearMapObjectLinks(ctx, o.ID); err != nil {
			return err
		}
		for _, target := range o.Links {
			if err := s.q.AddMapObjectLink(ctx, queries.AddMapObjectLinkParams{ObjectID: o.ID, TargetID: target}); err != nil {
				return err
			}
		}
	}
	return nil
}

func mapRow(m queries.CampaignMap) domain.Map {
	return domain.Map{
		ID: domain.MapID(m.ID), CampaignID: m.CampaignID, Name: m.Name, Kind: m.Kind, ImageKey: m.ImageKey, ImageType: m.ImageType,
		Width: int(m.WidthPx), Height: int(m.HeightPx), HexSize: m.HexSizePx, OriginX: m.OriginX, OriginY: m.OriginY,
		Ambient: m.Ambient, UpdatedAt: m.UpdatedAt,
	}
}

// Owner holds a Postgres advisory lock per live Session, so two processes can never run the same Session.
type Owner struct {
	Pool *pgxpool.Pool
}

// Acquire takes the lock on a connection of its own, outside the pool: a running Session holds it for
// hours, and pooled connections held that long starve every request once enough Sessions run.
func (o Owner) Acquire(ctx context.Context, id domain.SessionID) (func(), error) {
	conn, err := pgx.ConnectConfig(ctx, o.Pool.Config().ConnConfig.Copy())
	if err != nil {
		return nil, err
	}
	q := queries.New(conn)
	key := "session:" + uuid.UUID(id).String()
	ok, err := q.LockSessionOwner(ctx, key)
	if err != nil || !ok {
		_ = conn.Close(context.Background())
		if err == nil {
			err = apperr.ErrConflict
		}
		return nil, err
	}
	return func() {
		_, _ = q.UnlockSessionOwner(context.Background(), key)
		_ = conn.Close(context.Background())
	}, nil
}

// tokenFrom reads one stored token, without its attacks, saves and reaction settings.
func tokenFrom(t queries.SessionTokensRow) domain.Token {
	tok := domain.Token{
		ID: domain.TokenID(t.ID), Label: t.Label, Kind: t.Kind, Q: int(t.Q), R: int(t.R), Hidden: t.Hidden, DarkvisionFt: int(t.DarkvisionFt),
		Tactics: t.Tactics, CanShield: t.CanShield, Disguise: t.Disguise.String,
	}
	if t.ControllerMemberID.Valid {
		id := uuid.UUID(t.ControllerMemberID.Bytes)
		tok.Controller = &id
	}
	if t.SummonEffectID.Valid {
		e := domain.EffectID(t.SummonEffectID.Bytes)
		tok.Summon = &e
	}
	if t.StatSource.Valid {
		tok.Stats = &domain.Stats{
			Source: t.StatSource.String, AC: int(t.ArmorClass.Int32), HP: int(t.Hp.Int32), HPMax: int(t.HpMax.Int32), Attacks: []domain.Attack{},
			Intelligence: int(t.Intelligence.Int32), SpellDC: int(t.SpellDc.Int32), Stealth: int(t.Stealth), Perception: int(t.Perception),
			Initiative: int(t.Initiative), SpeedFt: int(t.SpeedFt), UnarmedDC: int(t.UnarmedDc), AttacksPerAction: int(t.AttacksPerAction), TempHP: int(t.TempHp),
			Strength: int(t.Strength), CreatureType: t.CreatureType,
		}
		if t.Legend != nil {
			var l domain.Legend
			_ = json.Unmarshal(t.Legend, &l) // written by insertToken and saveLegends
			tok.Stats.Legend = &l
		}
	}
	return tok
}

// combatantFrom reads one stored Combatant.
func combatantFrom(x queries.PlayCombatant) domain.Combatant {
	c := domain.Combatant{
		ID: domain.CombatantID(x.ID), TokenID: domain.TokenID(x.TokenID), RollID: domain.RollID(x.RollID), InitiativeBonus: int(x.InitiativeBonus),
		SpeedFt: int(x.SpeedFt), Done: x.Done, Shielded: x.Shielded, Surprised: x.Surprised, Disengaged: x.Disengaged,
		Economy: combat.Economy{
			Action: x.HasAction, BonusAction: x.HasBonusAction, Reaction: x.HasReaction, MovementFt: int(x.MovementFt),
			AttacksLeft: int(x.AttacksLeft), LightAttack: x.LightAttack, OffHand: x.OffHand, Interaction: x.Interaction, Equips: int(x.Equips),
		},
	}
	if x.Initiative.Valid {
		n := int(x.Initiative.Int32)
		c.Initiative = &n
	}
	c.Cleaved, c.Commanded = x.Cleaved, x.Commanded
	if x.OwnerCombatantID.Valid {
		owner := domain.CombatantID(x.OwnerCombatantID.Bytes)
		c.Owner = &owner
	}
	if x.CleaveFrom.Valid {
		from := domain.TokenID(x.CleaveFrom.Bytes)
		c.CleaveFrom = &from
	}
	if x.ReadiedTrigger.Valid {
		c.Readied = &domain.Readied{Trigger: actions.Trigger{Kind: actions.TriggerKind(x.ReadiedTrigger.String), Who: ""}, AttackNo: int(x.ReadiedAttack.Int32)}
		if x.ReadiedWho.Valid {
			c.Readied.Trigger.Who = uuid.UUID(x.ReadiedWho.Bytes).String()
		}
	}
	return c
}

// loadReactionSettings reads every token's reaction settings.
func (s *Store) loadReactionSettings(ctx context.Context, sid uuid.UUID, tokens []domain.Token) error {
	rows, err := s.q.SessionTokenReactions(ctx, sid)
	if err != nil {
		return err
	}
	for _, r := range rows {
		i := slices.IndexFunc(tokens, func(t domain.Token) bool { return uuid.UUID(t.ID) == r.TokenID })
		if tokens[i].Reactions == nil {
			tokens[i].Reactions = map[string]reactions.Setting{}
		}
		tokens[i].Reactions[r.Kind] = reactions.Setting{Mode: reactions.Mode(r.Mode), Condition: reactions.Condition(r.Condition)}
	}
	return nil
}

// saveReactionSettings writes a token's reaction settings.
func (s *Store) saveReactionSettings(ctx context.Context, t domain.Token) error {
	for kind, set := range t.Reactions {
		p := queries.SetTokenReactionParams{TokenID: uuid.UUID(t.ID), Kind: kind, Mode: string(set.Mode), Condition: string(set.Condition)}
		if err := s.q.SetTokenReaction(ctx, p); err != nil {
			return err
		}
	}
	return nil
}
