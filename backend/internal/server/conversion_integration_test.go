package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/bookfile"
	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

const sampleTxt = `第一章

他后来发现，面馆里的师傅头发花白，说话却很干净利落。
一根白发落在碗里。

第二章

心里想着三里路，软件也要更新。
`

// TestIntegrationConversionFlow drives the whole wizard: upload → analyze →
// settings → run (async, polled) → report → resolve → export → library →
// tokenized reader chapter.
func TestIntegrationConversionFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := seed.Run(ctx, pool, seed.Sources{}, logger); err != nil {
		t.Fatalf("seed: %v", err)
	}

	handler, err := server.NewHandler(pool, logger)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	conv := fantiv1connect.NewConversionServiceClient(http.DefaultClient, srv.URL)
	lib := fantiv1connect.NewLibraryServiceClient(http.DefaultClient, srv.URL)

	t.Run("RejectEmptyBook", func(t *testing.T) {
		emptyEPUB, err := bookfile.WriteEPUB(bookfile.EPUBMeta{Title: "Empty"}, nil)
		if err != nil {
			t.Fatalf("WriteEPUB: %v", err)
		}

		for _, tc := range []struct {
			filename string
			data     []byte
		}{
			{filename: "empty.txt", data: []byte("  \n\t\n  ")},
			{filename: "empty.srt", data: []byte("1\n00:00:01,000 --> 00:00:02,000\n\n")},
			{filename: "empty.epub", data: emptyEPUB},
		} {
			_, err := conv.CreateConversion(ctx, connect.NewRequest(&fantiv1.CreateConversionRequest{
				Filename: tc.filename,
				Data:     tc.data,
			}))
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("%s code = %v, want InvalidArgument", tc.filename, connect.CodeOf(err))
			}
		}
	})

	// 1. Upload + analyze.
	created, err := conv.CreateConversion(ctx, connect.NewRequest(&fantiv1.CreateConversionRequest{
		Filename: "sample.txt",
		Data:     []byte(sampleTxt),
	}))
	if err != nil {
		t.Fatalf("CreateConversion: %v", err)
	}

	job := created.Msg
	if job.GetState() != fantiv1.Conversion_STATE_READY {
		t.Fatalf("state = %v, want READY", job.GetState())
	}

	if job.GetDetectedScript() != fantiv1.Script_SCRIPT_SIMPLIFIED {
		t.Errorf("detected = %v, want SIMPLIFIED", job.GetDetectedScript())
	}

	if job.GetSettings().GetDirection() != fantiv1.ConversionDirection_CONVERSION_DIRECTION_S2T {
		t.Errorf("default direction = %v, want S2T", job.GetSettings().GetDirection())
	}

	var chapterUnit int32

	for _, u := range job.GetUnitCounts() {
		if u.GetUnit() == "章" {
			chapterUnit = u.GetCount()
		}
	}

	if chapterUnit != 2 {
		t.Errorf("章 count = %d, want 2", chapterUnit)
	}

	if len(job.GetLayout().GetChapterTitles()) != 2 {
		t.Errorf("chapter titles = %v", job.GetLayout().GetChapterTitles())
	}

	// 2. Update settings (keep localization on, set punctuation off).
	_, err = conv.UpdateConversion(ctx, connect.NewRequest(&fantiv1.UpdateConversionRequest{
		Conversion: &fantiv1.Conversion{
			Name: job.GetName(),
			Settings: &fantiv1.ConversionSettings{
				Direction:          fantiv1.ConversionDirection_CONVERSION_DIRECTION_S2T,
				LocalizeVocabulary: true,
				ConvertPunctuation: false,
			},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"settings"}},
	}))
	if err != nil {
		t.Fatalf("UpdateConversion: %v", err)
	}

	// 3. Run and poll.
	if _, err := conv.RunConversion(ctx, connect.NewRequest(&fantiv1.RunConversionRequest{
		Name: job.GetName(),
	})); err != nil {
		t.Fatalf("RunConversion: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)

	var final *fantiv1.Conversion

	for {
		got, err := conv.GetConversion(ctx, connect.NewRequest(&fantiv1.GetConversionRequest{
			Name: job.GetName(),
		}))
		if err != nil {
			t.Fatalf("GetConversion: %v", err)
		}

		if got.Msg.GetState() == fantiv1.Conversion_STATE_SUCCEEDED {
			final = got.Msg

			break
		}

		if got.Msg.GetState() == fantiv1.Conversion_STATE_FAILED {
			t.Fatalf("conversion failed: %s", got.Msg.GetErrorMessage())
		}

		if time.Now().After(deadline) {
			t.Fatalf("conversion did not finish; state %v", got.Msg.GetState())
		}

		time.Sleep(50 * time.Millisecond)
	}

	report := final.GetReport()
	if report.GetAmbiguousCount() == 0 || report.GetExactCount() == 0 {
		t.Errorf("report counts: %+v", report)
	}

	var hasFa bool

	for _, ex := range report.GetExceptions() {
		if ex.GetSourceChar() == "发" {
			hasFa = true

			if len(ex.GetOptions()) < 2 {
				t.Errorf("发 options = %v", ex.GetOptions())
			}
		}
	}

	if !hasFa {
		t.Error("发 exception missing from report")
	}

	if len(report.GetDiff().GetTokens()) == 0 {
		t.Error("diff preview empty")
	}

	// 4. Resolve an exception.
	for _, tc := range []struct {
		name       string
		sourceChar string
		resolved   string
	}{
		{name: "unknown source", sourceChar: "無", resolved: "无"},
		{name: "unoffered replacement", sourceChar: "面", resolved: "任意"},
		{name: "multiple characters", sourceChar: "面", resolved: "麵館"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conv.ResolveException(ctx, connect.NewRequest(&fantiv1.ResolveExceptionRequest{
				Name: job.GetName(), SourceChar: tc.sourceChar, Resolved: tc.resolved,
			}))
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Errorf("code = %v, want InvalidArgument", connect.CodeOf(err))
			}
		})
	}

	resolved, err := conv.ResolveException(ctx, connect.NewRequest(&fantiv1.ResolveExceptionRequest{
		Name:       job.GetName(),
		SourceChar: "面",
		Resolved:   "面",
	}))
	if err != nil {
		t.Fatalf("ResolveException: %v", err)
	}

	var found bool

	for _, ex := range resolved.Msg.GetReport().GetExceptions() {
		if ex.GetSourceChar() == "面" && ex.GetResolved() == "面" {
			found = true
		}
	}

	if !found {
		t.Error("resolution not reflected in report")
	}

	// 5. Export EPUB — resolution must be applied.
	exported, err := conv.ExportConversion(ctx, connect.NewRequest(&fantiv1.ExportConversionRequest{
		Name: job.GetName(),
	}))
	if err != nil {
		t.Fatalf("ExportConversion: %v", err)
	}

	if len(exported.Msg.GetData()) == 0 || !strings.HasSuffix(exported.Msg.GetFilename(), ".epub") {
		t.Errorf("export = %q (%d bytes)", exported.Msg.GetFilename(), len(exported.Msg.GetData()))
	}

	// 6. Add to library and read it back tokenized.
	book, err := conv.AddConversionToLibrary(ctx, connect.NewRequest(&fantiv1.AddConversionToLibraryRequest{
		Name: job.GetName(),
	}))
	if err != nil {
		t.Fatalf("AddConversionToLibrary: %v", err)
	}

	if book.Msg.GetChapterCount() != 2 {
		t.Errorf("book chapters = %d, want 2", book.Msg.GetChapterCount())
	}

	chapter, err := lib.GetChapter(ctx, connect.NewRequest(&fantiv1.GetChapterRequest{
		Name: book.Msg.GetName() + "/chapters/0",
		View: fantiv1.ChapterView_CHAPTER_VIEW_FULL,
	}))
	if err != nil {
		t.Fatalf("GetChapter: %v", err)
	}

	paras := chapter.Msg.GetTraditionalParagraphs()
	if len(paras) == 0 || len(paras[0].GetTokens()) == 0 {
		t.Fatal("tokenized paragraphs empty")
	}

	// The converted text must contain 頭髮 with pinyin + tappable link on 髮.
	var sawFa bool

	for _, tk := range paras[0].GetTokens() {
		if tk.GetText() == "髮" {
			sawFa = true

			if tk.GetPinyin() == "" || tk.GetCharacter() != "characters/髮" {
				t.Errorf("髮 token = %+v", tk)
			}
		}
	}

	if !sawFa {
		t.Errorf("髮 not found in converted chapter: %v", paras[0].GetTokens())
	}

	if len(paras[0].GetSentences()) == 0 {
		t.Error("sentence spans missing")
	}

	// Re-adding the same conversion must refresh all derived metadata and
	// preserve the chapter titles edited in the layout step.
	layout := final.GetLayout()
	layout.CoverColor = "#123456"
	layout.ChapterTitles = []string{"自訂第一章", "自訂第二章"}

	if _, err := conv.UpdateConversion(ctx, connect.NewRequest(&fantiv1.UpdateConversionRequest{
		Conversion: &fantiv1.Conversion{Name: job.GetName(), Layout: layout},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"layout"}},
	})); err != nil {
		t.Fatalf("UpdateConversion layout: %v", err)
	}

	refreshed, err := conv.AddConversionToLibrary(ctx,
		connect.NewRequest(&fantiv1.AddConversionToLibraryRequest{Name: job.GetName()}))
	if err != nil {
		t.Fatalf("AddConversionToLibrary again: %v", err)
	}

	if refreshed.Msg.GetCoverColor() != "#123456" {
		t.Errorf("cover color = %q, want refreshed value", refreshed.Msg.GetCoverColor())
	}

	chapter, err = lib.GetChapter(ctx, connect.NewRequest(&fantiv1.GetChapterRequest{
		Name: refreshed.Msg.GetName() + "/chapters/0",
		View: fantiv1.ChapterView_CHAPTER_VIEW_BASIC,
	}))
	if err != nil {
		t.Fatalf("GetChapter after re-add: %v", err)
	}

	if chapter.Msg.GetTitle() != "自訂第一章" {
		t.Errorf("chapter title = %q, want edited layout title", chapter.Msg.GetTitle())
	}

	// A process restart cannot resume an in-memory job, so startup must make
	// persisted RUNNING conversions explicitly retryable instead of stranding polling clients.
	if _, err := pool.Exec(ctx, `
		UPDATE conversions SET state = 'running', progress_percent = 42, error_message = ''
		WHERE id = $1`, strings.TrimPrefix(job.GetName(), "conversions/")); err != nil {
		t.Fatalf("simulate interrupted conversion: %v", err)
	}

	if _, err := server.NewHandler(pool, logger); err != nil {
		t.Fatalf("NewHandler after restart: %v", err)
	}

	interrupted, err := conv.GetConversion(ctx, connect.NewRequest(&fantiv1.GetConversionRequest{
		Name: job.GetName(),
	}))
	if err != nil {
		t.Fatalf("GetConversion after restart: %v", err)
	}

	if interrupted.Msg.GetState() != fantiv1.Conversion_STATE_FAILED ||
		interrupted.Msg.GetErrorMessage() == "" {
		t.Errorf("interrupted conversion = %v/%q, want FAILED with retry guidance",
			interrupted.Msg.GetState(), interrupted.Msg.GetErrorMessage())
	}
}
