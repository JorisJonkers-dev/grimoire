package pgstore_test

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
)

// upTo is the embedded migrations up to and including one version.
func upTo(t *testing.T, last string) fs.FS {
	t.Helper()
	out := fstest.MapFS{}
	entries, err := fs.ReadDir(db.Migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() <= last+"~" {
			body, err := fs.ReadFile(db.Migrations, "migrations/"+e.Name())
			if err != nil {
				t.Fatal(err)
			}
			out["migrations/"+e.Name()] = &fstest.MapFile{Data: body}
		}
	}
	return out
}

// Existing Members become Accounts on their subject, and each existing Character splits into a
// Character owned by that Account and its one Campaign Character, sharing an id.
func TestMembersAndCharactersMoveToAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	url := pgtest.EmptyURL(t)
	if err := pg.MigrateFS(ctx, url, upTo(t, "00095")); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, q := range []string{
		`INSERT INTO campaign.campaigns (id, name, created_by) VALUES ('0190c7a8-0000-7000-8000-0000000000c1', 'Morvain', 'estate-1')`,
		`INSERT INTO campaign.members (id, campaign_id, auth_subject, display_name, role) VALUES
			('0190c7a8-0000-7000-8000-0000000000d1', '0190c7a8-0000-7000-8000-0000000000c1', 'estate-1', '  Tamsin of the Very Long Name That Goes On  ', 'dm'),
			('0190c7a8-0000-7000-8000-0000000000d2', '0190c7a8-0000-7000-8000-0000000000c1', 'estate-2', 'Odile', 'player')`,
		`INSERT INTO campaign.characters (id, campaign_id, owner_member_id, name, ruleset, species_slug, class_slug, background_slug, ability_method, hp_max, hp_current, level)
			VALUES ('0190c7a8-0000-7000-8000-0000000000e1', '0190c7a8-0000-7000-8000-0000000000c1', '0190c7a8-0000-7000-8000-0000000000d2', 'Aria', 'srd-2024', 'human', 'fighter', 'soldier', 'standard-array', 30, 22, 3)`,
	} {
		if _, err := pool.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := pg.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	var subject, username, nickname string
	var email *string
	if err := pool.QueryRow(ctx, `SELECT subject, username, nickname, email FROM identity.accounts WHERE subject = 'estate-1'`).Scan(&subject, &username, &nickname, &email); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(username, "member-") || len(username) != 17 || nickname != "Tamsin of the Very Long Name That Goes O" || email != nil {
		t.Fatalf("account = %q %q %v", username, nickname, email)
	}
	var owner, name string
	var level int
	if err := pool.QueryRow(ctx, `SELECT a.owner_subject, a.name, c.level FROM campaign.account_characters a
		JOIN campaign.characters c ON c.character_id = a.id WHERE a.id = '0190c7a8-0000-7000-8000-0000000000e1'`).Scan(&owner, &name, &level); err != nil {
		t.Fatal(err)
	}
	if owner != "estate-2" || name != "Aria" || level != 3 {
		t.Fatalf("character = %q %q %d", owner, name, level)
	}
	var accounts int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM identity.accounts`).Scan(&accounts); err != nil || accounts != 2 {
		t.Fatalf("accounts = %d %v", accounts, err)
	}
}
