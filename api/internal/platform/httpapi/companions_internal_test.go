package httpapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/campaign/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

// Nobody keeps Companions without saying who they are.
func TestCompanionHandlersNeedAnIdentity(t *testing.T) {
	t.Parallel()
	ctx, h := context.Background(), &Handler{}
	calls := map[string]func() (any, error){
		"list":   func() (any, error) { return h.ListCompanions(ctx, oas.ListCompanionsParams{}) },
		"create": func() (any, error) { return h.CreateCompanion(ctx, &oas.CompanionInput{}, oas.CreateCompanionParams{}) },
		"update": func() (any, error) { return h.UpdateCompanion(ctx, &oas.CompanionInput{}, oas.UpdateCompanionParams{}) },
		"delete": func() (any, error) { return h.DeleteCompanion(ctx, oas.DeleteCompanionParams{}) },
	}
	for name, call := range calls {
		res, err := call()
		p, ok := res.(*oas.ProblemStatusCodeWithHeaders)
		if err != nil || !ok || p.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s = %#v, %v", name, res, err)
		}
	}
}

// A Companion goes out with who runs it and the hit points it kept, when it has either.
func TestACompanionOnTheWire(t *testing.T) {
	t.Parallel()
	who, hp := domain.MemberID(uuid.New()), 4
	out := companionOut(domain.Companion{Name: "Fang", Kind: domain.KindCompanion, MonsterSlug: "wolf", Controller: &who, HP: &hp})
	if id, ok := out.ControllerId.Get(); !ok || uuid.UUID(id) != uuid.UUID(who) {
		t.Fatalf("controller = %v", out.ControllerId)
	}
	if got, ok := out.Hp.Get(); !ok || got != 4 {
		t.Fatalf("hit points = %v", out.Hp)
	}
	if bare := companionOut(domain.Companion{Name: "Bors", Kind: domain.KindHireling, MonsterSlug: "goblin"}); bare.ControllerId.Set || bare.Hp.Set {
		t.Fatalf("a Companion the DM runs, never hurt = %+v", bare)
	}
}
