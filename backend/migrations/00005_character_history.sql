-- +goose Up

-- Attested character forms from oldest to newest. A NULL svg records an
-- honest source gap; the regular row is a sentinel rendered from the modern
-- character already returned by GetCharacter.
CREATE TABLE character_history (
    ch TEXT NOT NULL REFERENCES characters (traditional) ON DELETE CASCADE,
    stage TEXT NOT NULL CHECK (stage IN ('oracle', 'bronze', 'seal', 'clerical', 'regular')),
    stage_order SMALLINT NOT NULL CHECK (stage_order BETWEEN 1 AND 5),
    svg BYTEA,
    source_title TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    source_sha1 TEXT NOT NULL DEFAULT '',
    license TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ch, stage),
    UNIQUE (ch, stage_order)
);

-- +goose Down

DROP TABLE character_history;
