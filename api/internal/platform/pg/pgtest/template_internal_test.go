package pgtest

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// On a server that outlives the run, one template per schema serves every test binary and every run:
// whoever needs it first makes it, the rest find it there.
func TestOneTemplatePerSchemaOnAServerThatStays(t *testing.T) {
	t.Parallel()
	admin := os.Getenv("GRIMOIRE_TEST_POSTGRES_URL")
	if admin == "" {
		t.Skip("needs GRIMOIRE_TEST_POSTGRES_URL")
	}
	ctx := context.Background()
	names := make([]string, 4)
	var wg sync.WaitGroup
	for i := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name, err := sharedTemplate(ctx, admin)
			if err != nil {
				t.Errorf("template: %v", err)
			}
			names[i] = name
		}()
	}
	wg.Wait()
	for _, n := range names {
		if n != names[0] || n != "grimoire_template_"+schemaDigest() || len(schemaDigest()) != 12 {
			t.Fatalf("templates = %v", names)
		}
	}
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var whole, building int
	if err := conn.QueryRow(ctx, "SELECT count(*) FILTER (WHERE datname = $1), count(*) FILTER (WHERE datname = $1 || '_building') FROM pg_database", names[0]).Scan(&whole, &building); err != nil || whole != 1 || building != 0 {
		t.Fatalf("databases named for the template: %d whole, %d building, %v", whole, building, err)
	}
	// It is the migrated schema: a database made from it has the tables.
	var tables int
	tconn, err := pgx.Connect(ctx, withDatabase(admin, names[0]))
	if err != nil {
		t.Fatal(err)
	}
	if err := tconn.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'play'").Scan(&tables); err != nil || tables < 10 {
		t.Fatalf("tables in the template: %d, %v", tables, err)
	}
	_ = tconn.Close(ctx)
}
