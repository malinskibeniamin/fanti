package server

import (
	"testing"
	"time"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

// TestNextReview pins the SRS ladder to the prototype's scheduleReview:
// AGAIN resets to level 0 with a 10-minute retry; GOOD steps one level and
// EASY two from the current level (-1 when unseen), capped at level 5.
func TestNextReview(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	tests := []struct {
		name      string
		seen      bool
		level     int32
		grade     fantiv1.Grade
		wantLevel int32
		wantDue   time.Time
	}{
		{"unseen good starts the ladder", false, 0, fantiv1.Grade_GRADE_GOOD, 0, now.Add(1 * day)},
		{"unseen easy skips to level 1", false, 0, fantiv1.Grade_GRADE_EASY, 1, now.Add(3 * day)},
		{"good advances one level", true, 0, fantiv1.Grade_GRADE_GOOD, 1, now.Add(3 * day)},
		{"easy advances two levels", true, 1, fantiv1.Grade_GRADE_EASY, 3, now.Add(14 * day)},
		{"easy caps at the top", true, 4, fantiv1.Grade_GRADE_EASY, 5, now.Add(60 * day)},
		{"good holds at the top", true, 5, fantiv1.Grade_GRADE_GOOD, 5, now.Add(60 * day)},
		{"again resets to level 0", true, 3, fantiv1.Grade_GRADE_AGAIN, 0, now.Add(10 * time.Minute)},
		{"again on unseen retries soon", false, 0, fantiv1.Grade_GRADE_AGAIN, 0, now.Add(10 * time.Minute)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, due, err := nextReview(tt.seen, tt.level, tt.grade, now)
			if err != nil {
				t.Fatalf("nextReview: %v", err)
			}

			if level != tt.wantLevel || !due.Equal(tt.wantDue) {
				t.Errorf("nextReview = level %d due %v, want level %d due %v",
					level, due, tt.wantLevel, tt.wantDue)
			}
		})
	}
}

func TestNextReviewRejectsUnspecifiedGrade(t *testing.T) {
	_, _, err := nextReview(true, 2, fantiv1.Grade_GRADE_UNSPECIFIED, time.Now())
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", connect.CodeOf(err))
	}
}

// TestCoverageOf pins the coverage curve to the design's anchors: 140
// characters cover about half of everyday speech and 1,000 about 90%.
func TestCoverageOf(t *testing.T) {
	tests := []struct {
		learned int32
		want    float64
	}{
		{0, 0},
		{70, 0.25},
		{140, 0.5},
		{570, 0.7},
		{1000, 0.9},
		{3000, 0.98},
		{9999, 0.98},
	}

	for _, tt := range tests {
		got := coverageOf(tt.learned)
		if diff := got - tt.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("coverageOf(%d) = %v, want %v", tt.learned, got, tt.want)
		}
	}

	if coverageOf(5) >= coverageOf(6) {
		t.Error("coverage must grow with learned count")
	}
}
