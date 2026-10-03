package httpapi

import (
	"context"

	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

// ChosenDice reads the Dice Set a subject rolls with.
type ChosenDice interface {
	ChosenDiceSet(ctx context.Context, subject string) (*domain.DiceSet, error)
}

// DiceLooks dresses the rolls live play shares in their roller's Dice Set.
type DiceLooks struct {
	Sets ChosenDice
}

// diceImageURL is where a Dice Set's picture is served, asked for by the hash of its content: the link
// shows that picture or nothing, and no cache can hand back another under the same address.
func diceImageURL(d domain.DiceSet) string {
	return "/api/v1/dice-sets/" + d.ID.String() + "/image?v=" + d.Image.Digest()
}

// DiceLook is the look of the Dice Set a subject rolls with, or nil for the plain dice. The picture is
// named only when an Admin approved it: until then it shows on its owner's screen alone.
func (l DiceLooks) DiceLook(ctx context.Context, subject string) *live.DiceLook {
	d, err := l.Sets.ChosenDiceSet(ctx, subject)
	if err != nil || d == nil {
		return nil
	}
	look := &live.DiceLook{Dice: make(map[string]live.DieLook, len(d.Design.Dice)), ImageURL: ""}
	for die, l := range d.Design.Dice {
		look.Dice[die] = live.DieLook{Pattern: l.Pattern, Body: l.Body, Numbers: l.Numbers, Image: (*live.DiePlacement)(l.Image)}
	}
	if d.Cleared() {
		look.ImageURL = diceImageURL(*d)
	}
	return look
}
