package server

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

// Study serves fanti.v1.StudyService.
type Study struct {
	pool *pgxpool.Pool
	// newRand seeds quiz shuffling; tests inject a fixed seed.
	newRand func() *rand.Rand
}

// NewStudy builds the study service.
func NewStudy(pool *pgxpool.Pool) *Study {
	return &Study{
		pool: pool,
		newRand: func() *rand.Rand {
			//nolint:gosec // quiz shuffling needs no cryptographic randomness
			return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		},
	}
}

// queryRower is the read surface shared by pgxpool.Pool and pgx.Tx.
type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// againDelay is the retry delay after a failed recall.
const againDelay = 10 * time.Minute

var errGradeRequired = errors.New("grade must be AGAIN, GOOD, or EASY")

// nextReview ports the prototype's scheduleReview: the interval ladder is
// 1/3/7/14/30/60 days. AGAIN resets to level 0 and retries in 10 minutes;
// GOOD advances one level and EASY two, capped at the top. Unseen cards
// start at level -1 before advancing.
func nextReview(seen bool, level int32, grade fantiv1.Grade, now time.Time) (int32, time.Time, error) {
	intervals := [...]int32{1, 3, 7, 14, 30, 60}

	const topLevel = int32(len(intervals) - 1)

	var step int32

	switch grade {
	case fantiv1.Grade_GRADE_AGAIN:
		return 0, now.Add(againDelay), nil
	case fantiv1.Grade_GRADE_GOOD:
		step = 1
	case fantiv1.Grade_GRADE_EASY:
		step = 2
	case fantiv1.Grade_GRADE_UNSPECIFIED:
	}

	if step == 0 {
		return 0, time.Time{}, connect.NewError(connect.CodeInvalidArgument, errGradeRequired)
	}

	cur := int32(-1)
	if seen {
		cur = level
	}

	lv := max(min(cur+step, topLevel), 0)

	return lv, now.Add(time.Duration(intervals[lv]) * 24 * time.Hour), nil
}

// deckWhere selects the active character deck: starter characters plus
// anything explicitly added to the memory bank.
const deckWhere = " WHERE (c.starter_deck OR COALESCE(r.in_deck, FALSE)) "

const dueWhere = " AND (r.due_time IS NULL OR r.due_time <= now()) "

// dueOrder puts never-reviewed cards first (the prototype treats a missing
// review as due since forever), then the earliest due dates.
const dueOrder = " ORDER BY r.due_time ASC NULLS FIRST, c.frequency_rank = 0, c.frequency_rank, c.traditional "

// ListDueCards lists the review deck ordered by due date.
func (s *Study) ListDueCards(
	ctx context.Context, req *connect.Request[fantiv1.ListDueCardsRequest],
) (*connect.Response[fantiv1.ListDueCardsResponse], error) {
	size := req.Msg.GetPageSize()
	if size <= 0 || size > 200 {
		size = defaultPageSize
	}

	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}

	var (
		cards    []*fantiv1.DueCard
		dueCount int32
	)

	if req.Msg.GetMode() == fantiv1.CardMode_CARD_MODE_WORD {
		cards, dueCount, err = s.listDueWords(ctx, int(size), offset)
	} else {
		cards, dueCount, err = s.listDueCharacters(ctx, int(size), offset)
	}

	if err != nil {
		return nil, err
	}

	resp := &fantiv1.ListDueCardsResponse{DueCards: cards, DueCount: dueCount}

	if len(resp.GetDueCards()) > int(size) {
		resp.DueCards = resp.GetDueCards()[:size]
		resp.NextPageToken = encodePageToken(offset + int(size))
	}

	return connect.NewResponse(resp), nil
}

func (s *Study) listDueCharacters(ctx context.Context, size, offset int) ([]*fantiv1.DueCard, int32, error) {
	var dueCount int32
	if err := s.pool.QueryRow(ctx,
		"SELECT count(*)"+characterFrom+deckWhere+dueWhere).Scan(&dueCount); err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	sql := "SELECT" + characterColumns + characterFrom + deckWhere + dueWhere + dueOrder +
		" LIMIT " + strconv.Itoa(size+1) + " OFFSET " + strconv.Itoa(offset)

	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var chars []*fantiv1.Character

	for rows.Next() {
		ch, err := scanCharacter(rows)
		if err != nil {
			return nil, 0, connect.NewError(connect.CodeInternal, err)
		}

		chars = append(chars, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	if err := NewDictionary(s.pool).fillCharacterSourceMetadata(ctx, chars); err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	ids := make([]string, 0, len(chars))
	for _, ch := range chars {
		ids = append(ids, ch.GetTraditional())
	}

	reviews, err := s.reviewsFor(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	cards := make([]*fantiv1.DueCard, 0, len(chars))

	for _, ch := range chars {
		review := reviews[ch.GetTraditional()]
		if review == nil {
			review = &fantiv1.Review{
				Name:               "reviews/" + ch.GetTraditional(),
				PracticeDifficulty: fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED,
			}
		}

		cards = append(cards, &fantiv1.DueCard{Character: ch, Review: review})
	}

	return cards, dueCount, nil
}

func (s *Study) listDueWords(ctx context.Context, size, offset int) ([]*fantiv1.DueCard, int32, error) {
	var dueCount int32
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM word_cards w LEFT JOIN reviews r ON r.ch = w.word
		WHERE r.due_time IS NULL OR r.due_time <= now()`).Scan(&dueCount); err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT w.word, w.pinyin, w.pos, w.meaning, w.simplified, w.traditional, w.story,
			r.ch IS NOT NULL, COALESCE(r.srs_level, 0), COALESCE(r.due_time, now()),
			COALESCE(r.mistake_count, 0), COALESCE(r.learned, FALSE),
			COALESCE(r.practice_difficulty, 1)
		FROM word_cards w LEFT JOIN reviews r ON r.ch = w.word
		WHERE r.due_time IS NULL OR r.due_time <= now()
		ORDER BY r.due_time ASC NULLS FIRST, w.word
		LIMIT $1 OFFSET $2`, size+1, offset)
	if err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var cards []*fantiv1.DueCard

	for rows.Next() {
		var (
			word, trad string
			ch         fantiv1.Character
			hasReview  bool
			level, mc  int32
			due        time.Time
			learned    bool
			difficulty fantiv1.PracticeDifficulty
		)

		if err := rows.Scan(&word, &ch.Pinyin, &ch.Pos, &ch.Meaning, &ch.Simplified,
			&trad, &ch.Story, &hasReview, &level, &due, &mc, &learned, &difficulty); err != nil {
			return nil, 0, connect.NewError(connect.CodeInternal, err)
		}

		if trad == "" {
			trad = word
		}

		ch.Name = "characters/" + word
		ch.Traditional = trad
		ch.Learned = learned
		ch.MistakeCount = mc

		review := &fantiv1.Review{
			Name:               "reviews/" + word,
			PracticeDifficulty: difficulty,
		}
		if hasReview {
			review.SrsLevel = level
			review.DueTime = timestamppb.New(due)
			review.MistakeCount = mc
			review.Learned = learned
		}

		cards = append(cards, &fantiv1.DueCard{Character: &ch, Review: review})
	}

	if err := rows.Err(); err != nil {
		return nil, 0, connect.NewError(connect.CodeInternal, err)
	}

	return cards, dueCount, nil
}

func (s *Study) reviewsFor(ctx context.Context, chars []string) (map[string]*fantiv1.Review, error) {
	reviews := make(map[string]*fantiv1.Review, len(chars))
	if len(chars) == 0 {
		return reviews, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT ch, srs_level, due_time, mistake_count, learned, practice_difficulty
		FROM reviews WHERE ch = ANY($1)`, chars)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			ch         string
			level, mc  int32
			due        time.Time
			learned    bool
			difficulty fantiv1.PracticeDifficulty
		)

		if err := rows.Scan(&ch, &level, &due, &mc, &learned, &difficulty); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		reviews[ch] = &fantiv1.Review{
			Name:               "reviews/" + ch,
			SrsLevel:           level,
			DueTime:            timestamppb.New(due),
			MistakeCount:       mc,
			Learned:            learned,
			PracticeDifficulty: difficulty,
		}
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return reviews, nil
}

// GetReview returns one character's review state and handwriting preference.
// Known characters without a review row receive the guided default.
func (s *Study) GetReview(
	ctx context.Context, req *connect.Request[fantiv1.GetReviewRequest],
) (*connect.Response[fantiv1.Review], error) {
	ch, err := parseName("reviews", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	review := &fantiv1.Review{Name: "reviews/" + ch}

	var due time.Time
	err = s.pool.QueryRow(ctx, `
		SELECT srs_level, due_time, mistake_count, learned, practice_difficulty
		FROM reviews WHERE ch = $1`, ch).
		Scan(&review.SrsLevel, &due, &review.MistakeCount, &review.Learned,
			&review.PracticeDifficulty)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := s.requireCard(ctx, s.pool, ch); err != nil {
			return nil, err
		}

		review.PracticeDifficulty = fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED

		return connect.NewResponse(review), nil
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	review.DueTime = timestamppb.New(due)

	return connect.NewResponse(review), nil
}

// UpdateReview updates learner-controlled review fields.
func (s *Study) UpdateReview(
	ctx context.Context, req *connect.Request[fantiv1.UpdateReviewRequest],
) (*connect.Response[fantiv1.Review], error) {
	review := req.Msg.GetReview()
	ch, err := parseName("reviews", review.GetName())
	if err != nil {
		return nil, err
	}

	paths := req.Msg.GetUpdateMask().GetPaths()
	if len(paths) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errEmptyMask)
	}
	for _, path := range paths {
		if path != "practice_difficulty" {
			return nil, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("update_mask path %q is not updatable", path)) //nolint:err113 // request detail
		}
	}

	difficulty := review.GetPracticeDifficulty()
	if difficulty < fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED ||
		difficulty > fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_MASTERY {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("practice_difficulty %v is not known", difficulty)) //nolint:err113 // request detail
	}

	if err := s.requireCard(ctx, s.pool, ch); err != nil {
		return nil, err
	}

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO reviews (ch, practice_difficulty)
		VALUES ($1, $2)
		ON CONFLICT (ch) DO UPDATE
		SET practice_difficulty = EXCLUDED.practice_difficulty`,
		ch, difficulty); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return s.GetReview(ctx, connect.NewRequest(&fantiv1.GetReviewRequest{
		Name: review.GetName(),
	}))
}

// GradeCard applies an SRS grade: reschedules the card, marks today
// practiced, and on GOOD/EASY records the character as learned.
func (s *Study) GradeCard(
	ctx context.Context, req *connect.Request[fantiv1.GradeCardRequest],
) (*connect.Response[fantiv1.Review], error) {
	ch, err := parseName("reviews", req.Msg.GetName())
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var (
		level, mistakes int32
		learned         bool
		difficulty      = fantiv1.PracticeDifficulty_PRACTICE_DIFFICULTY_GUIDED
	)

	seen := true

	err = tx.QueryRow(ctx,
		`SELECT srs_level, mistake_count, learned, practice_difficulty
		FROM reviews WHERE ch = $1 FOR UPDATE`, ch).
		Scan(&level, &mistakes, &learned, &difficulty)
	if errors.Is(err, pgx.ErrNoRows) {
		seen = false

		if err := s.requireCard(ctx, tx, ch); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	newLevel, due, err := nextReview(seen, level, req.Msg.GetGrade(), time.Now())
	if err != nil {
		return nil, err
	}

	again := req.Msg.GetGrade() == fantiv1.Grade_GRADE_AGAIN
	if again {
		mistakes++
	}

	newLearned := learned || !again

	if _, err := tx.Exec(ctx, `
		INSERT INTO reviews (ch, srs_level, due_time, mistake_count, learned, in_deck)
		VALUES ($1, $2, $3, $4, $5, TRUE)
		ON CONFLICT (ch) DO UPDATE SET
			srs_level = EXCLUDED.srs_level, due_time = EXCLUDED.due_time,
			mistake_count = EXCLUDED.mistake_count, learned = EXCLUDED.learned,
			in_deck = TRUE`,
		ch, newLevel, due, mistakes, newLearned); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if newLearned && !learned {
		if err := recordLearned(ctx, tx, ch); err != nil {
			return nil, err
		}
	}

	if err := markPractice(ctx, tx); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fantiv1.Review{
		Name:               "reviews/" + ch,
		SrsLevel:           newLevel,
		DueTime:            timestamppb.New(due),
		MistakeCount:       mistakes,
		Learned:            newLearned,
		PracticeDifficulty: difficulty,
	}), nil
}

// MarkLearned marks a character learned and keeps it in the deck.
func (s *Study) MarkLearned(
	ctx context.Context, req *connect.Request[fantiv1.MarkLearnedRequest],
) (*connect.Response[fantiv1.Review], error) {
	return s.upsertDeckReview(ctx, req.Msg.GetCharacter(), true)
}

// AddToDeck adds a discovered character to the active deck.
func (s *Study) AddToDeck(
	ctx context.Context, req *connect.Request[fantiv1.AddToDeckRequest],
) (*connect.Response[fantiv1.Review], error) {
	return s.upsertDeckReview(ctx, req.Msg.GetCharacter(), false)
}

// upsertDeckReview puts a card in the deck, optionally marking it learned
// (with learning-record and milestone bookkeeping on the first time).
func (s *Study) upsertDeckReview(
	ctx context.Context, name string, markLearned bool,
) (*connect.Response[fantiv1.Review], error) {
	ch, err := parseName("characters", name)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	defer func() { _ = tx.Rollback(ctx) }()

	var wasLearned bool

	err = tx.QueryRow(ctx, "SELECT learned FROM reviews WHERE ch = $1 FOR UPDATE", ch).
		Scan(&wasLearned)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := s.requireCard(ctx, tx, ch); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO reviews (ch, learned, in_deck) VALUES ($1, $2, TRUE)
		ON CONFLICT (ch) DO UPDATE SET
			learned = reviews.learned OR EXCLUDED.learned, in_deck = TRUE`,
		ch, markLearned); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if markLearned && !wasLearned {
		if err := recordLearned(ctx, tx, ch); err != nil {
			return nil, err
		}
	}

	review := &fantiv1.Review{Name: "reviews/" + ch}

	var due time.Time

	err = tx.QueryRow(ctx,
		`SELECT srs_level, due_time, mistake_count, learned, practice_difficulty
		FROM reviews WHERE ch = $1`, ch).
		Scan(&review.SrsLevel, &due, &review.MistakeCount, &review.Learned,
			&review.PracticeDifficulty)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	review.DueTime = timestamppb.New(due)

	if err := tx.Commit(ctx); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(review), nil
}

// requireCard checks the id names a known character or word card.
func (s *Study) requireCard(ctx context.Context, q queryRower, ch string) error {
	var exists bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM characters WHERE traditional = $1)
			OR EXISTS (SELECT 1 FROM word_cards WHERE word = $1)`, ch).Scan(&exists); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if !exists {
		return connect.NewError(connect.CodeNotFound,
			fmt.Errorf("card %q not found", ch)) //nolint:err113 // request detail
	}

	return nil
}

// recordLearned appends a "learned" learning record and, when the learned
// count lands exactly on a milestone threshold, a "milestone" record too.
// The caller must have already set learned = TRUE on the review row.
func recordLearned(ctx context.Context, tx pgx.Tx, ch string) error {
	if _, err := tx.Exec(ctx,
		"INSERT INTO learning_records (record_type, ch) VALUES ('learned', $1)", ch); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*)
		FROM reviews r
		JOIN characters c ON c.traditional = r.ch
		WHERE r.learned
			AND c.catalog_kind = 'curriculum'
			AND c.curriculum_rank BETWEEN 1 AND $1`,
		coreCurriculumSize,
	).Scan(&count); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	var threshold int

	err := tx.QueryRow(ctx,
		"SELECT threshold FROM milestones WHERE threshold = $1", count).Scan(&threshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}

	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO learning_records (record_type, milestone_threshold)
		VALUES ('milestone', $1)`, threshold); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
}

// markPractice marks today practiced (idempotent).
func markPractice(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx,
		"INSERT INTO practice_days (day) VALUES (CURRENT_DATE) ON CONFLICT DO NOTHING"); err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
}

// RecordPractice marks a local date practiced (idempotent per day).
func (s *Study) RecordPractice(
	ctx context.Context, req *connect.Request[fantiv1.RecordPracticeRequest],
) (*connect.Response[fantiv1.StudyProfile], error) {
	day, err := time.Parse("2006-01-02", req.Msg.GetDay())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("day must be YYYY-MM-DD: %w", err))
	}

	if _, err := s.pool.Exec(ctx,
		"INSERT INTO practice_days (day) VALUES ($1) ON CONFLICT DO NOTHING", day); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	profile, err := s.studyProfile(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(profile), nil
}

var errClozeInvalid = errors.New("cloze card needs a character and a sentence containing it")

// CreateClozeCard saves a sentence mined from reading. Duplicate
// (character, sentence) pairs return the existing card.
func (s *Study) CreateClozeCard(
	ctx context.Context, req *connect.Request[fantiv1.CreateClozeCardRequest],
) (*connect.Response[fantiv1.ClozeCard], error) {
	card := req.Msg.GetClozeCard()

	ch := card.GetCharacter()
	sentence := card.GetSentence()

	if ch == "" || sentence == "" || !strings.Contains(sentence, ch) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errClozeInvalid)
	}

	var id int64
	if err := s.pool.QueryRow(ctx, `
		INSERT INTO cloze_cards (ch, sentence) VALUES ($1, $2)
		ON CONFLICT (ch, sentence) DO UPDATE SET ch = EXCLUDED.ch
		RETURNING id`, ch, sentence).Scan(&id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&fantiv1.ClozeCard{
		Name:      "clozeCards/" + strconv.FormatInt(id, 10),
		Character: ch,
		Sentence:  sentence,
	}), nil
}

// ListClozeCards lists saved cloze cards.
func (s *Study) ListClozeCards(
	ctx context.Context, req *connect.Request[fantiv1.ListClozeCardsRequest],
) (*connect.Response[fantiv1.ListClozeCardsResponse], error) {
	size := req.Msg.GetPageSize()
	if size <= 0 || size > 200 {
		size = defaultPageSize
	}

	offset, err := decodePageToken(req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}

	rows, err := s.pool.Query(ctx,
		"SELECT id, ch, sentence FROM cloze_cards ORDER BY id LIMIT $1 OFFSET $2",
		size+1, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	resp := &fantiv1.ListClozeCardsResponse{}

	for rows.Next() {
		var (
			id   int64
			card fantiv1.ClozeCard
		)

		if err := rows.Scan(&id, &card.Character, &card.Sentence); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		card.Name = "clozeCards/" + strconv.FormatInt(id, 10)
		resp.ClozeCards = append(resp.ClozeCards, &card)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if len(resp.GetClozeCards()) > int(size) {
		resp.ClozeCards = resp.GetClozeCards()[:size]
		resp.NextPageToken = encodePageToken(offset + int(size))
	}

	return connect.NewResponse(resp), nil
}

// GetLesson assembles today's lesson: the weakest characters to warm up on
// plus the next unlearned automatic-curriculum entry.
func (s *Study) GetLesson(
	ctx context.Context, _ *connect.Request[fantiv1.GetLessonRequest],
) (*connect.Response[fantiv1.Lesson], error) {
	rows, err := s.pool.Query(ctx,
		"SELECT"+characterColumns+characterFrom+
			`WHERE COALESCE(r.mistake_count, 0) > 0
			ORDER BY r.mistake_count DESC, c.traditional LIMIT 2`)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	lesson := &fantiv1.Lesson{}
	weak := []string{}

	for rows.Next() {
		ch, err := scanCharacter(rows)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		lesson.WeakCharacters = append(lesson.WeakCharacters, ch)
		weak = append(weak, ch.GetTraditional())
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	next, err := scanCharacter(s.pool.QueryRow(ctx,
		"SELECT"+characterColumns+characterFrom+
			`WHERE c.catalog_kind = 'curriculum'
				AND NOT COALESCE(r.learned, FALSE)
				AND NOT (c.traditional = ANY($1))
			ORDER BY c.curriculum_rank = 0, c.curriculum_rank, c.traditional
			LIMIT 1`,
		weak))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Everything is learned — no next character.
	case err != nil:
		return nil, connect.NewError(connect.CodeInternal, err)
	default:
		lesson.NextCharacter = next
	}

	return connect.NewResponse(lesson), nil
}
