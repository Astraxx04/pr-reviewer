# PR Reviewer — Comprehensive Test & Code-Review Plan

> Goal: review every part of the code and exercise every API / feature manually.
> This document is the map. Work top-down: orient → set up → review by track → test by track.

---

## 0. System map (where everything lives)

```
cmd/server/main.go        ← composition root: wires DB, River queue, AI, handlers, periodic jobs
internal/
  config/                 env loading + Validate() (DATABASE_URL, ENCRYPTION_KEY required)
  http/
    router.go             ← THE route table (87 endpoints). Start here for the API surface.
    middleware/           auth (JWT + API token + session), cors, ratelimit
    handlers/             one file per resource (auth, repos, reviews, providers, …)
    webhook_handler.go    GitHub webhook ingress (HMAC verify → enqueue River job)
    inprocess_handler.go  fallback when no DB (synchronous, jobs lost on restart)
  jobs/                   River workers: review_job (core!), conversation, team_sync, email_digest, index_repo
  ai/
    reviewer.go           ← core review pipeline: prompt → fan-out agents → merge → score
    agent.go / agents/    code_review, security, conversation agents (MCP-style)
    llm/ + llm/adapters/  provider registry; anthropic + openai_compatible + ollama adapters
    rag/                  pgvector retriever + indexer (RAG context)
    embeddings/           openai + ollama embedders
  github/                 GitHub REST client, GitHub App auth, CODEOWNERS parsing
  pr/                     PR context builder (diff fetch + assembly)
  review/                 aggregator (agent results → final review verdict)
  rules/                  .pr-reviewer.yml custom rules + .pr-reviewer-ignore
  db/                     gorm connect, AutoMigrate, crypto (AES-256 field encryption)
    models/ repo/         schema + repositories (data access)
  notifications/          slack, email (Resend), webhook, digest templates
  integration/            jira, slack (slash commands, events, signature verify)
  assignments/            reviewer auto-assignment rule evaluator
  events/                 SSE hub
web/                      Next.js app (dashboard, settings, PR views)
```

**Core data flow (the thing that must work):**
```
GitHub PR event → /webhooks (HMAC verify) → River "review" job
  → pr.BuildContext (fetch diff) → load repo config / ignore / .pr-reviewer.yml / Jira ctx
  → ai.Review: render prompt → fan out [code-review, security] agents in parallel
              → parse JSON → consensus filter → merge → computeScore
  → aggregator.Aggregate → GHClient.PostReview (inline) + PostSummaryComment + commit status
  → persist review/comments → notify (slack/email) → index for RAG → assign reviewers → labels → SSE
```

---

## 1. Current environment state (verified)

| Thing | State |
|---|---|
| Backend | Running on `:8001`, healthy (`/healthz` → db ok, providers:1) |
| Frontend | Running on `:3000` |
| Database | Remote **Neon** Postgres (pgvector), setup complete |
| Go build | `CGO_ENABLED=0 go build ./cmd/server` → passes; `go vet ./internal/...` clean |
| Go version | 1.26.3 |
| Node | v20.18.1 |
| **Missing tools** | `golangci-lint`, `air`, `psql` (install before full local dev) |
| Docker | binary present, **daemon (OrbStack) not running** |
| ngrok | installed (needed for live webhook testing) |

**Install the gaps before deep work:**
```bash
brew install golangci-lint
go install github.com/air-verse/air@latest
brew install libpq && brew link --force libpq   # for psql
```

---

## 2. Code-review tracks (prioritized)

Review in this order — highest risk / blast radius first. Each track lists what to read and what to look for.

### Track A — Authorization & security (DO FIRST) 🔴
The auth layer gates everything. Findings here are the most damaging.

**Read:** `middleware/auth.go`, `middleware/cors.go`, `middleware/ratelimit.go`, `db/crypto.go`, `handlers/auth.go`, `handlers/sso.go`, `webhook_handler.go`, `integration/slack/verify.go`.

**Checklist:**
- [ ] **RBAC enforcement.** `RequireRole()` exists in `middleware/auth.go` but is **not referenced in `router.go`**. Confirm whether admin-only endpoints self-check the role. Test as a non-admin user against: `PATCH /api/users/{id}/role`, `/approve`, `/reject`, `DELETE /api/users/{login}/data` (GDPR erase), `PUT /api/settings/retention`, `PUT/DELETE /api/settings/sso`, `POST /api/providers`, `PUT /api/settings/github-app`, `/api/tokens`, `/api/settings/integrations/jira`, `/api/settings/slack-app`. Only `sso.go, users.go, team.go, helpers.go` reference roles — everything else is suspect.
- [ ] JWT validation: algorithm pinned to HMAC (✓ in code) — verify no `alg:none` bypass, expiry enforced, session revocation honored (`sid` claim → Session row).
- [ ] API token path (`prt_` prefix): SHA-256 lookup, expiry check, `last_used_at` updated in goroutine using request DB — check for races / context-after-return.
- [ ] Encryption: `db/crypto.go` AES-256-GCM — verify nonce uniqueness, key length validation, that secrets (provider API keys, GitHub token, webhook secret, Jira creds) are never returned in API responses.
- [ ] Webhook HMAC: `webhook_handler.go` — constant-time signature compare; rejects missing/invalid sig; replay handling via delivery ID.
- [ ] Slack signature verify: `integration/slack/verify.go` — timestamp window, signing-secret compare.
- [ ] CORS: `middleware/cors.go` — origin allowlist not `*` with credentials.
- [ ] Rate limiter: keyed by user after auth (per `router.go`) — confirm unauthenticated routes aren't unbounded.
- [ ] SSE auth: `/api/events` authenticates via `?token=` query param (logged in URLs?) — check token leakage.
- [ ] SSRF: provider `base_url`, Ollama URL, Jira URL, custom webhook URLs are user-supplied and called server-side — check for internal-network reachability.

### Track B — Core review pipeline 🔴
This is the product. If it's wrong, nothing else matters.

**Read:** `jobs/review_job.go`, `ai/reviewer.go`, `ai/agent.go`, `ai/agents/*`, `review/aggregator.go`, `pr/service.go`, `ai/prompt.go`.

**Checklist:**
- [ ] `parseAgentResponse` (reviewer.go:225) — fragile markdown-fence stripping + `json.Unmarshal`. Test with: no fences, ```json fences, prose-before-JSON, malformed JSON, empty content. A parse failure silently drops the agent's findings (`continue`).
- [ ] Consensus filter (reviewer.go:151-196) — only applies when `ConsensusThreshold > 1`, only to p2/p3. Verify p0/p1 always survive; verify dedup-by-(path,line) is correct.
- [ ] `computeScore` (reviewer.go:240) — p0=-25, p1=-15, p2=-5, p3=-1, floored at 0. Confirm score→label thresholds in `applyLabels` (review_job.go:507) and commit-status `MinScore` agree.
- [ ] `max_diff_lines` guard (review_job.go:174) — when exceeded, diff is dropped to nil. Confirm prompt notes truncation and review still posts something sane.
- [ ] `.pr-reviewer-ignore` filter + `filtered := diff[:0]` in-place reuse (review_job.go:164) — aliasing bug risk; verify original `prCtx.Diff` (used later for indexing) isn't corrupted.
- [ ] Stale-comment auto-resolve on `synchronize` (review_job.go:129) — only resolves comments whose path is NOT in changed paths. Re-read that condition; confirm it matches intent.
- [ ] Idempotency: same PR event delivered twice → duplicate reviews/comments? Check River job uniqueness + `Upsert`.
- [ ] Error classification: `isClientError` → `river.JobCancel` (no retry on 4xx). Verify 5xx/network errors DO retry.
- [ ] Token accounting: `input_tokens`/`output_tokens` type-asserted as `int` from metadata map — confirm adapters actually put `int` (not `float64`/`int64`), else silently zero.
- [ ] Agent fan-out is hardcoded to `["code-review","security"]` (reviewer.go:92) despite per-agent repo config — confirm `conversation` agent isn't meant here and config keys line up.

### Track C — Background jobs & queue
**Read:** `jobs/*`, River wiring in `main.go:160-262`.
- [ ] Worker retry/backoff, `MaxWorkers: 5`, queue depth gauge.
- [ ] Periodic jobs: team sync (6h), daily/weekly digest, weekly re-index — `RunOnStart:false`, cadence correctness, the "worker filters by period" comment (main.go:211).
- [ ] `conversation_job` — reply detection → re-review loop; infinite-loop guard (bot replying to itself?).
- [ ] `index_repo_job` / `IndexAllReposWorker` fan-out via enqueuer.
- [ ] Jobs running with `nil` optional deps (Indexer, NotifService) — nil-guards present?

### Track D — GitHub integration
**Read:** `github/client.go`, `github/app.go`, `github/codeowners.go`, `pr/service.go`.
- [ ] App installation token minting + caching/expiry; per-repo `InstallationID`.
- [ ] Pagination on diff/files/reviews fetch (large PRs).
- [ ] GraphQL/REST rate-limit handling, secondary-rate-limit backoff.
- [ ] CODEOWNERS parsing edge cases (globs, teams, comments, nested).

### Track E — Data layer
**Read:** `db/db.go` (AutoMigrate), `db/models/*`, `db/repo/*`.
- [ ] AutoMigrate vs the River migrations (both run at startup) — ordering, pgvector extension creation.
- [ ] Neon/PgBouncer: `QueryExecModeSimpleProtocol` set for pgx pool (main.go:111) but **gorm connection** uses a separate path — verify gorm also tolerates prepared-statement caching on Neon.
- [ ] N+1 queries on list endpoints (reviews, prs, audit) with `Preload`.
- [ ] Raw SQL in review_job.go:204 (false-positive patterns) + main.go queue query — injection-safe (they're static) but confirm no string interpolation elsewhere.
- [ ] Soft-delete / retention purge correctness (`runRetentionPurge`, `purgeOldDeliveries`).

### Track F — AI providers / RAG
**Read:** `ai/llm/registry.go`, `ai/llm/adapters/*`, `ai/rag/*`, `ai/embeddings/*`, provider loading in `main.go:526`.
- [ ] Adapter request/response mapping per provider; streaming vs non-streaming; timeout/cancel.
- [ ] Registry key scheme `db-{id}` and per-agent `provider_id` lookup — what happens when a referenced provider was deleted?
- [ ] pgvector retriever: dimension mismatch between embedder model and stored vectors; cosine vs L2; top-k=5.
- [ ] Embedder selection at startup (`buildEmbedderFromDB`) — first `supports_embeddings=true` provider; what if none?

### Track G — Notifications & integrations
**Read:** `notifications/*`, `integration/jira/*`, `integration/slack/*`.
- [ ] Slack slash command + events signature, response_url flow (review_job.go:357).
- [ ] Email digest period filtering; Resend API errors non-fatal.
- [ ] Jira ticket extraction regex from PR title/body; credential decryption.
- [ ] Notification config CRUD + `test` endpoints actually send.

### Track H — Frontend (web/)
**Read:** `web/lib/api.ts`, `web/lib/auth.ts`, `web/hooks/*`, `web/app/(dashboard)/**`, `web/middleware`/proxy.
- [ ] Token storage (localStorage vs cookie) and XSS exposure; `useToken`, `auth.ts`.
- [ ] API base URL / proxy (`proxy.ts`, `next.config.ts`) and CORS alignment.
- [ ] SSE hook (`useSSE.ts`) reconnect + token-in-URL.
- [ ] Every settings page round-trips to its API; error/loading states; optimistic updates.
- [ ] Accessibility / keyboard shortcuts / dark mode (per memory: Step 10 polish).

---

## 3. API endpoint test matrix

Public (no auth): `/webhooks`, `/health`, `/healthz`, `/metrics`, `/slack/commands`, `/slack/events`, `/api/events` (token query), `/auth/github*`, `/auth/oidc*`, `/api/setup/*`.

For **every** `/api/*` endpoint below, run the 4 baseline cases, then resource-specific cases:
1. **No token** → expect 401.
2. **Valid token** → expect 200/expected shape.
3. **Non-admin token on admin action** → expect 403 (⚠ likely the bug — see Track A).
4. **Malformed/oversized/empty body** → expect 400, not 500.

| Group | Endpoints | Notes / specific cases |
|---|---|---|
| Auth | `GET /api/auth/me`, `POST /api/auth/logout` | logout revokes session → subsequent calls 401 |
| Users (admin) | `GET /api/users`, `PATCH /{id}/role`, `/approve`, `/reject` | role escalation, self-demotion, approve in invite-only mode |
| Sessions | `GET /api/sessions`, `DELETE /{id}`, `DELETE /api/sessions` | revoke-all kills current session too? |
| Repos | `GET /api/repos`, `PATCH /{id}`, `GET/PUT /{id}/config`, `POST /sync`, `POST /{id}/index` | config JSON schema validation; index requires embedder |
| Reviews | `GET /api/reviews`, `GET /{id}`, `GET /export`(.csv), `GET /export.pdf` | pagination, export injection (CSV formula), large export |
| PRs | `GET /api/prs`, `GET /{owner}/{repo}/{number}`, `/diff`, `POST /re-review` | re-review enqueues job; path traversal in owner/repo |
| Dashboard | `GET /api/dashboard/stats` | empty-DB zero values |
| Team | `GET /api/team`, member CRUD, `GET/PUT /settings/team-sync`, `POST /sync/trigger` | sync trigger requires GitHub token |
| Assignments | `GET/POST /api/repos/{repo_id}/assignments/rules` | rule validation |
| Analytics | `GET /api/analytics`, `GET /api/analytics/cost` | date-range params, cost math |
| Providers | `GET/POST /api/providers`, `PUT/DELETE /{id}`, `POST /{id}/test`, `GET /health` | API key never echoed; test endpoint SSRF; delete in-use provider |
| GitHub App | `GET/PUT /api/settings/github-app`, `POST /test` | private key handling; secrets masked |
| Notifications | list/create/update/delete, `POST /{id}/test`, `POST /digest/trigger` | test actually sends |
| Feedback | `GET/POST /api/reviews/comments/{id}/feedback` | vote -1/+1 feeds false-positive learning |
| Explain | `POST /api/reviews/comments/{id}/explain` | LLM call; rate/cost |
| Audit (admin) | `GET /api/audit`, `GET /export` | who can read audit log? |
| Retention (admin) | `GET/PUT /api/settings/retention`, `DELETE /api/users/{login}/data` | GDPR erase is destructive — must be admin-gated |
| SSO (admin) | `GET/PUT/DELETE /api/settings/sso` | OIDC config secrets |
| Tokens | `GET/POST /api/tokens`, `DELETE /{id}` | raw token shown once; scope; expiry |
| Integrations | Jira get/put/delete/test, Slack-app get/put/delete/test | credential encryption; test SSRF |
| System | `GET /api/metrics/system`, `GET /api/webhooks/deliveries` | |

**Tooling:** capture a JWT from the browser (DevTools → Network → any `/api` call → `Authorization` header), then drive with `curl` or import the matrix into a Bruno/Hoppscotch/Postman collection. Example:
```bash
TOKEN="<paste from browser>"
curl -s localhost:8001/api/auth/me -H "Authorization: Bearer $TOKEN" | jq
curl -s localhost:8001/api/reviews -H "Authorization: Bearer $TOKEN" | jq
# the 401 baseline:
curl -s -o /dev/null -w '%{http_code}\n' localhost:8001/api/reviews
```

---

## 4. Manual functional testing (runbook)

### 4a. Run locally from scratch (clean DB)
```bash
# 1. Start Postgres+pgvector (start OrbStack/Docker first)
docker compose -f docker-compose.dev.yml up -d postgres
# 2. Backend with live reload (after `go install air`)
air            # or: go run ./cmd/server
# 3. Frontend
cd web && npm install && npm run dev
```
Then open http://localhost:3000.

> You already have a working instance against Neon. To test **destructive/admin** flows (retention erase, user reject, provider delete) use a **local docker Postgres**, not Neon, so you don't damage real data. Point `DATABASE_URL` at the local one in a scratch `.env`.

### 4b. First-run / setup flow
- [ ] `GET /api/setup/status` on empty DB → `setup_complete:false`.
- [ ] Complete setup via `/setup` page; confirm idempotency and `POST /api/setup/reset`.
- [ ] Configure an AI provider in Settings → Providers → **Test** (green).
- [ ] Configure GitHub App (App ID + private key) in Settings → GitHub App → **Test**.

### 4c. Auth flow
- [ ] GitHub OAuth login end-to-end; `REQUIRED_GITHUB_ORG` rejection (set it, log in with non-member).
- [ ] `INVITE_ONLY=true` → new user lands `pending`; admin approves → access granted.
- [ ] OIDC SSO login (if configured).
- [ ] Logout → token rejected; revoke session from another device.

### 4d. End-to-end PR review (the headline test) — needs ngrok
```bash
ngrok http 8001
# GitHub repo/App webhook → Payload URL: https://<id>.ngrok-free.app/webhooks
# Content-Type: application/json, Secret: matches GITHUB_WEBHOOK_SECRET (stored in DB), event: Pull requests
```
- [ ] Open a PR in a test repo → within seconds a review appears with inline P0–P3 comments + a summary comment + (if enabled) a commit status.
- [ ] Push a fix → on `synchronize`, stale bot comments get the "✅ addressed" reply.
- [ ] Add `.pr-reviewer-ignore` (e.g. `*.md`) → those files excluded.
- [ ] Add `.pr-reviewer.yml` custom rules → violations surface.
- [ ] Open a huge PR (> `max_diff_lines`, default 3000) → truncation noted, no crash.
- [ ] Reference a Jira ticket in the PR body → Jira context pulled in (if configured).
- [ ] Thumbs-down a comment twice (feedback API) → that pattern suppressed on next review (false-positive learning).
- [ ] Re-review via `POST /api/prs/{owner}/{repo}/{number}/re-review` and via UI.
- [ ] `/review` Slack slash command → result posts back to Slack.

**Replay webhooks without GitHub** (faster iteration): grab a delivery payload and POST it with a correct signature:
```bash
BODY='{"action":"opened", ...}'                       # a real pull_request payload
SIG="sha256=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$GITHUB_WEBHOOK_SECRET" | awk '{print $2}')"
curl -s -X POST localhost:8001/webhooks \
  -H "X-GitHub-Event: pull_request" \
  -H "X-Hub-Signature-256: $SIG" \
  -H "Content-Type: application/json" -d "$BODY"
```
Verify in `GET /api/webhooks/deliveries` and in the River `river_job` table.

### 4e. Observability
- [ ] `/metrics` exposes `review_queue_depth`, `review_duration_*`, provider health.
- [ ] SSE: open dashboard, trigger a review, watch `review_complete` event arrive live.
- [ ] Set `OTEL_EXPORTER_OTLP_ENDPOINT` → traces emitted (`review.job` span).

---

## 5. Automated tests — current state & gaps

**Existing (run these now):**
```bash
go test -race ./...                  # unit + handler tests
DATABASE_URL=... make test-integration   # tagged integration (needs DB)
cd web && npx tsc --noEmit           # type-check
```

**Coverage is ~555 lines of test vs ~12k LOC.** Biggest gaps to add tests for (highest value first):
1. `ai/reviewer.go` — `parseAgentResponse`, consensus filter, `computeScore`, priority mapping. Pure functions, table-driven tests, no I/O. **Start here.**
2. `middleware/auth.go` — JWT valid/expired/wrong-alg/revoked-session, API-token path, (and assert RBAC once added).
3. `jobs/review_job.go` — pipeline with mocked GHClient + AIService; ignore filter, truncation, stale-comment resolve, label thresholds.
4. `db/crypto.go` — encrypt/decrypt round-trip, tamper detection, key-length errors.
5. `rules/` — `ShouldIgnore` glob matching, `.pr-reviewer.yml` parse + evaluate.
6. `github/codeowners.go` — parsing matrix.
7. `review/aggregator.go` — verdict selection.
8. Handler tests per resource using `httptest` (the matrix in §3 maps 1:1 to test cases).

Add a coverage gate: `go test -coverprofile=cover.out ./... && go tool cover -func=cover.out`.

---

## 6. Suggested order of attack

1. **Run the existing test suite** (§5) — know your baseline.
2. **Track A (auth/security)** review + the §3 baseline 401/403 cases — confirm or refute the RBAC-not-wired finding first.
3. **Track B (review pipeline)** review + write the reviewer.go unit tests (§5.1).
4. **§4d end-to-end PR review** against a throwaway repo via ngrok — proves the whole stack.
5. Work Tracks C→H, exercising each via §3 matrix + §4 manual flows as you go.
6. Backfill automated tests (§5) for every bug you find so it can't regress.

---

## 7. Open questions to resolve while reviewing
- Is `RequireRole` deliberately unused, or is RBAC genuinely missing on admin endpoints? (Track A — verify empirically.)
- Does the in-process handler path (no DB) still work, or is it dead code now that DB is required by `config.Validate`?
- Do gorm AutoMigrate and River migrations ever conflict on shared startup?
- Are provider API keys / GitHub token / webhook secret ever serialized back to any GET endpoint?
```
