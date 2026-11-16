-- +goose Up

-- product_listing = one row per product with everything a product card shows.
-- The "From €X" price is defined here, in ONE place, so the store, the search and
-- the homepage carousels cannot disagree about it.
--
-- "Best offer" = the cheapest ACTIVE offer that has at least one AVAILABLE key.
-- A cheaper offer that is sold out must not set the price, because nobody can buy it.
-- A product with no such offer keeps its row, with NULL prices and offer_count 0
-- (it is shown as sold out rather than disappearing from the store).
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
-- LATERAL lets the subquery refer to the current product row (p.id): it is evaluated
-- once per product, like a correlated subquery that may return several columns.
-- LEFT JOIN ... ON true keeps products for which it returns no row.
LEFT JOIN LATERAL (
    SELECT o.price_cents,
           o.original_price_cents,
           -- A window function is computed over all rows that pass the WHERE, before
           -- LIMIT cuts them down to one, so this is the number of buyable offers.
           count(*) OVER () AS offer_count
    FROM offers o
    WHERE o.product_id = p.id
      AND o.is_active
      AND EXISTS (
          SELECT 1 FROM license_keys k
          WHERE k.offer_id = o.id AND k.status = 'available'
      )
    -- o.id breaks ties so that two offers at the same price always resolve the same way.
    ORDER BY o.price_cents, o.id
    LIMIT 1
) best ON true;

-- +goose Down
DROP VIEW product_listing;
