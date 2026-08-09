package seed

import (
	"strings"
	"testing"
)

func TestParseUnihanReadings(t *testing.T) {
	// Real Unihan_Readings.txt shapes: comments, kMandarin (already in
	// diacritic form, space-separated alternatives), kHanyuPinyin
	// (location:readings with comma-separated lists), other keys to
	// ignore, and a kHanyuPinyin-only codepoint.
	input := "# Unihan_Readings.txt\n" +
		"U+3400\tkMandarin\tqiū\n" +
		"U+3401\tkHanyuPinyin\t10019.020:tiàn,tián\n" +
		"U+4E2D\tkMandarin\tzhōng zhòng\n" +
		"U+4E2D\tkHanyuPinyin\t10067.010:zhōng,zhòng\n" +
		"U+4E2D\tkCantonese\tzung1\n" +
		"not a data line\n"

	readings, err := ParseUnihanReadings(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseUnihanReadings() error = %v", err)
	}

	tests := map[string]string{
		"㐀": "qiū",   // kMandarin
		"㐁": "tiàn",  // kHanyuPinyin fallback, first reading
		"中": "zhōng", // kMandarin preferred over kHanyuPinyin, first value
	}

	if len(readings) != len(tests) {
		t.Errorf("readings = %d entries, want %d: %v", len(readings), len(tests), readings)
	}

	for ch, want := range tests {
		if got := readings[ch]; got != want {
			t.Errorf("readings[%s] = %q, want %q", ch, got, want)
		}
	}
}

func TestParseUnihanReadingsEmpty(t *testing.T) {
	if _, err := ParseUnihanReadings(strings.NewReader("# only comments\n")); err == nil {
		t.Error("ParseUnihanReadings(no data) expected error, got nil")
	}
}
