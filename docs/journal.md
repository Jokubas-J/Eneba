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
