package bookfile

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// EPUBMeta describes the book-level metadata for WriteEPUB.
type EPUBMeta struct {
	Title    string
	Author   string
	Language string // BCP 47 tag, e.g. "zh-TW"
	// FrontMatter, when non-empty, becomes a page before chapter 1.
	FrontMatter string
	// IndentFirstLine adds text-indent:2em to the paragraph CSS.
	IndentFirstLine bool
}

const (
	epubMimetype      = "application/epub+zip"
	mimetypeEntryName = "mimetype"
	containerXMLPath  = "META-INF/container.xml"
	contentOPFPath    = "OEBPS/content.opf"
	navXHTMLPath      = "OEBPS/nav.xhtml"
	epubContainerXML  = `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`
	// epubModified is a fixed dcterms:modified timestamp so identical input
	// produces identical bytes.
	epubModified   = "2026-07-06T00:00:00Z"
	frontMatterDoc = "front.xhtml"
	frontMatterID  = "front"
)

// WriteEPUB builds an EPUB3 file from the given metadata and chapters. The
// mimetype entry is written first and uncompressed, as the EPUB OCF spec
// requires.
func WriteEPUB(meta EPUBMeta, chapters []Chapter) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	mimetypeWriter, err := zw.CreateHeader(&zip.FileHeader{Name: mimetypeEntryName, Method: zip.Store})
	if err != nil {
		return nil, fmt.Errorf("epub: create mimetype entry: %w", err)
	}
	if _, err := mimetypeWriter.Write([]byte(epubMimetype)); err != nil {
		return nil, fmt.Errorf("epub: write mimetype entry: %w", err)
	}
	entries := []struct{ name, data string }{
		{containerXMLPath, epubContainerXML},
		{contentOPFPath, buildOPF(meta, chapters)},
		{navXHTMLPath, buildNav(meta, chapters)},
	}
	if meta.FrontMatter != "" {
		entries = append(entries, struct{ name, data string }{"OEBPS/" + frontMatterDoc, buildFrontPage(meta)})
	}
	for i, ch := range chapters {
		entries = append(entries, struct{ name, data string }{"OEBPS/" + chapterDoc(i), buildChapterPage(meta, ch)})
	}
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			return nil, fmt.Errorf("epub: create entry %q: %w", e.name, err)
		}
		if _, err := w.Write([]byte(e.data)); err != nil {
			return nil, fmt.Errorf("epub: write entry %q: %w", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("epub: finalize zip: %w", err)
	}
	return buf.Bytes(), nil
}

func chapterDoc(i int) string {
	return "ch" + strconv.Itoa(i+1) + ".xhtml"
}

func chapterID(i int) string {
	return "ch" + strconv.Itoa(i+1)
}

func buildOPF(meta EPUBMeta, chapters []Chapter) string {
	var manifest, spine strings.Builder
	manifest.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`)
	if meta.FrontMatter != "" {
		manifest.WriteString(`<item id="` + frontMatterID + `" href="` + frontMatterDoc + `" media-type="application/xhtml+xml"/>`)
		spine.WriteString(`<itemref idref="` + frontMatterID + `"/>`)
	}
	for i := range chapters {
		manifest.WriteString(`<item id="` + chapterID(i) + `" href="` + chapterDoc(i) + `" media-type="application/xhtml+xml"/>`)
		spine.WriteString(`<itemref idref="` + chapterID(i) + `"/>`)
	}
	return `<?xml version="1.0" encoding="utf-8"?>` +
		`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">` +
		`<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">` +
		`<dc:identifier id="uid">urn:uuid:fa9t1-` + url.QueryEscape(meta.Title) + `</dc:identifier>` +
		`<dc:title>` + xmlEscape(meta.Title) + `</dc:title>` +
		`<dc:creator>` + xmlEscape(meta.Author) + `</dc:creator>` +
		`<dc:language>` + xmlEscape(meta.Language) + `</dc:language>` +
		`<meta property="dcterms:modified">` + epubModified + `</meta>` +
		`</metadata>` +
		`<manifest>` + manifest.String() + `</manifest>` +
		`<spine>` + spine.String() + `</spine>` +
		`</package>`
}

func buildNav(meta EPUBMeta, chapters []Chapter) string {
	var items strings.Builder
	if meta.FrontMatter != "" {
		items.WriteString(`<li><a href="` + frontMatterDoc + `">扉頁</a></li>`)
	}
	for i, ch := range chapters {
		label := ch.Title
		if label == "" {
			label = strconv.Itoa(i + 1)
		}
		items.WriteString(`<li><a href="` + chapterDoc(i) + `">` + xmlEscape(label) + `</a></li>`)
	}
	return `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
		`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + xmlEscape(meta.Language) + `">` +
		`<head><title>目錄</title></head>` +
		`<body><nav epub:type="toc"><h1>目錄</h1><ol>` + items.String() + `</ol></nav></body></html>`
}

func buildFrontPage(meta EPUBMeta) string {
	var paras strings.Builder
	for line := range strings.SplitSeq(normalizeNewlines(meta.FrontMatter), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paras.WriteString("<p>" + xmlEscape(line) + "</p>")
		}
	}
	return xhtmlPage(meta, "扉頁", paras.String())
}

func buildChapterPage(meta EPUBMeta, ch Chapter) string {
	var body strings.Builder
	body.WriteString("<h2>" + xmlEscape(ch.Title) + "</h2>")
	for _, p := range ch.Paragraphs {
		body.WriteString("<p>" + xmlEscape(p) + "</p>")
	}
	return xhtmlPage(meta, ch.Title, body.String())
}

func xhtmlPage(meta EPUBMeta, title, body string) string {
	indent := ""
	if meta.IndentFirstLine {
		indent = "text-indent:2em;"
	}
	css := "body{font-family:serif;line-height:2;margin:8% 10%}" +
		"h2{text-align:center;font-weight:normal;margin-bottom:2em}" +
		"p{" + indent + "margin:0 0 .6em}"
	return `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
		`<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="` + xmlEscape(meta.Language) + `">` +
		`<head><title>` + xmlEscape(title) + `</title><style>` + css + `</style></head>` +
		`<body>` + body + `</body></html>`
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
