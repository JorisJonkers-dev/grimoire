package config_test

import (
	"errors"
	"strings"
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
	if c.Addr != ":8080" || c.RateLimit != 600 || c.AutoMigrate || c.DevSubject != "" || !c.TrustForwardAuth || c.SMTP != nil || c.AdminSubjects != nil || c.OIDC != nil || c.Changelog != "CHANGELOG.md" {
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
		"GRIMOIRE_CHANGELOG":             "/CHANGELOG.md",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9000" || c.RateLimit != 42 || !c.AutoMigrate || !c.AutoImport || c.DevSubject != "dev" || c.OAuthIssuer != "https://auth.example" || *c.Push != (config.Push{PublicKey: "pub", PrivateKey: "priv", Contact: "mailto:dm@example.com"}) {
		t.Fatalf("overrides not applied: %+v", c)
	}
	if c.TrustForwardAuth || len(c.AdminSubjects) != 2 || c.AdminSubjects[1] != "root" || c.BaseURL != "https://grimoire.example" || c.SMTP.Addr != "smtp.example:587" || c.Changelog != "/CHANGELOG.md" {
		t.Fatalf("identity settings = %+v", c)
	}
}

func TestLoadOIDC(t *testing.T) {
	t.Parallel()
	base := map[string]string{"GRIMOIRE_DATABASE_URL": "postgres://x", "GRIMOIRE_OIDC_ISSUER": "https://auth.example/realm"}
	if _, err := config.Load(env(base)); !errors.Is(err, config.ErrIncompleteOIDC) {
		t.Fatalf("an issuer alone = %v", err)
	}
	base["GRIMOIRE_OIDC_CLIENT_ID"] = "grimoire"
	if _, err := config.Load(env(base)); !errors.Is(err, config.ErrIncompleteOIDC) {
		t.Fatalf("without a base URL = %v", err)
	}
	base["GRIMOIRE_BASE_URL"] = "https://grimoire.example"
	c, err := config.Load(env(base))
	if err != nil {
		t.Fatal(err)
	}
	want := config.OIDC{Issuer: "https://auth.example/realm", ClientID: "grimoire", ClientSecret: "", Name: "auth.example", GrantRole: "SERVICE_GRIMOIRE", AdminRole: "ROLE_ADMIN", RolesClaim: "roles"}
	if *c.OIDC != want {
		t.Fatalf("defaults = %+v", *c.OIDC)
	}
	for k, v := range map[string]string{
		"GRIMOIRE_OIDC_CLIENT_SECRET": "s", "GRIMOIRE_OIDC_NAME": "example.org", "GRIMOIRE_OIDC_GRANT_ROLE": "play",
		"GRIMOIRE_OIDC_ADMIN_ROLE": "boss", "GRIMOIRE_OIDC_ROLES_CLAIM": "groups",
	} {
		base[k] = v
	}
	if c, _ = config.Load(env(base)); *c.OIDC != (config.OIDC{Issuer: "https://auth.example/realm", ClientID: "grimoire", ClientSecret: "s", Name: "example.org", GrantRole: "play", AdminRole: "boss", RolesClaim: "groups"}) {
		t.Fatalf("overrides = %+v", *c.OIDC)
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

// The server trusts as many reverse proxies as it is told stand in front of it, and none by default.
func TestLoadProxyHops(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]int{"": 0, "0": 0, "1": 1, "2": 2, "8": 8} {
		c, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "x", "GRIMOIRE_TRUSTED_PROXY_HOPS": v}))
		if err != nil || c.ProxyHops != want {
			t.Fatalf("%q: %d %v", v, c.ProxyHops, err)
		}
	}
	for _, v := range []string{"abc", "-1", "9", "1.5"} {
		if _, err := config.Load(env(map[string]string{"GRIMOIRE_DATABASE_URL": "x", "GRIMOIRE_TRUSTED_PROXY_HOPS": v})); err == nil || !strings.Contains(err.Error(), "GRIMOIRE_TRUSTED_PROXY_HOPS") {
			t.Fatalf("accepted %q: %v", v, err)
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
