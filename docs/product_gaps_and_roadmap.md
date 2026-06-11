# Product Gaps & Roadmap — AI-Powered PR Reviewer

## Current State (Honest Assessment)

The codebase is a well-structured skeleton. The architecture is sound, but **nothing actually works end-to-end yet**. The webhook receives events, fetches the PR diff from GitHub, then calls `Review()` which returns a hardcoded placeholder and never posts anything back to GitHub.

Key stubs / TODOs in the current code:

| File | What's missing |
|------|---------------|
| `internal/ai/reviewer.go:41` | LLM is never called — returns `"Automated review placeholder"` |
| `internal/ai/prompt.go:27` | `Render()` is a no-op, ignores the data map |
| `internal/review/aggregator.go:20` | Aggregation is stubbed, always returns `COMMENT` status |
| `internal/http/webhook_handler.go:132` | Result is only logged, never posted back to GitHub |
| `cmd/server/main.go:36` | No agents are ever registered with the orchestrator |
| `internal/ai/rag/` | Interface only, no implementation |
| `internal/ai/embeddings/` | Interface only, no implementation |

---

## Gap Analysis

### 1. Core Backend — AI Pipeline Not Wired Up

The most critical gap: there is no LLM call anywhere in the code.

**Missing pieces:**
- OpenAI/Anthropic SDK integration in `reviewerImpl`
- Prompt template `Render()` actually substituting `{{.Diff}}` and `{{.Context}}`
- At least one concrete `mcp.Agent` implementation (e.g. `CodeReviewAgent`, `SecurityAgent`)
- `AgentOrchestrator` receiving registered agents at startup
- RAG retriever backed by a vector DB (pgvector, Qdrant, Weaviate)
- Embedder implementation calling an embedding model
- `Aggregator.Aggregate()` deduplicating and ranking comments, computing the `APPROVE / REQUEST_CHANGES / COMMENT` status
- Posting the `FinalReview` back to GitHub via `PostComment` — the call is there in the client but never invoked
- Handling GitHub's "create review" (batched comments) vs single-comment endpoints correctly

### 2. Persistence Layer — Nothing Is Stored

There is no database. Every review is fire-and-forget. This blocks all product features downstream.

**Missing pieces:**
- Database schema (repositories, pull_requests, reviews, comments, users, teams)
- ORM or query layer (sqlc + PostgreSQL recommended given the Go stack)
- Repository registry: which repos are connected and by whom
- Review history: every run's output persisted for audit, replay, analytics
- Idempotency: webhook deduplication to avoid double-reviewing the same push event

### 3. Authentication & Multi-Tenancy

There is no concept of an authenticated user. The app currently works only for a single hard-coded GitHub token.

**Missing pieces:**
- GitHub OAuth App or GitHub App installation flow (App is better: scoped permissions, webhook management, no PAT)
- User accounts (sign-in with GitHub)
- Organisation model: a GitHub org maps to a tenant
- Role-based access: admin, reviewer, read-only
- API key / JWT middleware on all internal API routes
- Per-installation token management (GitHub App installation tokens expire every hour)

### 4. Async Job Queue

The current goroutine fire-and-forget (`go func()` in the webhook handler) is fragile — if the process restarts mid-review the job is silently lost.

**Missing pieces:**
- A proper job queue (Temporal, River, or simple Postgres-backed queue)
- Job status tracking (pending → processing → done / failed)
- Retry with backoff on LLM or GitHub API failures
- Dead-letter handling for permanently failed jobs
- Concurrency limits (prevent hammering the LLM API)

### 5. Assignment System

The product goal includes "assignments to people." There is currently no such concept.

**Missing pieces:**
- Reviewer assignment rules per repository (round-robin, load-balanced, CODEOWNERS-aware)
- AI confidence threshold: if score is below X, require a human reviewer
- Escalation rules: security findings always assign a specific team
- Assignment notification (GitHub review request, email, Slack)
- Assignment history and workload dashboard

### 6. Frontend Dashboard

There is no frontend at all.

**Recommended stack:** Next.js (App Router) + Tailwind CSS + shadcn/ui, deployed separately, consuming a REST or tRPC API from the Go backend.

**Pages / features needed:**

| Page | Description |
|------|-------------|
| `/login` | Sign in with GitHub OAuth |
| `/dashboard` | Summary metrics: PRs reviewed today/week, avg score, top issues |
| `/repos` | List of connected repositories with enable/disable toggle |
| `/repos/[owner]/[repo]` | Per-repo config: agent selection, severity thresholds, assignment rules |
| `/reviews` | Paginated list of all past reviews with status and score |
| `/reviews/[id]` | Full review detail: diff view with inline AI comments, final verdict |
| `/team` | Team members, roles, assignment stats, current queue depth |
| `/settings` | GitHub App connection, API keys, notification channels (email/Slack) |
| `/analytics` | Charts: review volume over time, common issue categories, score trends |

### 7. Configuration & Policy Engine

Right now all prompts are hardcoded. Teams need to customise behaviour.

**Missing pieces:**
- Per-repo config stored in DB (which agents to run, severity filters, language/framework hints)
- Optional `.pr-reviewer.yaml` in the repository root to override org defaults
- Agent enable/disable per repo (e.g. disable security agent on a docs-only repo)
- Severity levels: `info`, `warning`, `error` — with thresholds for blocking merge
- Custom prompt injection: teams can add domain-specific instructions

### 8. Observability & Ops

**Missing pieces:**
- Structured metrics (Prometheus / OpenTelemetry) — latency, LLM token usage, error rates
- Distributed tracing (each review should have a trace ID through all components)
- Alerting: LLM API errors, queue depth spikes, webhook failure rate
- Health check endpoint (`/healthz`) already referenced in router but not shown
- Cost tracking: tokens consumed per review, per repo, per user — critical for billing

### 9. GitHub App (vs Personal Access Token)

The current setup requires a PAT and a manually configured webhook per repo. This does not scale.

**GitHub App gives:**
- Webhook management via API (no manual setup)
- Installation-scoped tokens (no single user's token)
- Org-level installation (one click covers all repos in an org)
- Marketplace listing potential

---

## Suggested Architecture (Target State)

```
┌─────────────────────────────────────────────────────────┐
│                    Frontend (Next.js)                    │
│  Dashboard / Repo Config / Review Detail / Team / Auth  │
└───────────────────────┬─────────────────────────────────┘
                        │ REST / tRPC
┌───────────────────────▼─────────────────────────────────┐
│                   Go API Server                         │
│  /api/* (authed)  +  /webhooks (GitHub App)             │
│  Auth middleware (JWT / GitHub App JWT)                  │
└──────┬────────────────┬────────────────┬────────────────┘
       │                │                │
  PostgreSQL       Job Queue         GitHub API
  (reviews,       (Temporal /        (webhooks,
   users,          River)             comments)
   repos)              │
                  ┌────▼────────────────────┐
                  │    Review Worker Pool    │
                  │  ┌─────────────────────┐│
                  │  │  AI Reviewer        ││
                  │  │  ├─ RAG Retrieval   ││
                  │  │  ├─ Code Agent      ││
                  │  │  ├─ Security Agent  ││
                  │  │  └─ Aggregator      ││
                  │  └─────────────────────┘│
                  └─────────────────────────┘
                         │
                   LLM Provider
                (OpenAI / Anthropic)
                  + Vector DB
               (pgvector / Qdrant)
```

---

## Assignments

Roles used below:

- **BE** — Backend Engineer (Go)
- **AI** — AI / ML Engineer
- **FE** — Frontend Engineer
- **Infra** — DevOps / Infrastructure

---

### Track 1 — Make It Actually Work (Sprint 1–2)

| # | Task | Owner | Notes |
|---|------|-------|-------|
| 1.1 | Wire LLM call in `reviewerImpl.Review()` | **AI** | Start with OpenAI `gpt-4o`; use the existing `OpenAIKey` config field |
| 1.2 | Fix `PromptTemplate.Render()` — use `text/template` | **BE** | Replace the stub with proper template execution |
| 1.3 | Implement `CodeReviewAgent` (MCP agent) | **AI** | Concrete agent that calls the LLM with the code diff |
| 1.4 | Implement `SecurityAgent` (MCP agent) | **AI** | Focused prompt on OWASP top 10 + secrets detection |
| 1.5 | Register both agents in `main.go` | **BE** | Remove the `// TODO: Register agents` comment |
| 1.6 | Implement `Aggregator.Aggregate()` | **AI + BE** | Deduplicate comments, compute APPROVE/REQUEST_CHANGES threshold |
| 1.7 | Post `FinalReview` back to GitHub | **BE** | Call `ghClient.PostComment` (or upgrade to `CreateReview` for batched) |
| 1.8 | End-to-end smoke test on a real PR | **BE + AI** | Open a test PR on a private repo and verify comments appear |

---

### Track 2 — Persistence & Auth (Sprint 2–3)

| # | Task | Owner | Notes |
|---|------|-------|-------|
| 2.1 | Add PostgreSQL; define schema (repos, prs, reviews, users) | **BE** | Use `sqlc` + `golang-migrate` |
| 2.2 | Convert GitHub PAT flow to GitHub App | **BE** | Register a GitHub App, implement installation OAuth flow |
| 2.3 | User authentication (sign in with GitHub) | **BE** | Issue short-lived JWTs; store user + org in DB |
| 2.4 | Persist every review run and its comments | **BE** | Insert on completion; include token usage and latency |
| 2.5 | Webhook idempotency (deduplicate by delivery ID) | **BE** | GitHub sends `X-GitHub-Delivery` header; store and check |
| 2.6 | Migrate goroutine fire-and-forget to a job queue | **BE** | River (Postgres-backed) is the lowest-ops choice for this stack |

---

### Track 3 — Frontend (Sprint 3–5)

| # | Task | Owner | Notes |
|---|------|-------|-------|
| 3.1 | Scaffold Next.js app with Tailwind + shadcn/ui | **FE** | `create-next-app` in `/web`; configure API base URL via env |
| 3.2 | Login page (GitHub OAuth) | **FE** | Use NextAuth.js with GitHub provider |
| 3.3 | Dashboard: review volume, avg score, top issues | **FE** | Charts via Recharts or Tremor |
| 3.4 | Repo list + enable/disable toggle | **FE + BE** | BE needs `GET /api/repos` and `PATCH /api/repos/:id` |
| 3.5 | Review history list (paginated) | **FE + BE** | BE needs `GET /api/reviews?repo=&page=` |
| 3.6 | Review detail page with inline diff + comments | **FE** | Render the diff with `react-diff-viewer`; overlay AI comments |
| 3.7 | Team page: members, roles, workload | **FE + BE** | BE needs team CRUD endpoints |
| 3.8 | Settings: notification channels, API keys | **FE + BE** | Slack webhook URL, email address per team |

---

### Track 4 — Assignment System (Sprint 4–5)

| # | Task | Owner | Notes |
|---|------|-------|-------|
| 4.1 | Assignment rules model in DB | **BE** | Rules: round-robin, load-balanced, CODEOWNERS-based |
| 4.2 | Rule evaluation engine | **BE** | Evaluate rules after AI review completes |
| 4.3 | CODEOWNERS parsing | **BE** | Fetch `CODEOWNERS` via GitHub API; map changed files to owners |
| 4.4 | Create GitHub review request on assignment | **BE** | `PullRequests.RequestReviewers` API |
| 4.5 | Slack notification on assignment | **BE** | Post to configured Slack webhook with PR link + AI summary |
| 4.6 | Email notification on assignment | **BE** | Transactional email (Resend / Postmark) |
| 4.7 | Assignment UI in repo settings | **FE** | Drag-and-drop rule builder or simple form |
| 4.8 | Workload view: queue depth per reviewer | **FE** | Show open assignments per team member |

---

### Track 5 — RAG & Context (Sprint 5–6)

| # | Task | Owner | Notes |
|---|------|-------|-------|
| 5.1 | Embedder implementation (OpenAI `text-embedding-3-small`) | **AI** | Implement the existing `Embedder` interface |
| 5.2 | Vector DB setup (pgvector extension on existing Postgres) | **Infra** | Avoids a separate DB dependency |
| 5.3 | Codebase indexing job | **AI + BE** | On repo connect: clone, chunk, embed, store vectors |
| 5.4 | RAG retriever implementation | **AI** | Implement `Retriever`; cosine similarity top-K lookup |
| 5.5 | Wire retriever into `reviewerImpl` | **AI** | Use retrieved docs as additional context in the review prompt |
| 5.6 | Incremental re-indexing on push | **AI + BE** | Only re-embed changed files, not the whole repo |

---

### Track 6 — Infra & Ops (Sprint 2 onwards, parallel)

| # | Task | Owner | Notes |
|---|------|-------|-------|
| 6.1 | Dockerfile + docker-compose (app + postgres) | **Infra** | Dev environment should be `docker compose up` |
| 6.2 | GitHub Actions CI: build, lint (`golangci-lint`), test | **Infra** | Add a workflow file; tests are currently absent |
| 6.3 | OpenTelemetry tracing through all layers | **Infra + BE** | Trace ID per review; export to Jaeger / Honeycomb |
| 6.4 | Prometheus metrics endpoint `/metrics` | **Infra + BE** | LLM latency, token cost, queue depth, error rate |
| 6.5 | Health check endpoint `/healthz` | **BE** | Check DB + LLM API connectivity |
| 6.6 | Cost budget alerting | **Infra** | Alert when monthly LLM spend exceeds threshold |
| 6.7 | Production deployment (Fly.io or Railway for low-ops start) | **Infra** | Single-region to start; autoscale workers separately |

---

## Priority Order

If bandwidth is limited, do these first:

1. **Track 1** — without this, the product does not function at all.
2. **Track 2 (2.1–2.2)** — persistence and GitHub App are needed before any real user can onboard.
3. **Track 3 (3.1–3.6)** — users need a UI to see what the system is doing.
4. **Track 4 (4.1–4.4)** — assignment is the differentiating feature; Slack/email can come later.
5. **Track 5** — RAG improves review quality significantly but is not blocking.
6. **Track 6** — ops work can be threaded in alongside everything else.

---

## Quick Wins (Can Be Done This Week)

1. Fix `PromptTemplate.Render()` — 30 minutes, unblocks AI work.
2. Add a real OpenAI call in `reviewerImpl` — 2–3 hours, makes the app functional.
3. Post the review back to GitHub — 1 hour, closes the loop.
4. Add a `Dockerfile` and `docker-compose.yml` — 1 hour, removes the "works on my machine" problem.
5. Write a single integration test covering the webhook-to-comment path — 2–3 hours, prevents regressions as the codebase grows.
