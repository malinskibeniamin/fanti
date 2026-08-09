package bookfile

import (
	"regexp"
	"strings"
)

// chapterPattern matches a line that starts a new chapter: numbered headings
// (第一章, 第3節, 第十二回, ...) and common front/back-matter markers.
const chapterPattern = `(?m)^[\t 　]*(?:第?[0-9一二三四五六七八九十百千零两]{1,4}[章節回篇卷部]|简介|序|楔子|尾声|后记|番外)[　 ]{0,10}.*$`

// chapterRegexp compiles the chapter-heading pattern. It is compiled per call
// (cheap relative to parsing a whole book) to keep the package free of
// mutable globals.
func chapterRegexp() *regexp.Regexp {
	return regexp.MustCompile(chapterPattern)
}

// splitChapters splits plain text into chapters at chapter-heading lines. The
// heading line becomes the chapter title; text before the first heading
// becomes an untitled chapter when non-empty. Paragraphs are non-empty lines,
// trimmed.
func splitChapters(text string) []Chapter {
	text = normalizeNewlines(text)
	locs := chapterRegexp().FindAllStringIndex(text, -1)
	var chapters []Chapter
	preambleEnd := len(text)
	if len(locs) > 0 {
		preambleEnd = locs[0][0]
	}
	if paras := textToParagraphs(text[:preambleEnd]); len(paras) > 0 {
		chapters = append(chapters, Chapter{Title: "", Paragraphs: paras})
	}
	for i, loc := range locs {
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		chapters = append(chapters, Chapter{
			Title:      strings.TrimSpace(text[loc[0]:loc[1]]),
			Paragraphs: textToParagraphs(text[loc[1]:end]),
		})
	}
	return chapters
}

func normalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func textToParagraphs(text string) []string {
	lines := strings.Split(text, "\n")
	paras := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			paras = append(paras, trimmed)
		}
	}
	if len(paras) == 0 {
		return nil
	}
	return paras
}
