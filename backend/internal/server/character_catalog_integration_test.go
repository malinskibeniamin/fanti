package server_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

func TestIntegrationCharacterCatalogAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := seed.Run(ctx, pool, seed.Sources{}, logger); err != nil {
		t.Fatalf("seed authored fixtures: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO dict_entries (traditional, simplified, pinyin, definitions) VALUES
			('馬', '马', 'mǎ', ARRAY['horse']),
			('馬', '马', 'mà', ARRAY['to scold']),
			('發', '发', 'fā', ARRAY['to send out']),
			('髮', '发', 'fà', ARRAY['hair']);
		INSERT INTO char_pinyin (ch, pinyin) VALUES ('𠮷', 'jí')
		ON CONFLICT (ch) DO UPDATE SET pinyin = EXCLUDED.pinyin;
		INSERT INTO stroke_data (ch, medians, stroke_count, data, radical_parts) VALUES
			('馬', '[]', 10, '{"strokes":[],"medians":[]}', '[{"part":"馬","note":""}]'),
			('马', '[]', 3, '{"strokes":[],"medians":[]}', '[{"part":"马","note":""}]');
		INSERT INTO character_history (
			ch, stage, stage_order, svg, source_title, source_url, source_sha1, license
		) VALUES (
			'馬', 'oracle', 1, '<svg/>', '馬 oracle', 'https://example.test/horse',
			'sha', 'Public domain'
		)`); err != nil {
		t.Fatalf("insert catalog source fixtures: %v", err)
	}

	if err := seed.SeedCharacterCatalog(ctx, pool, logger); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if err := seed.RankCharacterCurriculum(ctx, pool); err != nil {
		t.Fatalf("rank catalog: %v", err)
	}

	handler, err := server.NewHandler(pool, logger)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	httpServer := httptest.NewServer(handler)
	t.Cleanup(httpServer.Close)

	client := fantiv1connect.NewDictionaryServiceClient(http.DefaultClient, httpServer.URL)

	t.Run("character metadata", func(t *testing.T) {
		resp, err := client.GetCharacter(ctx, connect.NewRequest(&fantiv1.GetCharacterRequest{
			Name: "characters/馬",
		}))
		if err != nil {
			t.Fatalf("GetCharacter: %v", err)
		}

		ch := resp.Msg
		if ch.GetCatalogKind() !=
			fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_CURRICULUM ||
			ch.GetCurriculumRank() <= 0 {
			t.Errorf("catalog kind/rank = %v/%d, want curriculum/positive",
				ch.GetCatalogKind(), ch.GetCurriculumRank())
		}
		if len(ch.GetSenses()) != 2 ||
			ch.GetSenses()[1].GetDefinitions()[0] != "to scold" {
			t.Errorf("senses = %+v, want both source senses", ch.GetSenses())
		}
		if ch.GetEntryCapabilities().GetReading() !=
			fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE ||
			ch.GetEntryCapabilities().GetMeaning() !=
				fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE {
			t.Errorf("entry capabilities = %+v, want reading and meaning", ch.GetEntryCapabilities())
		}

		traditional := findGlyph(t, ch.GetGlyphs(), "馬")
		if !slices.Equal(traditional.GetScripts(), []fantiv1.Script{
			fantiv1.Script_SCRIPT_TRADITIONAL,
		}) || traditional.GetCapabilities().GetStrokes() !=
			fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE ||
			traditional.GetCapabilities().GetComponents() !=
				fantiv1.CapabilityStatus_CAPABILITY_STATUS_NOT_APPLICABLE ||
			traditional.GetCapabilities().GetHistory() !=
				fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE {
			t.Errorf("traditional glyph = %+v", traditional)
		}

		simplified := findGlyph(t, ch.GetGlyphs(), "马")
		if !slices.Equal(simplified.GetScripts(), []fantiv1.Script{
			fantiv1.Script_SCRIPT_SIMPLIFIED,
		}) || simplified.GetCapabilities().GetHistory() !=
			fantiv1.CapabilityStatus_CAPABILITY_STATUS_UNAVAILABLE {
			t.Errorf("simplified glyph = %+v", simplified)
		}
	})

	t.Run("reference metadata", func(t *testing.T) {
		resp, err := client.GetCharacter(ctx, connect.NewRequest(&fantiv1.GetCharacterRequest{
			Name: "characters/𠮷",
		}))
		if err != nil {
			t.Fatalf("GetCharacter: %v", err)
		}

		ch := resp.Msg
		if ch.GetCatalogKind() !=
			fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_REFERENCE ||
			ch.GetCurriculumRank() != 0 ||
			ch.GetEntryCapabilities().GetReading() !=
				fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE ||
			ch.GetEntryCapabilities().GetMeaning() !=
				fantiv1.CapabilityStatus_CAPABILITY_STATUS_UNAVAILABLE {
			t.Errorf("reference character = %+v", ch)
		}
		if len(ch.GetGlyphs()) != 1 || len(ch.GetGlyphs()[0].GetScripts()) != 0 {
			t.Errorf("reference glyphs = %+v, want one unclassified glyph", ch.GetGlyphs())
		}
	})

	t.Run("global search and filters", func(t *testing.T) {
		sense, err := client.ListCharacters(ctx,
			connect.NewRequest(&fantiv1.ListCharactersRequest{Query: "to scold"}))
		if err != nil {
			t.Fatalf("search secondary sense: %v", err)
		}
		if len(sense.Msg.GetCharacters()) != 1 ||
			sense.Msg.GetCharacters()[0].GetTraditional() != "馬" {
			t.Errorf("secondary-sense search = %+v, want 馬", sense.Msg.GetCharacters())
		}

		reference, err := client.ListCharacters(ctx,
			connect.NewRequest(&fantiv1.ListCharactersRequest{
				Filter: `catalog_kind = "reference" AND missing_capability = "strokes"`,
				Query:  "𠮷",
			}))
		if err != nil {
			t.Fatalf("filter reference gaps: %v", err)
		}
		if len(reference.Msg.GetCharacters()) != 1 ||
			reference.Msg.GetCharacters()[0].GetTraditional() != "𠮷" {
			t.Errorf("reference gap filter = %+v, want 𠮷", reference.Msg.GetCharacters())
		}

		_, err = client.ListCharacters(ctx,
			connect.NewRequest(&fantiv1.ListCharactersRequest{
				Filter: `catalog_kind = "mystery"`,
			}))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("invalid filter code = %v, want InvalidArgument", connect.CodeOf(err))
		}
	})

	t.Run("coverage", func(t *testing.T) {
		resp, err := client.GetCharacterCoverage(ctx,
			connect.NewRequest(&fantiv1.GetCharacterCoverageRequest{
				Name: "characterCoverage",
			}))
		if err != nil {
			t.Fatalf("GetCharacterCoverage: %v", err)
		}

		coverage := resp.Msg
		if coverage.GetName() != "characterCoverage" ||
			coverage.GetTotalEntries() !=
				coverage.GetCurriculumEntries()+coverage.GetReferenceEntries() ||
			coverage.GetCoreEntries() != coverage.GetCurriculumEntries() ||
			coverage.GetTotalGlyphs() < coverage.GetTotalEntries() {
			t.Errorf("coverage totals = %+v", coverage)
		}
		if coverage.GetReferenceEntries() == 0 ||
			coverage.GetUnclassifiedForms() != coverage.GetReferenceEntries() {
			t.Errorf("reference coverage = %+v", coverage)
		}
		if len(coverage.GetEntryCapabilities()) != 2 ||
			len(coverage.GetScripts()) != 2 {
			t.Errorf("coverage groups = %+v", coverage)
		}
		if coverage.GetEntryCapabilities()[0].GetCapability() !=
			fantiv1.CharacterCapability_CHARACTER_CAPABILITY_READING ||
			coverage.GetEntryCapabilities()[1].GetCapability() !=
				fantiv1.CharacterCapability_CHARACTER_CAPABILITY_MEANING {
			t.Errorf("entry capability kinds = %+v", coverage.GetEntryCapabilities())
		}
	})
}

func findGlyph(
	t *testing.T,
	glyphs []*fantiv1.CharacterGlyph,
	want string,
) *fantiv1.CharacterGlyph {
	t.Helper()

	for _, glyph := range glyphs {
		if glyph.GetGlyph() == want {
			return glyph
		}
	}

	t.Fatalf("glyph %q not found in %+v", want, glyphs)

	return nil
}
