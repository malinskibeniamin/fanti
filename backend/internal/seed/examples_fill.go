package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// fillMaxExamples is how many sentences a character page shows.
	fillMaxExamples = 2
	// fillMinChars/fillMaxChars bound sentence length: single-character
	// fragments teach nothing and long community sentences overwhelm a
	// compact example row.
	fillMinChars = 2
	fillMaxChars = 20
	// fillCandidateLimit leaves headroom for denylist rejections.
	fillCandidateLimit = 25
	// exampleSourceTatoeba marks auto-picked examples apart from the
	// authored fixtures, so re-ranking can refresh them selectively.
	exampleSourceTatoeba = "tatoeba"
)

// deniedEnglish matches on word boundaries so innocent words (Sussex,
// assess) never trip it.
var deniedEnglish = regexp.MustCompile(`(?i)\b(fuck|fucking|shit|damn|bitch|whore|porn|sex|rape|nazi|nigger)\b`)

// deniedChinese are substring phrases; multi-character vulgarities don't
// collide with everyday text the way single characters would.
//
//nolint:gochecknoglobals // static content filter
var deniedChinese = []string{
	"他媽", "他妈", "傻逼", "混蛋", "婊", "操你", "幹你", "去死", "雞巴", "鸡巴", "屌",
}

// containsDenied reports whether a sentence pair is unfit as a beginner's
// example. Tatoeba is unmoderated community content; this is the last
// mechanical guard before a sentence lands on a character page.
func containsDenied(chinese, english string) bool {
	if deniedEnglish.MatchString(english) {
		return true
	}

	for _, phrase := range deniedChinese {
		if strings.Contains(chinese, phrase) {
			return true
		}
	}

	return false
}

// fillExample is the characters.examples JSONB element shape. Source is
// absent on authored fixtures and "tatoeba" on auto-picked rows.
type fillExample struct {
	HskLevel int    `json:"hskLevel"`
	Chinese  string `json:"chinese"`
	English  string `json:"english"`
	Source   string `json:"source"`
}

// fillTargets lists characters whose examples the pass may write: rows
// with no examples, plus — when force is set — rows whose every example
// is auto-picked. Authored examples are never eligible.
func fillTargets(ctx context.Context, pool *pgxpool.Pool, force bool) ([]string, error) {
	query := "SELECT traditional FROM characters WHERE examples = '[]'::jsonb"
	if force {
		query = `SELECT traditional FROM characters WHERE NOT EXISTS (
			SELECT 1 FROM jsonb_array_elements(examples) e
			WHERE e->>'source' IS DISTINCT FROM $1)`
	}

	var args []any
	if force {
		args = append(args, exampleSourceTatoeba)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query fill targets: %w", err)
	}

	defer rows.Close()

	var targets []string

	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, fmt.Errorf("scan fill target: %w", err)
		}

		targets = append(targets, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fill targets rows: %w", err)
	}

	return targets, nil
}

// pickExamples selects a character's example sentences: common-character
// sentences first (ranked, low max_freq_rank), then shorter, then oldest —
// a deterministic order.
func pickExamples(ctx context.Context, pool *pgxpool.Pool, ch string) ([]fillExample, error) {
	rows, err := pool.Query(ctx, `
		SELECT traditional, english FROM sentences
		WHERE chars @> ARRAY[$1]
		  AND char_count BETWEEN $2 AND $3
		  AND NOT ambiguous
		ORDER BY (max_freq_rank = 0), max_freq_rank, char_count, id
		LIMIT $4`,
		ch, fillMinChars, fillMaxChars, fillCandidateLimit)
	if err != nil {
		return nil, fmt.Errorf("query candidates %s: %w", ch, err)
	}

	defer rows.Close()

	var picked []fillExample

	for rows.Next() {
		var chinese, english string
		if err := rows.Scan(&chinese, &english); err != nil {
			return nil, fmt.Errorf("scan candidate %s: %w", ch, err)
		}

		if containsDenied(chinese, english) {
			continue
		}

		picked = append(picked, fillExample{
			Chinese: chinese, English: english, Source: exampleSourceTatoeba,
		})
		if len(picked) == fillMaxExamples {
			break
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("candidate rows %s: %w", ch, err)
	}

	return picked, nil
}

// FillExamples gives every example-less character real corpus sentences.
// Plain runs only touch characters with no examples at all; force re-picks
// rows whose examples are all auto-filled, leaving authored ones alone.
func FillExamples(ctx context.Context, pool *pgxpool.Pool, force bool, logger *slog.Logger) error {
	targets, err := fillTargets(ctx, pool, force)
	if err != nil {
		return err
	}

	filled := 0

	for _, ch := range targets {
		picked, err := pickExamples(ctx, pool, ch)
		if err != nil {
			return err
		}

		if len(picked) == 0 {
			continue
		}

		payload, err := json.Marshal(picked)
		if err != nil {
			return fmt.Errorf("marshal examples %s: %w", ch, err)
		}

		if _, err := pool.Exec(ctx,
			"UPDATE characters SET examples = $2 WHERE traditional = $1", ch, payload); err != nil {
			return fmt.Errorf("update examples %s: %w", ch, err)
		}

		filled++
	}

	logger.InfoContext(ctx, "filled character examples",
		slog.Int("targets", len(targets)),
		slog.Int("filled", filled),
		slog.Bool("force", force))

	return nil
}
