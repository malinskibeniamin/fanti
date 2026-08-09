package bookfile

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

type zipEntry struct {
	name string
	data string
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)

	return len(p), nil
}

func buildZipBytes(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", e.name, err)
		}
		if _, err := w.Write([]byte(e.data)); err != nil {
			t.Fatalf("write zip entry %s: %v", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func readZipEntry(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", name, err)
		}
		b, err := io.ReadAll(rc)
		if cerr := rc.Close(); cerr != nil {
			t.Fatalf("close entry %s: %v", name, cerr)
		}
		if err != nil {
			t.Fatalf("read entry %s: %v", name, err)
		}
		return string(b)
	}
	t.Fatalf("zip entry %s not found", name)
	return ""
}

// Shared CJK fixture strings, hoisted so repeated table entries stay
// constant-folded.
const (
	paraRain = "山中有雨。"
	paraBoat = "江上有船。"
	titleCh1 = "第一章 起點"
	langZhTW = "zh-TW"
)

func TestParseEPUB(t *testing.T) {
	opf := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="uid">urn:uuid:test</dc:identifier>
<dc:title>測試書</dc:title>
<dc:creator>作者甲</dc:creator>
<dc:language>zh-TW</dc:language>
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="coverpage" href="cover.xhtml" media-type="application/xhtml+xml"/>
<item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/>
<item id="ch2" href="ch2.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="coverpage"/><itemref idref="ch1"/><itemref idref="ch2"/></spine>
</package>`
	ch1 := `<?xml version="1.0" encoding="utf-8"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>ignored</title><style>p{margin:0}</style></head><body><h2>第一章 起點</h2><p>山中有雨。</p><p>甲<br/>乙</p></body></html>`
	ch2 := `<html><body><h3>後記</h3><p>他說&amp;完了。</p></body></html>`
	data := buildZipBytes(t, []zipEntry{
		{mimetypeEntryName, epubMimetype},
		{containerXMLPath, epubContainerXML},
		{contentOPFPath, opf},
		{navXHTMLPath, `<html><body><nav epub:type="toc"><ol><li>x</li></ol></nav></body></html>`},
		{"OEBPS/cover.xhtml", `<html><body><p>封面</p></body></html>`},
		{"OEBPS/ch1.xhtml", ch1},
		{"OEBPS/ch2.xhtml", ch2},
	})
	got, err := Parse("book.epub", data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Format != FormatEPUB {
		t.Errorf("Format = %q, want %q", got.Format, FormatEPUB)
	}
	if got.Title != "測試書" {
		t.Errorf("Title = %q, want 測試書", got.Title)
	}
	if got.Author != "作者甲" {
		t.Errorf("Author = %q, want 作者甲", got.Author)
	}
	want := []Chapter{
		{Title: titleCh1, Paragraphs: []string{paraRain, "甲", "乙"}},
		{Title: "後記", Paragraphs: []string{"他說&完了。"}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestParseEPUBSingleSpineDocFallsBackToRegex(t *testing.T) {
	opf := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:title>單檔書</dc:title>
<dc:creator>作者乙</dc:creator>
</metadata>
<manifest>
<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
<item id="all" href="all.xhtml" media-type="application/xhtml+xml"/>
</manifest>
<spine><itemref idref="all"/></spine>
</package>`
	all := `<html><body><p>第一章 一</p><p>內容一。</p><p>第二章 二</p><p>內容二。</p></body></html>`
	data := buildZipBytes(t, []zipEntry{
		{mimetypeEntryName, epubMimetype},
		{containerXMLPath, epubContainerXML},
		{contentOPFPath, opf},
		{navXHTMLPath, `<html><body><nav epub:type="toc"><ol><li>x</li></ol></nav></body></html>`},
		{"OEBPS/all.xhtml", all},
	})
	got, err := Parse("book.epub", data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := []Chapter{
		{Title: "第一章 一", Paragraphs: []string{"內容一。"}},
		{Title: "第二章 二", Paragraphs: []string{"內容二。"}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestParseEPUBDetectedByMagicWithoutExtension(t *testing.T) {
	opf := `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>魔數書</dc:title></metadata><manifest><item id="c" href="c.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c"/></spine></package>`
	data := buildZipBytes(t, []zipEntry{
		{mimetypeEntryName, epubMimetype},
		{containerXMLPath, epubContainerXML},
		{contentOPFPath, opf},
		{"OEBPS/c.xhtml", `<html><body><p>正文在此。</p></body></html>`},
	})
	got, err := Parse("upload-without-extension", data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Format != FormatEPUB {
		t.Errorf("Format = %q, want %q", got.Format, FormatEPUB)
	}
	if got.Title != "魔數書" {
		t.Errorf("Title = %q, want 魔數書", got.Title)
	}
}

func TestReadZipFileRejectsOversizedEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("large.xhtml")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := io.CopyN(w, zeroReader{}, maxZipEntryBytes+1); err != nil {
		t.Fatalf("write oversized entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}

	_, err = readZipFile(map[string]*zip.File{"large.xhtml": zr.File[0]}, "large.xhtml")
	if err == nil {
		t.Fatal("readZipFile accepted an oversized entry")
	}
}

func TestParseEPUBRejectsOversizedExpandedArchive(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := range 5 {
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:   "part-" + string(rune('a'+i)) + ".xhtml",
			Method: zip.Deflate,
		})
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := io.CopyN(w, zeroReader{}, maxZipEntryBytes); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	_, err := Parse("large.epub", buf.Bytes())
	if !errors.Is(err, errArchiveTooLarge) {
		t.Fatalf("Parse() error = %v, want errArchiveTooLarge", err)
	}
}
