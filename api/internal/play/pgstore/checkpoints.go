package pgstore

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// kept is one table a Checkpoint keeps, and which of its rows belong to a Session ($1).
type kept struct {
	table, where string
}

//nolint:gosec // which rows of a table are a Session's; no credentials
const (
	ofTokens  = "IN (SELECT id FROM play.tokens WHERE session_id = $1)"
	ofCombats = "combat_id IN (SELECT id FROM play.combats WHERE session_id = $1)"
	ofCasts   = "cast_id IN (SELECT id FROM play.area_casts WHERE session_id = $1)"
	ofZones   = "zone_id IN (SELECT id FROM play.encounter_zones WHERE session_id = $1)"
	onTheMap  = "map_id = (SELECT map_id FROM play.sessions WHERE id = $1)"
	ofObjects = "IN (SELECT id FROM campaign.map_objects WHERE " + onTheMap + ")"
	ofParty   = "character_id IN (SELECT c.id FROM campaign.characters c JOIN play.sessions s ON s.campaign_id = c.campaign_id WHERE s.id = $1)"
	inSession = "session_id = $1"
)

// CheckpointTables are the tables a Checkpoint keeps, a table after every table it refers to: the
// board, the fight, the Effects, what waits on rolls, the map as play changed it, and what the party's
// Characters have spent. A rewind empties them last to first and fills them first to last.
//
// What a Checkpoint leaves alone stays as it is after a rewind: Containers and coins, Shops, finished
// rests on the Character sheet, the Table Display, the world map and the Game Clock.
func CheckpointTables() []string {
	out := make([]string, 0, len(keptTables()))
	for _, k := range keptTables() {
		out = append(out, k.table)
	}
	return out
}

func keptTables() []kept {
	return []kept{
		{"play.tokens", inSession},
		{"play.token_attacks", "token_id " + ofTokens},
		{"play.token_forms", "token_id " + ofTokens},
		{"play.token_qualities", "token_id " + ofTokens},
		{"play.token_reactions", "token_id " + ofTokens},
		{"play.token_saves", "token_id " + ofTokens},
		{"play.token_senses", "token_id " + ofTokens},
		{"play.observed_damage", "observer_token_id " + ofTokens},
		{"play.combats", inSession},
		{"play.combatants", ofCombats},
		{"play.attacks", ofCombats},
		{"play.combat_resume_path", ofCombats},
		{"play.reaction_prompts", ofCombats},
		{"play.active_effects", inSession},
		{"play.pending_saves", inSession},
		{"play.manual_prompts", inSession},
		{"play.area_casts", inSession},
		{"play.area_hexes", ofCasts},
		{"play.area_targets", ofCasts},
		{"play.dying", inSession},
		{"play.pending_actions", inSession},
		{"play.sneak_rolls", inSession},
		{"play.surfaces", inSession},
		{"play.exploration_turns", inSession},
		{"play.encounter_zones", inSession},
		{"play.zone_checks", ofZones},
		{"play.zone_creatures", ofZones},
		{"play.rests", inSession},
		{"play.rest_resters", inSession},
		{"play.rest_agreements", inSession},
		{"campaign.map_reveals", onTheMap},
		{"campaign.map_walls", onTheMap},
		{"campaign.map_lights", onTheMap},
		{"campaign.map_elevations", onTheMap},
		{"campaign.map_objects", onTheMap},
		{"campaign.map_object_links", "object_id " + ofObjects},
		{"campaign.character_resources", ofParty},
	}
}

// The rolls a Checkpoint keeps are the ones still out when it was made: a rewind opens them again.
const (
	rollsOut   = "status = 'pending' AND campaign_id = (SELECT campaign_id FROM play.sessions WHERE id = $1)"
	stateOf    = "(SELECT state FROM play.checkpoints WHERE id = $2 AND session_id = $1)"
	rollsKept  = "(SELECT (r->>'id')::uuid FROM jsonb_array_elements(" + stateOf + "->'play.roll_requests') r)"
	rollsAsked = `SELECT roll_id FROM play.combatants WHERE ` + ofCombats + `
		UNION SELECT roll_id FROM play.attacks WHERE ` + ofCombats + `
		UNION SELECT damage_roll_id FROM play.area_casts WHERE session_id = $1
		UNION SELECT save_roll_id FROM play.area_targets WHERE ` + ofCasts + `
		UNION SELECT roll_id FROM play.dying WHERE session_id = $1
		UNION SELECT roll_id FROM play.pending_actions WHERE session_id = $1
		UNION SELECT roll_id FROM play.pending_saves WHERE session_id = $1
		UNION SELECT roll_id FROM play.rest_resters WHERE session_id = $1
		UNION SELECT roll_id FROM play.sneak_rolls WHERE session_id = $1
		UNION SELECT roll_id FROM play.zone_checks WHERE ` + ofZones
)

// snapshotSQL stores a Checkpoint with every kept row of the Session, by table.
func snapshotSQL() string {
	var b strings.Builder
	b.WriteString("INSERT INTO play.checkpoints (id, session_id, name, kind, round, action_seq, created_at, state)\n")
	b.WriteString("SELECT $2, $1, $3, $4, $5, $6, $7, jsonb_build_object(\n")
	b.WriteString("'session', (SELECT to_jsonb(s) FROM play.sessions s WHERE s.id = $1),\n")
	b.WriteString("'play.roll_requests', (SELECT coalesce(jsonb_agg(to_jsonb(t)), '[]'::jsonb) FROM play.roll_requests t WHERE " + rollsOut + "),\n")
	b.WriteString("'play.roll_dice', (SELECT coalesce(jsonb_agg(to_jsonb(t)), '[]'::jsonb) FROM play.roll_dice t WHERE roll_id IN (SELECT id FROM play.roll_requests WHERE " + rollsOut + "))")
	for _, k := range keptTables() {
		b.WriteString(",\n'" + k.table + "', (SELECT coalesce(jsonb_agg(to_jsonb(t)), '[]'::jsonb) FROM " + k.table + " t WHERE " + k.where + ")")
	}
	b.WriteString(")")
	return b.String()
}

// step is one statement of a rewind; one that reads the Checkpoint takes its id as well as the Session's.
type step struct {
	sql   string
	reads bool
}

// restoreSteps are the statements of a rewind, in order: the Session's own settings, the kept tables
// emptied last to first and filled first to last, and the rolls that were out opened again.
func restoreSteps() []step {
	tables := keptTables()
	out := []step{{`UPDATE play.sessions s SET map_id = c.map_id, world_map_id = c.world_map_id, sneaking = c.sneaking, grid_radius = c.grid_radius
		FROM jsonb_populate_record(NULL::play.sessions, ` + stateOf + `->'session') c WHERE s.id = $1`, true}}
	for i := len(tables) - 1; i >= 0; i-- {
		out = append(out, step{"DELETE FROM " + tables[i].table + " WHERE " + tables[i].where, false})
	}
	for _, k := range tables {
		out = append(out, step{"INSERT INTO " + k.table + " SELECT * FROM jsonb_populate_recordset(NULL::" + k.table + ", " + stateOf + "->'" + k.table + "')", true})
	}
	return append(out,
		step{`UPDATE play.roll_requests r SET status = c.status, total = c.total, resolved_at = c.resolved_at, choosing = c.choosing, rerolled = c.rerolled
		FROM jsonb_populate_recordset(NULL::play.roll_requests, ` + stateOf + `->'play.roll_requests') c WHERE r.id = c.id`, true},
		step{"DELETE FROM play.roll_dice WHERE roll_id IN " + rollsKept, true},
		step{"INSERT INTO play.roll_dice SELECT * FROM jsonb_populate_recordset(NULL::play.roll_dice, " + stateOf + "->'play.roll_dice')", true},
	)
}

func checkpoint(r queries.ListCheckpointsRow) domain.Checkpoint {
	return domain.Checkpoint{ID: r.ID, Name: r.Name, Kind: r.Kind, Round: int(r.Round), ActionSeq: r.ActionSeq, CreatedAt: r.CreatedAt}
}

// Checkpoints lists a Session's Checkpoints, oldest first.
func (s *Store) Checkpoints(ctx context.Context, id domain.SessionID) ([]domain.Checkpoint, error) {
	rows, err := s.q.ListCheckpoints(ctx, uuid.UUID(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.Checkpoint, 0, len(rows))
	for _, r := range rows {
		out = append(out, checkpoint(r))
	}
	return out, nil
}

// NoUndo reads whether a Campaign is played without undo.
func (s *Store) NoUndo(ctx context.Context, campaign uuid.UUID) (bool, error) {
	return s.q.CampaignNoUndo(ctx, campaign)
}

func (s *Store) snapshot(ctx context.Context, sid domain.SessionID, c domain.Checkpoint) error {
	//nolint:gosec // a round is small
	_, err := s.db.Exec(ctx, snapshotSQL(), uuid.UUID(sid), c.ID, c.Name, c.Kind, int32(c.Round), c.ActionSeq, c.CreatedAt)
	return err
}

// MarkRound keeps the start of a round as a Checkpoint, after the Action that started it, and lets go
// of the rounds older than the latest keep. It is not an Action itself: nobody did it.
func (s *Store) MarkRound(ctx context.Context, sess domain.Session, c domain.Checkpoint, keep int) error {
	return s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		if err := tx.snapshot(ctx, sess.ID, c); err != nil {
			return err
		}
		//nolint:gosec // a handful of rounds
		return tx.q.DropOldRounds(ctx, queries.DropOldRoundsParams{SessionID: uuid.UUID(sess.ID), Keep: int32(keep)})
	})
}

// SaveCheckpoint keeps the Session as it is under the name the DM gave, and logs that.
func (s *Store) SaveCheckpoint(ctx context.Context, sess domain.Session, c domain.Checkpoint, actor domain.Member, cl caller.Caller) (live.Committed, error) {
	var done live.Committed
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		var err error
		if done.Seq, err = tx.q.BumpSessionSeq(ctx, uuid.UUID(sess.ID)); err != nil {
			return err
		}
		if _, done.Action, err = tx.loggedAction(ctx, sess.CampaignID, sess.ID, domain.ActionCheckpointCreated, actor, cl, c.CreatedAt); err != nil {
			return err
		}
		c.ActionSeq = done.Action
		return tx.snapshot(ctx, sess.ID, c)
	})
	return done, err
}

// staleRolls are the rolls asked for since a Checkpoint that nobody answered: a rewind withdraws them.
func (s *Store) staleRolls(ctx context.Context, sid, checkpoint uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.db.Query(ctx, "SELECT id FROM play.roll_requests WHERE status = 'pending' AND id IN ("+rollsAsked+") AND id NOT IN "+rollsKept, sid, checkpoint)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

// restore puts the kept rows back and withdraws the rolls asked for since.
func (s *Store) restore(ctx context.Context, sid, checkpoint uuid.UUID) error {
	var there bool
	if err := s.db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM play.checkpoints WHERE id = $2 AND session_id = $1)", sid, checkpoint).Scan(&there); err != nil {
		return err
	}
	if !there {
		return apperr.ErrNotFound
	}
	stale, err := s.staleRolls(ctx, sid, checkpoint)
	if err != nil {
		return err
	}
	for _, st := range restoreSteps() {
		args := []any{sid}
		if st.reads {
			args = append(args, checkpoint)
		}
		if _, err := s.db.Exec(ctx, st.sql, args...); err != nil {
			return err
		}
	}
	_, err = s.db.Exec(ctx, "DELETE FROM play.roll_requests WHERE id = ANY($1) AND status = 'pending'", stale)
	return err
}

// Rewind puts the Session back as a Checkpoint kept it, and logs that. Rolls asked for since then
// that nobody answered are withdrawn, and Checkpoints of the future it takes back go with it. read is
// handed the store of the same transaction; when it fails, nothing of the rewind stays.
func (s *Store) Rewind(ctx context.Context, sess domain.Session, c domain.Checkpoint, actor domain.Member, cl caller.Caller, now time.Time, read func(live.Store) error) (live.Committed, error) {
	var done live.Committed
	sid := uuid.UUID(sess.ID)
	err := s.InTx(ctx, func(r app.Repository) error {
		tx := r.(*Store) //nolint:forcetypeassert // InTx always hands back a *Store
		var err error
		if done.Seq, err = tx.q.BumpSessionSeq(ctx, sid); err != nil {
			return err
		}
		if err := tx.restore(ctx, sid, c.ID); err != nil {
			return err
		}
		if err := tx.q.DropLaterCheckpoints(ctx, queries.DropLaterCheckpointsParams{SessionID: sid, ActionSeq: c.ActionSeq}); err != nil {
			return err
		}
		var action uuid.UUID
		if action, done.Action, err = tx.loggedAction(ctx, sess.CampaignID, sess.ID, domain.ActionSessionRewound, actor, cl, now); err != nil {
			return err
		}
		if err := tx.q.InsertRewind(ctx, queries.InsertRewindParams{ActionID: action, SessionID: sid, ToActionSeq: c.ActionSeq}); err != nil {
			return err
		}
		return read(tx)
	})
	return done, err
}
