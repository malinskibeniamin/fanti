package seed

import (
	"bytes"
	"strings"
	"testing"
)

// The fixtures deliberately cover the hazards found in the real exports:
// fields that begin with a double quote (Tatoeba TSV has no quote
// escaping), multiple English links per Mandarin sentence, links whose
// English side is missing from the export, and Mandarin rows with no
// link at all.
const (
	prepareCmnTSV = "1\tcmn\t我們試試看！\n" +
		"2\tcmn\t我该去睡觉了。\n" +
		"3\tcmn\t\"好\"是什麼意思？\n" +
		"4\tcmn\t沒有翻譯的句子。\n" +
		"9\tcmn\t你好。\n"

	prepareLinksTSV = "1\t413789\n" +
		"1\t1039389\n" +
		"2\t1277\n" +
		"3\t500\n" +
		"9\t999999\n"

	// The 413789 text carries a literal tab, which must be flattened or
	// the derivative row would mis-split on re-parse.
	prepareEngTSV = "500\teng\t\"OK,\" she said.\n" +
		"1277\teng\tI have to go to sleep.\n" +
		"413789\teng\tLet's try\tsomething.\n" +
		"1039389\teng\tLet us try something.\n"
)

func TestPrepareTatoeba(t *testing.T) {
	var out bytes.Buffer

	n, err := PrepareTatoeba(
		strings.NewReader(prepareCmnTSV),
		strings.NewReader(prepareLinksTSV),
		strings.NewReader(prepareEngTSV),
		&out)
	if err != nil {
		t.Fatalf("PrepareTatoeba() error = %v", err)
	}

	// Sentence 1's English carried a literal tab; sanitization flattens
	// it back to the plain phrase.
	want := "1\t我們試試看！\tLet's try something.\n" +
		"2\t我该去睡觉了。\tI have to go to sleep.\n" +
		"3\t\"好\"是什麼意思？\t\"OK,\" she said.\n"

	if got := out.String(); got != want {
		t.Errorf("PrepareTatoeba() output =\n%q\nwant\n%q", got, want)
	}

	if n != 3 {
		t.Errorf("PrepareTatoeba() rows = %d, want 3", n)
	}
}

func TestPrepareTatoebaEmpty(t *testing.T) {
	var out bytes.Buffer

	if _, err := PrepareTatoeba(
		strings.NewReader(""), strings.NewReader(""), strings.NewReader(""), &out,
	); err == nil {
		t.Error("PrepareTatoeba() with empty corpus: expected error, got nil")
	}
}
