-- +goose Up

CREATE TABLE seed_runs (
    name TEXT PRIMARY KEY,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE seed_runs;
