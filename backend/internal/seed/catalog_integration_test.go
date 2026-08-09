package seed

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationCharacterCatalog(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := Run(ctx, pool, Sources{}, logger); err != nil {
		t.Fatalf("seed authored fixtures: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO dict_entries (traditional, simplified, pinyin, definitions) VALUES
			('馬', '马', 'mǎ', ARRAY['horse from source']),
			('鬱', '郁', 'yù', ARRAY['depressed', 'luxuriant']),
			('鬱', '郁', 'yù', ARRAY['dense', 'luxuriant']),
			('發', '发', 'fā', ARRAY['to send out']),
			('髮', '发', 'fà', ARRAY['hair']);
		INSERT INTO char_pinyin (ch, pinyin) VALUES
			('马', 'mǎ'),
			('𠮷', 'jí'),
			('喵', 'miāo')
		ON CONFLICT (ch) DO UPDATE SET pinyin = EXCLUDED.pinyin;
		INSERT INTO characters (traditional, simplified, pinyin)
		VALUES ('喵', '喵', 'miāo')
		ON CONFLICT (traditional) DO NOTHING;
		INSERT INTO stroke_data (ch, medians, stroke_count, data)
		VALUES ('鬱', '[]', 29, '{"strokes":[],"medians":[]}')
		ON CONFLICT (ch) DO NOTHING`); err != nil {
		t.Fatalf("insert source rows: %v", err)
	}

	run := func() {
		t.Helper()

		if err := SeedCharacterCatalog(ctx, pool, logger); err != nil {
			t.Fatalf("SeedCharacterCatalog: %v", err)
		}
		if err := RankCharacterCurriculum(ctx, pool); err != nil {
			t.Fatalf("RankCharacterCurriculum: %v", err)
		}
	}

	run()
	run()

	type catalogRow struct {
		simplified     string
		pinyin         string
		meaning        string
		catalogKind    string
		strokeCount    int
		curriculumRank int
	}

	get := func(traditional string) catalogRow {
		t.Helper()

		var row catalogRow
		if err := pool.QueryRow(ctx, `
			SELECT simplified, pinyin, meaning, catalog_kind, stroke_count, curriculum_rank
			FROM characters WHERE traditional = $1`, traditional).Scan(
			&row.simplified, &row.pinyin, &row.meaning, &row.catalogKind,
			&row.strokeCount, &row.curriculumRank,
		); err != nil {
			t.Fatalf("get %s: %v", traditional, err)
		}

		return row
	}

	yu := get("鬱")
	if yu.simplified != "郁" || yu.pinyin != "yù" ||
		yu.meaning != "depressed; luxuriant" ||
		yu.catalogKind != catalogKindCurriculum || yu.strokeCount != 29 ||
		yu.curriculumRank <= 0 {
		t.Errorf("鬱 = %+v, want a ranked curriculum entry", yu)
	}

	reference := get("𠮷")
	if reference.simplified != "𠮷" || reference.pinyin != "jí" ||
		reference.meaning != "" || reference.catalogKind != catalogKindReference ||
		reference.curriculumRank != 0 {
		t.Errorf("𠮷 = %+v, want an unranked reference entry", reference)
	}

	existingReference := get("喵")
	if existingReference.catalogKind != catalogKindReference ||
		existingReference.curriculumRank != 0 {
		t.Errorf("existing empty-meaning 喵 = %+v, want reference entry", existingReference)
	}

	// Source sync must not overwrite authored learning content.
	var horseStory, horseMeaning string
	if err := pool.QueryRow(ctx,
		"SELECT story, meaning FROM characters WHERE traditional = '馬'").
		Scan(&horseStory, &horseMeaning); err != nil {
		t.Fatalf("get authored 馬: %v", err)
	}
	if horseStory == "" || horseMeaning == "horse from source" {
		t.Errorf("authored 馬 was overwritten: story=%q meaning=%q", horseStory, horseMeaning)
	}

	var faRows int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM characters WHERE traditional IN ('發', '髮')").
		Scan(&faRows); err != nil {
		t.Fatalf("count ambiguous mappings: %v", err)
	}
	if faRows != 2 {
		t.Errorf("ambiguous 发 mappings = %d, want 2 separate entries", faRows)
	}

	var duplicateReference int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM characters WHERE traditional = '马'").
		Scan(&duplicateReference); err != nil {
		t.Fatalf("count related simplified form: %v", err)
	}
	if duplicateReference != 0 {
		t.Error("related simplified form 马 became a standalone reference entry")
	}
}

func TestIntegrationVendoredCharacterCatalogIsComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	cedictFile, err := os.Open("../../data/downloads/cedict.txt.gz")
	if err != nil {
		t.Fatalf("open CEDICT: %v", err)
	}
	defer func() { _ = cedictFile.Close() }()

	cedict, err := gzip.NewReader(cedictFile)
	if err != nil {
		t.Fatalf("open CEDICT gzip: %v", err)
	}
	defer func() { _ = cedict.Close() }()

	unihan, err := zip.OpenReader("../../data/downloads/unihan.zip")
	if err != nil {
		t.Fatalf("open Unihan: %v", err)
	}
	defer func() { _ = unihan.Close() }()

	readings, err := unihan.Open("Unihan_Readings.txt")
	if err != nil {
		t.Fatalf("open Unihan readings: %v", err)
	}
	defer func() { _ = readings.Close() }()

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := Run(ctx, pool, Sources{
		CEDICT:         cedict,
		UnihanReadings: readings,
	}, logger); err != nil {
		t.Fatalf("seed full character catalog: %v", err)
	}

	var total, curriculum, reference, ranked int
	if err := pool.QueryRow(ctx, `
		SELECT count(*),
			count(*) FILTER (WHERE catalog_kind = 'curriculum'),
			count(*) FILTER (WHERE catalog_kind = 'reference'),
			count(*) FILTER (WHERE curriculum_rank > 0)
		FROM characters`).Scan(&total, &curriculum, &reference, &ranked); err != nil {
		t.Fatalf("count catalog: %v", err)
	}

	if total != 41710 || curriculum != 11709 || reference != 30001 ||
		ranked != curriculum {
		t.Errorf(
			"catalog total/curriculum/reference/ranked = %d/%d/%d/%d, want 41710/11709/30001/11709",
			total,
			curriculum,
			reference,
			ranked,
		)
	}

	var senses, traditionalForms, simplifiedForms int
	if err := pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (
				WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
			),
			count(DISTINCT traditional) FILTER (
				WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
			),
			count(DISTINCT simplified) FILTER (
				WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
			)
		FROM dict_entries`).Scan(&senses, &traditionalForms, &simplifiedForms); err != nil {
		t.Fatalf("count CEDICT forms: %v", err)
	}

	if senses != 13532 || traditionalForms != 11709 || simplifiedForms != 10793 {
		t.Errorf(
			"CEDICT senses/traditional/simplified = %d/%d/%d, want 13532/11709/10793",
			senses,
			traditionalForms,
			simplifiedForms,
		)
	}

	var coveredGlyphs int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT traditional AS glyph FROM characters
			UNION
			SELECT simplified AS glyph FROM characters
			UNION
			SELECT traditional AS glyph FROM dict_entries
			WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
			UNION
			SELECT simplified AS glyph FROM dict_entries
			WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
		) AS covered_glyphs`).Scan(&coveredGlyphs); err != nil {
		t.Fatalf("count covered glyphs: %v", err)
	}
	if coveredGlyphs != 44402 {
		t.Errorf("covered glyphs = %d, want 44402", coveredGlyphs)
	}
}
