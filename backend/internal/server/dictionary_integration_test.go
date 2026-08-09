package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationDictionaryService(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := seed.Run(ctx, pool, seed.Sources{}, logger); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Mark 馬 learned so learned-state joins are exercised.
	if _, err := pool.Exec(ctx, `
		INSERT INTO reviews (ch, learned, in_deck, mistake_count)
		VALUES ('馬', TRUE, TRUE, 2)`); err != nil {
		t.Fatalf("insert review: %v", err)
	}

	handler, err := server.NewHandler(pool, logger)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := fantiv1connect.NewDictionaryServiceClient(http.DefaultClient, srv.URL)
	library := fantiv1connect.NewLibraryServiceClient(http.DefaultClient, srv.URL)
	const fallbackCharacterName = "characters/鬱"

	t.Run("GetCharacter", func(t *testing.T) {
		resp, err := client.GetCharacter(ctx, connect.NewRequest(&fantiv1.GetCharacterRequest{
			Name: "characters/馬",
		}))
		if err != nil {
			t.Fatalf("GetCharacter: %v", err)
		}

		ch := resp.Msg
		if ch.GetSimplified() != "马" || ch.GetPinyin() != "mǎ" {
			t.Errorf("馬 = %q/%q, want 马/mǎ", ch.GetSimplified(), ch.GetPinyin())
		}

		if !ch.GetLearned() || ch.GetMistakeCount() != 2 {
			t.Errorf("learned = %v mistakes = %d, want true/2", ch.GetLearned(), ch.GetMistakeCount())
		}

		if len(ch.GetExamples()) == 0 || len(ch.GetRadicalParts()) == 0 {
			t.Errorf("examples/radicals missing: %d/%d", len(ch.GetExamples()), len(ch.GetRadicalParts()))
		}
	})

	t.Run("GetCharacterHistory", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO character_history (
				ch, stage, stage_order, svg, source_title, source_url, source_sha1, license
			) VALUES
				('馬', 'oracle', 1, '<svg/>', 'File:馬-oracle.svg',
					'https://commons.wikimedia.org/wiki/File:馬-oracle.svg', 'oracle-sha', 'Public domain'),
				('馬', 'bronze', 2, NULL, '', '', '', ''),
				('馬', 'seal', 3, NULL, '', '', '', ''),
				('馬', 'clerical', 4, NULL, '', '', '', ''),
				('馬', 'regular', 5, NULL, '', '', '', '')`); err != nil {
			t.Fatalf("insert history: %v", err)
		}

		resp, err := client.GetCharacterHistory(ctx,
			connect.NewRequest(&fantiv1.GetCharacterHistoryRequest{
				Name: "characters/馬/history",
			}))
		if err != nil {
			t.Fatalf("GetCharacterHistory: %v", err)
		}

		history := resp.Msg
		if history.GetName() != "characters/馬/history" {
			t.Errorf("name = %q, want characters/馬/history", history.GetName())
		}
		if len(history.GetForms()) != 5 {
			t.Fatalf("forms = %d, want 5", len(history.GetForms()))
		}

		oracle := history.GetForms()[0]
		if oracle.GetStage() != fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_ORACLE ||
			!oracle.GetAvailable() || string(oracle.GetSvg()) != "<svg/>" ||
			oracle.GetSourceTitle() != "File:馬-oracle.svg" {
			t.Errorf("oracle form = %+v", oracle)
		}

		bronze := history.GetForms()[1]
		if bronze.GetStage() != fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_BRONZE ||
			bronze.GetAvailable() {
			t.Errorf("bronze gap = %+v", bronze)
		}

		regular := history.GetForms()[4]
		if regular.GetStage() != fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_REGULAR ||
			!regular.GetAvailable() || len(regular.GetSvg()) != 0 {
			t.Errorf("regular form = %+v", regular)
		}
	})

	t.Run("GetCharacterHistoryNotFound", func(t *testing.T) {
		_, err := client.GetCharacterHistory(ctx,
			connect.NewRequest(&fantiv1.GetCharacterHistoryRequest{
				Name: "characters/不存在/history",
			}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("code = %v, want NotFound", connect.CodeOf(err))
		}
	})

	t.Run("GetCharacterNotFound", func(t *testing.T) {
		_, err := client.GetCharacter(ctx, connect.NewRequest(&fantiv1.GetCharacterRequest{
			Name: fallbackCharacterName,
		}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("code = %v, want NotFound", connect.CodeOf(err))
		}
	})

	t.Run("GetCharacterFromDictionaryEntry", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO dict_entries (traditional, simplified, pinyin, definitions)
			VALUES ('鬱', '郁', 'yù', ARRAY['depressed', 'luxuriant'])`); err != nil {
			t.Fatalf("insert dictionary entry: %v", err)
		}

		resp, err := client.GetCharacter(ctx, connect.NewRequest(&fantiv1.GetCharacterRequest{
			Name: fallbackCharacterName,
		}))
		if err != nil {
			t.Fatalf("GetCharacter: %v", err)
		}

		ch := resp.Msg
		if ch.GetName() != fallbackCharacterName || ch.GetTraditional() != "鬱" ||
			ch.GetSimplified() != "郁" || ch.GetPinyin() != "yù" ||
			ch.GetMeaning() != "depressed; luxuriant" {
			t.Errorf("dictionary fallback = %+v", ch)
		}

		if _, err := pool.Exec(ctx, `
			INSERT INTO books (id, title) VALUES ('fallback', 'Fallback');
			INSERT INTO chapters (book_id, idx, traditional_paragraphs, simplified_paragraphs)
			VALUES ('fallback', 0, ARRAY['鬱。'], ARRAY['郁。'])`); err != nil {
			t.Fatalf("insert fallback reader chapter: %v", err)
		}

		chapter, err := library.GetChapter(ctx, connect.NewRequest(&fantiv1.GetChapterRequest{
			Name: "books/fallback/chapters/0",
			View: fantiv1.ChapterView_CHAPTER_VIEW_FULL,
		}))
		if err != nil {
			t.Fatalf("GetChapter: %v", err)
		}
		if got := chapter.Msg.GetTraditionalParagraphs()[0].GetTokens()[0].GetCharacter(); got != fallbackCharacterName {
			t.Errorf("reader token link = %q, want characters/鬱", got)
		}
	})

	t.Run("ListCharactersByTopic", func(t *testing.T) {
		resp, err := client.ListCharacters(ctx, connect.NewRequest(&fantiv1.ListCharactersRequest{
			Filter: `topic = "food"`,
		}))
		if err != nil {
			t.Fatalf("ListCharacters: %v", err)
		}

		if len(resp.Msg.GetCharacters()) == 0 {
			t.Fatal("no food-topic characters")
		}

		for _, ch := range resp.Msg.GetCharacters() {
			found := false

			for _, topic := range ch.GetTopics() {
				if topic == "food" {
					found = true
				}
			}

			if !found {
				t.Errorf("%s lacks food topic %v", ch.GetTraditional(), ch.GetTopics())
			}
		}
	})

	t.Run("ListCharactersQueryAndPaging", func(t *testing.T) {
		resp, err := client.ListCharacters(ctx, connect.NewRequest(&fantiv1.ListCharactersRequest{
			PageSize: 10,
		}))
		if err != nil {
			t.Fatalf("ListCharacters: %v", err)
		}

		if len(resp.Msg.GetCharacters()) != 10 || resp.Msg.GetNextPageToken() == "" {
			t.Fatalf("page = %d chars, token %q", len(resp.Msg.GetCharacters()), resp.Msg.GetNextPageToken())
		}

		if resp.Msg.GetTotalSize() != 28 {
			t.Errorf("total = %d, want 28", resp.Msg.GetTotalSize())
		}

		second, err := client.ListCharacters(ctx, connect.NewRequest(&fantiv1.ListCharactersRequest{
			PageSize:  10,
			PageToken: resp.Msg.GetNextPageToken(),
		}))
		if err != nil {
			t.Fatalf("second page: %v", err)
		}

		if second.Msg.GetCharacters()[0].GetName() == resp.Msg.GetCharacters()[0].GetName() {
			t.Error("second page repeats first page")
		}

		horse, err := client.ListCharacters(ctx, connect.NewRequest(&fantiv1.ListCharactersRequest{
			Query: "horse",
		}))
		if err != nil {
			t.Fatalf("query: %v", err)
		}

		if len(horse.Msg.GetCharacters()) != 1 || horse.Msg.GetCharacters()[0].GetTraditional() != "馬" {
			t.Errorf("query horse = %v", horse.Msg.GetCharacters())
		}
	})

	t.Run("ListCompounds", func(t *testing.T) {
		resp, err := client.ListCompounds(ctx, connect.NewRequest(&fantiv1.ListCompoundsRequest{}))
		if err != nil {
			t.Fatalf("ListCompounds: %v", err)
		}

		if resp.Msg.GetTotalSize() != 5 {
			t.Errorf("compounds = %d, want 5", resp.Msg.GetTotalSize())
		}

		// 馬上 needs 馬 (learned) and 上 (not learned) — locked.
		for _, c := range resp.Msg.GetCompounds() {
			if c.GetWord() == "馬上" && c.GetUnlocked() {
				t.Error("馬上 unlocked with 上 unlearned")
			}
		}
	})

	t.Run("SearchEntriesValidation", func(t *testing.T) {
		_, err := client.SearchEntries(ctx, connect.NewRequest(&fantiv1.SearchEntriesRequest{}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("code = %v, want InvalidArgument", connect.CodeOf(err))
		}
	})
}
