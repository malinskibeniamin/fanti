package seed

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/internal/convert"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

// fakeTatoebaTSV covers a natively-simplified sentence, a natively-
// traditional one, a phrase-ambiguous conversion (头发 must become 頭髮,
// not 頭發), a row with no Han characters at all, a sentence whose 干
// has several traditional forms — a conversion OpenCC can get wrong — a
// mixed-script sentence that detects as traditional yet still carries a
// convertible 干, and community rows that must never enter the table.
const fakeTatoebaTSV = "10\t" + sleepSimplified + "\t" + sleepEnglish + "\n" +
	"11\t我們試試看！\tLet's try it.\n" +
	"12\t我的头发很长。\tMy hair is long.\n" +
	"13\tOK 123!\tOK 123!\n" +
	"14\t你干得很好。\tYou did a good job.\n" +
	"15\t你在干什麼啊？\tWhat are you doing?\n" +
	"16\t他媽的！\tDamn it!\n" +
	"1531795\t愛喜歡愛。\tLove loves love.\n" +
	"1531796\t愛愛愛。\tLove loves love.\n"

// seedAmbiguityDict registers 干's one-to-many traditional forms so the
// seed can flag risky conversions.
func seedAmbiguityDict(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	for _, row := range [][2]string{{"乾", "干"}, {"幹", "干"}, {"干", "干"}} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO dict_entries (traditional, simplified, pinyin, definitions)
			VALUES ($1, $2, '', '{}')`, row[0], row[1]); err != nil {
			t.Fatalf("insert dict %s: %v", row[0], err)
		}
	}
}

func seedTatoebaForTest(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	engine, err := convert.NewEngine()
	if err != nil {
		t.Fatalf("convert.NewEngine() error = %v", err)
	}

	logger := slog.New(slog.DiscardHandler)

	run := func() {
		t.Helper()

		if err := SeedTatoeba(ctx, pool, engine, strings.NewReader(fakeTatoebaTSV), logger); err != nil {
			t.Fatalf("SeedTatoeba() error = %v", err)
		}
	}

	run()
	run() // idempotency: the second pass must skip, not duplicate
}

func TestIntegrationTatoeba(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()

	seedAmbiguityDict(ctx, t, pool)
	seedTatoebaForTest(ctx, t, pool)

	var total int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM sentences").Scan(&total); err != nil {
		t.Fatalf("count sentences: %v", err)
	}

	if total != 5 {
		t.Errorf("sentences = %d, want 5 (unfit rows excluded, no duplicates)", total)
	}

	// Sentence 14's 干 maps to several traditional forms, and sentence
	// 15 keeps a convertible 干 even though it detects as traditional —
	// both must carry the ambiguous flag; the safe sentence 10 must not.
	assertAmbiguous := func(id int64, want bool) {
		t.Helper()

		var got bool
		if err := pool.QueryRow(ctx,
			"SELECT ambiguous FROM sentences WHERE id = $1", id).Scan(&got); err != nil {
			t.Fatalf("ambiguous %d: %v", id, err)
		}

		if got != want {
			t.Errorf("sentence %d ambiguous = %v, want %v", id, got, want)
		}
	}

	assertAmbiguous(14, true)
	assertAmbiguous(15, true)
	assertAmbiguous(10, false)

	// The profane row is dropped at seed time — the speakable summary
	// queries sentences directly, so storage is already too far.
	var denied int64
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM sentences WHERE id = 16").Scan(&denied); err != nil {
		t.Fatalf("denied count: %v", err)
	}

	if denied != 0 {
		t.Error("profane sentence 16 was stored, want dropped at seed time")
	}

	var lowQuality int64
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM sentences WHERE id IN (1531795, 1531796)").Scan(&lowQuality); err != nil {
		t.Fatalf("low-quality count: %v", err)
	}

	if lowQuality != 0 {
		t.Error("known low-quality sentences were stored, want dropped at seed time")
	}

	type row struct {
		traditional string
		simplified  string
		english     string
		chars       []string
		charCount   int
	}

	get := func(id int64) row {
		t.Helper()

		var s row
		if err := pool.QueryRow(ctx, `
			SELECT traditional, simplified, english, chars, char_count
			FROM sentences WHERE id = $1`, id).Scan(
			&s.traditional, &s.simplified, &s.english, &s.chars, &s.charCount); err != nil {
			t.Fatalf("sentence %d: %v", id, err)
		}

		return s
	}

	// Natively simplified: source verbatim in simplified, converted traditional.
	simp := get(10)
	if simp.simplified != sleepSimplified || simp.traditional != sleepTraditional {
		t.Errorf("sentence 10 = %+v, want converted traditional %s", simp, sleepTraditional)
	}

	if simp.charCount != 6 || len(simp.chars) != 6 {
		t.Errorf("sentence 10 counts = %d/%d, want 6 distinct of 6", simp.charCount, len(simp.chars))
	}

	// Natively traditional: source verbatim in traditional.
	trad := get(11)
	if trad.traditional != "我們試試看！" || trad.simplified != "我们试试看！" {
		t.Errorf("sentence 11 = %+v, want verbatim traditional", trad)
	}

	// 试 appears twice but chars are distinct; char_count keeps both.
	if trad.charCount != 5 || len(trad.chars) != 4 {
		t.Errorf("sentence 11 counts = %d distinct %d, want 5 and 4", trad.charCount, len(trad.chars))
	}

	// Phrase-level conversion must resolve the ambiguous 发 to 髮 (hair),
	// not 發 (to emit).
	hair := get(12)
	if !strings.Contains(hair.traditional, "頭髮") {
		t.Errorf("sentence 12 traditional = %q, want 頭髮 (not 頭發)", hair.traditional)
	}
}
