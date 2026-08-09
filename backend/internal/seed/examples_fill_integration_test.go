package seed

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/internal/convert"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

// fillTatoebaTSV: 該 appears in two sentences (the shorter, all-common one
// must win); 罵 appears only in a profane sentence (must stay unfilled);
// 馬 appears in a sentence but the curated fixture already covers 馬.
const fillTatoebaTSV = "20\t" + sleepSimplified + "\t" + sleepEnglish + "\n" +
	"21\t你应该更加小心谨慎才对。\tYou should be more careful.\n" +
	"22\t他妈的，别骂我！\tDamn it, don't insult me!\n" +
	"23\t我有一匹马。\tI have a horse.\n"

type storedExample struct {
	HskLevel int    `json:"hskLevel"`
	Chinese  string `json:"chinese"`
	English  string `json:"english"`
	Source   string `json:"source"`
}

func getExamples(ctx context.Context, t *testing.T, pool *pgxpool.Pool, ch string) []storedExample {
	t.Helper()

	var raw []byte
	if err := pool.QueryRow(ctx,
		"SELECT examples FROM characters WHERE traditional = $1", ch).Scan(&raw); err != nil {
		t.Fatalf("examples %s: %v", ch, err)
	}

	var examples []storedExample
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatalf("decode examples %s: %v", ch, err)
	}

	return examples
}

func TestIntegrationFillExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	// Curated fixtures (馬 carries authored examples), plus bare rows the
	// fill must target.
	if err := Run(ctx, pool, Sources{}, logger); err != nil {
		t.Fatalf("seed.Run() error = %v", err)
	}

	for _, row := range [][2]string{{"該", "该"}, {"罵", "骂"}} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO characters (traditional, simplified) VALUES ($1, $2)
			ON CONFLICT (traditional) DO NOTHING`, row[0], row[1]); err != nil {
			t.Fatalf("insert %s: %v", row[0], err)
		}
	}

	engine, err := convert.NewEngine()
	if err != nil {
		t.Fatalf("convert.NewEngine() error = %v", err)
	}

	if err := SeedTatoeba(ctx, pool, engine, strings.NewReader(fillTatoebaTSV), logger); err != nil {
		t.Fatalf("SeedTatoeba() error = %v", err)
	}

	curatedBefore := getExamples(ctx, t, pool, "馬")
	if len(curatedBefore) == 0 {
		t.Fatal("fixture 馬 has no examples; test premise broken")
	}

	fill := func(force bool) {
		t.Helper()

		if err := FillExamples(ctx, pool, force, logger); err != nil {
			t.Fatalf("FillExamples(force=%v) error = %v", force, err)
		}
	}

	fill(false)

	// 該: both sentences qualify; the shorter one leads, traditional
	// script, provenance marked.
	gai := getExamples(ctx, t, pool, "該")
	if len(gai) != 2 {
		t.Fatalf("該 examples = %d, want 2", len(gai))
	}

	if gai[0].Chinese != sleepTraditional || gai[0].Source != "tatoeba" || gai[0].HskLevel != 0 {
		t.Errorf("該 first example = %+v, want shorter traditional sentence marked tatoeba", gai[0])
	}

	// 罵: its only sentence is denied content — must stay empty.
	if ma := getExamples(ctx, t, pool, "罵"); len(ma) != 0 {
		t.Errorf("罵 examples = %v, want none (denylist)", ma)
	}

	// Curated 馬 untouched even though sentence 23 contains 馬.
	if got := getExamples(ctx, t, pool, "馬"); len(got) != len(curatedBefore) || got[0].Chinese != curatedBefore[0].Chinese {
		t.Errorf("馬 examples changed: %v -> %v", curatedBefore, got)
	}

	// Second plain run: no churn.
	fill(false)

	if again := getExamples(ctx, t, pool, "該"); len(again) != 2 || again[0].Chinese != gai[0].Chinese {
		t.Errorf("plain rerun changed 該: %v", again)
	}

	// Force re-pick refreshes auto-filled rows but never curated ones.
	fill(true)

	if forced := getExamples(ctx, t, pool, "該"); len(forced) != 2 || forced[0].Source != "tatoeba" {
		t.Errorf("forced rerun broke 該: %v", forced)
	}

	if got := getExamples(ctx, t, pool, "馬"); got[0].Chinese != curatedBefore[0].Chinese {
		t.Errorf("forced rerun touched curated 馬: %v", got)
	}
}
