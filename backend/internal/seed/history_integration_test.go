package seed_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationSeedCharacterHistoryStoresFormsAndSkipsCheckedCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	var apiCalls atomic.Int32

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/w/api.php", func(w http.ResponseWriter, req *http.Request) {
		apiCalls.Add(1)

		titles := req.URL.Query().Get("titles")
		if count := strings.Count(titles, "|") + 1; count > 50 {
			t.Errorf("Commons title batch = %d, want at most 50", count)
		}
		pages := make([]map[string]any, 0, strings.Count(titles, "|")+1)
		for title := range strings.SplitSeq(titles, "|") {
			page := map[string]any{"title": title, "missing": true}
			if title == "File:馬-oracle.svg" {
				delete(page, "missing")
				page["imageinfo"] = []map[string]any{{
					"url":            server.URL + "/horse-oracle.svg",
					"descriptionurl": "https://commons.wikimedia.org/wiki/File:馬-oracle.svg",
					"mime":           "image/svg+xml",
					"size":           38,
					"sha1":           "99f4a73ab7e5027f0191ce8093740e7dac8fa722",
					"extmetadata": map[string]any{
						"LicenseShortName": map[string]string{"value": "Public domain"},
						"Copyrighted":      map[string]string{"value": "False"},
					},
				}}
			}
			pages = append(pages, page)
		}

		if err := json.NewEncoder(w).Encode(map[string]any{
			"query": map[string]any{"pages": pages},
		}); err != nil {
			t.Errorf("encode API response: %v", err)
		}
	})
	mux.HandleFunc("/horse-oracle.svg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = w.Write([]byte(`<svg viewBox="0 0 10 10"><path d="M1 1"/></svg>`))
	})

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := seed.Run(ctx, pool, seed.Sources{}, logger); err != nil {
		t.Fatalf("seed fixtures: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO characters (
			traditional, simplified, frequency_rank
		) VALUES ('龜', '龟', 600)`); err != nil {
		t.Fatalf("insert later-rank character: %v", err)
	}

	apiURL, err := url.JoinPath(server.URL, "w/api.php")
	if err != nil {
		t.Fatalf("build API URL: %v", err)
	}

	run := func(rankLimit int) {
		t.Helper()

		if err := seed.SeedCharacterHistory(ctx, pool, seed.CharacterHistoryOptions{
			APIURL:    apiURL,
			CacheDir:  t.TempDir(),
			Client:    server.Client(),
			RankLimit: rankLimit,
		}, logger); err != nil {
			t.Fatalf("SeedCharacterHistory: %v", err)
		}
	}

	run(500)
	firstCallCount := apiCalls.Load()
	if firstCallCount != 3 {
		t.Fatalf("Commons API calls = %d, want 3 batched requests", firstCallCount)
	}

	type storedForm struct {
		stage     string
		svg       []byte
		sourceURL string
	}

	rows, err := pool.Query(ctx, `
		SELECT stage, svg, source_url
		FROM character_history
		WHERE ch = '馬'
		ORDER BY stage_order`)
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	defer rows.Close()

	var forms []storedForm
	for rows.Next() {
		var form storedForm
		if err := rows.Scan(&form.stage, &form.svg, &form.sourceURL); err != nil {
			t.Fatalf("scan history: %v", err)
		}
		forms = append(forms, form)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("history rows: %v", err)
	}

	if len(forms) != 5 {
		t.Fatalf("history forms = %d, want 5", len(forms))
	}
	if forms[0].stage != "oracle" || !strings.Contains(string(forms[0].svg), "<svg") {
		t.Errorf("oracle form = %+v, want stored SVG", forms[0])
	}
	if forms[1].stage != "bronze" || forms[1].svg != nil {
		t.Errorf("bronze form = %+v, want explicit missing stage", forms[1])
	}
	if forms[4].stage != "regular" || forms[4].svg != nil {
		t.Errorf("regular form = %+v, want local-render sentinel", forms[4])
	}

	run(500)
	if got := apiCalls.Load(); got != firstCallCount {
		t.Errorf("Commons API calls after rerun = %d, want %d", got, firstCallCount)
	}

	run(1000)
	if got := apiCalls.Load(); got != firstCallCount+1 {
		t.Errorf("Commons API calls after extending coverage = %d, want %d",
			got, firstCallCount+1)
	}

	var turtleStages int64
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM character_history WHERE ch = '龜'").Scan(&turtleStages); err != nil {
		t.Fatalf("count extended character history: %v", err)
	}
	if turtleStages != 5 {
		t.Errorf("extended character stages = %d, want 5", turtleStages)
	}
}
