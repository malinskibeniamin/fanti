package server

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

// Question type names as stored in quizzes.questions.
const (
	qtReading = "reading"
	qtTone    = "tone"
	qtWrite   = "write"
	qtType    = "type"
	qtMeaning = "meaning"
	qtListen  = "listen"
	qtScript  = "script"
	qtMatch   = "match"
	qtCloze   = "cloze"
)

//nolint:gochecknoglobals // static proto enum mapping
var questionTypeEnum = map[string]fantiv1.QuestionType{
	qtReading: fantiv1.QuestionType_QUESTION_TYPE_READING,
	qtTone:    fantiv1.QuestionType_QUESTION_TYPE_TONE,
	qtWrite:   fantiv1.QuestionType_QUESTION_TYPE_WRITE,
	qtType:    fantiv1.QuestionType_QUESTION_TYPE_TYPE,
	qtMeaning: fantiv1.QuestionType_QUESTION_TYPE_MEANING,
	qtListen:  fantiv1.QuestionType_QUESTION_TYPE_LISTEN,
	qtScript:  fantiv1.QuestionType_QUESTION_TYPE_SCRIPT,
	qtMatch:   fantiv1.QuestionType_QUESTION_TYPE_MATCH,
	qtCloze:   fantiv1.QuestionType_QUESTION_TYPE_CLOZE,
}

// quizEntry is one deck member available to the quiz builder.
type quizEntry struct {
	Character  string
	Simplified string
	Pinyin     string
	Meaning    string
	HasStrokes bool
}

// clozeEntry is a saved reading sentence plus its blanked character.
type clozeEntry struct {
	Entry    quizEntry
	Sentence string
}

// quizQuestionRow is the stored question shape, answers included. Only the
// answerless projection ever leaves the server.
type quizQuestionRow struct {
	Type      string `json:"type"`
	Prompt    string `json:"prompt,omitempty"`
	Character string `json:"character"`
	// Simplified backs IME-leniency grading of TYPE questions.
	Simplified string   `json:"simplified,omitempty"`
	TtsText    string   `json:"ttsText,omitempty"`
	Options    []string `json:"options,omitempty"`
	// Answer is the correct option index, or -1 for WRITE and TYPE.
	Answer int `json:"answer"`
}

// toneOf ports the prototype's tone detector: decompose the pinyin and
// look for the combining tone marks in mark order.
func toneOf(pinyin string) int {
	d := norm.NFD.String(pinyin)

	switch {
	case strings.ContainsRune(d, '̄'): // macron — first tone
		return 1
	case strings.ContainsRune(d, '́'): // acute — second tone
		return 2
	case strings.ContainsRune(d, '̌'): // caron — third tone
		return 3
	case strings.ContainsRune(d, '̀'): // grave — fourth tone
		return 4
	default:
		return 0
	}
}

// shuffled returns a shuffled copy.
func shuffled[T any](rng *rand.Rand, in []T) []T {
	out := slices.Clone(in)
	rng.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })

	return out
}

// optionEntries picks the answer plus up to three decoys from the deck,
// shuffled, exactly like the prototype's buildQ option assembly.
func optionEntries(rng *rand.Rand, e quizEntry, deck []quizEntry) ([]quizEntry, int) {
	others := make([]quizEntry, 0, len(deck))

	for _, o := range deck {
		if o.Character != e.Character {
			others = append(others, o)
		}
	}

	others = shuffled(rng, others)
	if len(others) > 3 {
		others = others[:3]
	}

	opts := shuffled(rng, append([]quizEntry{e}, others...))

	answer := slices.IndexFunc(opts, func(o quizEntry) bool { return o.Character == e.Character })

	return opts, answer
}

func optionLabels(opts []quizEntry, label func(quizEntry) string) []string {
	labels := make([]string, 0, len(opts))
	for _, o := range opts {
		labels = append(labels, label(o))
	}

	return labels
}

func (e quizEntry) supportsQuestion(qType string) bool {
	switch qType {
	case qtReading:
		return e.Pinyin != ""
	case qtTone:
		return e.Pinyin != "" && toneOf(e.Pinyin) > 0
	case qtWrite:
		return e.HasStrokes
	case qtType:
		return true
	case qtMeaning:
		return e.Meaning != ""
	case qtListen:
		return e.Pinyin != ""
	case qtScript, qtMatch:
		return e.Simplified != "" && e.Simplified != e.Character
	case qtCloze:
		return true
	default:
		return false
	}
}

func adaptiveQuestionType(e quizEntry, preferred string) string {
	if e.supportsQuestion(preferred) {
		return preferred
	}

	for _, candidate := range []string{
		qtReading,
		qtMeaning,
		qtListen,
		qtWrite,
		qtType,
		qtScript,
		qtMatch,
		qtTone,
	} {
		if e.supportsQuestion(candidate) {
			return candidate
		}
	}

	return qtType
}

func optionDeck(qType string, deck []quizEntry) []quizEntry {
	switch qType {
	case qtReading, qtMeaning, qtMatch:
		eligible := make([]quizEntry, 0, len(deck))
		for _, candidate := range deck {
			if candidate.supportsQuestion(qType) {
				eligible = append(eligible, candidate)
			}
		}

		return eligible
	default:
		return deck
	}
}

// buildQuestion ports the prototype's buildQ for one entry and type.
func buildQuestion(rng *rand.Rand, qType string, e quizEntry, deck []quizEntry) quizQuestionRow {
	switch qType {
	case qtScript:
		q := quizQuestionRow{
			Type: qtScript, Prompt: e.Character, Character: e.Character,
			Options: []string{"简体", "繁體"}, Answer: 1,
		}
		if rng.Float64() < 0.5 {
			q.Prompt = e.Simplified
			q.Answer = 0
		}

		return q
	case qtTone:
		return quizQuestionRow{
			Type: qtTone, Character: e.Character, TtsText: e.Character,
			Options: []string{"第一聲", "第二聲", "第三聲", "第四聲"},
			Answer:  toneOf(e.Pinyin) - 1,
		}
	case qtListen:
		opts, answer := optionEntries(rng, e, optionDeck(qType, deck))

		return quizQuestionRow{
			Type: qtListen, Character: e.Character, TtsText: e.Character,
			Options: optionLabels(opts, func(o quizEntry) string { return o.Character }),
			Answer:  answer,
		}
	case qtWrite, qtType:
		return quizQuestionRow{
			Type: qType, Character: e.Character, Simplified: e.Simplified, Answer: -1,
		}
	default: // reading, meaning, match — glyph prompt with typed option labels
		opts, answer := optionEntries(rng, e, optionDeck(qType, deck))
		q := quizQuestionRow{Type: qType, Prompt: e.Character, Character: e.Character, Answer: answer}

		switch qType {
		case qtMeaning:
			q.Options = optionLabels(opts, func(o quizEntry) string { return o.Meaning })
		case qtMatch:
			q.Options = optionLabels(opts, func(o quizEntry) string { return o.Simplified })
		default: // reading
			q.Options = optionLabels(opts, func(o quizEntry) string { return o.Pinyin })
		}

		return q
	}
}

// buildQuiz ports the prototype's startQuiz composition. A lesson quiz is
// two questions on the lesson character plus two interleaved deck
// questions; otherwise up to eight pool questions cycle through the type
// list (with script/match and tone fallbacks), and up to two saved cloze
// cards replace the tail.
func buildQuiz(rng *rand.Rand, source, deck []quizEntry, cloze []clozeEntry, lesson *quizEntry) []quizQuestionRow {
	if lesson != nil {
		return buildLessonQuiz(rng, *lesson, deck)
	}

	if len(source) == 0 {
		source = deck
	}

	types := []string{qtReading, qtTone, qtWrite, qtType, qtMeaning, qtListen, qtScript, qtMatch}

	pool := shuffled(rng, source)
	if len(pool) > 8 {
		pool = pool[:8]
	}

	questions := make([]quizQuestionRow, 0, len(pool))

	for i, e := range pool {
		qType := types[i%len(types)]

		qType = adaptiveQuestionType(e, qType)

		questions = append(questions, buildQuestion(rng, qType, e, deck))
	}

	tail := shuffled(rng, cloze)
	if len(tail) > 2 {
		tail = tail[:2]
	}

	if len(tail) == 0 {
		return questions
	}

	questions = questions[:max(0, len(questions)-len(tail))]

	for _, c := range tail {
		opts, answer := optionEntries(rng, c.Entry, deck)
		questions = append(questions, quizQuestionRow{
			Type:      qtCloze,
			Prompt:    strings.Replace(c.Sentence, c.Entry.Character, "＿", 1),
			Character: c.Entry.Character,
			Options:   optionLabels(opts, func(o quizEntry) string { return o.Character }),
			Answer:    answer,
		})
	}

	return questions
}

func buildLessonQuiz(rng *rand.Rand, lesson quizEntry, deck []quizEntry) []quizQuestionRow {
	others := make([]quizEntry, 0, len(deck))

	for _, o := range deck {
		if o.Character != lesson.Character {
			others = append(others, o)
		}
	}

	others = shuffled(rng, others)
	if len(others) > 2 {
		others = others[:2]
	}

	questions := make([]quizQuestionRow, 0, 2+len(others))
	lessonTypes := []string{}
	for _, qType := range []string{
		qtReading,
		qtType,
		qtMeaning,
		qtWrite,
		qtListen,
		qtScript,
		qtMatch,
		qtTone,
	} {
		if lesson.supportsQuestion(qType) {
			lessonTypes = append(lessonTypes, qType)
		}
		if len(lessonTypes) == 2 {
			break
		}
	}
	for _, qType := range lessonTypes {
		questions = append(questions, buildQuestion(rng, qType, lesson, deck))
	}

	for i, o := range others {
		qType := qtReading
		if i > 0 {
			qType = qtMeaning
		}

		questions = append(questions,
			buildQuestion(rng, adaptiveQuestionType(o, qType), o, deck))
	}

	return questions
}

var (
	errLessonCharRequired = errors.New("lesson_character is required for QUIZ_POOL_LESSON")
	errDeckEmpty          = errors.New("the deck has no cards to quiz")
)

// CreateQuiz composes and stores a new quiz for the requested pool.
func (s *Study) CreateQuiz(
	ctx context.Context, req *connect.Request[fantiv1.CreateQuizRequest],
) (*connect.Response[fantiv1.Quiz], error) {
	deck, err := s.quizDeck(ctx, deckWhere)
	if err != nil {
		return nil, err
	}

	var (
		source []quizEntry
		cloze  []clozeEntry
		lesson *quizEntry
	)

	switch req.Msg.GetPool() {
	case fantiv1.QuizPool_QUIZ_POOL_WEAK:
		source, err = s.quizDeck(ctx, " WHERE COALESCE(r.mistake_count, 0) > 0 ")
		if err != nil {
			return nil, err
		}
	case fantiv1.QuizPool_QUIZ_POOL_LESSON:
		lesson, err = s.lessonEntry(ctx, req.Msg.GetLessonCharacter())
		if err != nil {
			return nil, err
		}
	case fantiv1.QuizPool_QUIZ_POOL_ALL, fantiv1.QuizPool_QUIZ_POOL_UNSPECIFIED:
		source = deck
	}

	if lesson == nil {
		cloze, err = s.clozeEntries(ctx)
		if err != nil {
			return nil, err
		}
	}

	questions := buildQuiz(s.newRand(), source, deck, cloze, lesson)
	if len(questions) == 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errDeckEmpty)
	}

	raw, err := json.Marshal(questions)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	id := newQuizID()

	if _, err := s.pool.Exec(ctx,
		"INSERT INTO quizzes (id, questions, lesson_ch) VALUES ($1, $2, $3)",
		id, raw, req.Msg.GetLessonCharacter()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(quizProto(id, questions, 0, 0, false, nil,
		req.Msg.GetLessonCharacter())), nil
}

// quizDeck loads quiz entries for a deck WHERE clause in frequency order.
func (s *Study) quizDeck(ctx context.Context, where string) ([]quizEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT c.traditional, c.simplified, c.pinyin, c.meaning,
			EXISTS (
				SELECT 1 FROM stroke_data
				WHERE stroke_data.ch = c.traditional
					AND stroke_data.data IS NOT NULL
			)`+characterFrom+where+
			"ORDER BY c.frequency_rank = 0, c.frequency_rank, c.traditional")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var deck []quizEntry

	for rows.Next() {
		var e quizEntry
		if err := rows.Scan(
			&e.Character,
			&e.Simplified,
			&e.Pinyin,
			&e.Meaning,
			&e.HasStrokes,
		); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		deck = append(deck, e)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return deck, nil
}

func (s *Study) lessonEntry(ctx context.Context, ch string) (*quizEntry, error) {
	if ch == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errLessonCharRequired)
	}

	var e quizEntry

	err := s.pool.QueryRow(ctx,
		`SELECT traditional, simplified, pinyin, meaning,
			EXISTS (
				SELECT 1 FROM stroke_data
				WHERE stroke_data.ch = characters.traditional
					AND stroke_data.data IS NOT NULL
			)
		FROM characters WHERE traditional = $1`, ch).
		Scan(&e.Character, &e.Simplified, &e.Pinyin, &e.Meaning, &e.HasStrokes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("character %q not found", ch)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &e, nil
}

// clozeEntries loads every saved cloze card with the blanked character's
// entry (curated character first, ruby pinyin fallback otherwise).
func (s *Study) clozeEntries(ctx context.Context) ([]clozeEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT cc.ch, cc.sentence, COALESCE(c.simplified, cc.ch),
			COALESCE(c.pinyin, cp.pinyin, ''), COALESCE(c.meaning, '')
		FROM cloze_cards cc
		LEFT JOIN characters c ON c.traditional = cc.ch
		LEFT JOIN char_pinyin cp ON cp.ch = cc.ch
		ORDER BY cc.id`)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var cards []clozeEntry

	for rows.Next() {
		var c clozeEntry
		if err := rows.Scan(&c.Entry.Character, &c.Sentence, &c.Entry.Simplified,
			&c.Entry.Pinyin, &c.Entry.Meaning); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		cards = append(cards, c)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return cards, nil
}

// quizProto projects stored questions into the answerless wire shape.
func quizProto(
	id string, questions []quizQuestionRow,
	index, score int32, finished bool, mistakes []string, lessonCh string,
) *fantiv1.Quiz {
	quiz := &fantiv1.Quiz{
		Name:            "quizzes/" + id,
		CurrentIndex:    index,
		Score:           score,
		Finished:        finished,
		Mistakes:        mistakes,
		LessonCharacter: lessonCh,
	}

	for _, q := range questions {
		quiz.Questions = append(quiz.Questions, &fantiv1.QuizQuestion{
			Type:      questionTypeEnum[q.Type],
			Prompt:    q.Prompt,
			Character: q.Character,
			TtsText:   q.TtsText,
			Options:   slices.Clone(q.Options),
		})
	}

	return quiz
}

func newQuizID() string {
	var b [6]byte

	_, _ = cryptorand.Read(b[:])

	return "qz" + hex.EncodeToString(b[:])
}

var (
	errQuizFinished  = errors.New("quiz is already finished")
	errWrongQuestion = errors.New("question_index does not match the quiz's current question")
	errAnswerShape   = errors.New("answer kind does not match the question type")
)

// SubmitQuizAnswer grades one answer server-side, advances the quiz, and
// returns tutor feedback on misses.
func (s *Study) SubmitQuizAnswer(
	ctx context.Context, req *connect.Request[fantiv1.SubmitQuizAnswerRequest],
) (*connect.Response[fantiv1.SubmitQuizAnswerResponse], error) {
	id, err := parseName("quizzes", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var (
		raw          []byte
		index, score int32
		finished     bool
		mistakes     []string
		lessonCh     string
	)

	err = tx.QueryRow(ctx, `
		SELECT questions, current_index, score, finished, mistakes, lesson_ch
		FROM quizzes WHERE id = $1 FOR UPDATE`, id).
		Scan(&raw, &index, &score, &finished, &mistakes, &lessonCh)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("quiz %q not found", id)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if finished {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errQuizFinished)
	}

	var questions []quizQuestionRow
	if err := json.Unmarshal(raw, &questions); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decode questions: %w", err))
	}

	if req.Msg.GetQuestionIndex() != index || int(index) >= len(questions) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errWrongQuestion)
	}

	question := questions[index]

	correct, given, err := gradeAnswer(question, req.Msg)
	if err != nil {
		return nil, err
	}

	resp := &fantiv1.SubmitQuizAnswerResponse{
		Correct:       correct,
		CorrectAnswer: correctAnswerOf(question),
	}

	if correct {
		score++
	} else {
		mistakes = append(mistakes, question.Character)

		if _, err := tx.Exec(ctx, `
			INSERT INTO reviews (ch, mistake_count) VALUES ($1, 1)
			ON CONFLICT (ch) DO UPDATE SET mistake_count = reviews.mistake_count + 1`,
			question.Character); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp.Feedback, err = explainAnswer(ctx, tx, question.Character, given, question.Type)
		if err != nil {
			return nil, err
		}
	}

	index++
	finished = int(index) >= len(questions)

	if _, err := tx.Exec(ctx, `
		UPDATE quizzes SET current_index = $2, score = $3, finished = $4, mistakes = $5
		WHERE id = $1`, id, index, score, finished, mistakes); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if finished {
		if err := markPractice(ctx, tx); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp.Quiz = quizProto(id, questions, index, score, finished, mistakes, lessonCh)

	return connect.NewResponse(resp), nil
}

// gradeAnswer checks one answer against the stored question and reports
// what the learner gave (for tutor feedback).
func gradeAnswer(q quizQuestionRow, msg *fantiv1.SubmitQuizAnswerRequest) (bool, string, error) {
	switch answer := msg.GetAnswer().(type) {
	case *fantiv1.SubmitQuizAnswerRequest_SelfCorrect:
		if q.Type != qtWrite {
			return false, "", connect.NewError(connect.CodeInvalidArgument, errAnswerShape)
		}

		return answer.SelfCorrect, "", nil
	case *fantiv1.SubmitQuizAnswerRequest_TypedText:
		if q.Type != qtType {
			return false, "", connect.NewError(connect.CodeInvalidArgument, errAnswerShape)
		}

		typed := strings.TrimSpace(answer.TypedText)
		// IME leniency: typing the simplified form of the same character
		// counts; typing a sibling character does not.
		correct := typed == q.Character || (q.Simplified != "" && typed == q.Simplified)

		return correct, typed, nil
	case *fantiv1.SubmitQuizAnswerRequest_OptionIndex:
		if q.Answer < 0 || len(q.Options) == 0 {
			return false, "", connect.NewError(connect.CodeInvalidArgument, errAnswerShape)
		}

		given := ""
		if idx := int(answer.OptionIndex); idx >= 0 && idx < len(q.Options) {
			given = q.Options[idx]
		}

		return int(answer.OptionIndex) == q.Answer, given, nil
	default:
		return false, "", connect.NewError(connect.CodeInvalidArgument, errAnswerShape)
	}
}

// correctAnswerOf renders the stored answer for feedback display.
func correctAnswerOf(q quizQuestionRow) string {
	if q.Answer >= 0 && q.Answer < len(q.Options) {
		return q.Options[q.Answer]
	}

	return q.Character
}
