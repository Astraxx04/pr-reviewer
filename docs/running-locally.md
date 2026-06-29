# Running PR Reviewer Locally

This guide explains how to run the backend API server and the frontend dashboard on your local machine for development.

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.22+ | `brew install go` |
| Node.js | 22+ | `brew install node` |
| npm | 11+ | bundled with Node.js |
| PostgreSQL | 16 (with pgvector) | Docker (see below) or Neon cloud |

---

## 1. Clone and configure

```bash
git clone <repo-url>
cd pr-reviewer
```

Copy the environment template and fill in your secrets:

```bash
cp .env.example .env   # or create .env from scratch
```

Minimum required variables:

```env
# ── Server ────────────────────────────────────────────────────────────────────
SERVER_PORT=8001
APP_ENV=development

# ── Database ─────────────────────────────────────────────────────────────────
# Option A: Local Docker (see section 2)
DATABASE_URL=postgres://pr_reviewer:pr_reviewer@localhost:5432/pr_reviewer?sslmode=disable

# Option B: Neon cloud — paste your connection string here
# DATABASE_URL=postgresql://user:pass@host/dbname?sslmode=require

# ── GitHub OAuth App ─────────────────────────────────────────────────────────
# Create at: https://github.com/settings/developers → OAuth Apps → New OAuth App
#   Homepage URL:           http://localhost:3000
#   Authorization callback: http://localhost:8001/auth/github/callback
GITHUB_CLIENT_ID=your_client_id
GITHUB_CLIENT_SECRET=your_client_secret

# Note: the GitHub token used to post reviews and the webhook secret are NOT
# env vars — configure them in Settings → GitHub App after first login. They are
# stored encrypted in the database.

# ── Security ──────────────────────────────────────────────────────────────────
# Generate with: openssl rand -hex 32
JWT_SECRET=generate_a_64_char_hex_string
# Generate with: openssl rand -hex 32
ENCRYPTION_KEY=generate_another_64_char_hex_string

# ── Frontend ──────────────────────────────────────────────────────────────────
SERVER_URL=http://localhost:8001
FRONTEND_URL=http://localhost:3000
```

> **LLM & embedding providers are not env vars.** Add at least one AI provider
> (OpenAI, Anthropic, or Ollama) and, optionally, an embedding provider for RAG
> in **Settings → Providers** after first login. Keys are stored encrypted in the
> database.

---

## 2. Start PostgreSQL (local Docker)

Skip this if you are using a cloud database (Neon, Supabase, etc.).

```bash
docker run -d \
  --name pr-reviewer-postgres \
  -e POSTGRES_USER=pr_reviewer \
  -e POSTGRES_PASSWORD=pr_reviewer \
  -e POSTGRES_DB=pr_reviewer \
  -p 5432:5432 \
  pgvector/pgvector:pg16-alpine
```

Wait a few seconds for it to initialise, then verify:

```bash
docker exec -it pr-reviewer-postgres pg_isready -U pr_reviewer
# pr_reviewer-postgres:5432 - accepting connections
```

---

## 3. Start the backend

The server auto-migrates the database schema on startup (GORM + River migrations).

```bash
go run ./cmd/server
```

Or compile first for faster restarts:

```bash
go build -o bin/pr-reviewer ./cmd/server
./bin/pr-reviewer
```

Check it is healthy:

```bash
curl http://localhost:8001/health
# ok

curl http://localhost:8001/healthz
# {"status":"ok","db":"ok","providers":0,"timestamp":"..."}
```

> The server reads `.env` automatically via `godotenv`. You can also export variables manually — exported vars take precedence over the file.

---

## 4. Start the frontend

```bash
cd web
npm install    # first time only
npm run dev
```

Open **http://localhost:3000** in your browser.

The frontend expects the API at `http://localhost:8001`. Override with:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8001 npm run dev
```

---

## 5. One-command dev mode (recommended)

From the project root, `make up` starts the full Docker Compose dev stack
(Postgres + backend with live-reload + frontend + ngrok) detached:

```bash
make up        # start (detached)
make build     # first run / after dependency or Dockerfile.dev changes
make logs      # follow logs (make logs-app for just the server)
make down      # stop everything
```

---

## 6. Expose webhooks locally (to receive GitHub PR events)

GitHub needs a public HTTPS URL to deliver webhook payloads. Use **ngrok**:

```bash
brew install ngrok/ngrok/ngrok
ngrok http 8001
# Forwarding: https://xxxx-xxx.ngrok-free.app → localhost:8001
```

Set that URL in your GitHub App or repo webhook settings:

- **Payload URL**: `https://xxxx-xxx.ngrok-free.app/webhooks`
- **Content type**: `application/json`
- **Secret**: must match the webhook secret set in **Settings → GitHub App**
- **Events**: select *Pull requests*

Then update `.env`:

```env
SERVER_URL=https://xxxx-xxx.ngrok-free.app
```

And restart the backend.

---

## 7. First-time setup wizard

On your first visit to `http://localhost:3000` the app will detect that setup is incomplete and redirect you to the wizard. It walks you through:

1. Database connectivity check
2. GitHub App / OAuth configuration
3. Adding your first AI provider
4. Selecting repositories to watch

---

## 8. Docker Compose (all-in-one)

To run the full stack (Postgres + backend + frontend) in containers:

```bash
cp .env .env.compose   # review and adjust DATABASE_URL if needed
docker compose up --build
```

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| Backend API | http://localhost:8001 |
| Prometheus metrics | http://localhost:8001/metrics |

Stop everything:

```bash
docker compose down          # keep data
docker compose down -v       # also delete the postgres volume
```

---

## 9. Useful make targets

```
make up               start the full dev stack (postgres + backend + frontend + ngrok), detached
make build            clear caches, rebuild images, then start
make down             stop the dev stack (ARGS=-v also drops the postgres volume)
make logs             tail logs from all services (logs-app / logs-web / logs-postgres / logs-ngrok)
make shell            open a shell in a container (defaults to app; override with ser=web)
make test             run Go tests (race detector) + TypeScript type-check
make format           format Go + frontend source
make lint             golangci-lint (same as CI)
make migrate          apply pending migrations (also migrate-down / migrate-status / migrate-new)
make seed             seed the database with sample data
make hooks            install the git pre-commit hook
make help             print all targets
```

---

## 10. Environment variable reference

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SERVER_PORT` | no | `8001` | Port the API listens on |
| `DATABASE_URL` | yes | — | PostgreSQL connection string (pgvector required) |
| `GITHUB_CLIENT_ID` | yes | — | GitHub OAuth App client ID |
| `GITHUB_CLIENT_SECRET` | yes | — | GitHub OAuth App client secret |
| `JWT_SECRET` | yes | — | 64-char hex secret for signing JWTs |
| `ENCRYPTION_KEY` | yes | — | 64-char hex key for AES-256 encryption at rest |
| `SERVER_URL` | no | `http://localhost:8001` | Public base URL (used in OAuth callback) |
| `FRONTEND_URL` | no | `http://localhost:3000` | CORS allowed origin |
| `APP_ENV` | no | `development` | `development` or `production` |
| `REQUIRED_GITHUB_ORG` | no | — | Restrict login to members of this GitHub org |
| `INVITE_ONLY` | no | `false` | New users require admin approval |
| `JWT_TTL_HOURS` | no | `24` | JWT lifetime in hours |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | — | OpenTelemetry tracing: unset = off, `stdout` = print spans to terminal, or an OTLP/HTTP collector URL |

> The table above is the complete set of env vars. Everything else — the GitHub
> App token + webhook secret, AI/embedding providers, Slack/email notifications,
> Jira, and SSO — is configured in the Settings UI and stored encrypted in the
> database.
