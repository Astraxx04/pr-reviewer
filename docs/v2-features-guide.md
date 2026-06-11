# V2 Features — Architecture & Manual Test Guide

This guide walks through the five features added in this round, explains how each works (with
file references), and gives step-by-step manual tests plus the automated tests that cover them.

Features covered:

1. [Two-way Slack bot](#1-two-way-slack-bot)
2. [Branch protection / commit status](#2-branch-protection--commit-status)
3. [PDF report export](#3-pdf-report-export)
4. [Daily / weekly email digest](#4-daily--weekly-email-digest)
5. [VS Code extension](#5-vs-code-extension)

> **GitHub Action (12.3)** and **Linear (12.1)** were intentionally left out of scope.

---

## 0. Orientation — how the system fits together

Before testing, a 5-minute mental model. (See `docs/running-locally.md` for environment setup.)

```
                         ┌──────────────────────────┐
   GitHub ──webhook────▶ │  POST /webhooks          │
   Slack  ──command────▶ │  POST /slack/commands    │  (public; verified by signature)
   Slack  ──event──────▶ │  POST /slack/events      │
                         └─────────────┬────────────┘
                                       │ enqueue
   Browser / CLI / VS Code            ▼
   ──Bearer JWT/token──▶  /api/*  →  River job queue (Postgres)  →  Workers
                          (auth                                     - ReviewWorker
                           middleware)                              - EmailDigestWorker
                                                                    - TeamSyncWorker, …
                                       │                                   │
                                       ▼                                   ▼
                                   Postgres (GORM models)          GitHub API / Slack / Resend
```

Key building blocks you'll see referenced throughout:

| Concern | Where | Notes |
|---|---|---|
| HTTP routing | `internal/http/router.go` | **Public** routes on the main mux (`/webhooks`, `/slack/*`, `/health`); **authenticated** routes on the `api` mux wrapped by `middleware.Auth`. |
| Auth | `internal/http/middleware/auth.go` | `Authorization: Bearer <jwt>` for the web app, or `Bearer prt_…` API tokens (used by CLI + VS Code). |
| Background jobs | River (`cmd/server/main.go`) | Workers registered with `river.AddWorker`; recurring work via `river.NewPeriodicJob`. |
| Secrets at rest | `internal/db/crypto.go` (`db.Encrypt`/`db.Decrypt`) | AES-256-GCM, keyed by `ENCRYPTION_KEY`. Used for Slack/Jira/GitHub-App secrets. |
| Review pipeline | `internal/jobs/review_job.go` (`ReviewWorker.Work`) | Builds context → runs AI → posts to GitHub → persists → notifies. The hook points for commit status and Slack replies live here. |
| Per-repo config | `Repository.Config` JSON | Section-8 settings live under a nested `{agents, commit_status, …}` shape. |
| Frontend | `web/app/(dashboard)/…` + `web/lib/api.ts` | Pages are client components; all API calls go through the `apiFetch` wrapper which attaches the bearer token. |

To run everything: `make dev` (backend live-reload + frontend) then open http://localhost:3000.
To run the test suite: `make test` (Go race tests + `tsc`), or targeted `go test ./internal/...`.

---

## 1. Two-way Slack bot

### What it does
- `/review owner/repo#42` — queues a review; replies in Slack when it finishes.
- `/review-status owner/repo#42` — replies inline with the latest stored review.
- `@mention` the bot with a PR reference — queues a re-review and acknowledges in-channel.
- All inbound requests are verified using Slack's request signature (HMAC-SHA256).

### Code walkthrough (data flow)

**Credentials.** A single global `SlackAppConfig` (`internal/db/models/models.go`) stores the
signing secret and bot token as AES-GCM ciphertext. Managed by the admin endpoints in
`internal/http/handlers/slack_app.go` (`Get`/`Put`/`Delete`/`Test`), surfaced at
**Settings → Slack Bot** (`web/app/(dashboard)/settings/integrations/slack/page.tsx`).

**Inbound slash command** (`SlackAppHandler.HandleCommand`):
1. `verified()` reads the raw body and calls `slack.VerifySignature` (`internal/integration/slack/verify.go`)
   — rebuilds `v0:{timestamp}:{body}`, HMAC-SHA256 with the signing secret, constant-time compares
   against `X-Slack-Signature`, and rejects requests older than 5 minutes (replay protection).
2. Parses the form, extracts the PR ref via `slack.ParsePRRef` (`parse.go`, regex `owner/repo#N`).
3. `/review` → enqueues `jobs.ReviewJobArgs{… SlackResponseURL: response_url}`; immediately returns
   an ephemeral "queued" ack. `/review-status` → `latestReviewText()` queries the most recent review
   and returns it inline.

**Async reply.** When `ReviewWorker.Work` (`internal/jobs/review_job.go`) finishes and
`args.SlackResponseURL` is set, it POSTs the verdict + score to that URL via
`slack.PostResponseURL` (`client.go`). The response URL is valid ~30 min, which comfortably covers a review.

**Events API** (`HandleEvents`): answers the one-time `url_verification` challenge, and on
`app_mention` enqueues a re-review and replies with the bot token via `slack.PostMessage` (`chat.postMessage`).

**Routing** (`router.go`): `/slack/commands` and `/slack/events` are **public** (no JWT — they
authenticate by signature); `/api/settings/slack-app*` are admin endpoints behind auth.

### Manual test

**A. Signature verification (no Slack workspace needed)** — proves the security gate works:

```bash
# 1. Save a signing secret in the UI (Settings → Slack Bot), e.g. "testsecret".
# 2. A request with a bad signature must be rejected:
curl -i -X POST http://localhost:8001/slack/commands \
  -H 'X-Slack-Request-Timestamp: 9999999999' \
  -H 'X-Slack-Signature: v0=deadbeef' \
  -d 'command=/review&text=acme/web%231'
# Expect: HTTP 401 invalid signature
```

**B. Full Slack workspace test** (needs a Slack App + a public URL via `ngrok http 8001`):
1. Create a Slack App at https://api.slack.com/apps.
2. In **Settings → Slack Bot** of PR Reviewer, paste the **Signing Secret** (Basic Information) and
   **Bot User OAuth Token** (`xoxb-…`), Save, then click **Test bot token** → expect "Bot token is valid".
3. In Slack App config, copy the URLs shown on the PR Reviewer settings page:
   - Slash Commands → create `/review` and `/review-status`, Request URL = `https://<ngrok>/slack/commands`.
   - Event Subscriptions → Request URL = `https://<ngrok>/slack/events` (Slack verifies it live),
     subscribe to the `app_mention` bot event.
4. In Slack: `/review-status owner/repo#1` → expect an inline reply (latest review or "no review yet").
5. `/review owner/repo#1` → expect an immediate "⏳ queued" ack, then a follow-up message with the
   score/verdict once the review completes.
6. `@YourBot please look at owner/repo#1` → expect a "Re-review queued" reply.

### Automated tests
`internal/integration/slack/slack_test.go`:
- `TestVerifySignature` — valid signature accepted; tampered signature, stale timestamp, and empty
  secret all rejected.
- `TestParsePRRef` — parses `acme/web#42`, handles surrounding text/mentions, rejects `#0` and non-refs.

Run: `go test ./internal/integration/slack/...`

---

## 2. Branch protection / commit status

### What it does
Posts a GitHub commit status named **`pr-reviewer`**: `pending` when a review starts, then
`success`/`failure` based on whether the score meets a per-repo threshold. You can then mark this
status **required** in GitHub branch protection to block merges of low-scoring PRs.

### Code walkthrough
- **Client method** `gh.Client.CreateStatus` (`internal/github/client.go`) wraps go-github's
  `Repositories.CreateStatus`; the `CommitStatus` struct (`internal/github/models.go`) carries
  state/description/context/target URL.
- **Per-repo config**: `commitStatusConfig{Enabled, MinScore}` parsed from `Repository.Config`
  under the `commit_status` key (`internal/jobs/review_job.go`).
- **In the pipeline** (`ReviewWorker.Work`): if enabled, posts `pending` right after building PR
  context (using `prCtx.PR.Head.Sha`); after posting the review, calls `postCommitStatus` with
  `success` (score ≥ `MinScore`) or `failure`. `TargetURL` deep-links to `<FRONTEND_URL>/prs/owner/repo/N`.
  All status calls are non-fatal — a status API hiccup never fails the review.
- **UI**: the "Branch protection" card on the repo config page
  (`web/app/(dashboard)/repos/[id]/page.tsx`) toggles `enabled` and sets `min_score`. This page now
  reads/writes the **nested** config shape (`{agents, commit_status, …}`) while still loading legacy
  flat configs, so older repos keep working.

### Manual test
1. **Settings → AI Providers**: configure at least one provider (reviews need an LLM).
2. Open a repo (**Repos → a repo → Configure**), enable **Post commit status on each review**, set
   **Minimum score to pass** (e.g. 60), Save.
3. Ensure `GITHUB_TOKEN` (or GitHub App token) has access to the repo; expose `/webhooks` via ngrok
   and point the repo/App webhook at it (see running-locally §6).
4. Open or push to a PR → on the PR's commit in GitHub you should see a **pr-reviewer** check go
   `pending` → then `success` or `failure` depending on the score. The check links back to the PR
   detail page in the dashboard.
5. (Optional) In GitHub → Branch protection, mark **pr-reviewer** as a required status check and
   confirm a `failure` blocks merge.

> No GitHub access handy? You can still confirm wiring: with commit status enabled and an invalid
> token, the server logs `failed to post commit status` (non-fatal) while the review still completes —
> proving the hook fires at the right point without breaking the pipeline.

### Automated tests
The commit-status decision is a thin `score < min_score` branch inside the worker; the GitHub call is
covered by the interface mock (`mockGHClient.CreateStatus` in `internal/http/webhook_handler_test.go`,
exercised by the webhook handler tests). End-to-end status posting is best verified manually against a
real repo as above.

---

## 3. PDF report export

### What it does
Adds a **Export PDF** button next to **Export CSV** on the Reviews page. The PDF is a summary report:
totals (count, average score, verdict breakdown), a per-repository table, and the most recent reviews.

### Code walkthrough
- **Handler** `ExportHandler.ReviewsPDF` (`internal/http/handlers/export.go`) reuses the same
  `queryRows` + `parseDateRange` as the CSV export, then calls two **pure, unit-tested** helpers:
  - `summarizeReviews(rows)` → totals + per-repo aggregates.
  - `renderReportPDF(rows, start, end, repoFilter, generatedAt)` → PDF bytes via `go-pdf/fpdf`.
- **Route**: `GET /api/reviews/export.pdf` (authenticated, `router.go`).
- **Auth note**: the export routes sit behind Bearer auth. CSV uses `window.open` (no auth header),
  but the PDF button calls `downloadReviewsPDF` (`web/lib/api.ts`) which does an authenticated
  `fetch` → blob → triggered download, so the token reaches the protected route correctly.

### Manual test
1. Make sure you have at least a few reviews in the DB (`make seed` populates sample data).
2. Go to **Reviews** → click **Export PDF**. The button shows "Generating…" then a
   `pr-reviewer-report_YYYYMMDD.pdf` downloads.
3. Open it: confirm the header, Summary numbers, the "By repository" table, and "Recent reviews" list
   match what you see in the UI.
4. (Optional) Hit the endpoint directly to verify auth:
   ```bash
   TOKEN=... # a JWT from localStorage, or a prt_ API token
   curl -s -H "Authorization: Bearer $TOKEN" \
     "http://localhost:8001/api/reviews/export.pdf?start=2026-01-01&end=2026-12-31" \
     -o report.pdf && file report.pdf      # → "PDF document"
   curl -i "http://localhost:8001/api/reviews/export.pdf"   # no token → 401
   ```

### Automated tests
`internal/http/handlers/export_test.go`:
- `TestSummarizeReviews` / `TestSummarizeReviewsEmpty` — aggregate counts, average, per-repo tallies.
- `TestRenderReportPDF` — output is non-empty and starts with the `%PDF` magic header.
- `TestParseDateRange` — start/end (end exclusive, +1 day) and repo filter parsing.

Run: `go test ./internal/http/handlers/...`

---

## 4. Daily / weekly email digest

### What it does
Sends one summary email per period (instead of per-event alerts) to recipients whose email channel is
set to `digest: daily` or `digest: weekly`. The email aggregates the period's reviews (count, average
score, verdict breakdown, per-repo, recent list).

### Code walkthrough
- **Config**: `EmailChannelConfig.Digest` (`internal/notifications/service.go`) — `none|daily|weekly`,
  set per email channel in the notifications UI (the **Digest** dropdown in `EmailForm`).
- **Worker** `EmailDigestWorker.Work` (`internal/jobs/email_digest_job.go`): loads enabled email
  configs, filters to those whose `Digest` equals the run period, `aggregate()`s reviews for the
  config's installation (and optional repo) since the cutoff, renders HTML with `renderDigestHTML`
  (titles HTML-escaped), and sends via `notifications.SendEmail` (Resend). Configs with no activity in
  the window are skipped (no empty emails).
- **Scheduling** (`cmd/server/main.go`): two `river.NewPeriodicJob`s — daily (24h, `Period:"daily"`)
  and weekly (7d, `Period:"weekly"`). Both are always scheduled; the worker only emails configs whose
  cadence matches.
- **On-demand trigger**: `POST /api/settings/notifications/digest/trigger?period=daily` enqueues a
  digest immediately (`NotificationHandler.TriggerDigest`). Exposed as the **Send digest now** button
  on the notifications page — added specifically so you don't have to wait 24h to test.

### Manual test
1. **Settings → Notifications → Add channel → Email**. Set recipients, a Resend API key (or set
   `RESEND_API_KEY` in `.env`), tick the **Review complete** event, and set **Digest = Daily**. Save.
2. Ensure there are some reviews in the last 24h (`make seed`, or run a couple of reviews).
3. Click **Send digest now** → toast confirms it's queued. Within a moment the recipient gets a
   "[PR Reviewer] Daily digest — N reviews" email; check the summary numbers and that PR titles render
   safely (no raw HTML).
4. Verify scoping: a digest only includes reviews for the channel's installation (and its specific
   repo if the channel is repo-scoped).
5. (Optional, no Resend key) Trigger it anyway and watch the server log: with no API key,
   `SendEmail` is a no-op, but you'll see the worker run and `digest emails sent` only when something
   actually sends — confirming the job path executes.

   Direct trigger via curl:
   ```bash
   curl -X POST -H "Authorization: Bearer $TOKEN" \
     "http://localhost:8001/api/settings/notifications/digest/trigger?period=daily"
   # → {"ok":true,"period":"daily"}
   ```

### Automated tests
`internal/jobs/email_digest_test.go`:
- `TestRenderDigestHTML` — includes the period heading, PR refs, counts, and **escapes** a title
  containing `<script>` (injection guard).
- `TestCapitalize` — period label capitalization.

Run: `go test ./internal/jobs/...`

---

## 5. VS Code extension

### What it does
A standalone extension in `vscode-extension/` that talks only to the PR Reviewer API (never GitHub):
- **PR Reviewer: Show Findings for a PR** — detects the repo from your git `origin`, lists PRs, and
  renders the chosen PR's review comments as diagnostics (squiggles + Problems panel), colored by
  priority (P0/P1 → error, P2 → warning, P3 → info).
- **PR Reviewer: Trigger Review** — queues a review for a chosen PR.
- **PR Reviewer: Clear Findings**.

### Code walkthrough
- `src/git.ts` `detectRepo` runs `git remote get-url origin` and `parseRemote` extracts `owner/repo`
  from SSH or HTTPS URLs.
- `src/api.ts` `Client` is a thin fetch wrapper sending `Authorization: Bearer <prt_ token>`; methods
  map to `GET /api/prs?repo=`, `GET /api/prs/{owner}/{repo}/{number}`, and the `re-review` POST.
- `src/extension.ts` registers the three commands; `renderDiagnostics` maps each `PRComment`
  (path, line, priority, body) to a `vscode.Diagnostic` in the matching workspace file, with
  `severityFor` translating priority to severity.
- Auth reuses the existing API-token mechanism — generate a `prt_` token under **Settings → API Tokens**.

### Manual test
1. Build it:
   ```bash
   cd vscode-extension
   npm install
   npm run compile
   ```
2. Open the `vscode-extension/` folder in VS Code and press **F5** → an Extension Development Host opens.
3. In the dev host, open a checked-out repo that PR Reviewer has reviewed. Set in Settings:
   - `prReviewer.serverUrl` = `http://localhost:8001`
   - `prReviewer.apiToken` = a `prt_…` token from the dashboard.
4. Command Palette → **PR Reviewer: Show Findings for a PR** → pick a PR → findings appear as
   squiggles in the referenced files and in the Problems panel.
5. **PR Reviewer: Trigger Review** → pick a PR → toast "review queued"; confirm a new review appears
   in the dashboard shortly after.
6. **PR Reviewer: Clear Findings** removes the diagnostics.

### Automated tests
The git-remote parser (`parseRemote`) is the riskiest pure logic. It was validated against SSH/HTTPS/
`.git`-suffixed/invalid inputs (all pass). The extension has no bundled test runner; quickest re-check
after changes:
```bash
cd vscode-extension && npm run compile
node -e 'const {parseRemote}=require("./out/git.js"); console.log(parseRemote("git@github.com:acme/web.git"))'
# → { owner: 'acme', repo: 'web' }
```

---

## 6. Running the full automated suite

```bash
# Everything (Go race tests + frontend type-check):
make test

# Just the new backend unit tests:
go test ./internal/integration/slack/... ./internal/http/handlers/... ./internal/jobs/...

# Build the VS Code extension:
cd vscode-extension && npm install && npm run compile
```

> Note: `web/app/(dashboard)/settings/audit/page.tsx` and `web/app/(dashboard)/team/page.tsx` have two
> pre-existing `tsc` errors unrelated to these features. They were present before this work.

---

## 7. Quick manual test checklist

- [ ] Slack: bad-signature curl returns 401
- [ ] Slack: `/review-status` replies inline; `/review` acks then follows up; `@mention` re-reviews
- [ ] Commit status: PR shows `pr-reviewer` going pending → success/failure with the right threshold
- [ ] PDF: Reviews → Export PDF downloads a valid report; unauthenticated endpoint returns 401
- [ ] Digest: email channel with Digest=Daily + **Send digest now** delivers a summary email
- [ ] VS Code: Show Findings renders diagnostics; Trigger Review queues a review
