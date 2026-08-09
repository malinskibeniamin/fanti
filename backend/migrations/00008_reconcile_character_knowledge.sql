-- +goose Up

-- Main and PR #17 briefly assigned different schemas to migration version 5.
-- Reconcile both histories: main databases already have character_history,
-- while databases created from the PR branch already have radical_parts.
CREATE TABLE IF NOT EXISTS character_history (
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

ALTER TABLE stroke_data
    ADD COLUMN IF NOT EXISTS radical_parts JSONB NOT NULL DEFAULT '[]';

-- +goose Down

-- Migration 5 owns character_history on the canonical main lineage.
ALTER TABLE stroke_data DROP COLUMN IF EXISTS radical_parts;
