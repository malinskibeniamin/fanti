// Package seed loads dictionary data and authored content into Postgres.
package seed

import "strings"

//nolint:gochecknoglobals // static tone-mark lookup tables
var toneMarks = map[rune][5]rune{
	'a': {'a', 'ā', 'á', 'ǎ', 'à'},
	'e': {'e', 'ē', 'é', 'ě', 'è'},
	'i': {'i', 'ī', 'í', 'ǐ', 'ì'},
	'o': {'o', 'ō', 'ó', 'ǒ', 'ò'},
	'u': {'u', 'ū', 'ú', 'ǔ', 'ù'},
	'ü': {'ü', 'ǖ', 'ǘ', 'ǚ', 'ǜ'},
	'A': {'A', 'Ā', 'Á', 'Ǎ', 'À'},
	'E': {'E', 'Ē', 'É', 'Ě', 'È'},
	'I': {'I', 'Ī', 'Í', 'Ǐ', 'Ì'},
	'O': {'O', 'Ō', 'Ó', 'Ǒ', 'Ò'},
	'U': {'U', 'Ū', 'Ú', 'Ǔ', 'Ù'},
	'Ü': {'Ü', 'Ǖ', 'Ǘ', 'Ǚ', 'Ǜ'},
}

// MarkPinyin converts CEDICT numbered pinyin ("chuan2 tong3") to
// diacritic form ("chuán tǒng"). Unknown syllables pass through unchanged.
func MarkPinyin(numbered string) string {
	parts := strings.Fields(numbered)
	for i, p := range parts {
		parts[i] = markSyllable(p)
	}

	return strings.Join(parts, " ")
}

func markSyllable(syl string) string {
	if syl == "" {
		return syl
	}

	tone := 0

	last := syl[len(syl)-1]
	if last >= '1' && last <= '5' {
		tone = int(last - '0')
		syl = syl[:len(syl)-1]
	}

	// CEDICT writes ü as "u:".
	syl = strings.ReplaceAll(syl, "u:", "ü")
	syl = strings.ReplaceAll(syl, "U:", "Ü")

	if tone == 0 || tone == 5 {
		return syl
	}

	runes := []rune(syl)

	idx := markIndex(runes)
	if idx < 0 {
		return syl
	}

	if marks, ok := toneMarks[runes[idx]]; ok {
		runes[idx] = marks[tone]
	}

	return string(runes)
}

// markIndex picks which vowel carries the tone mark:
// 'a' if present, else 'e', else the 'o' of "ou", else the last vowel.
func markIndex(runes []rune) int {
	lastVowel := -1

	for i, r := range runes {
		switch r {
		case 'a', 'A':
			return i
		case 'e', 'E':
			return i
		case 'o', 'O':
			if i+1 < len(runes) && (runes[i+1] == 'u' || runes[i+1] == 'U') {
				return i
			}

			lastVowel = i
		case 'i', 'I', 'u', 'U', 'ü', 'Ü':
			lastVowel = i
		}
	}

	return lastVowel
}
