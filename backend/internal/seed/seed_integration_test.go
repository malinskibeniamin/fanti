package seed_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

const sampleCEDICT = "# CC-CEDICT sample\n" +
	"傳統 传统 [chuan2 tong3] /tradition/convention/\n" +
	"馬 马 [ma3] /horse/CL:匹[pi3]/\n" +
	"茶 茶 [cha2] /tea/tea plant/\n"

// fakeStrokesTarball builds an npm-style tgz with a licence and one char.
func fakeStrokesTarball(t *testing.T) io.Reader {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	write := func(name, content string) {
		t.Helper()

		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatalf("tar header: %v", err)
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}

	write("package/ARPHICPL.TXT", "ARPHIC PUBLIC LICENSE ...")
	write("package/package.json", `{"name":"hanzi-writer-data"}`)
	write("package/馬.json", `{"medians":[[[100,200],[300,400]],[[500,600],[700,800]]]}`)

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return &buf
}

func TestIntegrationSeedRunTwiceIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	run := func() {
		t.Helper()

		err := seed.Run(ctx, pool, seed.Sources{
			CEDICT: strings.NewReader(sampleCEDICT),
			Decompositions: strings.NewReader(
				`{"character":"馬","decomposition":"⿹𠂉灬"}` + "\n",
			),
			StrokesTarball: fakeStrokesTarball(t),
		}, logger)
		if err != nil {
			t.Fatalf("seed.Run() error = %v", err)
		}
	}

	run()
	run() // idempotency

	counts := map[string]int64{
		"characters":   28,
		"word_cards":   8,
		"compounds":    5,
		"milestones":   8,
		"books":        7, // 5 classics + 2 graded stories
		"dict_entries": 3,
		"stroke_data":  1,
	}

	for table, want := range counts {
		var got int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}

		if got != want {
			t.Errorf("%s count = %d, want %d", table, got, want)
		}
	}

	// The ambiguous 髮 fixture must be keyed by its first form only.
	var simplified string
	if err := pool.QueryRow(ctx,
		"SELECT simplified FROM characters WHERE traditional = '髮'").Scan(&simplified); err != nil {
		t.Fatalf("髮 row: %v", err)
	}

	if simplified != "发" {
		t.Errorf("髮 simplified = %q, want 发", simplified)
	}

	// char_pinyin: authored fixture rows plus CEDICT single-char fallback (茶, 马).
	var py string
	if err := pool.QueryRow(ctx,
		"SELECT pinyin FROM char_pinyin WHERE ch = '茶'").Scan(&py); err != nil {
		t.Fatalf("茶 pinyin: %v", err)
	}

	if py != "chá" {
		t.Errorf("茶 pinyin = %q, want chá", py)
	}

	// The sample chapter must be readable.
	var paras []string
	if err := pool.QueryRow(ctx,
		"SELECT traditional_paragraphs FROM chapters WHERE book_id = 'skt'").Scan(&paras); err != nil {
		t.Fatalf("skt chapter: %v", err)
	}

	if len(paras) != 3 {
		t.Errorf("skt paragraphs = %d, want 3", len(paras))
	}

	var strokeParts []byte
	if err := pool.QueryRow(ctx,
		"SELECT radical_parts FROM stroke_data WHERE ch = '馬'").Scan(&strokeParts); err != nil {
		t.Fatalf("馬 stroke decomposition: %v", err)
	}
	if !strings.Contains(string(strokeParts), "灬") {
		t.Errorf("馬 stroke decomposition = %s, want seeded parts", strokeParts)
	}
}
