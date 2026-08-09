package bookfile

import (
	"reflect"
	"testing"
)

func TestParseSRT(t *testing.T) {
	input := "\uFEFF1\r\n" +
		"00:00:01,000 --> 00:00:03,000\r\n" +
		"你好。\r\n" +
		"\r\n" +
		"2\r\n" +
		"00:00:04,000 --> 00:00:06,000\r\n" +
		"第一行\r\n" +
		"第二行\r\n" +
		"\r\n" +
		"3\r\n" +
		"00:00:07,000 --> 00:00:09,000\r\n" +
		"再見。\r\n"
	got, err := Parse("subs.srt", []byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Format != FormatSRT {
		t.Errorf("Format = %q, want %q", got.Format, FormatSRT)
	}
	want := []Chapter{
		{Title: "", Paragraphs: []string{"你好。", "第一行 第二行", "再見。"}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}
