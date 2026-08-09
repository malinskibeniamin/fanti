package main

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestCharacterHistorySeedFailureIsNonBlocking(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	if err := seed.Run(ctx, pool, seed.Sources{}, slog.Default()); err != nil {
		t.Fatalf("seed authored fixtures: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	seedCharacterHistoryBestEffort(ctx, pool, seed.CharacterHistoryOptions{
		APIURL:    server.URL,
		CacheDir:  t.TempDir(),
		Client:    server.Client(),
		RankLimit: 500,
	}, logger)

	if !strings.Contains(logs.String(), "character history import deferred") {
		t.Errorf("logs = %q, want deferred import warning", logs.String())
	}

	var count int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM character_history").Scan(&count); err != nil {
		t.Fatalf("count character history: %v", err)
	}
	if count != 0 {
		t.Errorf("character history rows = %d, want 0 after failed import", count)
	}
}

func TestSeedDataPresentRequiresCompleteBootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()

	present, err := seedDataPresent(ctx, pool)
	if err != nil {
		t.Fatalf("seedDataPresent empty: %v", err)
	}
	if present {
		t.Fatal("fresh database reported complete seed data")
	}

	if err := seed.Run(ctx, pool, seed.Sources{}, slog.Default()); err != nil {
		t.Fatalf("seed authored fixtures: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO dict_entries (traditional, simplified, pinyin) VALUES ('馬', '马', 'mǎ');
		INSERT INTO stroke_data (ch, medians, stroke_count) VALUES ('馬', '[]', 10);
		INSERT INTO sentences (id, traditional, simplified, english) VALUES (1, '馬。', '马。', 'Horse.');
	`); err != nil {
		t.Fatalf("seed external sentinels: %v", err)
	}

	present, err = seedDataPresent(ctx, pool)
	if err != nil {
		t.Fatalf("seedDataPresent complete: %v", err)
	}
	if present {
		t.Fatal("unmarked partial seed reported complete")
	}

	if _, err := pool.Exec(ctx, "INSERT INTO seed_runs (name) VALUES ('full')"); err != nil {
		t.Fatalf("insert legacy seed marker: %v", err)
	}
	present, err = seedDataPresent(ctx, pool)
	if err != nil {
		t.Fatalf("seedDataPresent legacy marker: %v", err)
	}
	if present {
		t.Fatal("legacy seed marker skipped the decomposition upgrade")
	}

	if _, err := pool.Exec(ctx, "INSERT INTO seed_runs (name) VALUES ('full-v2')"); err != nil {
		t.Fatalf("insert pre-catalog seed marker: %v", err)
	}
	present, err = seedDataPresent(ctx, pool)
	if err != nil {
		t.Fatalf("seedDataPresent pre-catalog marker: %v", err)
	}
	if present {
		t.Fatal("pre-catalog seed marker skipped the catalog upgrade")
	}

	if _, err := pool.Exec(ctx, "INSERT INTO seed_runs (name) VALUES ('full-v3')"); err != nil {
		t.Fatalf("insert pre-Tatoeba-frequency seed marker: %v", err)
	}
	present, err = seedDataPresent(ctx, pool)
	if err != nil {
		t.Fatalf("seedDataPresent pre-Tatoeba-frequency marker: %v", err)
	}
	if present {
		t.Fatal("pre-Tatoeba-frequency seed marker skipped the frequency-source upgrade")
	}

	if err := markSeedDataPresent(ctx, pool); err != nil {
		t.Fatalf("markSeedDataPresent: %v", err)
	}
	present, err = seedDataPresent(ctx, pool)
	if err != nil {
		t.Fatalf("seedDataPresent marked: %v", err)
	}
	if !present {
		t.Fatal("completed bootstrap marker reported missing")
	}
}
