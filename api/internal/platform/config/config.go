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
}

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
