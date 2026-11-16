# Eneba design reference

Extracted from the live site **eneba.com on 2026-11-16** (desktop, 1568 px wide viewport, English EU / EUR,
logged out). Values come from the browser's *computed styles* (read with JavaScript in the page), not
from guessing off screenshots. Screenshots are in [`docs/screenshots/`](screenshots/).

> How to use this file: all UI work must take colors, font sizes, spacing and component shapes from
> here. Where a value is marked *(approx.)* it was estimated from a screenshot rather than measured.

## 1. Overall look

- **Dark, saturated purple everywhere.** There is no white page background; white is only used for
  text, thin outlines and the payment-logo strip.
- **Completely square corners.** Cards, buttons, inputs and panels all have `border-radius: 0`.
  The only rounded things are pill badges (`50px`), checkboxes (`3px`) and one promo nav pill (`5px`).
- **Very bold type.** Almost all UI text is weight 800; body copy is 400.
- **Flat.** Shadows are rare and soft; depth comes from darker/lighter purple panels.
- **One loud accent:** yellow call-to-action buttons with black text.
- Content is centred in a **1200 px** container (1240 px including 20 px side padding).

## 2. Color palette

| Token (suggested name) | Hex | Where Eneba uses it |
|---|---|---|
| `purple-base` | `#4618AC` | Page background (`body`) |
| `purple-light` | `#5825CC` | Raised panels: buy box, category tiles, search dropdown, mobile filter header |
| `purple-dark` | `#351183` | Sunk panels: cart/summary panels, notice box, alternating homepage bands, checkout top bar |
| `purple-deep` | `#1F0A4D` | Product card body, text color on teal badges, inset shadow color |
| `overlay-nav` | `rgba(0,0,0,0.25)` | Nav bar strip under the header |
| `overlay-row` | `rgba(0,0,0,0.30)` | Offer rows on the product page |
| `overlay-input` | `rgba(0,0,0,0.20)` | Inputs/selects background (with inset shadow) |
| `overlay-item` | `rgba(0,0,0,0.12)` | Cart line item |
| `overlay-footer` | `rgba(0,0,0,0.70)` | Footer background (over `purple-base` → almost black-purple) |
| `yellow` | `#FAD318` | Primary buttons (Add to cart, Buy now, Proceed to checkout), "Clear all" link, notice title, "FEATURED OFFER" label |
| `teal` | `#63E3C2` | CASHBACK badge background, product card border |
| `green-region` | `#23C299` | Region label when the key works in the visitor's country; "Cashback deal" pill outline |
| `green-cashback` | `#84E916` | Cashback amounts, discount percent (`-69%`), "Best cashback" pill |
| `pink-region` | `#E76A8C` | Region label when the key is for a different region (warning) |
| `text` | `#FFFFFF` | Main text |
| `text-nav` | `#DCD8E6` | Nav links, footer links |
| `text-muted` | `#B3AAC9` | "From" label, legal text, helper text |
| `text-faded` | `rgba(255,255,255,0.7)` | Filter counts, struck-through old price |
| `black` | `#000000` | Text on yellow buttons |
| `white-strip` | `#FFFFFF` | Payment-methods strip above the footer; PayPal button |
| promo gradient | `linear-gradient(90deg, #D32252 0%, #3C1395 100%)` | Highlighted nav pill ("GTA VI") |

## 3. Typography

- **Font family:** `metropolis, Arial, Helvetica, sans-serif`. Loaded weights: **400, 600, 800**
  (700 is requested for headings and rendered with the nearest loaded weight).
  Metropolis is an open-source font (available e.g. as the `@fontsource/metropolis` npm package),
  so we can use the same typeface legally.
- Base: `16px / 24px`, weight 400, white.

| Element | Size / line-height | Weight | Color |
|---|---|---|---|
| Page title `h1` (store) | 30 / 37.5 px | 700 | white |
| Product title `h1` | 26 / 32.5 px | 700 | white |
| Section heading `h2` (homepage blocks) | 30 / 37.5 px | 700 | white |
| Nav link | 16 / 16 px | 600 | `#DCD8E6` |
| Search input text | 19 / 19 px | 800 | white (placeholder white) |
| Button label | 15 / 15 px | 800 | black on yellow / white on outline |
| Card title | 12 / 18 px | 800 | white (2 lines max, then ellipsis) |
| Card region label | 12 / 12 px | 800, UPPERCASE | green or pink |
| Card "From" | 12 / 12 px | 800 | `#B3AAC9` |
| Card price | 22 / 22 px | 800 | white |
| Card cashback line | 12 / 20 px | 800 | `#84E916` |
| Card platform strip | 10–12 px | 800 | white 85% |
| Buy-box price | 28 / 28 px | 800 | white |
| Offer-row price | 20 / 25 px | 800 | white |
| Old (struck) price | 12 / 18 px | 800 | white 70% |
| Seller name | 14 px | 600 | white |
| Seller rating | 11 px | 800 | white 85% |
| Filter group title | 16 / 22.4 px | 800 | white |
| Filter option / count | 14 px | 400 | white / white 70% |
| Footer column title | 14 / 21 px | 800 | white |
| Footer link | 12 / 18 px | 400 | `#DCD8E6` |
| Legal text | 12 / 18 px | 600 | `#B3AAC9` |
| Tiny label ("FEATURED OFFER") | 10 / 15 px | 800, UPPERCASE | yellow |

## 4. Spacing, radius, shadows

- **Container:** max-width 1200 px, side padding 20 px.
- **Spacing scale seen in the wild:** 5, 10, 15, 20, 40 px (multiples of 5, not 4 or 8).
  - Card text block padding: `15px 15px 10px`
  - Offer row padding: `20px`
  - Button horizontal padding: `10px`; nav link padding: `10px`
  - Homepage sections: `40px` top padding
  - Card gap in carousels/grids: ≈ 20 px *(approx.)*
- **Border radius:** `0` by default. `50px` pills, `3px` checkbox, `5px` promo nav pill.
- **Borders:** 1 px. White for outline buttons and the search box; teal `#63E3C2` around product
  cards; yellow around the notice box.
- **Shadows (only three):**
  - Raised panel (buy box): `0 3px 9px rgba(0,0,0,0.25)`
  - Category tile: `0 7px 15px rgba(0,0,0,0.2)`
  - Input (inset): `inset 0 1px 3px #1F0A4D`

## 5. Components

### Header (screenshot 01)
- Transparent over the purple page, total height ≈ 147 px in two rows:
  1. **Top row (57 px):** logo (left) · search box · language/currency selector with flag
     ("English EU | EUR") · wishlist heart · cart icon (with red count bubble) · "Log in | Register".
  2. **Nav strip (50 px):** background `rgba(0,0,0,0.25)`, full width. "☰ Categories" first, then
     text links (16 px / 600 / `#DCD8E6`), one highlighted promo pill with the pink→purple gradient.
- **Checkout header** is different: logo + 3-step progress bar ("1 Cart — 2 Payment — 3 Get your
  product", active step in teal) on `#351183`, no search or nav (screenshot 14).

### Search bar (screenshots 01, 09)
- 550 × 57 px, transparent, **1 px white border**, no radius. Magnifier icon on the left
  (input has 40 px left padding), clear "✕" on the right when text is present.
- Text 19 px / 800; placeholder "Search for games, top-ups and more" in white.
- **Dropdown** (panel `#5825CC`, same width as the input) has two parts:
  1. up to 5 **query suggestions** (corrected spellings — typing `witchr 3` suggests `witcher 3`,
     `witcher 3 remastered`, …), each with a small magnifier icon;
  2. **product hits**: thumbnail, pill "Digital good", bold title, right-aligned "From ~~€49.99~~" and
     price. The page behind is dimmed.
- Pressing Enter goes to the store with `?text=<query>`.

### Product card (screenshots 02–05)
- ≈ 222 × 456 px, background `#1F0A4D`, **1 px teal border `#63E3C2`**, no radius.
- Top: cover image 222 × 311 px, `object-fit: cover` (portrait, ratio ≈ 5:7).
- Over the image bottom-left: **CASHBACK badge** — teal `#63E3C2`, text `#1F0A4D` 12 px / 800,
  padding `7px 5px`, small icon, height 30 px.
- Directly under it, overlapping the image bottom: **platform strip** — full width, 22 px high,
  `rgba(0,0,0,0.55)`, centred platform icon + name ("Steam", "PSN", "Xbox Live").
- Text block (padding `15px 15px 10px`):
  - title, 2 lines clamp;
  - REGION in uppercase — **green `#23C299`** if usable in your country, **pink `#E76A8C`** if not;
  - "From" + optional ~~old price~~ and `-49%` in green;
  - price 22 px / 800 with a small ⓘ icon;
  - "Cashback: €1.69" in green.
- **Hover:** the image area shrinks, a wishlist heart appears top-right, and two full-width buttons
  slide in at the bottom: yellow **Add to cart** and outline **View offers** (screenshot 03).

### Buttons
| Variant | Background | Text | Border | Size |
|---|---|---|---|---|
| Primary | `#FAD318` | black, 15 px / 800 | none | height 35 px, padding `0 10px`, radius 0 |
| Secondary / outline | transparent | white, 15 px / 800 | 1 px white | same |
| "Show all" / "Load more" | outline variant, ≈ 194 × 35 px, centred under a block | | | |
| White (PayPal) | `#FFFFFF` | logo | none | same height |

### Badges
- **CASHBACK** (card): square teal block, see above.
- **Pill outline** ("Cashback deal", "Best cashback", "Digital good"): transparent, 1 px border in
  the text color, radius 50 px, 10–16 px / 800, padding `3px 5px` (small) or `0 12px` (large).
- **Discount** (`-69%`): plain green text `#84E916` next to the struck old price, no background.
- **Cart count:** small red circle on the cart icon.

### Offer row (product page, screenshot 12)
- Full-width row, background `rgba(0,0,0,0.3)`, padding 20 px, ≈ 109 px high, small gap between rows.
- Left: round seller avatar (≈ 40 px), **seller name** (14 px / 600) with a green verified tick,
  below it ★ **rating "9.80"** + "Superb rating" (11 px).
- Right: ~~old price~~ + green `-62%`, **price** 20 px / 800, "11% Cashback" line, then an outline
  **Buy now** button and a small cart-plus icon button.
- Lists: "Recommended offers" (featured offer "Verified by Eneba" first), then
  "**15** other offers" (count in teal), then an outline "Load 12 more offers" button.

### Buy box (product page right column, screenshot 11)
- Panel `#5825CC`, shadow `0 3px 9px rgba(0,0,0,.25)`, ≈ 310 px wide, sticky on the right.
- "FEATURED OFFER" (10 px yellow caps) · ~~€49.99~~ `-69%` · **€15.57** (28 px) · "14% Cashback" +
  "Best cashback" pill · promo countdown text · white PayPal button · yellow **Buy now** + icon-only
  add-to-cart · link "+15 offers from €14.48".
- Under it: "Safe Checkout Guaranteed" panel with payment-method logos.

### Product page top (screenshot 11)
- Breadcrumbs (Store › Categories › Games › GOG Games › title).
- Cover image left (≈ 220 px), title `h1`, then a 2 × 2 grid of **info tiles**, each icon + bold
  title + small text + yellow link: *Region* ("Global — Can be activated in Lithuania"),
  *Platform* ("GOG.com — Activate/redeem on…"), *Digital key* ("Instant delivery"),
  *Instant refunds*. Then "Works on: Windows".
- Screenshot/video strip, a **notice box** (`#351183`, 1 px yellow border, yellow bold title).
- Lower: offers list, "Gamers also viewed", reviews, description, system requirements, languages,
  release date / publisher / developers.

### Filters sidebar (store, screenshots 08, 10)
- 220 px wide column left of the grid. Each group is a panel with a bold title (16 px / 800) and a
  collapse chevron: **Price range (EUR)** (two inputs "min – max"), **Country** (select "All
  countries"), **Product type**, **Operating system**, **Platforms**, **Genres**, **Regions**.
- Options are checkboxes (20 × 20 px, 2 px white-70% border, radius 3 px) with the label left and
  the **result count right** (white 70%). Long groups show 8 options then "292 more"; big groups
  have a small search box inside.
- Inputs/selects: `rgba(0,0,0,0.2)` background, inset shadow, height 40 px, no radius.
- Above the grid: "Results found: **82**" left, "⇅ Popularity: Most popular ⌄" sort dropdown right.
  Sort options: Popularity, Price low→high, Price high→low, Date newest, Date oldest, Alphabet A–Z.
- **Active filters** are shown as dark chips above the results ("Text: witchr 3 ✕") with a yellow
  "Clear all" link. Counts next to each filter update to match the current result set.
- Grid: 4 cards per row beside the sidebar at this width.

### Homepage blocks (screenshots 01–06), in order
1. **Banner mosaic:** one large promo + three smaller tiles, full container width, ≈ 310 px high.
2. **Category icon rail:** horizontally scrollable white line icons with labels (Games, Gaming
   eCards, eGift Cards, E-money, Steam, PSN, Xbox, …) with arrows.
3. **"Recommended for you"**: row of 5 product cards + centred outline "See all".
4. **"Top games"**: carousel of product cards on a darker band (`#351183`), arrows on the sides,
   thin progress bar under it, "Show all".
5. **"Best selling eGift Cards"**: carousel of brand tiles.
6. **"Best selling gift cards"**: card carousel on a darker band.
7. **"Upcoming games"**: card carousel.
8. **"Categories"**: grid of square tiles (`#5825CC`, shadow) — count on top, white icon, name
   (Action 20698, Adventure, Simulation, Strategy, RPG, Indie, FPS/TPS) + "Load more".
- Section headings are `h2` 30 px with a small ⓘ icon; bands alternate `#4618AC` / `#351183`.

### Cart & checkout (screenshots 13–15)
- **Added-to-cart modal:** centred panel `#5825CC`, cart icon with teal tick, "Added to cart",
  "You have **1 item** in your cart", buttons yellow "Continue shopping" + outline "View cart";
  page dimmed behind.
- **Cart page** (`/checkout`): two columns.
  - Left panel "My cart" (`#351183`): line item (`rgba(0,0,0,.12)`) with cover thumbnail,
    "Sold by ⟨seller⟩", product title, "Digital product", quantity stepper `− 1 +`, price, green
    cashback, wishlist and trash icons.
  - Right: "Where to deliver the order?" (email field, "Remember me") and **Summary** panel: yellow
    full-width "Proceed to checkout", "1 product €15.57", "Service fee", **Total** (large), cashback,
    consent checkboxes (yellow when ticked), collapsible "Got discount code?".
- **Empty cart:** ⓘ icon, "Your cart is empty", yellow "Back to Homepage".
- I stopped at the cart: going further needs an e-mail address and a real payment method.

### Footer (screenshot 07)
- White strip with payment logos, then footer on `rgba(0,0,0,0.7)` (reads as near-black purple).
- Link columns: About Eneba · Buy · Help · Community · Business · Follow us (icons) · Download the
  App (store badges) · Trustpilot score.
- Newsletter row: outline input + outline "Subscribe" button; language/currency button on the right.
- Legal line 12 px `#B3AAC9` with underlined links.

## 6. How Eneba's data model appears to work

These are **inferences from the public site**, not documented facts.

- **Product = one sellable variant, not "a game".** The unit shown on a card and given its own URL
  is the combination *title + edition + platform + region*, e.g.
  `/gog-the-witcher-3-wild-hunt-goty-gog-com-key-global`. "The Witcher 3 (Xbox One) Xbox Live Key
  EUROPE" is a separate product with its own page. Search for one game returns many such products.
- **Offer = one seller's price for one product.** A product page lists many offers
  ("+15 offers from €14.48"). Each offer has a seller, a price, an old/reference price (shown as a
  discount %), a cashback %, and flags such as "Featured offer / Verified by Eneba".
  The card's "From €X" is the cheapest offer; the product page promotes one **featured offer** in
  the buy box (not necessarily the cheapest).
- **Seller (merchant):** name, avatar, verified tick, rating out of 10 with a label ("9.80 Superb").
- **The cart holds offers, not products:** the cart line says "Sold by Digital Imperium".
- **Platform** = where the key is redeemed (Steam, GOG.com, Xbox Live, PSN, Nintendo, EA App…),
  separate from **operating system** (Windows / Mac / Linux) which is its own filter.
- **Region** = where the key can be activated (Global, Europe, United States, a single country…).
  The site compares it with the visitor's country and colors it green ("Can be activated in
  Lithuania") or pink.
- **Product type** (Game, DLC, eGift Card, Gaming eCard, Game points, Software, Subscription) and
  **genres** (many per product) are further facets. Every facet shows a live count.
- **Product details:** description, gallery, system requirements, languages, release date,
  publisher, developer, user reviews with sub-ratings, wishlist count.
- **Store/search state lives in the URL** (`/store/all?text=witchr%203`), so results are shareable.
- **Search** tolerates typos (`witchr 3` → 82 Witcher results) and offers query suggestions plus
  instant product hits in a dropdown.
- **Checkout** is a 3-step flow: Cart → Payment → Get your product. Guests can check out with just
  an e-mail on the real site (our assignment instead requires sign-in for checkout).
- **Homepage blocks** are independent curated/computed lists (recommended, top games, best-selling
  gift cards, upcoming, categories with counts), each with its own "show all" link into the store.
