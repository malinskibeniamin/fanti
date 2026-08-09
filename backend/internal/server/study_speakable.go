package server

import (
	"context"

	"connectrpc.com/connect"

	fantiv1 "github.com/malinskibeniamin/fanti/backend/gen/fanti/v1"
)

const (
	// speakableSampleLimit caps the unlocked sentences shown on the card.
	speakableSampleLimit = 3
	// speakableAlmostLimit caps the nearest-unlockable teasers.
	speakableAlmostLimit = 2
)

// learnedCharsCTE gathers the learned set as one array; COALESCE keeps
// the zero-learned case a real empty array rather than NULL (`x <@ NULL`
// is NULL, which would silently drop every row).
const learnedCharsCTE = `WITH learned AS (
	SELECT COALESCE(array_agg(ch), '{}') AS chars FROM reviews WHERE learned
)`

// GetSpeakableSummary reports what the learned characters unlock in the
// sentence corpus: counts, the easiest unlocked sentences, and the
// nearest locked ones with their missing characters.
func (s *Study) GetSpeakableSummary(
	ctx context.Context, _ *connect.Request[fantiv1.GetSpeakableSummaryRequest],
) (*connect.Response[fantiv1.GetSpeakableSummaryResponse], error) {
	res := &fantiv1.GetSpeakableSummaryResponse{}

	// char_count > 0 everywhere: '{}' <@ anything is true, so zero-Han
	// rows would count as "speakable" and inflate both counts. Ambiguous
	// rows (risky script conversion) never reach learners, and only
	// in-course sentences count — progress toward sentences containing
	// characters the app cannot teach would be unreachable.
	if err := s.pool.QueryRow(ctx, learnedCharsCTE+`
		SELECT count(*) FILTER (WHERE s.chars <@ l.chars), count(*)
		FROM sentences s CROSS JOIN learned l
		WHERE s.char_count > 0 AND NOT s.ambiguous AND s.in_course`).Scan(
		&res.UnlockedCount, &res.TotalCount); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var err error

	res.Sentences, err = s.speakableUnlocked(ctx)
	if err != nil {
		return nil, err
	}

	res.AlmostUnlocked, err = s.speakableAlmost(ctx)
	if err != nil {
		return nil, err
	}

	res.Topics, err = s.speakableTopics(ctx)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(res), nil
}

// speakableUnlocked samples fully-unlocked sentences, easiest first:
// common-character sentences (ranked, low max_freq_rank), then shorter,
// then oldest — deterministic throughout.
func (s *Study) speakableUnlocked(ctx context.Context) ([]*fantiv1.SpeakableSentence, error) {
	rows, err := s.pool.Query(ctx, learnedCharsCTE+`
		SELECT s.id, s.traditional, s.english
		FROM sentences s CROSS JOIN learned l
		WHERE s.char_count > 0 AND NOT s.ambiguous AND s.in_course AND s.chars <@ l.chars
		ORDER BY (s.max_freq_rank = 0), s.max_freq_rank, s.char_count, s.id
		LIMIT $1`, speakableSampleLimit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var sentences []*fantiv1.SpeakableSentence

	for rows.Next() {
		var sentence fantiv1.SpeakableSentence
		if err := rows.Scan(&sentence.Id, &sentence.Traditional, &sentence.English); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		sentences = append(sentences, &sentence)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return sentences, nil
}

// speakableAlmost finds the nearest locked sentences: fewest missing
// characters first, easier sentences breaking ties. The missing
// characters keep sentence order, so the learner reads them in context.
func (s *Study) speakableAlmost(ctx context.Context) ([]*fantiv1.SpeakableSentence, error) {
	rows, err := s.pool.Query(ctx, learnedCharsCTE+`
		SELECT t.id, t.traditional, t.english, t.missing
		FROM (
			SELECT s.id, s.traditional, s.english, s.max_freq_rank, s.char_count,
				ARRAY(SELECT c FROM unnest(s.chars) AS c WHERE NOT c = ANY (l.chars)) AS missing
			FROM sentences s CROSS JOIN learned l
			WHERE s.char_count > 0 AND NOT s.ambiguous AND s.in_course AND NOT (s.chars <@ l.chars)
		) t
		ORDER BY cardinality(t.missing), (t.max_freq_rank = 0), t.max_freq_rank, t.char_count, t.id
		LIMIT $1`, speakableAlmostLimit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var sentences []*fantiv1.SpeakableSentence

	for rows.Next() {
		var sentence fantiv1.SpeakableSentence
		if err := rows.Scan(&sentence.Id, &sentence.Traditional, &sentence.English,
			&sentence.MissingCharacters); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		sentences = append(sentences, &sentence)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return sentences, nil
}

// speakableTopics lists the topics the learned characters cover.
func (s *Study) speakableTopics(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT t.topic
		FROM characters c
		JOIN reviews r ON r.ch = c.traditional AND r.learned
		CROSS JOIN LATERAL unnest(c.topics) AS t(topic)
		ORDER BY t.topic`)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer rows.Close()

	var topics []string

	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		topics = append(topics, topic)
	}

	if err := rows.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return topics, nil
}
