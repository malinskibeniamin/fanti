package server

import (
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

func TestToneOf(t *testing.T) {
	tests := []struct {
		pinyin string
		want   int
	}{
		{"shū", 1},
		{"lóng", 2},
		{"mǎ", 3},
		{"ài", 4},
		{"ma", 0},
		{"", 0},
		{"hǎo ma", 3},
		// Mark priority mirrors the prototype: macron wins over grave.
		{"àā", 1},
	}

	for _, tt := range tests {
		if got := toneOf(tt.pinyin); got != tt.want {
			t.Errorf("toneOf(%q) = %d, want %d", tt.pinyin, got, tt.want)
		}
	}
}

func testRand(seed uint64) *rand.Rand {
	//nolint:gosec // deterministic seeding is the point of these tests
	return rand.New(rand.NewPCG(seed, seed))
}

func testDeck() []quizEntry {
	deck := []quizEntry{
		{Character: "馬", Simplified: "马", Pinyin: "mǎ", Meaning: "horse"},
		{Character: "愛", Simplified: "爱", Pinyin: "ài", Meaning: "to love"},
		{Character: "書", Simplified: "书", Pinyin: "shū", Meaning: "book"},
		{Character: "龍", Simplified: "龙", Pinyin: "lóng", Meaning: "dragon"},
		{Character: "髮", Simplified: "发", Pinyin: "fà", Meaning: "hair"},
		// Same in both scripts — script/match must fall back for it.
		{Character: "山", Simplified: "山", Pinyin: "shān", Meaning: "mountain"},
		// No tone mark — a tone question must fall back to listen.
		{Character: "嗎", Simplified: "吗", Pinyin: "ma", Meaning: "question particle"},
		{Character: "火", Simplified: "火", Pinyin: "huǒ", Meaning: "fire"},
		{Character: "水", Simplified: "水", Pinyin: "shuǐ", Meaning: "water"},
	}
	for index := range deck {
		deck[index].HasStrokes = true
	}

	return deck
}

func entryByChar(t *testing.T, deck []quizEntry, ch string) quizEntry {
	t.Helper()

	for _, e := range deck {
		if e.Character == ch {
			return e
		}
	}

	t.Fatalf("entry %q not in deck", ch)

	return quizEntry{}
}

func TestBuildQuizComposition(t *testing.T) {
	deck := testDeck()

	for seed := uint64(1); seed <= 20; seed++ {
		questions := buildQuiz(testRand(seed), deck, deck, nil, nil)

		if len(questions) != 8 {
			t.Fatalf("seed %d: %d questions, want 8", seed, len(questions))
		}

		for i, q := range questions {
			e := entryByChar(t, deck, q.Character)

			switch q.Type {
			case qtScript, qtMatch:
				if e.Character == e.Simplified {
					t.Errorf("seed %d q%d: %s question on %s, which is identical in both scripts",
						seed, i, q.Type, q.Character)
				}
			case qtTone:
				if toneOf(e.Pinyin) == 0 {
					t.Errorf("seed %d q%d: tone question on toneless %q", seed, i, e.Pinyin)
				}

				if q.Answer != toneOf(e.Pinyin)-1 || len(q.Options) != 4 {
					t.Errorf("seed %d q%d: tone answer %d options %d", seed, i, q.Answer, len(q.Options))
				}
			case qtWrite, qtType:
				if q.Answer != -1 || len(q.Options) != 0 {
					t.Errorf("seed %d q%d: %s must have no options, got answer %d options %v",
						seed, i, q.Type, q.Answer, q.Options)
				}
			}

			assertAnswerConsistent(t, q, e)
		}
	}
}

func TestBuildQuizSkipsQuestionsUnsupportedByReferenceData(t *testing.T) {
	reference := quizEntry{
		Character:  "㐀",
		Simplified: "㐀",
		Pinyin:     "qiū",
	}
	deck := append(testDeck(), reference)

	for seed := uint64(1); seed <= 100; seed++ {
		questions := buildQuiz(testRand(seed), deck, deck, nil, nil)

		for _, question := range questions {
			if question.Character != reference.Character {
				continue
			}

			switch question.Type {
			case qtMeaning, qtWrite, qtScript, qtMatch:
				t.Fatalf("seed %d: reference character received unsupported %s question",
					seed, question.Type)
			}
		}
	}
}

// assertAnswerConsistent checks the stored answer indexes the label of the
// character under test.
func assertAnswerConsistent(t *testing.T, q quizQuestionRow, e quizEntry) {
	t.Helper()

	if q.Answer < 0 {
		return
	}

	if q.Answer >= len(q.Options) {
		t.Fatalf("answer %d out of range of %d options", q.Answer, len(q.Options))
	}

	got := q.Options[q.Answer]

	switch q.Type {
	case qtReading:
		if got != e.Pinyin {
			t.Errorf("reading answer = %q, want %q", got, e.Pinyin)
		}
	case qtMeaning:
		if got != e.Meaning {
			t.Errorf("meaning answer = %q, want %q", got, e.Meaning)
		}
	case qtMatch:
		if got != e.Simplified {
			t.Errorf("match answer = %q, want %q", got, e.Simplified)
		}
	case qtListen, qtCloze:
		if got != e.Character {
			t.Errorf("%s answer = %q, want %q", q.Type, got, e.Character)
		}
	case qtScript:
		if (q.Answer == 0) != (q.Prompt == e.Simplified) {
			t.Errorf("script answer %d does not match prompt %q", q.Answer, q.Prompt)
		}
	}
}

func TestBuildQuizIsDeterministicPerSeed(t *testing.T) {
	deck := testDeck()
	cloze := []clozeEntry{
		{Entry: entryByChar(t, deck, "馬"), Sentence: "他騎馬上學。"},
	}

	a := buildQuiz(testRand(7), deck, deck, cloze, nil)
	b := buildQuiz(testRand(7), deck, deck, cloze, nil)

	if !reflect.DeepEqual(a, b) {
		t.Error("same seed produced different quizzes")
	}

	c := buildQuiz(testRand(8), deck, deck, cloze, nil)
	if reflect.DeepEqual(a, c) {
		t.Error("different seeds produced identical quizzes (suspicious)")
	}
}

func TestBuildQuizClozeReplacesTail(t *testing.T) {
	deck := testDeck()
	cloze := []clozeEntry{
		{Entry: entryByChar(t, deck, "馬"), Sentence: "他騎馬上學。"},
		{Entry: entryByChar(t, deck, "書"), Sentence: "我在看書。"},
		{Entry: entryByChar(t, deck, "火"), Sentence: "山上有火。"},
	}

	questions := buildQuiz(testRand(3), deck, deck, cloze, nil)

	if len(questions) != 8 {
		t.Fatalf("%d questions, want 8", len(questions))
	}

	for i, q := range questions[:6] {
		if q.Type == qtCloze {
			t.Errorf("q%d is cloze; cloze must only replace the tail", i)
		}
	}

	for i, q := range questions[6:] {
		if q.Type != qtCloze {
			t.Fatalf("tail q%d = %s, want cloze", 6+i, q.Type)
		}

		if !strings.Contains(q.Prompt, "＿") {
			t.Errorf("cloze prompt %q lacks the blank", q.Prompt)
		}

		if strings.Contains(q.Prompt, q.Character) {
			t.Errorf("cloze prompt %q still contains %q", q.Prompt, q.Character)
		}
	}
}

func TestBuildQuizWeakPoolFallsBackToDeck(t *testing.T) {
	deck := testDeck()

	questions := buildQuiz(testRand(5), nil, deck, nil, nil)
	if len(questions) != 8 {
		t.Errorf("%d questions, want 8 from the deck fallback", len(questions))
	}
}

func TestBuildLessonQuiz(t *testing.T) {
	deck := testDeck()
	lesson := entryByChar(t, deck, "龍")

	questions := buildQuiz(testRand(9), nil, deck, nil, &lesson)

	if len(questions) != 4 {
		t.Fatalf("%d questions, want 4", len(questions))
	}

	wantTypes := []string{qtReading, qtType, qtReading, qtMeaning}
	for i, q := range questions {
		if q.Type != wantTypes[i] {
			t.Errorf("q%d type = %s, want %s", i, q.Type, wantTypes[i])
		}
	}

	if questions[0].Character != "龍" || questions[1].Character != "龍" {
		t.Errorf("first two questions must drill the lesson character, got %q %q",
			questions[0].Character, questions[1].Character)
	}

	for i, q := range questions[2:] {
		if q.Character == "龍" {
			t.Errorf("interleaved q%d reuses the lesson character", 2+i)
		}
	}
}

func TestGradeAnswerTypeQuestion(t *testing.T) {
	q := quizQuestionRow{Type: qtType, Character: "髮", Simplified: "发", Answer: -1}

	tests := []struct {
		typed   string
		correct bool
	}{
		{"髮", true},
		// IME leniency: the simplified form of the same character counts.
		{"发", true},
		{" 髮 ", true},
		// A sibling that shares the simplified form does not.
		{"發", false},
		{"", false},
	}

	for _, tt := range tests {
		correct, given, err := gradeAnswer(q, &fantiv1.SubmitQuizAnswerRequest{
			Answer: &fantiv1.SubmitQuizAnswerRequest_TypedText{TypedText: tt.typed},
		})
		if err != nil {
			t.Fatalf("gradeAnswer(%q): %v", tt.typed, err)
		}

		if correct != tt.correct {
			t.Errorf("gradeAnswer(%q) = %v, want %v", tt.typed, correct, tt.correct)
		}

		if want := strings.TrimSpace(tt.typed); given != want {
			t.Errorf("given = %q, want %q", given, want)
		}
	}
}

func TestGradeAnswerShapeMismatch(t *testing.T) {
	q := quizQuestionRow{Type: qtReading, Character: "馬", Options: []string{"mǎ", "ài"}, Answer: 0}

	if _, _, err := gradeAnswer(q, &fantiv1.SubmitQuizAnswerRequest{
		Answer: &fantiv1.SubmitQuizAnswerRequest_SelfCorrect{SelfCorrect: true},
	}); err == nil {
		t.Error("self_correct on a reading question must be rejected")
	}

	correct, given, err := gradeAnswer(q, &fantiv1.SubmitQuizAnswerRequest{
		Answer: &fantiv1.SubmitQuizAnswerRequest_OptionIndex{OptionIndex: 1},
	})
	if err != nil || correct || given != "ài" {
		t.Errorf("wrong option = (%v, %q, %v), want (false, ài, nil)", correct, given, err)
	}
}
