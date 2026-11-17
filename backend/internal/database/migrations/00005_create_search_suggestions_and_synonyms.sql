-- +goose Up

-- Pairs of interchangeable search phrases. The search treats them as equal in both
-- directions, so each pair is stored once: ('gta', 'grand theft auto') also means
-- 'grand theft auto' -> 'gta'.
--
-- Both sides are stored already simplified (as normalize_search would make them),
-- because they are compared with simplified queries and names. The CHECK makes the
-- database refuse anything else, e.g. 'GTA' or 'half-life'.
CREATE TABLE search_synonyms (
    id         bigserial PRIMARY KEY,
    term       text NOT NULL CHECK (term <> '' AND term = normalize_search(term)),
    synonym    text NOT NULL CHECK (synonym <> '' AND synonym = normalize_search(synonym)),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (term <> synonym),
    UNIQUE (term, synonym)
);

-- Search queries offered in the top part of the search dropdown.
--   source 'curated': written by us for this catalog (below), always shown.
--   source 'visitor': a query someone searched that returned results; shown once it
--                     has been searched often enough (catalog.minVisitorSearches).
CREATE TABLE search_suggestions (
    id                bigserial PRIMARY KEY,
    query             text NOT NULL UNIQUE CHECK (query <> '' AND query = normalize_search(query)),
    source            text NOT NULL CHECK (source IN ('curated', 'visitor')),
    search_count      integer NOT NULL DEFAULT 0 CHECK (search_count >= 0),
    last_result_count integer NOT NULL DEFAULT 0 CHECK (last_result_count >= 0),
    last_searched_at  timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);
-- Serves the typo-tolerant part of the suggestion lookup ($1 <% query).
CREATE INDEX search_suggestions_query_trgm_idx ON search_suggestions USING gin (query gin_trgm_ops);

INSERT INTO search_synonyms (term, synonym) VALUES
    -- Roman numerals in titles ("Grand Theft Auto V", "Dark Souls III") and the
    -- digits people type instead.
    ('1', 'i'), ('2', 'ii'), ('3', 'iii'), ('4', 'iv'), ('5', 'v'), ('6', 'vi'),
    ('7', 'vii'), ('8', 'viii'), ('9', 'ix'), ('10', 'x'), ('11', 'xi'), ('12', 'xii'),
    ('13', 'xiii'), ('14', 'xiv'), ('15', 'xv'), ('16', 'xvi'),
    -- Common abbreviations of titles in our catalog.
    ('gta', 'grand theft auto'),
    ('rdr', 'red dead redemption'),
    ('re', 'resident evil'),
    ('bg', 'baldurs gate'),
    ('ff', 'final fantasy'),
    ('ac', 'assassins creed'),
    ('tes', 'the elder scrolls'),
    ('mh', 'monster hunter'),
    ('mk', 'mortal kombat'),
    ('ds', 'dark souls'),
    ('hl', 'half life'),
    ('l4d', 'left 4 dead'),
    ('nms', 'no mans sky'),
    ('ets', 'euro truck simulator'),
    ('ats', 'american truck simulator'),
    ('aoe', 'age of empires'),
    ('civ', 'civilization'),
    ('hoi', 'hearts of iron'),
    ('eu', 'europa universalis'),
    ('ck', 'crusader kings'),
    ('cod', 'call of duty'),
    ('dmc', 'devil may cry'),
    ('sf', 'street fighter'),
    ('fh', 'forza horizon'),
    ('nfs', 'need for speed'),
    ('botw', 'breath of the wild'),
    -- Spellings that differ by a space or a hyphen in the official title.
    ('halflife', 'half life'),
    ('spiderman', 'spider man'),
    ('repo', 'r e p o'),
    ('warhammer 40k', 'warhammer 40 000');

-- Curated suggestions: the short forms people type for the games in our catalog.
-- Every one of them is checked by a test to return at least one product.
INSERT INTO search_suggestions (query, source)
SELECT q, 'curated' FROM (VALUES
    ('witcher'), ('witcher 3'), ('the witcher 3'), ('witcher 3 remastered'),
    ('cyberpunk'), ('cyberpunk 2077'),
    ('elden ring'), ('baldurs gate 3'),
    ('red dead redemption 2'), ('rdr2'),
    ('gta'), ('gta v'), ('gta 5'), ('grand theft auto v'),
    ('resident evil'), ('resident evil 2'), ('resident evil 3'), ('resident evil 4'),
    ('resident evil 7'), ('resident evil village'),
    ('dark souls'), ('dark souls 3'), ('sekiro'), ('lies of p'), ('armored core 6'),
    ('god of war'), ('hogwarts legacy'), ('starfield'),
    ('fallout'), ('fallout 4'), ('fallout 76'),
    ('skyrim'), ('oblivion'), ('the elder scrolls'),
    ('stardew valley'), ('terraria'), ('hollow knight'), ('hades'), ('hades 2'),
    ('celeste'), ('balatro'), ('palworld'), ('valheim'), ('rust'), ('dayz'),
    ('subnautica'), ('no mans sky'), ('the forest'), ('sons of the forest'), ('raft'),
    ('project zomboid'), ('7 days to die'), ('ark'), ('green hell'), ('enshrouded'),
    ('sea of thieves'), ('dying light'), ('dying light 2'),
    ('doom'), ('doom eternal'), ('half life'), ('half life 2'), ('portal'), ('portal 2'),
    ('left 4 dead 2'), ('garrys mod'), ('among us'), ('lethal company'), ('phasmophobia'),
    ('dead by daylight'), ('it takes two'), ('split fiction'), ('a way out'),
    ('helldivers 2'), ('monster hunter'), ('monster hunter wilds'), ('monster hunter rise'),
    ('monster hunter world'),
    ('final fantasy'), ('final fantasy 7'), ('final fantasy 16'), ('final fantasy 14'),
    ('persona 5 royal'), ('persona 3 reload'), ('persona 4 golden'),
    ('yakuza'), ('like a dragon'), ('street fighter 6'), ('tekken 8'),
    ('mortal kombat'), ('mortal kombat 1'), ('mortal kombat 11'),
    ('forza horizon 5'), ('forza'), ('assetto corsa'), ('euro truck simulator 2'),
    ('american truck simulator'), ('need for speed'),
    ('civilization'), ('civilization 6'), ('civilization 5'), ('total war'),
    ('total war warhammer 3'), ('stellaris'), ('hearts of iron 4'), ('europa universalis 4'),
    ('crusader kings 3'), ('age of empires'), ('age of empires 2'), ('age of empires 4'),
    ('cities skylines'), ('factorio'), ('satisfactory'), ('rimworld'),
    ('frostpunk'), ('frostpunk 2'), ('planet zoo'), ('planet coaster'),
    ('two point hospital'), ('powerwash simulator'), ('supermarket simulator'),
    ('kerbal space program'), ('microsoft flight simulator'),
    ('assassins creed'), ('assassins creed valhalla'), ('assassins creed shadows'),
    ('assassins creed odyssey'), ('far cry'), ('far cry 6'), ('far cry 5'),
    ('watch dogs'), ('the division 2'),
    ('borderlands'), ('borderlands 3'), ('borderlands 4'), ('bioshock'), ('dishonored'),
    ('prey'), ('dead space'), ('metro exodus'), ('mass effect'), ('dragon age'),
    ('star wars jedi'), ('battlefield'), ('battlefield 2042'), ('titanfall 2'), ('halo'),
    ('spider man'), ('horizon forbidden west'), ('ghost of tsushima'), ('the last of us'),
    ('uncharted'), ('days gone'), ('black myth wukong'), ('devil may cry 5'),
    ('nier automata'), ('octopath traveler'), ('dragon quest 11'),
    ('slay the spire'), ('vampire survivors'), ('dead cells'), ('cuphead'), ('undertale'),
    ('deltarune'), ('outer wilds'), ('inscryption'), ('cult of the lamb'), ('risk of rain 2'),
    ('the binding of isaac'), ('darkest dungeon'), ('enter the gungeon'), ('dave the diver'),
    ('sea of stars'), ('stray'), ('outlast'), ('deep rock galactic'), ('space marine 2'),
    ('the planet crafter'), ('dyson sphere program'), ('timberborn'), ('against the storm'),
    ('kenshi'), ('repo'), ('peak'), ('inzoi'), ('clair obscur expedition 33'),
    ('indiana jones'), ('wolfenstein')
) AS curated (q);

-- +goose Down
DROP TABLE search_suggestions;
DROP TABLE search_synonyms;
