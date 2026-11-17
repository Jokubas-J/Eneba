# Gap analysis: eneba.com vs. our app

Walked through both sites side by side with Playwright on **2026-11-17** (desktop, 1568 px,
logged out). Eneba: `https://www.eneba.com`. Ours: `https://eneba-seven.vercel.app` at commit
`b85eb79`. Screenshots of this session are in `.playwright-mcp/` (git-ignored); the reference set
is `docs/screenshots/`.

**Visibility** = how likely an evaluator is to notice it in the first minutes.
**Effort** = S (under 2 h), M (half a day), L (a day or more).

## 1. Ranked gaps

| # | Area | Eneba | Ours | Visibility | Effort | Plan |
|---|---|---|---|---|---|---|
| 1 | **Homepage** | Banner mosaic, category icon rail, 6–8 product carousels, category tiles | A test page that shows "Backend status: ok" | Very high — first thing anyone sees | L | PLAN day 6 (deep C). Could be moved earlier. |
| 2 | **Product page** | Cover, title, info tiles, offer list, buy box, description, reviews, requirements | "Page not found" for every product link | Very high — every card click | L | PLAN day 4 (deep B). |
| 3 | **Search dropdown** | Opens while typing: 5 query suggestions, 6 product rows (cover, "Digital good" pill, name, "From ~~€x~~ €y"), "Show all N results"; ARIA combobox/listbox | None; Enter goes to the store | High — the box is on every page and evaluators type into it | M | **Built in this step**, see §3. |
| 4 | **Categories menu** | "☰ Categories" opens a left drawer: All offers · Games (Platforms, Popular genres, Niche genres, Discover by price, Upcoming, DLCs…) · Gaming eCards · eGift Cards · E-money · PSN · Xbox · … | "☰ All games" is a plain link | High — first item of the nav strip | M | Proposal in §2, waiting for the author's decision. |
| 5 | **Common search spellings** | `gta 5` → 229 results, suggestions "gta 5", "gta v" | `gta 5` → **0 results** (name says "V") | High for anyone testing search | S–M | **Built in this step** (synonyms). |
| 6 | **Nav strip content** | Categories, Cheap Games, Software, eGift Cards, Eneba Gift Cards, FM27, Giveaway, promo pill "GTA VI" | All games, Steam, GOG.com, Epic Games, Xbox Live, PSN, Cheapest first | Medium — looks like Eneba already, the labels differ | S | Comes with the Categories menu (same data). |
| 7 | **Store sidebar extras** | Collapsible groups (chevron), "Country" select, "Operating system" group, search box inside long groups, "Show all" | Fixed open groups; no Country / OS | Medium | S–M | Collapsing: S, day 6. Country/OS: see §4. |
| 8 | **Store sort control** | Icon + "Popularity: Most popular ⌄" custom dropdown | Native `<select>` with the same options | Low–medium | S | Day 6 polish. |
| 9 | **Product card details** | Cashback badge, ⓘ next to the price, hover reveals "Add to cart" / "View offers", wishlist heart | Cover, platform strip, title, region, price | Medium | M | Hover actions need the cart (day 4). Cashback: out of scope. |
| 10 | **Pagination look** | Numbered buttons with « » arrows | Numbered links with Previous / Next | Low | S | Day 6. |
| 11 | **Header icons** | Flag + "English EU \| EUR" selector, wishlist heart, cart with count bubble | Text "English EU \| EUR", cart icon, no count | Low–medium | S | Cart count: day 4. |
| 12 | **SEO text block under the store** | Several paragraphs ("Buy games, gift cards…") | None | Low | S | Not planned. |
| 13 | **Promo widgets** | "BE GTA VI READY!" countdown, cookie banner, location prompt | None | Low | — | Not planned (marketing). |

What already matches closely: colors, fonts, the header layout and search box size, the nav
strip, the store's two-column layout, filter groups with live counts, active-filter chips with
"Clear all", "Results found", typo-tolerant results.

## 2. Categories menu — proposal (not built)

### What Eneba's menu contains

A drawer from the left with nested levels. Measured structure (trimmed):

```
All offers
Mobile games ›          (PUBG UC, Genshin Impact, …)        ← top-ups: out of scope
Games ›
   All offers · Cheap Steam games · Trending now
   Platforms ›          Steam, PSN, Xbox, Nintendo Switch, Epic, Origin, Ubisoft, GOG, Battle.net
   Upcoming Games · DLCs
   Popular genres ›     Singleplayer, Multiplayer, Action, …
   Niche genres ›       Indie, RPG, Horror, …
   Skins & Bundles ›                                        ← out of scope
   Discover by price ›  Games under €1 / €2 / €5 / €10 / €25 / €50 · €50 and more
Gaming eCards › · eGift Cards › · E-money cards ›          ← out of scope
PSN › · Xbox › · …
Gift Guide
```

### Proposed menu for our catalog

Only what our data can fill. Every entry is a link into the store with filters in the URL, so
the menu adds no new page and no new API for results.

```
All games                                   /store
Platforms ›   Steam 402 · PSN 21 · Xbox Live 19 · Epic Games 17 · GOG.com 17
Genres ›      Action 321 · Adventure 226 · RPG 166 · Indie 142 · Simulation 96 · …
Regions ›     Global · Europe · United States
By price ›    Under €5 · Under €10 · Under €25 · Under €50 · €50 and more   (with counts)
Deals ›       Biggest discounts · Cheapest first · New releases · Most popular
```

Numbers are live counts from the database, as in the store sidebar. "Biggest discounts" needs a
new sort option (`sort=discount`); everything else uses existing filters.

**Desktop:** "☰ Categories" opens a drawer on the left (as on Eneba) with the five groups;
hovering or focusing a group shows its items in a second column. **Mobile:** the same drawer,
full width, a group opens in place. Keyboard: Escape closes, arrow keys move, focus returns to
the button.

### Where the menu content comes from (decision needed)

The content must come from the backend. Two ways to store it:

| Option | How | For | Against |
|---|---|---|---|
| **A. Generated** | `GET /api/menu` builds the tree in Go: platforms, genres and regions from their lookup tables (with counts); price buckets and deal links as a fixed list in the Go service. | Nothing to maintain: a new genre appears by itself. Small. | Changing order or labels of the fixed parts needs a deploy. |
| **B. Table-driven** (recommended) | Tables `menu_sections` (title, kind, order) and `menu_links` (section, label, URL, order). A section's `kind` is either `links` (rows from `menu_links`) or `platforms` / `genres` / `regions` (rows generated from the lookup table, with counts). | Same idea as `home_blocks` on the homepage: what the menu shows is data, order and labels change without a deploy, and the generated sections stay automatic. | Two more tables and a migration. |

I recommend **B**, because it is the same pattern as the backend-driven homepage (V1/V2 in the
brief), so one explanation covers both. The nav strip links (§1 row 6) would become one more
`menu_sections` row of kind `links`.

Effort: about half a day (migration, endpoint with tests, drawer component).

## 3. Search dropdown — what is built in this step

See `docs/search.md` for the full design. In short:

- Opens after **2 characters**, requests are **debounced (200 ms)** and the previous request is
  **cancelled** when the text changes.
- Top section: up to 5 **query suggestions** from a `search_suggestions` table (a curated list for
  our catalog, plus queries visitors searched that returned results).
- Below: up to 6 **product rows** (cover, platform pill, name, "From ~~€x~~ €y"), then
  "Show all N results".
- A `search_synonyms` table: `gta ↔ grand theft auto`, roman numerals ↔ digits, a few common
  abbreviations. Synonyms apply to the full search (Enter) as well, so `gta 5` finds GTA V.
- Keyboard and screen-reader behaviour from **Downshift** (`useCombobox`); the choice against
  React Aria is in `docs/tooling.md`.

## 4. Out of scope for our catalog, and why

| Eneba feature | Why we leave it out |
|---|---|
| Gift cards, eGift cards, gaming eCards, e-money, mobile top-ups (PUBG UC, V-Bucks…), skins, software, subscriptions | Our catalog is games from the Steam store snapshot. These are different product types with different delivery (codes for a balance, direct top-up to a game account) and no Steam data to seed them with. Building them would mean inventing a second catalog — breadth, which the brief warns against. |
| Mobile games menu, "Gift Guide", Eneba Gift Cards, giveaways, FM27 / GTA VI promo pills | Marketing or Eneba's own products, not marketplace mechanics. |
| Cashback badges and amounts | A loyalty-program feature with its own rules and wallet; the brief does not ask for it. |
| Wishlist heart | Needs accounts plus another table and page; not in the brief's list. Candidate for "next steps". |
| Language / currency switch, flag, country detection, "Country" filter | One language and EUR only (PLAN §1). The region color on cards assumes an EU visitor instead. |
| Operating system filter | Steam's data has the flags, but every product in our catalog is a PC/console key and OS adds little; could be added from the snapshot later if wanted. |
| Reviews, Trustpilot, seller pages | Not in the brief; the seller rating on the offer list (day 4) covers trust. |
| SEO text, cookie banner, location prompt, countdown widget | No tracking cookies are set; the rest is marketing. |

## 5. Suggested order after this step

1. Categories menu (§2) — once the author picks A or B.
2. Product page with offer comparison, cart, auth (PLAN day 4) — the biggest visible gap after
   the homepage, and the start of deep feature B.
3. Homepage (PLAN day 6) — could be pulled forward because it is the most visible gap; its block
   API is independent of checkout.
