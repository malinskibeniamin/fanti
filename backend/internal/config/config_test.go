package config_test

import (
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(nil)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}

	if cfg.DatabaseURL == "" {
		t.Error("DatabaseURL default must not be empty")
	}

	if cfg.BlobDir == "" {
		t.Error("BlobDir default must not be empty")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"FANTI_LISTEN_ADDR":  ":9999",
		"FANTI_DATABASE_URL": "postgres://other:5432/x",
		"FANTI_BLOB_DIR":     "/tmp/blobs",
	}

	cfg, err := config.Load(func(key string) (string, bool) {
		v, ok := env[key]

		return v, ok
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ListenAddr != ":9999" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9999")
	}

	if cfg.DatabaseURL != "postgres://other:5432/x" {
		t.Errorf("DatabaseURL = %q, want override", cfg.DatabaseURL)
	}

	if cfg.BlobDir != "/tmp/blobs" {
		t.Errorf("BlobDir = %q, want override", cfg.BlobDir)
	}
}
