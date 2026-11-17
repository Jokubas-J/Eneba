-- +goose Up

-- Visitor searches become public suggestions only when the same query was searched
-- on at least two different days (see docs/search.md, "Query suggestions"). These
-- columns count the days: days_searched goes up at most once per calendar day.
ALTER TABLE search_suggestions
    ADD COLUMN days_searched    integer NOT NULL DEFAULT 0 CHECK (days_searched >= 0),
    ADD COLUMN last_searched_on date;

-- Rows recorded before this migration: one day each (their searches cannot be told
-- apart by day any more).
UPDATE search_suggestions
SET days_searched = 1, last_searched_on = last_searched_at::date
WHERE search_count > 0;

-- A curated suggestion that only works now that accents are handled (00006).
INSERT INTO search_suggestions (query, source) VALUES ('god of war ragnarok', 'curated')
ON CONFLICT (query) DO UPDATE SET source = 'curated';

-- +goose Down
DELETE FROM search_suggestions WHERE query = 'god of war ragnarok' AND search_count = 0;
ALTER TABLE search_suggestions DROP COLUMN last_searched_on, DROP COLUMN days_searched;
