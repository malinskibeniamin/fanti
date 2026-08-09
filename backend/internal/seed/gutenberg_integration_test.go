package seed

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/malinskibeniamin/fanti/backend/internal/convert"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

// fakeGutenberg mimics a Project Gutenberg plain-text ebook: licence header,
// transcriber credit, three simplified-Chinese chapters with a separator
// artifact, and the legacy plus modern end-of-book markers.
const fakeGutenberg = `The Project Gutenberg eBook of 測試書

This eBook is for the use of anyone anywhere in the United States.

Title: 測試書

*** START OF THE PROJECT GUTENBERG EBOOK 測試書 ***

Produced by Test Person

第一章　开头

这是简体中文写的头一段。后来这个人对着广大的观众说话。

-----------------------------------------------------------------

第二章　中间

他们讨论怎么发动机器，还谈到了头发和面条。

第三章　结尾

最后大家在台湾吃面，听说这里的面很好吃。

*** END OF THE PROJECT GUTENBERG EBOOK 測試書 ***

End of the Project Gutenberg EBook of 測試書

Updated editions will replace the previous one.
`

func TestIntegrationGutenbergSeedText(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := Run(ctx, pool, Sources{}, logger); err != nil {
		t.Fatalf("seed.Run() error = %v", err)
	}

	engine, err := convert.NewEngine()
	if err != nil {
		t.Fatalf("convert.NewEngine() error = %v", err)
	}

	seedText := func() {
		t.Helper()

		if err := seedGutenbergText(ctx, pool, engine, "skt", fakeGutenberg); err != nil {
			t.Fatalf("seedGutenbergText() error = %v", err)
		}
	}

	seedText()
	seedText() // A re-run must replace the chapters cleanly, not duplicate them.

	rows, err := pool.Query(ctx, `
		SELECT title, traditional_paragraphs, simplified_paragraphs
		FROM chapters WHERE book_id = 'skt' ORDER BY idx`)
	if err != nil {
		t.Fatalf("query chapters: %v", err)
	}

	defer rows.Close()

	type chapter struct {
		title string
		trad  []string
		simp  []string
	}

	var chapters []chapter

	for rows.Next() {
		var ch chapter
		if err := rows.Scan(&ch.title, &ch.trad, &ch.simp); err != nil {
			t.Fatalf("scan chapter: %v", err)
		}

		chapters = append(chapters, ch)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if len(chapters) != 3 {
		t.Fatalf("chapters = %d, want 3", len(chapters))
	}

	// Titles are stored in traditional script (the converted form here,
	// since the fake source is simplified).
	if chapters[0].title != "第一章　開頭" {
		t.Errorf("chapter 0 title = %q, want 第一章　開頭", chapters[0].title)
	}

	var sourceRunes int64

	for i, ch := range chapters {
		if len(ch.trad) == 0 || len(ch.simp) == 0 {
			t.Fatalf("chapter %d: empty script column (trad %d, simp %d)", i, len(ch.trad), len(ch.simp))
		}

		if len(ch.trad) != len(ch.simp) {
			t.Errorf("chapter %d: paragraph counts differ (trad %d, simp %d)", i, len(ch.trad), len(ch.simp))
		}

		for _, p := range append(append([]string{}, ch.trad...), ch.simp...) {
			if strings.Contains(p, "Project Gutenberg") || strings.Contains(p, "Produced by") {
				t.Errorf("chapter %d: boilerplate leaked into paragraph %q", i, p)
			}

			if strings.Trim(p, "-") == "" {
				t.Errorf("chapter %d: separator artifact stored as paragraph %q", i, p)
			}
		}

		for _, p := range ch.simp {
			sourceRunes += int64(utf8.RuneCountInString(p))
		}
	}

	// The source is simplified, so the two script columns must differ.
	if strings.Join(chapters[0].trad, "") == strings.Join(chapters[0].simp, "") {
		t.Error("traditional and simplified paragraphs are identical; conversion did not run")
	}

	var charCount int64
	if err := pool.QueryRow(ctx,
		"SELECT char_count FROM books WHERE id = 'skt'").Scan(&charCount); err != nil {
		t.Fatalf("book char_count: %v", err)
	}

	if charCount == 0 || charCount != sourceRunes {
		t.Errorf("char_count = %d, want %d (source paragraph runes)", charCount, sourceRunes)
	}
}
