package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/internal/bookfile"
	"github.com/malinskibeniamin/fanti/backend/internal/convert"
)

// Conversions serves fanti.v1.ConversionService.
type Conversions struct {
	pool   *pgxpool.Pool
	engine *convert.Engine
	logger *slog.Logger
}

// NewConversions builds the conversion service.
func NewConversions(pool *pgxpool.Pool, engine *convert.Engine, logger *slog.Logger) *Conversions {
	return &Conversions{pool: pool, engine: engine, logger: logger}
}

// storedSettings is the settings JSONB shape.
type storedSettings struct {
	Direction   string            `json:"direction"`
	Localize    bool              `json:"localize"`
	Punctuation bool              `json:"punctuation"`
	Resolutions map[string]string `json:"resolutions,omitempty"`
}

// storedLayout is the layout JSONB shape.
type storedLayout struct {
	Title           string   `json:"title"`
	Author          string   `json:"author"`
	CoverColor      string   `json:"coverColor"`
	TitleFont       string   `json:"titleFont"`
	BodyFont        string   `json:"bodyFont"`
	ChapterTitles   []string `json:"chapterTitles"`
	FrontMatter     string   `json:"frontMatter"`
	IndentFirstLine bool     `json:"indentFirstLine"`
}

// storedChapter is the source/result JSONB chapter shape.
type storedChapter struct {
	Title      string   `json:"title"`
	Paragraphs []string `json:"paragraphs"`
}

// storedReport is the report JSONB shape.
type storedReport struct {
	Exact      int64               `json:"exact"`
	Ambiguous  int64               `json:"ambiguous"`
	Manual     int64               `json:"manual"`
	Total      int64               `json:"total"`
	Exceptions []convert.Exception `json:"exceptions"`
	Diff       convert.Diff        `json:"diff"`
}

// chapterUnits are the section-marker characters the analyzer counts.
const chapterUnits = "章節回篇卷部"

// CreateConversion uploads and analyzes a file.
func (c *Conversions) CreateConversion(
	ctx context.Context, req *connect.Request[fantiv1.CreateConversionRequest],
) (*connect.Response[fantiv1.Conversion], error) {
	filename := req.Msg.GetFilename()
	if filename == "" || len(req.Msg.GetData()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errUploadRequired)
	}

	parsed, err := bookfile.Parse(filename, req.Msg.GetData())

	switch {
	case errors.Is(err, bookfile.ErrUnsupportedFormat),
		errors.Is(err, bookfile.ErrDRM),
		errors.Is(err, bookfile.ErrHuffCompression):
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if parsed.CharCount == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errEmptyBook)
	}

	sample := analysisSample(parsed)

	direction, err := c.engine.DetectScript(sample)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	id := req.Msg.GetConversionId()
	if id == "" {
		id = newConversionID()
	}

	title := parsed.Title
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	}

	// Convert title/author/front matter into the target script so the
	// layout editors start in the output language.
	frontMatter := "繁體中文版　由繁体 Fanti 自简体原檔轉換。一字多繁之處已依語境對應，詳見轉換報告。"
	if direction == convert.T2S {
		frontMatter = "简体中文版　由繁体 Fanti 自繁體原档转换，详见转换报告。"
	}

	outTitle, err := c.engine.ConvertText(title, convert.Options{Direction: direction})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	outAuthor, err := c.engine.ConvertText(parsed.Author, convert.Options{Direction: direction})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	chapterTitles := make([]string, len(parsed.Chapters))
	for i, ch := range parsed.Chapters {
		chapterTitles[i] = ch.Title
	}

	settings := storedSettings{
		Direction: string(direction), Localize: true, Punctuation: true,
	}
	layout := storedLayout{
		Title: outTitle, Author: outAuthor, CoverColor: "#8f1d18",
		TitleFont: "kai", BodyFont: "serif",
		ChapterTitles: chapterTitles, FrontMatter: frontMatter,
		IndentFirstLine: true,
	}

	source := make([]storedChapter, len(parsed.Chapters))
	for i, ch := range parsed.Chapters {
		source[i] = storedChapter{Title: ch.Title, Paragraphs: ch.Paragraphs}
	}

	settingsJSON, layoutJSON, sourceJSON, err := marshalAll(settings, layout, source)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	unitCounts := countUnits(parsed)

	unitsJSON, err := json.Marshal(unitCounts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	detected := scriptTraditional
	if direction == convert.S2T {
		detected = scriptSimplified
	}

	if _, err := c.pool.Exec(ctx, `
		INSERT INTO conversions (
			id, state, filename, format, detected_script, char_count,
			unit_counts, settings, layout, source
		) VALUES ($1, 'ready', $2, $3, $4, $5, $6, $7, $8, $9)`,
		id, filename, string(parsed.Format), detected, parsed.CharCount,
		unitsJSON, settingsJSON, layoutJSON, sourceJSON); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return c.getConversion(ctx, id)
}

// GetConversion returns the job resource; clients poll it while RUNNING.
func (c *Conversions) GetConversion(
	ctx context.Context, req *connect.Request[fantiv1.GetConversionRequest],
) (*connect.Response[fantiv1.Conversion], error) {
	id, err := parseName("conversions", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	return c.getConversion(ctx, id)
}

// ListConversions lists jobs newest first.
func (c *Conversions) ListConversions(
	ctx context.Context, _ *connect.Request[fantiv1.ListConversionsRequest],
) (*connect.Response[fantiv1.ListConversionsResponse], error) {
	rows, err := c.pool.Query(ctx,
		"SELECT id FROM conversions ORDER BY create_time DESC LIMIT 100")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.ListConversionsResponse{}

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		conv, err := c.getConversion(ctx, id)
		if err != nil {
			return nil, err
		}

		resp.Conversions = append(resp.Conversions, conv.Msg)
	}

	return connect.NewResponse(resp), rows.Err()
}

// UpdateConversion updates settings and layout while READY.
func (c *Conversions) UpdateConversion(
	ctx context.Context, req *connect.Request[fantiv1.UpdateConversionRequest],
) (*connect.Response[fantiv1.Conversion], error) {
	id, err := parseName("conversions", req.Msg.GetConversion().GetName())
	if err != nil {
		return nil, err
	}

	paths := req.Msg.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errEmptyMask)
	}

	for _, p := range paths {
		switch p {
		case "settings":
			msg := req.Msg.GetConversion().GetSettings()
			settings := storedSettings{
				Direction:   directionFromProto(msg.GetDirection()),
				Localize:    msg.GetLocalizeVocabulary(),
				Punctuation: msg.GetConvertPunctuation(),
			}

			raw, err := json.Marshal(settings)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Direction changes invalidate previous resolutions, which is
			// exactly what rebuilding the JSON without them does.
			if _, err := c.pool.Exec(ctx, `
				UPDATE conversions SET settings = $2, update_time = now()
				WHERE id = $1 AND state IN ('ready','succeeded')`, id, raw); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

		case "layout":
			msg := req.Msg.GetConversion().GetLayout()
			layout := storedLayout{
				Title: msg.GetTitle(), Author: msg.GetAuthor(),
				CoverColor: msg.GetCoverColor(), TitleFont: msg.GetTitleFont(),
				BodyFont: msg.GetBodyFont(), ChapterTitles: msg.GetChapterTitles(),
				FrontMatter: msg.GetFrontMatter(), IndentFirstLine: msg.GetIndentFirstLine(),
			}

			raw, err := json.Marshal(layout)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if _, err := c.pool.Exec(ctx, `
				UPDATE conversions SET layout = $2, update_time = now()
				WHERE id = $1 AND state IN ('ready','succeeded')`, id, raw); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

		default:
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("update_mask path %q is not updatable", p)) //nolint:err113 // request detail
		}
	}

	return c.getConversion(ctx, id)
}

// DeleteConversion removes a job.
func (c *Conversions) DeleteConversion(
	ctx context.Context, req *connect.Request[fantiv1.DeleteConversionRequest],
) (*connect.Response[emptypb.Empty], error) {
	id, err := parseName("conversions", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	tag, err := c.pool.Exec(ctx, "DELETE FROM conversions WHERE id = $1", id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("conversion %q not found", id)) //nolint:err113 // request detail
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// RunConversion starts the async job; poll GetConversion for progress.
func (c *Conversions) RunConversion(
	ctx context.Context, req *connect.Request[fantiv1.RunConversionRequest],
) (*connect.Response[fantiv1.Conversion], error) {
	id, err := parseName("conversions", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	// Single-flight: only one transition into running.
	tag, err := c.pool.Exec(ctx, `
		UPDATE conversions SET state = 'running', progress_percent = 0,
			error_message = '', update_time = now()
		WHERE id = $1 AND state IN ('ready','succeeded','failed')`, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("conversion %q is not runnable (missing or already running)", id)) //nolint:err113 // request detail
	}

	// The job outlives the request deliberately — it is an LRO.
	go c.runJob(id) //nolint:contextcheck,gosec // detached job context

	return c.getConversion(ctx, id)
}

// runJob executes the conversion outside the request lifecycle.
func (c *Conversions) runJob(id string) {
	ctx := context.Background()

	chapters, opts, _, err := c.loadSource(ctx, id)
	if err != nil {
		c.failJob(ctx, id, err)

		return
	}

	lastPct := -1

	converted, report, err := c.engine.ConvertChapters(chapters, opts, func(done, total int) {
		pct := done * 100 / total
		if pct == lastPct || pct%2 != 0 {
			return
		}

		lastPct = pct

		_, _ = c.pool.Exec(ctx,
			"UPDATE conversions SET progress_percent = $2 WHERE id = $1", id, pct)
	})
	if err != nil {
		c.failJob(ctx, id, err)

		return
	}

	result := make([]storedChapter, len(converted))
	for i, ch := range converted {
		result[i] = storedChapter{Title: ch.Title, Paragraphs: ch.Paragraphs}
	}

	stored := storedReport{
		Exact: report.Exact, Ambiguous: report.Ambiguous, Manual: report.Manual,
		Total: report.Total, Exceptions: report.Exceptions, Diff: report.Diff,
	}

	reportJSON, err := json.Marshal(stored)
	if err != nil {
		c.failJob(ctx, id, err)

		return
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		c.failJob(ctx, id, err)

		return
	}

	if _, err := c.pool.Exec(ctx, `
		UPDATE conversions SET state = 'succeeded', progress_percent = 100,
			report = $2, result = $3, update_time = now()
		WHERE id = $1`, id, reportJSON, resultJSON); err != nil {
		c.logger.ErrorContext(ctx, "store conversion result",
			slog.String("conversion", id), slog.Any("error", err))
	}
}

func (c *Conversions) failJob(ctx context.Context, id string, cause error) {
	c.logger.WarnContext(ctx, "conversion failed",
		slog.String("conversion", id), slog.Any("error", cause))

	_, err := c.pool.Exec(ctx, `
		UPDATE conversions SET state = 'failed', error_message = $2, update_time = now()
		WHERE id = $1`, id, cause.Error())
	if err != nil {
		c.logger.ErrorContext(ctx, "store conversion failure",
			slog.String("conversion", id), slog.Any("error", err))
	}
}

// ResolveException records the chosen form for an ambiguous character.
func (c *Conversions) ResolveException(
	ctx context.Context, req *connect.Request[fantiv1.ResolveExceptionRequest],
) (*connect.Response[fantiv1.Conversion], error) {
	id, err := parseName("conversions", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	if req.Msg.GetSourceChar() == "" || req.Msg.GetResolved() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errResolutionRequired)
	}

	var reportRaw []byte
	if err := c.pool.QueryRow(ctx,
		"SELECT COALESCE(report, 'null') FROM conversions WHERE id = $1", id).Scan(&reportRaw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound,
				fmt.Errorf("conversion %q not found", id)) //nolint:err113 // request detail
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var report *storedReport
	if err := json.Unmarshal(reportRaw, &report); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	valid := false
	if report != nil && utf8.RuneCountInString(req.Msg.GetResolved()) == 1 {
		for _, exception := range report.Exceptions {
			if exception.SourceChar == req.Msg.GetSourceChar() &&
				slices.Contains(exception.Options, req.Msg.GetResolved()) {
				valid = true

				break
			}
		}
	}

	if !valid {
		return nil, connect.NewError(connect.CodeInvalidArgument, errInvalidResolution)
	}

	resolution, err := json.Marshal(map[string]string{
		req.Msg.GetSourceChar(): req.Msg.GetResolved(),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tag, err := c.pool.Exec(ctx, `
		UPDATE conversions
		SET settings = jsonb_set(settings, '{resolutions}',
			COALESCE(settings->'resolutions', '{}'::jsonb) || $2::jsonb),
			update_time = now()
		WHERE id = $1`, id, resolution)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if tag.RowsAffected() == 0 {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("conversion %q not found", id)) //nolint:err113 // request detail
	}

	return c.getConversion(ctx, id)
}

// ExportConversion converts with current settings and returns EPUB3 bytes.
func (c *Conversions) ExportConversion(
	ctx context.Context, req *connect.Request[fantiv1.ExportConversionRequest],
) (*connect.Response[fantiv1.ExportConversionResponse], error) {
	id, err := parseName("conversions", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	epub, layout, err := c.buildEpub(ctx, id)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&fantiv1.ExportConversionResponse{
		Filename: layout.Title + ".epub",
		Data:     epub,
	}), nil
}

// AddConversionToLibrary stores the converted book with both scripts:
// the source text provides one, the conversion the other, powering the
// reader's 繁/简 toggle.
func (c *Conversions) AddConversionToLibrary(
	ctx context.Context, req *connect.Request[fantiv1.AddConversionToLibraryRequest],
) (*connect.Response[fantiv1.Book], error) {
	id, err := parseName("conversions", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	chapters, opts, layout, err := c.loadSource(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	converted, _, err := c.engine.ConvertChapters(chapters, opts, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	epub, _, err := c.buildEpub(ctx, id)
	if err != nil {
		return nil, err
	}

	script := scriptTraditional
	if opts.Direction == convert.T2S {
		script = scriptSimplified
	}

	var charCount int64

	for _, ch := range converted {
		for _, p := range ch.Paragraphs {
			charCount += int64(len([]rune(p)))
		}
	}

	sizeLabel := fmt.Sprintf("EPUB3 · %.0f kB", float64(len(epub))/1024)

	meta, err := json.Marshal([]map[string]string{
		{"label": "轉換 Conversion", "value": string(opts.Direction)},
		{"label": "檔案 File", "value": sizeLabel},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		INSERT INTO books (id, title, author, script, source_format,
			cover_color, description, file_size_label, metadata_fields, char_count)
		VALUES ($1,$2,$3,$4,'epub',$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title,
			author = EXCLUDED.author, script = EXCLUDED.script,
			source_format = EXCLUDED.source_format,
			cover_color = EXCLUDED.cover_color,
			description = EXCLUDED.description,
			file_size_label = EXCLUDED.file_size_label,
			metadata_fields = EXCLUDED.metadata_fields,
			char_count = EXCLUDED.char_count, update_time = now()`,
		id, layout.Title, layout.Author, script, layout.CoverColor,
		"由繁体 Fanti 轉換。Converted with Fanti.", sizeLabel, meta, charCount); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := tx.Exec(ctx, "DELETE FROM chapters WHERE book_id = $1", id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for i, ch := range converted {
		trad, simp := ch.Paragraphs, chapters[i].Paragraphs
		tradTitle, simpTitle := ch.Title, chapters[i].Title

		if opts.Direction == convert.T2S {
			trad, simp = simp, trad
			tradTitle, simpTitle = simpTitle, tradTitle
		}

		title := tradTitle
		if title == "" {
			title = simpTitle
		}
		if i < len(layout.ChapterTitles) && layout.ChapterTitles[i] != "" {
			title = layout.ChapterTitles[i]
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO chapters (book_id, idx, title, traditional_paragraphs, simplified_paragraphs)
			VALUES ($1,$2,$3,$4,$5)`,
			id, i, title, trad, simp); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	book, err := scanBook(c.pool.QueryRow(ctx,
		"SELECT"+bookColumns+" FROM books b WHERE b.id = $1", id))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(book), nil
}

// --- helpers ---

func (c *Conversions) getConversion(ctx context.Context, id string) (*connect.Response[fantiv1.Conversion], error) {
	var (
		state, filename, format, detected, errMsg string
		charCount                                 int64
		progress                                  int32
		unitsRaw, settingsRaw, layoutRaw          []byte
		reportRaw                                 []byte
		createTime, updateTime                    time.Time
	)

	err := c.pool.QueryRow(ctx, `
		SELECT state, filename, format, detected_script, char_count,
			unit_counts, settings, layout, progress_percent,
			COALESCE(report, 'null'), error_message, create_time, update_time
		FROM conversions WHERE id = $1`, id).
		Scan(&state, &filename, &format, &detected, &charCount, &unitsRaw,
			&settingsRaw, &layoutRaw, &progress, &reportRaw, &errMsg,
			&createTime, &updateTime)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("conversion %q not found", id)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	conv := &fantiv1.Conversion{
		Name:            "conversions/" + id,
		State:           conversionState(state),
		Filename:        filename,
		Format:          formatEnum[format],
		DetectedScript:  scriptEnum[detected],
		CharCount:       charCount,
		ProgressPercent: progress,
		ErrorMessage:    errMsg,
		CreateTime:      timestamppb.New(createTime),
		UpdateTime:      timestamppb.New(updateTime),
	}

	var units []struct {
		Unit  string `json:"unit"`
		Count int32  `json:"count"`
	}
	if err := json.Unmarshal(unitsRaw, &units); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, u := range units {
		conv.UnitCounts = append(conv.UnitCounts, &fantiv1.UnitCount{Unit: u.Unit, Count: u.Count})
	}

	var settings storedSettings
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	conv.Settings = &fantiv1.ConversionSettings{
		Direction:          directionToProto(settings.Direction),
		LocalizeVocabulary: settings.Localize,
		ConvertPunctuation: settings.Punctuation,
	}

	var layout storedLayout
	if err := json.Unmarshal(layoutRaw, &layout); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	conv.Layout = &fantiv1.BookLayout{
		Title: layout.Title, Author: layout.Author, CoverColor: layout.CoverColor,
		TitleFont: layout.TitleFont, BodyFont: layout.BodyFont,
		ChapterTitles: layout.ChapterTitles, FrontMatter: layout.FrontMatter,
		IndentFirstLine: layout.IndentFirstLine,
	}

	var report *storedReport
	if err := json.Unmarshal(reportRaw, &report); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if report != nil {
		conv.Report = reportToProto(*report, settings.Resolutions)
	}

	return connect.NewResponse(conv), nil
}

func reportToProto(r storedReport, resolutions map[string]string) *fantiv1.ConversionReport {
	out := &fantiv1.ConversionReport{
		ExactCount:         r.Exact,
		AmbiguousCount:     r.Ambiguous,
		ManualCount:        r.Manual,
		TotalSubstitutions: r.Total,
		Diff:               &fantiv1.DiffPreview{SourceText: r.Diff.SourceText},
	}

	for _, ex := range r.Exceptions {
		out.Exceptions = append(out.Exceptions, &fantiv1.ConversionException{
			SourceChar: ex.SourceChar,
			Options:    ex.Options,
			Note:       &fantiv1.LocalizedText{En: ex.Note.En, Tc: ex.Note.Tc, Sc: ex.Note.Sc},
			Context:    ex.Context,
			Status:     mappingStatus[ex.Status],
			Resolved:   resolutions[ex.SourceChar],
		})
	}

	for _, tk := range r.Diff.Tokens {
		out.Diff.Tokens = append(out.Diff.Tokens, &fantiv1.DiffToken{
			Text:   tk.Text,
			Status: mappingStatus[tk.Status],
		})
	}

	return out
}

// loadSource reads the parsed chapters and conversion options for a job.
func (c *Conversions) loadSource(ctx context.Context, id string) ([]convert.Chapter, convert.Options, storedLayout, error) {
	var sourceRaw, settingsRaw, layoutRaw []byte

	err := c.pool.QueryRow(ctx,
		"SELECT source, settings, layout FROM conversions WHERE id = $1", id).
		Scan(&sourceRaw, &settingsRaw, &layoutRaw)
	if err != nil {
		return nil, convert.Options{}, storedLayout{}, fmt.Errorf("load conversion %s: %w", id, err)
	}

	var stored []storedChapter
	if err := json.Unmarshal(sourceRaw, &stored); err != nil {
		return nil, convert.Options{}, storedLayout{}, fmt.Errorf("decode source: %w", err)
	}

	var settings storedSettings
	if err := json.Unmarshal(settingsRaw, &settings); err != nil {
		return nil, convert.Options{}, storedLayout{}, fmt.Errorf("decode settings: %w", err)
	}

	var layout storedLayout
	if err := json.Unmarshal(layoutRaw, &layout); err != nil {
		return nil, convert.Options{}, storedLayout{}, fmt.Errorf("decode layout: %w", err)
	}

	chapters := make([]convert.Chapter, len(stored))
	for i, ch := range stored {
		chapters[i] = convert.Chapter{Title: ch.Title, Paragraphs: ch.Paragraphs}
	}

	opts := convert.Options{
		Direction:   convert.Direction(settings.Direction),
		Localize:    settings.Localize,
		Punctuation: settings.Punctuation,
		Resolutions: settings.Resolutions,
	}

	return chapters, opts, layout, nil
}

// buildEpub converts with the current settings synchronously (fast: a full
// novel benchmarks at ~270ms) and packages EPUB3.
func (c *Conversions) buildEpub(ctx context.Context, id string) ([]byte, storedLayout, error) {
	chapters, opts, layout, err := c.loadSource(ctx, id)
	if err != nil {
		return nil, storedLayout{}, connect.NewError(connect.CodeInternal, err)
	}

	converted, _, err := c.engine.ConvertChapters(chapters, opts, nil)
	if err != nil {
		return nil, storedLayout{}, connect.NewError(connect.CodeInternal, err)
	}

	out := make([]bookfile.Chapter, len(converted))
	for i, ch := range converted {
		title := ch.Title
		if i < len(layout.ChapterTitles) && layout.ChapterTitles[i] != "" {
			title = layout.ChapterTitles[i]
		}

		out[i] = bookfile.Chapter{Title: title, Paragraphs: ch.Paragraphs}
	}

	lang := "zh-TW"
	if opts.Direction == convert.T2S {
		lang = "zh-CN"
	}

	epub, err := bookfile.WriteEPUB(bookfile.EPUBMeta{
		Title: layout.Title, Author: layout.Author, Language: lang,
		FrontMatter: layout.FrontMatter, IndentFirstLine: layout.IndentFirstLine,
	}, out)
	if err != nil {
		return nil, storedLayout{}, connect.NewError(connect.CodeInternal, err)
	}

	return epub, layout, nil
}

func analysisSample(p bookfile.Parsed) string {
	var b strings.Builder

	for _, ch := range p.Chapters {
		for _, para := range ch.Paragraphs {
			b.WriteString(para)

			if b.Len() > 4000 {
				return b.String()
			}
		}
	}

	return b.String()
}

func countUnits(p bookfile.Parsed) []map[string]any {
	counts := map[rune]int{}

	for _, ch := range p.Chapters {
		for _, r := range ch.Title {
			if strings.ContainsRune(chapterUnits, r) {
				counts[r]++

				break
			}
		}
	}

	out := make([]map[string]any, 0, len(chapterUnits))
	for _, u := range chapterUnits {
		out = append(out, map[string]any{"unit": string(u), "count": counts[u]})
	}

	return out
}

func conversionState(s string) fantiv1.Conversion_State {
	switch s {
	case "ready":
		return fantiv1.Conversion_STATE_READY
	case "running":
		return fantiv1.Conversion_STATE_RUNNING
	case "succeeded":
		return fantiv1.Conversion_STATE_SUCCEEDED
	case "failed":
		return fantiv1.Conversion_STATE_FAILED
	default:
		return fantiv1.Conversion_STATE_UNSPECIFIED
	}
}

func directionFromProto(d fantiv1.ConversionDirection) string {
	if d == fantiv1.ConversionDirection_CONVERSION_DIRECTION_T2S {
		return "t2s"
	}

	return "s2t"
}

func directionToProto(d string) fantiv1.ConversionDirection {
	if d == "t2s" {
		return fantiv1.ConversionDirection_CONVERSION_DIRECTION_T2S
	}

	return fantiv1.ConversionDirection_CONVERSION_DIRECTION_S2T
}

func marshalAll(a, b, c any) ([]byte, []byte, []byte, error) {
	aj, err := json.Marshal(a)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal: %w", err)
	}

	bj, err := json.Marshal(b)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal: %w", err)
	}

	cj, err := json.Marshal(c)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("marshal: %w", err)
	}

	return aj, bj, cj, nil
}

func newConversionID() string {
	var b [6]byte

	_, _ = rand.Read(b[:])

	return "cv" + hex.EncodeToString(b[:])
}

var (
	errUploadRequired     = errors.New("filename and data are required")
	errEmptyBook          = errors.New("book contains no readable text")
	errResolutionRequired = errors.New("source_char and resolved are required")
	errInvalidResolution  = errors.New("resolved must be a single-character option from the conversion report")
)
