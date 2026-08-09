package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/internal/bookfile"
)

// Library serves fanti.v1.LibraryService.
type Library struct {
	pool *pgxpool.Pool
}

// NewLibrary builds the library service.
func NewLibrary(pool *pgxpool.Pool) *Library {
	return &Library{pool: pool}
}

const (
	scriptSimplified  = "simplified"
	scriptTraditional = "traditional"
)

//nolint:gochecknoglobals // static proto enum mappings
var scriptEnum = map[string]fantiv1.Script{
	scriptSimplified:  fantiv1.Script_SCRIPT_SIMPLIFIED,
	scriptTraditional: fantiv1.Script_SCRIPT_TRADITIONAL,
}

//nolint:gochecknoglobals // static proto enum mappings
var formatEnum = map[string]fantiv1.FileFormat{
	"epub": fantiv1.FileFormat_FILE_FORMAT_EPUB,
	"txt":  fantiv1.FileFormat_FILE_FORMAT_TXT,
	"srt":  fantiv1.FileFormat_FILE_FORMAT_SRT,
	"mobi": fantiv1.FileFormat_FILE_FORMAT_MOBI,
}

const bookColumns = `
	b.id, b.title, b.title_english, b.author, b.script, b.source_format,
	b.cover_color, b.reading_progress, b.current_chapter_index,
	b.description, b.file_size_label, b.metadata_fields, b.graded_story,
	b.level_label, b.char_count,
	(SELECT count(*) FROM chapters c WHERE c.book_id = b.id),
	b.create_time, b.update_time`

func scanBook(row pgx.Row) (*fantiv1.Book, error) {
	var (
		b                      fantiv1.Book
		id, script, format     string
		currentChapter         int32
		metaRaw                []byte
		chapterCount           int64
		createTime, updateTime time.Time
	)

	err := row.Scan(&id, &b.Title, &b.TitleEnglish, &b.Author, &script, &format,
		&b.CoverColor, &b.ReadingProgress, &currentChapter, &b.Description,
		&b.FileSizeLabel, &metaRaw, &b.GradedStory, &b.LevelLabel, &b.CharCount,
		&chapterCount, &createTime, &updateTime)
	if err != nil {
		return nil, err
	}

	b.Name = "books/" + id
	b.Script = scriptEnum[script]
	b.SourceFormat = formatEnum[strings.ToLower(format)]
	b.ChapterCount = int32(min(chapterCount, 1<<31-1)) //nolint:gosec // bounded
	b.CurrentChapter = fmt.Sprintf("books/%s/chapters/%d", id, currentChapter)
	b.CreateTime = timestamppb.New(createTime)
	b.UpdateTime = timestamppb.New(updateTime)

	var fields []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(metaRaw, &fields); err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}

	for _, f := range fields {
		b.MetadataFields = append(b.MetadataFields, &fantiv1.MetadataField{
			Label: f.Label, Value: f.Value,
		})
	}

	return &b, nil
}

// ListBooks lists the library ordered by creation.
func (l *Library) ListBooks(
	ctx context.Context, req *connect.Request[fantiv1.ListBooksRequest],
) (*connect.Response[fantiv1.ListBooksResponse], error) {
	size := req.Msg.GetPageSize()
	if size <= 0 || size > 200 {
		size = defaultPageSize
	}

	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}

	rows, err := l.pool.Query(ctx,
		"SELECT"+bookColumns+" FROM books b ORDER BY b.create_time, b.id LIMIT $1 OFFSET $2",
		size+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.ListBooksResponse{}

	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp.Books = append(resp.Books, b)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if len(resp.GetBooks()) > int(size) {
		resp.Books = resp.GetBooks()[:size]
		resp.NextPageToken = encodePageToken(offset + int(size))
	}

	return connect.NewResponse(resp), nil
}

// GetBook returns one book.
func (l *Library) GetBook(
	ctx context.Context, req *connect.Request[fantiv1.GetBookRequest],
) (*connect.Response[fantiv1.Book], error) {
	id, err := parseName("books", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	b, err := scanBook(l.pool.QueryRow(ctx,
		"SELECT"+bookColumns+" FROM books b WHERE b.id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("book %q not found", id)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(b), nil
}

// UpdateBook applies a FieldMask over the mutable book fields.
func (l *Library) UpdateBook(
	ctx context.Context, req *connect.Request[fantiv1.UpdateBookRequest],
) (*connect.Response[fantiv1.Book], error) {
	id, err := parseName("books", req.Msg.GetBook().GetName())
	if err != nil {
		return nil, err
	}

	paths := req.Msg.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errEmptyMask)
	}

	sets := []string{"update_time = now()"}

	var args []any

	arg := func(v any) string {
		args = append(args, v)

		return "$" + strconv.Itoa(len(args))
	}

	for _, p := range paths {
		switch p {
		case "reading_progress":
			sets = append(sets, "reading_progress = "+arg(req.Msg.GetBook().GetReadingProgress()))
		case "current_chapter":
			idx, err := chapterIndex(id, req.Msg.GetBook().GetCurrentChapter())
			if err != nil {
				return nil, err
			}

			sets = append(sets, "current_chapter_index = "+arg(idx))
		case "description":
			sets = append(sets, "description = "+arg(req.Msg.GetBook().GetDescription()))
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("update_mask path %q is not updatable", p)) //nolint:err113 // request detail
		}
	}

	tag, err := l.pool.Exec(ctx,
		"UPDATE books SET "+strings.Join(sets, ", ")+" WHERE id = "+arg(id), args...)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("book %q not found", id)) //nolint:err113 // request detail
	}

	return l.GetBook(ctx, connect.NewRequest(&fantiv1.GetBookRequest{
		Name: req.Msg.GetBook().GetName(),
	}))
}

// DeleteBook removes a book and its chapters.
func (l *Library) DeleteBook(
	ctx context.Context, req *connect.Request[fantiv1.DeleteBookRequest],
) (*connect.Response[emptypb.Empty], error) {
	id, err := parseName("books", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	tag, err := l.pool.Exec(ctx, "DELETE FROM books WHERE id = $1", id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("book %q not found", id)) //nolint:err113 // request detail
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// ListChapters lists chapter titles.
func (l *Library) ListChapters(
	ctx context.Context, req *connect.Request[fantiv1.ListChaptersRequest],
) (*connect.Response[fantiv1.ListChaptersResponse], error) {
	bookID, err := parseName("books", req.Msg.GetParent())
	if err != nil {
		return nil, err
	}

	rows, err := l.pool.Query(ctx,
		"SELECT idx, title FROM chapters WHERE book_id = $1 ORDER BY idx", bookID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.ListChaptersResponse{}

	for rows.Next() {
		var (
			idx   int32
			title string
		)

		if err := rows.Scan(&idx, &title); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp.Chapters = append(resp.Chapters, &fantiv1.Chapter{
			Name:  fmt.Sprintf("books/%s/chapters/%d", bookID, idx),
			Title: title,
			Index: idx,
		})
	}

	return connect.NewResponse(resp), rows.Err()
}

// GetChapter returns one chapter, tokenized when CHAPTER_VIEW_FULL.
func (l *Library) GetChapter(
	ctx context.Context, req *connect.Request[fantiv1.GetChapterRequest],
) (*connect.Response[fantiv1.Chapter], error) {
	bookID, idx, err := parseChapterName(req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	var (
		title      string
		trad, simp []string
	)

	err = l.pool.QueryRow(ctx, `
		SELECT title, traditional_paragraphs, simplified_paragraphs
		FROM chapters WHERE book_id = $1 AND idx = $2`, bookID, idx).
		Scan(&title, &trad, &simp)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("chapter %q not found", req.Msg.GetName())) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	chapter := &fantiv1.Chapter{Name: req.Msg.GetName(), Title: title, Index: idx}

	if req.Msg.GetView() != fantiv1.ChapterView_CHAPTER_VIEW_FULL {
		return connect.NewResponse(chapter), nil
	}

	tok, err := newTokenizer(ctx, l.pool, trad, simp, []string{title})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, p := range trad {
		chapter.TraditionalParagraphs = append(chapter.TraditionalParagraphs, tok.paragraph(p))
	}

	for _, p := range simp {
		chapter.SimplifiedParagraphs = append(chapter.SimplifiedParagraphs, tok.paragraph(p))
	}

	return connect.NewResponse(chapter), nil
}

// DownloadBook builds an EPUB3 from the stored chapters.
func (l *Library) DownloadBook(
	ctx context.Context, req *connect.Request[fantiv1.DownloadBookRequest],
) (*connect.Response[fantiv1.DownloadBookResponse], error) {
	id, err := parseName("books", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	var title, author, script string

	err = l.pool.QueryRow(ctx,
		"SELECT title, author, script FROM books WHERE id = $1", id).
		Scan(&title, &author, &script)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("book %q not found", id)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	rows, err := l.pool.Query(ctx, `
		SELECT title, traditional_paragraphs, simplified_paragraphs
		FROM chapters WHERE book_id = $1 ORDER BY idx`, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var chapters []bookfile.Chapter

	for rows.Next() {
		var (
			chTitle    string
			trad, simp []string
		)

		if err := rows.Scan(&chTitle, &trad, &simp); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		paras := trad
		if script == scriptSimplified {
			paras = simp
		}

		chapters = append(chapters, bookfile.Chapter{Title: chTitle, Paragraphs: paras})
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	lang := "zh-TW"
	if script == scriptSimplified {
		lang = "zh-CN"
	}

	epub, err := bookfile.WriteEPUB(bookfile.EPUBMeta{
		Title: title, Author: author, Language: lang,
	}, chapters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fantiv1.DownloadBookResponse{
		Filename: title + ".epub",
		Data:     epub,
	}), nil
}

// parseChapterName parses books/{book}/chapters/{idx}.
func parseChapterName(name string) (string, int32, error) {
	parts := strings.Split(name, "/")
	if len(parts) != 4 || parts[0] != "books" || parts[2] != "chapters" {
		return "", 0, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must be books/{book}/chapters/{chapter}, got %q", name)) //nolint:err113 // request detail
	}

	idx, err := strconv.ParseInt(parts[3], 10, 32)
	if err != nil {
		return "", 0, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("chapter id must be numeric: %w", err))
	}

	return parts[1], int32(idx), nil
}

// chapterIndex validates a chapter reference belongs to the book.
func chapterIndex(bookID, chapterName string) (int32, error) {
	gotBook, idx, err := parseChapterName(chapterName)
	if err != nil {
		return 0, err
	}

	if gotBook != bookID {
		return 0, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("current_chapter %q does not belong to book %q", chapterName, bookID)) //nolint:err113 // request detail
	}

	return idx, nil
}

var errEmptyMask = errors.New("update_mask must list at least one path")
