-- +goose Up

-- Icon keys are names from the frontend's icon set (lucide, kebab-case, e.g.
-- "car-front"). The frontend maps each key to an icon component and draws a default
-- icon for a key it does not know, so content here can never break the page.

-- An icon per genre / platform / region. Keyed by slug, without a foreign key (like
-- activation_guides): `seed load -reset` deletes and re-creates those lookup tables
-- with new ids, but the slugs stay the same, so these rows survive a re-seed.
CREATE TABLE taxonomy_icons (
    kind text NOT NULL CHECK (kind IN ('genre', 'platform', 'region')),
    slug text NOT NULL,
    icon text NOT NULL,
    PRIMARY KEY (kind, slug)
);

INSERT INTO taxonomy_icons (kind, slug, icon) VALUES
    ('genre', 'action', 'swords'),
    ('genre', 'adventure', 'compass'),
    ('genre', 'rpg', 'shield'),
    ('genre', 'indie', 'sparkles'),
    ('genre', 'simulation', 'plane'),
    ('genre', 'strategy', 'crown'),
    ('genre', 'casual', 'puzzle'),
    ('genre', 'racing', 'car-front'),
    ('genre', 'massively-multiplayer', 'users'),
    ('genre', 'sports', 'trophy'),
    ('platform', 'steam', 'monitor'),
    ('platform', 'gog', 'monitor'),
    ('platform', 'epic-games', 'monitor'),
    ('platform', 'xbox-live', 'gamepad-2'),
    ('platform', 'psn', 'gamepad-2'),
    ('region', 'global', 'earth'),
    ('region', 'europe', 'map-pin'),
    ('region', 'united-states', 'map-pin');

-- The site menu (docs/gap-analysis.md §2, option B), the same idea as home_blocks:
-- what the menu shows is data. A section is
--   kind 'link'       a single entry that is itself a link (url),
--   kind 'links'      a hand-written list of links (menu_links),
--   kind 'platforms' / 'genres' / 'regions'
--                     generated from that lookup table with live in-stock counts,
--                     so a new genre appears in the menu by itself.
-- placement 'drawer' = in the "Categories" drawer; 'strip' = the links of the nav
-- strip under the header.
CREATE TABLE menu_sections (
    id         bigserial PRIMARY KEY,
    title      text NOT NULL,
    kind       text NOT NULL CHECK (kind IN ('link', 'links', 'platforms', 'genres', 'regions')),
    placement  text NOT NULL CHECK (placement IN ('drawer', 'strip')),
    url        text,
    icon       text,
    sort_order integer NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,
    CHECK ((kind = 'link') = (url IS NOT NULL))
);

CREATE TABLE menu_links (
    id         bigserial PRIMARY KEY,
    section_id bigint NOT NULL REFERENCES menu_sections (id) ON DELETE CASCADE,
    label      text NOT NULL,
    url        text NOT NULL,
    icon       text,
    sort_order integer NOT NULL DEFAULT 0
);
CREATE INDEX menu_links_section_id_idx ON menu_links (section_id);

-- Explicit ids so the menu_links below can refer to their sections; the sequence is
-- moved past them afterwards.
INSERT INTO menu_sections (id, title, kind, placement, url, icon, sort_order) VALUES
    (1, 'All games',         'link',      'drawer', '/store', 'layout-grid', 10),
    (2, 'Platforms',         'platforms', 'drawer', NULL,     'monitor',     20),
    (3, 'Genres',            'genres',    'drawer', NULL,     'swords',      30),
    (4, 'Regions',           'regions',   'drawer', NULL,     'globe',       40),
    (5, 'By price',          'links',     'drawer', NULL,     'euro',        50),
    (6, 'Deals & discovery', 'links',     'drawer', NULL,     'tag',         60),
    (7, 'Quick links',       'links',     'strip',  NULL,     NULL,          10);
SELECT setval('menu_sections_id_seq', 7);

-- Prices in store URLs are in euros (the store converts them to cents for the API).
INSERT INTO menu_links (section_id, label, url, icon, sort_order) VALUES
    (5, 'Under €5',          '/store?price_max=5',      'euro',          10),
    (5, 'Under €10',         '/store?price_max=10',     'euro',          20),
    (5, 'Under €25',         '/store?price_max=25',     'euro',          30),
    (5, 'Under €50',         '/store?price_max=50',     'euro',          40),
    (5, '€50 and more',      '/store?price_min=50',     'euro',          50),
    (6, 'Biggest discounts', '/store?sort=discount',    'badge-percent', 10),
    (6, 'Cheapest first',    '/store?sort=price_asc',   'tag',           20),
    (6, 'New releases',      '/store?sort=newest',      'calendar-plus', 30),
    (6, 'Most popular',      '/store?sort=popular',     'trending-up',   40),
    -- The nav strip: the same links that Header.tsx had hardcoded until now.
    (7, 'Steam',             '/store?platform=steam',      NULL, 10),
    (7, 'GOG.com',           '/store?platform=gog',        NULL, 20),
    (7, 'Epic Games',        '/store?platform=epic-games', NULL, 30),
    (7, 'Xbox Live',         '/store?platform=xbox-live',  NULL, 40),
    (7, 'PSN',               '/store?platform=psn',        NULL, 50),
    (7, 'Cheapest first',    '/store?sort=price_asc',      NULL, 60);

-- +goose Down
DROP TABLE menu_links;
DROP TABLE menu_sections;
DROP TABLE taxonomy_icons;
