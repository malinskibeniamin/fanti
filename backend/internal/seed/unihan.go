package seed

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UnihanURL is the Unicode 17.0.0 Unihan database (Unicode Terms of Use —
// see NOTICES.md). Pinned to a release path, never "latest".
const UnihanURL = "https://www.unicode.org/Public/17.0.0/ucd/Unihan.zip"

var errEmptyUnihan = errors.New("no readings parsed from unihan data")

// ParseUnihanReadings extracts one pinyin reading per codepoint from
// Unihan_Readings.txt (`U+4E2D<tab>kMandarin<tab>zhōng`). kMandarin (already
// in diacritic form; first alternative wins) is preferred; kHanyuPinyin
// (`10067.010:zhōng,zhòng`; first reading of the first location) fills in
// codepoints kMandarin lacks.
func ParseUnihanReadings(r io.Reader) (map[string]string, error) {
	mandarin := map[string]string{}
	hanyu := map[string]string{}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)

	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}

		ch, ok := codepointChar(parts[0])
		if !ok {
			continue
		}

		switch parts[1] {
		case "kMandarin":
			// Alternatives are space-separated; the first is preferred.
			if fields := strings.Fields(parts[2]); len(fields) > 0 {
				mandarin[ch] = fields[0]
			}
		case "kHanyuPinyin":
			hanyu[ch] = firstHanyuReading(parts[2])
		}
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read unihan readings: %w", err)
	}

	for ch, py := range hanyu {
		if _, ok := mandarin[ch]; !ok && py != "" {
			mandarin[ch] = py
		}
	}

	if len(mandarin) == 0 {
		return nil, errEmptyUnihan
	}

	return mandarin, nil
}

// codepointChar decodes a `U+4E2D` field into its character.
func codepointChar(field string) (string, bool) {
	hex, ok := strings.CutPrefix(field, "U+")
	if !ok {
		return "", false
	}

	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil || value > unicode.MaxRune {
		return "", false
	}

	return string(rune(value)), true
}

// firstHanyuReading picks the first reading of a kHanyuPinyin value
// (`10019.020:tiàn,tián` → tiàn).
func firstHanyuReading(value string) string {
	_, readings, ok := strings.Cut(value, ":")
	if !ok {
		return ""
	}

	first, _, _ := strings.Cut(readings, ",")

	return strings.TrimSpace(first)
}

// SeedUnihan backfills char_pinyin with long-tail readings for codepoints
// the dictionary sources lack; existing rows — authored and CEDICT alike —
// are never overwritten.
//
//nolint:revive // seed.Run/seed.SeedFrequency/seed.SeedUnihan read as a family
func SeedUnihan(ctx context.Context, pool *pgxpool.Pool, r io.Reader, logger *slog.Logger) error {
	readings, err := ParseUnihanReadings(r)
	if err != nil {
		return err
	}

	// Stage the readings and keep only the gaps. One transaction keeps
	// the temporary table on a single connection (temp tables are
	// per-connection, and the pool would otherwise split the steps).
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin backfill: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		CREATE TEMPORARY TABLE unihan_readings (
			ch TEXT PRIMARY KEY, pinyin TEXT NOT NULL
		) ON COMMIT DROP`); err != nil {
		return fmt.Errorf("create staging table: %w", err)
	}

	rows := make([][]any, 0, len(readings))
	for ch, py := range readings {
		rows = append(rows, []any{ch, py})
	}

	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"unihan_readings"},
		[]string{"ch", columnPinyin},
		pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy unihan readings: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO char_pinyin (ch, pinyin)
		SELECT ch, pinyin FROM unihan_readings
		ON CONFLICT (ch) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("backfill char_pinyin: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit backfill: %w", err)
	}

	logger.InfoContext(ctx, "seeded unihan readings",
		slog.Int("parsed", len(readings)),
		slog.Int64("backfilled", tag.RowsAffected()))

	return nil
}
