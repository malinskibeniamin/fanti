-- +goose Up

ALTER TABLE characters
    ADD COLUMN catalog_kind TEXT NOT NULL DEFAULT 'curriculum'
        CHECK (catalog_kind IN ('curriculum', 'reference')),
    ADD COLUMN curriculum_rank INT NOT NULL DEFAULT 0
        CHECK (curriculum_rank >= 0);

CREATE INDEX characters_catalog_order_idx
    ON characters (catalog_kind, curriculum_rank, traditional);

-- +goose Down

DROP INDEX characters_catalog_order_idx;

ALTER TABLE characters
    DROP COLUMN curriculum_rank,
    DROP COLUMN catalog_kind;
