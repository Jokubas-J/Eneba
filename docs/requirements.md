# Requirements analysis — Eneba Intern Assignment 2027

Source: `Eneba Internship Assignment.pdf` (2 pages). Every sentence of the brief is mapped to a
checklist item below. IDs (F = functional, T = technical, D = deliverable, P = process) are used in
`PLAN.md`.

## 1. Process requirements

> Status 2026-11-19 (day 7): everything built and verified is ticked. Still open: P1 (sending the
> results, by the author), P2/D3 (the final log export, at the end of the last session) and P4 (the
> author's own ability to explain the code).

- [ ] **P1** Finish within 7 days of receiving the assignment; reply to the registration e-mail with the results.
- [ ] **P2** Save a **full** log of AI conversations (prompts and responses) and submit it. → `ai-logs/`
- [x] **P3** Show *how* AI was used (they evaluate the process, not only the result).
- [ ] **P4** Be able to explain the code.
- [x] **P5** Decide the scope yourself and **justify** it ("shortcuts are allowed, specs are intentionally vague").
- [x] **P6** "A few features done extensively beat many fitting minimal requirements."

## 2. Functional requirements

### Store (catalog)
- [x] **F1** A catalog page where visitors can browse **every** product.
- [x] **F2** Filters that narrow the list (examples given: platform, region, price).
- [x] **F3** Change the order (sorting).
- [x] **F4** Search by name inside the store.

### Homepage
- [x] **F5** Landing page assembled from **several different blocks** (examples: banner, best sellers, deals).
- [x] **F6** Each block gets its content from **its own backend service or database**.
- [x] **F7** Frontend only displays what it receives — **nothing hardcoded** in the frontend.

### Search
- [x] **F8** Search by product name from **anywhere** in the app (→ global search in the header).
- [x] **F9** See the matching results.
- [x] **F10** Cope with **typos** "where it can".
- [x] **F11** Cope with **partial names** "where it can".

### Product page
- [x] **F12** Page for a single product showing its details.
- [x] **F13** Shows **offers from several sellers** so a buyer can compare and choose.

### Checkout
- [x] **F14** Put products in a cart.
- [x] **F15** Pay — simulated or a payment provider's test mode; no real money.
- [x] **F16** Receive the **digital key** that was bought.

### Authentication
- [x] **F17** Create an account.
- [x] **F18** Sign in.
- [x] **F19** Some parts are for signed-in users only — named examples: **checkout** and **order history**
      (so an order history page is implied).

### Styling
- [x] **F20** Looks and feels like Eneba: colors, typography, spacing and components follow the
      design reference. Applies to **every screen** (stated in the flow diagram).

### Purchase flow (diagram in the PDF)
- [x] **F21** Entry points Homepage and Search → Product page → Cart → Checkout → *(sign-in
      required)* → Payment → Order and key.

## 3. Technical requirements

- [x] **T1** React frontend (TypeScript preferred).
- [x] **T2** Backend in PHP or Go.
- [x] **T3** REST or GraphQL API.
- [x] **T4** Any database; decide the data model yourself.
- [x] **T5** Architecture, data model and scope are ours to decide (and to justify).

## 4. Deliverables ("no evaluation without the following")

- [x] **D1** GitHub link(s) to the project.
- [x] **D2** Public URL of the app deployed on a free-tier service (e.g. Render or Vercel).
- [ ] **D3** Agent chat history — full, prompts and responses — as a Markdown file or a link.

## 5. Where the spec is vague, and the options

| # | Vague point | Options | Chosen in PLAN.md |
|---|---|---|---|
| V1 | **"Its own backend service or database"** for each homepage block | (a) one endpoint returns the whole page; (b) one backend, but each block has its own endpoint, its own service module and its own data source, and the list of blocks itself comes from the backend; (c) truly separate deployed microservices / databases per block | **(b)** — satisfies "own service" and "nothing hardcoded" without running five sleeping free-tier servers |
| V2 | **"Nothing is hardcoded there"** | (a) only the data is dynamic; (b) data **and** which blocks exist, their order and titles are dynamic | **(b)** — block layout comes from a `home_blocks` table |
| V3 | **"Cope with typos and partial names where it can"** — how tolerant? | (a) substring match only; (b) trigram fuzzy matching in Postgres; (c) dedicated search engine (Meilisearch/Typesense) | **(b)** plus prefix/substring; with a written list of test queries |
| V4 | **Search vs. store search** — two features or one? | (a) separate results page; (b) header search with instant dropdown that lands on the store page with `?text=` (what Eneba does) | **(b)** |
| V5 | **What is a "product"?** | (a) a game; (b) a sellable variant: game + platform + region (what Eneba does) | **(b)** — makes platform/region filters natural |
| V6 | **Which filters / sort orders?** ("for example platform, region or price") | Minimum: those three. More: product type, genre, facet counts | platform, region, price range, genre + counts; sort by popularity, price ↑↓, newest, name |
| V7 | **"Pay"** — simulated or provider test mode | (a) fake "Pay" button; (b) Stripe test mode with webhooks; (c) both behind one interface | **(c)** — Stripe test mode is the real path, a simulated provider is the fallback and is used in tests |
| V8 | **"Receive the digital key"** — how? | (a) show on the order page; (b) e-mail it; (c) both | **(a)** — on the order page and in order history; e-mail is out of scope |
| V9 | **Where do keys come from? Can one be sold twice?** Not mentioned | (a) generate a random key at purchase time; (b) pre-loaded stock of keys per offer, each sold exactly once | **(b)** — this is the real marketplace problem, and a good place to go deep |
| V10 | **Cart** — before or after sign-in? Stored where? | (a) server-side cart for signed-in users only; (b) browser-stored cart for everyone, validated by the server at checkout | **(b)** — the flow diagram puts sign-in *after* the cart |
| V11 | **Authentication depth** | e-mail + password only / e-mail verification / password reset / social login | e-mail + password only; rest out of scope (documented) |
| V12 | **"Design reference"** — none is attached | The live eneba.com site is the reference | Extracted into `docs/design-reference.md` |
| V13 | **Seller side** (who creates offers?) | (a) seeded data only; (b) seller dashboard | **(a)** — sellers, offers and key stock are seeded |
| V14 | **Catalog data** — "decide on the data you need yourself" | hand-written seed / generated / imported from a public games API | Steam store data fetched **once** by a seed script into our database; never called at runtime; sellers, prices, stock and keys are generated |
| V15 | **Currency, language, regions of the visitor** | multi-currency + i18n like Eneba / single currency | EUR and English only |
| V16 | **Responsive / mobile** — not mentioned | desktop only / responsive | Responsive basics (grid collapses, filters become a drawer if time allows) |
| V17 | **Admin features** (edit banners, blocks) | none / admin UI | None — content is edited in the database/seed; noted as future work |
| V18 | **"Github link(s)"** — one repo or two? | monorepo / separate frontend and backend repos | Monorepo (`frontend/`, `backend/`) |
| V19 | **7 days — from when exactly?** | — | Planned as 7 days starting 2026-11-16; the author tracks the exact deadline |
