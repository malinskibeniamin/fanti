package seed

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

var errLicenceMissing = errors.New("tarball contained no ARPHICPL.TXT licence")

// StrokeChar is one character's stroke data from hanzi-writer-data:
// the decoded medians plus the raw per-character JSON (outlines and
// medians together — the shape the hanzi-writer renderer consumes).
type StrokeChar struct {
	Char    string
	Medians [][][2]float64
	Data    json.RawMessage
}

// hanziWriterFile is the per-character JSON shape in hanzi-writer-data.
type hanziWriterFile struct {
	Medians [][][2]float64 `json:"medians"`
}

// ExtractStrokeData reads a hanzi-writer-data npm tarball (.tgz) and returns
// every character's stroke medians plus the Arphic Public License text.
func ExtractStrokeData(tgz io.Reader) ([]StrokeChar, string, error) {
	gz, err := gzip.NewReader(tgz)
	if err != nil {
		return nil, "", fmt.Errorf("gunzip: %w", err)
	}

	defer func() { _ = gz.Close() }()

	var (
		chars   []StrokeChar
		licence string
	)

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, "", fmt.Errorf("read tar: %w", err)
		}

		name := path.Clean(hdr.Name)

		switch {
		case strings.HasSuffix(name, "ARPHICPL.TXT"):
			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil, "", fmt.Errorf("read licence: %w", err)
			}

			licence = string(raw)

		case strings.HasSuffix(name, ".json"):
			// Character files sit at package/<char>.json — a single-rune
			// basename. Skips package.json and any aggregates.
			base := strings.TrimSuffix(path.Base(name), ".json")
			if len([]rune(base)) != 1 {
				continue
			}

			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil, "", fmt.Errorf("read %s: %w", name, err)
			}

			var f hanziWriterFile
			if err := json.Unmarshal(raw, &f); err != nil {
				return nil, "", fmt.Errorf("decode %s: %w", name, err)
			}

			if len(f.Medians) == 0 {
				continue
			}

			chars = append(chars, StrokeChar{Char: base, Medians: f.Medians, Data: raw})
		}
	}

	if licence == "" {
		return nil, "", errLicenceMissing
	}

	return chars, licence, nil
}
