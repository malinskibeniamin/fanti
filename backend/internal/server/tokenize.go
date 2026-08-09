package server

import (
	"context"
	"strings"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

// sentenceEnders close a sentence span for cloze mining.
const sentenceEnders = "。！？"

// tokenizer annotates reader text with pinyin and character-page links.
type tokenizer struct {
	pinyin   map[string]string
	tappable map[string]bool
}

// newTokenizer loads annotation data for every distinct character across
// the given paragraphs.
func newTokenizer(ctx context.Context, pool *pgxpool.Pool, paragraphs ...[]string) (*tokenizer, error) {
	seen := map[string]bool{}

	var chars []string

	for _, paras := range paragraphs {
		for _, p := range paras {
			for _, r := range p {
				if !unicode.Is(unicode.Han, r) || seen[string(r)] {
					continue
				}

				seen[string(r)] = true
				chars = append(chars, string(r))
			}
		}
	}

	t := &tokenizer{pinyin: map[string]string{}, tappable: map[string]bool{}}

	if len(chars) == 0 {
		return t, nil
	}

	rows, err := pool.Query(ctx,
		"SELECT ch, pinyin FROM char_pinyin WHERE ch = ANY($1)", chars)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ch, py string
		if err := rows.Scan(&ch, &py); err != nil {
			return nil, err
		}

		t.pinyin[ch] = py
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Tappable: curated characters, plus any single-character CEDICT entry
	// (GetCharacter synthesizes a page for those).
	tapRows, err := pool.Query(ctx, `
		SELECT traditional FROM characters WHERE traditional = ANY($1)
		UNION
		SELECT traditional FROM dict_entries
		WHERE char_length(traditional) = 1 AND traditional = ANY($1)
		UNION
		SELECT simplified FROM dict_entries
		WHERE char_length(simplified) = 1 AND simplified = ANY($1)`, chars)
	if err != nil {
		return nil, err
	}
	defer tapRows.Close()

	for tapRows.Next() {
		var ch string
		if err := tapRows.Scan(&ch); err != nil {
			return nil, err
		}

		t.tappable[ch] = true
	}

	return t, tapRows.Err()
}

// paragraph tokenizes one paragraph with sentence spans.
func (t *tokenizer) paragraph(text string) *fantiv1.Paragraph {
	para := &fantiv1.Paragraph{}
	runes := []rune(text)
	sentenceStart := 0

	for i, r := range runes {
		ch := string(r)
		token := &fantiv1.Token{Text: ch, Pinyin: t.pinyin[ch]}

		if t.tappable[ch] {
			token.Character = "characters/" + ch
		}

		para.Tokens = append(para.Tokens, token)

		if strings.ContainsRune(sentenceEnders, r) {
			para.Sentences = append(para.Sentences, &fantiv1.SentenceSpan{
				Start: int32(sentenceStart), End: int32(i + 1),
			})
			sentenceStart = i + 1
		}
	}

	if sentenceStart < len(runes) {
		para.Sentences = append(para.Sentences, &fantiv1.SentenceSpan{
			//nolint:gosec // paragraph lengths are far below int32 range
			Start: int32(sentenceStart), End: int32(len(runes)),
		})
	}

	return para
}
