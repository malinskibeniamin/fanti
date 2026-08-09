package bookfile

import (
	"archive/zip"
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func testWriteChapters() []Chapter {
	return []Chapter{
		{Title: titleCh1, Paragraphs: []string{paraRain, "雨中有人。"}},
		{Title: "第二章 轉折", Paragraphs: []string{paraBoat}},
	}
}

func TestWriteEPUBRoundTrip(t *testing.T) {
	meta := EPUBMeta{Title: "我的書", Author: "作者乙", Language: langZhTW, IndentFirstLine: true}
	chapters := testWriteChapters()
	out, err := WriteEPUB(meta, chapters)
	if err != nil {
		t.Fatalf("WriteEPUB() error = %v", err)
	}
	got, err := Parse("book.epub", out)
	if err != nil {
		t.Fatalf("Parse(WriteEPUB()) error = %v", err)
	}
	if got.Format != FormatEPUB {
		t.Errorf("Format = %q, want %q", got.Format, FormatEPUB)
	}
	if got.Title != meta.Title {
		t.Errorf("Title = %q, want %q", got.Title, meta.Title)
	}
	if got.Author != meta.Author {
		t.Errorf("Author = %q, want %q", got.Author, meta.Author)
	}
	if !reflect.DeepEqual(got.Chapters, chapters) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, chapters)
	}
}

func TestWriteEPUBMimetypeFirstAndStored(t *testing.T) {
	out, err := WriteEPUB(EPUBMeta{Title: "書", Language: langZhTW}, testWriteChapters())
	if err != nil {
		t.Fatalf("WriteEPUB() error = %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	if len(zr.File) == 0 {
		t.Fatal("zip has no entries")
	}
	first := zr.File[0]
	if first.Name != mimetypeEntryName {
		t.Errorf("first entry = %q, want mimetype", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("mimetype method = %d, want Store (%d)", first.Method, zip.Store)
	}
	// EPUB readers sniff the mimetype at a fixed byte offset: local header is
	// 30 bytes, then the 8-byte name, then the stored payload.
	if string(out[30:38]) != mimetypeEntryName || string(out[38:58]) != epubMimetype {
		t.Errorf("mimetype payload not at expected raw offset: %q", out[30:58])
	}
}

func TestWriteEPUBFrontMatter(t *testing.T) {
	meta := EPUBMeta{Title: "書", Author: "人", Language: langZhTW, FrontMatter: "本書由 Fanti 產生。"}
	chapters := testWriteChapters()
	out, err := WriteEPUB(meta, chapters)
	if err != nil {
		t.Fatalf("WriteEPUB() error = %v", err)
	}
	opf := readZipEntry(t, out, "OEBPS/content.opf")
	frontIdx := strings.Index(opf, `<itemref idref="front"/>`)
	ch1Idx := strings.Index(opf, `<itemref idref="ch1"/>`)
	if frontIdx < 0 || ch1Idx < 0 || frontIdx > ch1Idx {
		t.Errorf("spine must list front before ch1, got OPF: %s", opf)
	}
	nav := readZipEntry(t, out, "OEBPS/nav.xhtml")
	if !strings.Contains(nav, "扉頁") || !strings.Contains(nav, "front.xhtml") {
		t.Errorf("nav must link front matter as 扉頁, got: %s", nav)
	}
	got, err := Parse("book.epub", out)
	if err != nil {
		t.Fatalf("Parse(WriteEPUB()) error = %v", err)
	}
	want := append([]Chapter{{Title: "", Paragraphs: []string{"本書由 Fanti 產生。"}}}, chapters...)
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestWriteEPUBIndentFirstLine(t *testing.T) {
	tests := []struct {
		name   string
		indent bool
	}{
		{name: "indent on", indent: true},
		{name: "indent off", indent: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := WriteEPUB(EPUBMeta{Title: "書", Language: langZhTW, IndentFirstLine: tc.indent}, testWriteChapters())
			if err != nil {
				t.Fatalf("WriteEPUB() error = %v", err)
			}
			ch1 := readZipEntry(t, out, "OEBPS/ch1.xhtml")
			if got := strings.Contains(ch1, "text-indent:2em"); got != tc.indent {
				t.Errorf("ch1.xhtml contains text-indent:2em = %v, want %v", got, tc.indent)
			}
		})
	}
}
