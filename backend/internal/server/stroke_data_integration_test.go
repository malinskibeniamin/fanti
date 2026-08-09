package server_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationGetStrokeData(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()

	const zhongData = `{"strokes": ["M 100 200 L 300 400"], "medians": [[[100, 200], [300, 400]]]}`

	if _, err := pool.Exec(ctx, `
		INSERT INTO stroke_data (ch, medians, stroke_count, data)
		VALUES ('中', '[[[100,200],[300,400]]]', 1, $1)`, zhongData); err != nil {
		t.Fatalf("insert stroke row: %v", err)
	}

	// A pre-backfill row: medians only, data NULL — must read as absent.
	if _, err := pool.Exec(ctx, `
		INSERT INTO stroke_data (ch, medians, stroke_count)
		VALUES ('好', '[[[10,20],[30,40]]]', 1)`); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	handler, err := server.NewHandler(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := fantiv1connect.NewDictionaryServiceClient(http.DefaultClient, srv.URL)

	get := func(name string) (*fantiv1.GetStrokeDataResponse, error) {
		t.Helper()

		res, err := client.GetStrokeData(ctx,
			connect.NewRequest(&fantiv1.GetStrokeDataRequest{Name: name}))
		if err != nil {
			return nil, err
		}

		return res.Msg, nil
	}

	// Present: the raw JSON comes back verbatim.
	got, err := get("characters/中")
	if err != nil {
		t.Fatalf("GetStrokeData(中): %v", err)
	}

	if !strings.Contains(got.GetData(), `"strokes"`) {
		t.Errorf("data = %q, want raw hanzi-writer JSON", got.GetData())
	}

	// Absent row and NULL-data row both read as clean NOT_FOUND.
	for _, name := range []string{"characters/貓", "characters/好"} {
		_, err := get(name)

		var connectErr *connect.Error
		if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeNotFound {
			t.Errorf("GetStrokeData(%s) error = %v, want NOT_FOUND", name, err)
		}
	}

	// Malformed resource name.
	if _, err := get("中"); err == nil {
		t.Error("GetStrokeData(bare id) expected INVALID_ARGUMENT, got nil")
	}
}

func TestIntegrationGetCharacterUsesSeededDecompositionWithoutReplacingCuratedParts(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO dict_entries (traditional, simplified, pinyin, definitions)
		VALUES ('俢', '俢', 'xiū', ARRAY['repair']);
		INSERT INTO stroke_data (ch, medians, stroke_count, radical_parts)
		VALUES
			('俢', '[]', 3, '[{"part":"亻","note":""},{"part":"夂","note":""},{"part":"彡","note":""}]'),
			('馬', '[]', 10, '[{"part":"𠂉","note":""},{"part":"灬","note":""}]');
		INSERT INTO characters (traditional, simplified, radical_parts)
		VALUES ('馬', '马', '[{"part":"馬","note":"curated pictograph"}]');
	`); err != nil {
		t.Fatalf("insert character fixtures: %v", err)
	}

	handler, err := server.NewHandler(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := fantiv1connect.NewDictionaryServiceClient(http.DefaultClient, srv.URL)

	get := func(ch string) *fantiv1.Character {
		t.Helper()
		res, err := client.GetCharacter(ctx,
			connect.NewRequest(&fantiv1.GetCharacterRequest{Name: "characters/" + ch}))
		if err != nil {
			t.Fatalf("GetCharacter(%s): %v", ch, err)
		}

		return res.Msg
	}

	longTail := get("俢")
	if got := len(longTail.GetRadicalParts()); got != 3 {
		t.Errorf("俢 parts = %d, want 3", got)
	}

	curated := get("馬")
	if got := curated.GetRadicalParts(); len(got) != 1 || got[0].GetPart() != "馬" {
		t.Errorf("馬 parts = %v, want curated 馬 only", got)
	}
}
