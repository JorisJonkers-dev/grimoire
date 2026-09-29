package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/auth"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/oas"
)

func TestForwardAuthRejectsEmptySubject(t *testing.T) {
	t.Parallel()
	_, err := auth.ForwardAuth{}.HandleForwardAuth(context.Background(), oas.GetStatusOperation, oas.ForwardAuth{})
	if !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("err = %v", err)
	}
}

func TestForwardAuthStoresIdentity(t *testing.T) {
	t.Parallel()
	ctx, err := auth.ForwardAuth{}.HandleForwardAuth(context.Background(), oas.GetStatusOperation, oas.ForwardAuth{APIKey: "u-1"})
	if err != nil {
		t.Fatal(err)
	}
	id, ok := auth.FromContext(ctx)
	if !ok || id.Subject != "u-1" {
		t.Fatalf("identity = %+v, %v", id, ok)
	}
}

func TestFromContextWithoutIdentity(t *testing.T) {
	t.Parallel()
	if _, ok := auth.FromContext(context.Background()); ok {
		t.Fatal("expected no identity")
	}
}
