-- +goose Up
-- pg_trgm provides trigram similarity (similarity(), the % operator and GIN index
-- support). The typo-tolerant product search is built on it, so it must exist
-- before the products table and its search index are created.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- +goose Down
DROP EXTENSION IF EXISTS pg_trgm;
