// Package config loads runtime settings from environment variables, with an
// optional .env file for local development. Settings are returned as a value
// rather than stored in package globals so callers and tests stay isolated.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every setting the tool reads from the environment. Google
// fields are only validated when a sync needs them, so
// parse-only runs work without any credentials.
type Config struct {
	CalendarID      string         // CALENDAR_ID (required for sync)
	CredentialsFile string         // GOOGLE_CREDENTIALS_FILE (required for sync)
	TokenFile       string         // GOOGLE_TOKEN_FILE (required for sync)
	SourceURL       string         // CALENDAR_URL, default https://www.rit.edu/calendar
	Seasons         []string       // TERM_SEASONS, default "Fall,Spring"; trimmed, empty entries dropped
	Timezone        *time.Location // TIMEZONE, default America/New_York
	HTTPTimeout     time.Duration  // HTTP_TIMEOUT, default 30s (time.ParseDuration)
	LogLevel        slog.Level     // LOG_LEVEL debug|info|warn|error, default info
}

// Load reads the environment (after an optional .env) and applies defaults.
// When requireGoogle is true it fails on the first missing Google variable,
// so a sync never starts half-configured; --parse-only passes false.
func Load(requireGoogle bool) (Config, error) {
	_ = godotenv.Load() // optional; env vars already set take precedence
	cfg := Config{
		CalendarID:      os.Getenv("CALENDAR_ID"),
		CredentialsFile: os.Getenv("GOOGLE_CREDENTIALS_FILE"),
		TokenFile:       os.Getenv("GOOGLE_TOKEN_FILE"),
		SourceURL:       getenv("CALENDAR_URL", "https://www.rit.edu/calendar"),
	}
	if requireGoogle {
		for _, v := range []struct{ name, val string }{
			{"CALENDAR_ID", cfg.CalendarID},
			{"GOOGLE_CREDENTIALS_FILE", cfg.CredentialsFile},
			{"GOOGLE_TOKEN_FILE", cfg.TokenFile},
		} {
			if v.val == "" {
				return Config{}, fmt.Errorf("%s environment variable is required", v.name)
			}
		}
	}
	for s := range strings.SplitSeq(getenv("TERM_SEASONS", "Fall,Spring"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			cfg.Seasons = append(cfg.Seasons, s)
		}
	}
	loc, err := time.LoadLocation(getenv("TIMEZONE", "America/New_York"))
	if err != nil {
		return Config{}, fmt.Errorf("parsing TIMEZONE: %w", err)
	}
	cfg.Timezone = loc
	if cfg.HTTPTimeout, err = time.ParseDuration(getenv("HTTP_TIMEOUT", "30s")); err != nil {
		return Config{}, fmt.Errorf("parsing HTTP_TIMEOUT: %w", err)
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(getenv("LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("parsing LOG_LEVEL: %w", err)
	}
	return cfg, nil
}

// getenv returns the named variable, or def when it is unset or empty, so an
// empty line in .env behaves the same as leaving the variable out.
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
