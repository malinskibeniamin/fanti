// Package bookfile parses uploaded book files (EPUB, TXT, SRT, MOBI) into a
// common chapter structure and writes EPUB3 output.
package bookfile

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Format identifies a supported book file format.
type Format string

// Supported formats.
const (
	FormatEPUB Format = "epub"
	FormatTXT  Format = "txt"
	FormatSRT  Format = "srt"
	FormatMOBI Format = "mobi"
)

// Chapter is one chapter of a parsed book.
type Chapter struct {
	Title      string
	Paragraphs []string
}

// Parsed is the format-independent result of parsing a book file.
type Parsed struct {
	Format Format
	// Title comes from metadata when available (EPUB dc:title, MOBI full
	// name), else "".
	Title string
	// Author comes from metadata when available, else "".
	Author   string
	Chapters []Chapter
	// CharCount is the total number of runes (CJK and other) across all
	// paragraphs.
	CharCount int64
}

// Sentinel errors returned by Parse.
var (
	// ErrUnsupportedFormat means the file's extension and magic bytes match
	// no supported format.
	ErrUnsupportedFormat = errors.New("unsupported book format")
	// ErrDRM means the MOBI file is encrypted or carries DRM records.
	ErrDRM = errors.New("mobi file is DRM-protected")
	// ErrHuffCompression means the MOBI file uses HUFF/CDIC compression.
	ErrHuffCompression = errors.New("mobi file uses HUFF/CDIC compression; convert it to EPUB or uncompressed MOBI with Calibre first")

	errMalformed       = errors.New("malformed book file")
	errArchiveTooLarge = errors.New("expanded EPUB exceeds size limit")
)

// Parse sniffs the format from the filename extension and magic bytes, then
// dispatches to the matching format parser.
func Parse(filename string, data []byte) (Parsed, error) {
	format := detectFormat(filename, data)
	var (
		parsed Parsed
		err    error
	)
	switch format {
	case FormatEPUB:
		parsed, err = parseEPUB(data)
	case FormatTXT:
		parsed, err = parseTXT(data)
	case FormatSRT:
		parsed, err = parseSRT(data)
	case FormatMOBI:
		parsed, err = parseMOBI(data)
	default:
		return Parsed{}, fmt.Errorf("%q: %w", filename, ErrUnsupportedFormat)
	}
	if err != nil {
		return Parsed{}, err
	}
	parsed.Format = format
	parsed.CharCount = countRunes(parsed.Chapters)
	return parsed, nil
}

func detectFormat(filename string, data []byte) Format {
	if len(data) >= 68 && string(data[60:68]) == "BOOKMOBI" {
		return FormatMOBI
	}
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return FormatEPUB
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".txt":
		return FormatTXT
	case ".srt":
		return FormatSRT
	case ".epub":
		return FormatEPUB
	case ".mobi", ".azw", ".azw3":
		return FormatMOBI
	default:
		return ""
	}
}

func countRunes(chapters []Chapter) int64 {
	var total int64
	for _, ch := range chapters {
		for _, p := range ch.Paragraphs {
			total += int64(utf8.RuneCountInString(p))
		}
	}
	return total
}
