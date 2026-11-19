# Accounts and sessions: how they work and what they protect against

Code: `backend/internal/auth/` (rules, hashing, sessions), `backend/internal/httpapi/auth.go`
(endpoints, cookie, Origin check), `frontend/src/auth/` and `frontend/src/pages/AuthPage.tsx`.
Tests: `auth_unit_test.go`, `service_test.go` (against the test database), `httpapi/auth_test.go`,
`frontend/src/auth/safeNextPath.test.ts`.

## Endpoints

| Method & path | Body | Success | Errors |
|---|---|---|---|
| `POST /api/auth/register` | `{email, password}` | 201 `{user}` + session cookie | 400 `invalid_request` (address, password length), 409 `email_taken`, 429 `too_many_signups` |
| `POST /api/auth/login` | `{email, password}` | 200 `{user}` + session cookie | 401 `invalid_credentials`, 429 `too_many_attempts` |
| `POST /api/auth/logout` | – | 204, cookie deleted | – (always succeeds) |
| `GET /api/auth/me` | – | 200 `{user}` or `{user: null}` | 500 if the database fails |

Every state-changing request from another site's page gets 403 `forbidden_origin`.

## Design

- **Server-side sessions, not JWTs.** The cookie holds a random 32-byte token; the `sessions`
  table holds its SHA-256 hash, the user and an expiry (30 days). Logging out deletes the row, so
  the token stops working immediately everywhere, which a self-contained JWT cannot do without a
  blocklist.
- **Passwords: Argon2id** with OWASP's parameters (19 MiB, 2 passes, 1 thread), random 16-byte
  salt, stored in the standard PHC string format (`$argon2id$v=19$m=…`), compared in constant time.
- **Addresses** are trimmed and lower-cased before storing and comparing; the `users` table also
  enforces lower case with a CHECK constraint, and its UNIQUE constraint decides "already
  registered" (no check-then-insert race).

## Failure cases and how each is handled

| Case | What happens | Why | Test |
|---|---|---|---|
| Database stolen (backup, injection elsewhere) | Passwords are Argon2id hashes; session tokens are SHA-256 hashes. Neither can be used directly. | Slow, salted, memory-hard hash for low-entropy passwords; a fast hash is enough for 256-bit random tokens. | `TestRegisterLoginLogout/the password and the token are not stored in plain text` |
| Script injected into a page (XSS) tries to steal the session | The cookie is `HttpOnly`: JavaScript cannot read it. | Limits an XSS bug to the open tab instead of a stolen account. | `TestRegisterAndLoginSetTheSessionCookie` (flags) |
| Session sent over plain HTTP | `Secure` flag in production: the browser only sends it over HTTPS. | Off locally only, where the dev server is `http://localhost`. | same test, both variants |
| Another site submits a form or `fetch` to our API (CSRF) | 1) `SameSite=Lax`: the browser leaves the cookie off cross-site POSTs. 2) The `Origin` header must equal `APP_BASE_URL` for POST/PUT/PATCH/DELETE, else 403. | Two independent layers, as OWASP recommends. The Origin check also stops a forged *login* form (no cookie involved). Requests without `Origin` are not from a browser page and carry no victim cookies. | `TestOriginCheck` (other site, other scheme, look-alike host, no header, GET allowed) |
| Guessing passwords | After 5 failed logins for an address within 15 minutes, further attempts are refused (429) until the window passes, without checking the password. A successful login resets the count. | Per address, not per IP: behind Render's proxy the client IP would come from a header we would have to trust, and attackers change IPs cheaply. **Trade-off:** someone who knows an address can lock its logins for 15 minutes. Counts are in memory: lost on restart, not shared between instances (we run one). | `TestLoginLimiter`, `TestLoginIsLimitedAfterRepeatedFailures` |
| Finding out which addresses have accounts | Login: unknown address and wrong password return the same 401 and message, and an unknown address still runs one Argon2 check against a dummy hash, so the response time is the same. | Prevents account enumeration through the login form. **Not covered:** registration necessarily says "already exists" (409); hiding that would need e-mail confirmation, which is out of scope. | `TestRegisterLoginLogout/wrong password and unknown address give the same error` |
| Two registrations of the same address at once | The database's UNIQUE constraint lets exactly one succeed; the other gets 409. | A SELECT-then-INSERT check would leave a gap between the two steps. | `…/the same address cannot register twice, in any letter case` |
| Expired or forged token | `UserBySession` only accepts a token whose hash exists and has `expires_at > now()`; `/me` then also deletes the useless cookie. Expired rows of a user are removed at their next login. | No background job needed. | `TestSessions`, `TestMe` |
| Database error during `/me` | 500, not "signed out". | A failure must not look like a logout to the frontend (it would show the wrong header and redirect wrongly). | `TestMe/a failing session lookup is a 500` |
| Mass account creation (spam, or making the server hash passwords non-stop) | At most 30 new accounts site-wide per 10 minutes (429 with `Retry-After`), counted in the database and checked **before** the password is hashed. | Site-wide, not per IP, for the same reason as the login limit. **Trade-off:** a flood pauses sign-ups for everyone for a few minutes; a real shop would add e-mail verification or a CAPTCHA. Survives restarts because it counts rows, not memory. | `TestSignupsAreLimitedSiteWide` |
| Very long password or body | Passwords are limited to 128 characters; request bodies to 64 KiB. | Hashing is deliberately expensive; unbounded input would be a cheap way to load the server. | `TestValidatePassword` |
| Weak password rules | Minimum 8 characters, no composition rules, spaces allowed. | NIST SP 800-63B: length matters, "one digit and one symbol" rules mostly produce predictable passwords. | `TestValidatePassword` |
| Open redirect after login (`/login?next=https://evil.example`) | The frontend only follows `next` if it is a path on this site (starts with one `/`, not `//` or `/\`). | Otherwise the login page could send people to a phishing copy of itself. | `safeNextPath.test.ts` |
| Cached auth response | Responses that set or clear the session are `Cache-Control: no-store`. | A cache between us and the user must never store a response carrying someone's cookie. | `TestRegisterAndLoginSetTheSessionCookie` |

## Why e-mail and password only (no "magic link", no social login)

Eneba's sign-in page offers "Continue with Google / Facebook / Apple / Steam / Discord / Twitch /
X" and a password-free "Get magic link" e-mail. Our pages copy its layout but keep e-mail and
password, on purpose:

- **Magic links need an e-mail provider that actually delivers.** Sending sign-in mail from a
  real service (Postmark, SES, Resend…) requires a domain we own and have verified with SPF/DKIM
  records; mail from an unverified sender lands in spam or is refused. The project has no domain
  of its own (it runs on `*.vercel.app` and `*.onrender.com`). It would also add one-time tokens,
  their expiry and reuse rules, and a new failure mode: an evaluator who cannot receive the mail
  cannot sign in at all.
- **Social login is a separate setup per provider.** Each one needs an OAuth app registered in
  that provider's console, client id and secret, exact redirect URLs per environment, review or
  verification for some (Apple, Facebook), and account-linking rules when the same e-mail arrives
  from two providers. Seven providers would be seven of those.
- **Scope.** Authentication is "minimal but complete" in PLAN §1; the effort goes to search,
  offers and safe key delivery. E-mail + password covers the brief's requirement with nothing
  outside our control, and everything about it is tested here.

## Not done (stated honestly)

- No e-mail verification, password reset, "log out everywhere" button or 2FA (PLAN §1: auth is
  "minimal but complete").
- The login limiter is in memory and per address only (see trade-off above).
- No breached-password check (e.g. against the Have I Been Pwned range API).

## Cookie through the Vercel rewrite

The browser only talks to the Vercel domain; Vercel forwards `/api/*` to Render. So the cookie
is first-party for the Vercel domain and no CORS is needed. Verified on the deployed site after
this change (see the journal for the date and the exact check).
