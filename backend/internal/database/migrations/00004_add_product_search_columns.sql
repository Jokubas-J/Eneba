-- +goose Up

-- Search works on a simplified copy of the product name, not on the name itself:
--   "Baldur's Gate 3 (PC) Steam Key GLOBAL"  ->  "baldurs gate 3 pc steam key global"
-- Lower case, apostrophes dropped (so "baldurs" finds "Baldur's"), every other run of
-- punctuation turned into one space. The Go code simplifies the visitor's query with
-- the same rules (catalog.NormalizeQuery), so both sides are compared like for like.
--
-- IMMUTABLE promises PostgreSQL that the result depends only on the argument, which
-- is what allows the function to be used in a generated column.
-- +goose StatementBegin
CREATE FUNCTION normalize_search(input text) RETURNS text
LANGUAGE sql IMMUTABLE RETURNS NULL ON NULL INPUT
AS $$
    SELECT btrim(regexp_replace(translate(lower(input), '''’', ''), '[^a-z0-9]+', ' ', 'g'))
$$;
-- +goose StatementEnd

-- GENERATED ... STORED columns are computed by the database whenever a row is
-- written, so they can never be out of date with the name and existing rows get
-- their values right now, without re-running the seed.
ALTER TABLE products
    ADD COLUMN search_name text GENERATED ALWAYS AS (normalize_search(name)) STORED,
    -- First character of every word: "grand theft auto v pc steam key global" ->
    -- "gtavpskg". A query such as "gta" or "rdr2" is a prefix of it, which is how
    -- abbreviations are found without a hand-kept list of them.
    ADD COLUMN search_initials text GENERATED ALWAYS AS (
        regexp_replace(normalize_search(name), '([a-z0-9])[a-z0-9]*\s*', '\1', 'g')
    ) STORED;

-- The trigram index moves from the raw name to the column that is searched.
DROP INDEX products_name_trgm_idx;
CREATE INDEX products_search_name_trgm_idx ON products USING gin (search_name gin_trgm_ops);

-- The listing view gains the two columns. CREATE OR REPLACE may only ADD columns at
-- the end, so the rest of the definition is repeated unchanged from migration 00003.
CREATE OR REPLACE VIEW product_listing AS
SELECT p.id,
       p.slug,
       p.name,
       p.product_type,
       p.cover_url,
       p.release_date,
       p.popularity,
       p.steam_app_id,
       pl.slug AS platform_slug,
       pl.name AS platform_name,
       r.slug  AS region_slug,
       r.name  AS region_name,
       best.price_cents,
       best.original_price_cents,
       COALESCE(best.offer_count, 0) AS offer_count,
       p.search_name,
       p.search_initials
FROM products p
JOIN platforms pl ON pl.id = p.platform_id
JOIN regions r ON r.id = p.region_id
LEFT JOIN LATERAL (
    SELECT o.price_cents,
           o.original_price_cents,
           count(*) OVER () AS offer_count
    FROM offers o
    WHERE o.product_id = p.id
      AND o.is_active
      AND EXISTS (
          SELECT 1 FROM license_keys k
          WHERE k.offer_id = o.id AND k.status = 'available'
      )
    ORDER BY o.price_cents, o.id
    LIMIT 1
) best ON true;

-- +goose Down
-- A view cannot lose columns in place, so it is dropped and rebuilt as in 00003.
DROP VIEW product_listing;

DROP INDEX products_search_name_trgm_idx;
ALTER TABLE products DROP COLUMN search_initials, DROP COLUMN search_name;
DROP FUNCTION normalize_search(text);
CREATE INDEX products_name_trgm_idx ON products USING gin (name gin_trgm_ops);

CREATE VIEW product_listing AS
SELECT p.id,
       p.slug,
       p.name,
       p.product_type,
       p.cover_url,
       p.release_date,
       p.popularity,
       p.steam_app_id,
       pl.slug AS platform_slug,
       pl.name AS platform_name,
       r.slug  AS region_slug,
       r.name  AS region_name,
       best.price_cents,
       best.original_price_cents,
       COALESCE(best.offer_count, 0) AS offer_count
FROM products p
JOIN platforms pl ON pl.id = p.platform_id
JOIN regions r ON r.id = p.region_id
LEFT JOIN LATERAL (
    SELECT o.price_cents,
           o.original_price_cents,
           count(*) OVER () AS offer_count
    FROM offers o
    WHERE o.product_id = p.id
      AND o.is_active
      AND EXISTS (
          SELECT 1 FROM license_keys k
          WHERE k.offer_id = o.id AND k.status = 'available'
      )
    ORDER BY o.price_cents, o.id
    LIMIT 1
) best ON true;
