package bookfile

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// extracted is the readable text pulled out of one (X)HTML document.
type extracted struct {
	// heading is the content of the first h1-h6 element, used as the
	// chapter title. It is excluded from paras.
	heading string
	paras   []string
}

// lines returns the heading (when present) followed by the paragraphs, for
// callers that re-split the document with the chapter regex.
func (e extracted) lines() []string {
	if e.heading == "" {
		return e.paras
	}
	return append([]string{e.heading}, e.paras...)
}

// stripMarkup extracts readable text from lenient (X)HTML without an HTML
// parser dependency: tags are dropped, block-level boundaries become
// paragraph breaks, and the first heading becomes the title.
func stripMarkup(src string) extracted {
	s := &markupStripper{}
	for len(src) > 0 {
		lt := strings.IndexByte(src, '<')
		if lt < 0 {
			s.text(src)
			break
		}
		s.text(src[:lt])
		src = src[lt:]
		if strings.HasPrefix(src, "<!--") {
			end := strings.Index(src, "-->")
			if end < 0 {
				break
			}
			src = src[end+3:]
			continue
		}
		gt := strings.IndexByte(src, '>')
		if gt < 0 {
			break
		}
		s.tag(src[1:gt])
		src = src[gt+1:]
	}
	return s.finish()
}

type markupStripper struct {
	paras       []string
	cur         strings.Builder
	headingBuf  strings.Builder
	heading     string
	skipUntil   string // element name whose closing tag ends skipped content
	headingTag  string // heading element currently being captured
	headingDone bool
}

func (s *markupStripper) text(seg string) {
	if s.skipUntil != "" || seg == "" {
		return
	}
	if s.headingTag != "" {
		s.headingBuf.WriteString(decodeEntities(seg))
		return
	}
	s.cur.WriteString(decodeEntities(seg))
}

func (s *markupStripper) tag(raw string) {
	closing := strings.HasPrefix(raw, "/")
	name := tagName(raw)
	if s.skipUntil != "" {
		if closing && name == s.skipUntil {
			s.skipUntil = ""
		}
		return
	}
	switch {
	case name == "script" || name == "style" || name == "head" || name == "title":
		if !closing && !strings.HasSuffix(strings.TrimSpace(raw), "/") {
			s.skipUntil = name
		}
	case isHeadingTag(name):
		s.headingBoundary(name, closing)
	case isBlockTag(name):
		s.flush()
	}
}

func (s *markupStripper) headingBoundary(name string, closing bool) {
	if closing {
		if s.headingTag == name {
			s.heading = collapseWhitespace(s.headingBuf.String())
			s.headingTag = ""
			s.headingDone = true
			return
		}
		s.flush()
		return
	}
	s.flush()
	if !s.headingDone {
		s.headingTag = name
	}
}

func (s *markupStripper) flush() {
	p := collapseWhitespace(s.cur.String())
	s.cur.Reset()
	if p != "" {
		s.paras = append(s.paras, p)
	}
}

func (s *markupStripper) finish() extracted {
	if s.headingTag != "" && !s.headingDone {
		s.heading = collapseWhitespace(s.headingBuf.String())
	}
	s.flush()
	return extracted{heading: s.heading, paras: s.paras}
}

func tagName(raw string) string {
	raw = strings.TrimPrefix(raw, "/")
	for i := range len(raw) {
		if !isAlnumByte(raw[i]) {
			raw = raw[:i]
			break
		}
	}
	return strings.ToLower(raw)
}

func isAlnumByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isHeadingTag(name string) bool {
	return len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '6'
}

func isBlockTag(name string) bool {
	switch name {
	case "p", "div", "br", "li", "tr", "blockquote", "section", "article", "hr":
		return true
	default:
		return false
	}
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// decodeEntities resolves the named entities common in book XHTML plus
// numeric character references.
func decodeEntities(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	if strings.Contains(s, "&#") {
		re := regexp.MustCompile(`&#(?:[0-9]+|[xX][0-9a-fA-F]+);`)
		s = re.ReplaceAllStringFunc(s, decodeNumericEntity)
	}
	return strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
		"&nbsp;", " ",
	).Replace(s)
}

func decodeNumericEntity(m string) string {
	body := m[2 : len(m)-1]
	base := 10
	if body[0] == 'x' || body[0] == 'X' {
		base = 16
		body = body[1:]
	}
	n, err := strconv.ParseInt(body, base, 32)
	if err != nil || n < 1 || n > utf8.MaxRune || !utf8.ValidRune(rune(n)) {
		return m
	}
	return string(rune(n))
}
