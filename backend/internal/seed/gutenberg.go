package seed

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/malinskibeniamin/fanti/backend/internal/bookfile"
	"github.com/malinskibeniamin/fanti/backend/internal/convert"
)

// GutenbergBook describes one classic to fetch.
type GutenbergBook struct {
	// BookID is the existing books.id row whose chapters the text fills.
	BookID string
	// Number is the Project Gutenberg ebook number.
	Number int
	// URL is the plain-text UTF-8 download location.
	URL string
}

// GutenbergBooks lists the full-text classics from the design's library.
//
//nolint:gochecknoglobals // static catalogue, like the punctuation tables in convert
var GutenbergBooks = []GutenbergBook{
	{BookID: "skt", Number: 23950, URL: "https://www.gutenberg.org/cache/epub/23950/pg23950.txt"},
	{BookID: "dream", Number: 24264, URL: "https://www.gutenberg.org/cache/epub/24264/pg24264.txt"},
	{BookID: "rulin", Number: 24032, URL: "https://www.gutenberg.org/cache/epub/24032/pg24032.txt"},
}

// Sentinel errors for Gutenberg seeding.
var (
	errNoGutenbergMarker = errors.New("gutenberg boilerplate marker not found")
	errNoChapters        = errors.New("no chapters parsed from gutenberg text")
	errUnknownBook       = errors.New("book row does not exist")
)

// gutenbergSeededThreshold: more chapters than this means the real text is
// already loaded (the placeholder sample has 1).
const gutenbergSeededThreshold = 5

// SeedGutenberg downloads each classic (via DownloadCached into downloadDir),
// strips the Gutenberg licence header/footer, parses chapters, converts to
// fill both script columns, and replaces the chapters of the existing book
// row. A book whose chapters already exceed the placeholder count is skipped
// unless force is true.
//
//nolint:revive // the design names this step SeedGutenberg; seed.Run/seed.SeedGutenberg read as a pair
func SeedGutenberg(
	ctx context.Context, pool *pgxpool.Pool, engine *convert.Engine,
	downloadDir string, force bool, logger *slog.Logger,
) error {
	for _, book := range GutenbergBooks {
		var existing int64
		if err := pool.QueryRow(ctx,
			"SELECT count(*) FROM chapters WHERE book_id = $1", book.BookID).Scan(&existing); err != nil {
			return fmt.Errorf("count chapters for %s: %w", book.BookID, err)
		}

		if existing > gutenbergSeededThreshold && !force {
			logger.InfoContext(ctx, "classic already seeded, skipping",
				slog.String("book", book.BookID), slog.Int64("chapters", existing))

			continue
		}

		path, err := DownloadCached(ctx, downloadDir, fmt.Sprintf("pg%d.txt", book.Number), book.URL)
		if err != nil {
			return fmt.Errorf("download %s: %w", book.BookID, err)
		}

		raw, err := os.ReadFile(path) //nolint:gosec // operator-controlled cache dir
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		if err := seedGutenbergText(ctx, pool, engine, book.BookID, string(raw)); err != nil {
			return fmt.Errorf("seed %s: %w", book.BookID, err)
		}

		logger.InfoContext(ctx, "seeded classic",
			slog.String("book", book.BookID), slog.Int("ebook", book.Number))
	}

	return nil
}

// seedGutenbergText runs the per-book pipeline on already-downloaded text:
// strip boilerplate, parse chapters, convert to the other script, and store.
func seedGutenbergText(
	ctx context.Context, pool *pgxpool.Pool, engine *convert.Engine, bookID, rawText string,
) error {
	body, err := stripGutenbergBoilerplate(rawText)
	if err != nil {
		return err
	}

	body = normalizeHeadingZeros(body)

	parsed, err := bookfile.Parse("gutenberg.txt", []byte(body))
	if err != nil {
		return fmt.Errorf("parse text: %w", err)
	}

	chapters := cleanChapters(parsed.Chapters)
	if len(chapters) == 0 {
		return errNoChapters
	}

	direction, err := engine.DetectScript(sampleText(chapters))
	if err != nil {
		return fmt.Errorf("detect script: %w", err)
	}

	source := make([]convert.Chapter, len(chapters))
	for i, ch := range chapters {
		source[i] = convert.Chapter{Title: ch.Title, Paragraphs: ch.Paragraphs}
	}

	converted, _, err := engine.ConvertChapters(source, convert.Options{
		Direction:   direction,
		Punctuation: true,
	}, nil)
	if err != nil {
		return fmt.Errorf("convert chapters: %w", err)
	}

	// The source fills its own script column; the conversion fills the other.
	traditional, simplified := converted, source
	if direction == convert.T2S {
		traditional, simplified = source, converted
	}

	return storeChapters(ctx, pool, bookID, traditional, simplified, charCount(chapters))
}

// storeChapters replaces the book's chapters and refreshes its counters in
// one transaction.
func storeChapters(
	ctx context.Context, pool *pgxpool.Pool, bookID string,
	traditional, simplified []convert.Chapter, chars int64,
) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "DELETE FROM chapters WHERE book_id = $1", bookID); err != nil {
		return fmt.Errorf("delete old chapters: %w", err)
	}

	for i := range traditional {
		if _, err := tx.Exec(ctx, `
			INSERT INTO chapters (book_id, idx, title, traditional_paragraphs, simplified_paragraphs)
			VALUES ($1, $2, $3, $4, $5)`,
			bookID, i, traditional[i].Title,
			traditional[i].Paragraphs, simplified[i].Paragraphs); err != nil {
			return fmt.Errorf("insert chapter %d: %w", i, err)
		}
	}

	tag, err := tx.Exec(ctx, `
		UPDATE books SET char_count = $2, update_time = now() WHERE id = $1`, bookID, chars)
	if err != nil {
		return fmt.Errorf("update book: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("book %s: %w", bookID, errUnknownBook)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// stripGutenbergBoilerplate cuts the text down to the body between the
// "*** START OF ... ***" and "*** END OF ... ***" markers (tolerating the
// THE/THIS wording variants), drops the legacy "End of Project Gutenberg"
// footer some ebooks carry inside the markers, and removes transcriber
// credit lines. Regexes are compiled per call to keep the package free of
// mutable globals (see bookfile.chapterRegexp).
func stripGutenbergBoilerplate(raw string) (string, error) {
	start := regexp.MustCompile(`(?m)^[ \t]*\*{3} ?START OF.*?\*{3}.*$`)
	end := regexp.MustCompile(`(?m)^[ \t]*(?:\*{3} ?END OF.*?\*{3}.*|End of (?:[Tt]he |[Tt]his )?Project Gutenberg.*)$`)

	loc := start.FindStringIndex(raw)
	if loc == nil {
		return "", fmt.Errorf("start marker: %w", errNoGutenbergMarker)
	}

	body := raw[loc[1]:]

	loc = end.FindStringIndex(body)
	if loc == nil {
		return "", fmt.Errorf("end marker: %w", errNoGutenbergMarker)
	}

	body = body[:loc[0]]

	credit := regexp.MustCompile(`(?m)^[ \t]*Produced by .*$`)

	return credit.ReplaceAllString(body, ""), nil
}

// normalizeHeadingZeros rewrites circle-style zeros (第一○○回, 第一一〇回)
// on chapter-heading lines to 零 so the bookfile chapter regex recognizes
// them; body text keeps its circles. Without this, PG #23950 merges chapters
// 100-110 into chapter 99.
func normalizeHeadingZeros(text string) string {
	heading := regexp.MustCompile(`(?m)^[\t 　]*第?[0-9一二三四五六七八九十百千零两○〇]{1,4}[章節回篇卷部]`)

	return heading.ReplaceAllStringFunc(text, func(m string) string {
		return strings.NewReplacer("○", "零", "〇", "零").Replace(m)
	})
}

// maxTitleRunes: chapter titles beyond this are heading lines with the body
// glued on (a PG #24032 transcription quirk), not real titles.
const maxTitleRunes = 50

// cleanChapters drops separator-only paragraphs (dash rules and similar
// transcription artifacts), demotes glued-on body text from overlong titles
// to a leading paragraph, and drops chapters left with nothing at all.
func cleanChapters(chapters []bookfile.Chapter) []bookfile.Chapter {
	numeral := regexp.MustCompile(`^[\t 　]*第?[0-9一二三四五六七八九十百千零两○〇]{1,4}[章節回篇卷部]`)
	cleaned := make([]bookfile.Chapter, 0, len(chapters))

	for _, ch := range chapters {
		paras := make([]string, 0, len(ch.Paragraphs))

		if utf8.RuneCountInString(ch.Title) > maxTitleRunes {
			if m := numeral.FindString(ch.Title); m != "" {
				paras = append(paras, ch.Title)
				ch.Title = strings.TrimSpace(m)
			}
		}

		for _, p := range ch.Paragraphs {
			if hasText(p) {
				paras = append(paras, p)
			}
		}

		if ch.Title == "" && len(paras) == 0 {
			continue
		}

		cleaned = append(cleaned, bookfile.Chapter{Title: ch.Title, Paragraphs: paras})
	}

	return cleaned
}

// hasText reports whether the paragraph carries any letter or digit.
func hasText(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}

	return false
}

// sampleText gathers enough opening prose for script detection (DetectScript
// caps its sample at 2000 runes).
func sampleText(chapters []bookfile.Chapter) string {
	const maxRunes = 2000

	var b strings.Builder

	count := 0

	for _, ch := range chapters {
		for _, p := range ch.Paragraphs {
			b.WriteString(p)

			count += utf8.RuneCountInString(p)
			if count >= maxRunes {
				return b.String()
			}
		}
	}

	return b.String()
}

// charCount totals the runes across all source paragraphs.
func charCount(chapters []bookfile.Chapter) int64 {
	var total int64

	for _, ch := range chapters {
		for _, p := range ch.Paragraphs {
			total += int64(utf8.RuneCountInString(p))
		}
	}

	return total
}
