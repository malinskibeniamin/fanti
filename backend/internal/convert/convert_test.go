package convert

import (
	"strings"
	"testing"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()

	e, err := NewEngine()
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	return e
}

// The design's own diff sample: 简 → 繁 with Taiwan-standard forms.
func TestConvertTextS2T(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	got, err := e.ConvertText("他后来发现，面馆里的师傅头发花白，说话却很干净利落。", Options{Direction: S2T})
	if err != nil {
		t.Fatalf("ConvertText() error = %v", err)
	}

	want := "他後來發現，麵館裡的師傅頭髮花白，說話卻很乾淨俐落。"
	// Taiwan standard must give 麵館裡, not 麪館裏.
	for _, must := range []string{"麵館裡", "頭髮", "乾淨"} {
		if !strings.Contains(got, must) {
			t.Errorf("ConvertText() = %q, must contain %q (design target %q)", got, must, want)
		}
	}
}

func TestConvertTextT2S(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	got, err := e.ConvertText("他對書法的熱愛始終如一，乾淨的紙上寫著幾行小字。", Options{Direction: T2S})
	if err != nil {
		t.Fatalf("ConvertText() error = %v", err)
	}

	for _, must := range []string{"干净", "写着", "热爱"} {
		if !strings.Contains(got, must) {
			t.Errorf("ConvertText() = %q, must contain %q", got, must)
		}
	}
}

func TestLocalizeVocabulary(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	plain, err := e.ConvertText("软件和信息", Options{Direction: S2T})
	if err != nil {
		t.Fatalf("plain: %v", err)
	}

	if strings.Contains(plain, "軟體") {
		t.Errorf("localization off must not produce 軟體: %q", plain)
	}

	localized, err := e.ConvertText("软件和信息", Options{Direction: S2T, Localize: true})
	if err != nil {
		t.Fatalf("localized: %v", err)
	}

	if !strings.Contains(localized, "軟體") || !strings.Contains(localized, "資訊") {
		t.Errorf("localization on must produce 軟體/資訊: %q", localized)
	}
}

func TestConvertPunctuation(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	got, err := e.ConvertText("他说：“好久不见！”", Options{Direction: S2T, Punctuation: true})
	if err != nil {
		t.Fatalf("ConvertText() error = %v", err)
	}

	if !strings.Contains(got, "「好久不見！」") {
		t.Errorf("punctuation conversion failed: %q", got)
	}

	back, err := e.ConvertText("他說：「好久不見！」", Options{Direction: T2S, Punctuation: true})
	if err != nil {
		t.Fatalf("back: %v", err)
	}

	if !strings.Contains(back, "“好久不见！”") {
		t.Errorf("reverse punctuation failed: %q", back)
	}
}

func TestConvertChaptersReport(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	chapters := []Chapter{
		{Title: "第一章", Paragraphs: []string{
			"他后来发现，面馆里的师傅头发花白。",
			"一根白发落在碗里。",
		}},
		{Title: "第二章", Paragraphs: []string{"心里想着三里路。"}},
	}

	var progressCalls int

	converted, report, err := e.ConvertChapters(chapters, Options{Direction: S2T},
		func(_, _ int) { progressCalls++ })
	if err != nil {
		t.Fatalf("ConvertChapters() error = %v", err)
	}

	if len(converted) != 2 || converted[0].Title != "第一章" {
		t.Fatalf("chapters = %+v", converted)
	}

	if !strings.Contains(converted[0].Paragraphs[0], "頭髮") {
		t.Errorf("paragraph not converted: %q", converted[0].Paragraphs[0])
	}

	if progressCalls == 0 {
		t.Error("progress callback never called")
	}

	if report.Total == 0 || report.Exact == 0 {
		t.Errorf("report counts empty: %+v", report)
	}

	// 发 (3×) and 里 (4×) are curated ambiguous characters.
	var found int

	for _, ex := range report.Exceptions {
		switch ex.SourceChar {
		case "发":
			found++

			if ex.Occurrences != 3 {
				t.Errorf("发 occurrences = %d, want 3", ex.Occurrences)
			}

			if ex.Context == "" {
				t.Error("发 context empty")
			}
		case "里":
			found++

			if ex.Occurrences != 4 {
				t.Errorf("里 occurrences = %d, want 4", ex.Occurrences)
			}
		}
	}

	if found != 2 {
		t.Errorf("expected 发 and 里 exceptions, got %+v", report.Exceptions)
	}

	if report.Ambiguous < 7 {
		t.Errorf("ambiguous = %d, want >= 7 (3×发 + 4×里)", report.Ambiguous)
	}

	if len(report.Diff.Tokens) == 0 || report.Diff.SourceText == "" {
		t.Error("diff preview empty")
	}
}

func TestResolveExceptionOverride(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	chapters := []Chapter{{Title: "", Paragraphs: []string{"一碗面。"}}}

	// Override 面 → 面 (keep the surface form instead of 麵).
	converted, _, err := e.ConvertChapters(chapters,
		Options{Direction: S2T, Resolutions: map[string]string{"面": "面"}}, nil)
	if err != nil {
		t.Fatalf("ConvertChapters() error = %v", err)
	}

	if !strings.Contains(converted[0].Paragraphs[0], "一碗面") {
		t.Errorf("resolution override ignored: %q", converted[0].Paragraphs[0])
	}
}

func TestDetectScript(t *testing.T) {
	t.Parallel()

	e := newTestEngine(t)

	simp, err := e.DetectScript("话说天下大势，分久必合，合久必分。")
	if err != nil || simp != S2T {
		t.Errorf("simplified sample → %v (%v), want S2T", simp, err)
	}

	trad, err := e.DetectScript("話說天下大勢，分久必合，合久必分。")
	if err != nil || trad != T2S {
		t.Errorf("traditional sample → %v (%v), want T2S", trad, err)
	}
}
