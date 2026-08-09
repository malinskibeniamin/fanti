package seed

import "testing"

func TestMarkPinyin(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{"ma1", "mā"},
		{"ma2", "má"},
		{"ma3", "mǎ"},
		{"ma4", "mà"},
		{"ma5", "ma"},
		{"lu:4", "lǜ"},
		{"nu:3", "nǚ"},
		{"xiu1", "xiū"},
		{"gui4", "guì"},
		{"hao3", "hǎo"},
		{"xie4", "xiè"},
		{"zhong1", "zhōng"},
		{"Zhong1", "Zhōng"},
		{"er2", "ér"},
		{"r5", "r"},
		{"chuan2 tong3", "chuán tǒng"},
		{"hao3 jiu3 bu4 jian4", "hǎo jiǔ bù jiàn"},
	}

	for _, c := range cases {
		if got := MarkPinyin(c.in); got != c.want {
			t.Errorf("MarkPinyin(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseCEDICTLine(t *testing.T) {
	t.Parallel()

	e, ok := parseCEDICTLine("傳統 传统 [chuan2 tong3] /tradition/convention/traditional/")
	if !ok {
		t.Fatal("expected line to parse")
	}

	if e.Traditional != "傳統" || e.Simplified != "传统" {
		t.Errorf("forms = %q/%q", e.Traditional, e.Simplified)
	}

	if e.Pinyin != "chuán tǒng" {
		t.Errorf("pinyin = %q, want chuán tǒng", e.Pinyin)
	}

	if len(e.Definitions) != 3 || e.Definitions[0] != "tradition" {
		t.Errorf("definitions = %v", e.Definitions)
	}

	if _, ok := parseCEDICTLine("# CC-CEDICT comment"); ok {
		t.Error("comment lines must not parse")
	}

	if _, ok := parseCEDICTLine(""); ok {
		t.Error("blank lines must not parse")
	}
}
