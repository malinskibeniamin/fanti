-- +goose Up

-- Full per-character hanzi-writer JSON (stroke outlines + medians), the
-- shape the hanzi-writer renderer consumes for animation and quizzes.
-- Earlier seeds kept only the medians; rows from before this migration
-- carry NULL until the seed backfills them.
ALTER TABLE stroke_data ADD COLUMN data JSONB;

-- +goose Down

ALTER TABLE stroke_data DROP COLUMN data;
