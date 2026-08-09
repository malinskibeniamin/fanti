package seed

import (
	"slices"
	"strings"
	"testing"
)

// Shared across the tatoeba and fill tests: the simplified source
// sentence and its expected traditional conversion.
const (
	sleepSimplified  = "我该去睡觉了。"
	sleepTraditional = "我該去睡覺了。"
	sleepEnglish     = "I have to go to sleep."
)

func TestParseTatoebaPairs(t *testing.T) {
	// A quote-leading English field and a malformed row must not derail
	// the surrounding rows (Tatoeba TSV has no quote escaping).
	input := "1\t我們試試看！\tLet's try it.\n" +
		"2\t\"好\"是什麼意思？\t\"OK,\" she said.\n" +
		"broken line without tabs\n" +
		"3\t你好。\tHello.\n"

	pairs, err := ParseTatoebaPairs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTatoebaPairs() error = %v", err)
	}

	if len(pairs) != 3 {
		t.Fatalf("ParseTatoebaPairs() len = %d, want 3", len(pairs))
	}

	if pairs[1].ID != 2 || pairs[1].Mandarin != `"好"是什麼意思？` || pairs[1].English != `"OK," she said.` {
		t.Errorf("quote-leading pair = %+v", pairs[1])
	}
}

func TestParseTatoebaPairsEmpty(t *testing.T) {
	if _, err := ParseTatoebaPairs(strings.NewReader("")); err == nil {
		t.Error("ParseTatoebaPairs(empty) expected error, got nil")
	}
}

func TestParseTatoebaPairsDuplicateID(t *testing.T) {
	// A duplicated id would abort the whole CopyFrom on the primary key;
	// the first occurrence wins.
	input := "1\t你好。\tHello.\n" +
		"1\t再見。\tGoodbye.\n"

	pairs, err := ParseTatoebaPairs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseTatoebaPairs() error = %v", err)
	}

	if len(pairs) != 1 || pairs[0].Mandarin != "你好。" {
		t.Errorf("pairs = %+v, want single first occurrence", pairs)
	}
}

func TestHanChars(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"mixed script and digits", "今天是６月１８号，也是Muiriel的生日！", []string{"今", "天", "是", "月", "号", "也", "的", "生", "日"}},
		{"repeated characters collapse", "我們試試看！", []string{"我", "們", "試", "看"}},
		{"no han characters", "OK 123!", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hanChars(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("hanChars(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSentenceMaxFreqRank(t *testing.T) {
	ranks := map[string]int{"我": 7, "們": 12, "看": 30}

	// Every character ranked → the rarest one's rank.
	if got := sentenceMaxFreqRank([]string{"我", "們", "看"}, ranks); got != 30 {
		t.Errorf("all ranked = %d, want 30", got)
	}

	// Any unranked character marks the sentence unranked (0), so ranking
	// never mistakes a rare-character sentence for an easy one.
	if got := sentenceMaxFreqRank([]string{"我", "齉"}, ranks); got != 0 {
		t.Errorf("unranked char = %d, want 0", got)
	}
}
