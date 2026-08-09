-- +goose Up

ALTER TABLE reviews
    ADD COLUMN practice_difficulty SMALLINT NOT NULL DEFAULT 1
        CHECK (practice_difficulty BETWEEN 1 AND 3);

CREATE TABLE handwriting_attempts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ch TEXT NOT NULL,
    difficulty SMALLINT NOT NULL CHECK (difficulty BETWEEN 1 AND 3),
    hint_used BOOLEAN NOT NULL DEFAULT FALSE,
    correct BOOLEAN NOT NULL,
    create_time TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX handwriting_attempts_ch_time_idx
    ON handwriting_attempts (ch, create_time DESC);

-- +goose Down

DROP TABLE handwriting_attempts;

ALTER TABLE reviews DROP COLUMN practice_difficulty;
