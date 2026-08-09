package server_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
	"github.com/malinskibeniamin/fanti/backend/gen/fanti/v1/fantiv1connect"
	"github.com/malinskibeniamin/fanti/backend/internal/seed"
	"github.com/malinskibeniamin/fanti/backend/internal/server"
	"github.com/malinskibeniamin/fanti/backend/internal/testdb"
)

// storedQuestion mirrors the quizzes.questions row shape so the test can
// read the server-side answers straight from the database.
type storedQuestion struct {
	Type      string   `json:"type"`
	Prompt    string   `json:"prompt"`
	Character string   `json:"character"`
	Options   []string `json:"options"`
	Answer    int32    `json:"answer"`
}

const (
	// profileResource is the StudyProfile singleton name.
	profileResource = "studyProfile"
	// typedQuestion is the stored name of IME-typing questions.
	typedQuestion = "type"
	// practiceReviewResource is isolated from cards graded later in the flow.
	practiceReviewResource = "reviews/勢"
)

func TestIntegrationStudyFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}

	pool := testdb.StartMigrated(t)
	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	if err := seed.Run(ctx, pool, seed.Sources{}, logger); err != nil {
		t.Fatalf("seed: %v", err)
	}

	handler, err := server.NewHandler(pool, logger)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	study := fantiv1connect.NewStudyServiceClient(http.DefaultClient, srv.URL)
	tutor := fantiv1connect.NewTutorServiceClient(http.DefaultClient, srv.URL)

	storedQuestions := func(t *testing.T, name string) []storedQuestion {
		t.Helper()

		id := strings.TrimPrefix(name, "quizzes/")

		var raw []byte
		if err := pool.QueryRow(ctx,
			"SELECT questions FROM quizzes WHERE id = $1", id).Scan(&raw); err != nil {
			t.Fatalf("load quiz %s: %v", id, err)
		}

		var questions []storedQuestion
		if err := json.Unmarshal(raw, &questions); err != nil {
			t.Fatalf("decode questions: %v", err)
		}

		return questions
	}

	answerReq := func(name string, index int32, q storedQuestion, right bool) *fantiv1.SubmitQuizAnswerRequest {
		req := &fantiv1.SubmitQuizAnswerRequest{Name: name, QuestionIndex: index}

		switch {
		case q.Type == "write":
			req.Answer = &fantiv1.SubmitQuizAnswerRequest_SelfCorrect{SelfCorrect: right}
		case q.Type == typedQuestion && right:
			req.Answer = &fantiv1.SubmitQuizAnswerRequest_TypedText{TypedText: q.Character}
		case q.Type == typedQuestion:
			req.Answer = &fantiv1.SubmitQuizAnswerRequest_TypedText{TypedText: "錯"}
		case right:
			req.Answer = &fantiv1.SubmitQuizAnswerRequest_OptionIndex{OptionIndex: q.Answer}
		default:
			req.Answer = &fantiv1.SubmitQuizAnswerRequest_OptionIndex{
				OptionIndex: (q.Answer + 1) % int32(len(q.Options)), //nolint:gosec // at most four options
			}
		}

		return req
	}

	t.Run("ListDueCardsStarterDeck", func(t *testing.T) {
		resp, err := study.ListDueCards(ctx, connect.NewRequest(&fantiv1.ListDueCardsRequest{}))
		if err != nil {
			t.Fatalf("ListDueCards: %v", err)
		}

		if len(resp.Msg.GetDueCards()) != 5 || resp.Msg.GetDueCount() != 5 {
			t.Fatalf("cards = %d due = %d, want 5/5",
				len(resp.Msg.GetDueCards()), resp.Msg.GetDueCount())
		}

		// Never-reviewed cards sort first by frequency: 書 (300) leads.
		first := resp.Msg.GetDueCards()[0]
		if first.GetCharacter().GetTraditional() != "書" {
			t.Errorf("first card = %s, want 書", first.GetCharacter().GetTraditional())
		}

		if first.GetCharacter().GetPinyin() == "" || len(first.GetCharacter().GetExamples()) == 0 {
			t.Error("due card lacks full character content")
		}
		if first.GetCharacter().GetEntryCapabilities().GetReading() !=
			fantiv1.CapabilityStatus_CAPABILITY_STATUS_AVAILABLE {
			t.Error("due card lacks entry capability metadata")
		}
		if len(first.GetCharacter().GetGlyphs()) == 0 ||
			first.GetCharacter().GetGlyphs()[0].GetCapabilities().GetStrokes() ==
				fantiv1.CapabilityStatus_CAPABILITY_STATUS_UNSPECIFIED {
			t.Error("due card lacks glyph capability metadata")
		}

		if first.GetReview().GetName() != "reviews/書" || first.GetReview().GetDueTime() != nil {
			t.Errorf("unseen review = %v, want reviews/書 with no due time", first.GetReview())
		}
		if first.GetReview().GetPracticeDifficulty() !=
			fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED {
			t.Errorf("starter difficulty = %s, want guided",
				first.GetReview().GetPracticeDifficulty())
		}
	})

	t.Run("PracticeDifficultyDefaultsAndUpdatesPerCharacter", func(t *testing.T) {
		got, err := study.GetReview(ctx, connect.NewRequest(&fantiv1.GetReviewRequest{
			Name: practiceReviewResource,
		}))
		if err != nil {
			t.Fatalf("GetReview default: %v", err)
		}
		if got.Msg.GetPracticeDifficulty() != fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED {
			t.Errorf("default difficulty = %s, want guided", got.Msg.GetPracticeDifficulty())
		}

		updated, err := study.UpdateReview(ctx, connect.NewRequest(&fantiv1.UpdateReviewRequest{
			Review: &fantiv1.Review{
				Name:               practiceReviewResource,
				PracticeDifficulty: fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_MASTERY,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"practice_difficulty"}},
		}))
		if err != nil {
			t.Fatalf("UpdateReview: %v", err)
		}
		if updated.Msg.GetPracticeDifficulty() != fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_MASTERY {
			t.Errorf("updated difficulty = %s, want mastery", updated.Msg.GetPracticeDifficulty())
		}

		persisted, err := study.GetReview(ctx, connect.NewRequest(&fantiv1.GetReviewRequest{
			Name: practiceReviewResource,
		}))
		if err != nil {
			t.Fatalf("GetReview persisted: %v", err)
		}
		if persisted.Msg.GetPracticeDifficulty() != fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_MASTERY {
			t.Errorf("persisted difficulty = %s, want mastery", persisted.Msg.GetPracticeDifficulty())
		}
	})

	t.Run("ListDueCardsWordMode", func(t *testing.T) {
		resp, err := study.ListDueCards(ctx, connect.NewRequest(&fantiv1.ListDueCardsRequest{
			Mode: fantiv1.CardMode_CARD_MODE_WORD,
		}))
		if err != nil {
			t.Fatalf("ListDueCards words: %v", err)
		}

		if len(resp.Msg.GetDueCards()) != 8 || resp.Msg.GetDueCount() != 8 {
			t.Fatalf("cards = %d due = %d, want 8/8",
				len(resp.Msg.GetDueCards()), resp.Msg.GetDueCount())
		}

		for _, card := range resp.Msg.GetDueCards() {
			if card.GetCharacter().GetTraditional() == "" || card.GetCharacter().GetMeaning() == "" {
				t.Errorf("word card %v lacks synthesized content", card.GetCharacter())
			}
		}
	})

	t.Run("ListDueCardsExcludesFutureReviews", func(t *testing.T) {
		future := time.Now().Add(24 * time.Hour)
		for _, card := range []string{"書", "你好"} {
			if _, err := pool.Exec(ctx, `
				INSERT INTO reviews (ch, due_time, in_deck)
				VALUES ($1, $2, TRUE)
				ON CONFLICT (ch) DO UPDATE SET due_time = EXCLUDED.due_time`, card, future); err != nil {
				t.Fatalf("set future review for %s: %v", card, err)
			}
		}

		defer func() {
			if _, err := pool.Exec(ctx, "DELETE FROM reviews WHERE ch = ANY($1)", []string{"書", "你好"}); err != nil {
				t.Errorf("delete future reviews: %v", err)
			}
		}()

		for _, tc := range []struct {
			name string
			mode fantiv1.CardMode
			want int
		}{
			{name: "character", want: 4},
			{name: "word", mode: fantiv1.CardMode_CARD_MODE_WORD, want: 7},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp, err := study.ListDueCards(ctx, connect.NewRequest(&fantiv1.ListDueCardsRequest{
					Mode: tc.mode,
				}))
				if err != nil {
					t.Fatalf("ListDueCards: %v", err)
				}

				if len(resp.Msg.GetDueCards()) != tc.want || int(resp.Msg.GetDueCount()) != tc.want {
					t.Errorf("cards/due = %d/%d, want %d/%d",
						len(resp.Msg.GetDueCards()), resp.Msg.GetDueCount(), tc.want, tc.want)
				}
			})
		}
	})

	t.Run("GradeCardGood", func(t *testing.T) {
		resp, err := study.GradeCard(ctx, connect.NewRequest(&fantiv1.GradeCardRequest{
			Name:  "reviews/馬",
			Grade: fantiv1.Grade_GRADE_GOOD,
		}))
		if err != nil {
			t.Fatalf("GradeCard: %v", err)
		}

		review := resp.Msg
		if review.GetSrsLevel() != 0 || !review.GetLearned() {
			t.Errorf("review = level %d learned %v, want 0/true",
				review.GetSrsLevel(), review.GetLearned())
		}

		due := review.GetDueTime().AsTime()
		if d := time.Until(due); d < 23*time.Hour || d > 25*time.Hour {
			t.Errorf("due in %v, want about one day", d)
		}

		if _, err := study.GradeCard(ctx, connect.NewRequest(&fantiv1.GradeCardRequest{
			Name: "reviews/鬱", Grade: fantiv1.Grade_GRADE_GOOD,
		})); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("unknown card code = %v, want NotFound", connect.CodeOf(err))
		}

		if _, err := study.GradeCard(ctx, connect.NewRequest(&fantiv1.GradeCardRequest{
			Name: "reviews/馬",
		})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("missing grade code = %v, want InvalidArgument", connect.CodeOf(err))
		}
	})

	t.Run("ProfileAfterFirstLearn", func(t *testing.T) {
		resp, err := study.GetStudyProfile(ctx, connect.NewRequest(&fantiv1.GetStudyProfileRequest{
			Name: profileResource,
		}))
		if err != nil {
			t.Fatalf("GetStudyProfile: %v", err)
		}

		profile := resp.Msg
		if profile.GetLearnedCount() != 1 || profile.GetCourseSize() != 3000 {
			t.Errorf("learned = %d course = %d, want 1/3000",
				profile.GetLearnedCount(), profile.GetCourseSize())
		}

		curriculum := profile.GetCurriculumProgress()
		if curriculum.GetCoreLearned() != 1 || curriculum.GetCompleteLearned() != 1 {
			t.Errorf("curriculum learned = core %d complete %d, want 1/1",
				curriculum.GetCoreLearned(), curriculum.GetCompleteLearned())
		}
		if curriculum.GetCoreSize() == 0 ||
			curriculum.GetCoreSize() > 3000 ||
			curriculum.GetCompleteSize() < curriculum.GetCoreSize() {
			t.Errorf("curriculum sizes = core %d complete %d, want 1..3000 and no smaller complete path",
				curriculum.GetCoreSize(), curriculum.GetCompleteSize())
		}
		if curriculum.GetReferenceLearned() != 0 {
			t.Errorf("reference progress = %d/%d, want none learned",
				curriculum.GetReferenceLearned(), curriculum.GetReferenceSize())
		}

		if want := 0.5 / 140; profile.GetCoverage() < want-1e-9 || profile.GetCoverage() > want+1e-9 {
			t.Errorf("coverage = %v, want %v", profile.GetCoverage(), want)
		}

		var today string
		if err := pool.QueryRow(ctx,
			"SELECT to_char(CURRENT_DATE, 'YYYY-MM-DD')").Scan(&today); err != nil {
			t.Fatalf("current date: %v", err)
		}

		if !slices.Contains(profile.GetPracticeDays(), today) {
			t.Errorf("practice days %v missing today %s", profile.GetPracticeDays(), today)
		}

		if len(profile.GetRecords()) != 1 || profile.GetRecords()[0].GetType() != "learned" ||
			profile.GetRecords()[0].GetCharacter() != "馬" {
			t.Errorf("records = %v, want one learned 馬", profile.GetRecords())
		}

		if len(profile.GetMilestones()) != 8 || profile.GetMilestones()[0].GetLabel().GetEn() == "" {
			t.Errorf("milestones = %d entries, want 8 localized", len(profile.GetMilestones()))
		}

		if len(profile.GetExamReadiness()) == 0 {
			t.Fatal("exam readiness missing")
		}

		hsk1 := profile.GetExamReadiness()[0]
		if !strings.HasPrefix(hsk1.GetLevel(), "HSK 1") || !strings.Contains(hsk1.GetLevel(), "A1") {
			t.Errorf("first level = %q, want HSK 1 · A1", hsk1.GetLevel())
		}

		if hsk1.GetProgress() <= 0 || hsk1.GetProgress() > 1 {
			t.Errorf("HSK 1 progress = %v with 馬 learned", hsk1.GetProgress())
		}
	})

	t.Run("ReferenceLearningStaysSeparateFromCurriculumProgress", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO characters (
				traditional, simplified, pinyin, meaning,
				catalog_kind, curriculum_rank
			) VALUES ('㐀', '㐀', 'qiū', '', 'reference', 0)`); err != nil {
			t.Fatalf("insert reference character: %v", err)
		}
		defer func() {
			if _, err := pool.Exec(ctx, `
				DELETE FROM learning_records WHERE ch = '㐀';
				DELETE FROM reviews WHERE ch = '㐀';
				DELETE FROM characters WHERE traditional = '㐀'`); err != nil {
				t.Errorf("delete reference study fixture: %v", err)
			}
		}()

		if _, err := study.MarkLearned(ctx, connect.NewRequest(&fantiv1.MarkLearnedRequest{
			Character: "characters/㐀",
		})); err != nil {
			t.Fatalf("MarkLearned reference: %v", err)
		}

		resp, err := study.GetStudyProfile(ctx, connect.NewRequest(&fantiv1.GetStudyProfileRequest{
			Name: profileResource,
		}))
		if err != nil {
			t.Fatalf("GetStudyProfile: %v", err)
		}

		profile := resp.Msg
		if profile.GetLearnedCount() != 1 {
			t.Errorf("curriculum learned count = %d, want 1", profile.GetLearnedCount())
		}
		progress := profile.GetCurriculumProgress()
		if progress.GetCoreLearned() != 1 ||
			progress.GetCompleteLearned() != 1 ||
			progress.GetReferenceLearned() != 1 {
			t.Errorf("separate progress = core %d complete %d reference %d, want 1/1/1",
				progress.GetCoreLearned(),
				progress.GetCompleteLearned(),
				progress.GetReferenceLearned())
		}
	})

	t.Run("MilestoneAtFiveLearned", func(t *testing.T) {
		for _, ch := range []string{"愛", "書", "龍", "髮"} {
			if _, err := study.MarkLearned(ctx, connect.NewRequest(&fantiv1.MarkLearnedRequest{
				Character: "characters/" + ch,
			})); err != nil {
				t.Fatalf("MarkLearned %s: %v", ch, err)
			}
		}

		// Marking again must not duplicate records or milestones.
		if _, err := study.MarkLearned(ctx, connect.NewRequest(&fantiv1.MarkLearnedRequest{
			Character: "characters/愛",
		})); err != nil {
			t.Fatalf("MarkLearned again: %v", err)
		}

		resp, err := study.GetStudyProfile(ctx, connect.NewRequest(&fantiv1.GetStudyProfileRequest{
			Name: profileResource,
		}))
		if err != nil {
			t.Fatalf("GetStudyProfile: %v", err)
		}

		profile := resp.Msg
		if profile.GetLearnedCount() != 5 {
			t.Fatalf("learned = %d, want 5", profile.GetLearnedCount())
		}

		milestones := 0

		for _, r := range profile.GetRecords() {
			if r.GetType() == "milestone" {
				milestones++

				if r.GetMilestoneThreshold() != 5 {
					t.Errorf("milestone threshold = %d, want 5", r.GetMilestoneThreshold())
				}
			}
		}

		if milestones != 1 {
			t.Errorf("milestone records = %d, want exactly 1", milestones)
		}

		if m := profile.GetMilestones()[0]; m.GetThreshold() != 5 || !m.GetReached() {
			t.Errorf("milestone 5 = %v, want reached", m)
		}

		if m := profile.GetMilestones()[1]; m.GetReached() {
			t.Errorf("milestone %d reached with only 5 learned", m.GetThreshold())
		}
	})

	t.Run("AddToDeck", func(t *testing.T) {
		review, err := study.AddToDeck(ctx, connect.NewRequest(&fantiv1.AddToDeckRequest{
			Character: "characters/秦",
		}))
		if err != nil {
			t.Fatalf("AddToDeck: %v", err)
		}

		if review.Msg.GetLearned() {
			t.Error("AddToDeck must not mark learned")
		}

		resp, err := study.ListDueCards(ctx, connect.NewRequest(&fantiv1.ListDueCardsRequest{}))
		if err != nil {
			t.Fatalf("ListDueCards: %v", err)
		}

		if len(resp.Msg.GetDueCards()) != 5 {
			t.Errorf("due = %d cards after AddToDeck, want 5", len(resp.Msg.GetDueCards()))
		}
	})

	var wrongChar string

	t.Run("QuizWrongThenRight", func(t *testing.T) {
		created, err := study.CreateQuiz(ctx, connect.NewRequest(&fantiv1.CreateQuizRequest{
			Pool: fantiv1.QuizPool_QUIZ_POOL_ALL,
		}))
		if err != nil {
			t.Fatalf("CreateQuiz: %v", err)
		}

		quiz := created.Msg
		if len(quiz.GetQuestions()) != 6 {
			t.Fatalf("questions = %d, want 6 (deck size)", len(quiz.GetQuestions()))
		}

		questions := storedQuestions(t, quiz.GetName())
		wrongChar = questions[0].Character

		wrong, err := study.SubmitQuizAnswer(ctx,
			connect.NewRequest(answerReq(quiz.GetName(), 0, questions[0], false)))
		if err != nil {
			t.Fatalf("SubmitQuizAnswer wrong: %v", err)
		}

		if wrong.Msg.GetCorrect() {
			t.Fatal("deliberately wrong answer graded correct")
		}

		if wrong.Msg.GetFeedback().GetEn() == "" || wrong.Msg.GetFeedback().GetTc() == "" ||
			wrong.Msg.GetFeedback().GetSc() == "" {
			t.Errorf("feedback missing locales: %v", wrong.Msg.GetFeedback())
		}

		if wrong.Msg.GetCorrectAnswer() == "" {
			t.Error("correct_answer missing on a miss")
		}

		if !slices.Contains(wrong.Msg.GetQuiz().GetMistakes(), wrongChar) {
			t.Errorf("mistakes %v missing %s", wrong.Msg.GetQuiz().GetMistakes(), wrongChar)
		}

		var mistakes int
		if err := pool.QueryRow(ctx,
			"SELECT mistake_count FROM reviews WHERE ch = $1", wrongChar).Scan(&mistakes); err != nil {
			t.Fatalf("mistake count: %v", err)
		}

		if mistakes < 1 {
			t.Errorf("mistake_count = %d, want at least 1", mistakes)
		}

		// Answer the rest correctly.
		var last *fantiv1.SubmitQuizAnswerResponse

		count := int32(len(questions)) //nolint:gosec // a handful of questions

		for i := int32(1); i < count; i++ {
			resp, err := study.SubmitQuizAnswer(ctx,
				connect.NewRequest(answerReq(quiz.GetName(), i, questions[i], true)))
			if err != nil {
				t.Fatalf("SubmitQuizAnswer %d: %v", i, err)
			}

			if !resp.Msg.GetCorrect() {
				t.Fatalf("question %d (%s) graded wrong for the stored answer",
					i, questions[i].Type)
			}

			if resp.Msg.GetFeedback().GetEn() != "" {
				t.Errorf("question %d has feedback despite being correct", i)
			}

			last = resp.Msg
		}

		if !last.GetQuiz().GetFinished() || last.GetQuiz().GetScore() != count-1 {
			t.Errorf("finished = %v score = %d, want true/%d",
				last.GetQuiz().GetFinished(), last.GetQuiz().GetScore(), count-1)
		}

		if _, err := study.SubmitQuizAnswer(ctx,
			connect.NewRequest(answerReq(quiz.GetName(), count, questions[0], true)),
		); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("finished quiz code = %v, want FailedPrecondition", connect.CodeOf(err))
		}
	})

	t.Run("WeakQuizDrawsFromMistakes", func(t *testing.T) {
		created, err := study.CreateQuiz(ctx, connect.NewRequest(&fantiv1.CreateQuizRequest{
			Pool: fantiv1.QuizPool_QUIZ_POOL_WEAK,
		}))
		if err != nil {
			t.Fatalf("CreateQuiz weak: %v", err)
		}

		for _, q := range created.Msg.GetQuestions() {
			if q.GetCharacter() != wrongChar {
				t.Errorf("weak quiz question on %s, want only %s", q.GetCharacter(), wrongChar)
			}
		}
	})

	t.Run("ClozeCardsJoinTheQuiz", func(t *testing.T) {
		first, err := study.CreateClozeCard(ctx, connect.NewRequest(&fantiv1.CreateClozeCardRequest{
			ClozeCard: &fantiv1.ClozeCard{Character: "馬", Sentence: "他騎馬上學。"},
		}))
		if err != nil {
			t.Fatalf("CreateClozeCard: %v", err)
		}

		// Idempotent on the (character, sentence) pair.
		again, err := study.CreateClozeCard(ctx, connect.NewRequest(&fantiv1.CreateClozeCardRequest{
			ClozeCard: &fantiv1.ClozeCard{Character: "馬", Sentence: "他騎馬上學。"},
		}))
		if err != nil {
			t.Fatalf("CreateClozeCard again: %v", err)
		}

		if first.Msg.GetName() != again.Msg.GetName() {
			t.Errorf("duplicate cloze card renamed: %s vs %s",
				first.Msg.GetName(), again.Msg.GetName())
		}

		if _, err := study.CreateClozeCard(ctx, connect.NewRequest(&fantiv1.CreateClozeCardRequest{
			ClozeCard: &fantiv1.ClozeCard{Character: "書", Sentence: "我在看書。"},
		})); err != nil {
			t.Fatalf("CreateClozeCard second: %v", err)
		}

		if _, err := study.CreateClozeCard(ctx, connect.NewRequest(&fantiv1.CreateClozeCardRequest{
			ClozeCard: &fantiv1.ClozeCard{Character: "火", Sentence: "沒有那個字。"},
		})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("bad cloze code = %v, want InvalidArgument", connect.CodeOf(err))
		}

		listed, err := study.ListClozeCards(ctx, connect.NewRequest(&fantiv1.ListClozeCardsRequest{}))
		if err != nil {
			t.Fatalf("ListClozeCards: %v", err)
		}

		if len(listed.Msg.GetClozeCards()) != 2 {
			t.Fatalf("cloze cards = %d, want 2", len(listed.Msg.GetClozeCards()))
		}

		created, err := study.CreateQuiz(ctx, connect.NewRequest(&fantiv1.CreateQuizRequest{
			Pool: fantiv1.QuizPool_QUIZ_POOL_ALL,
		}))
		if err != nil {
			t.Fatalf("CreateQuiz: %v", err)
		}

		questions := created.Msg.GetQuestions()
		if len(questions) != 6 {
			t.Fatalf("questions = %d, want 6", len(questions))
		}

		for i, q := range questions[len(questions)-2:] {
			if q.GetType() != fantiv1.QuestionType_QUESTION_TYPE_CLOZE {
				t.Fatalf("tail question %d = %v, want CLOZE", i, q.GetType())
			}

			if !strings.Contains(q.GetPrompt(), "＿") {
				t.Errorf("cloze prompt %q lacks the blank", q.GetPrompt())
			}
		}

		for _, q := range questions[:len(questions)-2] {
			if q.GetType() == fantiv1.QuestionType_QUESTION_TYPE_CLOZE {
				t.Error("cloze question outside the tail")
			}
		}
	})

	t.Run("LessonQuiz", func(t *testing.T) {
		created, err := study.CreateQuiz(ctx, connect.NewRequest(&fantiv1.CreateQuizRequest{
			Pool:            fantiv1.QuizPool_QUIZ_POOL_LESSON,
			LessonCharacter: "龍",
		}))
		if err != nil {
			t.Fatalf("CreateQuiz lesson: %v", err)
		}

		quiz := created.Msg
		if quiz.GetLessonCharacter() != "龍" || len(quiz.GetQuestions()) != 4 {
			t.Fatalf("lesson quiz = %s with %d questions, want 龍 with 4",
				quiz.GetLessonCharacter(), len(quiz.GetQuestions()))
		}

		wantTypes := []fantiv1.QuestionType{
			fantiv1.QuestionType_QUESTION_TYPE_READING,
			fantiv1.QuestionType_QUESTION_TYPE_TYPE,
			fantiv1.QuestionType_QUESTION_TYPE_READING,
			fantiv1.QuestionType_QUESTION_TYPE_MEANING,
		}
		for i, q := range quiz.GetQuestions() {
			if q.GetType() != wantTypes[i] {
				t.Errorf("q%d type = %v, want %v", i, q.GetType(), wantTypes[i])
			}
		}

		if quiz.GetQuestions()[0].GetCharacter() != "龍" ||
			quiz.GetQuestions()[1].GetCharacter() != "龍" {
			t.Error("lesson quiz must open with the lesson character")
		}

		if _, err := study.CreateQuiz(ctx, connect.NewRequest(&fantiv1.CreateQuizRequest{
			Pool: fantiv1.QuizPool_QUIZ_POOL_LESSON,
		})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("missing lesson char code = %v, want InvalidArgument", connect.CodeOf(err))
		}
	})

	t.Run("GetLesson", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			INSERT INTO characters (
				traditional, simplified, pinyin, meaning, frequency_rank,
				catalog_kind, curriculum_rank
			) VALUES ('㐀', '㐀', 'qiū', '', 1, 'reference', 0)`); err != nil {
			t.Fatalf("insert reference character: %v", err)
		}
		defer func() {
			if _, err := pool.Exec(ctx, "DELETE FROM characters WHERE traditional = '㐀'"); err != nil {
				t.Errorf("delete reference character: %v", err)
			}
		}()

		resp, err := study.GetLesson(ctx, connect.NewRequest(&fantiv1.GetLessonRequest{}))
		if err != nil {
			t.Fatalf("GetLesson: %v", err)
		}

		lesson := resp.Msg
		if len(lesson.GetWeakCharacters()) == 0 || len(lesson.GetWeakCharacters()) > 2 {
			t.Fatalf("weak = %d characters, want 1-2", len(lesson.GetWeakCharacters()))
		}

		if got := lesson.GetWeakCharacters()[0].GetTraditional(); got != wrongChar {
			t.Errorf("weakest = %s, want %s", got, wrongChar)
		}

		// Reference 㐀 has a lower frequency rank than 上, but reference
		// characters never enter the automatic path.
		next := lesson.GetNextCharacter()
		if next.GetTraditional() != "上" ||
			next.GetCatalogKind() != fantiv1.CharacterCatalogKind_CHARACTER_CATALOG_KIND_CURRICULUM {
			t.Errorf("next = %s (%s), want curriculum character 上",
				next.GetTraditional(), next.GetCatalogKind())
		}
	})

	t.Run("UpdateProfileAndRecordPractice", func(t *testing.T) {
		updated, err := study.UpdateStudyProfile(ctx, connect.NewRequest(&fantiv1.UpdateStudyProfileRequest{
			StudyProfile: &fantiv1.StudyProfile{
				Name:    profileResource,
				Goal:    fantiv1.Goal_GOAL_EXAM,
				Mission: "Pass HSK 2 in December.",
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"goal", "mission"}},
		}))
		if err != nil {
			t.Fatalf("UpdateStudyProfile: %v", err)
		}

		if updated.Msg.GetGoal() != fantiv1.Goal_GOAL_EXAM ||
			updated.Msg.GetMission() != "Pass HSK 2 in December." {
			t.Errorf("profile = %v/%q after update",
				updated.Msg.GetGoal(), updated.Msg.GetMission())
		}

		if _, err := study.UpdateStudyProfile(ctx, connect.NewRequest(&fantiv1.UpdateStudyProfileRequest{
			StudyProfile: &fantiv1.StudyProfile{Name: profileResource},
			UpdateMask:   &fieldmaskpb.FieldMask{Paths: []string{"learned_count"}},
		})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("output-only path code = %v, want InvalidArgument", connect.CodeOf(err))
		}

		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

		profile, err := study.RecordPractice(ctx, connect.NewRequest(&fantiv1.RecordPracticeRequest{
			Day: yesterday,
		}))
		if err != nil {
			t.Fatalf("RecordPractice: %v", err)
		}

		if !slices.Contains(profile.Msg.GetPracticeDays(), yesterday) {
			t.Errorf("practice days %v missing %s", profile.Msg.GetPracticeDays(), yesterday)
		}

		if _, err := study.RecordPractice(ctx, connect.NewRequest(&fantiv1.RecordPracticeRequest{
			Day: "2026-13-40",
		})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("bad day code = %v, want InvalidArgument", connect.CodeOf(err))
		}
	})

	t.Run("GradeWordCard", func(t *testing.T) {
		resp, err := study.GradeCard(ctx, connect.NewRequest(&fantiv1.GradeCardRequest{
			Name:  "reviews/你好",
			Grade: fantiv1.Grade_GRADE_EASY,
		}))
		if err != nil {
			t.Fatalf("GradeCard word: %v", err)
		}

		if resp.Msg.GetSrsLevel() != 1 {
			t.Errorf("easy on unseen word = level %d, want 1", resp.Msg.GetSrsLevel())
		}
	})

	t.Run("GradeHandwriting", func(t *testing.T) {
		medians, err := json.Marshal([][][2]float64{
			{{100, 450}, {900, 450}},
			{{100, 250}, {900, 250}},
		})
		if err != nil {
			t.Fatalf("marshal medians: %v", err)
		}

		if _, err := pool.Exec(ctx,
			"INSERT INTO stroke_data (ch, medians, stroke_count) VALUES ('馬', $1, 2)",
			medians); err != nil {
			t.Fatalf("insert stroke data: %v", err)
		}

		good, err := tutor.GradeHandwriting(ctx, connect.NewRequest(&fantiv1.GradeHandwritingRequest{
			Character:          "馬",
			PracticeDifficulty: fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_RECALL,
			HintUsed:           true,
			Strokes: []*fantiv1.Stroke{
				{Points: []*fantiv1.Point{{X: 25, Y: 115}, {X: 230, Y: 115}}},
				{Points: []*fantiv1.Point{{X: 25, Y: 165}, {X: 230, Y: 165}}},
			},
			CanvasWidth:  260,
			CanvasHeight: 260,
		}))
		if err != nil {
			t.Fatalf("GradeHandwriting: %v", err)
		}

		if !good.Msg.GetCorrect() || len(good.Msg.GetBadStrokes()) != 0 {
			t.Errorf("good attempt = correct %v bad %v", good.Msg.GetCorrect(), good.Msg.GetBadStrokes())
		}

		if good.Msg.GetFeedback().GetTc() == "" {
			t.Error("praise feedback missing")
		}

		var (
			recordedDifficulty int32
			recordedHint       bool
			recordedCorrect    bool
		)
		if err := pool.QueryRow(ctx, `
			SELECT difficulty, hint_used, correct
			FROM handwriting_attempts WHERE ch = '馬'
			ORDER BY id DESC LIMIT 1`).
			Scan(&recordedDifficulty, &recordedHint, &recordedCorrect); err != nil {
			t.Fatalf("load recorded handwriting attempt: %v", err)
		}
		if recordedDifficulty != int32(fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_RECALL) ||
			!recordedHint || !recordedCorrect {
			t.Errorf("recorded attempt = difficulty %d hint %v correct %v",
				recordedDifficulty, recordedHint, recordedCorrect)
		}

		short, err := tutor.GradeHandwriting(ctx, connect.NewRequest(&fantiv1.GradeHandwritingRequest{
			Character: "馬",
			Strokes: []*fantiv1.Stroke{
				{Points: []*fantiv1.Point{{X: 25, Y: 115}, {X: 230, Y: 115}}},
			},
			CanvasWidth:  260,
			CanvasHeight: 260,
		}))
		if err != nil {
			t.Fatalf("GradeHandwriting short: %v", err)
		}

		if short.Msg.GetCorrect() || short.Msg.GetExpectedStrokes() != 2 || short.Msg.GetGotStrokes() != 1 {
			t.Errorf("short attempt = %v %d/%d, want wrong 2/1", short.Msg.GetCorrect(),
				short.Msg.GetExpectedStrokes(), short.Msg.GetGotStrokes())
		}

		if !strings.Contains(short.Msg.GetFeedback().GetEn(), "2") {
			t.Errorf("count feedback %q lacks the expected count", short.Msg.GetFeedback().GetEn())
		}

		if _, err := tutor.GradeHandwriting(ctx, connect.NewRequest(&fantiv1.GradeHandwritingRequest{
			Character:    "鬱",
			Strokes:      []*fantiv1.Stroke{{Points: []*fantiv1.Point{{X: 1, Y: 1}}}},
			CanvasWidth:  260,
			CanvasHeight: 260,
		})); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("missing stroke data code = %v, want NotFound", connect.CodeOf(err))
		}
	})

	t.Run("ExplainMistake", func(t *testing.T) {
		sibling, err := tutor.ExplainMistake(ctx, connect.NewRequest(&fantiv1.ExplainMistakeRequest{
			Character:    "髮",
			GivenAnswer:  "發",
			QuestionType: "type",
		}))
		if err != nil {
			t.Fatalf("ExplainMistake: %v", err)
		}

		en := sibling.Msg.GetExplanation().GetEn()
		if !strings.Contains(en, "發") || !strings.Contains(en, "髮") || !strings.Contains(en, "发") {
			t.Errorf("sibling explanation %q must mention both characters and the shared simplified form", en)
		}

		if sibling.Msg.GetExplanation().GetTc() == "" || sibling.Msg.GetExplanation().GetSc() == "" {
			t.Error("sibling explanation missing locales")
		}

		tone, err := tutor.ExplainMistake(ctx, connect.NewRequest(&fantiv1.ExplainMistakeRequest{
			Character:    "馬",
			QuestionType: "tone",
		}))
		if err != nil {
			t.Fatalf("ExplainMistake tone: %v", err)
		}

		if !strings.Contains(tone.Msg.GetExplanation().GetTc(), "第三聲") {
			t.Errorf("tone explanation %q must name the third tone", tone.Msg.GetExplanation().GetTc())
		}

		generic, err := tutor.ExplainMistake(ctx, connect.NewRequest(&fantiv1.ExplainMistakeRequest{
			Character:    "愛",
			GivenAnswer:  "book",
			QuestionType: "meaning",
		}))
		if err != nil {
			t.Fatalf("ExplainMistake generic: %v", err)
		}

		if !strings.Contains(generic.Msg.GetExplanation().GetEn(), "ài") {
			t.Errorf("generic explanation %q must restate the reading", generic.Msg.GetExplanation().GetEn())
		}
	})
}
