package config_test

import (
	"errors"
	"testing"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/config"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	c, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":8080" || c.RateLimit != 600 || c.AutoMigrate || c.DevSubject != "" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Parallel()
	c, err := config.Load(env(map[string]string{
		"GRIMOIRE_DATABASE_URL":          "postgres://x",
		"GRIMOIRE_ADDR":                  ":9000",
		"GRIMOIRE_DEV_SUBJECT":           "dev",
		"GRIMOIRE_AUTO_MIGRATE":          "true",
		"GRIMOIRE_RATE_LIMIT_PER_MINUTE": "42",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9000" || c.RateLimit != 42 || !c.AutoMigrate || c.DevSubject != "dev" {
		t.Fatalf("overrides not applied: %+v", c)
	}
}

func TestLoadRejectsMissingDatabase(t *testing.T) {
	t.Parallel()
	if _, err := config.Load(env(nil)); !errors.Is(err, config.ErrMissingDatabaseURL) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadRejectsBadRateLimit(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"abc", "0", "-3"} {
		_, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "x", "GRIMOIRE_RATE_LIMIT_PER_MINUTE": v}))
		if err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}
