// Package app holds the play use cases.
package app

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/caller"
)

// Members looks up who may act in a Campaign.
type Members interface {
	Membership(ctx context.Context, campaign uuid.UUID, subject string) (domain.Member, error)
	Member(ctx context.Context, campaign, member uuid.UUID) (domain.Member, error)
}

// LogEntry is an Action about to be appended.
type LogEntry struct {
	Kind   string
	Actor  domain.Member
	Caller caller.Caller
	Seed   *uint64
	RollID domain.RollID
	DieNo  *int
	Value  int
	At     time.Time
}

// Repository is the play persistence port.
type Repository interface {
	InTx(ctx context.Context, fn func(Repository) error) error
	InsertRoll(ctx context.Context, r domain.Roll, now time.Time) (domain.RollID, error)
	LockRoll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (string, error)
	Roll(ctx context.Context, campaign uuid.UUID, id domain.RollID) (domain.Roll, error)
	Rolls(ctx context.Context, campaign uuid.UUID, limit int) ([]domain.Roll, error)
	SetDie(ctx context.Context, id domain.RollID, no, value int, mode string) (bool, error)
	ResolveRoll(ctx context.Context, id domain.RollID, total int, now time.Time) error
	Append(ctx context.Context, campaign uuid.UUID, e LogEntry) error
	ActionLog(ctx context.Context, campaign uuid.UUID, limit int) ([]domain.Action, error)
}

// Rolls runs the dice use cases.
type Rolls struct {
	Repo    Repository
	Members Members
	Seed    func() uint64
	Source  func(seed uint64) dice.Source
	Now     func() time.Time
}

// RollInput is a new Roll Request.
type RollInput struct {
	Purpose   string
	Notation  string
	Labels    map[int]string
	Modifiers []domain.Modifier
	// Roller is who throws the dice; empty means the caller. Only a DM may ask someone else.
	Roller *uuid.UUID
}

// Fill is one die being set: rolled by the server, or entered from a physical die.
type Fill struct {
	Auto  bool
	Value int
}

// invalidNotation turns a dice error into a reason for the player: the detail after the last colon.
func invalidNotation(err error) error {
	msg := err.Error()
	return apperr.Refuse(msg[strings.LastIndex(msg, ": ")+2:])
}

func validInput(in RollInput) (RollInput, dice.Spec, error) {
	in.Purpose = strings.TrimSpace(in.Purpose)
	if in.Purpose == "" || utf8.RuneCountInString(in.Purpose) > 120 {
		return in, dice.Spec{}, apperr.ErrInvalid
	}
	spec, err := dice.Parse(in.Notation)
	if err != nil {
		return in, dice.Spec{}, invalidNotation(err)
	}
	in.Notation = spec.String()
	for g := range in.Labels {
		if g < 0 || g >= len(spec.Groups) {
			return in, dice.Spec{}, apperr.ErrInvalid
		}
	}
	return in, spec, nil
}

// Create opens a Roll Request.
func (s *Rolls) Create(ctx context.Context, c caller.Caller, campaign uuid.UUID, in RollInput) (domain.Roll, error) {
	in, spec, err := validInput(in)
	if err != nil {
		return domain.Roll{}, err
	}
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return domain.Roll{}, err
	}
	roller := me
	if in.Roller != nil && *in.Roller != me.ID {
		if !me.DM {
			return domain.Roll{}, apperr.ErrForbidden
		}
		if roller, err = s.Members.Member(ctx, campaign, *in.Roller); err != nil {
			return domain.Roll{}, err
		}
	}
	r := domain.Roll{
		CampaignID: campaign, Purpose: in.Purpose, Notation: in.Notation, Labels: in.Labels, Modifiers: in.Modifiers,
		RequestedBy: me.Name, Roller: roller, Status: domain.StatusPending,
	}
	for g, group := range spec.Groups {
		for range group.Count {
			r.Dice = append(r.Dice, domain.Die{No: len(r.Dice), Group: g, Faces: group.Faces})
		}
	}
	var id domain.RollID
	err = s.Repo.InTx(ctx, func(tx Repository) error {
		now := s.Now()
		if id, err = tx.InsertRoll(ctx, r, now); err != nil {
			return err
		}
		return tx.Append(ctx, campaign, LogEntry{Kind: domain.ActionRollRequested, Actor: me, Caller: c, RollID: id, At: now})
	})
	if err != nil {
		return domain.Roll{}, err
	}
	return s.Get(ctx, c, campaign, id)
}

// Get returns a Roll Request to any Member.
func (s *Rolls) Get(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.RollID) (domain.Roll, error) {
	if _, err := s.Members.Membership(ctx, campaign, c.Subject); err != nil {
		return domain.Roll{}, err
	}
	r, err := s.Repo.Roll(ctx, campaign, id)
	if err != nil {
		return domain.Roll{}, err
	}
	return withKept(r), nil
}

// withKept marks the dice that count, once every die of their group has a face.
func withKept(r domain.Roll) domain.Roll {
	spec, _ := dice.Parse(r.Notation) // stored notation was validated when the request was made
	for g, group := range spec.Groups {
		var idx, faces []int
		for i, d := range r.Dice {
			if d.Group == g {
				idx, faces = append(idx, i), append(faces, d.Value)
			}
		}
		if len(faces) != group.Count || containsZero(faces) {
			continue
		}
		for j, k := range group.Kept(faces) {
			r.Dice[idx[j]].Kept = k
		}
	}
	return r
}

func containsZero(xs []int) bool {
	for _, x := range xs {
		if x == 0 {
			return true
		}
	}
	return false
}

// List returns the Campaign's recent Roll Requests, newest first.
func (s *Rolls) List(ctx context.Context, c caller.Caller, campaign uuid.UUID, limit int) ([]domain.Roll, error) {
	if _, err := s.Members.Membership(ctx, campaign, c.Subject); err != nil {
		return nil, err
	}
	rolls, err := s.Repo.Rolls(ctx, campaign, limit)
	for i := range rolls {
		rolls[i] = withKept(rolls[i])
	}
	return rolls, err
}

// Log returns the Campaign's recent Action Log. DM only.
func (s *Rolls) Log(ctx context.Context, c caller.Caller, campaign uuid.UUID, limit int) ([]domain.Action, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return nil, err
	}
	if !me.DM {
		return nil, apperr.ErrForbidden
	}
	return s.Repo.ActionLog(ctx, campaign, limit)
}

// SetDie rolls or enters one die; the roller or a DM, while the request is pending. The last die resolves it.
func (s *Rolls) SetDie(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.RollID, no int, f Fill) (domain.Roll, error) {
	return s.fill(ctx, c, campaign, id, func(r domain.Roll) (map[int]Fill, error) {
		if no < 0 || no >= len(r.Dice) {
			return nil, apperr.ErrNotFound
		}
		if r.Dice[no].Value != 0 {
			return nil, apperr.ErrConflict
		}
		return map[int]Fill{no: f}, nil
	})
}

// RollRest rolls every die still empty.
func (s *Rolls) RollRest(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.RollID) (domain.Roll, error) {
	return s.fill(ctx, c, campaign, id, func(r domain.Roll) (map[int]Fill, error) {
		out := map[int]Fill{}
		for _, d := range r.Dice {
			if d.Value == 0 {
				out[d.No] = Fill{Auto: true}
			}
		}
		return out, nil
	})
}

func (s *Rolls) fill(ctx context.Context, c caller.Caller, campaign uuid.UUID, id domain.RollID, pick func(domain.Roll) (map[int]Fill, error)) (domain.Roll, error) {
	me, err := s.Members.Membership(ctx, campaign, c.Subject)
	if err != nil {
		return domain.Roll{}, err
	}
	err = s.Repo.InTx(ctx, func(tx Repository) error {
		status, err := tx.LockRoll(ctx, campaign, id)
		if err != nil {
			return err
		}
		r, err := tx.Roll(ctx, campaign, id)
		if err != nil {
			return err
		}
		if r.Roller.ID != me.ID && !me.DM {
			return apperr.ErrForbidden
		}
		if status != domain.StatusPending {
			return apperr.ErrConflict
		}
		fills, err := pick(r)
		if err != nil {
			return err
		}
		return s.apply(ctx, tx, c, me, r, fills)
	})
	if err != nil {
		return domain.Roll{}, err
	}
	return s.Get(ctx, c, campaign, id)
}

func (s *Rolls) apply(ctx context.Context, tx Repository, c caller.Caller, me domain.Member, r domain.Roll, fills map[int]Fill) error {
	now := s.Now()
	for i := range r.Dice {
		f, ok := fills[i]
		if !ok {
			continue
		}
		d := &r.Dice[i]
		entry := LogEntry{Kind: domain.ActionDieEntered, Actor: me, Caller: c, RollID: r.ID, DieNo: &d.No, At: now}
		if f.Auto {
			seed := s.Seed()
			f.Value, entry.Kind, entry.Seed = dice.Face(s.Source(seed), d.Faces), domain.ActionDieRolled, &seed
		} else if err := dice.CheckFace(d.Faces, f.Value); err != nil {
			return invalidNotation(err)
		}
		mode := map[bool]string{true: domain.ModeAuto, false: domain.ModeManual}[f.Auto]
		if _, err := tx.SetDie(ctx, r.ID, d.No, f.Value, mode); err != nil {
			return err
		}
		d.Value, entry.Value = f.Value, f.Value
		if err := tx.Append(ctx, r.CampaignID, entry); err != nil {
			return err
		}
	}
	return s.resolve(ctx, tx, c, me, r, now)
}

func (s *Rolls) resolve(ctx context.Context, tx Repository, c caller.Caller, me domain.Member, r domain.Roll, now time.Time) error {
	spec, _ := dice.Parse(r.Notation) // stored notation was validated when the request was made
	faces := make([][]int, len(spec.Groups))
	for _, d := range r.Dice {
		if d.Value == 0 {
			return nil
		}
		faces[d.Group] = append(faces[d.Group], d.Value)
	}
	modifier := 0
	for _, m := range r.Modifiers {
		modifier += m.Value
	}
	res, _ := dice.Resolve(spec, faces, modifier) // every face was checked as it was set
	if err := tx.ResolveRoll(ctx, r.ID, res.Total, now); err != nil {
		return err
	}
	return tx.Append(ctx, r.CampaignID, LogEntry{Kind: domain.ActionRollResolved, Actor: me, Caller: c, RollID: r.ID, Value: res.Total, At: now})
}
