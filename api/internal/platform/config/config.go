// Package config reads the process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Config is everything the binary needs to start.
type Config struct {
	Addr        string
	DatabaseURL string
	DevSubject  string
	AutoMigrate bool
	AutoImport  bool
	RateLimit   int
	// ProxyHops is how many reverse proxies stand in front of the server (GRIMOIRE_TRUSTED_PROXY_HOPS).
	// Each is trusted to append the address it saw to X-Forwarded-For, so that anonymous callers are
	// rate limited one by one and not as the proxy. Leave it 0 where the server can be reached directly.
	ProxyHops int
	AssetDir  string
	// OAuthIssuer is the authorization server MCP agents sign in with; empty leaves discovery out.
	OAuthIssuer string
	S3          *S3
	Push        *Push
	// TrustForwardAuth keeps the platform's identity header; once Grimoire runs its own sessions only,
	// set GRIMOIRE_TRUST_FORWARD_AUTH=false so no client can claim an identity itself.
	TrustForwardAuth bool
	// AdminSubjects act as Admins without an Account flag, to send the first invite.
	AdminSubjects []string
	// BaseURL is where links in emails point.
	BaseURL string
	SMTP    *SMTP
	OIDC    *OIDC
	// Changelog is the release-please changelog Release Notes are drafted from.
	Changelog string
}

// OIDC is the external login an Account can sign in with; nil offers none. A login needs GrantRole
// among the roles in RolesClaim, and AdminRole makes its Account an Admin.
type OIDC struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	Name         string
	GrantRole    string
	AdminRole    string
	RolesClaim   string
}

// ErrIncompleteOIDC is returned when an OIDC issuer is set without its client or the base URL its
// callback lives under.
var ErrIncompleteOIDC = errors.New("config: GRIMOIRE_OIDC_ISSUER needs GRIMOIRE_OIDC_CLIENT_ID and GRIMOIRE_BASE_URL")

// SMTP is the server Grimoire sends email through; nil logs emails instead.
type SMTP struct {
	Addr     string
	Username string
	Password string
	From     string
}

// ErrIncompleteSMTP is returned when an SMTP server is set without its sender, or the reverse.
var ErrIncompleteSMTP = errors.New("config: GRIMOIRE_SMTP_ADDR and GRIMOIRE_SMTP_FROM go together")

// Push holds the VAPID keys Web Push is signed with; nil sends no notifications.
type Push struct {
	PublicKey  string
	PrivateKey string
	Contact    string
}

// ErrIncompletePush is returned when only part of the Web Push settings is given.
var ErrIncompletePush = errors.New("config: GRIMOIRE_VAPID_PUBLIC_KEY, GRIMOIRE_VAPID_PRIVATE_KEY and GRIMOIRE_VAPID_CONTACT go together")

// S3 locates the asset bucket; nil keeps assets on local disk under AssetDir.
type S3 struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
}

// ErrIncompleteS3 is returned when an S3 endpoint is set without its bucket or keys.
var ErrIncompleteS3 = errors.New("config: GRIMOIRE_S3_ENDPOINT needs GRIMOIRE_S3_BUCKET, GRIMOIRE_S3_ACCESS_KEY_ID and GRIMOIRE_S3_SECRET_ACCESS_KEY")

// ErrMissingDatabaseURL is returned when GRIMOIRE_DATABASE_URL is unset.
var ErrMissingDatabaseURL = errors.New("config: GRIMOIRE_DATABASE_URL is required")

// Load reads Config through getenv, applying defaults.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:             getenv("GRIMOIRE_ADDR"),
		DatabaseURL:      getenv("GRIMOIRE_DATABASE_URL"),
		DevSubject:       getenv("GRIMOIRE_DEV_SUBJECT"),
		AutoMigrate:      getenv("GRIMOIRE_AUTO_MIGRATE") == "true",
		AutoImport:       getenv("GRIMOIRE_AUTO_IMPORT") == "true",
		RateLimit:        600,
		AssetDir:         getenv("GRIMOIRE_ASSET_DIR"),
		OAuthIssuer:      getenv("GRIMOIRE_OAUTH_ISSUER"),
		TrustForwardAuth: getenv("GRIMOIRE_TRUST_FORWARD_AUTH") != "false",
		BaseURL:          getenv("GRIMOIRE_BASE_URL"),
		Changelog:        or(getenv("GRIMOIRE_CHANGELOG"), "CHANGELOG.md"),
	}
	for _, s := range strings.Split(getenv("GRIMOIRE_ADMIN_SUBJECTS"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			c.AdminSubjects = append(c.AdminSubjects, s)
		}
	}
	if err := c.loadSMTP(getenv); err != nil {
		return Config{}, err
	}
	if err := c.loadOIDC(getenv); err != nil {
		return Config{}, err
	}
	if c.AssetDir == "" {
		c.AssetDir = "/tmp/grimoire-assets"
	}
	if err := c.loadS3(getenv); err != nil {
		return Config{}, err
	}
	if err := c.loadPush(getenv); err != nil {
		return Config{}, err
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.DatabaseURL == "" {
		return Config{}, ErrMissingDatabaseURL
	}
	if err := c.loadLimits(getenv); err != nil {
		return Config{}, err
	}
	return c, nil
}

// loadLimits reads the rate limit and how many reverse proxies to trust for it.
func (c *Config) loadLimits(getenv func(string) string) error {
	if v := getenv("GRIMOIRE_RATE_LIMIT_PER_MINUTE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return fmt.Errorf("config: GRIMOIRE_RATE_LIMIT_PER_MINUTE must be a positive integer, got %q", v)
		}
		c.RateLimit = n
	}
	if v := getenv("GRIMOIRE_TRUSTED_PROXY_HOPS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > maxProxyHops {
			return fmt.Errorf("config: GRIMOIRE_TRUSTED_PROXY_HOPS must be an integer from 0 to %d, got %q", maxProxyHops, v)
		}
		c.ProxyHops = n
	}
	return nil
}

// maxProxyHops is the longest chain of reverse proxies the server can be told to trust.
const maxProxyHops = 8

func (c *Config) loadS3(getenv func(string) string) error {
	endpoint := getenv("GRIMOIRE_S3_ENDPOINT")
	if endpoint == "" {
		return nil
	}
	s := &S3{
		Endpoint: endpoint, Region: getenv("GRIMOIRE_S3_REGION"), Bucket: getenv("GRIMOIRE_S3_BUCKET"),
		AccessKey: getenv("GRIMOIRE_S3_ACCESS_KEY_ID"), SecretKey: getenv("GRIMOIRE_S3_SECRET_ACCESS_KEY"),
	}
	if s.Bucket == "" || s.AccessKey == "" || s.SecretKey == "" {
		return ErrIncompleteS3
	}
	if s.Region == "" {
		s.Region = "garage"
	}
	c.S3 = s
	return nil
}

func (c *Config) loadSMTP(getenv func(string) string) error {
	s := SMTP{Addr: getenv("GRIMOIRE_SMTP_ADDR"), Username: getenv("GRIMOIRE_SMTP_USERNAME"), Password: getenv("GRIMOIRE_SMTP_PASSWORD"), From: getenv("GRIMOIRE_SMTP_FROM")}
	switch {
	case s.Addr == "" && s.From == "":
		return nil
	case s.Addr == "" || s.From == "":
		return ErrIncompleteSMTP
	}
	c.SMTP = &s
	return nil
}

func (c *Config) loadOIDC(getenv func(string) string) error {
	issuer := getenv("GRIMOIRE_OIDC_ISSUER")
	if issuer == "" {
		return nil
	}
	o := OIDC{
		Issuer: issuer, ClientID: getenv("GRIMOIRE_OIDC_CLIENT_ID"), ClientSecret: getenv("GRIMOIRE_OIDC_CLIENT_SECRET"),
		Name: or(getenv("GRIMOIRE_OIDC_NAME"), hostOf(issuer)), GrantRole: or(getenv("GRIMOIRE_OIDC_GRANT_ROLE"), "SERVICE_GRIMOIRE"),
		AdminRole: or(getenv("GRIMOIRE_OIDC_ADMIN_ROLE"), "ROLE_ADMIN"), RolesClaim: or(getenv("GRIMOIRE_OIDC_ROLES_CLAIM"), "roles"),
	}
	if o.ClientID == "" || c.BaseURL == "" {
		return ErrIncompleteOIDC
	}
	c.OIDC = &o
	return nil
}

func or(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// hostOf is an issuer URL without its scheme and path.
func hostOf(issuer string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")
	host, _, _ = strings.Cut(host, "/")
	return host
}

func (c *Config) loadPush(getenv func(string) string) error {
	p := Push{PublicKey: getenv("GRIMOIRE_VAPID_PUBLIC_KEY"), PrivateKey: getenv("GRIMOIRE_VAPID_PRIVATE_KEY"), Contact: getenv("GRIMOIRE_VAPID_CONTACT")}
	switch {
	case p == (Push{}):
		return nil
	case p.PublicKey == "" || p.PrivateKey == "" || p.Contact == "":
		return ErrIncompletePush
	}
	c.Push = &p
	return nil
}
