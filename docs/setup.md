# Setup checklist: tools, accounts, keys

What has to exist outside the repository, who creates it, and where every value goes.

**Rule for secrets:** never paste a secret into the Claude chat and never commit one. Secrets go
only into (a) the local file `backend/.env` (git-ignored) and (b) the hosting provider's
"Environment variables" screen. Non-secret values (public URLs) can be shared freely.

## 1. Install on the development machine (Windows)

Already present: Git 2.46, Node.js 24, npm, Claude Code. Missing:

| # | Tool | Install command (PowerShell) | Check | Needed for |
|---|---|---|---|---|
| 1 | **Go** (latest stable) | `winget install GoLang.Go` then open a **new** terminal | `go version` | the backend |
| 2 | **gopls** (Go language server) | `go install golang.org/x/tools/gopls@latest` | `gopls version` (if "not found", add `%USERPROFILE%\go\bin` to PATH) | type errors and navigation for Claude |
| 3 | gopls plugin for Claude Code | `claude plugin install gopls-lsp@claude-plugins-official --scope project` (run in the project folder) | `claude plugin list` | same |
| 4 | **Stripe CLI** | `winget install Stripe.StripeCli` (or download from <https://docs.stripe.com/stripe-cli/install>) | `stripe version` | forwarding test webhooks to localhost (needed from day 5) |

Not needed: Docker, a local PostgreSQL, PHP.
The first use of the Playwright plugin downloads a browser automatically.

## 2. Accounts to create, in this order

All are free and none needs a credit card. Sign up with GitHub where offered — it makes connecting
the repository easier.

1. **GitHub** — repo `Jokubas-J/Eneba` exists. Make it **public** before submitting.
2. **Neon** (<https://neon.com>) — needed first, on day 2.
   - Create one project (region: an EU one, e.g. Frankfurt; newest Postgres version).
   - Create two extra branches from `main`: **`dev`** and **`test`**.
   - For each of the three branches copy the **direct** connection string
     (`postgresql://…`, connection pooling switched **off** in the "Connect" dialog).
3. **Stripe** (<https://dashboard.stripe.com/register>) — needed on day 5, but create it now.
   - Stay in **test mode / sandbox**. Do not activate the account for live payments.
   - Copy the **Secret key** that starts with `sk_test_` (Developers → API keys).
   - The webhook secret comes later (section 4).
4. **Render** (<https://render.com>) — sign up with GitHub. Do **not** create a service yet; we do
   that together on day 2 when there is code to deploy.
5. **Vercel** (<https://vercel.com>) — sign up with GitHub on the free Hobby plan. Also no project
   yet.
6. *(Day 2, optional)* **UptimeRobot** or **cron-job.org** — free pinger that calls
   `/api/healthz` every 10 minutes so the Render service does not fall asleep.

## 3. Environment variables — complete list

The backend reads everything from environment variables. The frontend needs none in production:
it calls `/api/...` on its own domain. Its only variable is the optional, development-only
`API_PROXY_TARGET` in `frontend/.env` (default `http://localhost:8080`), which tells the Vite dev
server where the Go server runs; it is not a secret and is not compiled into the browser bundle.

| Variable | Secret? | Local `backend/.env` | Render (production) | Where the value comes from |
|---|---|---|---|---|
| `DATABASE_URL` | **yes** | Neon **`dev`** branch string | Neon **`main`** branch string | Neon → project → Connect |
| `TEST_DATABASE_URL` | **yes** | Neon **`test`** branch string | *(not set)* | Neon → project → Connect. Tests wipe this database. |
| `APP_ENV` | no | `development` | `production` | fixed values. Controls the `Secure` cookie flag. |
| `PORT` | no | `8080` | *(set by Render automatically)* | — |
| `APP_BASE_URL` | no | `http://localhost:5173` | `https://<project>.vercel.app` | the public address of the frontend; used for Stripe return URLs and the `Origin` check |
| `PAYMENT_PROVIDER` | no | `stripe` or `simulated` | `stripe` (fallback: `simulated`) | our own switch |
| `STRIPE_SECRET_KEY` | **yes** | `sk_test_…` | same `sk_test_…` | Stripe → Developers → API keys |
| `STRIPE_WEBHOOK_SECRET` | **yes** | `whsec_…` printed by `stripe listen` | `whsec_…` of the dashboard webhook endpoint (different value!) | see section 4 |

Other places configuration lives:

| Where | What | Secret? |
|---|---|---|
| `backend/.env.example` (committed) | every variable name above with a fake value | no |
| `frontend/.env.example` (committed) | `API_PROXY_TARGET` with its default value | no |
| `.claude/redact.local.txt` (git-ignored) | personal strings (e.g. a private e-mail address) that the log exporter removes from `ai-logs/` | personal, not committed |
| `frontend/vercel.json` (committed) | rewrite `/api/*` → `https://<service>.onrender.com/api/*` | no — but I need the Render URL once the service exists |
| Vercel project settings | nothing | — |
| GitHub Actions | nothing (CI starts its own temporary Postgres and uses the simulated payment provider) | — |

There is no JWT or session secret: session tokens are random values stored (hashed) in the database.

## 4. Stripe webhook secrets (day 5)

There are two different `whsec_…` values:

- **Local:** run `stripe login` once, then
  `stripe listen --forward-to localhost:8080/api/webhooks/stripe`.
  It prints `Your webhook signing secret is whsec_…` → put it in `backend/.env`.
- **Production:** Stripe dashboard (test mode) → Developers → Webhooks → *Add endpoint* →
  URL `https://<service>.onrender.com/api/webhooks/stripe`, events
  `checkout.session.completed` and `checkout.session.expired` → copy the endpoint's signing
  secret → Render environment variable `STRIPE_WEBHOOK_SECRET`.

Test card: `4242 4242 4242 4242`, any future expiry date, any CVC.

## 5. What Claude needs to be told in chat

Only non-secret facts: "Neon/Stripe values are in `backend/.env`", the Render service URL and the
Vercel project URL. Everything secret stays in `.env` and in the providers' dashboards.

## 6. Adding a game to the catalog

There is exactly **one** way to add a game, and it is the seed command. The application has no
admin page and no API endpoint that writes products, offers, keys or screenshots; the only code
that inserts them is `backend/internal/seed/load.go`. (The running server never calls Steam.)

1. **Add the Steam app id** to `backend/seed/app_ids.txt` (one per line, `# name` as a comment).
   The id is the number in the game's Steam store URL: `store.steampowered.com/app/292030/…`.
2. **Fetch** (needs internet, about 7 minutes for ~250 ids; one request every 1.6 s to stay under
   Steam's rate limit). Run from `backend/`:
   ```powershell
   go run ./cmd/seed fetch
   ```
   It downloads every listed game again and rewrites `backend/seed/steam_games.json`: name,
   short description, genres, release date, developer, publisher, price, popularity, the
   portrait cover URL and up to 8 screenshot URLs. Free-to-play, unreleased and delisted games are
   skipped and listed in the output.
3. **Review and commit** `app_ids.txt` and `steam_games.json`. The snapshot in git is what every
   database is seeded from, so seeding never depends on Steam being reachable.
4. **Load** into a database. It generates the platform/region versions, sellers' offers, prices
   and keys from the snapshot with a fixed random seed:
   ```powershell
   # dev branch (DATABASE_URL from backend/.env):
   go run ./cmd/seed load -reset

   # production (Neon main branch); type the value in your own terminal, never in the chat:
   $env:DATABASE_URL = "<Neon main branch connection string>"
   go run ./cmd/seed load -reset
   Remove-Item Env:DATABASE_URL
   ```
   A variable set in the terminal wins over `backend/.env`, so the second form targets production.

**What `-reset` deletes:** all products, screenshots, genres, platforms, regions, sellers, offers,
keys **and all orders** (orders point at offers and keys). Accounts, sessions, search suggestions
and activation guides are kept. Without `-reset` the command refuses to run on a database that
already has products. While there are no real orders this is fine; once checkout is used, adding
a game this way also removes order history. An "add without reset" mode would be the next step if
that ever matters; it is not built.

Images are not copied: covers and screenshots are hot-linked from Steam's CDN. If Steam later
removes one, the site shows a placeholder for a cover and leaves a missing screenshot out
(`frontend/src/components/RemoteImage.tsx`).
