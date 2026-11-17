-- +goose Up

-- Screenshots of a product's gallery. Only the URLs are stored; the images are
-- hot-linked from Steam's CDN like the covers. The seed fills this table from the
-- Steam snapshot; the running app never calls Steam.
CREATE TABLE product_screenshots (
    id            bigserial PRIMARY KEY,
    -- A screenshot means nothing without its product, so it goes with it.
    product_id    bigint NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    thumbnail_url text NOT NULL,
    full_url      text NOT NULL,
    sort_order    integer NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    -- One position per product; the index it creates also serves "screenshots of a
    -- product, in order".
    UNIQUE (product_id, sort_order)
);

-- How to activate a key, per platform: the "Important notice" box and the
-- "Activation guide" tab of the product page.
--
-- Keyed by the platform's slug and deliberately WITHOUT a foreign key to platforms:
-- the seed empties and refills the platforms table (seed load -reset), and a foreign
-- key would force it to delete these rows too. This text belongs to the migrations,
-- not to the generated demo catalog.
CREATE TABLE activation_guides (
    platform_slug text PRIMARY KEY,
    notice        text NOT NULL,
    steps         text[] NOT NULL CHECK (cardinality(steps) > 0),
    redeem_url    text,
    created_at    timestamptz NOT NULL DEFAULT now()
);

INSERT INTO activation_guides (platform_slug, notice, steps, redeem_url) VALUES
(
    'steam',
    'This is a Steam key. It is activated in the Steam client or on the Steam website and adds the game to your Steam library. It does not work on other platforms. A free Steam account is required.',
    ARRAY[
        'Install the Steam client and sign in, or create a free Steam account.',
        'In the Steam client, open the "Games" menu and choose "Activate a Product on Steam…". You can also use the activation page on the Steam website.',
        'Enter the key and follow the instructions. The game appears in your library, ready to download.'
    ],
    'https://store.steampowered.com/account/registerkey'
),
(
    'gog',
    'This is a GOG.com key. It is redeemed on GOG.com and does not work on Steam or other platforms. GOG games have no copy protection: after redeeming, you can download the installer or use GOG Galaxy.',
    ARRAY[
        'Sign in to your GOG.com account, or create one for free.',
        'Open the redeem page on GOG.com and enter the key.',
        'The game appears in your GOG library. Download it with GOG Galaxy or as an offline installer.'
    ],
    'https://www.gog.com/redeem'
),
(
    'epic-games',
    'This is an Epic Games Store key. It is redeemed with an Epic Games account and does not work on Steam or other platforms.',
    ARRAY[
        'Open the Epic Games Launcher and sign in, or create a free Epic Games account.',
        'Click your profile name and choose "Redeem Code".',
        'Enter the key and confirm. The game is added to your library.'
    ],
    NULL
),
(
    'xbox-live',
    'This is an Xbox key for Xbox Series X|S. It is redeemed with a Microsoft account and does not work on PC stores or other consoles.',
    ARRAY[
        'Sign in with your Microsoft account on the Xbox, or on the Microsoft redeem page in a browser.',
        'On the console, open the Microsoft Store, choose "Redeem" and enter the code.',
        'The game appears in "My games & apps", ready to install.'
    ],
    'https://redeem.microsoft.com'
),
(
    'psn',
    'This is a PlayStation Network key for PS5. It is redeemed with a PlayStation account whose country matches the key''s region, and does not work on other platforms.',
    ARRAY[
        'On the PS5, open PlayStation Store and sign in.',
        'Open the menu (…) at the top of the Store and choose "Redeem Code".',
        'Enter the code and confirm. The game is added to your game library.'
    ],
    NULL
);

-- +goose Down
DROP TABLE activation_guides;
DROP TABLE product_screenshots;
