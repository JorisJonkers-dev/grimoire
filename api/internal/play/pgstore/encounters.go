package pgstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	prep "github.com/JorisJonkers-dev/grimoire/api/internal/prep/domain"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// LoadPrep reads everything an Encounter Check draws on.
func (s *Store) LoadPrep(ctx context.Context, campaign uuid.UUID) (prep.Prep, error) {
	return preppg.LoadPrep(ctx, s.q, campaign)
}

// LoadChecks reads a Session's Encounter Checks.
func (s *Store) LoadChecks(ctx context.Context, campaign uuid.UUID, sid domain.SessionID) ([]prep.Check, error) {
	return preppg.SessionChecks(ctx, s.q, campaign, uuid.UUID(sid))
}

// saveCheck writes an Encounter Check with its Revision, the percentile roll an open check opens, and
// the checks scheduled or used up.
func (s *Store) saveCheck(ctx context.Context, sess domain.Session, w live.Write, actor domain.Member, c caller.Caller, now time.Time) error {
	if sch := w.Schedule; sch != nil {
		return s.q.InsertScheduledCheck(ctx, queries.InsertScheduledCheckParams{ID: sch.ID, CampaignID: sess.CampaignID, TableID: uuid.UUID(sch.TableID), Due: sch.Due, Now: now})
	}
	if w.Check == nil {
		return nil
	}
	if w.Unschedule != (uuid.UUID{}) {
		if err := s.q.DeleteScheduledCheck(ctx, w.Unschedule); err != nil {
			return err
		}
	}
	action := "update"
	if w.Kind == domain.ActionEncounterChecked {
		action = "create"
		if err := s.openRolls(ctx, sess, w.Rolls, actor, c, now); err != nil {
			return err
		}
	}
	if err := preppg.WriteCheck(ctx, s.q, sess.CampaignID, *w.Check, now); err != nil {
		return err
	}
	return preppg.RecordCheck(ctx, s.q, sess.CampaignID, w.Check.ID, action, actor.Name, c, now)
}
