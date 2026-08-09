package bookfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"
)

func u32len(t *testing.T, n int) uint32 {
	t.Helper()
	if n < 0 || n > math.MaxUint32 {
		t.Fatalf("length %d out of uint32 range", n)
	}
	return uint32(n) //nolint:gosec // bounds checked above
}

type mobiFixture struct {
	compression  uint16
	encryption   uint16
	textEncoding uint32
	fullName     string
	textRecord   []byte
	textLength   uint32
}

// buildTestMOBI constructs a minimal two-record PalmDB/MOBI file: record 0
// holds the PalmDoc + MOBI headers and the full name, record 1 the text.
func buildTestMOBI(t *testing.T, fix mobiFixture) []byte {
	t.Helper()
	name := []byte(fix.fullName)
	rec0 := make([]byte, 248+len(name)+2)
	binary.BigEndian.PutUint16(rec0[0:], fix.compression)
	binary.BigEndian.PutUint32(rec0[4:], fix.textLength)
	binary.BigEndian.PutUint16(rec0[8:], 1) // recordCount
	binary.BigEndian.PutUint16(rec0[10:], 4096)
	binary.BigEndian.PutUint16(rec0[12:], fix.encryption)
	copy(rec0[16:], "MOBI")
	binary.BigEndian.PutUint32(rec0[20:], 232) // MOBI header length
	binary.BigEndian.PutUint32(rec0[24:], 2)   // mobiType: book
	binary.BigEndian.PutUint32(rec0[28:], fix.textEncoding)
	binary.BigEndian.PutUint32(rec0[84:], 248) // fullNameOffset
	binary.BigEndian.PutUint32(rec0[88:], u32len(t, len(name)))
	binary.BigEndian.PutUint32(rec0[168:], 0xFFFFFFFF) // DRM offset: none
	binary.BigEndian.PutUint32(rec0[172:], 0xFFFFFFFF) // DRM count: none
	copy(rec0[248:], name)

	header := make([]byte, 78+2*8)
	copy(header[0:], "testbook")
	copy(header[60:], "BOOK")
	copy(header[64:], "MOBI")
	binary.BigEndian.PutUint16(header[76:], 2)
	binary.BigEndian.PutUint32(header[78:], u32len(t, len(header)))
	binary.BigEndian.PutUint32(header[86:], u32len(t, len(header)+len(rec0)))

	out := append([]byte{}, header...)
	out = append(out, rec0...)
	return append(out, fix.textRecord...)
}

func TestParseMOBIUncompressed(t *testing.T) {
	html := "<html><body><h2>第一章 起點</h2><p>山中有雨。</p><h2>第二章 轉折</h2><p>江上有船。</p></body></html>"
	data := buildTestMOBI(t, mobiFixture{
		compression:  1,
		textEncoding: 65001,
		fullName:     "測試小說",
		textRecord:   []byte(html),
		textLength:   u32len(t, len(html)),
	})
	got, err := Parse("book.mobi", data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Format != FormatMOBI {
		t.Errorf("Format = %q, want %q", got.Format, FormatMOBI)
	}
	if got.Title != "測試小說" {
		t.Errorf("Title = %q, want 測試小說", got.Title)
	}
	want := []Chapter{
		{Title: titleCh1, Paragraphs: []string{paraRain}},
		{Title: "第二章 轉折", Paragraphs: []string{paraBoat}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestParseMOBIPalmDocCompressed(t *testing.T) {
	// "<p>hello" literals, 0xE8 = space+'h', pair {0x80,0x31} copies "ello"
	// (distance 6, length 4), then "</p>" literals: "<p>hello hello</p>".
	compressed := append([]byte("<p>hello"), 0xE8, 0x80, 0x31)
	compressed = append(compressed, []byte("</p>")...)
	data := buildTestMOBI(t, mobiFixture{
		compression:  2,
		textEncoding: 65001,
		fullName:     "compressed",
		textRecord:   compressed,
		textLength:   18,
	})
	got, err := Parse("book.mobi", data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := []Chapter{
		{Title: "", Paragraphs: []string{"hello hello"}},
	}
	if !reflect.DeepEqual(got.Chapters, want) {
		t.Errorf("Chapters = %#v, want %#v", got.Chapters, want)
	}
}

func TestParseMOBIHuffCompression(t *testing.T) {
	data := buildTestMOBI(t, mobiFixture{
		compression:  17480,
		textEncoding: 65001,
		fullName:     "huffed",
		textRecord:   []byte("irrelevant"),
		textLength:   10,
	})
	_, err := Parse("book.mobi", data)
	if !errors.Is(err, ErrHuffCompression) {
		t.Fatalf("Parse() error = %v, want ErrHuffCompression", err)
	}
}

func TestParseMOBIDRM(t *testing.T) {
	data := buildTestMOBI(t, mobiFixture{
		compression:  1,
		encryption:   2,
		textEncoding: 65001,
		fullName:     "locked",
		textRecord:   []byte("irrelevant"),
		textLength:   10,
	})
	_, err := Parse("book.mobi", data)
	if !errors.Is(err, ErrDRM) {
		t.Fatalf("Parse() error = %v, want ErrDRM", err)
	}
}

func TestPalmDocDecompress(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{
			name: "literals pairs and space codes",
			// "hello", space+'h', two overlapping history copies, literal
			// run of one '!', then space+'A'.
			in:   []byte{'h', 'e', 'l', 'l', 'o', 0xE8, 0x80, 0x32, 0x80, 0x32, 0x01, '!', 0xC1},
			want: []byte("hello hello hello! A"),
		},
		{
			name: "overlapping copy repeats single byte",
			in:   []byte{'a', 0x80, 0x0F}, // distance 1, length 10
			want: []byte("aaaaaaaaaaa"),
		},
		{
			name: "zero byte literal",
			in:   []byte{0x00, 'x'},
			want: []byte{0x00, 'x'},
		},
		{
			name: "multi byte literal run",
			in:   []byte{0x03, 0x80, 0xC1, 0x00, 'y'},
			want: []byte{0x80, 0xC1, 0x00, 'y'},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := palmDocDecompress(tc.in)
			if !bytes.Equal(got, tc.want) {
				t.Errorf("palmDocDecompress(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
