-- +goose Up

-- Conventions used in every table:
--   * id bigserial        - surrogate key, 64-bit so it never runs out.
--   * created_at          - set by the database, not by application code.
--   * money in *_cents    - integers; floating point cannot represent 0.10 exactly.
--   * status columns      - text + CHECK instead of a PostgreSQL ENUM type: the allowed
--                           values are still enforced by the database, but adding one
--                           later is a simple constraint change.
--   * foreign keys        - default ON DELETE behaviour (refuse) unless a row has no
--                           meaning without its parent, then ON DELETE CASCADE.

-- ---------------------------------------------------------------------------
-- Catalog
-- ---------------------------------------------------------------------------

CREATE TABLE platforms (
    id         bigserial PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE regions (
    id         bigserial PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE genres (
    id         bigserial PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- One row = one sellable variant (title + platform + region), as on Eneba, where a
-- "Europe" key and a "Global" key of the same game are different products.
CREATE TABLE products (
    id           bigserial PRIMARY KEY,
    slug         text NOT NULL UNIQUE,
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    product_type text NOT NULL DEFAULT 'game' CHECK (product_type IN ('game', 'dlc')),
    platform_id  bigint NOT NULL REFERENCES platforms (id),
    region_id    bigint NOT NULL REFERENCES regions (id),
    cover_url    text NOT NULL DEFAULT '',
    release_date date,
    developer    text NOT NULL DEFAULT '',
    publisher    text NOT NULL DEFAULT '',
    popularity   integer NOT NULL DEFAULT 0,
    -- Where the data came from; also groups the variants of the same game.
    steam_app_id integer,
    created_at   timestamptz NOT NULL DEFAULT now(),
    -- The same game cannot exist twice for one platform and region.
    UNIQUE (steam_app_id, platform_id, region_id)
);

-- Trigram index for typo-tolerant search on the name. GIN (not B-tree) because a
-- trigram search asks "which rows contain these 3-letter pieces", not "which rows
-- sort near this value".
CREATE INDEX products_name_trgm_idx ON products USING gin (name gin_trgm_ops);
-- PostgreSQL does not index foreign key columns automatically; these serve the
-- store filters.
CREATE INDEX products_platform_id_idx ON products (platform_id);
CREATE INDEX products_region_id_idx ON products (region_id);

CREATE TABLE product_genres (
    product_id bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    genre_id   bigint NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    -- The pair is the key: a product has each genre at most once. This index also
    -- serves "genres of a product"; the one below serves "products of a genre".
    PRIMARY KEY (product_id, genre_id)
);
CREATE INDEX product_genres_genre_id_idx ON product_genres (genre_id);

-- ---------------------------------------------------------------------------
-- Marketplace: sellers, their offers and the keys in stock
-- ---------------------------------------------------------------------------

CREATE TABLE sellers (
    id           bigserial PRIMARY KEY,
    name         text NOT NULL UNIQUE,
    -- numeric, not float: 9.7 must be stored and compared exactly.
    rating       numeric(3, 1) NOT NULL CHECK (rating >= 0 AND rating <= 10),
    rating_count integer NOT NULL DEFAULT 0 CHECK (rating_count >= 0),
    is_verified  boolean NOT NULL DEFAULT false,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE offers (
    id                   bigserial PRIMARY KEY,
    product_id           bigint NOT NULL REFERENCES products (id),
    seller_id            bigint NOT NULL REFERENCES sellers (id),
    price_cents          integer NOT NULL CHECK (price_cents > 0),
    -- Reference price the discount is shown against; the discount % is derived.
    original_price_cents integer NOT NULL,
    is_active            boolean NOT NULL DEFAULT true,
    created_at           timestamptz NOT NULL DEFAULT now(),
    CHECK (original_price_cents >= price_cents),
    -- One offer per seller per product. The index it creates also serves
    -- "all offers of a product".
    UNIQUE (product_id, seller_id)
);
CREATE INDEX offers_seller_id_idx ON offers (seller_id);

-- ---------------------------------------------------------------------------
-- Accounts
-- ---------------------------------------------------------------------------

CREATE TABLE users (
    id            bigserial PRIMARY KEY,
    -- Stored lower-cased so "A@x.com" and "a@x.com" cannot become two accounts; the
    -- CHECK makes the database enforce what the application is supposed to do.
    email         text NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id         bigserial PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- SHA-256 of the cookie token. The raw token is never stored, so a leaked
    -- database does not contain usable sessions.
    token_hash text NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

-- ---------------------------------------------------------------------------
-- Orders and key delivery
-- ---------------------------------------------------------------------------

CREATE TABLE orders (
    id                  bigserial PRIMARY KEY,
    user_id             bigint NOT NULL REFERENCES users (id),
    status              text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'expired')),
    total_cents         integer NOT NULL CHECK (total_cents >= 0),
    currency            text NOT NULL DEFAULT 'EUR',
    payment_provider    text NOT NULL CHECK (payment_provider IN ('stripe', 'simulated')),
    -- Id of the payment session at the provider. UNIQUE allows many NULLs (an order
    -- has none until the session is created) but never the same session twice.
    provider_session_id text UNIQUE,
    paid_at             timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    -- An order is paid exactly when it has a payment time.
    CHECK ((status = 'paid') = (paid_at IS NOT NULL))
);
CREATE INDEX orders_user_id_idx ON orders (user_id);

CREATE TABLE order_items (
    id               bigserial PRIMARY KEY,
    order_id         bigint NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    offer_id         bigint NOT NULL REFERENCES offers (id),
    -- Snapshots taken at purchase time: the order history must keep showing what
    -- was bought and for how much even if the offer is later changed.
    product_name     text NOT NULL,
    seller_name      text NOT NULL,
    unit_price_cents integer NOT NULL CHECK (unit_price_cents > 0),
    quantity         integer NOT NULL CHECK (quantity > 0),
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX order_items_order_id_idx ON order_items (order_id);

-- One row per license key. The stock of an offer is the number of its 'available'
-- rows, so stock can never disagree with the keys that actually exist.
CREATE TABLE license_keys (
    id             bigserial PRIMARY KEY,
    offer_id       bigint NOT NULL REFERENCES offers (id),
    key_value      text NOT NULL UNIQUE,
    status         text NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'reserved', 'sold')),
    order_item_id  bigint REFERENCES order_items (id),
    reserved_until timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    -- A key belongs to an order item exactly when it is not available. Even a bug in
    -- the application cannot leave a key "sold to nobody" or "available but owned".
    CHECK ((status = 'available') = (order_item_id IS NULL))
);
-- Partial index: contains only the available keys, which is the only set that the
-- stock count and the reservation query ever look at, so it stays small as keys sell.
CREATE INDEX license_keys_available_idx ON license_keys (offer_id) WHERE status = 'available';
CREATE INDEX license_keys_order_item_id_idx ON license_keys (order_item_id);

-- Every webhook received from a payment provider: an audit trail, and the UNIQUE
-- constraint is what detects a webhook that is delivered twice.
CREATE TABLE payment_events (
    id         bigserial PRIMARY KEY,
    provider   text NOT NULL,
    event_id   text NOT NULL,
    type       text NOT NULL,
    payload    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, event_id)
);

-- ---------------------------------------------------------------------------
-- Homepage content
-- ---------------------------------------------------------------------------

CREATE TABLE banners (
    id         bigserial PRIMARY KEY,
    title      text NOT NULL,
    subtitle   text NOT NULL DEFAULT '',
    image_url  text NOT NULL,
    link_url   text NOT NULL,
    sort_order integer NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- The layout of the homepage: which blocks, in which order, and the API path that
-- provides each block's data. The frontend renders whatever this table says.
CREATE TABLE home_blocks (
    id           bigserial PRIMARY KEY,
    type         text NOT NULL,
    title        text,
    source       text NOT NULL,
    show_all_url text,
    sort_order   integer NOT NULL DEFAULT 0,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
-- Reverse order of creation, so no table is dropped while another still refers to it.
DROP TABLE home_blocks;
DROP TABLE banners;
DROP TABLE payment_events;
DROP TABLE license_keys;
DROP TABLE order_items;
DROP TABLE orders;
DROP TABLE sessions;
DROP TABLE users;
DROP TABLE offers;
DROP TABLE sellers;
DROP TABLE product_genres;
DROP TABLE products;
DROP TABLE genres;
DROP TABLE regions;
DROP TABLE platforms;
