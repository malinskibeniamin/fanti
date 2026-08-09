package seed

import (
	"errors"
	"strings"
	"testing"

	"github.com/malinskibeniamin/fanti/backend/internal/bookfile"
)

func TestStripGutenbergBoilerplate(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "modern markers and credit stripped",
			raw: "header licence text\n" +
				"*** START OF THE PROJECT GUTENBERG EBOOK 測試 ***\n" +
				"Produced by Someone\n" +
				"正文第一段。\n" +
				"*** END OF THE PROJECT GUTENBERG EBOOK 測試 ***\n" +
				"licence tail\n",
			want: "正文第一段。",
		},
		{
			name: "this-variant markers",
			raw: "x\n*** START OF THIS PROJECT GUTENBERG EBOOK 測試 ***\n" +
				"正文乙。\n" +
				"*** END OF THIS PROJECT GUTENBERG EBOOK 測試 ***\n",
			want: "正文乙。",
		},
		{
			name: "legacy end-of line cuts before the marker",
			raw: "x\n*** START OF THE PROJECT GUTENBERG EBOOK 測試 ***\n" +
				"正文丙。\n" +
				"End of Project Gutenberg's 測試, by 作者\n" +
				"*** END OF THE PROJECT GUTENBERG EBOOK 測試 ***\n",
			want: "正文丙。",
		},
		{
			name:    "missing start marker",
			raw:     "正文。\n*** END OF THE PROJECT GUTENBERG EBOOK 測試 ***\n",
			wantErr: true,
		},
		{
			name:    "missing end marker",
			raw:     "*** START OF THE PROJECT GUTENBERG EBOOK 測試 ***\n正文。\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := stripGutenbergBoilerplate(tt.raw)
			if tt.wantErr {
				if !errors.Is(err, errNoGutenbergMarker) {
					t.Fatalf("error = %v, want errNoGutenbergMarker", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("error = %v", err)
			}

			if strings.TrimSpace(got) != tt.want {
				t.Errorf("body = %q, want %q", strings.TrimSpace(got), tt.want)
			}
		})
	}
}

func TestNormalizeHeadingZeros(t *testing.T) {
	in := "第九十九回：諸葛亮大破魏兵\n" +
		"第一○○回：漢兵劫寨破曹真\n" +
		"第一一○回：文鴦單騎退雄兵\n" +
		"身中數○箭而不倒。\n"

	got := normalizeHeadingZeros(in)

	if !strings.Contains(got, "第一零零回：漢兵劫寨破曹真") {
		t.Errorf("一○○ heading not normalized: %q", got)
	}

	if !strings.Contains(got, "第一一零回：文鴦單騎退雄兵") {
		t.Errorf("一一○ heading not normalized: %q", got)
	}

	// Body text keeps its circles; only heading numerals are rewritten.
	if !strings.Contains(got, "身中數○箭而不倒。") {
		t.Errorf("body ○ must stay untouched: %q", got)
	}
}

func TestCleanChaptersSplitsGluedTitles(t *testing.T) {
	glued := "第二十五回　鮑文卿南京遇舊　倪廷璽安慶招親" + strings.Repeat("話說鮑文卿到城北去尋人", 8)

	chapters := cleanChapters([]bookfile.Chapter{
		{Title: "第一回　說楔子敷陳大義　借名流隱括全文", Paragraphs: []string{"正文。"}},
		{Title: glued, Paragraphs: []string{"次段。"}},
	})

	if len(chapters) != 2 {
		t.Fatalf("chapters = %d, want 2", len(chapters))
	}

	if chapters[0].Title != "第一回　說楔子敷陳大義　借名流隱括全文" {
		t.Errorf("short title changed: %q", chapters[0].Title)
	}

	if chapters[1].Title != "第二十五回" {
		t.Errorf("glued title = %q, want 第二十五回", chapters[1].Title)
	}

	if len(chapters[1].Paragraphs) != 2 || chapters[1].Paragraphs[0] != glued {
		t.Errorf("glued line must be preserved as the first paragraph, got %q", chapters[1].Paragraphs)
	}
}
