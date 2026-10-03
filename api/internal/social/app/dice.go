package app

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/picture"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// Blobs is the object storage port, for the pictures uploaded onto Dice Sets.
type Blobs interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// DiceRepository keeps Dice Sets and which one each Account rolls with.
type DiceRepository interface {
	InsertDiceSet(ctx context.Context, d domain.DiceSet) error
	DiceSet(ctx context.Context, id domain.DiceSetID) (domain.DiceSet, error)
	DiceSetsOf(ctx context.Context, owner domain.AccountID) ([]domain.DiceSet, error)
	DiceSetsSharedWith(ctx context.Context, me domain.AccountID) ([]domain.DiceSet, error)
	DiceSetsAwaitingReview(ctx context.Context) ([]domain.DiceSet, error)
	UpdateDiceSet(ctx context.Context, id domain.DiceSetID, name string, design domain.DiceDesign, now time.Time) error
	// SetDiceSetSharing and SetDiceSetImage leave the review where the set then stands: waiting when it is
	// shared with everyone and carries a picture, and needing none otherwise.
	SetDiceSetSharing(ctx context.Context, id domain.DiceSetID, sharing string, now time.Time) error
	SetDiceSetImage(ctx context.Context, id domain.DiceSetID, img *domain.Picture, now time.Time) error
	// SetDiceSetReview decides on a set that waits with this picture on it; any other is a conflict.
	SetDiceSetReview(ctx context.Context, id domain.DiceSetID, review, imageKey string, now time.Time) error
	DeleteDiceSet(ctx context.Context, id domain.DiceSetID) error
	ChooseDiceSet(ctx context.Context, account domain.AccountID, id domain.DiceSetID) error
	UnchooseDiceSet(ctx context.Context, account domain.AccountID) error
	ChosenDiceSet(ctx context.Context, account domain.AccountID) (*domain.DiceSet, error)
}

const maxDiceSetName = 60

var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validDesign reports whether a design dresses only dice there are, in patterns and colours there are,
// with any picture placed on the sheet and not off it.
func validDesign(d domain.DiceDesign) bool {
	for die, look := range d.Dice {
		if !slices.Contains(domain.DieTypes(), die) || !slices.Contains(domain.DicePatterns(), look.Pattern) ||
			!hexColour.MatchString(look.Body) || !hexColour.MatchString(look.Numbers) {
			return false
		}
		if p := look.Image; p != nil && (p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 || p.Scale < 0.1 || p.Scale > 10 || p.Rotation < -360 || p.Rotation > 360) {
			return false
		}
	}
	return true
}

// cleanSetName trims a Dice Set's name and checks its length as the screens count it.
func cleanSetName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	return name, name != "" && len(utf16.Encode([]rune(name))) <= maxDiceSetName
}

// DiceSets lists the caller's Dice Sets, their own and their copies, and the one they roll with.
func (s *Service) DiceSets(ctx context.Context, subject string) ([]domain.DiceSet, *domain.DiceSetID, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, nil, err
	}
	sets, err := s.Repo.DiceSetsOf(ctx, me.ID)
	if err != nil {
		return nil, nil, err
	}
	chosen, err := s.Repo.ChosenDiceSet(ctx, me.ID)
	if err != nil || chosen == nil {
		return sets, nil, err
	}
	return sets, &chosen.ID, nil
}

// SharedDiceSets lists the sets of others the caller may take a copy of: their Friends' shared sets,
// and those shared with everyone.
func (s *Service) SharedDiceSets(ctx context.Context, subject string) ([]domain.DiceSet, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	return s.Repo.DiceSetsSharedWith(ctx, me.ID)
}

// CreateDiceSet makes a private Dice Set.
func (s *Service) CreateDiceSet(ctx context.Context, subject, name string, design domain.DiceDesign) (domain.DiceSet, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return domain.DiceSet{}, err
	}
	name, ok := cleanSetName(name)
	if !ok || !validDesign(design) {
		return domain.DiceSet{}, domain.ErrInvalid
	}
	now := s.Now()
	d := domain.DiceSet{
		ID: uuid.New(), Owner: me.ID, Name: name, Design: design, Image: nil, Sharing: domain.SharingPrivate, Review: domain.ReviewNone,
		CopiedFrom: nil, MadeBy: me.Username, CreatedAt: now, UpdatedAt: now,
	}
	return d, s.Repo.InsertDiceSet(ctx, d)
}

// own checks that a Dice Set is the caller's; anyone else's is not found. A copy cannot be changed, so
// asking to change one is a conflict.
func (s *Service) own(ctx context.Context, subject string, id domain.DiceSetID, change bool) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	d, err := s.Repo.DiceSet(ctx, id)
	if err != nil {
		return err
	}
	if d.Owner != me.ID {
		return domain.ErrNotFound
	}
	if change && d.Copy() {
		return domain.ErrConflict
	}
	return nil
}

// EditDiceSet changes a set's name and looks.
func (s *Service) EditDiceSet(ctx context.Context, subject string, id domain.DiceSetID, name string, design domain.DiceDesign) (domain.DiceSet, error) {
	if err := s.own(ctx, subject, id, true); err != nil {
		return domain.DiceSet{}, err
	}
	name, ok := cleanSetName(name)
	if !ok || !validDesign(design) {
		return domain.DiceSet{}, domain.ErrInvalid
	}
	if err := s.Repo.UpdateDiceSet(ctx, id, name, design, s.Now()); err != nil {
		return domain.DiceSet{}, err
	}
	return s.Repo.DiceSet(ctx, id)
}

// ShareDiceSet sets who a set is shared with. Sharing a set that carries a picture with everyone puts
// it before the Admins; until one approves it only its owner's Friends see it.
func (s *Service) ShareDiceSet(ctx context.Context, subject string, id domain.DiceSetID, sharing string) (domain.DiceSet, error) {
	if err := s.own(ctx, subject, id, true); err != nil {
		return domain.DiceSet{}, err
	}
	if !slices.Contains([]string{domain.SharingPrivate, domain.SharingFriends, domain.SharingEveryone}, sharing) {
		return domain.DiceSet{}, domain.ErrInvalid
	}
	if err := s.Repo.SetDiceSetSharing(ctx, id, sharing, s.Now()); err != nil {
		return domain.DiceSet{}, err
	}
	return s.Repo.DiceSet(ctx, id)
}

// visible reports whether an Account may see someone else's set: everyone may see a public one, and
// the owner's Friends one shared with Friends or with everyone.
func (s *Service) visible(ctx context.Context, me domain.AccountID, d domain.DiceSet) (bool, error) {
	if d.Copy() || d.Sharing == domain.SharingPrivate {
		return false, nil
	}
	if d.Public() {
		return true, nil
	}
	return s.Repo.AreFriends(ctx, me, d.Owner)
}

// CopyDiceSet takes a copy of a set shared with the caller. The copy is theirs to roll with and to
// delete, never to edit; it stays as it was when the original changes or stops being shared. The store
// keeps one copy of a set per Account, so a second is a conflict.
func (s *Service) CopyDiceSet(ctx context.Context, subject string, id domain.DiceSetID) (domain.DiceSet, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return domain.DiceSet{}, err
	}
	d, err := s.Repo.DiceSet(ctx, id)
	if err != nil {
		return domain.DiceSet{}, err
	}
	if ok, err := s.visible(ctx, me.ID, d); err != nil || !ok || d.Owner == me.ID {
		return domain.DiceSet{}, firstErr(err, domain.ErrNotFound)
	}
	now := s.Now()
	c := domain.DiceSet{
		ID: uuid.New(), Owner: me.ID, Name: d.Name, Design: d.Design, Image: d.Image, Sharing: domain.SharingPrivate, Review: domain.ReviewNone,
		CopiedFrom: &d.ID, MadeBy: d.MadeBy, CreatedAt: now, UpdatedAt: now,
	}
	// The copy carries the picture an Admin approved, so it carries the approval.
	if d.Cleared() {
		c.Review = domain.ReviewApproved
	}
	return c, s.Repo.InsertDiceSet(ctx, c)
}

// DeleteDiceSet removes one of the caller's sets, a copy too.
func (s *Service) DeleteDiceSet(ctx context.Context, subject string, id domain.DiceSetID) error {
	if err := s.own(ctx, subject, id, false); err != nil {
		return err
	}
	return s.Repo.DeleteDiceSet(ctx, id)
}

// SetDiceSetPicture puts an uploaded picture on a set. A set shared with everyone goes back before the
// Admins: the picture they approved is not the one it carries now.
func (s *Service) SetDiceSetPicture(ctx context.Context, subject string, id domain.DiceSetID, data []byte) (domain.DiceSet, error) {
	if err := s.own(ctx, subject, id, true); err != nil {
		return domain.DiceSet{}, err
	}
	contentType, ext, ok := picture.Sniff(data)
	if !ok || len(data) > picture.MaxBytes {
		return domain.DiceSet{}, domain.ErrInvalid
	}
	img := &domain.Picture{Key: picture.Key(data, ext), Type: contentType}
	if err := s.Blobs.Put(ctx, img.Key, img.Type, data); err != nil {
		return domain.DiceSet{}, err
	}
	if err := s.Repo.SetDiceSetImage(ctx, id, img, s.Now()); err != nil {
		return domain.DiceSet{}, err
	}
	return s.Repo.DiceSet(ctx, id)
}

// ClearDiceSetPicture takes the uploaded picture off a set; with nothing left to check, it needs no review.
func (s *Service) ClearDiceSetPicture(ctx context.Context, subject string, id domain.DiceSetID) (domain.DiceSet, error) {
	if err := s.own(ctx, subject, id, true); err != nil {
		return domain.DiceSet{}, err
	}
	if err := s.Repo.SetDiceSetImage(ctx, id, nil, s.Now()); err != nil {
		return domain.DiceSet{}, err
	}
	return s.Repo.DiceSet(ctx, id)
}

// DiceSetPicture reads a set's uploaded picture for its owner, for anyone the set is shared with, and
// for an Admin, who has to see it to review it. A picture an Admin approved is anyone's to see. Asked
// for by its digest, it is that picture or not found: what an Admin is shown is what they decide on.
func (s *Service) DiceSetPicture(ctx context.Context, subject string, id domain.DiceSetID, admin bool, digest string) (domain.Picture, []byte, error) {
	d, err := s.Repo.DiceSet(ctx, id)
	if err != nil {
		return domain.Picture{}, nil, err
	}
	if !admin {
		me, err := s.me(ctx, subject)
		if err != nil {
			return domain.Picture{}, nil, err
		}
		if seen, err := s.visible(ctx, me.ID, d); err != nil || (d.Owner != me.ID && !seen && !d.Cleared()) {
			return domain.Picture{}, nil, firstErr(err, domain.ErrNotFound)
		}
	}
	if d.Image == nil || (digest != "" && d.Image.Digest() != digest) {
		return domain.Picture{}, nil, domain.ErrNotFound
	}
	data, err := s.Blobs.Get(ctx, d.Image.Key)
	return *d.Image, data, err
}

// ChooseDiceSet sets the Dice Set the caller rolls with: one of their own, or none for the plain dice.
func (s *Service) ChooseDiceSet(ctx context.Context, subject string, id *domain.DiceSetID) error {
	me, err := s.me(ctx, subject)
	if err != nil {
		return err
	}
	if id == nil {
		return s.Repo.UnchooseDiceSet(ctx, me.ID)
	}
	if err := s.own(ctx, subject, *id, false); err != nil {
		return err
	}
	return s.Repo.ChooseDiceSet(ctx, me.ID, *id)
}

// ChosenDiceSet reads the Dice Set a subject rolls with, or nil for the plain dice.
func (s *Service) ChosenDiceSet(ctx context.Context, subject string) (*domain.DiceSet, error) {
	me, err := s.me(ctx, subject)
	if err != nil {
		return nil, err
	}
	return s.Repo.ChosenDiceSet(ctx, me.ID)
}

// OwnsDiceSet reports whether a Dice Set is the caller's own.
func (s *Service) OwnsDiceSet(ctx context.Context, subject string, d domain.DiceSet) bool {
	me, err := s.me(ctx, subject)
	return err == nil && me.ID == d.Owner
}

// DiceSetsToReview lists the sets that wait for an Admin, oldest first.
func (s *Service) DiceSetsToReview(ctx context.Context) ([]domain.DiceSet, error) {
	return s.Repo.DiceSetsAwaitingReview(ctx)
}

// ReviewDiceSet approves or rejects a set that waits. The decision is for the picture the Admin looked
// at, named by the whole hash of its content: when the owner has put another on the set since, or the
// set no longer waits, it is a conflict.
func (s *Service) ReviewDiceSet(ctx context.Context, id domain.DiceSetID, approve bool, seen string) (domain.DiceSet, error) {
	d, err := s.Repo.DiceSet(ctx, id)
	if err != nil {
		return domain.DiceSet{}, err
	}
	if d.Review != domain.ReviewPending || d.Image == nil || seen == "" || d.Image.Digest() != seen {
		return domain.DiceSet{}, domain.ErrConflict
	}
	review := domain.ReviewRejected
	if approve {
		review = domain.ReviewApproved
	}
	if err := s.Repo.SetDiceSetReview(ctx, id, review, d.Image.Key, s.Now()); err != nil {
		return domain.DiceSet{}, err
	}
	return s.Repo.DiceSet(ctx, id)
}
