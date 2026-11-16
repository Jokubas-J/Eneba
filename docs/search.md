# Product search: design and test table

How the search behind the header box, `GET /api/products?text=…` and
`GET /api/search/suggest?q=…` decides what matches and what comes first, and the table of queries
it was designed against.

Code: `backend/internal/catalog/search.go` (rules), `repository.go` (query),
migration `00004_add_product_search_columns.sql` (columns and index),
`search_test.go` (the table below as an automated test).

## What is searched

The schema has **no game-level title**. A product is one sellable variant, and its name carries
platform and region, as on Eneba: `The Witcher 3: Wild Hunt (PC) Steam Key GLOBAL`. Variants of
one game are grouped only by a number (`steam_app_id`).

The search therefore matches the **full variant name**, in a simplified copy
(`products.search_name`: lower case, apostrophes removed, punctuation → spaces). Two reasons:

- Platform and region become searchable for free: `skyrim psn`, `terraria xbox`.
- Adding a separate title column would have needed a schema change plus a re-seed of production,
  to solve a problem that `word_similarity` already solves (next section).

The visitor's query is simplified by the same rules in Go (`NormalizeQuery`); a test checks that
the Go and SQL versions give identical output.

## Matching

The query is split into words. A product matches when **every** word is found in the name, where
a word is "found" if it is either

1. the **beginning of a word** of the name — `witc` → *witcher*, `3` → *3*; or
2. **similar enough** to part of the name — `witchr` → *witcher* — measured with
   `pg_trgm` word similarity, threshold **0.55**. Only for words of 3+ characters.

Separately, a single word of 2–6 characters also matches as an **abbreviation**: it is compared
with the first letters of the name's words (`products.search_initials`), so `gta` finds
*Grand Theft Auto*, `rdr2` finds *Red Dead Redemption 2*.

Requiring every word is what keeps `witchr 3` on The Witcher 3 instead of every game with a 3.

### Why word similarity and not plain similarity

`pg_trgm` cuts text into overlapping three-character pieces (trigrams) and counts how many two
texts share. A typo only changes the trigrams around it, so the score stays high.

`similarity(a, b)` compares the whole of both texts. Our names are long, so a short query scores
very low against them whatever it is. `word_similarity(query, name)` compares the query with the
**best-matching stretch** of the name and ignores the rest, which is exactly the situation of a
visitor typing a few words of a long product name.

### Why 0.55

Measured on the real catalog before choosing (query word → best score of the intended game →
best score of an unrelated game):

| Typed | Intended game | Score | Best unrelated | Score |
|---|---|---|---|---|
| `witchr` | The Witcher 3 | 0.71 | Outer Wilds | 0.29 |
| `wicher` | The Witcher 3 | 0.56 | Outer Wilds | 0.43 |
| `cyberpnk` | Cyberpunk 2077 | 0.67 | Ratchet & Clank | 0.15 |
| `skyrm` | Skyrim | 0.67 | No Man's Sky | 0.50 |
| `hogwarst` | Hogwarts Legacy | 0.67 | Hollow Knight | 0.22 |
| `fantsy` | Final Fantasy | 0.57 | — | — |
| `battlefeild` | Battlefield | 0.58 | The Witcher 3 | 0.17 |
| `halflife` | Half-Life | 0.58 | Halo | 0.33 |
| `teraria` | Terraria | 0.70 | Teardown | 0.25 |
| `mincraft` | *(not in the catalog)* | — | This War of Mine | 0.33 |

Real one- and two-letter typos land at 0.56–0.71; unrelated names mostly at 0.50 or below.
PostgreSQL's default threshold of 0.60 would lose `wicher`, `fantsy`, `halflife` and
`battlefeild`; 0.50 would let `skyrm` match *No Man's Sky*. 0.55 sits in the gap.

## Ranking

Each match gets a score; higher comes first.

- Per word: **1.0** if it is an exact word beginning, otherwise its word similarity (0.55–1.0).
  The score is the **average** over the words.
- **+0.5** if the name starts with the whole query, **+0.25** if the query appears as a phrase
  later in the name. (`dark souls` puts *Dark Souls* above a game that just contains both words.)
- An abbreviation match scores **0.8**: above any typo match, below an exact word.
- Ties are common (every edition of a game matches equally), so they are broken by
  **popularity**, then by id so the order is stable between pages.

When the visitor picks a sort order in the store (price, date, name), that order replaces the
ranking; the set of matches stays the same.

## Test table

`go test ./internal/catalog -run TestSearchRanking -v` runs these queries against the real
catalog (236 games, 476 products) and prints this table. The test fails if the first result does
not contain the expected title, or if a "no match" query returns anything.

| Kind | Query | Results | First result |
|---|---|---|---|
| typo | `witchr 3` | 2 | The Witcher 3: Wild Hunt — Remastered (PC) Steam Key GLOBAL |
| typo | `wicher` | 2 | The Witcher 3: Wild Hunt — Remastered (PC) Steam Key GLOBAL |
| typo | `cyberpnk` | 2 | Cyberpunk 2077 (PC) Steam Key GLOBAL |
| typo | `skyrm` | 4 | The Elder Scrolls V: Skyrim Special Edition (PC) Steam Key GLOBAL |
| typo | `eldn ring` | 1 | ELDEN RING (PC) Steam Key GLOBAL |
| typo | `hogwarst` | 1 | Hogwarts Legacy (PC) Steam Key GLOBAL |
| typo | `stardw valley` | 2 | Stardew Valley (PC) Steam Key GLOBAL |
| typo | `red dead redemtion` | 2 | Red Dead Redemption 2 (PC) Steam Key GLOBAL |
| typo | `battlefeild` | 6 | Battlefield 2042 (PC) Steam Key GLOBAL |
| typo | `teraria` | 3 | Terraria (PC) Steam Key GLOBAL |
| typo | `civilzation` | 6 | Sid Meier’s Civilization VI (PC) Steam Key GLOBAL |
| typo | `fallot 4` | 1 | Fallout 4 (PC) Steam Key GLOBAL |
| partial | `witc` | 2 | The Witcher 3: Wild Hunt — Remastered (PC) Steam Key GLOBAL |
| partial | `cyber` | 2 | Cyberpunk 2077 (PC) Steam Key GLOBAL |
| partial | `hollow kn` | 3 | Hollow Knight (PC) Steam Key GLOBAL |
| partial | `red dead 2` | 2 | Red Dead Redemption 2 (PC) Steam Key GLOBAL |
| partial | `resident evil 4` | 2 | Resident Evil 4 (PC) Steam Key GLOBAL |
| partial | `half life 2` | 2 | Half-Life 2 (PC) Steam Key GLOBAL |
| partial | `dark souls` | 3 | DARK SOULS III (PC) Steam Key GLOBAL |
| partial | `god of war` | 5 | God of War (PC) Steam Key GLOBAL |
| partial | `souls` | 3 | DARK SOULS III (PC) Steam Key GLOBAL |
| abbreviation | `gta` | 1 | Grand Theft Auto V Enhanced (PC) Steam Key GLOBAL |
| abbreviation | `rdr2` | 2 | Red Dead Redemption 2 (PC) Steam Key GLOBAL |
| abbreviation | `bg3` | 2 | Baldur's Gate 3 (PC) Steam Key GLOBAL |
| abbreviation | `re4` | 2 | Resident Evil 4 (PC) Steam Key GLOBAL |
| spelling | `THE WITCHER` | 2 | The Witcher 3: Wild Hunt — Remastered (PC) Steam Key GLOBAL |
| spelling | `baldurs gate` | 2 | Baldur's Gate 3 (PC) Steam Key GLOBAL |
| spelling | `baldur's gate 3` | 2 | Baldur's Gate 3 (PC) Steam Key GLOBAL |
| spelling | `half-life` | 8 | Half-Life 2 (PC) Steam Key GLOBAL |
| spelling | `elden   ring` | 1 | ELDEN RING (PC) Steam Key GLOBAL |
| platform | `skyrim psn` | 1 | The Elder Scrolls V: Skyrim (PS5) PSN Key GLOBAL |
| platform | `terraria xbox` | 1 | Terraria (Xbox Series X\|S) Xbox Live Key GLOBAL |
| no match | `zzxqv` | 0 | (none) |
| no match | `qwerty` | 0 | (none) |
| no match | `mincraft` | 0 | (none) |
| no match | `asdf ghjk` | 0 | (none) |

`mincraft` was one of the example queries in the plan. Minecraft is not sold on Steam, so it is
not in the catalog; the correct result is "nothing", and the table checks that the search does
not offer the nearest wrong game instead.

## Known limits

- **Roman numerals:** `gta 5` does not find *Grand Theft Auto V* (the name says "V"). `gta` does.
- **Heavier misspellings:** a word with several errors falls below the threshold (`asasin` scores
  0.40 against *Assassin's*; `assasins` at 0.80 is found).
- **Typos in very short words:** a 3-4 letter word has so few trigrams that one wrong letter
  drops it below the threshold: `resident evl` finds nothing (`evl` scores 0.50 against *evil*),
  while `residnt evil` works.
- **Very short typos tie:** `eldn` alone scores 0.60 against both *Elden Ring* and *The Elder
  Scrolls*, so popularity decides the order. `eldn ring` is unambiguous.
- **No synonyms or translations**, and no "did you mean" text suggestions (Eneba's dropdown shows
  both; ours shows product hits only).
- **Index use:** the conditions are written in the forms the trigram GIN index supports
  (`LIKE` and the `<%` operator). With 476 products PostgreSQL will normally just read the whole
  table, which is faster at that size; the index matters as the catalog grows. I have not
  measured it with a large catalog.
- A dedicated search engine (Meilisearch, Elasticsearch) would rank better and add synonyms, at
  the cost of a second service to host and keep in sync — see `docs/research.md`.
