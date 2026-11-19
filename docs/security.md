# Security review (day 7, 2026-11-19)

A review of the whole codebase before submission. The built-in `/security-review` command
compares a branch with what is already pushed; everything was on `main`, so it had nothing to
review, and the review was done by hand along the list below. Details of the individual mechanisms
are in `docs/auth.md` (accounts, sessions) and `docs/checkout.md` (payments, key delivery).

## Checked, no problem found

| Area | What was checked | Result |
|---|---|---|
| SQL injection | Every query in `backend/internal` | All values are passed as parameters (`$1`, `$2`…). The parts joined into SQL text are fixed strings from our code (column lists, `ORDER BY` chosen from a map of allowed sorts). The one regular expression (whole-word synonym match) is a parameter value built from normalised text (`a–z`, `0–9`, spaces). |
| Cross-site scripting | React components | No `dangerouslySetInnerHTML` anywhere; Steam descriptions are plain text that React escapes. Since day 7 a Content Security Policy forbids scripts from anywhere but our own origin (see below), so even a mistake would not run injected script. |
| Session theft | Cookie | `HttpOnly` (unreadable by JavaScript), `Secure` in production, `SameSite=Lax`; token stored only as a SHA-256 hash, so a database leak does not leak usable sessions. |
| CSRF | State-changing requests | `SameSite=Lax` plus an `Origin` check against `APP_BASE_URL` on every non-GET request (403 otherwise). The Stripe webhook has no Origin and is protected by its signature instead. |
| Open redirect | `?next=` after sign-in | `safeNextPath` allows only paths on our own site (`/…`, not `//host` or `/\host`); unit-tested. |
| Someone else's order or keys | `GET /api/orders/{id}`, confirm, simulated pay | Owner check in SQL; another user's order is a 404, as if it did not exist (`TestOrdersAreOnlyVisibleToTheirOwner`). |
| Forged payments | Stripe webhook | Signature (HMAC-SHA256 of the raw body) verified before anything else; replays older than 5 minutes refused; duplicate events idempotent (`TestStripeWebhookSignature`, `TestDuplicateWebhook`). Only `sk_test_` keys are accepted at start-up. |
| Prices from the browser | Checkout | The request type has no price field; every price is read from the database in the reservation transaction (`TestCheckoutIgnoresPricesFromTheBrowser`). |
| Secrets in the repository | All tracked files, including `ai-logs/` | Only placeholder values (`user:password@ep-example…`); in the logs, connection-string passwords are replaced by `[REDACTED]` by the exporter; no Stripe keys or webhook secrets anywhere. `.env` files are git-ignored. |
| Error messages | 500 responses | The real error goes to the server log only; the browser gets a generic message (`TestHomeEndpoints` checks no internals leak). |
| Dependencies | `npm audit` | 0 known vulnerabilities (production and development packages). |

## Fixed during this review

| Problem | Fix | Test |
|---|---|---|
| One account could check out again and again without paying and hold an offer's whole stock | At most 3 unpaid orders and 20 reserved keys per user; the user row is locked during checkout so parallel requests cannot pass the limit together | `TestPendingOrderLimits` |
| Unlimited registration (spam, and making the server run the expensive password hash non-stop) | At most 30 new accounts per 10 minutes site-wide, checked before hashing | `TestSignupsAreLimitedSiteWide` |
| No security headers | `vercel.json`: Content Security Policy (own origin; images also from `*.steamstatic.com`), `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`. Vercel adds HSTS. | Checked on the live site: no CSP violations, all images load, checkout still redirects to Stripe |
| A crashing component left a blank page | Error boundary with a reload button | `resilience.test.tsx` |

## Known limits (accepted, documented)

- **Rate limits are simple.** Failed logins are counted per e-mail address in memory (lost on
  restart); sign-ups site-wide. Neither trusts a client IP from a proxy header. A real shop would
  add e-mail verification or a CAPTCHA, and a shared store (Redis) for counters.
- **No e-mail verification and no password reset** (out of scope, see `docs/auth.md`).
- **`style-src 'unsafe-inline'`** in the CSP: React sets a few inline styles (the carousel's
  scrollbar position, skeleton delays). Scripts stay strict.
- **`govulncheck`** (Go's vulnerability scanner) could not be run from the assistant's
  environment; run `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` in `backend/` before
  submitting.
- **The author's private e-mail address** is in 13 early commit authors and in the history of
  one early log commit (removed from current files by the log redaction). The author decided to
  leave the history as it is.
