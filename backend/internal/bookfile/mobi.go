package bookfile

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

const (
	palmHeaderSize      = 78
	palmRecordEntrySize = 8
	compressionNone     = 1
	compressionPalmDoc  = 2
	compressionHuff     = 17480
	mobiNoDRM           = 0xFFFFFFFF
)

// mobiEncoding codes from the MOBI header textEncoding field.
const (
	mobiEncodingCP1252 = 1252
	mobiEncodingGB     = 936
	mobiEncodingBig5   = 950
	mobiEncodingUTF8   = 65001
)

func parseMOBI(data []byte) (Parsed, error) {
	records, err := palmRecords(data)
	if err != nil {
		return Parsed{}, err
	}
	rec0 := records[0]
	if len(rec0) < 16 {
		return Parsed{}, fmt.Errorf("mobi: record 0 shorter than PalmDoc header: %w", errMalformed)
	}
	compression := binary.BigEndian.Uint16(rec0[0:2])
	textLength := int64(binary.BigEndian.Uint32(rec0[4:8]))
	recordCount := int(binary.BigEndian.Uint16(rec0[8:10]))
	if encryption := binary.BigEndian.Uint16(rec0[12:14]); encryption != 0 {
		return Parsed{}, fmt.Errorf("mobi: encryption type %d: %w", encryption, ErrDRM)
	}
	switch compression {
	case compressionNone, compressionPalmDoc:
	case compressionHuff:
		return Parsed{}, fmt.Errorf("mobi: compression %d: %w", compression, ErrHuffCompression)
	default:
		return Parsed{}, fmt.Errorf("mobi: unknown compression %d: %w", compression, errMalformed)
	}
	title, textEncoding, err := parseMOBIHeader(rec0)
	if err != nil {
		return Parsed{}, err
	}
	if recordCount > len(records)-1 {
		recordCount = len(records) - 1
	}
	var raw []byte
	for i := 1; i <= recordCount; i++ {
		// Trailing extra-bytes flags are assumed to be 0 for v1, so each
		// text record is decompressed whole.
		rec := records[i]
		if compression == compressionPalmDoc {
			rec = palmDocDecompress(rec)
		}
		raw = append(raw, rec...)
	}
	if int64(len(raw)) > textLength {
		raw = raw[:textLength]
	}
	ex := stripMarkup(decodeMOBIString(raw, textEncoding))
	return Parsed{
		Title:    title,
		Chapters: splitChapters(strings.Join(ex.lines(), "\n")),
	}, nil
}

// palmRecords slices a PalmDB container into its records.
func palmRecords(data []byte) ([][]byte, error) {
	if len(data) < palmHeaderSize+palmRecordEntrySize {
		return nil, fmt.Errorf("mobi: file shorter than PalmDB header: %w", errMalformed)
	}
	n := int(binary.BigEndian.Uint16(data[76:78]))
	if n == 0 || palmHeaderSize+n*palmRecordEntrySize > len(data) {
		return nil, fmt.Errorf("mobi: record list truncated: %w", errMalformed)
	}
	offsets := make([]int64, n+1)
	for i := range n {
		offsets[i] = int64(binary.BigEndian.Uint32(data[palmHeaderSize+i*palmRecordEntrySize:]))
	}
	offsets[n] = int64(len(data))
	records := make([][]byte, n)
	for i := range n {
		if offsets[i] > offsets[i+1] || offsets[i+1] > int64(len(data)) {
			return nil, fmt.Errorf("mobi: record %d offsets out of range: %w", i, errMalformed)
		}
		records[i] = data[offsets[i]:offsets[i+1]]
	}
	return records, nil
}

// parseMOBIHeader reads the optional MOBI header inside record 0, returning
// the full name (title) and text encoding. Field offsets are relative to the
// start of record 0.
func parseMOBIHeader(rec0 []byte) (string, uint32, error) {
	var textEncoding uint32 = mobiEncodingUTF8
	if len(rec0) < 24 || string(rec0[16:20]) != "MOBI" {
		return "", textEncoding, nil
	}
	headerLength := int64(binary.BigEndian.Uint32(rec0[20:24]))
	if len(rec0) >= 32 {
		textEncoding = binary.BigEndian.Uint32(rec0[28:32])
	}
	if headerLength+16 >= 176 && len(rec0) >= 176 {
		if drmCount := binary.BigEndian.Uint32(rec0[172:176]); drmCount != 0 && drmCount != mobiNoDRM {
			return "", 0, fmt.Errorf("mobi: %d DRM records: %w", drmCount, ErrDRM)
		}
	}
	title := ""
	if len(rec0) >= 92 {
		nameOffset := int64(binary.BigEndian.Uint32(rec0[84:88]))
		nameLength := int64(binary.BigEndian.Uint32(rec0[88:92]))
		if nameOffset > 0 && nameLength > 0 && nameOffset+nameLength <= int64(len(rec0)) {
			title = decodeMOBIString(rec0[nameOffset:nameOffset+nameLength], textEncoding)
		}
	}
	return title, textEncoding, nil
}

// decodeMOBIString decodes bytes according to the MOBI textEncoding field,
// defaulting to a UTF-8 attempt with a cp1252 fallback.
func decodeMOBIString(raw []byte, textEncoding uint32) string {
	decode := func(s string, err error) string {
		if err != nil {
			return string(raw)
		}
		return s
	}
	switch textEncoding {
	case mobiEncodingCP1252:
		return decode(decodeBytes(charmap.Windows1252, raw))
	case mobiEncodingGB:
		return decode(decodeBytes(simplifiedchinese.GB18030, raw))
	case mobiEncodingBig5:
		return decode(decodeBytes(traditionalchinese.Big5, raw))
	default:
		if utf8.Valid(raw) {
			return string(raw)
		}
		return decode(decodeBytes(charmap.Windows1252, raw))
	}
}

// palmDocDecompress applies PalmDoc LZ77 decompression.
func palmDocDecompress(src []byte) []byte {
	out := make([]byte, 0, len(src)*2)
	for i := 0; i < len(src); {
		b := src[i]
		i++
		switch {
		case b == 0x00 || (b >= 0x09 && b <= 0x7F):
			out = append(out, b)
		case b <= 0x08: // 0x01-0x08: literal run
			n := int(b)
			if i+n > len(src) {
				n = len(src) - i
			}
			out = append(out, src[i:i+n]...)
			i += n
		case b <= 0xBF: // 0x80-0xBF: history copy
			if i >= len(src) {
				return out
			}
			pair := (uint16(b)<<8 | uint16(src[i])) & 0x3FFF
			i++
			distance := int(pair >> 3 & 0x7FF)
			length := int(pair&7) + 3
			if distance == 0 || distance > len(out) {
				continue
			}
			for range length {
				out = append(out, out[len(out)-distance])
			}
		default: // 0xC0-0xFF: space plus char
			out = append(out, ' ', b^0x80)
		}
	}
	return out
}
