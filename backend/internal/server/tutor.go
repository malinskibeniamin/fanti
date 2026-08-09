package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

// Tutor serves fanti.v1.TutorService with rule-based feedback. An
// LLM-backed implementation can replace it behind the same contract.
type Tutor struct {
	pool *pgxpool.Pool
}

// NewTutor builds the tutor service.
func NewTutor(pool *pgxpool.Pool) *Tutor {
	return &Tutor{pool: pool}
}

var (
	errCharacterRequired = errors.New("character is required")
	errCanvasRequired    = errors.New("canvas_width and canvas_height must be positive")
)

// Handwriting grading thresholds, ported verbatim from the prototype.
const (
	// hanzi-writer medians live in a 1024-unit box with the baseline at
	// y=900 and the y axis pointing up; the canvas y axis points down.
	medianScale    = 1024.0
	medianBaseline = 900.0
	// A reference stroke shorter than this (normalized) is a dot: only
	// its position is graded.
	dotLength = 0.08
	// A dot whose start is further off than this is wrong.
	dotStartOff = 0.4
	// A stroke drawn more than this many degrees off, starting further
	// than 0.45 away, or ending further than 0.3 away is wrong.
	maxAngleDiff = 50.0
	maxStartOff  = 0.45
	maxEndOff    = 0.3
)

// handwritingResult mirrors the prototype's gradeHandwriting return shape.
type handwritingResult struct {
	correct       bool
	countMismatch bool
	expected      int
	got           int
	bad           []int
}

// gradeStrokes ports the prototype's gradeHandwriting: normalize both
// point sets to the unit square, treat short reference strokes as dots
// (position only), and mark a stroke bad when its start, end, or overall
// direction is off.
func gradeStrokes(medians [][][2]float64, strokes []*fantiv1.Stroke, width, height float64) handwritingResult {
	res := handwritingResult{expected: len(medians), got: len(strokes)}

	if len(strokes) != len(medians) {
		res.countMismatch = true

		return res
	}

	angle := func(a, b [2]float64) float64 {
		return math.Atan2(b[1]-a[1], b[0]-a[0]) * 180 / math.Pi
	}

	for i, median := range medians {
		points := strokes[i].GetPoints()
		if len(points) == 0 || len(median) == 0 {
			res.bad = append(res.bad, i+1)

			continue
		}

		m0 := normMedian(median[0])
		m1 := normMedian(median[len(median)-1])
		u0 := normUser(points[0], width, height)
		u1 := normUser(points[len(points)-1], width, height)

		length := math.Hypot(m1[0]-m0[0], m1[1]-m0[1])
		startOff := math.Hypot(m0[0]-u0[0], m0[1]-u0[1])
		endOff := math.Hypot(m1[0]-u1[0], m1[1]-u1[1])

		if length < dotLength {
			if startOff > dotStartOff {
				res.bad = append(res.bad, i+1)
			}

			continue
		}

		diff := math.Abs(angle(m0, m1) - angle(u0, u1))
		if diff > 180 {
			diff = 360 - diff
		}

		if diff > maxAngleDiff || startOff > maxStartOff || endOff > maxEndOff {
			res.bad = append(res.bad, i+1)
		}
	}

	res.correct = len(res.bad) == 0

	return res
}

func normMedian(p [2]float64) [2]float64 {
	return [2]float64{p[0] / medianScale, (medianBaseline - p[1]) / medianScale}
}

func normUser(p *fantiv1.Point, width, height float64) [2]float64 {
	return [2]float64{float64(p.GetX()) / width, float64(p.GetY()) / height}
}

// GradeHandwriting grades canvas strokes against reference stroke medians.
func (t *Tutor) GradeHandwriting(
	ctx context.Context, req *connect.Request[fantiv1.GradeHandwritingRequest],
) (*connect.Response[fantiv1.GradeHandwritingResponse], error) {
	ch := req.Msg.GetCharacter()
	if ch == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errCharacterRequired)
	}

	if req.Msg.GetCanvasWidth() <= 0 || req.Msg.GetCanvasHeight() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errCanvasRequired)
	}

	difficulty := req.Msg.GetPracticeDifficulty()
	if difficulty == fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_UNSPECIFIED {
		difficulty = fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED
	}
	if difficulty < fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED ||
		difficulty > fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_MASTERY {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("practice_difficulty %v is not known", difficulty)) //nolint:err113 // request detail
	}

	var raw []byte

	err := t.pool.QueryRow(ctx,
		"SELECT medians FROM stroke_data WHERE ch = $1", ch).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound,
			fmt.Errorf("no stroke data for %q yet — try again later", ch)) //nolint:err113 // request detail
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var medians [][][2]float64
	if err := json.Unmarshal(raw, &medians); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decode medians: %w", err))
	}

	res := gradeStrokes(medians, req.Msg.GetStrokes(),
		float64(req.Msg.GetCanvasWidth()), float64(req.Msg.GetCanvasHeight()))

	badStrokes := make([]int32, 0, len(res.bad))
	for _, b := range res.bad {
		badStrokes = append(badStrokes, int32(b)) //nolint:gosec // stroke counts are tiny
	}

	if _, err := t.pool.Exec(ctx, `
		INSERT INTO handwriting_attempts (ch, difficulty, hint_used, correct)
		VALUES ($1, $2, $3, $4)`,
		ch, difficulty, req.Msg.GetHintUsed(), res.correct); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fantiv1.GradeHandwritingResponse{
		Correct:         res.correct,
		ExpectedStrokes: int32(res.expected), //nolint:gosec // stroke counts are tiny
		GotStrokes:      int32(res.got),      //nolint:gosec // stroke counts are tiny
		BadStrokes:      badStrokes,
		Feedback:        handwritingFeedback(ch, res),
	}), nil
}

// handwritingFeedback composes the tutor critique of an attempt.
func handwritingFeedback(ch string, res handwritingResult) *fantiv1.LocalizedText {
	if res.countMismatch {
		return &fantiv1.LocalizedText{
			En: fmt.Sprintf("%s takes %d strokes but you drew %d. Watch the stroke-order animation and count along.",
				ch, res.expected, res.got),
			Tc: fmt.Sprintf("「%s」共 %d 筆，你寫了 %d 筆。看一次筆順動畫，邊看邊數。", ch, res.expected, res.got),
			Sc: fmt.Sprintf("“%s”共 %d 笔，你写了 %d 笔。看一次笔顺动画，边看边数。", ch, res.expected, res.got),
		}
	}

	if len(res.bad) == 0 {
		return &fantiv1.LocalizedText{
			En: fmt.Sprintf("Nicely written — every stroke of %s starts and ends in the right place.", ch),
			Tc: fmt.Sprintf("寫得漂亮 — 「%s」每一筆的起筆與收筆位置都對。", ch),
			Sc: fmt.Sprintf("写得漂亮 — “%s”每一笔的起笔与收笔位置都对。", ch),
		}
	}

	indices := strokeIndexList(res.bad)

	return &fantiv1.LocalizedText{
		En: fmt.Sprintf("Not quite — check where stroke %s of %s starts, ends, and moves. Trace the model once, then try again.", indices, ch),
		Tc: fmt.Sprintf("還差一點 — 檢查「%s」第 %s 筆的起筆、收筆與走向。先描一次範字，再試一次。", ch, indices),
		Sc: fmt.Sprintf("还差一点 — 检查“%s”第 %s 笔的起笔、收笔与走向。先描一次范字，再试一次。", ch, indices),
	}
}

func strokeIndexList(bad []int) string {
	parts := make([]string, 0, len(bad))
	for _, b := range bad {
		parts = append(parts, strconv.Itoa(b))
	}

	return strings.Join(parts, ", ")
}

// ExplainMistake explains why an answer was wrong (sibling characters,
// tones, lookalikes).
func (t *Tutor) ExplainMistake(
	ctx context.Context, req *connect.Request[fantiv1.ExplainMistakeRequest],
) (*connect.Response[fantiv1.ExplainMistakeResponse], error) {
	if req.Msg.GetCharacter() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errCharacterRequired)
	}

	explanation, err := explainAnswer(ctx, t.pool,
		req.Msg.GetCharacter(), req.Msg.GetGivenAnswer(), req.Msg.GetQuestionType())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&fantiv1.ExplainMistakeResponse{Explanation: explanation}), nil
}

// explainAnswer composes rule-based tutor feedback for a missed answer.
// The rules mirror the prototype: a sibling character that collapses to
// the same simplified form gets the 一字多繁 explanation, tone questions
// get the tone of the correct reading, and everything else gets the
// correct answer restated.
func explainAnswer(ctx context.Context, q queryRower, ch, given, questionType string) (*fantiv1.LocalizedText, error) {
	var (
		simplified, pinyin, meaning string
		siblings                    []string
	)

	err := q.QueryRow(ctx,
		"SELECT simplified, pinyin, meaning, siblings FROM characters WHERE traditional = $1", ch).
		Scan(&simplified, &pinyin, &meaning, &siblings)
	if errors.Is(err, pgx.ErrNoRows) {
		return &fantiv1.LocalizedText{
			En: fmt.Sprintf("The correct answer is %s.", ch),
			Tc: fmt.Sprintf("正解是「%s」。", ch),
			Sc: fmt.Sprintf("正解是“%s”。", ch),
		}, nil
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	sibling, err := isSibling(ctx, q, ch, given, simplified, siblings)
	if err != nil {
		return nil, err
	}

	switch {
	case sibling:
		return &fantiv1.LocalizedText{
			En: fmt.Sprintf("%s and %s are different characters that share the simplified form %s. Here the answer is %s (%s) — %s.",
				given, ch, simplified, ch, pinyin, meaning),
			Tc: fmt.Sprintf("「%s」和「%s」是不同的字，簡體都寫作「%s」。這裡的正解是「%s」（%s），意思是「%s」。",
				given, ch, simplified, ch, pinyin, meaning),
			Sc: fmt.Sprintf("“%s”和“%s”是不同的字，简体都写作“%s”。这里的正解是“%s”（%s），意思是“%s”。",
				given, ch, simplified, ch, pinyin, meaning),
		}, nil
	case questionType == qtTone:
		return toneExplanation(ch, pinyin), nil
	default:
		return &fantiv1.LocalizedText{
			En: fmt.Sprintf("The correct answer is %s, read %s — %s.", ch, pinyin, meaning),
			Tc: fmt.Sprintf("正解是「%s」，讀作「%s」，意思是「%s」。", ch, pinyin, meaning),
			Sc: fmt.Sprintf("正解是“%s”，读作“%s”，意思是“%s”。", ch, pinyin, meaning),
		}, nil
	}
}

// isSibling reports whether the given answer is a different traditional
// character that collapses to the same simplified form: either listed in
// the character's siblings, or sharing simplified in the characters table.
func isSibling(ctx context.Context, q queryRower, ch, given, simplified string, siblings []string) (bool, error) {
	if given == "" || given == ch {
		return false, nil
	}

	if slices.Contains(siblings, given) {
		return true, nil
	}

	var shared bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM characters
			WHERE traditional = $1 AND simplified = $2 AND traditional <> $3)`,
		given, simplified, ch).Scan(&shared); err != nil {
		return false, connect.NewError(connect.CodeInternal, err)
	}

	return shared, nil
}

func toneExplanation(ch, pinyin string) *fantiv1.LocalizedText {
	tone := toneOf(pinyin)
	if tone == 0 {
		return &fantiv1.LocalizedText{
			En: fmt.Sprintf("%s is read %s — a neutral tone, light and unstressed.", ch, pinyin),
			Tc: fmt.Sprintf("「%s」讀作「%s」，是輕聲，讀得輕而短。", ch, pinyin),
			Sc: fmt.Sprintf("“%s”读作“%s”，是轻声，读得轻而短。", ch, pinyin),
		}
	}

	toneEn := []string{"first (high and level)", "second (rising)", "third (dipping)", "fourth (falling)"}
	toneTc := []string{"第一聲", "第二聲", "第三聲", "第四聲"}
	toneSc := []string{"第一声", "第二声", "第三声", "第四声"}

	return &fantiv1.LocalizedText{
		En: fmt.Sprintf("%s is read %s — the %s tone.", ch, pinyin, toneEn[tone-1]),
		Tc: fmt.Sprintf("「%s」讀作「%s」，是%s。", ch, pinyin, toneTc[tone-1]),
		Sc: fmt.Sprintf("“%s”读作“%s”，是%s。", ch, pinyin, toneSc[tone-1]),
	}
}
