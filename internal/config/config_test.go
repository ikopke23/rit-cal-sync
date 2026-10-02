package config

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
)

// setEnv sets every variable Load reads, so the host environment can't leak
// into a test. Overrides replace the blank defaults.
func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	vars := map[string]string{
		"CALENDAR_ID":             "",
		"GOOGLE_CREDENTIALS_FILE": "",
		"GOOGLE_TOKEN_FILE":       "",
		"CALENDAR_URL":            "",
		"TERM_SEASONS":            "",
		"TIMEZONE":                "",
		"HTTP_TIMEOUT":            "",
		"LOG_LEVEL":               "",
	}
	for k, v := range overrides {
		vars[k] = v
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

var googleVars = map[string]string{
	"CALENDAR_ID":             "cal@group.calendar.google.com",
	"GOOGLE_CREDENTIALS_FILE": "creds.json",
	"GOOGLE_TOKEN_FILE":       "token.json",
}

func TestLoad_RequiredVars(t *testing.T) {
	tests := []struct {
		name    string
		unset   string
		wantVar string
	}{
		{name: "missing calendar id", unset: "CALENDAR_ID", wantVar: "CALENDAR_ID"},
		{name: "missing credentials file", unset: "GOOGLE_CREDENTIALS_FILE", wantVar: "GOOGLE_CREDENTIALS_FILE"},
		{name: "missing token file", unset: "GOOGLE_TOKEN_FILE", wantVar: "GOOGLE_TOKEN_FILE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range googleVars {
				env[k] = v
			}
			env[tt.unset] = ""
			setEnv(t, env)
			_, err := Load(true)
			if err == nil {
				t.Fatalf("got nil error, want error naming %s", tt.wantVar)
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("got %v, want error naming %s", err, tt.wantVar)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	setEnv(t, googleVars)
	cfg, err := Load(true)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SourceURL != "https://www.rit.edu/calendar" {
		t.Errorf("SourceURL: got %v, want %v", cfg.SourceURL, "https://www.rit.edu/calendar")
	}
	if want := []string{"Fall", "Spring"}; !slices.Equal(cfg.Seasons, want) {
		t.Errorf("Seasons: got %v, want %v", cfg.Seasons, want)
	}
	if cfg.Timezone.String() != "America/New_York" {
		t.Errorf("Timezone: got %v, want %v", cfg.Timezone, "America/New_York")
	}
	if cfg.HTTPTimeout != 30*time.Second {
		t.Errorf("HTTPTimeout: got %v, want %v", cfg.HTTPTimeout, 30*time.Second)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel: got %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.CalendarID != googleVars["CALENDAR_ID"] {
		t.Errorf("CalendarID: got %v, want %v", cfg.CalendarID, googleVars["CALENDAR_ID"])
	}
}

func TestLoad_SkipsGoogleValidation(t *testing.T) {
	setEnv(t, nil)
	if _, err := Load(false); err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		wantVar string
	}{
		{name: "bad http timeout", key: "HTTP_TIMEOUT", value: "soon", wantVar: "HTTP_TIMEOUT"},
		{name: "bad log level", key: "LOG_LEVEL", value: "loud", wantVar: "LOG_LEVEL"},
		{name: "bad timezone", key: "TIMEZONE", value: "Mars/Olympus", wantVar: "TIMEZONE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, map[string]string{tt.key: tt.value})
			_, err := Load(false)
			if err == nil {
				t.Fatalf("got nil error, want error naming %s", tt.wantVar)
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("got %v, want error naming %s", err, tt.wantVar)
			}
		})
	}
}

func TestLoad_ParsedValues(t *testing.T) {
	tests := []struct {
		name        string
		seasons     string
		logLevel    string
		wantSeasons []string
		wantLevel   slog.Level
	}{
		{name: "trims and drops empty seasons", seasons: " Fall , ,spring,", logLevel: "debug", wantSeasons: []string{"Fall", "spring"}, wantLevel: slog.LevelDebug},
		{name: "single season, uppercase level", seasons: "Summer", logLevel: "WARN", wantSeasons: []string{"Summer"}, wantLevel: slog.LevelWarn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, map[string]string{"TERM_SEASONS": tt.seasons, "LOG_LEVEL": tt.logLevel})
			cfg, err := Load(false)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if !slices.Equal(cfg.Seasons, tt.wantSeasons) {
				t.Errorf("Seasons: got %v, want %v", cfg.Seasons, tt.wantSeasons)
			}
			if cfg.LogLevel != tt.wantLevel {
				t.Errorf("LogLevel: got %v, want %v", cfg.LogLevel, tt.wantLevel)
			}
		})
	}
}
