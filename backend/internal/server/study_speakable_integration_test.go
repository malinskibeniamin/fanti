package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationSpeakableSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := seed.Run(ctx, pool, seed.Sources{}, logger); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A tiny corpus with a known unlock ladder, plus a zero-Han junk row
	// that must never count ('{}' <@ anything is true).
	sentences := []struct {
		id        int64
		text      string
		english   string
		chars     []string
		charCount int
		rank      int
		ambiguous bool
		inCourse  bool
	}{
		{1, "妳好。", "Hello.", []string{"妳", "好"}, 2, 20, false, true},
		{2, "我很好。", "I am fine.", []string{"我", "很", "好"}, 3, 30, false, true},
		{3, "他們在哪裡？", "Where are they?", []string{"他", "們", "在", "哪", "裡"}, 6, 0, false, true},
		{4, "OK!", "OK!", []string{}, 0, 0, false, false},
		// A risky simplified→traditional conversion: never counted or shown,
		// even when fully composed of learned characters.
		{5, "妳好乾。", "You are so dry.", []string{"妳", "好", "乾"}, 3, 10, true, true},
		// Contains a character the app cannot teach: unlockable progress
		// only, so it never counts and never offers a dead link.
		{6, "妳好嗨。", "You are so high.", []string{"妳", "好", "嗨"}, 3, 15, false, false},
	}
	for _, s := range sentences {
		if _, err := pool.Exec(ctx, `
			INSERT INTO sentences (id, traditional, simplified, english, chars, char_count, max_freq_rank, ambiguous, in_course)
			VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8)`,
			s.id, s.text, s.english, s.chars, s.charCount, s.rank, s.ambiguous, s.inCourse); err != nil {
			t.Fatalf("insert sentence %d: %v", s.id, err)
		}
	}

	// A learned character carrying a topic feeds the topics list.
	if _, err := pool.Exec(ctx, `
		INSERT INTO characters (traditional, simplified, topics) VALUES ('妳', '你', '{street}')
		ON CONFLICT (traditional) DO NOTHING`); err != nil {
		t.Fatalf("insert character: %v", err)
	}

	handler, err := server.NewHandler(pool, logger)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := fantiv1connect.NewStudyServiceClient(http.DefaultClient, srv.URL)

	summary := func() *fantiv1.GetSpeakableSummaryResponse {
		t.Helper()

		res, err := client.GetSpeakableSummary(ctx,
			connect.NewRequest(&fantiv1.GetSpeakableSummaryRequest{}))
		if err != nil {
			t.Fatalf("GetSpeakableSummary: %v", err)
		}

		return res.Msg
	}

	// Zero learned characters: nothing unlocked, no NULL-array artifact,
	// and the nearest challenge is still offered.
	empty := summary()
	if empty.GetUnlockedCount() != 0 || empty.GetTotalCount() != 3 {
		t.Errorf("empty summary = %d/%d, want 0/3 (junk row excluded)",
			empty.GetUnlockedCount(), empty.GetTotalCount())
	}

	if len(empty.GetSentences()) != 0 {
		t.Errorf("empty summary sentences = %v, want none", empty.GetSentences())
	}

	if len(empty.GetAlmostUnlocked()) == 0 {
		t.Fatal("empty summary almost_unlocked is empty, want nearest sentences")
	}

	// With zero learned characters the two-character sentence is nearest.
	if got := empty.GetAlmostUnlocked()[0]; got.GetId() != 1 || len(got.GetMissingCharacters()) != 2 {
		t.Errorf("nearest = id %d missing %v, want id 1 missing 2", got.GetId(), got.GetMissingCharacters())
	}

	// Learn 妳, 好, and 乾: sentence 1 unlocks; the ambiguous sentence 5
	// would too, but must stay excluded everywhere.
	for _, ch := range []string{"妳", "好", "乾"} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO reviews (ch, learned) VALUES ($1, TRUE)
			ON CONFLICT (ch) DO UPDATE SET learned = TRUE`, ch); err != nil {
			t.Fatalf("mark learned %s: %v", ch, err)
		}
	}

	got := summary()
	if got.GetUnlockedCount() != 1 || got.GetTotalCount() != 3 {
		t.Errorf("summary = %d/%d, want 1/3", got.GetUnlockedCount(), got.GetTotalCount())
	}

	if len(got.GetSentences()) != 1 || got.GetSentences()[0].GetTraditional() != "妳好。" {
		t.Errorf("unlocked sentences = %v, want 妳好。", got.GetSentences())
	}

	if len(got.GetAlmostUnlocked()) != 2 {
		t.Fatalf("almost_unlocked = %v, want 2 locked sentences", got.GetAlmostUnlocked())
	}

	nearest := got.GetAlmostUnlocked()[0]
	if nearest.GetId() != 2 || !slices.Equal(nearest.GetMissingCharacters(), []string{"我", "很"}) {
		t.Errorf("nearest = id %d missing %v, want id 2 missing [我 很]",
			nearest.GetId(), nearest.GetMissingCharacters())
	}

	if !slices.Contains(got.GetTopics(), "street") {
		t.Errorf("topics = %v, want to contain street", got.GetTopics())
	}
}
