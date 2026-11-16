# Project rules — Eneba Intern Assignment 2027

A marketplace web app in Eneba's style. The author must be able to **explain every line of code
and every decision**, and the full AI conversation history is a graded deliverable.
Read `PLAN.md` before starting work.

## Key documents
- `PLAN.md` — scope, architecture, data model, API, day-by-day plan, decisions (§10).
- `docs/requirements.md` — checklist of the brief (IDs F…, T…, D…, V…).
- `docs/design-reference.md` + `docs/screenshots/` — the UI source of truth.
- `docs/research.md` — why each technology was chosen (with sources).
- `docs/setup.md` — tools, accounts, and every environment variable and where it goes.
- `docs/tooling.md` — AI tools used; update it when a tool is added or removed.
- `ai-logs/README.md` — how conversation logs are produced.

## Decided stack (do not change without asking the author)
- **Backend:** Go, standard library `net/http`, REST + JSON, `pgx` with hand-written SQL, SQL
  migrations. Layers: handler → service → repository. No web framework, no ORM.
- **Frontend:** React + TypeScript + Vite, React Router, TanStack Query, **Tailwind CSS** with
  design tokens from `docs/design-reference.md`.
- **Database:** PostgreSQL on **Neon** — branch `main` = production, `dev` = local development,
  `test` = automated tests. **No Docker, no local Postgres.**
- **Search:** Postgres `pg_trgm`.
- **Payments:** Stripe Checkout in **test mode**, plus a simulated provider behind the same
  interface, selected with `PAYMENT_PROVIDER=stripe|simulated`. Local webhooks via the Stripe CLI.
- **Catalog data:** Steam store data, fetched **once** by the seed script and stored in our
  database (snapshot committed as JSON). **The application must never call Steam at runtime.**
  Cover images are hot-linked. Steam is credited in the README.
- **Hosting:** Vercel (frontend, rewrites `/api/*`) → Render (Go) → Neon.
- **Monorepo:** `frontend/` and `backend/`.

## Rules

### 1. Logging must stay enabled
- Never remove or weaken the hooks in `.claude/settings.json` or `.claude/hooks/export-log.mjs`.
- **Never delete or edit files in `ai-logs/`** by hand. The log files are written only by the hook
  script. (`ai-logs/README.md` is documentation and may be edited only when the author asks.)
- At the start of a session, check that `ai-logs/` has an up-to-date file for the previous session.
  If logs look incomplete, run `node .claude/hooks/export-log.mjs --all` and tell the author.
- If `ai-logs/hook-errors.log` appears or grows, tell the author.

### 2. Explain as you go
- For every non-trivial piece of code, explain in the chat **what it does and why this approach
  was chosen over the alternatives**, in plain language, so the author can defend it. The author
  knows C, C++ and C# well and Go/TypeScript less — relate new concepts to those when it helps.
- Prefer simple, explicit code over clever code. No abstractions "for later".
- Add a new dependency only with a stated reason; prefer the standard library.
- Code comments explain *why* (constraints, trade-offs), not what the next line does.
- When there is a real choice to make (scope, architecture, library), present the options and a
  recommendation and let the author decide. Do not silently pick.
- Do only what was asked. Propose extra work instead of doing it.

### 3. Commits
- Small commits, one logical change each, with clear messages (imperative mood, what and why).
- You may commit and push to the `main` branch.
- Include the updated `ai-logs/` files in commits. (The log of the *current* session keeps changing
  until the session ends, so its final version is committed at the start of the next session.)
- Never force-push, never rewrite history.
- Never commit the assignment PDF (it is git-ignored).

### 4. Secrets
- Never commit secrets. Real values live in `backend/.env` (git-ignored); every variable is
  documented in a committed `.env.example` with a fake value and in `docs/setup.md`.
- Never print secret values in the chat or write them into docs, tests or seed files. Do not ask
  the author to paste secrets into the chat — ask them to put the value into `.env`.
- Only Stripe **test-mode** keys are used in this project.

### 5. UI work
- Follow `docs/design-reference.md` for all UI: colors, typography, spacing, radius (square
  corners), components. Use the Tailwind theme tokens; do not invent new colors or sizes.
- Compare new screens with the matching file in `docs/screenshots/` before calling them done.
- The frontend must not hardcode homepage content: blocks and their data come from the API.

### 6. Quality
- Run the relevant tests and type checks before saying something works; report failures honestly.
- Money is integer cents. Prices and stock are always taken from the database, never trusted from
  the client.
- Anything touching auth, payments or key delivery gets tests and a short written explanation of
  the failure cases it handles.
- Database tests run only against `TEST_DATABASE_URL` (the Neon `test` branch), never against the
  `dev` or `main` branch.
