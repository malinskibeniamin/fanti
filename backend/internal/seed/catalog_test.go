package seed

import (
	"slices"
	"testing"
)

func TestBuildCharacterCatalogPlan(t *testing.T) {
	t.Parallel()

	senses := []catalogSense{
		{
			traditional: "髮",
			simplified:  "发",
			pinyin:      "fà",
			definitions: []string{glossHair},
		},
		{
			traditional: "發",
			simplified:  "发",
			pinyin:      "fā",
			definitions: []string{"to send out", "to issue"},
		},
		{
			traditional: "乾",
			simplified:  "干",
			pinyin:      "Qián",
			definitions: []string{"surname Gan"},
		},
		{
			traditional: "乾",
			simplified:  "干",
			pinyin:      "gān",
			definitions: []string{"dry", "clean"},
		},
	}

	got := buildCharacterCatalogPlan(
		senses,
		map[string]string{
			"发": "fā", // A related CEDICT form, not a standalone entry.
			"𠮷": "jí", // Unihan-only reference character.
		},
		map[string]int{"髮": 15, "发": 5, "乾": 11},
	)

	if len(got) != 4 {
		t.Fatalf("plan length = %d, want 4", len(got))
	}

	byTraditional := make(map[string]catalogUpsert, len(got))
	for _, item := range got {
		byTraditional[item.traditional] = item
	}

	qian := byTraditional["乾"]
	if qian.pinyin != "gān" || qian.meaning != "dry; clean" ||
		qian.catalogKind != catalogKindCurriculum || qian.strokeCount != 11 {
		t.Errorf("乾 = %+v, want everyday reading/gloss and curriculum metadata", qian)
	}

	faHair := byTraditional["髮"]
	if faHair.mappingStatus != mappingAmbiguous ||
		!slices.Equal(faHair.siblings, []string{"發"}) {
		t.Errorf("髮 mapping = %q siblings = %v, want ambiguous [發]",
			faHair.mappingStatus, faHair.siblings)
	}

	reference := byTraditional["𠮷"]
	if reference.catalogKind != catalogKindReference || reference.pinyin != "jí" ||
		reference.meaning != "" {
		t.Errorf("𠮷 = %+v, want reading-only reference entry", reference)
	}

	if _, duplicatedForm := byTraditional["发"]; duplicatedForm {
		t.Error("related simplified form 发 became a duplicate learning entry")
	}
}
