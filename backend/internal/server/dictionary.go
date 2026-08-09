package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

const (
	defaultPageSize      = 50
	regularCharacterForm = "regular"
)

// Dictionary serves fanti.v1.DictionaryService.
type Dictionary struct {
	pool *pgxpool.Pool
}

// NewDictionary builds the dictionary service.
func NewDictionary(pool *pgxpool.Pool) *Dictionary {
	return &Dictionary{pool: pool}
}

//nolint:gochecknoglobals // static proto enum mapping
var mappingStatus = map[string]fantiv1.MappingStatus{
	"exact":     fantiv1.MappingStatus_MAPPING_STATUS_EXACT,
	"ambiguous": fantiv1.MappingStatus_MAPPING_STATUS_AMBIGUOUS,
	"manual":    fantiv1.MappingStatus_MAPPING_STATUS_MANUAL,
}

//nolint:gochecknoglobals // static proto enum mapping
var characterCatalogKind = map[string]fantiv1.CharacterCatalogKind{
	"curriculum": fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_CURRICULUM,
	"reference":  fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_REFERENCE,
}

//nolint:gochecknoglobals // fixed chronological stage mapping
var characterFormStages = []struct {
	name  string
	stage fantiv1.CharacterFormStage
}{
	{"oracle", fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_ORACLE},
	{"bronze", fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_BRONZE},
	{"seal", fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_SEAL},
	{"clerical", fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_CLERICAL},
	{regularCharacterForm, fantiv1.CharacterFormStage_CHARACTER_FORM_STAGE_REGULAR},
}

const characterColumns = `
	c.traditional, c.simplified, c.pinyin, c.zhuyin, c.pos, c.meaning,
	c.mapping_status, c.stroke_count, c.hsk_level, c.frequency_rank, c.topics,
	c.story, c.mnemonic, c.simplification_note, c.radical_parts, c.examples,
	c.siblings, c.catalog_kind, c.curriculum_rank,
	COALESCE(r.learned, FALSE), COALESCE(r.mistake_count, 0)`

const characterFrom = ` FROM characters c LEFT JOIN reviews r ON r.ch = c.traditional `

func scanCharacter(row pgx.Row) (*fantiv1.Character, error) {
	var (
		ch                    fantiv1.Character
		status                string
		catalogKind           string
		radicalsRaw, exampRaw []byte
		topics, siblings      []string
		strokes, hsk, freq    int32
		curriculumRank        int32
		learned               bool
		mistakes              int32
	)

	err := row.Scan(&ch.Traditional, &ch.Simplified, &ch.Pinyin, &ch.Zhuyin,
		&ch.Pos, &ch.Meaning, &status, &strokes, &hsk, &freq, &topics,
		&ch.Story, &ch.Mnemonic, &ch.SimplificationNote, &radicalsRaw,
		&exampRaw, &siblings, &catalogKind, &curriculumRank, &learned, &mistakes)
	if err != nil {
		return nil, err
	}

	ch.Name = "characters/" + ch.GetTraditional()
	ch.MappingStatus = mappingStatus[status]
	ch.StrokeCount = strokes
	ch.HskLevel = hsk
	ch.FrequencyRank = freq
	ch.Topics = topics
	ch.Siblings = siblings
	ch.CatalogKind = characterCatalogKind[catalogKind]
	ch.CurriculumRank = curriculumRank
	ch.Learned = learned
	ch.MistakeCount = mistakes

	radicals, err := decodeRadicalParts(radicalsRaw)
	if err != nil {
		return nil, err
	}
	ch.RadicalParts = radicals

	var examples []struct {
		HskLevel int32  `json:"hskLevel"`
		Chinese  string `json:"chinese"`
		English  string `json:"english"`
	}
	if err := json.Unmarshal(exampRaw, &examples); err != nil {
		return nil, fmt.Errorf("decode examples: %w", err)
	}

	for _, e := range examples {
		ch.Examples = append(ch.Examples, &fantiv1.ExampleSentence{
			HskLevel: e.HskLevel, Chinese: e.Chinese, English: e.English,
		})
	}

	return &ch, nil
}

func decodeRadicalParts(raw []byte) ([]*fantiv1.RadicalPart, error) {
	var radicals []struct {
		Part string `json:"part"`
		Note string `json:"note"`
	}
	if err := json.Unmarshal(raw, &radicals); err != nil {
		return nil, fmt.Errorf("decode radical parts: %w", err)
	}

	parts := make([]*fantiv1.RadicalPart, 0, len(radicals))
	for _, r := range radicals {
		parts = append(parts, &fantiv1.RadicalPart{Part: r.Part, Note: r.Note})
	}

	return parts, nil
}

func (d *Dictionary) fillSeededRadicalParts(
	ctx context.Context,
	ch *fantiv1.Character,
	fallbackCharacter string,
) error {
	if len(ch.GetRadicalParts()) > 0 {
		return nil
	}

	decompositionChar := ch.GetTraditional()
	if decompositionChar == "" {
		decompositionChar = fallbackCharacter
	}

	var radicalsRaw []byte
	err := d.pool.QueryRow(ctx,
		"SELECT radical_parts FROM stroke_data WHERE ch = $1", decompositionChar).Scan(&radicalsRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	ch.RadicalParts, err = decodeRadicalParts(radicalsRaw)

	return err
}

// GetStrokeData returns the raw hanzi-writer JSON (outlines + medians)
// for one character, or NOT_FOUND when no stroke data exists — rows
// seeded before the outlines backfill count as absent.
func (d *Dictionary) GetStrokeData(
	ctx context.Context, req *connect.Request[fantiv1.GetStrokeDataRequest],
) (*connect.Response[fantiv1.GetStrokeDataResponse], error) {
	ch, err := parseName("characters", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	var data string
	if err := d.pool.QueryRow(ctx, `
		SELECT data FROM stroke_data
		WHERE ch = $1 AND data IS NOT NULL`, ch).Scan(&data); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound,
				fmt.Errorf("no stroke data for %q", ch)) //nolint:err113 // request detail
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fantiv1.GetStrokeDataResponse{Data: data}), nil
}

// GetCharacter returns one character with its learning state.
func (d *Dictionary) GetCharacter(
	ctx context.Context, req *connect.Request[fantiv1.GetCharacterRequest],
) (*connect.Response[fantiv1.Character], error) {
	id, err := parseName("characters", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	row := d.pool.QueryRow(ctx,
		"SELECT"+characterColumns+characterFrom+"WHERE c.traditional = $1", id)

	ch, err := scanCharacter(row)
	if errors.Is(err, pgx.ErrNoRows) {
		var (
			definitions []string
			learned     bool
			mistakes    int32
		)

		ch = &fantiv1.Character{Name: "characters/" + id}
		err = d.pool.QueryRow(ctx, `
			SELECT d.traditional, d.simplified, d.pinyin, d.definitions,
				COALESCE(r.learned, FALSE), COALESCE(r.mistake_count, 0)
			FROM dict_entries d
			LEFT JOIN reviews r ON r.ch = $1
			WHERE (char_length(d.traditional) = 1 AND d.traditional = $1)
				OR (char_length(d.simplified) = 1 AND d.simplified = $1)
			ORDER BY d.traditional = $1 DESC, d.id
			LIMIT 1`, id).Scan(&ch.Traditional, &ch.Simplified, &ch.Pinyin,
			&definitions, &learned, &mistakes)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound,
				fmt.Errorf("character %q not found", id)) //nolint:err113 // request detail
		}

		if err == nil {
			ch.Meaning = strings.Join(definitions, "; ")
			ch.Learned = learned
			ch.MistakeCount = mistakes
		}
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := d.fillSeededRadicalParts(ctx, ch, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := d.fillCharacterSourceMetadata(ctx, []*fantiv1.Character{ch}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(ch), nil
}

// GetCharacterHistory returns the fixed visual timeline for one character.
// Missing Commons forms remain explicit unavailable stages; regular script is
// rendered locally from the modern character.
func (d *Dictionary) GetCharacterHistory(
	ctx context.Context,
	req *connect.Request[fantiv1.GetCharacterHistoryRequest],
) (*connect.Response[fantiv1.CharacterHistory], error) {
	ch, err := parseCharacterHistoryName(req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	var exists bool
	if err := d.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM characters WHERE traditional = $1
			UNION ALL
			SELECT 1 FROM dict_entries
			WHERE char_length(traditional) = 1
				AND (traditional = $1 OR simplified = $1)
		)`, ch).Scan(&exists); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !exists {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("character %q not found", ch)) //nolint:err113 // request detail
	}

	history := &fantiv1.CharacterHistory{Name: req.Msg.GetName()}
	formsByName := make(map[string]*fantiv1.CharacterForm, len(characterFormStages))

	for _, item := range characterFormStages {
		form := &fantiv1.CharacterForm{
			Stage:     item.stage,
			Available: item.name == regularCharacterForm,
		}
		history.Forms = append(history.Forms, form)
		formsByName[item.name] = form
	}

	rows, err := d.pool.Query(ctx, `
		SELECT stage, svg, source_title, source_url, license
		FROM character_history
		WHERE ch = $1
		ORDER BY stage_order`, ch)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			stage, sourceTitle, sourceURL, license string
			svg                                    []byte
		)
		if err := rows.Scan(&stage, &svg, &sourceTitle, &sourceURL, &license); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		form, ok := formsByName[stage]
		if !ok {
			return nil, connect.NewError(connect.CodeInternal,
				fmt.Errorf("unknown character history stage %q", stage)) //nolint:err113 // stored data
		}
		form.Available = stage == regularCharacterForm || len(svg) > 0
		form.Svg = svg
		form.SourceTitle = sourceTitle
		form.SourceUrl = sourceURL
		form.License = license
	}
	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(history), nil
}

// ListCharacters lists characters with search, filtering, and ordering.
func (d *Dictionary) ListCharacters(
	ctx context.Context, req *connect.Request[fantiv1.ListCharactersRequest],
) (*connect.Response[fantiv1.ListCharactersResponse], error) {
	where, args, err := buildCharacterFilter(req.Msg.GetFilter(), req.Msg.GetQuery())
	if err != nil {
		return nil, err
	}

	order := "c.catalog_kind = 'reference', c.curriculum_rank = 0, c.curriculum_rank, c.traditional"

	switch req.Msg.GetOrderBy() {
	case "":
	case "frequency_rank":
		order = "c.frequency_rank = 0, c.frequency_rank, c.traditional"
	case "hsk_level":
		order = "c.hsk_level = 0, c.hsk_level, c.traditional"
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("unsupported order_by %q", req.Msg.GetOrderBy())) //nolint:err113 // request detail
	}

	size := req.Msg.GetPageSize()
	if size <= 0 || size > 200 {
		size = defaultPageSize
	}

	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}

	var total int32
	if err := d.pool.QueryRow(ctx,
		"SELECT count(*)"+characterFrom+where, args...).Scan(&total); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sql := "SELECT" + characterColumns + characterFrom + where +
		" ORDER BY " + order +
		" LIMIT " + strconv.Itoa(int(size)+1) + " OFFSET " + strconv.Itoa(offset)

	rows, err := d.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.ListCharactersResponse{TotalSize: total}

	for rows.Next() {
		ch, err := scanCharacter(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp.Characters = append(resp.Characters, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if len(resp.GetCharacters()) > int(size) {
		resp.Characters = resp.GetCharacters()[:size]
		resp.NextPageToken = encodePageToken(offset + int(size))
	}

	if err := d.fillCharacterSourceMetadata(ctx, resp.GetCharacters()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

func parseCharacterHistoryName(name string) (string, error) {
	const (
		prefix = "characters/"
		suffix = "/history"
	)

	ch, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return "", connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must be characters/{character}/history, got %q", name)) //nolint:err113
	}
	ch, ok = strings.CutSuffix(ch, suffix)
	if !ok || ch == "" || strings.Contains(ch, "/") {
		return "", connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must be characters/{character}/history, got %q", name)) //nolint:err113
	}

	return ch, nil
}

// buildCharacterFilter supports the filter forms the UI issues, AND-combined,
// plus free-text query over every related glyph, reading, and definition.
func buildCharacterFilter(filter, query string) (string, []any, error) {
	clauses := []string{"TRUE"}

	var args []any

	arg := func(v any) string {
		args = append(args, v)

		return "$" + strconv.Itoa(len(args))
	}

	for part := range strings.SplitSeq(filter, " AND ") {
		part = strings.TrimSpace(part)

		switch {
		case part == "":
		case strings.HasPrefix(part, "topic = "):
			topic, ok := quotedFilterValue(part, "topic = ")
			if !ok || topic == "" {
				return "", nil, invalidCharacterFilter(part)
			}
			clauses = append(clauses, arg(topic)+" = ANY(c.topics)")
		case part == "learned = true":
			clauses = append(clauses, "COALESCE(r.learned, FALSE)")
		case part == "learned = false":
			clauses = append(clauses, "NOT COALESCE(r.learned, FALSE)")
		case strings.HasPrefix(part, "hsk_level <= "):
			n, err := strconv.Atoi(strings.TrimPrefix(part, "hsk_level <= "))
			if err != nil || n <= 0 {
				return "", nil, invalidCharacterFilter(part)
			}
			clauses = append(clauses, "c.hsk_level > 0 AND c.hsk_level <= "+arg(n))
		case strings.HasPrefix(part, "catalog_kind = "):
			kind, ok := quotedFilterValue(part, "catalog_kind = ")
			if !ok || (kind != "curriculum" && kind != "reference") {
				return "", nil, invalidCharacterFilter(part)
			}
			clauses = append(clauses, "c.catalog_kind = "+arg(kind))
		case strings.HasPrefix(part, "missing_capability = "):
			capability, ok := quotedFilterValue(part, "missing_capability = ")
			if !ok {
				return "", nil, invalidCharacterFilter(part)
			}

			switch capability {
			case qtReading:
				clauses = append(clauses, "c.pinyin = ''")
			case qtMeaning:
				clauses = append(clauses, "c.meaning = ''")
			case "strokes":
				clauses = append(clauses, `NOT EXISTS (
					SELECT 1 FROM stroke_data AS missing_strokes
					WHERE missing_strokes.ch = c.traditional
						AND missing_strokes.data IS NOT NULL
				)`)
			case "components":
				clauses = append(clauses, `NOT EXISTS (
					SELECT 1 FROM stroke_data AS missing_components
					WHERE missing_components.ch = c.traditional
						AND jsonb_array_length(missing_components.radical_parts) > 0
				)`)
			case "history":
				clauses = append(clauses, `NOT EXISTS (
					SELECT 1 FROM character_history AS missing_history
					WHERE missing_history.ch = c.traditional
						AND missing_history.stage <> 'regular'
						AND missing_history.svg IS NOT NULL
				)`)
			default:
				return "", nil, invalidCharacterFilter(part)
			}
		default:
			return "", nil, invalidCharacterFilter(part)
		}
	}

	if query != "" {
		q := arg("%" + query + "%")
		exact := arg(query)
		clauses = append(clauses,
			"(c.traditional = "+exact+" OR c.simplified = "+exact+
				" OR c.pinyin ILIKE "+q+" OR c.meaning ILIKE "+q+
				` OR EXISTS (
					SELECT 1 FROM dict_entries AS searched_sense
					WHERE searched_sense.traditional = c.traditional
						AND char_length(searched_sense.traditional) = 1
						AND char_length(searched_sense.simplified) = 1
						AND (
							searched_sense.simplified = `+exact+`
							OR searched_sense.pinyin ILIKE `+q+`
							OR array_to_string(searched_sense.definitions, ' ') ILIKE `+q+`
						)
				))`)
	}

	return " WHERE " + strings.Join(clauses, " AND "), args, nil
}

func quotedFilterValue(part, prefix string) (string, bool) {
	value := strings.TrimPrefix(part, prefix)
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return "", false
	}

	return value[1 : len(value)-1], true
}

func invalidCharacterFilter(part string) error {
	return connect.NewError(connect.CodeInvalidArgument,
		fmt.Errorf("unsupported character filter %q", part)) //nolint:err113 // request detail
}

// GetEntry returns one CC-CEDICT entry by numeric id.
func (d *Dictionary) GetEntry(
	ctx context.Context, req *connect.Request[fantiv1.GetEntryRequest],
) (*connect.Response[fantiv1.Entry], error) {
	id, err := parseName("entries", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	numeric, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("entry id must be numeric: %q", id)) //nolint:err113 // request detail
	}

	e := &fantiv1.Entry{Name: req.Msg.GetName()}

	err = d.pool.QueryRow(ctx,
		"SELECT traditional, simplified, pinyin, definitions FROM dict_entries WHERE id = $1",
		numeric).Scan(&e.Traditional, &e.Simplified, &e.Pinyin, &e.Definitions)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("entry %q not found", id)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(e), nil
}

// SearchEntries free-text searches CC-CEDICT.
func (d *Dictionary) SearchEntries(
	ctx context.Context, req *connect.Request[fantiv1.SearchEntriesRequest],
) (*connect.Response[fantiv1.SearchEntriesResponse], error) {
	query := req.Msg.GetQuery()
	if query == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errEmptyQuery)
	}

	size := req.Msg.GetPageSize()
	if size <= 0 || size > 200 {
		size = defaultPageSize
	}

	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}

	rows, err := d.pool.Query(ctx, `
		SELECT id, traditional, simplified, pinyin, definitions
		FROM dict_entries
		WHERE traditional = $1 OR simplified = $1
			OR pinyin ILIKE $2
			OR array_to_string(definitions, ' ') ILIKE $2
		ORDER BY char_length(traditional), id
		LIMIT $3 OFFSET $4`,
		query, "%"+query+"%", size+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.SearchEntriesResponse{}

	for rows.Next() {
		var (
			id int64
			e  fantiv1.Entry
		)

		if err := rows.Scan(&id, &e.Traditional, &e.Simplified, &e.Pinyin, &e.Definitions); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		e.Name = "entries/" + strconv.FormatInt(id, 10)
		resp.Entries = append(resp.Entries, &e)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if len(resp.GetEntries()) > int(size) {
		resp.Entries = resp.GetEntries()[:size]
		resp.NextPageToken = encodePageToken(offset + int(size))
	}

	return connect.NewResponse(resp), nil
}

// ListCompounds lists compounds with unlock state from learned characters.
func (d *Dictionary) ListCompounds(
	ctx context.Context, req *connect.Request[fantiv1.ListCompoundsRequest],
) (*connect.Response[fantiv1.ListCompoundsResponse], error) {
	rows, err := d.pool.Query(ctx, `
		SELECT word, pinyin, chars, gloss,
			(SELECT bool_and(COALESCE(r.learned, FALSE))
			 FROM unnest(chars) AS u(ch)
			 LEFT JOIN reviews r ON r.ch = u.ch) AS unlocked
		FROM compounds
		ORDER BY word`)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.ListCompoundsResponse{}

	for rows.Next() {
		var (
			c        fantiv1.Compound
			chars    []string
			unlocked *bool
		)

		if err := rows.Scan(&c.Word, &c.Pinyin, &chars, &c.Gloss, &unlocked); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, ch := range chars {
			c.Characters = append(c.Characters, "characters/"+ch)
		}

		c.Unlocked = unlocked != nil && *unlocked

		if req.Msg.GetUnlockedOnly() && !c.GetUnlocked() {
			continue
		}

		if c.GetUnlocked() {
			resp.UnlockedCount++
		}

		resp.TotalSize++
		resp.Compounds = append(resp.Compounds, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(resp), nil
}

var errEmptyQuery = errors.New("query must not be empty")
