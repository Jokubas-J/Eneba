-- +goose Up

-- A narrow banner tile (about 1:2) showed only a sliver of the wide hero art (3:1).
-- Each banner gets an optional portrait image for when it is drawn as a narrow tile:
-- Steam's 600x900 library cover, the same image the product cards use. The large tile
-- keeps the wide image. Checked with a HEAD request on 2026-11-17, like the heroes.
ALTER TABLE banners ADD COLUMN tile_image_url text;

UPDATE banners SET tile_image_url =
    'https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/' ||
    substring(image_url FROM '/apps/([0-9]+)/') || '/library_600x900.jpg'
WHERE image_url LIKE 'https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/%/library_hero.jpg';

-- +goose Down
ALTER TABLE banners DROP COLUMN tile_image_url;
