package bookfile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// maxZipEntryBytes bounds how much is read from a single archive entry, as a
// decompression-bomb guard.
const maxZipEntryBytes = 64 << 20

const maxZipTotalBytes = 256 << 20

type epubContainer struct {
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfItem struct {
	ID         string `xml:"id,attr"`
	Href       string `xml:"href,attr"`
	Properties string `xml:"properties,attr"`
}

type opfPackage struct {
	Metadata struct {
		Title   string `xml:"title"`
		Creator string `xml:"creator"`
	} `xml:"metadata"`
	Manifest struct {
		Items []opfItem `xml:"item"`
	} `xml:"manifest"`
	Spine struct {
		Itemrefs []struct {
			IDRef string `xml:"idref,attr"`
		} `xml:"itemref"`
	} `xml:"spine"`
}

func parseEPUB(data []byte) (Parsed, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Parsed{}, fmt.Errorf("epub: open zip: %w", err)
	}
	files := make(map[string]*zip.File, len(zr.File))
	var expandedBytes uint64
	for _, f := range zr.File {
		if f.UncompressedSize64 > uint64(maxZipTotalBytes)-expandedBytes {
			return Parsed{}, fmt.Errorf("epub: expanded archive exceeds %d bytes: %w",
				maxZipTotalBytes, errArchiveTooLarge)
		}
		expandedBytes += f.UncompressedSize64
		files[f.Name] = f
	}
	pkg, opfPath, err := readOPF(files)
	if err != nil {
		return Parsed{}, err
	}
	chapters, err := readSpineChapters(files, pkg, path.Dir(opfPath))
	if err != nil {
		return Parsed{}, err
	}
	return Parsed{
		Title:    strings.TrimSpace(pkg.Metadata.Title),
		Author:   strings.TrimSpace(pkg.Metadata.Creator),
		Chapters: chapters,
	}, nil
}

func readOPF(files map[string]*zip.File) (opfPackage, string, error) {
	containerData, err := readZipFile(files, containerXMLPath)
	if err != nil {
		return opfPackage{}, "", err
	}
	var container epubContainer
	if err := xml.Unmarshal(containerData, &container); err != nil {
		return opfPackage{}, "", fmt.Errorf("epub: parse container.xml: %w", err)
	}
	if len(container.Rootfiles) == 0 || container.Rootfiles[0].FullPath == "" {
		return opfPackage{}, "", fmt.Errorf("epub: container.xml has no rootfile: %w", errMalformed)
	}
	opfPath := container.Rootfiles[0].FullPath
	opfData, err := readZipFile(files, opfPath)
	if err != nil {
		return opfPackage{}, "", err
	}
	var pkg opfPackage
	if err := xml.Unmarshal(opfData, &pkg); err != nil {
		return opfPackage{}, "", fmt.Errorf("epub: parse OPF: %w", err)
	}
	return pkg, opfPath, nil
}

func readSpineChapters(files map[string]*zip.File, pkg opfPackage, opfDir string) ([]Chapter, error) {
	items := make(map[string]opfItem, len(pkg.Manifest.Items))
	for _, item := range pkg.Manifest.Items {
		items[item.ID] = item
	}
	var chapters []Chapter
	for _, ref := range pkg.Spine.Itemrefs {
		item, ok := items[ref.IDRef]
		if !ok || skipSpineItem(item) {
			continue
		}
		doc, err := readZipFile(files, path.Clean(path.Join(opfDir, item.Href)))
		if err != nil {
			return nil, err
		}
		ex := stripMarkup(string(doc))
		if ex.heading == "" && len(ex.paras) == 0 {
			continue
		}
		chapters = append(chapters, Chapter{Title: ex.heading, Paragraphs: ex.paras})
	}
	return splitSingleDocChapters(chapters), nil
}

// splitSingleDocChapters re-splits a whole-book-in-one-spine-document EPUB
// with the chapter heading regex.
func splitSingleDocChapters(chapters []Chapter) []Chapter {
	if len(chapters) != 1 {
		return chapters
	}
	lines := extracted{heading: chapters[0].Title, paras: chapters[0].Paragraphs}.lines()
	text := strings.Join(lines, "\n")
	if !chapterRegexp().MatchString(text) {
		return chapters
	}
	return splitChapters(text)
}

// skipSpineItem reports whether a spine document is navigation or cover
// furniture rather than book content.
func skipSpineItem(item opfItem) bool {
	for prop := range strings.FieldsSeq(item.Properties) {
		if prop == "nav" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(item.ID), "cover")
}

func readZipFile(files map[string]*zip.File, name string) ([]byte, error) {
	f, ok := files[name]
	if !ok {
		return nil, fmt.Errorf("epub: missing archive entry %q: %w", name, errMalformed)
	}
	if f.UncompressedSize64 > uint64(maxZipEntryBytes) {
		return nil, fmt.Errorf("epub: archive entry %q exceeds %d bytes: %w",
			name, maxZipEntryBytes, errMalformed)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("epub: open archive entry %q: %w", name, err)
	}
	data, readErr := io.ReadAll(io.LimitReader(rc, maxZipEntryBytes+1))
	closeErr := rc.Close()
	if readErr != nil {
		return nil, fmt.Errorf("epub: read archive entry %q: %w", name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("epub: close archive entry %q: %w", name, closeErr)
	}
	if len(data) > maxZipEntryBytes {
		return nil, fmt.Errorf("epub: archive entry %q exceeds %d bytes: %w",
			name, maxZipEntryBytes, errMalformed)
	}
	return data, nil
}
