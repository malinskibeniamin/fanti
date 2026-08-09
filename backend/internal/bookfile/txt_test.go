package bookfile

import (
	"errors"
	"reflect"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

func TestParseTXTChapterSplitting(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Chapter
	}{
		{
			name:  "three numbered chapters",
			input: "第一章 初遇\n" + paraRain + "\n雨中有人。\n\n第二章 再見\n" + paraBoat + "\n\n第三章 離別\n人已遠去。\n",
			want: []Chapter{
				{Title: "第一章 初遇", Paragraphs: []string{paraRain, "雨中有人。"}},
				{Title: "第二章 再見", Paragraphs: []string{paraBoat}},
				{Title: "第三章 離別", Paragraphs: []string{"人已遠去。"}},
			},
		},
		{
			name:  "preamble before first marker becomes untitled chapter",
			input: "作品概要在此。\n\n第一章 開始\n正文。\n",
			want: []Chapter{
				{Title: "", Paragraphs: []string{"作品概要在此。"}},
				{Title: "第一章 開始", Paragraphs: []string{"正文。"}},
			},
		},
		{
			name:  "prologue style markers",
			input: "序\n開篇的話。\n楔子\n很久以前。\n第十二回 大戰\n打了三天。\n",
			want: []Chapter{
				{Title: "序", Paragraphs: []string{"開篇的話。"}},
				{Title: "楔子", Paragraphs: []string{"很久以前。"}},
				{Title: "第十二回 大戰", Paragraphs: []string{"打了三天。"}},
			},
		},
		{
			name:  "arabic digit marker",
			input: "第3章 早晨\n吃早飯。\n",
			want: []Chapter{
				{Title: "第3章 早晨", Paragraphs: []string{"吃早飯。"}},
			},
		},
		{
			name:  "indented full-width marker keeps inner spacing in title",
			input: "　　第一章　起點\n　　正文第一段。\n",
			want: []Chapter{
				{Title: "第一章　起點", Paragraphs: []string{"正文第一段。"}},
			},
		},
		{
			name:  "marker text mid line does not split",
			input: "第一章 好書\n他說第一章很好。\n",
			want: []Chapter{
				{Title: "第一章 好書", Paragraphs: []string{"他說第一章很好。"}},
			},
		},
		{
			name:  "no markers yields one untitled chapter",
			input: "只是普通文字。\n第二行在此。\n",
			want: []Chapter{
				{Title: "", Paragraphs: []string{"只是普通文字。", "第二行在此。"}},
			},
		},
		{
			name:  "crlf input",
			input: "第一章 一\r\n內容。\r\n",
			want: []Chapter{
				{Title: "第一章 一", Paragraphs: []string{"內容。"}},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse("book.txt", []byte(tc.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got.Format != FormatTXT {
				t.Errorf("Format = %q, want %q", got.Format, FormatTXT)
			}
			if !reflect.DeepEqual(got.Chapters, tc.want) {
				t.Errorf("Chapters = %#v, want %#v", got.Chapters, tc.want)
			}
		})
	}
}

func TestParseTXTGB18030(t *testing.T) {
	text := "第一章 你好\n这是简体中文的内容，我们都说好。\n"
	raw, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if utf8.Valid(raw) {
		t.Fatal("fixture must not be valid UTF-8")
	}
	got, err := Parse("book.txt", raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := []Chapter{
		{Title: "第一章 你好", Paragraphs: []string{"这是简体中文的内容，我们都说好。"}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestParseTXTBig5(t *testing.T) {
	text := "第一章 妳好\n這是繁體中文的內容，我們都說好。\n"
	raw, err := traditionalchinese.Big5.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	if utf8.Valid(raw) {
		t.Fatal("fixture must not be valid UTF-8")
	}
	got, err := Parse("book.txt", raw)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := []Chapter{
		{Title: "第一章 妳好", Paragraphs: []string{"這是繁體中文的內容，我們都說好。"}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestParseCharCountCJK(t *testing.T) {
	got, err := Parse("book.txt", []byte("第一章 甲\n你好，世界！\nabc\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	// Paragraphs are 你好，世界！ (6 runes) and abc (3 runes); titles excluded.
	if got.CharCount != 9 {
		t.Errorf("CharCount = %d, want 9", got.CharCount)
	}
}

func TestParseUnsupportedFormat(t *testing.T) {
	_, err := Parse("book.xyz", []byte("hello there"))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("Parse() error = %v, want ErrUnsupportedFormat", err)
	}
}
