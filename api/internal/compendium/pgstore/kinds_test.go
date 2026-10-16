package pgstore

import (
	"testing"
)

// A kind without a store is an error, never a silent skip; only a kind that holds Parts nests them.
func TestKindsWithoutAStoreAreRefused(t *testing.T) {
	t.Parallel()
	if _, err := rowsOf("heal"); err == nil {
		t.Fatal("a kind without a store found one")
	}
	if _, ok := holdingOf("heal"); ok {
		t.Fatal("a kind without a store holds Parts")
	}
	if _, ok := holdingOf("bonus_die"); ok {
		t.Fatal("a bonus die holds no Parts")
	}
	if _, ok := holdingOf("branch"); !ok {
		t.Fatal("a branch holds the Parts it adds")
	}
}
