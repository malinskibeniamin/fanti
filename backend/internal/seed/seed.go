package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sources supplies the external datasets. Nil readers skip that step,
// so tests and partial seeds stay cheap.
type Sources struct {
	// CC-CEDICT text (already gunzipped).
	CEDICT io.Reader
	// Make Me a Hanzi dictionary.txt decompositions (newline-delimited JSON).
	Decompositions io.Reader
	// hanzi-writer-data npm tarball (.tgz).
	StrokesTarball io.Reader
	// Vendored Tatoeba sentence derivative used to calculate character ranks.
	Frequency io.Reader
	// Unihan_Readings.txt (extracted from Unihan.zip).
	UnihanReadings io.Reader
}

// Run seeds all authored fixtures plus any provided external sources.
// Every step is idempotent (upserts / skip-when-populated).
func Run(ctx context.Context, pool *pgxpool.Pool, src Sources, logger *slog.Logger) error {
	fixtures, err := LoadFixtures()
	if err != nil {
		return fmt.Errorf("load fixtures: %w", err)
	}

	steps := []struct {
		name string
		run  func(context.Context, *pgxpool.Pool, Fixtures) error
	}{
		{"characters", seedCharacters},
		{"char_pinyin", seedCharPinyin},
		{"word_cards", seedWords},
		{"compounds", seedCompounds},
		{"milestones", seedMilestones},
		{"books", seedBooks},
	}

	for _, step := range steps {
		if err := step.run(ctx, pool, fixtures); err != nil {
			return fmt.Errorf("seed %s: %w", step.name, err)
		}

		logger.InfoContext(ctx, "seeded", slog.String("step", step.name))
	}

	if src.CEDICT != nil {
		if err := seedCEDICT(ctx, pool, src.CEDICT, logger); err != nil {
			return fmt.Errorf("seed cedict: %w", err)
		}
	}

	// Unihan runs after CEDICT: it only fills the reading gaps the
	// dictionary left, so the richer source must land first.
	if src.UnihanReadings != nil {
		if err := SeedUnihan(ctx, pool, src.UnihanReadings, logger); err != nil {
			return fmt.Errorf("seed unihan: %w", err)
		}
	}

	if src.StrokesTarball != nil {
		if err := seedStrokes(ctx, pool, src.StrokesTarball, logger); err != nil {
			return fmt.Errorf("seed strokes: %w", err)
		}
	}

	// Decompositions attach to the stroke rows, so stroke data must land first.
	if src.Decompositions != nil {
		if err := seedDecompositions(ctx, pool, src.Decompositions, logger); err != nil {
			return fmt.Errorf("seed decompositions: %w", err)
		}
	}

	// The canonical catalog requires both complete character sources.
	// Partial test/operator seeds keep their existing smaller surface.
	if src.CEDICT != nil && src.UnihanReadings != nil {
		if err := SeedCharacterCatalog(ctx, pool, logger); err != nil {
			return fmt.Errorf("seed character catalog: %w", err)
		}
	}

	// Frequency runs after the catalog so corpus characters can resolve onto
	// canonical learning entries.
	if src.Frequency != nil {
		if err := SeedFrequency(ctx, pool, src.Frequency, logger); err != nil {
			return fmt.Errorf("seed frequency: %w", err)
		}
	}

	if err := RankCharacterCurriculum(ctx, pool); err != nil {
		return err
	}

	return nil
}

func seedCharacters(ctx context.Context, pool *pgxpool.Pool, f Fixtures) error {
	for _, c := range f.Characters {
		radicals, err := json.Marshal(c.RadicalParts)
		if err != nil {
			return fmt.Errorf("marshal radicals: %w", err)
		}

		examples, err := json.Marshal(c.Examples)
		if err != nil {
			return fmt.Errorf("marshal examples: %w", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO characters (
				traditional, simplified, pinyin, zhuyin, pos, meaning,
				mapping_status, stroke_count, hsk_level, frequency_rank, topics,
				story, mnemonic, simplification_note, radical_parts, examples,
				siblings, starter_deck
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
			ON CONFLICT (traditional) DO UPDATE SET
				simplified = EXCLUDED.simplified, pinyin = EXCLUDED.pinyin,
				zhuyin = EXCLUDED.zhuyin, pos = EXCLUDED.pos,
				meaning = EXCLUDED.meaning, mapping_status = EXCLUDED.mapping_status,
				stroke_count = EXCLUDED.stroke_count, hsk_level = EXCLUDED.hsk_level,
				frequency_rank = EXCLUDED.frequency_rank, topics = EXCLUDED.topics,
				story = EXCLUDED.story, mnemonic = EXCLUDED.mnemonic,
				simplification_note = EXCLUDED.simplification_note,
				radical_parts = EXCLUDED.radical_parts, examples = EXCLUDED.examples,
				siblings = EXCLUDED.siblings, starter_deck = EXCLUDED.starter_deck`,
			c.Key(), c.Simplified, c.Pinyin, c.Zhuyin, c.Pos, c.Meaning,
			c.MappingStatus, c.StrokeCount, c.HskLevel, c.FrequencyRank, c.Topics,
			c.Story, c.Mnemonic, c.SimplificationNote, radicals, examples,
			c.Siblings, c.StarterDeck)
		if err != nil {
			return fmt.Errorf("upsert %s: %w", c.Key(), err)
		}
	}

	return nil
}

func seedCharPinyin(ctx context.Context, pool *pgxpool.Pool, f Fixtures) error {
	for ch, py := range f.CharPinyin {
		if _, err := pool.Exec(ctx, `
			INSERT INTO char_pinyin (ch, pinyin) VALUES ($1, $2)
			ON CONFLICT (ch) DO UPDATE SET pinyin = EXCLUDED.pinyin`, ch, py); err != nil {
			return fmt.Errorf("upsert %s: %w", ch, err)
		}
	}

	// Curated characters carry their own readings — cover both forms so
	// reader ruby works regardless of script.
	for _, c := range f.Characters {
		if c.Pinyin == "" {
			continue
		}

		for _, ch := range []string{c.Key(), c.Simplified} {
			if _, err := pool.Exec(ctx, `
				INSERT INTO char_pinyin (ch, pinyin) VALUES ($1, $2)
				ON CONFLICT (ch) DO NOTHING`, ch, c.Pinyin); err != nil {
				return fmt.Errorf("upsert %s: %w", ch, err)
			}
		}
	}

	return nil
}

func seedWords(ctx context.Context, pool *pgxpool.Pool, f Fixtures) error {
	for _, w := range f.Words {
		if _, err := pool.Exec(ctx, `
			INSERT INTO word_cards (word, pinyin, pos, meaning, simplified, traditional, story)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (word) DO UPDATE SET
				pinyin = EXCLUDED.pinyin, pos = EXCLUDED.pos, meaning = EXCLUDED.meaning,
				simplified = EXCLUDED.simplified, traditional = EXCLUDED.traditional,
				story = EXCLUDED.story`,
			w.Word, w.Pinyin, w.Pos, w.Meaning, w.Simplified, w.Traditional, w.Story); err != nil {
			return fmt.Errorf("upsert %s: %w", w.Word, err)
		}
	}

	return nil
}

func seedCompounds(ctx context.Context, pool *pgxpool.Pool, f Fixtures) error {
	for _, c := range f.Compounds {
		if _, err := pool.Exec(ctx, `
			INSERT INTO compounds (word, pinyin, chars, gloss) VALUES ($1,$2,$3,$4)
			ON CONFLICT (word) DO UPDATE SET
				pinyin = EXCLUDED.pinyin, chars = EXCLUDED.chars, gloss = EXCLUDED.gloss`,
			c.Word, c.Pinyin, c.Characters, c.Gloss); err != nil {
			return fmt.Errorf("upsert %s: %w", c.Word, err)
		}
	}

	return nil
}

func seedMilestones(ctx context.Context, pool *pgxpool.Pool, f Fixtures) error {
	for _, m := range f.Milestones {
		if _, err := pool.Exec(ctx, `
			INSERT INTO milestones (threshold, label_en, label_tc, label_sc)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (threshold) DO UPDATE SET
				label_en = EXCLUDED.label_en, label_tc = EXCLUDED.label_tc,
				label_sc = EXCLUDED.label_sc`,
			m.Threshold, m.En, m.Tc, m.Sc); err != nil {
			return fmt.Errorf("upsert %d: %w", m.Threshold, err)
		}
	}

	return nil
}

// seedBooks loads the design's library: the sample-chapter classic plus
// graded stories. Real Gutenberg texts replace the sample in Phase 4.
func seedBooks(ctx context.Context, pool *pgxpool.Pool, f Fixtures) error {
	for _, b := range f.Books {
		meta, err := json.Marshal(b.MetadataFields)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}

		if _, err := pool.Exec(ctx, `
			INSERT INTO books (
				id, title, title_english, author, script, source_format,
				cover_color, description, file_size_label, metadata_fields
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (id) DO UPDATE SET
				title = EXCLUDED.title, title_english = EXCLUDED.title_english,
				author = EXCLUDED.author, script = EXCLUDED.script,
				source_format = EXCLUDED.source_format,
				cover_color = EXCLUDED.cover_color,
				description = EXCLUDED.description,
				file_size_label = EXCLUDED.file_size_label,
				metadata_fields = EXCLUDED.metadata_fields`,
			b.ID, b.Title, b.TitleEnglish, b.Author, b.Script,
			b.SourceFormat, b.CoverColor, b.Description, b.FileSizeLabel, meta); err != nil {
			return fmt.Errorf("upsert book %s: %w", b.ID, err)
		}
	}

	// The 三國演義 sample chapter from the design.
	if _, err := pool.Exec(ctx, `
		INSERT INTO chapters (book_id, idx, title, traditional_paragraphs, simplified_paragraphs)
		VALUES ('skt', 0, $1, $2, $3)
		ON CONFLICT (book_id, idx) DO UPDATE SET
			title = EXCLUDED.title,
			traditional_paragraphs = EXCLUDED.traditional_paragraphs,
			simplified_paragraphs = EXCLUDED.simplified_paragraphs`,
		f.Passage.Title, f.Passage.TraditionalParagraphs, f.Passage.SimplifiedParagraphs); err != nil {
		return fmt.Errorf("seed passage: %w", err)
	}

	for _, s := range f.Stories {
		id := "story-" + s.ID

		if _, err := pool.Exec(ctx, `
			INSERT INTO books (id, title, title_english, script, source_format,
				cover_color, graded_story, level_label, char_count, description)
			VALUES ($1,$2,$3,'traditional','story',$4,TRUE,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE SET
				title = EXCLUDED.title, level_label = EXCLUDED.level_label,
				char_count = EXCLUDED.char_count, description = EXCLUDED.description`,
			id, s.Title, s.Blurb.En, "#2f7a62", s.LevelLabel, s.CharCount, s.Blurb.Tc); err != nil {
			return fmt.Errorf("upsert story %s: %w", s.ID, err)
		}

		if _, err := pool.Exec(ctx, `
			INSERT INTO chapters (book_id, idx, title, traditional_paragraphs, simplified_paragraphs)
			VALUES ($1, 0, $2, $3, $4)
			ON CONFLICT (book_id, idx) DO UPDATE SET
				title = EXCLUDED.title,
				traditional_paragraphs = EXCLUDED.traditional_paragraphs,
				simplified_paragraphs = EXCLUDED.simplified_paragraphs`,
			id, s.Title, s.TraditionalParagraphs, s.SimplifiedParagraphs); err != nil {
			return fmt.Errorf("seed story chapter %s: %w", s.ID, err)
		}
	}

	return nil
}

// seedCEDICT atomically replaces dictionary entries from the pinned source,
// so an interrupted or partial seed always repairs itself on the next run.
func seedCEDICT(ctx context.Context, pool *pgxpool.Pool, r io.Reader, logger *slog.Logger) error {
	entries, err := ParseCEDICT(r)
	if err != nil {
		return err
	}

	rows := make([][]any, len(entries))
	for i, e := range entries {
		rows[i] = []any{e.Traditional, e.Simplified, e.Pinyin, e.Definitions}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin dictionary replace: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "TRUNCATE dict_entries RESTART IDENTITY"); err != nil {
		return fmt.Errorf("reset dictionary: %w", err)
	}

	if _, err := tx.CopyFrom(ctx,
		pgx.Identifier{"dict_entries"},
		[]string{columnTraditional, columnSimplified, columnPinyin, "definitions"},
		pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy entries: %w", err)
	}

	// Fill the per-character pinyin fallback from single-character entries
	// (authored fixtures win — they were inserted first).
	if _, err := tx.Exec(ctx, `
		INSERT INTO char_pinyin (ch, pinyin)
		SELECT DISTINCT ON (traditional) traditional, pinyin
		FROM dict_entries
		WHERE char_length(traditional) = 1
		ORDER BY traditional, id
		ON CONFLICT (ch) DO NOTHING`); err != nil {
		return fmt.Errorf("char pinyin fallback: %w", err)
	}

	// Mirror simplified single characters that differ from traditional.
	if _, err := tx.Exec(ctx, `
		INSERT INTO char_pinyin (ch, pinyin)
		SELECT DISTINCT ON (simplified) simplified, pinyin
		FROM dict_entries
		WHERE char_length(simplified) = 1
		ORDER BY simplified, id
		ON CONFLICT (ch) DO NOTHING`); err != nil {
		return fmt.Errorf("char pinyin simplified fallback: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit dictionary replace: %w", err)
	}

	logger.InfoContext(ctx, "seeded cedict", slog.Int("entries", len(entries)))

	return nil
}

// seedStrokes bulk-loads hanzi-writer stroke data. Existing rows are
// preserved; missing rows and pre-outlines rows are repaired in place.
func seedStrokes(ctx context.Context, pool *pgxpool.Pool, tgz io.Reader, logger *slog.Logger) error {
	var total int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stroke_data").Scan(&total); err != nil {
		return fmt.Errorf("count stroke_data: %w", err)
	}

	chars, licence, err := ExtractStrokeData(tgz)
	if err != nil {
		return err
	}

	if total == 0 {
		rows := make([][]any, 0, len(chars))

		for _, c := range chars {
			medians, err := json.Marshal(c.Medians)
			if err != nil {
				return fmt.Errorf("marshal medians %s: %w", c.Char, err)
			}

			rows = append(rows, []any{c.Char, medians, len(c.Medians), []byte(c.Data)})
		}

		if _, err := pool.CopyFrom(ctx,
			pgx.Identifier{"stroke_data"},
			[]string{"ch", "medians", "stroke_count", "data"},
			pgx.CopyFromRows(rows)); err != nil {
			return fmt.Errorf("copy stroke_data: %w", err)
		}

		logger.InfoContext(ctx, "seeded stroke data",
			slog.Int("characters", len(chars)),
			slog.Int("licence_bytes", len(licence)))

		return nil
	}

	// Pre-outlines rows exist: upsert the full JSON per character without
	// touching the medians columns, and add characters missing entirely.
	backfilled := 0

	for _, c := range chars {
		medians, err := json.Marshal(c.Medians)
		if err != nil {
			return fmt.Errorf("marshal medians %s: %w", c.Char, err)
		}

		tag, err := pool.Exec(ctx, `
			INSERT INTO stroke_data (ch, medians, stroke_count, data)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (ch) DO UPDATE SET data = EXCLUDED.data
			WHERE stroke_data.data IS NULL`,
			c.Char, medians, len(c.Medians), []byte(c.Data))
		if err != nil {
			return fmt.Errorf("backfill %s: %w", c.Char, err)
		}

		backfilled += int(tag.RowsAffected())
	}

	logger.InfoContext(ctx, "backfilled stroke outlines",
		slog.Int("characters", len(chars)),
		slog.Int("backfilled", backfilled))

	return nil
}
