package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/social/domain"
)

type chosen struct {
	set *domain.DiceSet
	err error
}

func (c chosen) ChosenDiceSet(context.Context, string) (*domain.DiceSet, error) { return c.set, c.err }

// A shared roll wears its roller's Dice Set; the picture goes along only once an Admin approved it.
func TestTheLookOfASharedRoll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	set := domain.DiceSet{
		ID: uuid.MustParse("0190c7a8-0000-7000-8000-000000000041"), Sharing: domain.SharingEveryone, Review: domain.ReviewPending,
		Image: &domain.Picture{Key: "sha256/0123456789abcdef.png", Type: "image/png"},
		Design: domain.DiceDesign{Dice: map[string]domain.DieLook{
			"d20": {Pattern: "marble", Body: "#7a1f1a", Numbers: "#f3d27a", Image: &domain.DiePlacement{X: 0.25, Y: 0.75, Scale: 2, Rotation: -45}},
			"d6":  {Pattern: "plain", Body: "#000000", Numbers: "#ffffff", Image: nil},
		}},
	}
	dice := `"dice":{"d20":{"pattern":"marble","body":"#7a1f1a","numbers":"#f3d27a","image":{"x":0.25,"y":0.75,"scale":2,"rotation":-45}},"d6":{"pattern":"plain","body":"#000000","numbers":"#ffffff"}}`
	look := func(c chosen) string {
		raw, _ := json.Marshal(httpapi.DiceLooks{Sets: c}.DiceLook(ctx, "aria"))
		return string(raw)
	}
	if got := look(chosen{set: &set}); got != `{`+dice+`}` {
		t.Fatalf("a set that waits = %s", got)
	}
	set.Review = domain.ReviewApproved
	if got := look(chosen{set: &set}); got != `{`+dice+`,"imageUrl":"/api/v1/dice-sets/0190c7a8-0000-7000-8000-000000000041/image?v=0123456789ab"}` {
		t.Fatalf("an approved set = %s", got)
	}
	for name, c := range map[string]chosen{"the plain dice": {}, "a store that fails": {set: &set, err: errors.New("down")}} {
		if got := look(c); got != "null" {
			t.Fatalf("%s = %s", name, got)
		}
	}
}
