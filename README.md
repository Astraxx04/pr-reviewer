# AI-Powered PR Reviewer

A **self-hosted, AI-driven Pull Request reviewer** written in Go. It listens to GitHub
webhooks, builds rich context from each PR (diffs + RAG over your codebase), and orchestrates
multiple specialized AI agents to post high-quality, actionable code reviews — all running on
**your own infrastructure** with **your own LLM keys**.

Bring your own provider (Anthropic, OpenAI, Ollama, or any OpenAI-compatible endpoint), keep
your code and secrets on your infra, and manage everything from a web dashboard, a CLI, or a
VS Code extension.

---

## Features

- **Multi-Agent Reviews** — Specialized agents for code quality, security, performance, and
  database changes run against each PR and their findings are aggregated into a single review.
- **Bring Your Own LLM** — Pluggable provider registry with adapters for **Anthropic**,
  **OpenAI**, **Ollama**, and any **OpenAI-compatible** API. Configured per-deployment in the UI.
- **RAG-Aware Context** — Retrieves relevant code from the repository (pgvector embeddings) so
  reviews understand the surrounding codebase, not just the diff.
- **Self-Hosted & Private** — Runs entirely on your infrastructure. Provider keys, GitHub
  tokens, and webhook secrets are stored **encrypted at rest** (AES-256-GCM).
- **Event-Driven Pipeline** — GitHub webhooks enqueue background jobs (River on Postgres);
  workers build context, run the review, post to GitHub, persist results, and notify.
- **Multiple Surfaces** — Web dashboard (Next.js), a CLI for CI/automation, and a VS Code
  extension to view findings and trigger reviews from your editor.
- **Branch Protection** — Posts commit statuses so reviews can gate merges.
- **Notifications** — Slack (two-way bot + slash commands) and email digests (daily/weekly).
- **Custom Rules & Assignments** — Per-repo configuration, rule evaluation, and reviewer
  assignment logic.
- **Reports & Audit** — PDF report export and an audit trail of activity.
- **Observability** — Built-in Prometheus metrics and OpenTelemetry tracing.

---

## Architecture

```
                         ┌──────────────────────────┐
   GitHub ──webhook────▶ │  POST /webhooks          │
   Slack  ──command────▶ │  POST /slack/commands    │  (public; verified by signature)
   Slack  ──event──────▶ │  POST /slack/events      │
                         └─────────────┬────────────┘
                                       │ enqueue
   Browser / CLI / VS Code            ▼
   ──Bearer JWT / API token──▶  /api/*  →  River job queue (Postgres)  →  Workers
                               (auth                                      - ReviewWorker
                                middleware)                               - EmailDigestWorker
                                                                          - TeamSyncWorker, …
                                       │                                   │
                                       ▼                                   ▼
                                  Postgres (GORM + pgvector)       GitHub API / Slack / Email
```

A review flows as: **webhook → build PR context (diff + RAG) → orchestrate agents → aggregate
findings → post review + commit status to GitHub → persist → notify.**

### Project layout

| Path | Responsibility |
|------|----------------|
| `cmd/server` | Application entrypoint and job-worker registration |
| `cmd/prrev` | `prrev` — auth, repos, reviews, tokens, providers |
| `cmd/migrate` / `cmd/seed` | Database migrations and sample-data seeding |
| `internal/ai` | Agents, LLM provider registry/adapters, RAG, embeddings, MCP contracts |
| `internal/github` | GitHub API adapter (PRs, diffs, comments, commit status) |
| `internal/pr` | PR context building and domain models |
| `internal/http` | Routing, webhook handling, auth middleware |
| `internal/jobs` | Background workers (review, digests, sync) |
| `internal/notifications` | Slack and email channels |
| `internal/rules` / `internal/assignments` | Rule evaluation and reviewer assignment |
| `internal/db` | GORM models, migrations, encryption (`Encrypt`/`Decrypt`) |
| `internal/metrics` / `internal/telemetry` | Prometheus + OpenTelemetry |
| `web/` | Next.js dashboard |
| `vscode-extension/` | VS Code extension |
| `deploy/helm` | Helm chart for Kubernetes |

See [`docs/`](docs/) for detailed documentation — start with
[`docs/tech_doc.md`](docs/tech_doc.md) and [`docs/running-locally.md`](docs/running-locally.md).

---

## Getting Started

### Prerequisites

| Tool | Version |
|------|---------|
| Go | 1.25+ |
| Node.js | 22+ (for the web dashboard) |
| PostgreSQL | 16 with **pgvector** |
| Docker | for the local dev stack (recommended) |

### Quick start (Docker dev stack)

The fastest way to run the full stack (Postgres + backend live-reload + frontend watch):

```bash
git clone <repository-url>
cd pr-reviewer
cp .env.example .env   # then fill in the values (see below)
make dev               # starts postgres + backend + frontend
```

`make help` lists all available targets.

### Manual build & run

```bash
# Apply database migrations (app schema + River queue)
make migrate

# Build and run the server
go build -o pr-reviewer ./cmd/server
./pr-reviewer
```

> **macOS note:** if you hit a `dyld: missing LC_UUID` error, build with `CGO_ENABLED=0`:
> ```bash
> CGO_ENABLED=0 go build -o pr-reviewer ./cmd/server
> ```

### Configuration

Only **infrastructure-level** settings live in `.env` (database URL, JWT secret, encryption
key, GitHub OAuth app, server/frontend URLs). The **GitHub token + webhook secret, AI
providers, and notification channels** are configured in the **Settings UI** after first
login and stored **encrypted** in the database.

Minimum `.env`:

```env
SERVER_PORT=8001
APP_ENV=development

# Local Docker Postgres (or paste a cloud connection string)
DATABASE_URL=postgres://pr_reviewer:pr_reviewer@localhost:5432/pr_reviewer?sslmode=disable

# GitHub OAuth App — https://github.com/settings/developers
#   Homepage URL:           http://localhost:3000
#   Authorization callback: http://localhost:8001/auth/github/callback
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret

# Generate each with: openssl rand -hex 32
JWT_SECRET=generate_a_64_char_hex_string
ENCRYPTION_KEY=generate_another_64_char_hex_string
```

---

## Webhook Setup (Local Development)

To receive webhooks locally, expose your server with [ngrok](https://ngrok.com):

```bash
ngrok http 8001
```

Then in your GitHub repo, go to **Settings → Webhooks → Add webhook**:

- **Payload URL**: `YOUR_NGROK_URL/webhooks`
- **Content type**: `application/json`
- **Secret**: match the webhook secret you set in **Settings → GitHub App**
- **Events**: "Let me select individual events" → check **Pull requests**

---

## CLI

`prrev` is a command-line client for managing repos, reviews, tokens, and providers. It
**signs in through your browser** (GitHub OAuth) and stores the resulting token in
`~/.config/pr-reviewer/config.json` — there is no token-paste or env-var login.

```bash
# Install a release (also installs the server binaries):
curl -fsSL https://raw.githubusercontent.com/Astraxx04/pr-reviewer/main/install.sh | sh

# …or build from source:
go build -o bin/prrev ./cmd/prrev

# Sign in and go:
prrev auth login --server https://your-server
prrev whoami
```

See [`docs/trying-the-cli.md`](docs/trying-the-cli.md) for the full walkthrough. For
**CI/automation** (no browser), mint an API token with `prrev tokens create` and send it as
an `Authorization: Bearer` header to the API directly.

---

## VS Code Extension

The extension in [`vscode-extension/`](vscode-extension/) lets you view AI review findings for
the current branch's PR and trigger reviews without leaving the editor. See its
[README](vscode-extension/README.md) for setup.

---

## Deployment

- **Docker Compose** — `docker-compose.yml` runs Postgres, migrations, the Go backend, and the
  web frontend.
- **Kubernetes** — a Helm chart is provided under [`deploy/helm/pr-reviewer`](deploy/helm/pr-reviewer).

---

## Development

```bash
make test     # Go tests (race detector) + frontend type-check
make lint     # golangci-lint (same version as CI)
make format   # gofmt + prettier
make hooks    # install the pre-commit hook
```

Database migrations:

```bash
make migrate            # apply pending migrations
make migrate-down       # roll back the latest
make migrate-status     # show current vs latest version
make migrate-new name=add_foo   # scaffold a new migration pair
make seed               # seed sample data
```

---

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
