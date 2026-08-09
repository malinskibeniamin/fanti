package seed

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type decompositionSeed struct {
	Character string
	Parts     []RadicalPart
}

type decompositionRecord struct {
	Character     string `json:"character"`
	Decomposition string `json:"decomposition"`
}

var idsArity = map[rune]int{ //nolint:gochecknoglobals // Unicode IDS grammar
	'⿰': 2,
	'⿱': 2,
	'⿲': 3,
	'⿳': 3,
	'⿴': 2,
	'⿵': 2,
	'⿶': 2,
	'⿷': 2,
	'⿸': 2,
	'⿹': 2,
	'⿺': 2,
	'⿻': 2,
}

var errNoValidDecompositions = errors.New("no valid decompositions found")

func parseDecompositions(r io.Reader) ([]decompositionSeed, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var rows []decompositionSeed
	for line := 1; scanner.Scan(); line++ {
		var record decompositionRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, fmt.Errorf("decode decomposition line %d: %w", line, err)
		}

		if utf8.RuneCountInString(record.Character) != 1 ||
			record.Decomposition == "" || strings.ContainsRune(record.Decomposition, '？') {
			continue
		}

		parts, ok := parseIDs(record.Decomposition)
		if !ok {
			continue
		}

		rows = append(rows, decompositionSeed{Character: record.Character, Parts: parts})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan decompositions: %w", err)
	}
	if len(rows) == 0 {
		return nil, errNoValidDecompositions
	}

	return rows, nil
}

func parseIDs(value string) ([]RadicalPart, bool) {
	runes := []rune(value)
	index := 0

	var parseNode func() ([]RadicalPart, bool)
	parseNode = func() ([]RadicalPart, bool) {
		if index >= len(runes) {
			return nil, false
		}

		current := runes[index]
		index++
		if arity, isOperator := idsArity[current]; isOperator {
			var parts []RadicalPart
			for range arity {
				children, ok := parseNode()
				if !ok {
					return nil, false
				}
				parts = append(parts, children...)
			}

			return parts, true
		}

		if current >= '\u2ff0' && current <= '\u2fff' {
			return nil, false
		}

		return []RadicalPart{{Part: string(current)}}, true
	}

	parts, ok := parseNode()

	return parts, ok && index == len(runes) && len(parts) > 0
}

func seedDecompositions(
	ctx context.Context,
	pool *pgxpool.Pool,
	source io.Reader,
	logger *slog.Logger,
) error {
	decompositions, err := parseDecompositions(source)
	if err != nil {
		return err
	}

	rows := make([][]any, 0, len(decompositions))
	for _, decomposition := range decompositions {
		parts, err := json.Marshal(decomposition.Parts)
		if err != nil {
			return fmt.Errorf("marshal decomposition %s: %w", decomposition.Character, err)
		}
		rows = append(rows, []any{decomposition.Character, parts})
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin decompositions: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE decomposition_seed (
			ch TEXT PRIMARY KEY,
			radical_parts JSONB NOT NULL
		) ON COMMIT DROP`); err != nil {
		return fmt.Errorf("create decomposition seed table: %w", err)
	}

	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"decomposition_seed"},
		[]string{"ch", "radical_parts"},
		pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy decompositions: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE stroke_data AS strokes
		SET radical_parts = seed.radical_parts
		FROM decomposition_seed AS seed
		WHERE strokes.ch = seed.ch`)
	if err != nil {
		return fmt.Errorf("update stroke decompositions: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit decompositions: %w", err)
	}

	logger.InfoContext(ctx, "seeded character decompositions",
		slog.Int("source_characters", len(decompositions)),
		slog.Int64("stroke_characters", tag.RowsAffected()))

	return nil
}
