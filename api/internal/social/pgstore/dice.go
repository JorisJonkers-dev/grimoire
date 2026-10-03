package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

func diceSet(r queries.SocialDiceSet) domain.DiceSet {
	d := domain.DiceSet{
		ID: r.ID, Owner: r.OwnerAccount, Name: r.Name, Design: domain.DiceDesign{Dice: map[string]domain.DieLook{}}, Image: nil,
		Sharing: r.Sharing, Review: r.Review, CopiedFrom: nil, MadeBy: r.MadeBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
	_ = json.Unmarshal(r.Design, &d.Design) // written by InsertDiceSet and UpdateDiceSet
	if r.ImageKey.Valid {
		d.Image = &domain.Picture{Key: r.ImageKey.String, Type: r.ImageType.String}
	}
	if r.CopiedFrom.Valid {
		id := domain.DiceSetID(r.CopiedFrom.Bytes)
		d.CopiedFrom = &id
	}
	return d
}

func diceSets(rows []queries.SocialDiceSet, err error) ([]domain.DiceSet, error) {
	if err != nil {
		return nil, err
	}
	out := make([]domain.DiceSet, 0, len(rows))
	for _, r := range rows {
		out = append(out, diceSet(r))
	}
	return out, nil
}

func pictureColumns(img *domain.Picture) (pgtype.Text, pgtype.Text) {
	if img == nil {
		return pgtype.Text{}, pgtype.Text{}
	}
	return pgtype.Text{String: img.Key, Valid: true}, pgtype.Text{String: img.Type, Valid: true}
}

// InsertDiceSet stores a new Dice Set; a second copy of the same set is a conflict.
func (s *Store) InsertDiceSet(ctx context.Context, d domain.DiceSet) error {
	design, _ := json.Marshal(d.Design) //nolint:errchkjson // a design is plain data
	key, kind := pictureColumns(d.Image)
	p := queries.InsertDiceSetParams{
		ID: d.ID, OwnerAccount: d.Owner, Name: d.Name, Design: design, ImageKey: key, ImageType: kind, Review: d.Review, CopiedFrom: pgtype.UUID{}, MadeBy: d.MadeBy, Now: d.CreatedAt,
	}
	if d.CopiedFrom != nil {
		p.CopiedFrom = pgtype.UUID{Bytes: *d.CopiedFrom, Valid: true}
	}
	return conflictOf(s.q.InsertDiceSet(ctx, p))
}

// DiceSet reads one Dice Set.
func (s *Store) DiceSet(ctx context.Context, id domain.DiceSetID) (domain.DiceSet, error) {
	r, err := s.q.DiceSet(ctx, id)
	if err != nil {
		return domain.DiceSet{}, notFound(err)
	}
	return diceSet(r), nil
}

// DiceSetsOf lists an Account's sets by name: its own and its copies.
func (s *Store) DiceSetsOf(ctx context.Context, owner domain.AccountID) ([]domain.DiceSet, error) {
	return diceSets(s.q.DiceSetsOf(ctx, owner))
}

// DiceSetsSharedWith lists the sets of others an Account may see.
func (s *Store) DiceSetsSharedWith(ctx context.Context, me domain.AccountID) ([]domain.DiceSet, error) {
	return diceSets(s.q.DiceSetsSharedWith(ctx, me))
}

// DiceSetsAwaitingReview lists the sets that wait for an Admin, oldest first.
func (s *Store) DiceSetsAwaitingReview(ctx context.Context) ([]domain.DiceSet, error) {
	return diceSets(s.q.DiceSetsAwaitingReview(ctx))
}

// UpdateDiceSet sets a set's name and looks.
func (s *Store) UpdateDiceSet(ctx context.Context, id domain.DiceSetID, name string, design domain.DiceDesign, now time.Time) error {
	raw, _ := json.Marshal(design) //nolint:errchkjson // a design is plain data
	return s.q.UpdateDiceSet(ctx, queries.UpdateDiceSetParams{Name: name, Design: raw, Now: now, ID: id})
}

// SetDiceSetSharing sets who a set is shared with; the review follows from the set as it then stands.
func (s *Store) SetDiceSetSharing(ctx context.Context, id domain.DiceSetID, sharing string, now time.Time) error {
	return s.q.SetDiceSetSharing(ctx, queries.SetDiceSetSharingParams{Sharing: sharing, Now: now, ID: id})
}

// SetDiceSetImage sets or clears a set's uploaded picture; the review follows from the set as it then stands.
func (s *Store) SetDiceSetImage(ctx context.Context, id domain.DiceSetID, img *domain.Picture, now time.Time) error {
	key, kind := pictureColumns(img)
	return s.q.SetDiceSetImage(ctx, queries.SetDiceSetImageParams{ImageKey: key, ImageType: kind, Now: now, ID: id})
}

// SetDiceSetReview records an Admin's decision on a set that waits with this picture on it. A set that
// stopped waiting, or carries another picture by now, is a conflict.
func (s *Store) SetDiceSetReview(ctx context.Context, id domain.DiceSetID, review, imageKey string, now time.Time) error {
	n, err := s.q.SetDiceSetReview(ctx, queries.SetDiceSetReviewParams{Review: review, Now: now, ID: id, ImageKey: imageKey})
	if err == nil && n == 0 {
		return domain.ErrConflict
	}
	return err
}

// DeleteDiceSet removes a set; whoever rolled with it goes back to the plain dice.
func (s *Store) DeleteDiceSet(ctx context.Context, id domain.DiceSetID) error {
	return s.q.DeleteDiceSet(ctx, id)
}

// ChooseDiceSet sets the set an Account rolls with.
func (s *Store) ChooseDiceSet(ctx context.Context, account domain.AccountID, id domain.DiceSetID) error {
	return s.q.ChooseDiceSet(ctx, queries.ChooseDiceSetParams{AccountID: account, DiceSetID: id})
}

// UnchooseDiceSet puts an Account back on the plain dice.
func (s *Store) UnchooseDiceSet(ctx context.Context, account domain.AccountID) error {
	return s.q.UnchooseDiceSet(ctx, account)
}

// ChosenDiceSet reads the set an Account rolls with, or nil for the plain dice.
func (s *Store) ChosenDiceSet(ctx context.Context, account domain.AccountID) (*domain.DiceSet, error) {
	r, err := s.q.ChosenDiceSet(ctx, account)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil //nolint:nilnil // no chosen set is not an error
	}
	if err != nil {
		return nil, err
	}
	d := diceSet(r)
	return &d, nil
}
