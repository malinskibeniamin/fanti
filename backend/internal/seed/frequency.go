package seed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// frequencyMeaningDefs is how many CEDICT definitions feed `meaning`.
	frequencyMeaningDefs = 2
)

// Character mapping statuses stored in characters.mapping_status.
const (
	mappingExact     = "exact"
	mappingAmbiguous = "ambiguous"
)

var errEmptyFrequencyList = errors.New("no rows parsed from frequency list")

// FrequencyEntry is one rank→character pair derived from the Tatoeba corpus.
type FrequencyEntry struct {
	Rank int
	Char string
}

// ParseFrequencyList counts Han characters in the Mandarin field of the
// vendored Tatoeba derivative. More frequent characters rank first; Unicode
// code-point order breaks count ties so repeated seeds stay deterministic.
func ParseFrequencyList(r io.Reader) ([]FrequencyEntry, error) {
	pairs, err := ParseTatoebaPairs(r)
	if err != nil {
		if errors.Is(err, errEmptyTatoebaPairs) {
			return nil, errEmptyFrequencyList
		}

		return nil, fmt.Errorf("read frequency corpus: %w", err)
	}

	counts := map[rune]int{}
	for _, pair := range pairs {
		for _, ch := range pair.Mandarin {
			if unicode.Is(unicode.Han, ch) {
				counts[ch]++
			}
		}
	}

	if len(counts) == 0 {
		return nil, errEmptyFrequencyList
	}

	type countedCharacter struct {
		char  rune
		count int
	}

	characters := make([]countedCharacter, 0, len(counts))
	for char, count := range counts {
		characters = append(characters, countedCharacter{char: char, count: count})
	}

	sort.Slice(characters, func(i, j int) bool {
		if characters[i].count != characters[j].count {
			return characters[i].count > characters[j].count
		}

		return characters[i].char < characters[j].char
	})

	entries := make([]FrequencyEntry, len(characters))
	for i, character := range characters {
		entries[i] = FrequencyEntry{Rank: i + 1, Char: string(character.char)}
	}

	return entries, nil
}

type frequencyDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// existingChar is a characters row snapshot used for rank assignment.
type existingChar struct {
	traditional string
	simplified  string
	rank        int
}

// dictChar is a single-character dict_entries row, in id order.
type dictChar struct {
	traditional string
	pinyin      string
	definitions []string
}

// freqLookups bundles the reference data new character rows draw from.
type freqLookups struct {
	// dict maps a simplified form to its single-character CEDICT entries.
	dict map[string][]dictChar
	// pinyin is the char_pinyin fallback table.
	pinyin map[string]string
	// strokes maps a character to its stroke_data median count.
	strokes map[string]int
}

// freqInsert is one planned new characters row.
type freqInsert struct {
	traditional   string
	simplified    string
	pinyin        string
	meaning       string
	mappingStatus string
	strokeCount   int
	rank          int
	siblings      []string
}

// freqUpdate is one planned rank-only update to an existing row.
type freqUpdate struct {
	traditional string
	rank        int
}

// buildFrequencyPlan assigns list ranks to character rows. Existing rows
// (matched by simplified or traditional form) only ever receive a rank —
// authored content stays untouched. Characters without a row yet become
// inserts, resolved against CEDICT. Rank order wins ties:
// once a row claims a rank, later entries cannot reassign it.
func buildFrequencyPlan(
	entries []FrequencyEntry, existing []existingChar, lk freqLookups,
) ([]freqInsert, []freqUpdate) {
	sorted := slices.Clone(entries)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Rank < sorted[j].Rank })

	rankByTrad := make(map[string]int, len(existing))
	bySimplified := make(map[string][]string, len(existing))

	for _, c := range existing {
		rankByTrad[c.traditional] = c.rank
		bySimplified[c.simplified] = append(bySimplified[c.simplified], c.traditional)
	}

	assigned := make(map[string]bool, len(sorted))

	var (
		inserts []freqInsert
		updates []freqUpdate
	)

	for _, e := range sorted {
		targets := existingTargets(e.Char, bySimplified, rankByTrad)
		for _, candidate := range lk.dict[e.Char] {
			if _, exists := rankByTrad[candidate.traditional]; exists &&
				!slices.Contains(targets, candidate.traditional) {
				targets = append(targets, candidate.traditional)
			}
		}

		if len(targets) > 0 {
			for _, trad := range targets {
				if assigned[trad] {
					continue
				}

				assigned[trad] = true

				if rankByTrad[trad] != e.Rank {
					updates = append(updates, freqUpdate{traditional: trad, rank: e.Rank})
				}
			}

			continue
		}

		ins := resolveInsert(e, lk)
		if assigned[ins.traditional] {
			continue
		}

		assigned[ins.traditional] = true
		inserts = append(inserts, ins)
	}

	return inserts, updates
}

// existingTargets lists the traditional keys of rows covering ch, matching
// the simplified column first and the primary key itself second.
func existingTargets(ch string, bySimplified map[string][]string, rankByTrad map[string]int) []string {
	targets := slices.Clone(bySimplified[ch])

	if _, ok := rankByTrad[ch]; ok && !slices.Contains(targets, ch) {
		targets = append(targets, ch)
	}

	return targets
}

// resolveInsert builds a new characters row for a ranked simplified form:
// the lowest-id single-character CEDICT entry supplies the traditional
// form, reading, and gloss; the character itself is the fallback.
func resolveInsert(e FrequencyEntry, lk freqLookups) freqInsert {
	ins := freqInsert{
		traditional:   e.Char,
		simplified:    e.Char,
		mappingStatus: mappingExact,
		rank:          e.Rank,
		siblings:      []string{},
	}

	if candidates := lk.dict[e.Char]; len(candidates) > 0 {
		chosen := pickDictCandidate(candidates)
		ins.traditional = chosen.traditional
		ins.pinyin = chosen.pinyin
		ins.meaning = joinDefinitions(chosen.definitions)

		if forms := distinctTraditionalForms(candidates); len(forms) > 1 {
			ins.mappingStatus = mappingAmbiguous
			ins.siblings = otherForms(forms, chosen.traditional)
		}
	}

	// The chosen entry's reading matches the gloss on the card; char_pinyin
	// only fills the gap when CEDICT had nothing.
	if ins.pinyin == "" {
		if py, ok := lk.pinyin[ins.traditional]; ok {
			ins.pinyin = py
		} else if py, ok := lk.pinyin[e.Char]; ok {
			ins.pinyin = py
		}
	}

	if n, ok := lk.strokes[ins.traditional]; ok {
		ins.strokeCount = n
	} else if n, ok := lk.strokes[e.Char]; ok {
		ins.strokeCount = n
	}

	return ins
}

// pickDictCandidate chooses the headword entry: the lowest-id one whose
// gloss is not a mere "variant of …" cross-reference or a surname entry.
// CC-CEDICT sorts rare variant glyphs (乹 for 干, 裏 for 里) and surname
// senses ("surname Hou" for 后) ahead of the standard forms, and those
// make poor headwords. Falls back to the first entry.
func pickDictCandidate(candidates []dictChar) dictChar {
	for _, c := range candidates {
		if len(c.definitions) == 0 {
			continue
		}

		lead := c.definitions[0]
		if isCrossReferenceOrSurname(lead) {
			continue
		}

		return c
	}

	return candidates[0]
}

// joinDefinitions glosses a character from its leading CEDICT definitions.
func joinDefinitions(defs []string) string {
	if len(defs) > frequencyMeaningDefs {
		defs = defs[:frequencyMeaningDefs]
	}

	return strings.Join(defs, "; ")
}

// distinctTraditionalForms keeps the entries' traditional forms in first-seen order.
func distinctTraditionalForms(candidates []dictChar) []string {
	forms := make([]string, 0, len(candidates))

	for _, c := range candidates {
		if !slices.Contains(forms, c.traditional) {
			forms = append(forms, c.traditional)
		}
	}

	return forms
}

// otherForms lists the sibling traditional forms, excluding the chosen one.
func otherForms(forms []string, chosen string) []string {
	siblings := make([]string, 0, len(forms))

	for _, f := range forms {
		if f != chosen {
			siblings = append(siblings, f)
		}
	}

	return siblings
}

// SeedFrequency loads every distinct character frequency rank. Existing
// rows — curated ones included — only ever get their frequency_rank set.
//
//nolint:revive // the design names this step SeedFrequency; seed.Run/seed.SeedFrequency read as a pair
func SeedFrequency(ctx context.Context, pool *pgxpool.Pool, r io.Reader, logger *slog.Logger) error {
	entries, err := ParseFrequencyList(r)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin frequency seed: %w", err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	// A source switch or corpus update can remove characters as well as add
	// them. Reset inside the transaction so stale ranks disappear atomically.
	if _, err := tx.Exec(ctx, "UPDATE characters SET frequency_rank = 0"); err != nil {
		return fmt.Errorf("reset frequency ranks: %w", err)
	}

	existing, err := loadExistingCharacters(ctx, tx)
	if err != nil {
		return err
	}

	lookups, err := loadFrequencyLookups(ctx, tx)
	if err != nil {
		return err
	}

	inserts, updates := buildFrequencyPlan(entries, existing, lookups)

	for _, ins := range inserts {
		catalogKind := catalogKindCurriculum
		if ins.meaning == "" {
			catalogKind = catalogKindReference
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO characters (
				traditional, simplified, pinyin, meaning, mapping_status,
				stroke_count, frequency_rank, siblings, catalog_kind
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (traditional) DO UPDATE SET frequency_rank = EXCLUDED.frequency_rank`,
			ins.traditional, ins.simplified, ins.pinyin, ins.meaning, ins.mappingStatus,
			ins.strokeCount, ins.rank, ins.siblings, catalogKind); err != nil {
			return fmt.Errorf("insert %s: %w", ins.traditional, err)
		}
	}

	for _, u := range updates {
		if _, err := tx.Exec(ctx,
			"UPDATE characters SET frequency_rank = $2 WHERE traditional = $1",
			u.traditional, u.rank); err != nil {
			return fmt.Errorf("update rank %s: %w", u.traditional, err)
		}
	}

	// Existing sentence rows survive reseeds. Regrade them against the new
	// ranks before committing so their difficulty never mixes two sources.
	if _, err := tx.Exec(ctx, `
		UPDATE sentences AS s
		SET max_freq_rank = CASE
			WHEN EXISTS (
				SELECT 1
				FROM unnest(s.chars) AS sentence_char(ch)
				LEFT JOIN characters AS c ON c.traditional = sentence_char.ch
				WHERE c.frequency_rank IS NULL OR c.frequency_rank = 0
			) THEN 0
			ELSE COALESCE((
				SELECT max(c.frequency_rank)
				FROM unnest(s.chars) AS sentence_char(ch)
				JOIN characters AS c ON c.traditional = sentence_char.ch
			), 0)
		END`); err != nil {
		return fmt.Errorf("regrade sentences: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit frequency seed: %w", err)
	}

	logger.InfoContext(ctx, "seeded frequency ranks",
		slog.Int("entries", len(entries)),
		slog.Int("inserted", len(inserts)),
		slog.Int("updated", len(updates)))

	return nil
}

// loadExistingCharacters snapshots the characters rows for rank assignment.
func loadExistingCharacters(ctx context.Context, db frequencyDB) ([]existingChar, error) {
	rows, err := db.Query(ctx, "SELECT traditional, simplified, frequency_rank FROM characters")
	if err != nil {
		return nil, fmt.Errorf("query characters: %w", err)
	}

	defer rows.Close()

	var existing []existingChar

	for rows.Next() {
		var c existingChar
		if err := rows.Scan(&c.traditional, &c.simplified, &c.rank); err != nil {
			return nil, fmt.Errorf("scan character: %w", err)
		}

		existing = append(existing, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("characters rows: %w", err)
	}

	return existing, nil
}

// loadFrequencyLookups gathers the CEDICT, pinyin, and stroke reference data.
func loadFrequencyLookups(ctx context.Context, db frequencyDB) (freqLookups, error) {
	lk := freqLookups{
		dict:    map[string][]dictChar{},
		pinyin:  map[string]string{},
		strokes: map[string]int{},
	}

	rows, err := db.Query(ctx, `
		SELECT simplified, traditional, pinyin, definitions FROM dict_entries
		WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
		ORDER BY id`)
	if err != nil {
		return lk, fmt.Errorf("query dict singles: %w", err)
	}

	defer rows.Close()

	for rows.Next() {
		var (
			simplified string
			d          dictChar
		)

		if err := rows.Scan(&simplified, &d.traditional, &d.pinyin, &d.definitions); err != nil {
			return lk, fmt.Errorf("scan dict single: %w", err)
		}

		lk.dict[simplified] = append(lk.dict[simplified], d)
	}

	if err := rows.Err(); err != nil {
		return lk, fmt.Errorf("dict singles rows: %w", err)
	}

	if err := scanStringMap(ctx, db,
		"SELECT ch, pinyin FROM char_pinyin", lk.pinyin); err != nil {
		return lk, err
	}

	if err := scanStringMap(ctx, db,
		"SELECT ch, stroke_count FROM stroke_data", lk.strokes); err != nil {
		return lk, err
	}

	return lk, nil
}

// scanStringMap fills dest from a two-column (TEXT key, value) query.
func scanStringMap[V any](ctx context.Context, db frequencyDB, sql string, dest map[string]V) error {
	rows, err := db.Query(ctx, sql)
	if err != nil {
		return fmt.Errorf("query %q: %w", sql, err)
	}

	defer rows.Close()

	for rows.Next() {
		var (
			key   string
			value V
		)

		if err := rows.Scan(&key, &value); err != nil {
			return fmt.Errorf("scan %q: %w", sql, err)
		}

		dest[key] = value
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows %q: %w", sql, err)
	}

	return nil
}
