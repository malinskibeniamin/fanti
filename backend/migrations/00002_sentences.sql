-- +goose Up

-- Real-corpus example sentences (Tatoeba, CC BY 2.0 FR — see NOTICES.md).
-- id is the Tatoeba Mandarin sentence id, kept for per-row attribution.
CREATE TABLE sentences (
    id BIGINT PRIMARY KEY,
    traditional TEXT NOT NULL,
    simplified TEXT NOT NULL,
    english TEXT NOT NULL,
    -- Distinct Han characters (traditional forms) the sentence uses;
    -- the GIN index makes "fully composed of learned characters" a cheap
    -- chars <@ learned_set containment query.
    chars TEXT[] NOT NULL DEFAULT '{}',
    char_count INT NOT NULL DEFAULT 0,
    -- Frequency rank of the sentence's rarest character (0 = unranked);
    -- lower means the sentence sticks to common characters.
    max_freq_rank INT NOT NULL DEFAULT 0,
    -- TRUE when the traditional text came from a simplified→traditional
    -- conversion crossing a one-to-many character mapping (乾/幹/干 class)
    -- that OpenCC may have resolved wrong, or still carries convertible
    -- simplified glyphs (mixed-script source). Such sentences are never
    -- shown to learners, only stored.
    ambiguous BOOLEAN NOT NULL DEFAULT FALSE,
    -- TRUE when every character has a characters-table row, so unlock
    -- progress only counts reachable sentences and missing-character
    -- links always resolve to a real page.
    in_course BOOLEAN NOT NULL DEFAULT FALSE,
    source TEXT NOT NULL DEFAULT 'tatoeba'
);

CREATE INDEX sentences_chars_idx ON sentences USING GIN (chars);
CREATE INDEX sentences_char_count_idx ON sentences (char_count);

-- +goose Down

DROP INDEX sentences_char_count_idx;
DROP INDEX sentences_chars_idx;
DROP TABLE sentences;
