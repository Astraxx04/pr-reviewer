# PR Reviewer — User Stories & Flows

This document describes every user-facing flow in the application, from first install through day-to-day usage. It is written from the perspective of the people who interact with the system, not the code.

---

## Personas

| Persona | Who they are |
|---------|-------------|
| **Owner** | The person who installs and runs the server. Automatically assigned on first login. Full control over everything. |
| **Admin** | A teammate the Owner promotes. Can manage settings, providers, users, and repos. Cannot demote the Owner. |
| **Reviewer** | An engineer on the team. Can see all reviews and PRs. Cannot change settings. |
| **Viewer** | Read-only access. Can browse the dashboard but cannot trigger actions. |
| **Developer** | Any GitHub user who opens a PR on a watched repo. May not have a dashboard account at all — they just receive AI review comments on their PR. |

---

## 1. First-Time Setup

**Who:** Owner  
**When:** Right after `docker compose up` or `go run ./cmd/server` on a fresh install

### 1a. Environment variables — what's required and why

Before starting the server for the first time, the Owner must populate a `.env` file. Copy `.env.example` and fill in the values below. These are the **only** vars that cannot be configured through the UI because they are needed before the UI is accessible or before the server can boot.

| Variable | Why it is required before first boot |
|---|---|
| `DATABASE_URL` | The server cannot start without a PostgreSQL connection. All data — users, reviews, config — lives here. Example: `postgresql://user:pass@host/db?sslmode=require` |
| `JWT_SECRET` | Used to sign and verify every JWT session token. Any string works; make it long and random. If this changes, all existing sessions are invalidated. |
| `ENCRYPTION_KEY` | A 64-character hex string (32 bytes). Used to encrypt secrets stored in the database — AI provider API keys, GitHub App private key, Jira token, SSO client secret. If this changes, all encrypted values become unreadable. Generate once with `openssl rand -hex 32`. |
| `GITHUB_CLIENT_ID` | The Client ID of a **GitHub OAuth App**. Required because it is the only login mechanism — without it, nobody can authenticate, including the Owner who would configure everything else. |
| `GITHUB_CLIENT_SECRET` | The Client Secret of the same GitHub OAuth App. Used to exchange the OAuth code for an access token during login. |
| `SERVER_URL` | The public base URL of the Go backend (e.g. `http://localhost:8001`). Used to construct the OAuth callback URL (`SERVER_URL/auth/github/callback`) that is registered in the GitHub OAuth App. Must match exactly. |
| `FRONTEND_URL` | The URL where the Next.js frontend is served (e.g. `http://localhost:3000`). After a successful OAuth login, the backend redirects to `FRONTEND_URL/auth/callback?token=...`. Must match the origin the browser uses. |

**Optional vars** (can be left unset; they only affect access-control behaviour at boot time):

| Variable | Default | Effect |
|---|---|---|
| `SERVER_PORT` | `8080` | Port the Go server listens on. |
| `APP_ENV` | `development` | Controls log format; `production` switches to JSON structured logs. |
| `REQUIRED_GITHUB_ORG` | _(none)_ | If set, only GitHub users who are members of this org can log in. |
| `INVITE_ONLY` | `false` | If `true`, new users land in `status=pending` and must be approved by an Admin before they can access the app. |
| `JWT_TTL_HOURS` | `24` | How long a session JWT is valid before the user must log in again. |

> **Everything else** — AI providers (OpenAI, Anthropic, Ollama…), notifications (Slack, email, webhook), Jira integration, OIDC SSO — is configured through the Settings UI after the first login. No env vars are needed for those.

### 1b. What is configured through the UI (not env vars)

Once logged in, the Owner configures everything else through **Settings → GitHub App**:

| What | How to get it |
|---|---|
| **GitHub App private key** | Created when you create the GitHub App on GitHub. Download the `.pem` file. |
| **Webhook secret** | A random string you invent. Paste it here *and* into the GitHub App's webhook settings on GitHub. Both sides must match or webhook signatures will fail. |
| **GitHub Personal Access Token** | Go to `github.com/settings/tokens` → New token → select `repo` + `read:org` scopes. Used to fetch PR diffs and sync org/team membership. |

All three are encrypted with AES-256-GCM before being stored in the database.

### 1d. Creating the GitHub OAuth App

The GitHub OAuth App is what allows users to sign in. It is separate from the GitHub App (which is for webhooks and repo access).

1. Go to **GitHub → Settings → Developer settings → OAuth Apps → New OAuth App**.
2. Set **Homepage URL** to `FRONTEND_URL` (e.g. `http://localhost:3000`).
3. Set **Authorization callback URL** to `SERVER_URL/auth/github/callback` (e.g. `http://localhost:8001/auth/github/callback`). This must match `SERVER_URL` in `.env` exactly.
4. Copy the **Client ID** → `GITHUB_CLIENT_ID` in `.env`.
5. Click **Generate a new client secret** → `GITHUB_CLIENT_SECRET` in `.env`.

### 1e. The setup wizard

1. Owner starts the server (`go run ./cmd/server`) and opens `http://localhost:3000`.
2. Because `setup_complete` is not yet written to the database, the frontend redirects to the **setup wizard** at `/setup`.
3. The wizard shows three checks:
   - **Database** — green immediately (the server connected and migrated on boot).
   - **GitHub OAuth** — green once `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET` are detected in the environment.
   - **AI Provider** — green once at least one provider has been saved via Settings → AI Providers. This step can be skipped and done after setup.
4. Owner clicks **Complete Setup**. The app writes `setup_complete=true` to `system_configs`. The wizard is never shown again on the same instance.

---

## 2. First Login — Owner

**Who:** Owner (first user ever)

### Flow

1. Owner clicks **Sign in with GitHub** on the login page.
2. GitHub OAuth redirects to `GET /auth/github`, which sets an `oauth_state` cookie and redirects to GitHub.
3. Owner authorises the app on GitHub.
4. GitHub redirects back to `/auth/github/callback`. The server:
   - Exchanges the code for an access token.
   - Fetches the GitHub user profile.
   - Checks the user count in the database — it is zero, so this user is assigned `role=owner, status=active` automatically.
   - Creates a session record and signs a JWT containing `sub`, `login`, `role`, and `sid`.
5. Owner is redirected to `http://localhost:3000/auth/callback?token=<jwt>`. The frontend stores the token in localStorage, then immediately replaces the URL with `/dashboard` so the token never stays in browser history.

---

## 3. Adding Team Members

### 3a. Open registration (INVITE_ONLY=false, default)

1. A teammate visits the app and clicks **Sign in with GitHub**.
2. On first login they are created with `role=viewer, status=active`.
3. They land on the Dashboard immediately — no approval needed.
4. An Admin or Owner can later promote them: **Settings → Users → Change role**.

### 3b. Invite-only registration (INVITE_ONLY=true)

1. A teammate visits the app and clicks **Sign in with GitHub**.
2. On first login they are created with `role=viewer, status=pending`.
3. They are redirected to `/auth/error?reason=pending_approval` — a "waiting for approval" screen.
4. An Admin/Owner sees them in **Settings → Users** with status `pending`.
5. Admin clicks **Approve** (`PATCH /api/users/{id}/approve`) → status becomes `active`.
6. The teammate can now log in and land on the Dashboard.
7. Admin can alternatively click **Reject** → status becomes `rejected` and they can never log in.

### 3c. GitHub org gate (REQUIRED_GITHUB_ORG set)

Before any role/status logic runs, the callback checks GitHub org membership. If the user is not a member of the configured org, they are redirected to `/auth/error?reason=org_required` and no user record is created.

---

## 4. Connecting the GitHub App

**Who:** Owner or Admin  
**Where:** Settings → GitHub App

This is what allows the app to receive webhook events from GitHub and sync repository lists.

### Flow

1. Admin goes to [github.com/settings/apps](https://github.com/settings/apps) → **New GitHub App**.
   - Sets the webhook URL to `https://your-server/webhooks`.
   - Generates a random webhook secret.
   - Downloads the private key PEM.
2. In the app: **Settings → GitHub App**, fills in **App ID** and pastes the **private key PEM**.
3. The server encrypts the private key with AES-256-GCM and stores it in `github_app_configs`.
4. Admin clicks **Test** — the server uses the App ID + key to generate a JWT, exchanges it for an installation access token, and calls the GitHub API to confirm it works.
5. Admin installs the GitHub App on their GitHub organisation or specific repos. GitHub fires an `installation` webhook.
6. The webhook handler receives the event and auto-creates `Installation` and `Repository` records in the database.

---

## 5. Selecting Repositories

**Who:** Owner or Admin  
**Where:** Repositories page

### Flow

1. Admin clicks **Sync from GitHub** → `POST /api/repos/sync`. The server uses the installation access token to list all repos the App has access to and upserts them into the database.
2. The repo list appears. Each repo shows its **Enabled / Disabled** toggle and indexing status.
3. Admin toggles a repo **on** — it is now watched. Any PR opened against it will trigger an AI review.
4. Admin toggles a repo **off** — webhooks for that repo are ignored going forward. If "purge embeddings on disable" is turned on in retention settings, the RAG embeddings for that repo are also deleted.
5. Admin can click **Index** on a repo to trigger a background job that indexes recent merged PRs into the vector store (enables RAG context for future reviews).

---

## 6. Configuring an AI Provider

**Who:** Owner or Admin  
**Where:** Settings → AI Providers

At least one provider is required before any review can happen.

### Flow

1. Admin clicks **Add Provider** and selects a type from the dropdown:
   - **OpenAI** — paste API key; model defaults to `gpt-4o`.
   - **Anthropic** — paste API key; model defaults to `claude-sonnet-4-6`.
   - **Ollama** — enter base URL (e.g. `http://localhost:11434`); no key needed.
   - **OpenAI-compatible**, **Google Gemini**, **Groq**, **Mistral**, **Together AI**, **Perplexity** — base URL pre-filled; paste API key.
2. Admin clicks **Test** — the server sends a minimal prompt to the provider's API and confirms it responds.
3. Provider is saved. API key is encrypted at rest with AES-256-GCM.
4. The provider is now available for reviews. If multiple providers are configured, they can be assigned per-repo via the repo config JSON.

---

## 7. A PR Gets Reviewed (Core Flow)

**Who:** Developer (opens a PR), AI (does the review), Developer (reads comments)

This is the primary value loop of the application.

### Flow

```
Developer opens or updates a PR on GitHub
          ↓
GitHub fires a pull_request webhook → POST /webhooks
          ↓
Server verifies HMAC-SHA256 signature (GITHUB_WEBHOOK_SECRET)
Checks for duplicate delivery ID → skips if already processed
Checks per-repo review rate limit (default: 10 reviews/hour)
          ↓
Enqueues a ReviewJob in the River queue
          ↓
ReviewWorker picks up the job and:
  1. Fetches PR diff, title, body, author from GitHub
  2. Checks .pr-reviewer-ignore → filters out ignored file paths
  3. Checks .pr-reviewer.yml → evaluates custom rules (e.g. "no console.log in JS")
  4. Applies max_diff_lines guard (default: 3000 lines) → sets DiffTruncated if over limit
  5. Fetches .github/pull_request_template.md for template-awareness
  6. Loads false-positive patterns (comments that received 2+ thumbs-down votes)
  7. Fetches Jira ticket summaries if PR title/body references tickets (e.g. PROJ-123)
  8. Retrieves RAG context — similar findings from past reviews of this repo
  9. Dispatches to two AI agents in parallel:
       - code-review agent  → correctness, architecture, performance
       - security agent     → vulnerabilities, OWASP top-10, secrets in code
  10. Merges agent results, applies consensus threshold for p2/p3 comments
  11. Posts review comments directly to the GitHub PR via the GitHub API
  12. Saves the Review + ReviewComments to the database
  13. Assigns reviewers if assignment rules are configured (round-robin, codeowners, load-balanced)
  14. Sends notifications (Slack / email / webhook) if configured
  15. Publishes a review_complete SSE event to all connected dashboard clients
          ↓
Developer sees inline comments on their PR in GitHub
```

### Comment severity labels

Each comment is prefixed with a priority label:

| Label | Priority | Score impact | Meaning |
|-------|----------|-------------|---------|
| 🔴 [P0 - Critical] | p0 | −25 | Security vulnerability, data loss risk, broken logic |
| 🟠 [P1 - High] | p1 | −15 | Significant bug, performance regression |
| 🟡 [P2 - Medium] | p2 | −5 | Code smell, maintainability issue |
| 🟢 [P3 - Low] | p3 | −1 | Style, minor nitpick |

A PR starts at score 100 and is reduced by the sum of comment penalties. Score cannot go below 0.

---

## 8. Dashboard

**Who:** All logged-in users  
**Where:** `/dashboard`

The dashboard is the landing page after login. It shows:

- **Stats cards**: total reviews, average score, approvals, change requests, comments, total repos, enabled repos.
- **Recent PRs**: a table of the latest pull requests with their status, score, and author.
- **Onboarding checklist** (dismissed once all steps are done): guides new installs through connecting the GitHub App, adding a provider, selecting repos, adding team members, and setting up notifications.
- **Real-time updates via SSE**: when a review completes, a toast notification appears without requiring a page refresh. The SSE connection passes the JWT via `?token=` query param (EventSource cannot send headers).

### Keyboard shortcuts

| Key | Action |
|-----|--------|
| `g` then `d` | Go to Dashboard |
| `g` then `r` | Go to Repositories |
| `g` then `p` | Go to Pull Requests |
| `g` then `a` | Go to Analytics |
| `j` / `k` | Move focus down / up in a list |
| `o` | Open the focused item |
| `?` | Show keyboard shortcuts help |

---

## 9. Viewing a Pull Request

**Who:** Reviewer, Admin, Owner  
**Where:** Pull Requests → click a PR

### Flow

1. User opens the PR list. Can filter by repo, author, and status (approved / changes requested / commented / pending).
2. Clicks a PR to open the detail view, which shows:
   - **PR metadata**: title, author, repo, head SHA, creation date.
   - **Review history**: a list of all AI reviews for this PR, each with score, status, summary, and timestamp.
   - **Latest comments**: all AI comments from the most recent review, grouped by file path and line.
   - **Diff view**: the full file diff with syntax-highlighted additions/deletions. AI comments are overlaid as inline rows beneath the relevant line — no need to switch to GitHub.
3. On each AI comment, the user can:
   - Click **Explain** → the AI generates a detailed explanation of why this is a problem and how to fix it.
   - Click 👍 or 👎 → submits feedback. Comments that receive 2+ thumbs-down votes are recorded as false positives and excluded from future review prompts for this repo.
4. If the PR needs a fresh review (e.g. the developer pushed a fix), an Admin/Owner can click **Re-review** → enqueues a new ReviewJob immediately.

---

## 10. Reviews List

**Who:** All logged-in users  
**Where:** `/reviews`

A paginated list of all AI review records across all repos, with score, status, summary, and timestamp. Clicking a review opens its detail including all comments. Reviews can be exported to CSV via **Settings → Audit Log** or the export endpoint.

---

## 11. Analytics

**Who:** All logged-in users  
**Where:** `/analytics`

- **Review volume chart**: daily count of reviews over a configurable time window (7 / 30 / 90 days).
- **Average score trend**: shows whether code quality is improving over time.
- **Cost breakdown**: estimated token usage and cost by repo (based on input/output token counts from the LLM API).
- **Provider health**: a per-provider status card showing last test time, latency, and whether the provider is reachable. Tested automatically every 30 minutes in the background.

---

## 12. Team Management

**Who:** Admin, Owner  
**Where:** `/team`

### Flow

1. Admin opens the Team page. Sees a list of all team members with their role and join date.
2. Clicks **Add member** → enters a GitHub login → `POST /api/team/members`. The person is added with role `reviewer` by default.
3. Can remove a member with the trash icon → `DELETE /api/team/members/{id}`.
4. Role changes happen via **Settings → Users** → **Change role** dropdown.

### Team sync

Admins can configure **Settings → Team Sync** to automatically sync team members from a GitHub organisation or team. A sync can be triggered manually or runs automatically every 6 hours.

---

## 13. Notifications

**Who:** Admin, Owner  
**Where:** Settings → Notifications

Three channel types are supported. Any number of channels can be configured. Channels can be scoped installation-wide or to a specific repo.

### Slack

1. Admin creates an incoming webhook in Slack (Apps → Incoming Webhooks).
2. Pastes the URL into the form. Selects which events to notify on: reviewer assigned, review complete, re-review requested, score below threshold.
3. Optionally sets a score threshold — only notify if score drops below N.
4. Optionally customises the message template using `{{pr.title}}`, `{{review.score}}`, `{{review.summary}}`, etc.
5. Clicks **Test** — sends a sample message to the channel.

### Email

1. Admin enters recipient addresses (comma-separated), an optional Resend API key and from address (falls back to the server env defaults).
2. Can choose a digest mode: `none` (immediate), `daily`, or `weekly`.
3. Selects events and optionally customises the email template.

### Outbound webhook

1. Admin enters an HTTPS endpoint URL and an optional signing secret.
2. On each event the server POSTs a JSON payload. If a secret is set, the payload is signed with HMAC-SHA256 and the signature is included in the `X-PR-Reviewer-Signature` header so the receiving service can verify authenticity.
3. All deliveries are logged in **Settings → Webhooks** — Admin can browse delivery history with status, response code, and timestamp.

---

## 14. Per-Repo Configuration

**Who:** Admin, Owner  
**Where:** Repositories → click repo name → Config

Each repo can have a JSON config blob that overrides global defaults:

```json
{
  "agents": {
    "code-review": { "provider_id": "2", "model": "gpt-4o-mini" },
    "security":    { "provider_id": "1", "model": "claude-sonnet-4-6" }
  },
  "max_diff_lines": 2000,
  "auto_label": true,
  "consensus_threshold": 2
}
```

- **agents**: route each agent to a specific provider and model.
- **max_diff_lines**: skip the diff and note it was too large if the PR exceeds this.
- **auto_label**: automatically apply GitHub labels (e.g. `ai-approved`, `needs-changes`) based on the review outcome.
- **consensus_threshold**: require both agents to independently flag a line before including it as a p2/p3 comment — reduces noise.

### `.pr-reviewer-ignore`

A file committed to the repo root. Any file path matching a pattern in it is excluded from the diff sent to the AI — useful for generated files, vendored code, or large lock files.

### `.pr-reviewer.yml`

A file committed to the repo root that defines custom rules evaluated before the AI prompt:

```yaml
rules:
  - name: no-console-log
    pattern: "console\\.log"
    message: "Remove debug logging before merging"
    severity: warning
    paths: ["src/**/*.js", "src/**/*.ts"]
```

Violations are pre-formatted and injected into the prompt as mandatory comments.

---

## 15. Reviewer Assignment

**Who:** Admin, Owner  
**Where:** Repositories → select repo → Assignment Rules

Assignment rules automatically request human reviewers on GitHub after the AI review completes.

### Strategies

| Strategy | How it works |
|----------|-------------|
| `round-robin` | Cycles through a configured list of team members in order |
| `codeowners` | Reads `.github/CODEOWNERS` and assigns the owner of the changed files |
| `load-balanced` | Assigns the team member with the fewest open review assignments |

---

## 16. SSO Login (if configured)

**Who:** Any user at an organisation that has set up SSO  
**Where:** Login page

When SSO is configured and enabled, a **Sign in with SSO** button appears alongside the GitHub button.

### Flow

1. User clicks **Sign in with SSO**.
2. Server loads OIDC config from DB, fetches the IdP's discovery document from `{issuer}/.well-known/openid-configuration` (cached 1 hour), and redirects the user to the IdP's `authorization_endpoint` with an `oidc_state` cookie.
3. User authenticates with their corporate IdP (Okta, Azure AD, Google Workspace, etc.).
4. IdP redirects back to `/auth/oidc/callback?code=...&state=...`.
5. Server:
   - Validates the state cookie.
   - Exchanges the code for tokens via the `token_endpoint`.
   - Fetches the JWKS from `jwks_uri` (cached 1 hour), validates the `id_token` signature and audience.
   - Extracts `sub`, `email`, `preferred_username`, and the groups claim.
   - Maps the user's IdP groups to a platform role using the configured `role_mapping` (e.g. `{"admin": ["platform-admins"], "reviewer": ["engineers"]}`).
   - Upserts the user record (always `status=active`).
   - Issues a JWT and redirects to the frontend.

> SSO users bypass the `INVITE_ONLY` gate — they are always immediately active. The `Enforced` flag in the SSO config is stored but GitHub OAuth is not currently blocked when enforcement is on.

---

## 17. API Tokens

**Who:** Any logged-in user  
**Where:** Settings → API Tokens

For use with the CLI or external automation (CI pipelines, scripts).

### Flow

1. User clicks **Generate token**, enters a name, selects scope (`read-only` or `read & write`), and optionally sets an expiry date.
2. The raw token is shown **once** — format is `prt_` followed by 43 base64url characters (256 bits of entropy). The user copies it immediately.
3. The server stores only a SHA-256 hash of the token — the raw value is never stored.
4. In subsequent API calls, the token is passed in the `Authorization: Bearer prt_...` header. The middleware hashes it and looks up the matching record.
5. Read-only tokens can only hit GET endpoints. Read & write tokens can trigger re-reviews and manage settings.
6. The token's `last_used_at` timestamp is updated asynchronously on each use.
7. The user can revoke any token with the trash icon — it is immediately invalid.

---

## 18. Audit Log

**Who:** Admin, Owner  
**Where:** Settings → Audit Log

Every administrative action is recorded: repo enabled/disabled, provider added/removed, team member added/removed, user role changed, config updated.

### Flow

1. Admin opens the Audit Log page. Sees a paginated table: timestamp, actor login, action, entity type, entity ID, IP address.
2. Can filter by actor login, entity type (repo, provider, team_member, user, config), and date range.
3. Clicks **Export CSV** to download up to 10,000 rows for compliance reporting.

---

## 19. Data Retention

**Who:** Admin, Owner  
**Where:** Settings → Data Retention

### Review retention

Admin sets a retention period in days (0 = keep forever). A background job runs daily and deletes reviews older than the threshold, along with their comments and embeddings.

### Purge embeddings on disable

If enabled, disabling a repo also deletes all its RAG code embeddings from the vector store.

### GDPR right to erasure

1. Admin enters a GitHub login in the erasure form and clicks **Erase all data for this user**.
2. A confirmation dialog warns that this is irreversible.
3. On confirm, the server:
   - Deletes the User record.
   - Anonymises all AuditLog entries for that actor (sets `actor_login` to `[deleted]`).
   - Deletes all API tokens belonging to that user.
4. A toast confirms erasure.

---

## 20. Jira Context Injection

**Who:** Admin (setup), Developer (passive benefit)  
**Where:** Settings → Jira (setup only)

### Setup

1. Admin goes to **Settings → Jira**, enters the Jira base URL (`https://yourcompany.atlassian.net`), account email, and API token.
2. Clicks **Test connection** — the server calls `/rest/api/3/myself` to verify credentials.
3. Saves. API token is encrypted at rest.

### Runtime (automatic)

When a ReviewJob runs, it scans the PR title and description for Jira ticket references matching the pattern `PROJECT-123` (uppercase project key, hyphen, number). For each reference found (up to 3), it calls the Jira API to fetch the ticket's summary, type, and status. This context is injected into the review prompt:

```
Linked Jira tickets (for additional context only):
- PROJ-123 [Story / In Progress]: Add rate limiting to the payment API
- INFRA-45 [Bug / Done]: Fix connection pool exhaustion under load
```

The AI uses this to understand the intent behind the PR without the reviewer having to summarise it in the PR description.

---

## 21. Rate Limiting

### API rate limiting

All authenticated API endpoints are rate-limited at 1,000 requests per hour per user (keyed by user ID). Unauthenticated requests are keyed by IP address. Exceeding the limit returns HTTP 429.

### Per-repo review rate limiting

Each repo has a configurable max number of AI reviews per hour (default: 10, stored in `system_config` as `review_rate_limit_per_hour`). This prevents a single active repo from flooding the review queue during a rapid push session.

---

## 22. Sessions

**Who:** Any logged-in user  
**Where:** Settings → Sessions (via the account menu)

Every login creates a session record with user agent, IP address, and expiry time. Users can:
- See all active sessions.
- Revoke a specific session (e.g. after logging in from a public computer).
- Revoke all sessions except the current one.

Revoking a session invalidates the JWT immediately — the auth middleware validates the session ID against the database on every request.

---

## 23. Dark Mode

The app respects the system preference by default. A sun/moon toggle in the bottom-left of the sidebar switches between light and dark mode. The preference is persisted across page reloads via `next-themes`.

---

## Summary: Role Permissions

| Action | Viewer | Reviewer | Admin | Owner |
|--------|--------|----------|-------|-------|
| View dashboard, PRs, reviews | ✓ | ✓ | ✓ | ✓ |
| View analytics | ✓ | ✓ | ✓ | ✓ |
| Trigger re-review | | ✓ | ✓ | ✓ |
| Submit comment feedback (👍/👎) | ✓ | ✓ | ✓ | ✓ |
| Manage repos (enable/disable/sync) | | | ✓ | ✓ |
| Manage AI providers | | | ✓ | ✓ |
| Manage notifications | | | ✓ | ✓ |
| Manage team members | | | ✓ | ✓ |
| Approve / reject pending users | | | ✓ | ✓ |
| View & export audit log | | | ✓ | ✓ |
| Manage data retention & GDPR | | | ✓ | ✓ |
| Configure SSO | | | ✓ | ✓ |
| Manage GitHub App settings | | | ✓ | ✓ |
| Generate / revoke own API tokens | ✓ | ✓ | ✓ | ✓ |
| Change any user's role | | | ✓ | ✓ |
| Demote / change Owner's role | | | | ✓ |
