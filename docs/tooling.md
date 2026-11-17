# AI tooling used in this project

What was added to Claude Code for this project, why, and what was deliberately left out.
Checked on 2026-11-16. Rule followed: prefer Anthropic's official marketplace and well-known
vendors; for anything third-party, read what it actually runs before installing.

## Installed (project scope — recorded in `.claude/settings.json`)

### 1. Conversation logging hook (written for this project)
- **What:** `.claude/hooks/export-log.mjs`, registered on `Stop`, `SubagentStop`, `PreCompact`,
  `SessionEnd`. Exports every session to `ai-logs/`.
- **Source:** this repository. No dependencies, ~350 lines of Node.js.
- **Why:** the assignment requires the full agent chat history.
- **How I use it:** automatic. Details in [`ai-logs/README.md`](../ai-logs/README.md).

### 2. Playwright MCP server (`playwright@claude-plugins-official`)
- **What:** Microsoft's official browser-automation MCP server. Lets Claude open pages, click,
  fill forms and take screenshots in a real browser.
- **Source:** <https://github.com/microsoft/playwright-mcp> (npm `@playwright/mcp`, last published
  2026-11-09), listed in Anthropic's official plugin marketplace
  <https://github.com/anthropics/claude-plugins-official> under `external_plugins/playwright`.
- **What it runs (checked):** the plugin contains only a manifest and an MCP config that starts
  `npx @playwright/mcp@latest`. No hooks, no other scripts.
- **Why:** visual checks of our pages against `docs/screenshots/`, and click-through tests of the
  purchase flow (search → product → cart → sign in → pay → key) on localhost and on the deployed URL.
- **How I use it:** "open localhost:5173/store, take a screenshot, compare with
  `docs/screenshots/08-store-filters.jpg` and list the differences"; end-of-day smoke test of the
  deployed app.
- **Note:** becomes active after restarting Claude Code; the first run downloads a browser.

### 3. TypeScript language server (`typescript-lsp@claude-plugins-official`)
- **What:** connects Claude Code to `typescript-language-server`, giving it real type errors,
  go-to-definition and find-references for `.ts/.tsx` instead of guessing from text.
- **Source:** Anthropic official marketplace (`plugins/typescript-lsp`); server:
  <https://github.com/typescript-language-server/typescript-language-server>.
- **What it runs (checked):** manifest only; it launches the language server binary, which was
  installed with `npm install -g typescript-language-server typescript`.
- **Why:** I know C#/C++ better than TypeScript — compiler-grade feedback on every edit catches
  type mistakes before I have to.
- **How I use it:** automatic while Claude edits frontend code.

## Planned (I install these before the first coding session — see `docs/setup.md`)

### 4. Go language server (`gopls-lsp@claude-plugins-official`)
- **What:** same idea for Go, using the Go team's `gopls`.
- **Source:** Anthropic official marketplace (`plugins/gopls-lsp`); <https://pkg.go.dev/golang.org/x/tools/gopls>.
- **Why not yet:** Go was not installed on this machine during the setup session; the plugin
  needs the `gopls` program. Go is the confirmed backend language.
- **Install:** `winget install GoLang.Go` → `go install golang.org/x/tools/gopls@latest` →
  `claude plugin install gopls-lsp@claude-plugins-official --scope project`.

### 5. Stripe CLI (not a Claude plugin)
- **What:** Stripe's official command-line tool; forwards test webhooks to localhost.
- **Source:** <https://docs.stripe.com/cli>.
- **Why:** needed to test the payment webhook locally (`stripe listen --forward-to …`).

## Built into Claude Code (no install needed)

| Tool | Use in this project |
|---|---|
| `/code-review` | Review each feature's diff for bugs before committing. |
| `/security-review` | Run before the auth and checkout work is merged (sessions, webhook, key access). |
| `/simplify` | Clean-up pass so the code stays explainable. |
| Web search / fetch | Checking current docs (Go, Stripe, Render, Neon) instead of trusting model memory. |
| Claude in Chrome | Used in the first session to inspect eneba.com and measure its styles. |

## Libraries chosen during the build

AI tooling is above; this section records the application libraries that needed a real choice.

### Combobox for the search dropdown: Downshift (chosen) vs React Aria

Decided 2026-11-17. The dropdown needs the ARIA combobox pattern (arrow keys, Enter, Escape,
`aria-activedescendant`, screen-reader announcements), which is easy to get subtly wrong by hand.

| | Downshift `useCombobox` | React Aria `ComboBox` (react-aria-components) |
|---|---|---|
| What it is | One hook that returns "prop getters"; renders nothing | Complete unstyled components (ComboBox, Input, Popover, ListBox, sections) |
| Size in our bundle (measured: esbuild, minified, React excluded) | **13.5 KB gzip** (43 KB min) | **61.6 KB gzip** (204 KB min) for the parts we would use |
| Our markup and Tailwind classes | Fully ours | Ours, but inside its component structure and data attributes |
| Server-side results, Enter submits free text | Natural: items are whatever we pass; Enter with nothing highlighted is left to the form | Supported (`allowsCustomValue`, `items`), more configuration |
| Sections (suggestions / products) | Not built in; one flat list (visual divider only) | Built in (`ListBoxSection`) |
| Maintenance | Active (v9.5.0) | Active, Adobe, very widely used |

**Chosen: Downshift.** Our whole app was 97 KB gzip; React Aria would add about 60% to it for one
widget. Downshift's hook model fits how the rest of the frontend is written (plain elements +
Tailwind), and its behaviour can be adjusted in one pure function (`searchStateReducer`) that is
unit tested. What we give up: React Aria's built-in sections and its broader component set. We
would reconsider if several more complex widgets (date pickers, menus) were needed.

Lesson recorded: the first version passed the input text in from React state ("controlled"), and
fast typing lost letters, because Downshift reports a change only after its own render. The box
now lets Downshift own the text (`initialInputValue`) and only mirrors it for the request.

Sources: [downshift on npm](https://npmjs.com/downshift),
[React Aria ComboBox](https://react-aria.adobe.com/ComboBox),
[a 2026 comparison of select libraries](https://www.pistack.xyz/posts/2026-08-24-react-select-libraries-react-select-downshift-react-aria-comparison/);
bundle sizes measured by me on 2026-11-17.

### Test runner: Vitest

The Vite project's own test runner: same config and TypeScript handling as the build, nothing
extra to set up. Used for logic without React (URL ↔ filters, money, dropdown state).

## Considered and rejected

| Tool | Source | Why not |
|---|---|---|
| `frontend-design` skill | Anthropic official | Its instructions push for a *distinctive, original* visual identity. This project needs the opposite: a faithful copy of Eneba's existing design from `docs/design-reference.md`. It would work against the brief. |
| `context7` MCP | Upstash (third-party, in official marketplace) | Remote server that returns library docs; now asks for an account/sign-in. Built-in web fetch of official docs covers the need without sending queries to another service. |
| `security-guidance` plugin | Anthropic official | Runs Python hooks and extra model calls on every edit. The built-in `/security-review` at key points is enough for a one-week project. |
| `code-review`, `pr-review-toolkit`, `commit-commands` plugins | Anthropic official | Overlap with built-in `/code-review`; this is a solo repo without pull requests. |
| `feature-dev`, `ralph-loop` and similar autonomous workflow plugins | Anthropic official | They make the agent work in long unattended loops. I need to follow and be able to explain every step, so I prefer short, reviewed steps. |
| Community "awesome" skill/agent packs | various GitHub repos | Unknown authors, large prompt bundles and hooks that would run on my machine; not worth the review effort or the risk. |
| Postgres MCP servers | various | Would give the agent direct access to the database with credentials. Migrations and SQL files in the repo are safer and reviewable. |
