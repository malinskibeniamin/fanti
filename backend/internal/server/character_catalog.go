package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

const (
	characterCoverageName = "characterCoverage"
	coreCurriculumSize    = 3000
)

var errCharacterCoverageName = errors.New("name must be characterCoverage")

func capabilityStatus(available bool) fantiv1.CapabilityStatus {
	if available {
		return fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE
	}

	return fantiv1.CapabilityStatus_CAPABILITY_STATUS_UNAVAILABLE
}

func entryCapabilities(ch *fantiv1.Character) *fantiv1.CharacterCapabilities {
	return &fantiv1.CharacterCapabilities{
		Reading: capabilityStatus(ch.GetPinyin() != ""),
		Meaning: capabilityStatus(ch.GetMeaning() != ""),
	}
}

// fillCharacterSourceMetadata attaches all CEDICT senses, related glyphs,
// and source-aware capability states without N+1 queries.
func (d *Dictionary) fillCharacterSourceMetadata(
	ctx context.Context,
	characters []*fantiv1.Character,
) error {
	if len(characters) == 0 {
		return nil
	}

	byTraditional := make(map[string]*fantiv1.Character, len(characters))
	keys := make([]string, 0, len(characters))
	glyphIndexes := make(map[string]map[string]int, len(characters))

	for _, ch := range characters {
		traditional := ch.GetTraditional()
		byTraditional[traditional] = ch
		keys = append(keys, traditional)
		ch.EntryCapabilities = entryCapabilities(ch)

		glyphIndexes[traditional] = map[string]int{}
		scripts := []fantiv1.Script(nil)
		if ch.GetCatalogKind() !=
			fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_REFERENCE {
			scripts = []fantiv1.Script{fantiv1.Script_SCRIPT_TRADITIONAL}
		}
		addCharacterGlyph(ch, glyphIndexes[traditional], traditional, scripts, true)
	}

	rows, err := d.pool.Query(ctx, `
		SELECT traditional, simplified, pinyin, definitions
		FROM dict_entries
		WHERE traditional = ANY($1)
			AND char_length(traditional) = 1
			AND char_length(simplified) = 1
		ORDER BY id`, keys)
	if err != nil {
		return fmt.Errorf("query character senses: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			traditional string
			sense       fantiv1.CharacterSense
		)
		if err := rows.Scan(
			&traditional,
			&sense.Simplified,
			&sense.Pinyin,
			&sense.Definitions,
		); err != nil {
			return fmt.Errorf("scan character sense: %w", err)
		}

		ch := byTraditional[traditional]
		if ch == nil {
			continue
		}

		ch.Senses = append(ch.Senses, &sense)
		if ch.GetCatalogKind() ==
			fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_UNSPECIFIED {
			ch.CatalogKind = fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_CURRICULUM
		}
		addCharacterGlyph(
			ch,
			glyphIndexes[traditional],
			sense.GetSimplified(),
			[]fantiv1.Script{fantiv1.Script_SCRIPT_SIMPLIFIED},
			false,
		)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("character sense rows: %w", err)
	}

	for _, ch := range characters {
		if ch.GetCatalogKind() ==
			fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_REFERENCE {
			continue
		}

		addCharacterGlyph(
			ch,
			glyphIndexes[ch.GetTraditional()],
			ch.GetSimplified(),
			[]fantiv1.Script{fantiv1.Script_SCRIPT_SIMPLIFIED},
			false,
		)
	}

	glyphs := make([]string, 0, len(characters)*2)
	seenGlyphs := make(map[string]bool)
	for _, ch := range characters {
		for _, form := range ch.GetGlyphs() {
			if !seenGlyphs[form.GetGlyph()] {
				seenGlyphs[form.GetGlyph()] = true
				glyphs = append(glyphs, form.GetGlyph())
			}
		}
	}

	capabilities, err := d.loadGlyphCapabilities(ctx, glyphs)
	if err != nil {
		return err
	}

	for _, ch := range characters {
		for _, form := range ch.GetGlyphs() {
			form.Capabilities = capabilities[form.GetGlyph()]
		}
	}

	return nil
}

func addCharacterGlyph(
	ch *fantiv1.Character,
	indexes map[string]int,
	glyph string,
	scripts []fantiv1.Script,
	primary bool,
) {
	if glyph == "" {
		return
	}

	if index, ok := indexes[glyph]; ok {
		form := ch.GetGlyphs()[index]
		form.Primary = form.GetPrimary() || primary
		for _, script := range scripts {
			if !slices.Contains(form.GetScripts(), script) {
				form.Scripts = append(form.Scripts, script)
			}
		}

		return
	}

	indexes[glyph] = len(ch.GetGlyphs())
	ch.Glyphs = append(ch.Glyphs, &fantiv1.CharacterGlyph{
		Glyph:   glyph,
		Scripts: scripts,
		Primary: primary,
	})
}

func (d *Dictionary) loadGlyphCapabilities(
	ctx context.Context,
	glyphs []string,
) (map[string]*fantiv1.CharacterCapabilities, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT requested.glyph,
			strokes.data IS NOT NULL,
			COALESCE(strokes.radical_parts, '[]'::jsonb),
			EXISTS (
				SELECT 1 FROM character_history AS history
				WHERE history.ch = requested.glyph
					AND history.stage <> 'regular'
					AND history.svg IS NOT NULL
			)
		FROM unnest($1::text[]) AS requested(glyph)
		LEFT JOIN stroke_data AS strokes ON strokes.ch = requested.glyph`, glyphs)
	if err != nil {
		return nil, fmt.Errorf("query glyph capabilities: %w", err)
	}
	defer rows.Close()

	capabilities := make(map[string]*fantiv1.CharacterCapabilities, len(glyphs))
	for rows.Next() {
		var (
			glyph       string
			hasStrokes  bool
			radicalsRaw []byte
			hasHistory  bool
		)
		if err := rows.Scan(&glyph, &hasStrokes, &radicalsRaw, &hasHistory); err != nil {
			return nil, fmt.Errorf("scan glyph capabilities: %w", err)
		}

		components, err := componentCapability(glyph, radicalsRaw)
		if err != nil {
			return nil, err
		}

		capabilities[glyph] = &fantiv1.CharacterCapabilities{
			Strokes:    capabilityStatus(hasStrokes),
			Components: components,
			History:    capabilityStatus(hasHistory),
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("glyph capability rows: %w", err)
	}

	return capabilities, nil
}

func componentCapability(
	glyph string,
	raw []byte,
) (fantiv1.CapabilityStatus, error) {
	var parts []struct {
		Part string `json:"part"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return fantiv1.CapabilityStatus_CAPABILITY_STATUS_UNSPECIFIED,
			fmt.Errorf("decode components for %s: %w", glyph, err)
	}

	switch {
	case len(parts) == 0:
		return fantiv1.CapabilityStatus_CAPABILITY_STATUS_UNAVAILABLE, nil
	case len(parts) == 1 && parts[0].Part == glyph:
		return fantiv1.CapabilityStatus_CAPABILITY_STATUS_NOT_APPLICABLE, nil
	default:
		return fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE, nil
	}
}

// GetCharacterCoverage returns aggregate entry and script-form coverage.
func (d *Dictionary) GetCharacterCoverage(
	ctx context.Context,
	req *connect.Request[fantiv1.GetCharacterCoverageRequest],
) (*connect.Response[fantiv1.CharacterCoverage], error) {
	if req.Msg.GetName() != characterCoverageName {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%w, got %q", errCharacterCoverageName, req.Msg.GetName()))
	}

	coverage := &fantiv1.CharacterCoverage{Name: characterCoverageName}

	var total, curriculum, reference, core, reading, meaning int32
	if err := d.pool.QueryRow(ctx, `
		SELECT count(*),
			count(*) FILTER (WHERE catalog_kind = 'curriculum'),
			count(*) FILTER (WHERE catalog_kind = 'reference'),
			count(*) FILTER (
				WHERE catalog_kind = 'curriculum'
					AND curriculum_rank BETWEEN 1 AND $1
			),
			count(*) FILTER (WHERE pinyin <> ''),
			count(*) FILTER (WHERE meaning <> '')
		FROM characters`, coreCurriculumSize).Scan(
		&total,
		&curriculum,
		&reference,
		&core,
		&reading,
		&meaning,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	coverage.TotalEntries = total
	coverage.CurriculumEntries = curriculum
	coverage.ReferenceEntries = reference
	coverage.CoreEntries = core
	coverage.UnclassifiedForms = reference
	coverage.EntryCapabilities = []*fantiv1.CapabilityCoverage{
		newCapabilityCoverage(
			fantiv1.CharacterCapability_CHARACTER_CAPABILITY_READING,
			reading,
			0,
			total-reading,
		),
		newCapabilityCoverage(
			fantiv1.CharacterCapability_CHARACTER_CAPABILITY_MEANING,
			meaning,
			0,
			total-meaning,
		),
	}

	var totalGlyphs int32
	if err := d.pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT traditional AS glyph FROM characters
			UNION
			SELECT simplified AS glyph FROM characters
			UNION
			SELECT traditional AS glyph FROM dict_entries
			WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
			UNION
			SELECT simplified AS glyph FROM dict_entries
			WHERE char_length(traditional) = 1 AND char_length(simplified) = 1
		) AS covered_glyphs`).Scan(&totalGlyphs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	coverage.TotalGlyphs = totalGlyphs

	scripts, err := d.scriptCoverage(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	coverage.Scripts = scripts

	return connect.NewResponse(coverage), nil
}

func (d *Dictionary) scriptCoverage(
	ctx context.Context,
) ([]*fantiv1.ScriptCoverage, error) {
	rows, err := d.pool.Query(ctx, `
		WITH forms AS (
			SELECT traditional AS glyph, 2::INT AS script
			FROM characters
			WHERE catalog_kind = 'curriculum'
			UNION
			SELECT simplified AS glyph, 1::INT AS script
			FROM characters
			WHERE catalog_kind = 'curriculum'
			UNION
			SELECT source.simplified AS glyph, 1::INT AS script
			FROM dict_entries AS source
			JOIN characters AS character
				ON character.traditional = source.traditional
				AND character.catalog_kind = 'curriculum'
			WHERE char_length(source.traditional) = 1
				AND char_length(source.simplified) = 1
		),
		history_forms AS (
			SELECT DISTINCT ch
			FROM character_history
			WHERE stage <> 'regular' AND svg IS NOT NULL
		)
		SELECT forms.script,
			count(*),
			count(*) FILTER (WHERE strokes.data IS NOT NULL),
			count(*) FILTER (WHERE strokes.data IS NULL),
			count(*) FILTER (
				WHERE jsonb_array_length(COALESCE(strokes.radical_parts, '[]'::jsonb)) > 0
					AND NOT (
						jsonb_array_length(strokes.radical_parts) = 1
						AND strokes.radical_parts->0->>'part' = forms.glyph
					)
			),
			count(*) FILTER (
				WHERE jsonb_array_length(COALESCE(strokes.radical_parts, '[]'::jsonb)) = 1
					AND strokes.radical_parts->0->>'part' = forms.glyph
			),
			count(*) FILTER (
				WHERE jsonb_array_length(COALESCE(strokes.radical_parts, '[]'::jsonb)) = 0
			),
			count(*) FILTER (WHERE history_forms.ch IS NOT NULL),
			count(*) FILTER (WHERE history_forms.ch IS NULL)
		FROM forms
		LEFT JOIN stroke_data AS strokes ON strokes.ch = forms.glyph
		LEFT JOIN history_forms ON history_forms.ch = forms.glyph
		GROUP BY forms.script
		ORDER BY forms.script`)
	if err != nil {
		return nil, fmt.Errorf("query script coverage: %w", err)
	}
	defer rows.Close()

	var scripts []*fantiv1.ScriptCoverage
	for rows.Next() {
		var (
			scriptNumber                                    int32
			total, strokes, missingStrokes                  int32
			components, atomicComponents, missingComponents int32
			history, missingHistory                         int32
		)
		if err := rows.Scan(
			&scriptNumber,
			&total,
			&strokes,
			&missingStrokes,
			&components,
			&atomicComponents,
			&missingComponents,
			&history,
			&missingHistory,
		); err != nil {
			return nil, fmt.Errorf("scan script coverage: %w", err)
		}

		scripts = append(scripts, &fantiv1.ScriptCoverage{
			Script:     fantiv1.Script(scriptNumber),
			TotalForms: total,
			Capabilities: []*fantiv1.CapabilityCoverage{
				newCapabilityCoverage(
					fantiv1.CharacterCapability_CHARACTER_CAPABILITY_STROKES,
					strokes,
					0,
					missingStrokes,
				),
				newCapabilityCoverage(
					fantiv1.CharacterCapability_CHARACTER_CAPABILITY_COMPONENTS,
					components,
					atomicComponents,
					missingComponents,
				),
				newCapabilityCoverage(
					fantiv1.CharacterCapability_CHARACTER_CAPABILITY_HISTORY,
					history,
					0,
					missingHistory,
				),
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("script coverage rows: %w", err)
	}

	return scripts, nil
}

func newCapabilityCoverage(
	capability fantiv1.CharacterCapability,
	available, notApplicable, unavailable int32,
) *fantiv1.CapabilityCoverage {
	return &fantiv1.CapabilityCoverage{
		Capability:    capability,
		Available:     available,
		NotApplicable: notApplicable,
		Unavailable:   unavailable,
	}
}
