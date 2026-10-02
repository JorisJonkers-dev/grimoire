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
	if c.Addr != ":8080" || c.RateLimit != 600 || c.AutoMigrate || c.DevSubject != "" || !c.TrustForwardAuth || c.SMTP != nil || c.AdminSubjects != nil {
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
		"GRIMOIRE_AUTO_IMPORT":           "true",
		"GRIMOIRE_RATE_LIMIT_PER_MINUTE": "42",
		"GRIMOIRE_OAUTH_ISSUER":          "https://auth.example",
		"GRIMOIRE_VAPID_PUBLIC_KEY":      "pub",
		"GRIMOIRE_VAPID_PRIVATE_KEY":     "priv",
		"GRIMOIRE_VAPID_CONTACT":         "mailto:dm@example.com",
		"GRIMOIRE_TRUST_FORWARD_AUTH":    "false",
		"GRIMOIRE_ADMIN_SUBJECTS":        " dev , root,,",
		"GRIMOIRE_BASE_URL":              "https://grimoire.example",
		"GRIMOIRE_SMTP_ADDR":             "smtp.example:587",
		"GRIMOIRE_SMTP_FROM":             "grimoire@example.com",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9000" || c.RateLimit != 42 || !c.AutoMigrate || !c.AutoImport || c.DevSubject != "dev" || c.OAuthIssuer != "https://auth.example" || *c.Push != (config.Push{PublicKey: "pub", PrivateKey: "priv", Contact: "mailto:dm@example.com"}) {
		t.Fatalf("overrides not applied: %+v", c)
	}
	if c.TrustForwardAuth || len(c.AdminSubjects) != 2 || c.AdminSubjects[1] != "root" || c.BaseURL != "https://grimoire.example" || c.SMTP.Addr != "smtp.example:587" {
		t.Fatalf("identity settings = %+v", c)
	}
}

func TestLoadRejectsHalfAnSMTPSetup(t *testing.T) {
	t.Parallel()
	if _, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "postgres://x", "GRIMOIRE_SMTP_ADDR": "smtp.example:587"})); !errors.Is(err, config.ErrIncompleteSMTP) {
		t.Fatalf("err = %v", err)
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

func TestLoadStorage(t *testing.T) {
	t.Parallel()
	base := map[string]string{"GRIMOIRE_DATABASE_URL": "postgres://x"}
	c, err := config.Load(env(base))
	if err != nil || c.AssetDir != "/tmp/grimoire-assets" || c.S3 != nil {
		t.Fatalf("default storage = %+v %v", c, err)
	}
	s3 := map[string]string{
		"GRIMOIRE_DATABASE_URL": "postgres://x", "GRIMOIRE_ASSET_DIR": "/tmp/a", "GRIMOIRE_S3_ENDPOINT": "http://garage:3900",
		"GRIMOIRE_S3_BUCKET": "grimoire-assets", "GRIMOIRE_S3_ACCESS_KEY_ID": "k", "GRIMOIRE_S3_SECRET_ACCESS_KEY": "s",
	}
	c, err = config.Load(env(s3))
	if err != nil || c.AssetDir != "/tmp/a" || c.S3.Region != "garage" || c.S3.Bucket != "grimoire-assets" {
		t.Fatalf("s3 = %+v %v", c.S3, err)
	}
	s3["GRIMOIRE_S3_REGION"] = "eu"
	if c, _ = config.Load(env(s3)); c.S3.Region != "eu" {
		t.Fatalf("region = %q", c.S3.Region)
	}
	delete(s3, "GRIMOIRE_S3_BUCKET")
	if _, err := config.Load(env(s3)); !errors.Is(err, config.ErrIncompleteS3) {
		t.Fatalf("incomplete s3 = %v", err)
	}
}

func TestLoadRejectsHalfAPushSetup(t *testing.T) {
	t.Parallel()
	if _, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "postgres://x", "GRIMOIRE_VAPID_PUBLIC_KEY": "pub"})); !errors.Is(err, config.ErrIncompletePush) {
		t.Fatalf("err = %v", err)
	}
	c, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "postgres://x"}))
	if err != nil || c.Push != nil {
		t.Fatalf("no push = %+v %v", c.Push, err)
	}
}
