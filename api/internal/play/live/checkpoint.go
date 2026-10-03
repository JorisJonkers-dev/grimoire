package live

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
)

// A Checkpoint keeps every row of the Session, so a Session keeps a bounded number of them: fifty the
// DM named, and the start of the last twenty rounds.
const (
	maxCheckpointName = 60
	maxNamed          = 50
	roundsKept        = 20
)

// takesBack lists the commands a Campaign played without undo refuses.
func takesBack(kind string) bool {
	return kind == CmdUndo || kind == CmdUndoDamage || kind == CmdCheckpoint || kind == CmdRewind
}

// round is the round of the fight under way, or 0 when nobody is fighting.
func (s *state) round() int {
	if s.combat == nil || s.combat.Status != domain.CombatActive {
		return 0
	}
	return s.combat.Round
}

func (s *state) checkpointViews() []CheckpointView {
	out := make([]CheckpointView, 0, len(s.checkpoints))
	for _, c := range s.checkpoints {
		out = append(out, CheckpointView{ID: c.ID.String(), Name: c.Name, Kind: c.Kind, Round: c.Round, ActionSeq: c.ActionSeq, At: c.CreatedAt.UTC()})
	}
	return out
}

// checkpoint keeps the Session as it is under the name the DM gives it.
func (r *runtime) checkpoint(req request) {
	name := strings.TrimSpace(req.cmd.Name)
	if name == "" || wireLen(name) > maxCheckpointName {
		r.reject(req, "Name the checkpoint, in up to 60 characters.")
		return
	}
	if r.st.count(domain.CheckpointNamed) >= maxNamed {
		r.reject(req, "A Session keeps up to 50 named checkpoints.")
		return
	}
	c := domain.Checkpoint{ID: uuid.New(), Name: name, Kind: domain.CheckpointNamed, Round: r.st.round(), ActionSeq: 0, CreatedAt: r.now()}
	done, err := r.store.SaveCheckpoint(context.Background(), r.st.session, c, req.from.Member, req.from.Caller)
	if err != nil {
		r.log.Error("live: save checkpoint", "error", err)
		r.reject(req, "That checkpoint could not be saved.")
		return
	}
	c.ActionSeq = done.Action
	next := r.st.clone()
	next.session.Seq, next.checkpoints = done.Seq, append(slices.Clone(r.st.checkpoints), c)
	r.st = next
	for sub := range r.subs {
		v := r.st.project(sub.Audience)
		u := Update{Kind: UpdView, Seq: done.Seq, View: &v}
		if sub == req.from {
			u.Nonce, u.ActionSeq = req.cmd.Nonce, done.Action
		}
		r.send(sub, u)
	}
}

// markRound keeps the start of every round of a fight as a Checkpoint of its own.
func (r *runtime) markRound(prev, next *state, action int64) {
	if next.noUndo || next.round() <= prev.round() {
		return
	}
	c := domain.Checkpoint{
		ID: uuid.New(), Name: fmt.Sprintf("Round %d", next.round()), Kind: domain.CheckpointRound, Round: next.round(), ActionSeq: action, CreatedAt: r.now(),
	}
	if err := r.store.MarkRound(context.Background(), next.session, c, roundsKept); err != nil {
		r.log.Error("live: mark round", "error", err)
		return
	}
	// The oldest rounds beyond those kept go, as they went where the Session is kept.
	over := next.count(domain.CheckpointRound) + 1 - roundsKept
	list := make([]domain.Checkpoint, 0, len(next.checkpoints)+1)
	for _, old := range next.checkpoints {
		if old.Kind == domain.CheckpointRound && over > 0 {
			over--
			continue
		}
		list = append(list, old)
	}
	list = append(list, c)
	next.checkpoints = list
}

// count is how many Checkpoints of a kind the Session keeps.
func (s *state) count(kind string) int {
	n := 0
	for _, c := range s.checkpoints {
		if c.Kind == kind {
			n++
		}
	}
	return n
}

// rewind puts the Session back as a Checkpoint kept it and shows every screen the Session afresh.
func (r *runtime) rewind(req request) {
	id, err := uuid.Parse(req.cmd.CheckpointID)
	at := slices.IndexFunc(r.st.checkpoints, func(c domain.Checkpoint) bool { return c.ID == id })
	if err != nil || at < 0 {
		r.reject(req, "No such checkpoint.")
		return
	}
	// The Session is read back inside the rewind: one that cannot be read is not made.
	var st *state
	done, err := r.store.Rewind(context.Background(), r.st.session, r.st.checkpoints[at], req.from.Member, req.from.Caller, r.now(), func(tx Store) error {
		var err error
		st, err = r.reload(context.Background(), tx)
		return err
	})
	if err != nil {
		r.log.Error("live: rewind", "error", err)
		r.reject(req, "The rewind could not be made.")
		return
	}
	r.st, r.lastRoll, r.armed = st, nil, uuid.Nil
	for sub := range r.subs {
		u := r.snapshot(sub.Audience)
		if sub == req.from {
			u.Nonce, u.ActionSeq = req.cmd.Nonce, done.Action
		}
		r.send(sub, u)
	}
	r.arm()
}
