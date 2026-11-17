-- +goose Up

-- The homepage, as data. The frontend renders exactly these blocks in this order;
-- each block's content comes from its own endpoint (source). Changing the order, a
-- title or hiding a block is an UPDATE, not a deploy. Docs: docs/homepage.md.
--
-- type tells the frontend which component draws the block. A type the frontend does
-- not know is skipped, so a new block type can be added here before the frontend
-- supports it without breaking the page.
INSERT INTO home_blocks (type, title, source, show_all_url, sort_order) VALUES
    ('banner_mosaic',    NULL,                '/api/home/banners',       NULL,                   10),
    ('product_carousel', 'Best sellers',      '/api/home/best-sellers',  '/store?sort=popular',  20),
    ('product_carousel', 'Biggest discounts', '/api/home/deals',         '/store?sort=discount', 30),
    ('product_carousel', 'New releases',      '/api/home/new-releases',  '/store?sort=newest',   40),
    ('category_grid',    'Categories',        '/api/home/categories',    '/store',               50);

-- Curated banners. The images are Steam's "library hero" artwork (wide, 3840x1240),
-- hot-linked like the covers. Which games have one was checked once, by hand, when
-- this migration was written (HEAD request, 2026-11-17); the app never calls Steam.
-- The first banner (lowest sort_order) is the large tile of the mosaic. No prices
-- here: a price written into a banner would go stale; the product page has the live one.
INSERT INTO banners (title, subtitle, image_url, link_url, sort_order) VALUES
    ('ELDEN RING', 'Rise, Tarnished, in the Lands Between',
     'https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/1245620/library_hero.jpg',
     '/product/elden-ring-pc-steam-key-global', 10),
    ('Cyberpunk 2077', 'Night City, open world RPG',
     'https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/1091500/library_hero.jpg',
     '/product/cyberpunk-2077-pc-steam-key-global', 20),
    ('Baldur''s Gate 3', 'Party-based RPG from Larian',
     'https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/1086940/library_hero.jpg',
     '/product/baldur-s-gate-3-pc-steam-key-global', 30),
    ('Black Myth: Wukong', 'Action RPG rooted in Chinese mythology',
     'https://shared.akamai.steamstatic.com/store_item_assets/steam/apps/2358720/library_hero.jpg',
     '/product/black-myth-wukong-pc-steam-key-global', 40);

-- +goose Down
DELETE FROM banners;
DELETE FROM home_blocks;
