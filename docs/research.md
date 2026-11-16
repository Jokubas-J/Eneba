# Research: choosing the approach

Researched on 2026-11-16 with web search and by reading vendor documentation. For every decision:
**Facts** (with sources) are separated from **Opinion** (my reasoning), followed by one
**Recommendation**. Constraints from the brief: React (TypeScript), backend in Go or PHP, REST or
GraphQL, any database, free-tier deployment. Personal constraint: the author knows C/C++ and C#
better than TypeScript and must be able to explain every line.

Where a source was a third-party blog rather than official documentation, it is marked *(blog)*.
Free-tier terms change often — **re-check the linked official pages on day 1**.

---

## 1. Backend language: Go vs PHP

**Facts**
- The brief says "Backend in PHP (or Go)". Eneba's own job ads describe PHP/Symfony as the primary
  backend language and Go as a secondary one "where speed/efficiency is required", with GraphQL as
  the main communication method and 100+ microservices
  ([Eneba job ad: Full Stack Engineer, Go/PHP & React](https://jobs.eu.lever.co/eneba/c254b535-d637-4c18-8ee8-a6af56c5903e),
  [Backend Engineer, PHP](https://jobs.eu.lever.co/eneba/23da951d-f432-439d-8575-4c30ada6eac4)).
- Go is statically typed and compiled to a single executable. Since Go 1.22 the standard library
  router supports HTTP methods and path parameters (`GET /products/{slug}`), so a REST API needs no
  web framework ([Go blog: Routing Enhancements for Go 1.22](https://go.dev/blog/routing-enhancements)).
- Neither Go nor PHP is installed on this machine yet (checked with `which go php`). Either choice
  needs a toolchain install.
- Render's free web services sleep after 15 minutes and take "about one minute" to spin up
  ([Render docs: Deploy for Free](https://render.com/docs/free)), so start-up time and memory use
  of the backend matter.

**Opinion**
- For someone coming from C/C++/C#, Go reads familiarly: explicit types, structs, compile errors
  instead of runtime surprises, explicit error returns, almost no "magic". The whole language is
  small enough to learn the used subset in a week.
- A modern PHP backend in practice means Symfony or Laravel. Those rely on conventions, dependency
  injection containers, attributes and an ORM — a lot of behaviour happens *outside* the lines you
  wrote, which is the opposite of "explain every line".
- PHP would match Eneba's main stack more closely, which is a real point in its favour. But the
  brief allows Go explicitly, Eneba uses Go too, and a clear, well-explained Go service is a better
  demonstration than a framework-heavy PHP one that is hard to defend.
- A Go binary in a small container starts in well under a second once the instance is up, which
  helps on a sleeping free tier.

**Recommendation: Go**, standard library `net/http`, no web framework.

### API style: REST vs GraphQL
- **Fact:** both are allowed; Eneba uses GraphQL internally (job ads above).
- **Opinion:** GraphQL in Go needs a schema, code generation (gqlgen) and resolvers — more
  generated code to explain and more setup for a 7-day project. REST maps one URL to one handler
  function, is trivial to test with `curl`, and "one endpoint per homepage block" maps directly
  onto requirement F6.
- **Recommendation: REST + JSON.** Mention GraphQL in the README as the path Eneba itself takes.

### Database access in Go
- **Facts:** `pgx` is the standard PostgreSQL driver/toolkit for Go; `sqlc` generates type-safe Go
  from hand-written SQL; GORM is the most popular ORM and hides SQL behind method chains
  ([JetBrains: Comparing database/sql, GORM, sqlx, and sqlc](https://blog.jetbrains.com/go/2023/04/27/comparing-db-packages/),
  [comparison (blog)](https://www.glukhov.org/post/2025/09/comparing-go-orms-gorm-ent-bun-sqlc/)).
- **Opinion:** the interesting parts of this project *are* SQL (fuzzy search, facet counts, row
  locking for keys). Hiding them behind an ORM would hide exactly what needs explaining. sqlc is
  nice but struggles with dynamically built filter queries and adds a code-generation step.
- **Recommendation: `pgx` with hand-written SQL** in small repository files. No ORM.

---

## 2. Database and typo-tolerant search

**Facts**
- PostgreSQL's `pg_trgm` extension provides trigram similarity (`similarity`, `word_similarity`,
  operators `%` and `<%`) and GIN/GiST index support for fast fuzzy and `LIKE '%…%'` search.
  Default thresholds: 0.3 (similarity) and 0.6 (word similarity), both configurable
  ([PostgreSQL docs: pg_trgm](https://www.postgresql.org/docs/current/pgtrgm.html)).
- `pg_trgm` only scores; ordering/ranking the results is up to the application, and it has no
  notion of words or proximity
  ([Meilisearch blog — vendor source](https://www.meilisearch.com/blog/postgres-full-text-search-limitations)).
- Meilisearch and Typesense have typo tolerance, ranking and faceting built in, but are separate
  servers that must be hosted and kept in sync with the database
  ([Typesense guide — vendor source](https://typesense.org/docs/guide/postgres-typesense-migration-fts.html)).
- Free hosting for those engines is doubtful: sources describe Meilisearch Cloud as a 14-day trial
  and Typesense Cloud as a one-time ~720-hour credit; reports conflict
  ([Meilisearch blog on Typesense pricing — vendor source](https://www.meilisearch.com/blog/typesense-pricing),
  [Typesense Cloud help: free tier](https://cloud-help.typesense.org/article/how-does-the-free-tier-work)).
  Self-hosting one on Render's free tier is impractical because the filesystem is ephemeral — the
  index would be lost on every spin-down ([Render docs](https://render.com/docs/free)).
- Neon's free Postgres supports `pg_trgm`, `unaccent` and `fuzzystrmatch`
  ([Neon docs: supported extensions](https://neon.com/docs/extensions/pg-extensions)).
- On eneba.com, typing `witchr 3` returns Witcher 3 products and suggests corrected queries (own
  observation, screenshot `docs/screenshots/09-search-autocomplete-typo.jpg`).

**Opinion**
- The catalog will be a few hundred to a few thousand rows. At that size `pg_trgm` is instant, and
  one database means: no second service to deploy, no sync job, no second cold start, and search
  filters/sorting can be combined with the catalog query in one SQL statement.
- The cost is that ranking has to be designed by hand (exact prefix first, then similarity, then
  popularity). That is a *feature* for this assignment: it is explainable and testable.
- A dedicated engine would give better relevance out of the box and is what a real marketplace
  with millions of products should use — worth saying in the README.

**Recommendation: PostgreSQL + `pg_trgm`** (GIN trigram index on the product name; query combines
prefix/substring match with `word_similarity`; ranked by match quality then popularity). Keep search
behind a small Go interface so it could be swapped for Meilisearch later.

---

## 3. Payments (test mode) and safe key delivery

**Facts — Stripe**
- Stripe Checkout works in test mode with test cards such as `4242 4242 4242 4242`; the Stripe CLI
  forwards webhooks to localhost with `stripe listen --forward-to …`
  ([Stripe docs: Fulfill orders](https://docs.stripe.com/checkout/fulfillment)).
- Stripe states that **webhooks are required** for fulfilment ("it's not guaranteed customers visit
  that page"), recommends *also* triggering fulfilment from the landing page, and warns the
  fulfilment function "might be called multiple times, possibly concurrently, for the same Checkout
  Session" — so it must be idempotent, must re-fetch the session and check `payment_status`
  (same source).
- The event to handle is `checkout.session.completed`; webhook payloads must be verified with the
  `Stripe-Signature` header and the endpoint secret, using the **raw** request body (same source).
- With a webhook endpoint and a `success_url`, Checkout waits up to 10 seconds for the webhook
  response before redirecting the customer (same source).
- Failed webhook deliveries are retried by Stripe automatically (same source) — relevant because
  our free backend may be asleep.

**Facts — concurrency**
- `SELECT … FOR UPDATE SKIP LOCKED` lets concurrent transactions each claim *different* free rows
  without blocking; it is the standard Postgres pattern for allocating finite inventory or queue
  jobs ([Arkency: inventory with SKIP LOCKED (blog)](https://blog.arkency.com/handling-concurrency-with-database-locks-and-skip-locked/),
  [2ndQuadrant: What is SKIP LOCKED for (blog)](https://blog.2ndquadrant.com/tag/queue/)).

**Opinion**
- A fake "Pay" button satisfies the brief, but the hard and interesting part of selling digital
  keys is exactly what a real provider forces you to handle: asynchronous confirmation, duplicate
  events, a customer who closes the tab. Doing this properly is a good "deep" feature.
- Hosted Stripe Checkout (redirect) is the least code and never lets card data touch our server.
- Design for key safety (my proposal, not from a source):
  1. Each key is one row in `license_keys` with a status: `available → reserved → sold`.
  2. Creating a checkout **reserves** keys for the order inside one transaction using
     `FOR UPDATE SKIP LOCKED` (two buyers can never get the same row). Reservations expire.
  3. Fulfilment does `UPDATE orders SET status='paid' WHERE id=$1 AND status='pending'`. Only the
     first caller changes a row; everyone else (webhook retry, success page, concurrent call) sees
     0 rows and does nothing. The keys become `sold` in the same transaction.
  4. Database constraints back this up (unique Stripe session id; a key belongs to at most one
     order item).
  5. Keys are only ever returned by the API to the signed-in owner of a paid order.
- A small `PaymentProvider` interface with two implementations — Stripe and a simulated one —
  keeps tests independent of Stripe and gives a fallback if Stripe setup causes trouble.

**Recommendation: Stripe Checkout in test mode + verified webhook + idempotent fulfilment**, key
stock reserved with `FOR UPDATE SKIP LOCKED`, plus a simulated provider behind the same interface.

---

## 4. Authentication

**Facts**
- OWASP: a session ID "must be meaningless", needs at least 64 bits of entropy, and should be sent
  in a cookie with `HttpOnly; Secure; SameSite=Strict` (preferred) or `SameSite=Lax`
  ([OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)).
- OWASP's first choice for password storage is **Argon2id** (minimum 19 MiB memory, 2 iterations,
  1 thread), then scrypt; bcrypt "should only be used … in legacy systems where Argon2 and scrypt
  are not available" (work factor ≥ 10, 72-byte input limit)
  ([OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)).
- JavaScript cannot read an `HttpOnly` cookie, so a stolen-by-XSS token is not possible the way it
  is with tokens kept in `localStorage` (same OWASP session sheet).
- Browsers restrict third-party cookies (Safari blocks them); a cookie set by `api.onrender.com`
  for a page on `app.vercel.app` is third-party. Proxying the API through the frontend's own
  domain makes it first-party ([Vercel docs: Rewrites](https://vercel.com/docs/rewrites),
  [Vercel guide: reverse proxy](https://vercel.com/kb/guide/vercel-reverse-proxy-rewrites-external)).

**Opinion**
- JWTs are attractive for stateless microservices. Here there is one backend and one database, and
  logout / "sign out everywhere" needs server state anyway. A sessions table is one lookup per
  request and can be explained in two sentences.
- Plan: random 32-byte token → cookie (`HttpOnly; Secure; SameSite=Lax`); only its SHA-256 hash is
  stored in a `sessions` table with an expiry; passwords hashed with Argon2id using OWASP's
  minimum parameters (`golang.org/x/crypto/argon2`, a ~40-line helper). bcrypt would be two
  function calls and is the simpler fallback, but OWASP now calls it a legacy choice.
  `SameSite=Lax` rather than `Strict`, because the buyer returns from Stripe's site via a
  cross-site redirect and must still be signed in on arrival. CSRF protection: `SameSite=Lax` +
  JSON-only bodies + an `Origin` check on state-changing requests.
- Out of scope, and said so in the README: e-mail verification, password reset, social login, 2FA.

**Recommendation: server-side sessions in Postgres with an httpOnly cookie**, served same-origin
through a frontend rewrite.

---

## 5. Homepage: "each block gets its content from its own backend service or database"

**Facts**
- The brief: blocks "such as a banner, best sellers or deals"; "each block gets its content from
  its own backend service or database, so the frontend only displays what it receives and nothing
  is hardcoded there".
- eneba.com's homepage is a stack of independent blocks — banners, recommended, top games,
  best-selling gift cards, upcoming games, categories — each with its own "show all" link (own
  observation, `docs/design-reference.md`).

**Opinion — three ways to read it**
1. *Minimal:* one `/home` endpoint returns everything. Simple, but "own service" is not visible.
2. *Modular (recommended):* one deployable backend, but
   - the **layout** comes from the backend: `GET /api/home` returns the ordered list of blocks
     (type, title, where to fetch data) from a `home_blocks` table, so adding/reordering/removing
     a block is a database change, not a frontend change;
   - each block has its **own endpoint and its own Go service** with its **own data source**:
     banners ← `banners` table (curated content); best sellers ← aggregated from real `order_items`;
     deals ← computed from `offers` (largest discount); new releases ← `products.release_date`;
     categories ← genre counts;
   - the frontend maps `block.type` → a React component, fetches each block independently, and
     shows a skeleton/error per block, so one failing block cannot break the page.
3. *Literal microservices:* a separate deployed service/database per block. On free tiers each
   would sleep and cold-start separately; much more operations work for no user-visible benefit.

Option 2 keeps the *boundaries* of option 3 (each block service could be moved to its own process
without touching the frontend contract) at the cost of option 1. That is the sensible reading for
a one-week project and is easy to justify.

**Recommendation: option 2 — backend-driven layout + one endpoint/service/data source per block.**

---

## 6. Free hosting (late 2026)

**Facts**
- **Render** (free web service): sleeps after 15 minutes without inbound traffic, ~1 minute to
  wake; 750 free instance-hours per workspace per month; ephemeral filesystem; custom domains OK;
  static sites are free. Free **Postgres expires 30 days after creation**, 1 GB
  ([Render docs: Deploy for Free](https://render.com/docs/free)).
- **Neon** (free Postgres): 100 CU-hours of compute per project per month, storage per project
  0.5–1 GB (official FAQ and a recent changelog differ; either is far more than needed), no expiry;
  compute scales to zero after 5 minutes idle and resumes on the next connection
  ([Neon: free plan limits](https://neon.com/faqs/free-plan-limits-and-quotas),
  [review (blog)](https://www.jetadmin.io/blog/neon-review/)).
- **Koyeb**: the pricing page no longer lists a free web-service instance (Pro from $29/month);
  only a small free Postgres ([Koyeb pricing](https://www.koyeb.com/pricing)).
- **Fly.io**: free allowances ended in 2024 for new accounts; **Railway**: one-time $5 trial credit
  only ([overview (blog)](https://dev.to/snapdeploy/free-backend-hosting-7-platforms-that-wont-kill-your-api-2026-1i1g)).
- **Vercel Hobby**: free static/React hosting for non-commercial personal use; supports rewrites
  that proxy a path to an external backend ([Vercel docs: Hobby plan](https://vercel.com/docs/plans/hobby),
  [Vercel docs: Rewrites](https://vercel.com/docs/rewrites)).
- **Google Cloud Run** and **Oracle Always Free** appear in "still free" lists but need a credit
  card ([overview (blog)](https://dev.to/snapdeploy/free-backend-hosting-7-platforms-that-wont-kill-your-api-2026-1i1g)).

**Opinion**
- The database must not disappear while evaluators are looking → **Neon, not Render Postgres**
  (30-day expiry is a trap, and evaluation may happen weeks after submission).
- Render is the only widely used option left with a free, card-less, always-available (if sleepy)
  container/Go service, and the brief names it.
- Putting the React app on Vercel (also named in the brief) means the page shell loads instantly
  even when the API is asleep; the UI can show skeletons while the backend wakes. Proxying `/api/*`
  through Vercel keeps cookies first-party and removes CORS entirely.
- Cold start is the main user-visible risk. Mitigations: a free uptime pinger hitting `/healthz`
  every ~10 minutes (750 h/month is enough for one service to stay up all month), a "waking up the
  server…" state in the UI, and a note in the README.
- Unverified detail to test on day 1: that `Set-Cookie` passes through a Vercel external rewrite
  and how long Vercel waits for a slow (waking) backend. Fallback if it misbehaves: let the Go
  server serve the built React files itself (single origin on Render).

**Recommendation: Vercel (frontend + `/api` rewrite) → Render free web service (Go) → Neon
(Postgres).** Deploy a "hello world" through all three on day 1.

---

## Summary of recommendations

| Decision | Recommendation |
|---|---|
| Backend language | Go, standard library HTTP server |
| API | REST + JSON |
| DB access | pgx + hand-written SQL, SQL migrations |
| Database | PostgreSQL on Neon |
| Search | `pg_trgm` trigram search in Postgres, hand-designed ranking |
| Payments | Stripe Checkout test mode + webhook, simulated provider as fallback |
| Key delivery | one row per key, reserve with `FOR UPDATE SKIP LOCKED`, idempotent fulfilment |
| Auth | server-side sessions, httpOnly cookie, Argon2id password hashes |
| Homepage | backend-driven block list; one endpoint + service + data source per block |
| Hosting | Vercel + Render + Neon, uptime ping against cold starts |
