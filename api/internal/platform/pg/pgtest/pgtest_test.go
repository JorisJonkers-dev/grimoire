package pgtest_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

// A test's database goes when the test does, so a server that outlives the run is not left full of them.
func TestADatabaseLastsAsLongAsItsTest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var migrated, empty string
	t.Run("a test", func(t *testing.T) {
		migrated, empty = pgtest.URL(t), pgtest.EmptyURL(t)
		for _, dsn := range []string{migrated, empty} {
			conn, err := pgx.Connect(ctx, dsn)
			if err != nil {
				t.Fatalf("during the test: %v", err)
			}
			// A connection the test forgot does not keep the database.
			t.Cleanup(func() { _ = conn.Close(ctx) })
		}
	})
	for _, dsn := range []string{migrated, empty} {
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			_ = conn.Close(ctx)
			t.Fatalf("%s is still there after its test", dsn)
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("after the test: %v", err)
		}
	}
}
