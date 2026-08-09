// Package convert wraps OpenCC dictionary conversion between Simplified
// and Traditional Chinese with exception detection and reporting.
package convert

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/longbridgeapp/opencc"

	"github.com/malinskibeniamin/fanti/backend/data"
)

// Direction of conversion.
type Direction string

// Conversion directions.
const (
	S2T Direction = "s2t"
	T2S Direction = "t2s"
)

// Options controls one conversion run.
type Options struct {
	Direction Direction
	// Localize vocabulary for the target region (软件→軟體).
	Localize bool
	// Convert quote punctuation ("…" ⇄ 「…」).
	Punctuation bool
	// Resolutions overrides ambiguous character mappings: source char →
	// chosen target form (applied after dictionary conversion).
	Resolutions map[string]string
}

// Chapter is one unit of converted text.
type Chapter struct {
	Title      string
	Paragraphs []string
}

// LocalizedText mirrors the authored tri-lingual note shape.
type LocalizedText struct {
	En string `json:"en"`
	Tc string `json:"tc"`
	Sc string `json:"sc"`
}

const statusAmbiguous = "ambiguous"

// Exception is a one-to-many mapping found in the source text.
type Exception struct {
	SourceChar  string        `json:"sourceChar"`
	Options     []string      `json:"options"`
	Note        LocalizedText `json:"note"`
	Context     string        `json:"context"`
	Status      string        `json:"status"` // "ambiguous" | "manual"
	Occurrences int64         `json:"occurrences"`
}

// DiffToken is one character of the diff preview with its mapping status.
type DiffToken struct {
	Text   string `json:"text"`
	Status string `json:"status"` // "", "exact", "ambiguous"
}

// Diff is a short before/after excerpt.
type Diff struct {
	SourceText string      `json:"sourceText"`
	Tokens     []DiffToken `json:"tokens"`
}

// Report summarizes a conversion run.
type Report struct {
	Exact      int64
	Ambiguous  int64
	Manual     int64
	Total      int64
	Exceptions []Exception
	Diff       Diff
}

// fixtureException mirrors conversion-exceptions.json.
type fixtureException struct {
	SourceChar string        `json:"sourceChar"`
	Options    []string      `json:"options"`
	Note       LocalizedText `json:"note"`
	Context    string        `json:"context"`
	Status     string        `json:"status"`
}

// Engine holds the loaded OpenCC converters.
type Engine struct {
	// base converts glyphs to the target standard; localized additionally
	// swaps regional vocabulary.
	base       map[Direction]*opencc.OpenCC
	localized  map[Direction]*opencc.OpenCC
	exceptions map[Direction][]fixtureException
}

// NewEngine loads the OpenCC dictionaries and curated exception data.
func NewEngine() (*Engine, error) {
	e := &Engine{
		base:      map[Direction]*opencc.OpenCC{},
		localized: map[Direction]*opencc.OpenCC{},
	}

	// s2tw/tw2s give Taiwan-standard glyphs (麵/裡); the *p variants add
	// regional vocabulary (软件→軟體).
	for name, slot := range map[string]struct {
		dir   Direction
		local bool
	}{
		"s2tw": {S2T, false}, "s2twp": {S2T, true},
		"tw2s": {T2S, false}, "tw2sp": {T2S, true},
	} {
		cc, err := opencc.New(name)
		if err != nil {
			return nil, fmt.Errorf("load opencc %s: %w", name, err)
		}

		if slot.local {
			e.localized[slot.dir] = cc
		} else {
			e.base[slot.dir] = cc
		}
	}

	raw, err := data.SeedFS.ReadFile("seed/conversion-exceptions.json")
	if err != nil {
		return nil, fmt.Errorf("read exceptions fixture: %w", err)
	}

	var fixture struct {
		S2T []fixtureException `json:"s2t"`
		T2S []fixtureException `json:"t2s"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		return nil, fmt.Errorf("decode exceptions fixture: %w", err)
	}

	e.exceptions = map[Direction][]fixtureException{S2T: fixture.S2T, T2S: fixture.T2S}

	return e, nil
}

//nolint:gochecknoglobals // static punctuation mapping
var punctS2T = strings.NewReplacer("“", "「", "”", "」", "‘", "『", "’", "』")

//nolint:gochecknoglobals // static punctuation mapping
var punctT2S = strings.NewReplacer("「", "“", "」", "”", "『", "‘", "』", "’")

// ConvertText converts one string.
func (e *Engine) ConvertText(text string, o Options) (string, error) {
	cc := e.base[o.Direction]
	if o.Localize {
		cc = e.localized[o.Direction]
	}

	if cc == nil {
		return "", fmt.Errorf("unknown direction %q: %w", o.Direction, errBadDirection)
	}

	out, err := cc.Convert(text)
	if err != nil {
		return "", fmt.Errorf("opencc convert: %w", err)
	}

	if o.Punctuation {
		if o.Direction == S2T {
			out = punctS2T.Replace(out)
		} else {
			out = punctT2S.Replace(out)
		}
	}

	if len(o.Resolutions) > 0 {
		out = e.applyResolutions(text, out, o)
	}

	return out, nil
}

// ConvertChapters converts every chapter, reporting progress after each
// paragraph, and builds the conversion report.
func (e *Engine) ConvertChapters(
	chapters []Chapter, o Options, progress func(done, total int),
) ([]Chapter, Report, error) {
	total := 0
	for _, ch := range chapters {
		total += len(ch.Paragraphs) + 1 // +1 for the title
	}

	report := Report{}
	counts := map[string]int64{}
	contexts := map[string]string{}
	done := 0

	converted := make([]Chapter, len(chapters))

	for ci, ch := range chapters {
		title, err := e.ConvertText(ch.Title, o)
		if err != nil {
			return nil, Report{}, err
		}

		done++

		converted[ci] = Chapter{Title: title, Paragraphs: make([]string, len(ch.Paragraphs))}

		for pi, para := range ch.Paragraphs {
			out, err := e.ConvertText(para, o)
			if err != nil {
				return nil, Report{}, err
			}

			converted[ci].Paragraphs[pi] = out
			e.tally(para, out, o.Direction, &report, counts, contexts)

			done++

			if progress != nil {
				progress(done, total)
			}
		}
	}

	e.buildExceptions(o.Direction, counts, contexts, &report)
	e.buildDiff(chapters, converted, o.Direction, &report)

	report.Total = report.Exact + report.Ambiguous + report.Manual

	return converted, report, nil
}

// applyResolutions re-walks the source/converted pair and forces the chosen
// target form wherever an overridden source character appears. Alignment is
// positional, so it only applies when the dictionary pass preserved length
// (character conversion does; vocabulary localization may not).
func (e *Engine) applyResolutions(src, converted string, o Options) string {
	srcRunes := []rune(src)

	outRunes := []rune(converted)
	if len(srcRunes) != len(outRunes) {
		return converted
	}

	for i, r := range srcRunes {
		if chosen, ok := o.Resolutions[string(r)]; ok && chosen != "" {
			replacement := []rune(chosen)
			if len(replacement) == 1 {
				outRunes[i] = replacement[0]
			}
		}
	}

	return string(outRunes)
}

// tally counts substitutions in one aligned source/converted paragraph pair.
func (e *Engine) tally(
	src, out string, dir Direction,
	report *Report, counts map[string]int64, contexts map[string]string,
) {
	ambiguousSet := map[string]bool{}
	for _, ex := range e.exceptions[dir] {
		if ex.Status == statusAmbiguous {
			ambiguousSet[ex.SourceChar] = true
		}
	}

	srcRunes := []rune(src)

	outRunes := []rune(out)
	if len(srcRunes) != len(outRunes) {
		// Vocabulary localization changed length; count the delta as exact.
		report.Exact += absInt64(int64(len(outRunes)) - int64(len(srcRunes)))
		srcRunes = srcRunes[:minInt(len(srcRunes), len(outRunes))]
		outRunes = outRunes[:len(srcRunes)]
	}

	for i, r := range srcRunes {
		ch := string(r)

		if ambiguousSet[ch] {
			report.Ambiguous++
			counts[ch]++

			if contexts[ch] == "" {
				contexts[ch] = snippet(srcRunes, i)
			}

			continue
		}

		if outRunes[i] != r {
			report.Exact++
		}
	}
}

// buildExceptions materializes curated exceptions found in the text.
func (e *Engine) buildExceptions(
	dir Direction, counts map[string]int64, contexts map[string]string, report *Report,
) {
	for _, fx := range e.exceptions[dir] {
		occurrences := counts[fx.SourceChar]
		if fx.Status == statusAmbiguous && occurrences == 0 {
			continue
		}

		context := contexts[fx.SourceChar]
		if context == "" {
			context = fx.Context
		}

		if fx.Status == "manual" {
			report.Manual++
		}

		report.Exceptions = append(report.Exceptions, Exception{
			SourceChar:  fx.SourceChar,
			Options:     fx.Options,
			Note:        fx.Note,
			Context:     context,
			Status:      fx.Status,
			Occurrences: occurrences,
		})
	}
}

// buildDiff renders the first sentence of the book as a per-character
// status preview.
func (e *Engine) buildDiff(src, converted []Chapter, dir Direction, report *Report) {
	const maxDiffRunes = 40

	ambiguousSet := map[string]bool{}
	for _, ex := range e.exceptions[dir] {
		if ex.Status == statusAmbiguous {
			ambiguousSet[ex.SourceChar] = true
		}
	}

	for ci := range src {
		for pi := range src[ci].Paragraphs {
			srcRunes := []rune(src[ci].Paragraphs[pi])

			outRunes := []rune(converted[ci].Paragraphs[pi])
			if len(srcRunes) == 0 || len(srcRunes) != len(outRunes) {
				continue
			}

			n := minInt(len(srcRunes), maxDiffRunes)
			report.Diff.SourceText = string(srcRunes[:n])

			for i := range n {
				token := DiffToken{Text: string(outRunes[i])}

				switch {
				case ambiguousSet[string(srcRunes[i])]:
					token.Status = "ambiguous"
				case srcRunes[i] != outRunes[i]:
					token.Status = "exact"
				}

				report.Diff.Tokens = append(report.Diff.Tokens, token)
			}

			return
		}
	}
}

// snippet extracts a short context window around position i.
func snippet(runes []rune, i int) string {
	const window = 6

	lo := maxInt(0, i-window)
	hi := minInt(len(runes), i+window+1)

	return "「" + string(runes[lo:hi]) + "」"
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func absInt64(v int64) int64 {
	if v < 0 {
		return -v
	}

	return v
}

var errBadDirection = errors.New("direction must be s2t or t2s")

// DetectScript guesses the script of a text sample: converting away from
// the actual source script changes many characters, converting toward it
// changes almost none.
func (e *Engine) DetectScript(sample string) (Direction, error) {
	const maxSample = 2000

	runes := []rune(sample)
	if len(runes) > maxSample {
		runes = runes[:maxSample]
	}

	text := string(runes)

	toTrad, err := e.ConvertText(text, Options{Direction: S2T})
	if err != nil {
		return "", err
	}

	toSimp, err := e.ConvertText(text, Options{Direction: T2S})
	if err != nil {
		return "", err
	}

	if runeDiff(text, toTrad) > runeDiff(text, toSimp) {
		return S2T, nil // source is simplified → natural direction 简→繁
	}

	return T2S, nil
}

func runeDiff(a, b string) int {
	ar, br := []rune(a), []rune(b)

	n := minInt(len(ar), len(br))
	diff := absInt64(int64(len(ar)) - int64(len(br)))

	for i := range n {
		if ar[i] != br[i] {
			diff++
		}
	}

	return int(diff)
}
