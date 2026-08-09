package seed

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParseDecompositionsFlattensIDSTrees(t *testing.T) {
	t.Parallel()

	rows, err := parseDecompositions(strings.NewReader(
		`{"character":"俢","decomposition":"⿰亻⿱夂彡"}` + "\n" +
			`{"character":"凶","decomposition":"⿶凵乂"}` + "\n",
	))
	if err != nil {
		t.Fatalf("parseDecompositions() error = %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	want := []string{"亻", "夂", "彡"}
	if got := partGlyphs(rows[0].Parts); !slices.Equal(got, want) {
		t.Errorf("俢 parts = %v, want %v", got, want)
	}

	want = []string{"凵", "乂"}
	if got := partGlyphs(rows[1].Parts); !slices.Equal(got, want) {
		t.Errorf("凶 parts = %v, want %v", got, want)
	}
}

func TestParseDecompositionsSkipsUnknownAndInvalidIDS(t *testing.T) {
	t.Parallel()

	rows, err := parseDecompositions(strings.NewReader(
		`{"character":"逢","decomposition":"⿱夂？"}` + "\n" +
			`{"character":"坏","decomposition":"⿰土"}` + "\n" +
			`{"character":"好","decomposition":"⿰女子木"}` + "\n" +
			`{"character":"丁","decomposition":"一"}` + "\n",
	))
	if err != nil {
		t.Fatalf("parseDecompositions() error = %v", err)
	}

	if len(rows) != 1 || rows[0].Character != "丁" {
		t.Errorf("rows = %+v, want only 丁", rows)
	}
}

func TestParseDecompositionsRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	_, err := parseDecompositions(strings.NewReader("{not-json}\n"))
	if err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("parseDecompositions() error = %v, want line-numbered decode error", err)
	}
}

func TestParseDecompositionsRejectsEmptyDataset(t *testing.T) {
	t.Parallel()

	_, err := parseDecompositions(strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "no valid decompositions") {
		t.Fatalf("parseDecompositions() error = %v, want empty-dataset error", err)
	}
}

func TestVendoredDecompositionsContainAllValidEntries(t *testing.T) {
	t.Parallel()

	f, err := os.Open("../../data/downloads/makemeahanzi-dictionary.txt")
	if err != nil {
		t.Fatalf("open vendored decompositions: %v", err)
	}
	defer func() { _ = f.Close() }()

	rows, err := parseDecompositions(f)
	if err != nil {
		t.Fatalf("parse vendored decompositions: %v", err)
	}
	if got, want := len(rows), 9125; got != want {
		t.Errorf("valid decompositions = %d, want %d", got, want)
	}
}

func partGlyphs(parts []RadicalPart) []string {
	glyphs := make([]string, len(parts))
	for i, part := range parts {
		glyphs[i] = part.Part
	}

	return glyphs
}
