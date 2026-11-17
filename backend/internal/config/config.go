// Package config reads the server's settings from environment variables.
//
// Environment variables are the only source of configuration: the same binary runs
// locally and on Render, and only its environment differs. Reading them all in one
// place means a missing or mistyped variable stops the server at start-up with a
// clear message, instead of failing later inside some request.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds every setting the server needs. Fields are added when the code that
// uses them is written; .env.example already lists the full set.
type Config struct {
	AppEnv      string // "development" or "production"
	Port        string // TCP port to listen on
	DatabaseURL string // PostgreSQL connection string (secret)
	// AppBaseURL is the public address of the frontend, e.g. https://x.vercel.app.
	// The API accepts state-changing requests only from this origin.
	AppBaseURL string
}

// Load reads the configuration from the process environment and validates it.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:      envOr("APP_ENV", "development"),
		Port:        envOr("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AppBaseURL:  strings.TrimRight(envOr("APP_BASE_URL", "http://localhost:5173"), "/"),
	}

	if cfg.AppEnv != "development" && cfg.AppEnv != "production" {
		return Config{}, fmt.Errorf(`APP_ENV must be "development" or "production", got %q`, cfg.AppEnv)
	}
	// An origin is scheme + host (+ port) and nothing else; a path would never match
	// the browser's Origin header and every form would be refused.
	if u, err := url.Parse(cfg.AppBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" {
		return Config{}, fmt.Errorf("APP_BASE_URL must look like https://example.com (no path), got %q", cfg.AppBaseURL)
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is not set (copy backend/.env.example to backend/.env and fill it in)")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// LoadDotEnv copies KEY=VALUE lines from a file into the process environment.
// It exists for local development only; in production there is no .env file and
// Render provides real environment variables.
//
// A missing file is not an error. A variable that is already set is left alone, so
// the real environment always wins over the file.
//
// This is ~30 lines instead of a dependency (godotenv) because we need only the
// simplest form of the format: no multi-line values, no variable expansion.
func LoadDotEnv(path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Cut splits at the FIRST "=", so values that contain "=" themselves
		// (connection strings with ?sslmode=require) stay intact.
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))

		if _, alreadySet := os.LookupEnv(key); !alreadySet {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// unquote removes one pair of matching quotes around a value: "abc" or 'abc'.
func unquote(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if first == last && (first == '"' || first == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
