package seed

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationSeedDecompositionsUpdatesStrokeRowsIdempotently(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if _, err := pool.Exec(ctx, `
		INSERT INTO stroke_data (ch, medians, stroke_count)
		VALUES ('俢', '[]', 1), ('馬', '[]', 10)`); err != nil {
		t.Fatalf("insert stroke rows: %v", err)
	}

	if err := Run(ctx, pool, Sources{}, logger); err != nil {
		t.Fatalf("seed curated fixtures: %v", err)
	}

	source := `{"character":"俢","decomposition":"⿰亻⿱夂彡"}` + "\n" +
		`{"character":"馬","decomposition":"⿹𠂉灬"}` + "\n"
	for range 2 {
		if err := seedDecompositions(ctx, pool, strings.NewReader(source), logger); err != nil {
			t.Fatalf("seedDecompositions() error = %v", err)
		}
	}

	var raw []byte
	if err := pool.QueryRow(ctx,
		"SELECT radical_parts FROM stroke_data WHERE ch = '俢'").Scan(&raw); err != nil {
		t.Fatalf("select 俢 parts: %v", err)
	}

	var parts []RadicalPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		t.Fatalf("decode 俢 parts: %v", err)
	}

	if got, want := partGlyphs(parts), []string{"亻", "夂", "彡"}; !slices.Equal(got, want) {
		t.Errorf("俢 parts = %v, want %v", got, want)
	}

	var curatedRaw []byte
	if err := pool.QueryRow(ctx,
		"SELECT radical_parts FROM characters WHERE traditional = '馬'").Scan(&curatedRaw); err != nil {
		t.Fatalf("select curated 馬 parts: %v", err)
	}
	if string(curatedRaw) == "[]" {
		t.Error("curated 馬 parts were replaced")
	}
}
