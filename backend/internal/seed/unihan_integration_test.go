package seed

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationUnihan(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	// An existing reading — authored or CEDICT — must never be clobbered.
	if _, err := pool.Exec(ctx,
		"INSERT INTO char_pinyin (ch, pinyin) VALUES ('中', 'zhōng·curated')"); err != nil {
		t.Fatalf("insert existing reading: %v", err)
	}

	const unihanFixture = "U+4E2D\tkMandarin\tzhōng\n" +
		"U+3401\tkHanyuPinyin\t10019.020:tiàn,tián\n"

	run := func() {
		t.Helper()

		if err := SeedUnihan(ctx, pool, strings.NewReader(unihanFixture), logger); err != nil {
			t.Fatalf("SeedUnihan() error = %v", err)
		}
	}

	run()
	run() // idempotency: the backfill only ever fills gaps

	get := func(ch string) string {
		t.Helper()

		var py string
		if err := pool.QueryRow(ctx,
			"SELECT pinyin FROM char_pinyin WHERE ch = $1", ch).Scan(&py); err != nil {
			t.Fatalf("reading %s: %v", ch, err)
		}

		return py
	}

	if got := get("中"); got != "zhōng·curated" {
		t.Errorf("中 = %q, want the pre-existing reading preserved", got)
	}

	if got := get("㐁"); got != "tiàn" {
		t.Errorf("㐁 = %q, want tiàn backfilled from kHanyuPinyin", got)
	}

	var total int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM char_pinyin").Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}

	if total != 2 {
		t.Errorf("char_pinyin rows = %d, want 2 (no duplicates after rerun)", total)
	}
}
