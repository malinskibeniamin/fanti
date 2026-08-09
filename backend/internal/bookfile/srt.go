package bookfile

import "strings"

// parseSRT parses SubRip cue blocks into a single untitled chapter whose
// paragraphs are the cue texts in order. Multi-line cue texts are joined with
// a space.
func parseSRT(data []byte) (Parsed, error) {
	text, err := decodeText(data)
	if err != nil {
		return Parsed{}, err
	}
	var paras []string
	for block := range strings.SplitSeq(normalizeNewlines(text), "\n\n") {
		if cue := parseCueBlock(block); cue != "" {
			paras = append(paras, cue)
		}
	}
	return Parsed{Chapters: []Chapter{{Title: "", Paragraphs: paras}}}, nil
}

func parseCueBlock(block string) string {
	var textLines []string
	sawTiming := false
	for line := range strings.SplitSeq(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !sawTiming {
			sawTiming = strings.Contains(line, "-->")
			continue
		}
		textLines = append(textLines, line)
	}
	if !sawTiming {
		return ""
	}
	return strings.Join(textLines, " ")
}
