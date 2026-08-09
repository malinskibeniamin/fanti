package seed

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHistoryRankLimit = 500
	commonsBatchSize        = 50
	maxHistorySVGBytes      = 256 << 10
	commonsAPIURL           = "https://commons.wikimedia.org/w/api.php"
	commonsUserAgent        = "Fanti/1.0 (https://github.com/malinskibeniamin/fanti)"
	historyRequestAttempts  = 5
	historyRetryDelay       = 100 * time.Millisecond
	historyRequestInterval  = 100 * time.Millisecond
)

var (
	errCommonsAPI         = errors.New("commons API error")
	errHistoryHTTPStatus  = errors.New("unexpected history HTTP status")
	errHistorySVGRoot     = errors.New("history SVG root element missing")
	errHistorySVGTooLarge = errors.New("history SVG exceeds size limit")
	errUnsafeHistorySVG   = errors.New("unsafe history SVG")
)

//nolint:gochecknoglobals // fixed immutable stage metadata
var historyStages = [...]struct {
	name   string
	order  int16
	suffix string
}{
	{name: "oracle", order: 1, suffix: "oracle"},
	{name: "bronze", order: 2, suffix: "bronze"},
	{name: "seal", order: 3, suffix: "seal"},
	{name: "clerical", order: 4, suffix: "clerical"},
	{name: "regular", order: 5},
}

// CharacterHistoryOptions controls the incremental Wikimedia Commons import.
type CharacterHistoryOptions struct {
	APIURL    string
	CacheDir  string
	Client    *http.Client
	RankLimit int
	Refresh   bool
	pacer     <-chan time.Time
}

type historyTarget struct {
	char  string
	stage string
}

type historyForm struct {
	svg         []byte
	sourceTitle string
	sourceURL   string
	sourceSHA1  string
	license     string
}

type commonsMetadata struct {
	Value json.RawMessage `json:"value"`
}

type commonsImageInfo struct {
	URL            string                     `json:"url"`
	DescriptionURL string                     `json:"descriptionurl"`
	MIME           string                     `json:"mime"`
	Size           int64                      `json:"size"`
	SHA1           string                     `json:"sha1"`
	ExtMetadata    map[string]commonsMetadata `json:"extmetadata"`
}

type commonsPage struct {
	Title     string             `json:"title"`
	Missing   bool               `json:"missing"`
	ImageInfo []commonsImageInfo `json:"imageinfo"`
}

type commonsRedirect struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type commonsResponse struct {
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
	Query struct {
		Pages     []commonsPage     `json:"pages"`
		Redirects []commonsRedirect `json:"redirects"`
	} `json:"query"`
}

// SeedCharacterHistory imports attested historical forms for the most common
// characters plus the authored character set. Five rows per checked character
// make gaps and idempotency explicit.
//
//nolint:revive // seed.Run/seed.SeedCharacterHistory read as a family
func SeedCharacterHistory(
	ctx context.Context,
	pool *pgxpool.Pool,
	options CharacterHistoryOptions,
	logger *slog.Logger,
) error {
	if options.APIURL == "" {
		options.APIURL = commonsAPIURL
	}
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 30 * time.Second}
	}
	if options.RankLimit <= 0 {
		options.RankLimit = defaultHistoryRankLimit
	}
	if options.CacheDir == "" {
		options.CacheDir = "data/downloads"
	}

	pacer := time.NewTicker(historyRequestInterval)
	defer pacer.Stop()
	options.pacer = pacer.C

	chars, err := historyCharacters(ctx, pool, options.RankLimit, options.Refresh)
	if err != nil {
		return err
	}

	if len(chars) == 0 {
		logger.InfoContext(ctx, "character history already populated, skipping")

		return nil
	}

	// Four remote stages fit 12 characters into Commons' 50-title batch.
	charsPerBatch := commonsBatchSize / (len(historyStages) - 1)
	for start := 0; start < len(chars); start += charsPerBatch {
		end := min(start+charsPerBatch, len(chars))
		if err := seedHistoryBatch(ctx, pool, options, chars[start:end]); err != nil {
			return fmt.Errorf("seed character history batch: %w", err)
		}
	}

	logger.InfoContext(ctx, "seeded character history", slog.Int("characters", len(chars)))

	return nil
}

func historyCharacters(
	ctx context.Context,
	pool *pgxpool.Pool,
	rankLimit int,
	refresh bool,
) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT c.traditional
		FROM characters c
		WHERE (c.frequency_rank BETWEEN 1 AND $1 OR c.story <> '')
			AND ($2 OR NOT EXISTS (
				SELECT 1 FROM character_history h
				WHERE h.ch = c.traditional AND h.stage = 'regular'
			))
		ORDER BY c.frequency_rank = 0, c.frequency_rank, c.traditional`,
		rankLimit, refresh)
	if err != nil {
		return nil, fmt.Errorf("list character history scope: %w", err)
	}
	defer rows.Close()

	var chars []string
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, fmt.Errorf("scan character history scope: %w", err)
		}
		chars = append(chars, ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list character history scope: %w", err)
	}

	return chars, nil
}

func seedHistoryBatch(
	ctx context.Context,
	pool *pgxpool.Pool,
	options CharacterHistoryOptions,
	chars []string,
) error {
	targets := make(map[string]historyTarget, len(chars)*(len(historyStages)-1))
	titles := make([]string, 0, len(targets))

	for _, ch := range chars {
		for _, stage := range historyStages[:len(historyStages)-1] {
			title := fmt.Sprintf("File:%s-%s.svg", ch, stage.suffix)
			targets[title] = historyTarget{char: ch, stage: stage.name}
			titles = append(titles, title)
		}
	}

	response, err := queryCommons(ctx, options, titles)
	if err != nil {
		return err
	}

	for _, redirect := range response.Query.Redirects {
		if target, ok := targets[redirect.From]; ok {
			targets[redirect.To] = target
		}
	}

	forms := make(map[historyTarget]historyForm)
	for _, page := range response.Query.Pages {
		target, ok := targets[page.Title]
		if !ok || page.Missing || len(page.ImageInfo) == 0 {
			continue
		}

		info := page.ImageInfo[0]
		license := commonsMetadataString(info.ExtMetadata, "LicenseShortName")
		copyrighted := commonsMetadataString(info.ExtMetadata, "Copyrighted")
		if info.MIME != "image/svg+xml" || info.Size <= 0 ||
			info.Size > maxHistorySVGBytes || !allowedHistoryLicense(license) ||
			copyrighted != "False" || !validCommonsSHA1(info.SHA1) ||
			!allowedHistoryImageURL(options.APIURL, info.URL) ||
			!allowedCommonsPageURL(info.DescriptionURL) {
			continue
		}

		svg, err := cachedHistorySVG(ctx, options, info)
		if err != nil {
			return fmt.Errorf("download %s: %w", page.Title, err)
		}

		forms[target] = historyForm{
			svg:         svg,
			sourceTitle: page.Title,
			sourceURL:   info.DescriptionURL,
			sourceSHA1:  info.SHA1,
			license:     license,
		}
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin character history batch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, ch := range chars {
		for _, stage := range historyStages {
			form := forms[historyTarget{char: ch, stage: stage.name}]
			if _, err := tx.Exec(ctx, `
				INSERT INTO character_history (
					ch, stage, stage_order, svg, source_title, source_url,
					source_sha1, license, checked_at
				) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())
				ON CONFLICT (ch, stage) DO UPDATE SET
					stage_order = EXCLUDED.stage_order,
					svg = EXCLUDED.svg,
					source_title = EXCLUDED.source_title,
					source_url = EXCLUDED.source_url,
					source_sha1 = EXCLUDED.source_sha1,
					license = EXCLUDED.license,
					checked_at = now()`,
				ch, stage.name, stage.order, form.svg, form.sourceTitle,
				form.sourceURL, form.sourceSHA1, form.license); err != nil {
				return fmt.Errorf("upsert %s %s: %w", ch, stage.name, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit character history batch: %w", err)
	}

	return nil
}

func queryCommons(
	ctx context.Context,
	options CharacterHistoryOptions,
	titles []string,
) (commonsResponse, error) {
	endpoint, err := url.Parse(options.APIURL)
	if err != nil {
		return commonsResponse{}, fmt.Errorf("parse Commons API URL: %w", err)
	}

	query := endpoint.Query()
	query.Set("action", "query")
	query.Set("format", "json")
	query.Set("formatversion", "2")
	query.Set("maxlag", "5")
	query.Set("prop", "imageinfo")
	query.Set("iiprop", "url|mime|size|sha1|extmetadata")
	query.Set("redirects", "1")
	query.Set("titles", strings.Join(titles, "|"))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return commonsResponse{}, fmt.Errorf("build Commons API request: %w", err)
	}
	req.Header.Set("User-Agent", commonsUserAgent)

	if options.pacer != nil {
		select {
		case <-ctx.Done():
			return commonsResponse{}, ctx.Err()
		case <-options.pacer:
		}
	}

	resp, err := doHistoryRequest(ctx, options.Client, req)
	if err != nil {
		return commonsResponse{}, fmt.Errorf("query Commons API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return commonsResponse{}, fmt.Errorf("query Commons API: %w: %s",
			errHistoryHTTPStatus, resp.Status)
	}

	var result commonsResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result); err != nil {
		return commonsResponse{}, fmt.Errorf("decode Commons API response: %w", err)
	}
	if result.Error != nil {
		return commonsResponse{}, fmt.Errorf("%w %s: %s",
			errCommonsAPI, result.Error.Code, result.Error.Info)
	}

	return result, nil
}

func allowedHistoryLicense(license string) bool {
	return license == "Public domain" || strings.HasPrefix(license, "CC0")
}

func commonsMetadataString(metadata map[string]commonsMetadata, key string) string {
	var value string
	if err := json.Unmarshal(metadata[key].Value, &value); err != nil {
		return ""
	}

	return value
}

func validCommonsSHA1(value string) bool {
	if len(value) != 40 {
		return false
	}

	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}

	return true
}

func allowedHistoryImageURL(apiURL, imageURL string) bool {
	api, apiErr := url.Parse(apiURL)
	image, imageErr := url.Parse(imageURL)
	if apiErr != nil || imageErr != nil || image.Host == "" {
		return false
	}

	if image.Scheme == "https" && image.Hostname() == "upload.wikimedia.org" {
		return true
	}

	return image.Scheme == api.Scheme && image.Host == api.Host
}

func allowedCommonsPageURL(value string) bool {
	page, err := url.Parse(value)

	return err == nil && page.Scheme == "https" &&
		page.Hostname() == "commons.wikimedia.org" &&
		strings.HasPrefix(page.Path, "/wiki/File:")
}

func cachedHistorySVG(
	ctx context.Context,
	options CharacterHistoryOptions,
	info commonsImageInfo,
) ([]byte, error) {
	cacheDir := filepath.Join(options.CacheDir, "character-history")
	if err := os.MkdirAll(cacheDir, 0o750); err != nil {
		return nil, fmt.Errorf("create character history cache: %w", err)
	}

	path := filepath.Join(cacheDir, info.SHA1+".svg")
	if svg, err := os.ReadFile(path); err == nil { //nolint:gosec // checksum-derived path
		if err := validateHistorySVG(svg); err != nil {
			return nil, fmt.Errorf("validate cached SVG: %w", err)
		}

		return svg, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read cached SVG: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build SVG request: %w", err)
	}
	req.Header.Set("User-Agent", commonsUserAgent)

	if options.pacer != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-options.pacer:
		}
	}

	resp, err := doHistoryRequest(ctx, options.Client, req)
	if err != nil {
		return nil, fmt.Errorf("fetch SVG: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch SVG: %w: %s", errHistoryHTTPStatus, resp.Status)
	}

	svg, err := io.ReadAll(io.LimitReader(resp.Body, maxHistorySVGBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read SVG: %w", err)
	}
	if len(svg) > maxHistorySVGBytes {
		return nil, fmt.Errorf("read SVG: %w: %d bytes",
			errHistorySVGTooLarge, maxHistorySVGBytes)
	}
	if err := validateHistorySVG(svg); err != nil {
		return nil, err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, svg, 0o600); err != nil {
		return nil, fmt.Errorf("cache SVG: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return nil, fmt.Errorf("finalize SVG cache: %w", err)
	}

	return svg, nil
}

func doHistoryRequest(
	ctx context.Context,
	client *http.Client,
	req *http.Request,
) (*http.Response, error) {
	var lastErr error

	for attempt := range historyRequestAttempts {
		delay := historyRetryDelay * time.Duration(1<<attempt)
		resp, err := client.Do(req.Clone(ctx))
		switch {
		case err != nil:
			lastErr = err
		case !retryableHistoryStatus(resp.StatusCode):
			return resp, nil
		case attempt == historyRequestAttempts-1:
			return resp, nil
		default:
			delay = nextHistoryRetryDelay(resp, delay)
			_ = resp.Body.Close()
		}

		if attempt == historyRequestAttempts-1 {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()

			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, lastErr
}

func nextHistoryRetryDelay(response *http.Response, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(response.Header.Get("Retry-After"))
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return max(fallback, time.Duration(seconds)*time.Second)
	}

	if retryAt, err := http.ParseTime(value); err == nil {
		return max(fallback, time.Until(retryAt))
	}

	return fallback
}

func retryableHistoryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func validateHistorySVG(svg []byte) error {
	decoder := xml.NewDecoder(strings.NewReader(string(svg)))
	foundRoot := false

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("parse SVG: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			if !foundRoot {
				if !strings.EqualFold(value.Name.Local, "svg") {
					return errHistorySVGRoot
				}
				foundRoot = true
			}
			if err := validateHistoryElement(value); err != nil {
				return err
			}
		case xml.Directive:
			if !allowedHistoryDirective(string(value)) {
				return fmt.Errorf("%w: doctype", errUnsafeHistorySVG)
			}
		}
	}

	if !foundRoot {
		return errHistorySVGRoot
	}

	return nil
}

func allowedHistoryDirective(value string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(value), " "))

	return strings.HasPrefix(normalized, `doctype svg public "-//w3c//dtd svg `) &&
		strings.Contains(normalized, `"http://www.w3.org/tr/`) &&
		strings.HasSuffix(normalized, `.dtd"`) &&
		!strings.Contains(normalized, "[") &&
		!strings.Contains(normalized, "entity")
}

func validateHistoryElement(element xml.StartElement) error {
	name := strings.ToLower(element.Name.Local)
	if name == "script" || name == "foreignobject" {
		return fmt.Errorf("%w element %q", errUnsafeHistorySVG, element.Name.Local)
	}

	for _, attr := range element.Attr {
		attrName := strings.ToLower(attr.Name.Local)
		attrValue := strings.ToLower(strings.TrimSpace(attr.Value))
		if strings.HasPrefix(attrName, "on") ||
			((attrName == "href" || attrName == "src") &&
				attrValue != "" && !strings.HasPrefix(attrValue, "#")) ||
			strings.Contains(attrValue, "javascript:") ||
			(attrName == "style" &&
				(strings.Contains(attrValue, "http:") ||
					strings.Contains(attrValue, "https:") ||
					strings.Contains(attrValue, "@import"))) {
			return fmt.Errorf("%w attribute %q", errUnsafeHistorySVG, attr.Name.Local)
		}
	}

	return nil
}
