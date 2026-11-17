# Journal

A few lines per working day: what was built, what was decided and why. The full detail is in the
commit history and in `ai-logs/`.

## 2026-11-16 — PLAN day 2 (skeleton + data), done on the evening of day 1

**Built**
- Backend: Go `net/http` server with `/api/healthz`, configuration from environment variables,
  pgx connection pool, goose migrations embedded in the binary (pg_trgm + the full schema of
  PLAN §3), a Dockerfile for Render.
- Seed command (`backend/cmd/seed`): `fetch` (Steam → `backend/seed/steam_games.json`) and `load`
  (snapshot → database). Snapshot: 236 games → 476 products, 1,648 offers, 11,414 demo keys.
- Frontend: Vite + React + TypeScript + Tailwind v4, design tokens from
  `docs/design-reference.md`, header/footer shell, React Router, TanStack Query, dev proxy for
  `/api`, `vercel.json` rewrites.
- CI: GitHub Actions runs gofmt, `go vet`, `go test` (with a temporary Postgres), lint and build.

**Decisions**
- *Health check does not query the database* — the uptime pinger would otherwise keep Neon awake
  and burn free compute hours.
- *Own 30-line `.env` loader instead of the godotenv dependency* — only the simplest form of the
  format is needed. Real environment variables always win over the file.
- *Status columns are `text` + `CHECK`, not PostgreSQL enums* — still enforced by the database,
  easier to change in a later migration.
- *`CHECK ((status = 'available') = (order_item_id IS NULL))` on `license_keys`* — the database
  itself rejects a key that is "sold to nobody", whatever the application does.
- *Only Steam's short description is stored* — it is plain text; the long one is publisher HTML and
  rendering it would be an XSS risk.
- *`seed load` refuses to overwrite an existing catalog without `-reset`* — replacing offers and
  keys deletes the orders that reference them, so it must be deliberate.
- *Tailwind's default colors, shadows and radii are removed from the theme* — only the measured
  tokens exist as classes, so an off-palette color cannot be used by accident. The spacing step
  is 5 px because Eneba's layout uses multiples of 5.
- *oxlint instead of ESLint* — it is what the current Vite template ships; PLAN.md said ESLint.
  Open for the author to overrule.
- *`product_listing` view postponed to day 3* — its columns follow from the store query.

**Verified (the "to verify on day 2" list in PLAN §10)**
- Steam endpoint: `https://store.steampowered.com/api/appdetails?appids=<id>&cc=lt&l=english`,
  one app per request, no key. 249 requests at one per 1.6 s completed in about ten minutes.
- Portrait cover pattern `…/store_item_assets/steam/apps/<id>/library_600x900.jpg` exists for 233
  of 236 games; the other three fall back to the landscape header image.
- 13 of the 249 listed ids were skipped (free to play, delisted or without a price).

**Not done yet (needs the author's accounts)**
- Neon connection strings in `backend/.env`; first run of migrations and seed against Neon.
- Render and Vercel deployment, the `Set-Cookie` round-trip check through the rewrite, the uptime
  pinger, re-checking the free-tier limits.

## 2026-11-17 — PLAN day 3 (store + search) and start of day 4

**Built**
- Store: `GET /api/products` with filters, sorting, pagination and facet counts (each group
  counted without its own selection); store page with all state in the URL.
- Search: typo-tolerant `pg_trgm` matching on a simplified name, synonyms table (numerals,
  abbreviations), accents via `unaccent`, query suggestions (curated + visitor), header dropdown
  with Downshift, debounced and cancellable requests. Test tables in `docs/search.md`.
- `docs/gap-analysis.md`: side-by-side comparison with eneba.com; Categories menu proposal.
- Product page: `GET /api/products/{slug}` with the offer comparison and other versions.

**Decisions (author)**
- Cents in the API, euros in the store page's URL.
- Search on the full variant name (there is no game-level title), with `word_similarity`.
- Facets: each group counted with all other filters except its own.
- Stay on Postgres search; Meilisearch/Typesense assessed and not adopted.
- Categories menu: table-driven (option B), built on day 6 with the homepage.
- Visitor suggestions public only after 2 different days (not sessions: visitors are anonymous).

**Mistakes caught**
- Controlled Downshift input lost letters during fast typing → Downshift now owns the text.
- `leading-N` with a 5px spacing step meant N×5px → exact pixel line heights.
- I edited an already-applied migration (00005) by accident; reverted, and the change went into
  a new migration (00007) instead. Applied migrations are never edited.

## 2026-11-17 (continued) — product page polish, accounts, cart

**Built**
- Product page measured against a live eneba.com product page at 1920px (Playwright): 1600px
  content, 1220px main column, 350px buy box, 109px offer rows. Screenshot gallery with lightbox,
  important notice and activation guide per platform (from the backend), genre tags, tabs.
- Steam snapshot re-fetched with screenshot URLs; migration 00008 (screenshots, guides).
- Accounts: register, log in, log out, `/me`; Argon2id; hashed session tokens; HttpOnly + Secure +
  SameSite=Lax cookie; Origin check; per-address login limiter. Failure cases in `docs/auth.md`.
- Cart in localStorage, validated by `POST /api/cart/validate`; cart page; Buy now / add to cart.

**Verified**
- Cookie round trip through the Vercel rewrite on the live site (2026-11-17): register via
  `https://eneba-seven.vercel.app/api/auth/register` → `201`, `Set-Cookie: session=…; HttpOnly;
  Secure; SameSite=Lax`; `/api/auth/me` with the cookie → the user; register with
  `Origin: https://evil.example` → `403`; logout → cookie deleted, `/me` → `null`.
  (This created one test account, `cookie-check-<timestamp>@example.com`, in production.)

**Mistakes caught**
- Header width was 1160px instead of 1200px: the container token did not include the padding.
- Dialogs closed themselves in development: their effect clean-up called `close()`, which fires
  the close event; React StrictMode mounts twice, so the clean-up ran at once.
- Whole-page queries with an abort signal stayed on the skeleton under StrictMode (the first,
  cancelled request was the one the page waited for). The signal is now used only where
  cancelling helps (search dropdown).
- One commit (`aef73e1`) does not build on its own: two files it imports went into the next
  commit. Not rewritten, because history is never rewritten in this repo.

## 2026-11-17 (continued) — PLAN day 5 (checkout and key delivery)

**Decided**
- Keys are reserved when the payment session is created, not assigned in the webhook: the
  buyer learns "sold out" before paying instead of after. Reservation = 30 minutes = the Stripe
  session's expiry; released by Stripe's `checkout.session.expired` webhook or by a clean-up job
  every minute (5 minutes of grace). Full comparison in `docs/checkout.md`.
- Neon Free: do not ping the database to keep it awake. Free has 100 CU-hours per project per
  month, shared by all branches; scale-to-zero after 5 minutes cannot be disabled; even 0.25 CU
  awake all month is about 182 CU-hours, and when the allowance runs out compute is suspended
  until next month. `/api/healthz` stays database-free so the Render pinger never wakes Neon.
- Stripe Checkout accepts cards only: Stripe also offered bank-redirect methods whose result can
  arrive after the reservation has expired.

**Built**
- `internal/orders`: checkout transaction (prices from the database, keys reserved with
  `FOR UPDATE SKIP LOCKED`), idempotent fulfilment (`UPDATE … WHERE status = 'pending'`),
  webhook events stored in `payment_events` in the same transaction as their effect, simulated
  and Stripe providers behind one interface (`PAYMENT_PROVIDER`). Migration 00009.
- API: checkout, orders, confirm, simulated pay, Stripe webhook with signature check. Only
  `sk_test_` keys are accepted.
- Frontend: checkout button, simulated payment page, order page with keys and copy buttons
  (polls `/confirm` after returning from Stripe), order history, "My orders" link.

**Verified**
- Tests on the Neon `test` branch: 20 buyers racing for the last key (1 wins, 19 get out of
  stock), duplicate and concurrent fulfilment, expiry via webhook and via the clean-up job,
  payment after expiry, browser-sent prices ignored, owner-only access, provider failure,
  forged / old / tampered webhooks.
- Locally in a browser: the simulated flow and the Stripe test-mode flow.
- Live (2026-11-17): on `https://eneba-seven.vercel.app` a new account bought one key with card
  4242 4242 4242 4242; Stripe returned to `/orders/1`, which showed the key within about 4
  seconds; the cart emptied; history listed the paid order. Stripe API: the webhook endpoint
  `https://eneba-2753.onrender.com/api/webhooks/stripe` is enabled for the two events, and the
  `checkout.session.completed` event for that order has `pending_webhooks: 0` (delivered with a
  2xx). This created a test account `stripe-live-check-<timestamp>@example.com` in production.

**Mistakes caught**
- The test fixture emptied `users` with `TRUNCATE … CASCADE` after loading the catalog; the
  cascade reaches orders → order items → license keys and deleted every key. It now runs first.
- I used a stripe-go field name (`PaymentMethodTypes`) that this version does not have; the
  compiler caught it. The right field is `AllowedPaymentMethodTypes`.
