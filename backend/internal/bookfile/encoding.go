package bookfile

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// commonHanzi lists high-frequency simplified and traditional characters used
// to break ties between candidate decodings: the correct decoding of real
// Chinese prose hits many of these, mojibake hits almost none.
const commonHanzi = "的一是不了人我在有他这为之大来以个中上们到说国和地也子时道出而要于就下得可你" +
	"年生自会那后能对着事其里所去行过家十用发天如然作方成者多日都三小军二无同么经" +
	"這為個們說國時來後對著裡發樣點還沒麼經與內學實現機關開問間題讓從書長話兒女好" +
	"看見聽想知覺得心手頭身回走進出門年月日星期天氣水火山雨風雲花草樹鳥魚馬牛羊"

func decodeText(data []byte) (string, error) {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if utf8.Valid(data) {
		return string(data), nil
	}
	gb, gbErr := decodeBytes(simplifiedchinese.GB18030, data)
	big5, big5Err := decodeBytes(traditionalchinese.Big5, data)
	switch {
	case gbErr == nil && big5Err == nil:
		return pickDecoding(gb, big5), nil
	case gbErr == nil:
		return gb, nil
	case big5Err == nil:
		return big5, nil
	default:
		return "", fmt.Errorf("text is neither UTF-8, GB18030 nor Big5: %w", errMalformed)
	}
}

// pickDecoding prefers the candidate with fewer U+FFFD replacement runes.
// Because almost every Big5 byte pair is also a valid GB18030 pair, the
// replacement counts usually tie; the tie is broken by scoring occurrences of
// high-frequency hanzi.
func pickDecoding(gb, big5 string) string {
	gbBad := strings.Count(gb, "�")
	big5Bad := strings.Count(big5, "�")
	if gbBad != big5Bad {
		if gbBad < big5Bad {
			return gb
		}
		return big5
	}
	if hanziScore(big5) > hanziScore(gb) {
		return big5
	}
	return gb
}

func hanziScore(s string) int {
	score := 0
	for _, r := range s {
		if strings.ContainsRune(commonHanzi, r) {
			score++
		}
	}
	return score
}

func decodeBytes(enc encoding.Encoding, data []byte) (string, error) {
	decoded, err := enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", fmt.Errorf("decode text: %w", err)
	}
	return string(decoded), nil
}
