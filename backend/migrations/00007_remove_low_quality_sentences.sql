-- +goose Up

-- Tatoeba wordplay fragments translated as "Love loves love"; neither is a
-- useful grammatical example for learners.
DELETE FROM sentences WHERE id IN (1531795, 1531796);

-- +goose Down

-- Data-quality removals are intentionally irreversible.
SELECT 1;
