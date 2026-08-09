package seed

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

// makeStrokeTarball builds a minimal hanzi-writer-data npm tarball: the
// licence plus per-character JSON files with outlines and medians.
func makeStrokeTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	write := func(name, content string) {
		t.Helper()

		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(content)),
		}); err != nil {
			t.Fatalf("tar header %s: %v", name, err)
		}

		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}

	write("package/ARPHICPL.TXT", "ARPHIC PUBLIC LICENSE")

	for name, content := range files {
		write("package/"+name, content)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}

	return buf.Bytes()
}

const (
	zhongJSON = `{"strokes":["M 100 200 L 300 400"],"medians":[[[100,200],[300,400]]]}`
	haoJSON   = `{"strokes":["M 10 20 L 30 40","M 50 60 L 70 80"],"medians":[[[10,20],[30,40]],[[50,60],[70,80]]]}`
)

func TestIntegrationStrokesBackfill(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	// A row seeded before migration 00003: medians present, data NULL.
	if _, err := pool.Exec(ctx, `
		INSERT INTO stroke_data (ch, medians, stroke_count)
		VALUES ('中', '[[[100,200],[300,400]]]', 1)`); err != nil {
		t.Fatalf("insert pre-migration row: %v", err)
	}

	tarball := makeStrokeTarball(t, map[string]string{
		"中.json": zhongJSON,
		"好.json": haoJSON,
	})

	run := func() {
		t.Helper()

		if err := seedStrokes(ctx, pool, bytes.NewReader(tarball), logger); err != nil {
			t.Fatalf("seedStrokes() error = %v", err)
		}
	}

	run()
	run() // second pass must not duplicate or rewrite

	type row struct {
		data        string
		strokeCount int
	}

	get := func(ch string) row {
		t.Helper()

		var r row
		if err := pool.QueryRow(ctx,
			"SELECT COALESCE(data::text, ''), stroke_count FROM stroke_data WHERE ch = $1", ch).
			Scan(&r.data, &r.strokeCount); err != nil {
			t.Fatalf("row %s: %v", ch, err)
		}

		return r
	}

	// Pre-existing row: data backfilled, medians-era fields untouched.
	zhong := get("中")
	if !strings.Contains(zhong.data, "strokes") {
		t.Errorf("中 data = %q, want backfilled outlines", zhong.data)
	}

	if zhong.strokeCount != 1 {
		t.Errorf("中 stroke_count = %d, want 1 (untouched)", zhong.strokeCount)
	}

	// New character: full row inserted with data.
	hao := get("好")
	if !strings.Contains(hao.data, "strokes") || hao.strokeCount != 2 {
		t.Errorf("好 = %+v, want fresh row with outlines and 2 strokes", hao)
	}

	var total int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stroke_data").Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}

	if total != 2 {
		t.Errorf("stroke_data rows = %d, want 2 (no duplicates)", total)
	}
}
