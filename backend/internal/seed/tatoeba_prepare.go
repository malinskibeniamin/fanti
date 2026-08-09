package seed

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// Tatoeba per-language export locations (CC BY 2.0 FR — see NOTICES.md).
// The English export is only read while preparing the vendored derivative;
// it is far too large to vendor itself.
const (
	TatoebaCmnURL   = "https://downloads.tatoeba.org/exports/per_language/cmn/cmn_sentences.tsv.bz2"
	TatoebaLinksURL = "https://downloads.tatoeba.org/exports/per_language/cmn/cmn-eng_links.tsv.bz2"
	TatoebaEngURL   = "https://downloads.tatoeba.org/exports/per_language/eng/eng_sentences.tsv.bz2"
)

var errEmptyTatoeba = errors.New("no sentence pairs joined from tatoeba exports")

// tatoebaScanner wraps the exports' long lines; Tatoeba has no quote
// escaping, so fields are split on literal tabs only (encoding/csv would
// misread fields that start with a double quote).
func tatoebaScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)

	return sc
}

// PrepareTatoeba joins the three Tatoeba exports — Mandarin sentences
// (`id\tlang\ttext`), cmn→eng links (`cmn_id\teng_id`), and English
// sentences — into the vendored derivative (`cmn_id\tcmn\teng`), one
// English translation per Mandarin sentence (lowest sentence id wins, so
// output is deterministic). Returns the number of rows written.
func PrepareTatoeba(cmn, links, eng io.Reader, out io.Writer) (int, error) {
	cmnText, err := readTatoebaSentences(cmn)
	if err != nil {
		return 0, err
	}

	engIDByCmn, needed, err := readTatoebaLinks(links, cmnText)
	if err != nil {
		return 0, err
	}

	engText, err := readNeededSentences(eng, needed)
	if err != nil {
		return 0, err
	}

	ids := make([]int64, 0, len(engIDByCmn))
	for id := range engIDByCmn {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	w := bufio.NewWriter(out)
	rows := 0

	for _, id := range ids {
		text := cmnText[id]

		english, ok := bestTranslation(engIDByCmn[id], engText)
		if !ok || text == "" {
			continue
		}

		// A literal tab inside community text would corrupt the row's
		// field boundaries on re-parse.
		text = sanitizeField(text)
		english = sanitizeField(english)

		if _, err := fmt.Fprintf(w, "%d\t%s\t%s\n", id, text, english); err != nil {
			return 0, fmt.Errorf("write pair %d: %w", id, err)
		}

		rows++
	}

	if err := w.Flush(); err != nil {
		return 0, fmt.Errorf("flush output: %w", err)
	}

	if rows == 0 {
		return 0, errEmptyTatoeba
	}

	return rows, nil
}

// sanitizeField flattens the derivative's separator characters out of a
// text field.
func sanitizeField(s string) string {
	return strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(s)
}

// readTatoebaSentences maps sentence id → text from an `id\tlang\ttext` export.
func readTatoebaSentences(r io.Reader) (map[int64]string, error) {
	text := map[int64]string{}
	sc := tatoebaScanner(r)

	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}

		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		text[id] = parts[2]
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read sentences: %w", err)
	}

	return text, nil
}

// readTatoebaLinks maps cmn id → linked eng ids, keeping only links whose
// Mandarin side exists, and collects the English ids worth retaining.
func readTatoebaLinks(r io.Reader, cmnText map[int64]string) (map[int64][]int64, map[int64]bool, error) {
	engIDByCmn := map[int64][]int64{}
	needed := map[int64]bool{}
	sc := tatoebaScanner(r)

	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 2)
		if len(parts) != 2 {
			continue
		}

		cmnID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}

		engID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}

		if _, ok := cmnText[cmnID]; !ok {
			continue
		}

		engIDByCmn[cmnID] = append(engIDByCmn[cmnID], engID)
		needed[engID] = true
	}

	if err := sc.Err(); err != nil {
		return nil, nil, fmt.Errorf("read links: %w", err)
	}

	return engIDByCmn, needed, nil
}

// readNeededSentences streams the (very large) English export, keeping
// only the linked sentences.
func readNeededSentences(r io.Reader, needed map[int64]bool) (map[int64]string, error) {
	text := map[int64]string{}
	sc := tatoebaScanner(r)

	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}

		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || !needed[id] {
			continue
		}

		text[id] = parts[2]
	}

	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read english sentences: %w", err)
	}

	return text, nil
}

// bestTranslation picks the lowest-id linked English sentence present in
// the export.
func bestTranslation(engIDs []int64, engText map[int64]string) (string, bool) {
	best := ""
	bestID := int64(-1)

	for _, id := range engIDs {
		t, ok := engText[id]
		if !ok || t == "" {
			continue
		}

		if bestID == -1 || id < bestID {
			bestID = id
			best = t
		}
	}

	return best, bestID != -1
}
