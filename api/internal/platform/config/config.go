// Package config reads the process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"strconv"
)

// Config is everything the binary needs to start.
type Config struct {
	Addr        string
	DatabaseURL string
	DevSubject  string
	AutoMigrate bool
	AutoImport  bool
	RateLimit   int
	AssetDir    string
	// OAuthIssuer is the authorization server MCP agents sign in with; empty leaves discovery out.
	OAuthIssuer string
	S3          *S3
}

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
		Addr:        getenv("GRIMOIRE_ADDR"),
		DatabaseURL: getenv("GRIMOIRE_DATABASE_URL"),
		DevSubject:  getenv("GRIMOIRE_DEV_SUBJECT"),
		AutoMigrate: getenv("GRIMOIRE_AUTO_MIGRATE") == "true",
		AutoImport:  getenv("GRIMOIRE_AUTO_IMPORT") == "true",
		RateLimit:   600,
		AssetDir:    getenv("GRIMOIRE_ASSET_DIR"),
		OAuthIssuer: getenv("GRIMOIRE_OAUTH_ISSUER"),
	}
	if c.AssetDir == "" {
		c.AssetDir = "/tmp/grimoire-assets"
	}
	if endpoint := getenv("GRIMOIRE_S3_ENDPOINT"); endpoint != "" {
		c.S3 = &S3{
			Endpoint: endpoint, Region: getenv("GRIMOIRE_S3_REGION"), Bucket: getenv("GRIMOIRE_S3_BUCKET"),
			AccessKey: getenv("GRIMOIRE_S3_ACCESS_KEY_ID"), SecretKey: getenv("GRIMOIRE_S3_SECRET_ACCESS_KEY"),
		}
		if c.S3.Bucket == "" || c.S3.AccessKey == "" || c.S3.SecretKey == "" {
			return Config{}, ErrIncompleteS3
		}
		if c.S3.Region == "" {
			c.S3.Region = "garage"
		}
	}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if c.DatabaseURL == "" {
		return Config{}, ErrMissingDatabaseURL
	}
	if v := getenv("GRIMOIRE_RATE_LIMIT_PER_MINUTE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("config: GRIMOIRE_RATE_LIMIT_PER_MINUTE must be a positive integer, got %q", v)
		}
		c.RateLimit = n
	}
	return c, nil
}
