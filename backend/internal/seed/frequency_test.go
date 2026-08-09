package seed

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// Shared fixture readings, hoisted so goconst stays quiet across the
// frequency test files.
const (
	pinyinMian = "miàn"
	pinyinMao  = "māo"
	glossDry   = "dry"
	pinyinGan  = "gān"
	glossHair  = "hair"
)

// sampleFrequencyCorpus mirrors the vendored Tatoeba derivative shape:
// `id\tMandarin\tEnglish`. English text must never affect the ranks.
const sampleFrequencyCorpus = `1	的的是	是 should not count
2	的一	One
3	了	Done
`

func TestParseFrequencyList(t *testing.T) {
	entries, err := ParseFrequencyList(strings.NewReader(sampleFrequencyCorpus))
	if err != nil {
		t.Fatalf("ParseFrequencyList() error = %v", err)
	}

	want := []FrequencyEntry{
		{Rank: 1, Char: "的"},
		{Rank: 2, Char: "一"},
		{Rank: 3, Char: "了"},
		{Rank: 4, Char: "是"},
	}

	if !slices.Equal(entries, want) {
		t.Errorf("entries = %v, want %v", entries, want)
	}
}

func TestParseFrequencyListEmpty(t *testing.T) {
	corpusWithoutHan := "1\tTry it.\tTry it.\n"

	if _, err := ParseFrequencyList(strings.NewReader(corpusWithoutHan)); !errors.Is(err, errEmptyFrequencyList) {
		t.Fatalf("error = %v, want errEmptyFrequencyList", err)
	}
}

func TestBuildFrequencyPlan(t *testing.T) {
	entries := []FrequencyEntry{
		{Rank: 1, Char: "的"},    // new, CEDICT 1:1, trad == simp
		{Rank: 2, Char: "发"},    // matches the curated 髮 row by simplified form
		{Rank: 3, Char: "马"},    // curated 馬 row, rank already correct
		{Rank: 4, Char: "面"},    // new, ambiguous: 面 and 麵 share it
		{Rank: 5, Char: "喵"},    // new, absent from CEDICT
		{Rank: 9009, Char: "猫"}, // full long-tail rank is still inserted
		{Rank: 9008, Char: "茶"}, // existing long-tail row gets the rank
	}

	existing := []existingChar{
		{traditional: "髮", simplified: "发", rank: 1500},
		{traditional: "馬", simplified: "马", rank: 3},
		{traditional: "茶", simplified: "茶", rank: 1000},
	}

	lookups := freqLookups{
		dict: map[string][]dictChar{
			"的": {{traditional: "的", pinyin: "de", definitions: []string{"of", "possessive particle", "really"}}},
			"面": {
				{traditional: "面", pinyin: pinyinMian, definitions: []string{"face", "side"}},
				{traditional: "麵", pinyin: pinyinMian, definitions: []string{"noodles", "flour"}},
			},
			"猫": {{traditional: "貓", pinyin: pinyinMao, definitions: []string{"cat"}}},
		},
		// 的 has a CEDICT reading, so this entry must NOT override it;
		// 喵 has no CEDICT entry and draws its reading from here.
		pinyin:  map[string]string{"的": "di4", "喵": "miāo"},
		strokes: map[string]int{"面": 9},
	}

	inserts, updates := buildFrequencyPlan(entries, existing, lookups)

	wantInserts := []freqInsert{
		{
			traditional: "的", simplified: "的", pinyin: "de",
			meaning: "of; possessive particle", mappingStatus: mappingExact,
			rank: 1, siblings: []string{},
		},
		{
			traditional: "面", simplified: "面", pinyin: pinyinMian,
			meaning: "face; side", mappingStatus: mappingAmbiguous,
			strokeCount: 9, rank: 4, siblings: []string{"麵"},
		},
		{
			traditional: "喵", simplified: "喵", pinyin: "miāo",
			meaning: "", mappingStatus: mappingExact,
			rank: 5, siblings: []string{},
		},
		{
			traditional: "貓", simplified: "猫", pinyin: pinyinMao,
			meaning: "cat", mappingStatus: mappingExact,
			rank: 9009, siblings: []string{},
		},
	}

	if len(inserts) != len(wantInserts) {
		t.Fatalf("inserts = %+v, want %+v", inserts, wantInserts)
	}

	for i, want := range wantInserts {
		got := inserts[i]
		if got.traditional != want.traditional || got.simplified != want.simplified ||
			got.pinyin != want.pinyin || got.meaning != want.meaning ||
			got.mappingStatus != want.mappingStatus || got.strokeCount != want.strokeCount ||
			got.rank != want.rank || !slices.Equal(got.siblings, want.siblings) {
			t.Errorf("insert %d = %+v, want %+v", i, got, want)
		}
	}

	// 髮 gets the real rank; long-tail 茶 still gets its rank;
	// 馬 already carries rank 3 so no update is planned for it.
	wantUpdates := []freqUpdate{
		{traditional: "髮", rank: 2},
		{traditional: "茶", rank: 9008},
	}

	if !slices.Equal(updates, wantUpdates) {
		t.Errorf("updates = %+v, want %+v", updates, wantUpdates)
	}
}

func TestBuildFrequencyPlanSkipsVariantHeadwords(t *testing.T) {
	// CC-CEDICT sorts rare variant glyphs like 乹 and surname senses first;
	// the headword must be the first entry that is neither.
	entries := []FrequencyEntry{{Rank: 1, Char: "干"}}

	lookups := freqLookups{
		dict: map[string][]dictChar{
			"干": {
				{traditional: "乹", pinyin: "qián", definitions: []string{"old variant of 乾|干[gan1]"}},
				{traditional: "乾", pinyin: "Qián", definitions: []string{"surname Gan"}},
				{traditional: "乾", pinyin: pinyinGan, definitions: []string{glossDry, "clean"}},
				{traditional: "干", pinyin: pinyinGan, definitions: []string{"to concern", "shield"}},
			},
		},
	}

	inserts, _ := buildFrequencyPlan(entries, nil, lookups)

	if len(inserts) != 1 {
		t.Fatalf("inserts = %+v, want one row", inserts)
	}

	got := inserts[0]
	if got.traditional != "乾" || got.meaning != "dry; clean" || got.mappingStatus != mappingAmbiguous {
		t.Errorf("insert = %+v, want 乾 headword with dry; clean gloss", got)
	}

	if !slices.Equal(got.siblings, []string{"乹", "干"}) {
		t.Errorf("siblings = %v, want [乹 干]", got.siblings)
	}
}

func TestBuildFrequencyPlanDuplicateResolution(t *testing.T) {
	// 干 resolves to 乾 via CEDICT; a later-ranked 乾 entry must not
	// clobber the rank the simplified form already claimed.
	entries := []FrequencyEntry{
		{Rank: 1, Char: "干"},
		{Rank: 2, Char: "乾"},
	}

	lookups := freqLookups{
		dict: map[string][]dictChar{
			"干": {{traditional: "乾", pinyin: pinyinGan, definitions: []string{glossDry}}},
			"乾": {{traditional: "乾", pinyin: pinyinGan, definitions: []string{glossDry}}},
		},
	}

	inserts, updates := buildFrequencyPlan(entries, nil, lookups)

	if len(inserts) != 1 || inserts[0].traditional != "乾" || inserts[0].rank != 1 {
		t.Errorf("inserts = %+v, want a single 乾 row at rank 1", inserts)
	}

	if len(updates) != 0 {
		t.Errorf("updates = %+v, want none", updates)
	}
}

func TestBuildFrequencyPlanRanksEveryAmbiguousCatalogEntry(t *testing.T) {
	t.Parallel()

	entries := []FrequencyEntry{{Rank: 42, Char: "发"}}
	existing := []existingChar{
		{traditional: "發", simplified: "發"},
		{traditional: "髮", simplified: "髮"},
	}
	lookups := freqLookups{
		dict: map[string][]dictChar{
			"发": {
				{traditional: "發", pinyin: "fā", definitions: []string{"to send out"}},
				{traditional: "髮", pinyin: "fà", definitions: []string{glossHair}},
			},
		},
	}

	inserts, updates := buildFrequencyPlan(entries, existing, lookups)

	if len(inserts) != 0 {
		t.Errorf("inserts = %+v, want existing canonical entries only", inserts)
	}
	if !slices.Equal(updates, []freqUpdate{
		{traditional: "發", rank: 42},
		{traditional: "髮", rank: 42},
	}) {
		t.Errorf("updates = %+v, want both ambiguous entries ranked", updates)
	}
}
