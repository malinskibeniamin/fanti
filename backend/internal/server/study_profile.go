package server

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

const (
	// profileName is the singleton resource name.
	profileName = "studyProfile"
	// practiceWindowDays is the practice-calendar window returned.
	practiceWindowDays = 28
	// recordsLimit caps the learning-records feed, like the prototype's
	// records.slice(0, 30).
	recordsLimit = 30
)

//nolint:gochecknoglobals // static proto enum mappings
var goalName = map[fantiv1.Goal]string{
	fantiv1.Goal_GOAL_PRACTICAL: "practical",
	fantiv1.Goal_GOAL_EXAM:      "exam",
	fantiv1.Goal_GOAL_READING:   "reading",
}

//nolint:gochecknoglobals // static proto enum mappings
var goalEnum = map[string]fantiv1.Goal{
	"practical": fantiv1.Goal_GOAL_PRACTICAL,
	"exam":      fantiv1.Goal_GOAL_EXAM,
	"reading":   fantiv1.Goal_GOAL_READING,
}

//nolint:gochecknoglobals // static HSK-to-CEFR mapping from the design
var cefrByHSK = map[int32]string{1: "A1", 2: "A2", 3: "B1", 4: "B2", 5: "C1", 6: "C2"}

// coverageOf estimates everyday-speech coverage from the learned count.
// The design's coverage fact anchors the curve: the 140 most common
// characters cover about 50% of everyday speech and 1,000 cover about 90%.
// Interpolate linearly between those anchors, then taper towards 98% at
// the full 3,000-character course.
func coverageOf(learned int32) float64 {
	n := float64(learned)

	switch {
	case n <= 0:
		return 0
	case n <= 140:
		return 0.5 * n / 140
	case n <= 1000:
		return 0.5 + 0.4*(n-140)/860
	default:
		return math.Min(0.98, 0.9+0.08*(n-1000)/2000)
	}
}

// GetStudyProfile returns the singleton learner profile with progress,
// streaks, milestones, and exam readiness.
func (s *Study) GetStudyProfile(
	ctx context.Context, req *connect.Request[fantiv1.GetStudyProfileRequest],
) (*connect.Response[fantiv1.StudyProfile], error) {
	if req.Msg.GetName() != profileName {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("name must be %q, got %q", profileName, req.Msg.GetName())) //nolint:err113 // request detail
	}

	profile, err := s.studyProfile(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(profile), nil
}

// UpdateStudyProfile updates goal and mission.
func (s *Study) UpdateStudyProfile(
	ctx context.Context, req *connect.Request[fantiv1.UpdateStudyProfileRequest],
) (*connect.Response[fantiv1.StudyProfile], error) {
	if got := req.Msg.GetStudyProfile().GetName(); got != profileName {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("study_profile.name must be %q, got %q", profileName, got)) //nolint:err113 // request detail
	}

	paths := req.Msg.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errEmptyMask)
	}

	sets := []string{}

	var args []any

	arg := func(v any) string {
		args = append(args, v)

		return "$" + strconv.Itoa(len(args))
	}

	for _, p := range paths {
		switch p {
		case "goal":
			name, ok := goalName[req.Msg.GetStudyProfile().GetGoal()]
			if !ok {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("goal %v is not a known goal", req.Msg.GetStudyProfile().GetGoal())) //nolint:err113 // request detail
			}

			sets = append(sets, "goal = "+arg(name))
		case "mission":
			sets = append(sets, "mission = "+arg(req.Msg.GetStudyProfile().GetMission()))
		default:
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("update_mask path %q is not updatable", p)) //nolint:err113 // request detail
		}
	}

	if _, err := s.pool.Exec(ctx,
		"UPDATE study_profile SET "+strings.Join(sets, ", ")+" WHERE id", args...); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	profile, err := s.studyProfile(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(profile), nil
}

// studyProfile assembles the full profile resource.
func (s *Study) studyProfile(ctx context.Context) (*fantiv1.StudyProfile, error) {
	profile := &fantiv1.StudyProfile{Name: profileName, CourseSize: coreCurriculumSize}

	var goal string
	if err := s.pool.QueryRow(ctx,
		"SELECT goal, mission FROM study_profile WHERE id").
		Scan(&goal, &profile.Mission); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	profile.Goal = goalEnum[goal]

	var err error

	profile.CurriculumProgress, err = s.profileCurriculumProgress(ctx)
	if err != nil {
		return nil, err
	}

	profile.LearnedCount = profile.GetCurriculumProgress().GetCoreLearned()
	profile.Coverage = coverageOf(profile.GetLearnedCount())

	profile.Milestones, err = s.profileMilestones(ctx, profile.GetLearnedCount())
	if err != nil {
		return nil, err
	}

	profile.PracticeDays, err = s.profilePracticeDays(ctx)
	if err != nil {
		return nil, err
	}

	profile.Records, err = s.profileRecords(ctx)
	if err != nil {
		return nil, err
	}

	profile.ExamReadiness, err = s.profileExamReadiness(ctx)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (s *Study) profileCurriculumProgress(
	ctx context.Context,
) (*fantiv1.CurriculumProgress, error) {
	progress := &fantiv1.CurriculumProgress{}

	if err := s.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (
				WHERE c.catalog_kind = 'curriculum'
					AND c.curriculum_rank BETWEEN 1 AND $1
					AND COALESCE(r.learned, FALSE)
			),
			count(*) FILTER (
				WHERE c.catalog_kind = 'curriculum'
					AND c.curriculum_rank BETWEEN 1 AND $1
			),
			count(*) FILTER (
				WHERE c.catalog_kind = 'curriculum'
					AND COALESCE(r.learned, FALSE)
			),
			count(*) FILTER (WHERE c.catalog_kind = 'curriculum'),
			count(*) FILTER (
				WHERE c.catalog_kind = 'reference'
					AND COALESCE(r.learned, FALSE)
			),
			count(*) FILTER (WHERE c.catalog_kind = 'reference')
		FROM characters c
		LEFT JOIN reviews r ON r.ch = c.traditional`,
		coreCurriculumSize,
	).Scan(
		&progress.CoreLearned,
		&progress.CoreSize,
		&progress.CompleteLearned,
		&progress.CompleteSize,
		&progress.ReferenceLearned,
		&progress.ReferenceSize,
	); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return progress, nil
}

func (s *Study) profileMilestones(ctx context.Context, learned int32) ([]*fantiv1.Milestone, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT threshold, label_en, label_tc, label_sc FROM milestones ORDER BY threshold")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var milestones []*fantiv1.Milestone

	for rows.Next() {
		var (
			m     fantiv1.Milestone
			label fantiv1.LocalizedText
		)

		if err := rows.Scan(&m.Threshold, &label.En, &label.Tc, &label.Sc); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		m.Label = &label
		m.Reached = learned >= m.GetThreshold()
		milestones = append(milestones, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return milestones, nil
}

func (s *Study) profilePracticeDays(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		"SELECT day FROM practice_days WHERE day > CURRENT_DATE - $1::int ORDER BY day",
		practiceWindowDays)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var days []string

	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		days = append(days, day.Format("2006-01-02"))
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return days, nil
}

func (s *Study) profileRecords(ctx context.Context) ([]*fantiv1.LearningRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT record_type, ch, milestone_threshold, record_time
		FROM learning_records ORDER BY record_time DESC, id DESC LIMIT $1`, recordsLimit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var records []*fantiv1.LearningRecord

	for rows.Next() {
		var (
			r  fantiv1.LearningRecord
			ts time.Time
		)

		if err := rows.Scan(&r.Type, &r.Character, &r.MilestoneThreshold, &ts); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		r.RecordTime = timestamppb.New(ts)
		records = append(records, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return records, nil
}

// profileExamReadiness reports per-HSK-level progress: the share of that
// level's characters marked learned, labelled with the design's CEFR map.
func (s *Study) profileExamReadiness(ctx context.Context) ([]*fantiv1.ExamReadiness, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.hsk_level, count(*),
			count(*) FILTER (WHERE COALESCE(r.learned, FALSE))
		FROM characters c LEFT JOIN reviews r ON r.ch = c.traditional
		WHERE c.hsk_level > 0
		GROUP BY c.hsk_level ORDER BY c.hsk_level`)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var levels []*fantiv1.ExamReadiness

	for rows.Next() {
		var (
			level          int32
			total, learned float64
		)

		if err := rows.Scan(&level, &total, &learned); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		label := "HSK " + strconv.Itoa(int(level))
		if cefr, ok := cefrByHSK[level]; ok {
			label += " · " + cefr
		}

		levels = append(levels, &fantiv1.ExamReadiness{
			Level:    label,
			Progress: learned / total,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return levels, nil
}
