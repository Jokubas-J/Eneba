# Product search: design and test tables

How the search decides what matches and what comes first, for the store
(`GET /api/products?text=…`) and the header dropdown (`GET /api/search/suggest?q=…`), and the
tables of queries it was designed against.

Code: `backend/internal/catalog/search.go` (rules), `repository.go` (queries), migrations
`00004_add_product_search_columns.sql` (search columns and index) and
`00005_create_search_suggestions_and_synonyms.sql` (synonyms, suggestions),
`search_test.go` (the tables below as automated tests), and
`frontend/src/search/` (the dropdown).

## What is searched

The schema has **no game-level title**. A product is one sellable variant, and its name carries
platform and region, as on Eneba: `The Witcher 3: Wild Hunt (PC) Steam Key GLOBAL`. Variants of
one game are grouped only by a number (`steam_app_id`).

The search therefore matches the **full variant name**, in a simplified copy
(`products.search_name`: accents removed, lower case, apostrophes removed, punctuation →
spaces). Two reasons:

- Platform and region become searchable for free: `skyrim psn`, `terraria xbox`.
- Adding a separate title column would have needed a schema change plus a re-seed of production,
  to solve a problem that `word_similarity` already solves (next section).

The visitor's query is simplified by the same rules in Go (`NormalizeQuery`); a test checks that
the Go and SQL versions give identical output, including accented and special letters.

**Accents** (migration 00006): PostgreSQL's `unaccent` extension turns *Ragnarök* into
*Ragnarok*, *Pokémon* into *Pokemon*, *ß* into *ss*. `unaccent()` cannot be used in a generated
column directly because PostgreSQL only marks it STABLE (its dictionary could change), so a small
wrapper `immutable_unaccent` names the dictionary explicitly. In Go the same is done with
Unicode decomposition (NFD: "ö" becomes "o" + a combining mark, and the marks are dropped) from
`golang.org/x/text`, plus a short table for letters that do not decompose (ß, æ, ø, ł, đ, þ).

## Matching

**1. Synonyms.** The query is first split into words and expanded with the `search_synonyms`
table. Each pair works in both directions, and the longest phrase wins:

| Query | Becomes |
|---|---|
| `gta 5` | (`gta` or **grand theft auto**) and (`5` or **v**) |
| `grand theft auto 5` | (`grand theft auto` or **gta**) and (`5` or **v**) |
| `civ 6` | (`civ` or **civilization**) and (`6` or **vi**) |

A synonym must appear in the name as **whole words** (`v` matches *Grand Theft Auto V* but not
*Valley*). The table holds roman numerals 1–16 ↔ digits, about 25 title abbreviations (`gta`,
`rdr`, `re`, `ff`, `civ`, …) and a few spellings (`spiderman` ↔ `spider man`, `repo` ↔ `r e p o`).
It is data, not code: a row added in the database works on the next search. The synonyms apply
to the store search too, so pressing Enter on `gta 5` finds GTA V.

**2. Words.** A product matches when **every** part of the query is found in the name. A typed
word is found if it is either

1. the **beginning of a word** of the name — `witc` → *witcher*, `3` → *3*; or
2. **similar enough** to part of the name — `witchr` → *witcher* — measured with
   `pg_trgm` word similarity, threshold **0.55**, for words of 3+ characters. In a query of two
   or more words, a word of 3–4 letters needs only **0.50** (see "Short words" below).

**3. Abbreviations.** A single word of 2–6 characters also matches the first letters of the
name's words (`products.search_initials`): `rdr2` → *Red Dead Redemption 2*, `bg3` →
*Baldur's Gate 3*. This covers abbreviations nobody put into the synonym table.

Requiring every part is what keeps `witchr 3` on The Witcher 3 instead of every game with a 3.

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

### Short words

A 3–4 letter word has so few trigrams that one wrong letter drops it to about 0.50: `evl` scores
0.50 against *evil*, so `resident evl` found nothing. Lowering the threshold for everyone would let
`skyrm` match *No Man's Sky*. The compromise: a short word gets **0.50 only when the query has
other words**, because those already keep the results on topic. `resident evl` now returns the 10
Resident Evil products and nothing else; `dak` alone still needs 0.55, so it does not match
DayZ, Darkest Dungeon and Dragon's Dogma.

## Ranking

Each match gets a score; higher comes first.

- Per query part: **1.0** if it is an exact word beginning or a matching synonym, otherwise the
  word similarity (0.50–1.0). The score is the **average** over the parts.
- **+0.5** if the name starts with the whole query, **+0.25** if the query appears as a phrase
  later in the name. (`dark souls` puts *Dark Souls* above a game that just contains both words.)
- An abbreviation match scores **0.8**: above any typo match, below an exact word.
- Ties are common (every edition of a game matches equally), so they are broken by
  **popularity**, then by id so the order is stable between pages.

When the visitor picks a sort order in the store (price, date, name), that order replaces the
ranking; the set of matches stays the same.

## The dropdown

Opens from **2 characters**. Requests wait until typing pauses for **200 ms** (debounce), and a
request whose answer is no longer wanted is **aborted** (TanStack Query passes an `AbortSignal`
to `fetch` and aborts it when the query key changes). Typing `gta 5` quickly sends one request.

Verified in the browser on 2026-11-17 with Playwright, with every suggest response delayed by
1.5 s: typing `witc`, pausing, then `her` gave `sent witc → ABORTED witc (net::ERR_ABORTED) →
sent witcher → finished witcher`, and the list showed the answer for "witcher".

Content, top to bottom, as on eneba.com:

1. Up to **5 query suggestions** from `search_suggestions` (below).
2. Up to **6 products**, ranked as above: cover, platform pill, name, "From ~~€x~~ €y".
3. **"Show all N results"** → the store with the query.

Enter with nothing highlighted searches the store for the typed text. Arrow keys, Enter, Escape
and the screen-reader announcements come from Downshift (`useCombobox`); see `docs/tooling.md`.

### Query suggestions

The `search_suggestions` table has two sources:

- **curated** — about 190 short forms people type for our games (`witcher 3`, `gta v`, `rdr2`…),
  inserted by migrations. A test checks that **every** curated suggestion finds products.
- **visitor** — a store search (page 1) is recorded with `INSERT … ON CONFLICT DO UPDATE` (one
  statement, safe when two people search at once).

Visitor suggestions are shown to everyone, so they are a way to put text on other people's
screens. The rules that make that hard (migration 00007, `isRecordable` and `RecordSearch`):

| Rule | What it stops |
|---|---|
| Recorded only if the search **returned products** | Gibberish and messages. And because *every* word must match a product name, a recorded query is made of the catalog's own vocabulary (plus near-typos of it). |
| Recorded only if it is at most **5 words / 40 characters** | Sentences. Real suggestions are short (`witcher 3 remastered` is 20). |
| Shown only after it was searched on **2 different days** | One person (or a script) searching the same text many times in a row. `days_searched` goes up at most once per calendar day. |
| Typing in the dropdown is never recorded | `wit`, `witc`, `witch` counting as three searches. |

Why **days** and not **different sessions**: the store is used without signing in, and telling
anonymous visitors apart would need a tracking cookie or storing IP addresses — personal data
for a small benefit. Counting days needs nothing about the visitor; only the simplified text, two
counters and the last day are stored. The remaining weakness: a determined person can search the
same on-topic text on two days. What they can show is still limited to words of game titles. The
next step, if it ever mattered, would be a moderation flag (`is_hidden`) or requiring more days.

Typing in the dropdown records nothing (otherwise `wit`, `witc`, `witch` would all count).

Suggestions are ranked: starts with the typed text, then contains a word starting with it, then
similarity (typos), then most searched, then shortest.

## Test tables

`go test ./internal/catalog -run 'TestSearchRanking|TestQuerySuggestions' -v` runs these against
the real catalog (236 games, 476 products) and prints both tables. The tests fail if the first
result does not contain the expected title, a "no match" query returns anything, or the expected
suggestions are missing.

### Products

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
| typo | `resident evl` | 10 | Resident Evil 4 (PC) Steam Key GLOBAL |
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
| synonym | `gta 5` | 1 | Grand Theft Auto V Enhanced (PC) Steam Key GLOBAL |
| synonym | `gta v` | 1 | Grand Theft Auto V Enhanced (PC) Steam Key GLOBAL |
| synonym | `grand theft auto 5` | 1 | Grand Theft Auto V Enhanced (PC) Steam Key GLOBAL |
| synonym | `dark souls 3` | 2 | DARK SOULS III (PC) Steam Key GLOBAL |
| synonym | `civilization 6` | 2 | Sid Meier’s Civilization VI (PC) Steam Key GLOBAL |
| synonym | `civ 6` | 2 | Sid Meier’s Civilization VI (PC) Steam Key GLOBAL |
| synonym | `ff 16` | 1 | FINAL FANTASY XVI (PC) Steam Key GLOBAL |
| synonym | `re 4` | 3 | Resident Evil 4 (PC) Steam Key GLOBAL |
| synonym | `spiderman` | 3 | Marvel’s Spider-Man Remastered (PC) Steam Key GLOBAL |
| synonym | `halflife 2` | 2 | Half-Life 2 (PC) Steam Key GLOBAL |
| synonym | `repo` | 3 | R.E.P.O. (PC) Steam Key GLOBAL |
| spelling | `THE WITCHER` | 2 | The Witcher 3: Wild Hunt — Remastered (PC) Steam Key GLOBAL |
| spelling | `baldurs gate` | 2 | Baldur's Gate 3 (PC) Steam Key GLOBAL |
| spelling | `baldur's gate 3` | 2 | Baldur's Gate 3 (PC) Steam Key GLOBAL |
| spelling | `half-life` | 8 | Half-Life 2 (PC) Steam Key GLOBAL |
| spelling | `elden   ring` | 1 | ELDEN RING (PC) Steam Key GLOBAL |
| spelling | `ragnarok` | 3 | God of War Ragnarök (PC) Steam Key GLOBAL |
| spelling | `god of war ragnarök` | 3 | God of War Ragnarök (PC) Steam Key GLOBAL |
| platform | `skyrim psn` | 1 | The Elder Scrolls V: Skyrim (PS5) PSN Key GLOBAL |
| platform | `terraria xbox` | 1 | Terraria (Xbox Series X\|S) Xbox Live Key GLOBAL |
| no match | `zzxqv` | 0 | (none) |
| no match | `qwerty` | 0 | (none) |
| no match | `mincraft` | 0 | (none) |
| no match | `asdf ghjk` | 0 | (none) |

`mincraft` was one of the example queries in the plan. Minecraft is not sold on Steam, so it is
not in the catalog; the correct result is "nothing", and the table checks that the search does
not offer the nearest wrong game instead.

### Query suggestions (dropdown, top part)

| Typed | Suggestions, in order |
|---|---|
| `resid` | resident evil, resident evil 2, resident evil 3, resident evil 4, resident evil 7 |
| `gta` | gta, gta 5, gta v |
| `witcher` | witcher, witcher 3, witcher 3 remastered, the witcher 3 |
| `witchr` | witcher, witcher 3, the witcher 3, witcher 3 remastered |
| `elden` | elden ring, the elder scrolls |
| `stard` | stardew valley, starfield, sea of stars, star wars jedi |
| `souls` | dark souls, dark souls 3 |
| `zzxqv` | (none) |

These are with no visitor searches recorded; in production, frequently searched queries move up.

## Known limits

- **Heavier misspellings:** a word with several errors falls below the threshold (`asasin` scores
  0.40 against *Assassin's*; `assasins` at 0.80 is found). Two typos in one short query also fail
  (`eldn rng`).
- **Very short typos tie:** `eldn` alone scores 0.60 against both *Elden Ring* and *The Elder
  Scrolls*, so popularity decides the order. `eldn ring` is unambiguous.
- **Synonyms are hand-written.** A new game with an abbreviation needs a row in
  `search_synonyms` (single-word abbreviations like `bg3` work without one).
- **Index use:** the conditions are written in the forms the trigram GIN index supports
  (`LIKE`, `~` and `<%`); the explicit `word_similarity() >= 0.5` for short words is not. With
  476 products PostgreSQL normally reads the whole table, which is faster at that size. Not
  measured with a large catalog.
- **Test time:** the curated-suggestion test runs about 190 searches against Neon and takes
  about a minute locally (seconds in CI, where Postgres runs next to the tests).

## Should we switch to Meilisearch or Typesense?

Researched 2026-11-17. Both are open-source search engines built for exactly this (typo
tolerance, prefix search, synonyms, ranking rules), and both would rank better out of the box.

| | Meilisearch | Typesense | Postgres (current) |
|---|---|---|---|
| Hosted free tier | Sources disagree; most say Cloud has only a 14-day trial, paid from about $20–30/month[^m] | Most sources describe a one-time credit (about 720 cluster-hours, roughly one month), then paid[^t] | Already on Neon's free tier |
| Self-hosted on our stack | Needs an always-on process with disk. Render's free web service sleeps after 15 min and has no persistent disk, so the index would be rebuilt on every wake-up | Same | — |
| Work now | New service + deploy, an indexer that pushes products, offers and stock on every change, facet counts moved to the engine, two systems to keep in sync | Same | Done; 49 queries tested |
| What we would gain | Better typo handling (e.g. `asasin`), built-in synonyms, highlighting | Same | — |

**Recommendation: stay on Postgres.** There is no lasting free hosting that fits our setup.
A second source of truth for prices and stock would also add the problem we most want to avoid
in a marketplace: search showing an offer that the database has already sold. The current search
passes every query in the tables above. I would revisit this when the catalog grows to tens of
thousands of products, or if search quality becomes the main complaint. The README will state
this trade-off.

[^m]: costbench.com (Aug/Oct 2026: "no free tier"), toolradar.com (read the pricing page on
  2026-11-03: Cloud from $20/month), meilisearch.com/blog/typesense-pricing (14-day trial).
  The official pricing page was not read directly; check it before relying on this.
[^t]: agentdeals.dev, api-pricing.pages.dev, xpay.sh (May 2026: no free tier advertised). The
  official Typesense Cloud pricing page could not be read; check it before relying on this.
