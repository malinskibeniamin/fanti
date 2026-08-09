package seed

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

var errEmptyCEDICT = errors.New("no entries parsed from cedict data")

// Entry is one parsed CC-CEDICT line.
type Entry struct {
	Traditional string
	Simplified  string
	Pinyin      string
	Definitions []string
}

// ParseCEDICT reads the CC-CEDICT text format: one entry per line,
// `TRAD SIMP [pin1 yin1] /def/def/…/`, with `#` comments.
func ParseCEDICT(r io.Reader) ([]Entry, error) {
	var entries []Entry

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		if e, ok := parseCEDICTLine(scanner.Text()); ok {
			entries = append(entries, e)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan cedict: %w", err)
	}
	if len(entries) == 0 {
		return nil, errEmptyCEDICT
	}

	return entries, nil
}

func parseCEDICTLine(line string) (Entry, bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return Entry{}, false
	}

	trad, rest, ok := strings.Cut(line, " ")
	if !ok {
		return Entry{}, false
	}

	simp, rest, ok := strings.Cut(rest, " ")
	if !ok {
		return Entry{}, false
	}

	if !strings.HasPrefix(rest, "[") {
		return Entry{}, false
	}

	numbered, rest, ok := strings.Cut(rest[1:], "]")
	if !ok {
		return Entry{}, false
	}

	defs := strings.Split(strings.Trim(strings.TrimSpace(rest), "/"), "/")
	if len(defs) == 1 && defs[0] == "" {
		return Entry{}, false
	}

	return Entry{
		Traditional: trad,
		Simplified:  simp,
		Pinyin:      MarkPinyin(numbered),
		Definitions: defs,
	}, true
}
