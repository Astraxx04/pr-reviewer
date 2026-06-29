# Project Documentation - AI-Powered PR Reviewer

This document provides a detailed overview of the system architecture, module responsibilities, and file descriptions for the AI-Powered PR Reviewer.
The system is designed to automatically review GitHub Pull Requests using LLMs, providing architectural, security, and performance feedback.

## System Architecture

The following diagram illustrates the data flow from a GitHub Webhook event through the system components to the final feedback generation.

```mermaid
graph TD
    User([GitHub User]) -->|Opens/Updates PR| GH[GitHub]
    GH -->|Webhook (JSON)| HTTP[HTTP Layer]
    
    subgraph "Core Application"
        HTTP -->|Event| Handler[Webhook Handler]
        Handler -->|1. Build Context| PRService[PR Service]
        PRService -->|Fetch PR & Diff| GHClient[GitHub Client]
        GHClient -->|GitHub API| GH
        
        Handler -->|2. Request Review| AI[AI Service]
        AI -->|Context| RAG[RAG Retriever]
        AI -->|Context| Agents[Agent Orchestrator]
        
        subgraph "AI Core"
            Agents -->|Dispatch| MCP[MCP Contract]
            MCP -->|Query| Agent1[Code Review Agent]
            MCP -->|Query| Agent2[Security Agent]
        end
        
        AI -->|Results| Aggregator[Review Aggregator]
        Aggregator -->|Final Report| Handler
    end
    
    Handler -->|3. Post Comment| GHClient
```

## Module & File Reference

### **`/cmd/server`**
The entry point of the application.
- **`main.go`**: Initializes the application.
    - Loads configuration via `internal/config`.
    - Initializes the structured logger.
    - Sets up the `GitHubToken` and `GitHubWebhookSecret`.
    - Initializes service dependencies: `PRService`, `AIService` (with Orchestrator), and `Aggregator`.
    - Registers HTTP handlers and starts the server on the configured port.
    - Handles graceful shutdown on implementation signals (SIGINT, SIGTERM).

### **`/internal/config`**
Handles application configuration management.
- **`config.go`**: Defines the `Config` struct (ServerPort, DatabaseURL, JWTSecret, EncryptionKey, ServerURL, FrontendURL, AppEnv, MigrateOnly, SkipMigrations, and access-control fields). GitHub credentials and LLM provider keys are **not** in env — they are loaded from the database at startup.
    - `Load()`: Reads from environment variables (checking `.env` first) and returns a populated `Config` object.
    - `Validate()`: Fails fast if required vars (`DATABASE_URL`, `ENCRYPTION_KEY`) are missing, or if `JWT_SECRET` is left at its default outside development.

### **`/internal/http`**
The presentation layer for incoming HTTP requests.
- **`router.go`**: Sets up the HTTP request multiplexer, routing paths (e.g., `/webhooks`) to handlers.
- **`webhook_handler.go`**: processing of GitHub webhooks.
    - `Handle()`: Validates the request signature using HMAC SHA256 and the webhook secret.
    - Parses the webhook payload to identify PR actions (`opened`, `synchronize`).
    - Asynchronously triggers the review process:
        1.  Builds PR context via `PRService`.
        2.  Requests an AI review via `AIService`.
        3.  Logs the result (posting to GitHub is pending implementation).

### **`/internal/pr`**
Domain logic for Pull Requests.
- **`service.go`**: Core service for retrieving PR data.
    - `BuildContext()`: Fetches PR metadata and diffs from GitHub to create a `PRContext`.
- **`models.go`**: Defines internal data structures like `PRContext`.

### **`/internal/github`**
Adapter layer for the GitHub API.
- **`client.go`**: Implements the `Client` interface for GitHub interactions.
    - encapsulates HTTP calls to GitHub API using a personal access token.
- **`models.go`**: Structs mirroring GitHub API responses (`PullRequest`, `FileDiff`, etc.).

### **`/internal/ai`**
The core AI orchestration layer, designed to be modular and agent-based.
- **`reviewer.go`**: Implements the `Service` interface.
    - orchestrates the review process by combining RAG retrieval (future) and Agent dispatch.
- **`agent.go`**: `AgentOrchestrator` manages multiple agents.
    - `RegisterAgent`: Adds an agent to the registry.
    - `Dispatch`: Routes a request to a specific agent by name.
- **`prompt.go`**: Contains `PromptTemplate` definitions for System and Review prompts, with capabilities for template rendering.
- **`models.go`**: Defines `AnalysisRequest` (inputs) and `ReviewResult` (outputs including score and comments).
- **`embeddings/`**: Directory for vector embedding logic (interfaces defined).
- **`rag/`**: Directory for RAG retrieval logic (interfaces defined).
- **`mcp/`**: Model Context Protocol implementation.
    - **`contract.go`**: Defines standard structures (`Request`, `Response`, `ToolCall`) for agent communication, ensuring a consistent interface between the core system and various specialized agents.

### **`/internal/review`**
Logic for post-processing AI outputs.
- **`aggregator.go`**: Aggregates and filters review results.
    - `NewAggregator()`: Creates a new aggregator instance.
- **`models.go`**: Defines `FinalReview` structure.

### **`/pkg/logger`**
Shared logging utility.
- **`logger.go`**: Provides a structured logger configuration (likely wrapping `log/slog`).

## Configuration

The application is configured via environment variables:
- `SERVER_PORT`: Port to listen on (binary default: 8001; the Docker and local dev setups override this to 8001).
- `APP_ENV`: Application environment (development/production).
- `DATABASE_URL`: Postgres connection string (loaded from `.env`, but an existing shell value takes precedence).
- `MIGRATE_ONLY`: When `true`, the server binary applies migrations and exits instead of serving. Used by the docker-compose `migrate` one-shot service.
- `SKIP_MIGRATIONS`: When `true`, the `MIGRATE_ONLY` run is a no-op that exits 0 without applying migrations (lets dependent services start without re-migrating). Default `false`.

The GitHub token used for API access, the webhook secret used to verify webhook
signatures, and the LLM provider keys are not env vars — they are configured in
the Settings UI after first login and stored encrypted in the database.

## Database Migrations & Docker

Migrations are an **explicit step**, not run on server boot. The server verifies
the schema is current at startup and **fails fast** if it is behind, rather than
running against a stale schema (`cmd/server/main.go`, `db.VerifySchema`).

There are two migration sets, both applied together:
- **App schema** — versioned SQL files in `internal/db/migrations`, embedded and
  applied by golang-migrate.
- **River queue** — the job-queue tables, migrated separately by `rivermigrate`.

### How to migrate

- **Host (no Docker):** `make migrate` runs `go run ./cmd/migrate up` against the
  `DATABASE_URL` resolved from your shell/`.env`. Related: `make migrate-down`,
  `make migrate-status`, `make migrate-new`. Recovery from a `DIRTY` state:
  `go run ./cmd/migrate force <version>` then `make migrate`.
- **Docker (both compose files):** a one-shot **`migrate` service** runs the
  server binary with `MIGRATE_ONLY=true`, applies both migration sets, then exits.
  The `app` service waits on it via `depends_on: { migrate: service_completed_successfully }`
  before starting. The migrate/app services override `DATABASE_URL` to the
  in-network `postgres` service, so they target the **Dockerized** Postgres
  regardless of the `.env` value.

### Compose files

| File | Started by | Purpose | Migrate service |
|---|---|---|---|
| `docker-compose.yml` | `docker compose up -d --build` | Production-like (built binary) | ✅ |
| `docker-compose.dev.yml` | `make up` / `make build` | Hot-reload dev (`air`) + `ngrok` | ✅ |

### Skipping migrations

Set `SKIP_MIGRATIONS=true` (in `.env` or inline, e.g. `SKIP_MIGRATIONS=true make up`)
to make the `migrate` service a no-op that still exits 0 — `app` then starts
without re-applying migrations. The flag lives in the binary (honored inside the
`MIGRATE_ONLY` block), so it behaves identically for dev and prod. Note: if the
schema is actually behind while migrations are skipped, `app` will still fail-fast
on its startup schema check by design.
