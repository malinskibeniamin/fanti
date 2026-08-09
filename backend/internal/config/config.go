// Package config loads server configuration from defaults and FANTI_* environment variables.
package config

import (
	"fmt"
	"os"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

// Config holds all server settings.
type Config struct {
	ListenAddr  string `koanf:"listen_addr"`
	DatabaseURL string `koanf:"database_url"`
	BlobDir     string `koanf:"blob_dir"`
	// StaticDir serves the built web app when non-empty (production image).
	StaticDir string `koanf:"static_dir"`
}

// LookupFunc resolves an environment variable, mirroring os.LookupEnv.
type LookupFunc func(key string) (string, bool)

//nolint:gochecknoglobals // static env-var → config-path mapping
var envKeys = map[string]string{
	"FANTI_LISTEN_ADDR":  "listen_addr",
	"FANTI_DATABASE_URL": "database_url",
	"FANTI_BLOB_DIR":     "blob_dir",
	"FANTI_STATIC_DIR":   "static_dir",
}

// Load builds a Config from defaults overridden by environment variables.
// A nil lookup uses the real process environment.
func Load(lookup LookupFunc) (Config, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}

	k := koanf.New(".")

	//nolint:gosec // the DSN is a local docker-compose dev default, not a production credential
	defaults := map[string]any{
		"listen_addr":  ":8080",
		"database_url": "postgres://fanti:fanti@localhost:5433/fanti?sslmode=disable",
		"blob_dir":     "data/blobs",
		"static_dir":   "",
	}
	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
		return Config{}, fmt.Errorf("load defaults: %w", err)
	}

	overrides := map[string]any{}

	for env, path := range envKeys {
		if v, ok := lookup(env); ok {
			overrides[path] = v
		}
	}

	if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
		return Config{}, fmt.Errorf("load env overrides: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}
