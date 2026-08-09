package server

import (
	"slices"
	"strings"
	"testing"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

// Reference stroke shapes in hanzi-writer median space (1024 box, y up,
// baseline 900) and matching user strokes on a 260x260 canvas (y down).
const canvas = 260.0

// horizontalMedian normalizes to start (0.098, 0.439) → end (0.879, 0.439).
func horizontalMedian() [][2]float64 {
	return [][2]float64{{100, 450}, {900, 450}}
}

// dotMedian is 0.022 long normalized — under the 0.08 dot threshold.
func dotMedian() [][2]float64 {
	return [][2]float64{{500, 450}, {520, 440}}
}

func stroke(points ...[2]float32) *fantiv1.Stroke {
	s := &fantiv1.Stroke{}
	for _, p := range points {
		s.Points = append(s.Points, &fantiv1.Point{X: p[0], Y: p[1]})
	}

	return s
}

// goodHorizontal starts on the median's start point and moves rightward.
func goodHorizontal() *fantiv1.Stroke {
	return stroke([2]float32{25, 115}, [2]float32{230, 115})
}

func TestGradeStrokesCountMismatch(t *testing.T) {
	res := gradeStrokes([][][2]float64{horizontalMedian(), horizontalMedian()},
		[]*fantiv1.Stroke{goodHorizontal()}, canvas, canvas)

	if !res.countMismatch || res.correct {
		t.Errorf("countMismatch = %v correct = %v, want true/false", res.countMismatch, res.correct)
	}

	if res.expected != 2 || res.got != 1 || len(res.bad) != 0 {
		t.Errorf("expected/got/bad = %d/%d/%v, want 2/1/[]", res.expected, res.got, res.bad)
	}
}

func TestGradeStrokesGoodStroke(t *testing.T) {
	res := gradeStrokes([][][2]float64{horizontalMedian()},
		[]*fantiv1.Stroke{goodHorizontal()}, canvas, canvas)

	if !res.correct || len(res.bad) != 0 {
		t.Errorf("correct = %v bad = %v, want true/[]", res.correct, res.bad)
	}
}

func TestGradeStrokesWrongDirection(t *testing.T) {
	// Starts at the right point but heads straight down: 90 degrees off.
	down := stroke([2]float32{25, 115}, [2]float32{25, 230})

	res := gradeStrokes([][][2]float64{horizontalMedian()},
		[]*fantiv1.Stroke{down}, canvas, canvas)

	if res.correct || !slices.Equal(res.bad, []int{1}) {
		t.Errorf("correct = %v bad = %v, want false/[1]", res.correct, res.bad)
	}
}

func TestGradeStrokesWrongStart(t *testing.T) {
	// Right direction, but starting from the far side of the canvas.
	backwards := stroke([2]float32{230, 115}, [2]float32{25, 115})

	res := gradeStrokes([][][2]float64{horizontalMedian()},
		[]*fantiv1.Stroke{backwards}, canvas, canvas)

	if res.correct || !slices.Equal(res.bad, []int{1}) {
		t.Errorf("correct = %v bad = %v, want false/[1]", res.correct, res.bad)
	}
}

func TestGradeStrokesRejectsTruncatedStroke(t *testing.T) {
	// Same start and direction as the reference, but far too short to form it.
	truncated := stroke([2]float32{25, 115}, [2]float32{40, 115})

	res := gradeStrokes([][][2]float64{horizontalMedian()},
		[]*fantiv1.Stroke{truncated}, canvas, canvas)

	if res.correct || !slices.Equal(res.bad, []int{1}) {
		t.Errorf("correct = %v bad = %v, want false/[1]", res.correct, res.bad)
	}
}

func TestGradeStrokesDotPositionOnly(t *testing.T) {
	// A dot near the reference position passes even drawn downward…
	nearDot := stroke([2]float32{127, 114}, [2]float32{130, 125})

	res := gradeStrokes([][][2]float64{dotMedian()},
		[]*fantiv1.Stroke{nearDot}, canvas, canvas)
	if !res.correct || len(res.bad) != 0 {
		t.Errorf("near dot: correct = %v bad = %v, want true/[]", res.correct, res.bad)
	}

	// …but a dot in the wrong corner fails on position.
	farDot := stroke([2]float32{10, 10}, [2]float32{12, 12})

	res = gradeStrokes([][][2]float64{dotMedian()},
		[]*fantiv1.Stroke{farDot}, canvas, canvas)
	if res.correct || !slices.Equal(res.bad, []int{1}) {
		t.Errorf("far dot: correct = %v bad = %v, want false/[1]", res.correct, res.bad)
	}
}

func TestGradeStrokesRejectsOneBadInEight(t *testing.T) {
	medians := make([][][2]float64, 8)
	strokes := make([]*fantiv1.Stroke, 8)

	for i := range medians {
		medians[i] = horizontalMedian()
		strokes[i] = goodHorizontal()
	}

	// One wrong stroke makes the character attempt incorrect.
	strokes[3] = stroke([2]float32{25, 115}, [2]float32{25, 230})

	res := gradeStrokes(medians, strokes, canvas, canvas)
	if res.correct || !slices.Equal(res.bad, []int{4}) {
		t.Errorf("correct = %v bad = %v, want false/[4]", res.correct, res.bad)
	}
}

func TestGradeStrokesEmptyStrokeIsBad(t *testing.T) {
	res := gradeStrokes([][][2]float64{horizontalMedian(), horizontalMedian()},
		[]*fantiv1.Stroke{goodHorizontal(), stroke()}, canvas, canvas)

	// An incomplete stroke makes the character attempt incorrect.
	if res.correct || !slices.Equal(res.bad, []int{2}) {
		t.Errorf("correct = %v bad = %v, want false/[2]", res.correct, res.bad)
	}
}

func TestHandwritingFeedbackMentionsCounts(t *testing.T) {
	fb := handwritingFeedback("馬", handwritingResult{countMismatch: true, expected: 10, got: 7})

	for _, text := range []string{fb.GetEn(), fb.GetTc(), fb.GetSc()} {
		if text == "" {
			t.Fatal("feedback locale missing")
		}
	}

	if !strings.Contains(fb.GetEn(), "10") || !strings.Contains(fb.GetEn(), "7") {
		t.Errorf("feedback %q lacks the stroke counts", fb.GetEn())
	}
}
