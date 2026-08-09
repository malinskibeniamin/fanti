package seed

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

// fakeFrequencyCorpus covers the interesting cases: curated rows (馬, 髮),
// an ambiguous non-curated simplified form (面), a plain CEDICT-backed
// character (猫), and one absent from CEDICT entirely (喵).
const fakeFrequencyCorpus = `1	的的的的的的的的的的	de
2	一一一一一一一一一	yi
3	发发发发发发发发	fa
4	马马马马马马马	ma
5	面面面面面面	mian
6	猫猫猫猫猫	mao
7	喵喵喵喵	miao
8	是是是	shi
9	不不	bu
10	茶	cha
`

// frequencyDictFixtures are single-character dict_entries rows the plan
// resolves against, in deliberate id order (面 before 麵 so 面 is chosen).
var frequencyDictFixtures = [][]string{
	{"的", "的", "de", "of|~'s (possessive particle)|really and truly"},
	{"一", "一", "yī", "one|single"},
	{"發", "发", "fā", "to send out|to issue"},
	{"髮", "发", "fà", glossHair},
	{"馬", "马", "mǎ", "horse|CL:匹[pǐ]"},
	{"面", "面", pinyinMian, "face|side|surface"},
	{"麵", "面", pinyinMian, "flour|noodles"},
	{"貓", "猫", pinyinMao, "cat|CL:隻[zhī]"},
	{"是", "是", "shì", "to be|yes"},
	{"不", "不", "bù", "no|not"},
	{"茶", "茶", "chá", "tea|tea plant"},
}

func seedFrequencyFixtures(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	for _, row := range frequencyDictFixtures {
		if _, err := pool.Exec(ctx, `
			INSERT INTO dict_entries (traditional, simplified, pinyin, definitions)
			VALUES ($1, $2, $3, $4)`,
			row[0], row[1], row[2], strings.Split(row[3], "|")); err != nil {
			t.Fatalf("insert dict fixture %s: %v", row[0], err)
		}
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO stroke_data (ch, medians, stroke_count) VALUES ('貓', '[]', 15)`); err != nil {
		t.Fatalf("insert stroke fixture: %v", err)
	}
}

func TestIntegrationFrequency(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	// Curated fixtures first (the 28 authored characters, 馬 included).
	if err := Run(ctx, pool, Sources{}, logger); err != nil {
		t.Fatalf("seed.Run() error = %v", err)
	}

	seedFrequencyFixtures(ctx, t, pool)

	run := func() {
		t.Helper()

		if err := SeedFrequency(ctx, pool, strings.NewReader(fakeFrequencyCorpus), logger); err != nil {
			t.Fatalf("SeedFrequency() error = %v", err)
		}
	}

	run()

	if _, err := pool.Exec(ctx, `
		INSERT INTO characters (traditional, simplified, frequency_rank)
		VALUES ('舊', '旧', 999);
		INSERT INTO sentences (id, traditional, simplified, english, chars, max_freq_rank)
		VALUES
			(9001, '的一', '的一', 'ranked', ARRAY['的', '一'], 777),
			(9002, '的舊', '的旧', 'unranked', ARRAY['的', '舊'], 777)`); err != nil {
		t.Fatalf("insert legacy frequency row: %v", err)
	}

	run() // a second pass must remove stale ranks without duplicating or reshuffling rows

	type charRow struct {
		traditional   string
		simplified    string
		pinyin        string
		meaning       string
		mappingStatus string
		strokeCount   int
		rank          int
		story         string
		siblings      []string
	}

	get := func(traditional string) charRow {
		t.Helper()

		var c charRow
		if err := pool.QueryRow(ctx, `
			SELECT traditional, simplified, pinyin, meaning, mapping_status,
				stroke_count, frequency_rank, story, siblings
			FROM characters WHERE traditional = $1`, traditional).Scan(
			&c.traditional, &c.simplified, &c.pinyin, &c.meaning, &c.mappingStatus,
			&c.strokeCount, &c.rank, &c.story, &c.siblings); err != nil {
			t.Fatalf("row %s: %v", traditional, err)
		}

		return c
	}

	if legacy := get("舊"); legacy.rank != 0 {
		t.Errorf("舊 stale rank = %d, want 0", legacy.rank)
	}

	for id, want := range map[int64]int{9001: 2, 9002: 0} {
		var rank int
		if err := pool.QueryRow(ctx,
			"SELECT max_freq_rank FROM sentences WHERE id = $1", id).Scan(&rank); err != nil {
			t.Fatalf("sentence %d rank: %v", id, err)
		}
		if rank != want {
			t.Errorf("sentence %d rank = %d, want %d", id, rank, want)
		}
	}

	// Curated 馬: rank updated to the real one, authored story preserved.
	ma := get("馬")
	if ma.rank != 4 {
		t.Errorf("馬 rank = %d, want 4", ma.rank)
	}

	if !strings.Contains(ma.story, "oracle-bone") {
		t.Errorf("馬 story was not preserved: %q", ma.story)
	}

	// Curated ambiguous 髮 covers simplified 发: rank lands on it, its
	// authored mapping and siblings survive, and no new 發 row appears.
	fa := get("髮")
	if fa.rank != 3 {
		t.Errorf("髮 rank = %d, want 3", fa.rank)
	}

	if fa.mappingStatus != mappingAmbiguous || len(fa.siblings) != 1 || fa.siblings[0] != "發" {
		t.Errorf("髮 mapping = %q siblings = %v, want ambiguous [發]", fa.mappingStatus, fa.siblings)
	}

	var faRows int64
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM characters WHERE simplified = '发'").Scan(&faRows); err != nil {
		t.Fatalf("count 发 rows: %v", err)
	}

	if faRows != 1 {
		t.Errorf("发 rows = %d, want 1 (no duplicate 發 insert)", faRows)
	}

	// New ambiguous 面: two traditional forms share the simplified form.
	mian := get("面")
	if mian.mappingStatus != mappingAmbiguous || len(mian.siblings) != 1 || mian.siblings[0] != "麵" {
		t.Errorf("面 mapping = %q siblings = %v, want ambiguous [麵]", mian.mappingStatus, mian.siblings)
	}

	if mian.rank != 5 || mian.meaning != "face; side" {
		t.Errorf("面 rank = %d meaning = %q, want 5 %q", mian.rank, mian.meaning, "face; side")
	}

	// New CEDICT-backed 貓: traditional resolved, gloss and strokes filled.
	mao := get("貓")
	if mao.simplified != "猫" || mao.pinyin != pinyinMao || mao.mappingStatus != mappingExact {
		t.Errorf("貓 = %+v, want simplified 猫, pinyin māo, exact", mao)
	}

	if mao.rank != 6 || mao.strokeCount != 15 || mao.meaning != "cat; CL:隻[zhī]" {
		t.Errorf("貓 rank = %d strokes = %d meaning = %q", mao.rank, mao.strokeCount, mao.meaning)
	}

	// 喵 is absent from CEDICT: the character itself is the row key.
	miao := get("喵")
	if miao.simplified != "喵" || miao.mappingStatus != mappingExact || miao.rank != 7 {
		t.Errorf("喵 = %+v, want itself as key with rank 7", miao)
	}

	// Curated 茶 keeps its identity but carries the real rank.
	if cha := get("茶"); cha.rank != 10 {
		t.Errorf("茶 rank = %d, want 10", cha.rank)
	}
}
