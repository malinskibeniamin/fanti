package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/data"
	"github.com/malinskibeniamin/fanti/backend/internal/convert"
)

var errEmptyTatoebaPairs = errors.New("no sentence pairs parsed from tatoeba derivative")

// These two wordplay fragments translate as "Love loves love" and do not
// provide a grammatical learner example.
func isKnownLowQualityTatoebaID(id int64) bool {
	return id == 1531795 || id == 1531796
}

// TatoebaPair is one row of the vendored derivative (`id\tcmn\teng`).
type TatoebaPair struct {
	// ID is the Tatoeba Mandarin sentence id (kept for attribution).
	ID       int64
	Mandarin string
	English  string
}

// ParseTatoebaPairs reads the vendored derivative. Fields are split on
// literal tabs — Tatoeba text has no quote escaping, so encoding/csv
// would corrupt fields that begin with a double quote.
func ParseTatoebaPairs(r io.Reader) ([]TatoebaPair, error) {
	var pairs []TatoebaPair

	seen := map[int64]bool{}

	sc := tatoebaScanner(r)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}

		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		// A duplicated id (hand-edited or corrupted derivative) would
		// abort the whole bulk load on the primary key; keep the first.
		if parts[1] == "" || parts[2] == "" || seen[id] {
			continue
		}

		seen[id] = true

		pairs = append(pairs, TatoebaPair{ID: id, Mandarin: parts[1], English: parts[2]})
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read tatoeba pairs: %w", err)
	}

	if len(pairs) == 0 {
		return nil, errEmptyTatoebaPairs
	}

	return pairs, nil
}

// hanChars lists the distinct Han characters of a sentence in first-seen
// order; digits, Latin, and punctuation never count toward unlocking.
func hanChars(s string) []string {
	var chars []string

	seen := map[rune]bool{}

	for _, r := range s {
		if !unicode.Is(unicode.Han, r) || seen[r] {
			continue
		}

		seen[r] = true
		chars = append(chars, string(r))
	}

	return chars
}

// sentenceMaxFreqRank grades a sentence by its rarest character: the
// highest frequency rank among its characters, or 0 (unranked) when any
// character falls outside the ranked set — an unranked character means
// the sentence cannot be called easy.
func sentenceMaxFreqRank(chars []string, ranks map[string]int) int {
	maxRank := 0

	for _, ch := range chars {
		rank, ok := ranks[ch]
		if !ok || rank <= 0 {
			return 0
		}

		if rank > maxRank {
			maxRank = rank
		}
	}

	return maxRank
}

// sentenceRow is one planned sentences insert.
type sentenceRow struct {
	id          int64
	traditional string
	simplified  string
	english     string
	chars       []string
	charCount   int
	maxFreqRank int
	// ambiguous marks a simplified→traditional conversion that crossed a
	// one-to-many character mapping — OpenCC may have picked the wrong
	// form, so the sentence is stored but never surfaced to learners.
	ambiguous bool
	// inCourse is true when every character has a characters-table row.
	inCourse bool
}

// buildSentenceRow normalizes one pair into both scripts. The detected
// source script stays verbatim; the other form is converted, so authentic
// text is never rewritten.
func buildSentenceRow(
	engine *convert.Engine, p TatoebaPair, ranks map[string]int, ambiguous map[string]bool,
) (sentenceRow, error) {
	dir, err := engine.DetectScript(p.Mandarin)
	if err != nil {
		return sentenceRow{}, fmt.Errorf("detect script %d: %w", p.ID, err)
	}

	traditional, simplified := p.Mandarin, p.Mandarin

	var risky bool

	// DetectScript names the conversion that would change the most
	// characters: S2T means the source reads as simplified.
	if dir == convert.S2T {
		if traditional, err = engine.ConvertText(p.Mandarin, convert.Options{Direction: convert.S2T}); err != nil {
			return sentenceRow{}, fmt.Errorf("convert %d to traditional: %w", p.ID, err)
		}

		risky = containsAny(simplified, ambiguous)
	} else {
		if simplified, err = engine.ConvertText(p.Mandarin, convert.Options{Direction: convert.T2S}); err != nil {
			return sentenceRow{}, fmt.Errorf("convert %d to simplified: %w", p.ID, err)
		}

		// A mixed-script source detects as traditional but still carries
		// convertible simplified glyphs; the round trip exposes them.
		round, err := engine.ConvertText(traditional, convert.Options{Direction: convert.S2T})
		if err != nil {
			return sentenceRow{}, fmt.Errorf("round-trip %d: %w", p.ID, err)
		}

		risky = round != traditional
	}

	chars := hanChars(traditional)

	return sentenceRow{
		id:          p.ID,
		traditional: traditional,
		simplified:  simplified,
		english:     p.English,
		chars:       chars,
		charCount:   hanCount(traditional),
		maxFreqRank: sentenceMaxFreqRank(chars, ranks),
		ambiguous:   risky,
	}, nil
}

// allInCourse reports whether every character has a characters-table row.
func allInCourse(chars []string, course map[string]bool) bool {
	for _, ch := range chars {
		if !course[ch] {
			return false
		}
	}

	return len(chars) > 0
}

// containsAny reports whether any Han character of s is in the set.
func containsAny(s string, set map[string]bool) bool {
	for _, ch := range hanChars(s) {
		if set[ch] {
			return true
		}
	}

	return false
}

// hanCount is the total Han rune count — the sentence's length as a
// learner experiences it.
func hanCount(s string) int {
	n := 0

	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}

	return n
}

// ambiguousSimplifiedChars gathers every simplified character whose
// traditional mapping is one-to-many: the CEDICT-derived set (a simplified
// form with several distinct traditional single-character entries) plus
// the curated conversion-exception fixture.
func ambiguousSimplifiedChars(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	chars := map[string]bool{}

	rows, err := pool.Query(ctx, `
		SELECT simplified FROM dict_entries
		WHERE char_length(simplified) = 1 AND char_length(traditional) = 1
		GROUP BY simplified
		HAVING count(DISTINCT traditional) > 1`)
	if err != nil {
		return nil, fmt.Errorf("query ambiguous singles: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, fmt.Errorf("scan ambiguous single: %w", err)
		}

		chars[ch] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ambiguous singles rows: %w", err)
	}

	raw, err := data.SeedFS.ReadFile("seed/conversion-exceptions.json")
	if err != nil {
		return nil, fmt.Errorf("read exceptions fixture: %w", err)
	}

	var fixture struct {
		S2T []struct {
			SourceChar string `json:"sourceChar"`
			Status     string `json:"status"`
		} `json:"s2t"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		return nil, fmt.Errorf("decode exceptions fixture: %w", err)
	}

	for _, ex := range fixture.S2T {
		if ex.Status == "ambiguous" {
			chars[ex.SourceChar] = true
		}
	}

	return chars, nil
}

// SeedTatoeba loads the vendored sentence pairs into sentences; skips
// when already populated. Ambiguous simplified→traditional conversions
// are counted and logged so mapping drift stays visible in seed output.
//
//nolint:revive // seed.Run/seed.SeedFrequency/seed.SeedTatoeba read as a family
func SeedTatoeba(
	ctx context.Context, pool *pgxpool.Pool, engine *convert.Engine, r io.Reader, logger *slog.Logger,
) error {
	var count int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sentences").Scan(&count); err != nil {
		return fmt.Errorf("count sentences: %w", err)
	}

	if count > 0 {
		logger.InfoContext(ctx, "sentences already populated, skipping", slog.Int64("count", count))

		return nil
	}

	pairs, err := ParseTatoebaPairs(r)
	if err != nil {
		return err
	}

	ranks := map[string]int{}
	if err := scanStringMap(ctx, pool, `
		SELECT traditional, frequency_rank FROM characters
		WHERE frequency_rank > 0`, ranks); err != nil {
		return err
	}

	ambiguous, err := ambiguousSimplifiedChars(ctx, pool)
	if err != nil {
		return err
	}

	course := map[string]bool{}
	if err := scanStringMap(ctx, pool,
		"SELECT traditional, TRUE FROM characters", course); err != nil {
		return err
	}

	rows := make([][]any, 0, len(pairs))
	ambiguousHits, denied, lowQuality := 0, 0, 0

	for _, p := range pairs {
		row, err := buildSentenceRow(engine, p, ranks, ambiguous)
		if err != nil {
			return err
		}

		if len(row.chars) == 0 {
			continue // no Han characters — nothing a learner could unlock
		}

		// Unfit community content never enters the table: the speakable
		// summary queries sentences directly, so filtering only at
		// example-pick time would still let it reach the study card.
		if containsDenied(row.traditional, row.english) {
			denied++

			continue
		}
		if isKnownLowQualityTatoebaID(row.id) {
			lowQuality++

			continue
		}

		if row.ambiguous {
			ambiguousHits++
		}

		row.inCourse = allInCourse(row.chars, course)

		rows = append(rows, []any{
			row.id, row.traditional, row.simplified, row.english,
			row.chars, row.charCount, row.maxFreqRank, row.ambiguous, row.inCourse,
		})
	}

	if _, err := pool.CopyFrom(ctx,
		pgx.Identifier{"sentences"},
		[]string{"id", columnTraditional, columnSimplified, "english", "chars", "char_count", "max_freq_rank", "ambiguous", "in_course"},
		pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy sentences: %w", err)
	}

	logger.InfoContext(ctx, "seeded tatoeba sentences",
		slog.Int("pairs", len(pairs)),
		slog.Int("inserted", len(rows)),
		slog.Int("denied_content_sentences", denied),
		slog.Int("low_quality_sentences", lowQuality),
		slog.Int("ambiguous_conversion_sentences", ambiguousHits))

	return nil
}
