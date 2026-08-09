package seed

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	catalogKindCurriculum = "curriculum"
	catalogKindReference  = "reference"
	columnTraditional     = "traditional"
	columnSimplified      = "simplified"
	columnPinyin          = "pinyin"
)

type catalogSense struct {
	traditional string
	simplified  string
	pinyin      string
	definitions []string
}

type catalogUpsert struct {
	traditional   string
	simplified    string
	pinyin        string
	meaning       string
	mappingStatus string
	strokeCount   int
	siblings      []string
	catalogKind   string
}

// buildCharacterCatalogPlan creates one learning entry per traditional
// CC-CEDICT headword and one standalone reference entry per remaining
// Mandarin-readable Unihan glyph. Simplified forms stay related to their
// traditional entry rather than becoming duplicate learning entries.
func buildCharacterCatalogPlan(
	senses []catalogSense,
	readings map[string]string,
	strokes map[string]int,
) []catalogUpsert {
	byTraditional := make(map[string][]catalogSense)
	simplifiedToTraditional := make(map[string][]string)
	cedictForms := make(map[string]bool)

	for _, sense := range senses {
		byTraditional[sense.traditional] = append(byTraditional[sense.traditional], sense)
		cedictForms[sense.traditional] = true
		cedictForms[sense.simplified] = true

		forms := simplifiedToTraditional[sense.simplified]
		if !slices.Contains(forms, sense.traditional) {
			simplifiedToTraditional[sense.simplified] = append(forms, sense.traditional)
		}
	}

	traditional := make([]string, 0, len(byTraditional))
	for ch := range byTraditional {
		traditional = append(traditional, ch)
	}
	sort.Strings(traditional)

	plan := make([]catalogUpsert, 0, len(traditional)+len(readings))
	for _, ch := range traditional {
		chosen := pickCatalogSense(byTraditional[ch])
		siblings := otherCatalogForms(simplifiedToTraditional[chosen.simplified], ch)

		strokeCount := strokes[ch]
		if strokeCount == 0 {
			strokeCount = strokes[chosen.simplified]
		}

		status := mappingExact
		if len(siblings) > 0 {
			status = mappingAmbiguous
		}

		plan = append(plan, catalogUpsert{
			traditional:   ch,
			simplified:    chosen.simplified,
			pinyin:        chosen.pinyin,
			meaning:       joinDefinitions(chosen.definitions),
			mappingStatus: status,
			strokeCount:   strokeCount,
			siblings:      siblings,
			catalogKind:   catalogKindCurriculum,
		})
	}

	reference := make([]string, 0, len(readings))
	for ch := range readings {
		if !cedictForms[ch] {
			reference = append(reference, ch)
		}
	}
	sort.Strings(reference)

	for _, ch := range reference {
		plan = append(plan, catalogUpsert{
			traditional:   ch,
			simplified:    ch,
			pinyin:        readings[ch],
			mappingStatus: mappingExact,
			strokeCount:   strokes[ch],
			siblings:      []string{},
			catalogKind:   catalogKindReference,
		})
	}

	return plan
}

func pickCatalogSense(senses []catalogSense) catalogSense {
	for _, sense := range senses {
		if len(sense.definitions) == 0 {
			continue
		}

		lead := sense.definitions[0]
		if isCrossReferenceOrSurname(lead) {
			continue
		}

		return sense
	}

	return senses[0]
}

func isCrossReferenceOrSurname(lead string) bool {
	return strings.Contains(lead, "variant of") || strings.HasPrefix(lead, "surname ")
}

func otherCatalogForms(forms []string, chosen string) []string {
	others := make([]string, 0, len(forms))
	for _, form := range forms {
		if form != chosen {
			others = append(others, form)
		}
	}

	return others
}

// SeedCharacterCatalog materializes the complete source-backed learning
// catalog from the already-loaded CC-CEDICT and Unihan tables.
//
//nolint:revive // exported to support explicit seed repair/verification
func SeedCharacterCatalog(
	ctx context.Context,
	pool *pgxpool.Pool,
	logger *slog.Logger,
) error {
	senses, err := loadCatalogSenses(ctx, pool)
	if err != nil {
		return err
	}

	readings := map[string]string{}
	if err := scanStringMap(ctx, pool,
		"SELECT ch, pinyin FROM char_pinyin", readings); err != nil {
		return err
	}

	strokes := map[string]int{}
	if err := scanStringMap(ctx, pool,
		"SELECT ch, stroke_count FROM stroke_data", strokes); err != nil {
		return err
	}

	plan := buildCharacterCatalogPlan(senses, readings, strokes)
	if len(plan) == 0 {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin catalog sync: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		CREATE TEMPORARY TABLE character_catalog_seed (
			traditional TEXT PRIMARY KEY,
			simplified TEXT NOT NULL,
			pinyin TEXT NOT NULL,
			meaning TEXT NOT NULL,
			mapping_status TEXT NOT NULL,
			stroke_count INT NOT NULL,
			siblings TEXT[] NOT NULL,
			catalog_kind TEXT NOT NULL
		) ON COMMIT DROP`); err != nil {
		return fmt.Errorf("create catalog staging table: %w", err)
	}

	rows := make([][]any, 0, len(plan))
	for _, item := range plan {
		rows = append(rows, []any{
			item.traditional,
			item.simplified,
			item.pinyin,
			item.meaning,
			item.mappingStatus,
			item.strokeCount,
			item.siblings,
			item.catalogKind,
		})
	}

	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"character_catalog_seed"},
		[]string{
			columnTraditional,
			columnSimplified,
			columnPinyin,
			"meaning",
			"mapping_status",
			"stroke_count",
			"siblings",
			"catalog_kind",
		},
		pgx.CopyFromRows(rows),
	); err != nil {
		return fmt.Errorf("copy catalog: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		INSERT INTO characters (
			traditional, simplified, pinyin, meaning, mapping_status,
			stroke_count, siblings, catalog_kind
		)
		SELECT traditional, simplified, pinyin, meaning, mapping_status,
			stroke_count, siblings, catalog_kind
		FROM character_catalog_seed
		ON CONFLICT (traditional) DO UPDATE SET
			simplified = CASE
				WHEN EXCLUDED.catalog_kind = 'curriculum'
					AND characters.catalog_kind = 'reference'
					THEN EXCLUDED.simplified
				WHEN characters.simplified = '' THEN EXCLUDED.simplified
				ELSE characters.simplified
			END,
			pinyin = CASE
				WHEN EXCLUDED.catalog_kind = 'curriculum'
					AND characters.catalog_kind = 'reference'
					THEN EXCLUDED.pinyin
				ELSE COALESCE(NULLIF(characters.pinyin, ''), EXCLUDED.pinyin)
			END,
			meaning = CASE
				WHEN EXCLUDED.catalog_kind = 'curriculum'
					AND characters.catalog_kind = 'reference'
					THEN EXCLUDED.meaning
				ELSE COALESCE(NULLIF(characters.meaning, ''), EXCLUDED.meaning)
			END,
			mapping_status = CASE
				WHEN EXCLUDED.catalog_kind = 'curriculum'
					THEN EXCLUDED.mapping_status
				ELSE characters.mapping_status
			END,
			stroke_count = CASE
				WHEN characters.stroke_count > 0 THEN characters.stroke_count
				ELSE EXCLUDED.stroke_count
			END,
			siblings = CASE
				WHEN EXCLUDED.catalog_kind = 'curriculum'
					THEN EXCLUDED.siblings
				ELSE characters.siblings
			END,
			catalog_kind = CASE
				WHEN EXCLUDED.catalog_kind = 'curriculum' THEN 'curriculum'
				WHEN characters.meaning = '' THEN 'reference'
				ELSE characters.catalog_kind
			END`)
	if err != nil {
		return fmt.Errorf("upsert catalog: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit catalog sync: %w", err)
	}

	logger.InfoContext(ctx, "seeded character catalog",
		slog.Int("source_senses", len(senses)),
		slog.Int("entries", len(plan)),
		slog.Int64("upserted", tag.RowsAffected()))

	return nil
}

func loadCatalogSenses(ctx context.Context, pool *pgxpool.Pool) ([]catalogSense, error) {
	rows, err := pool.Query(ctx, `
		SELECT traditional, simplified, pinyin, definitions
		FROM dict_entries
		WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query catalog senses: %w", err)
	}
	defer rows.Close()

	var senses []catalogSense
	for rows.Next() {
		var sense catalogSense
		if err := rows.Scan(
			&sense.traditional,
			&sense.simplified,
			&sense.pinyin,
			&sense.definitions,
		); err != nil {
			return nil, fmt.Errorf("scan catalog sense: %w", err)
		}
		senses = append(senses, sense)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog sense rows: %w", err)
	}

	return senses, nil
}

// RankCharacterCurriculum creates a dense, deterministic path: ranked
// CEDICT characters first, then the unranked remainder in Unicode order.
// Reference entries remain outside the automatic curriculum.
func RankCharacterCurriculum(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		UPDATE characters SET curriculum_rank = 0
		WHERE catalog_kind <> 'curriculum';

		WITH ranked AS (
			SELECT traditional,
				row_number() OVER (
					ORDER BY frequency_rank = 0, frequency_rank,
						traditional COLLATE "C"
				)::INT AS rank
			FROM characters
			WHERE catalog_kind = 'curriculum'
		)
		UPDATE characters AS characters
		SET curriculum_rank = ranked.rank
		FROM ranked
		WHERE characters.traditional = ranked.traditional
			AND characters.curriculum_rank IS DISTINCT FROM ranked.rank`); err != nil {
		return fmt.Errorf("rank character curriculum: %w", err)
	}

	return nil
}
