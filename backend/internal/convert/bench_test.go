package convert

import (
	"strings"
	"testing"
)

// BenchmarkConvertBook simulates a 紅樓夢-scale conversion (~600k chars)
// to prove the async job completes in sane wall-clock time.
func BenchmarkConvertBook(b *testing.B) {
	e, err := NewEngine()
	if err != nil {
		b.Fatal(err)
	}

	para := strings.Repeat("话说天下大势，分久必合，合久必分。他后来发现，面馆里的师傅头发花白，说话却很干净利落。", 10) // ~420 chars
	chapters := make([]Chapter, 120)

	for i := range chapters {
		chapters[i] = Chapter{Title: "第一回", Paragraphs: make([]string, 12)}
		for j := range chapters[i].Paragraphs {
			chapters[i].Paragraphs[j] = para
		}
	}
	// 120 chapters × 12 paragraphs × ~420 chars ≈ 605k chars

	b.ResetTimer()

	for range b.N {
		if _, _, err := e.ConvertChapters(chapters, Options{Direction: S2T}, nil); err != nil {
			b.Fatal(err)
		}
	}
}
