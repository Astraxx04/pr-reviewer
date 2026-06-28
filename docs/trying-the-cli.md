# Trying the CLI (`pr-reviewer-cli`)

A hands-on, step-by-step guide to building the CLI, authenticating, and running
your first commands against a local server.

> **The CLI is a thin client over the HTTP API.** It does nothing on its own —
> it needs a running backend to talk to. So the flow is: start the backend →
> build the CLI → get a token → point the CLI at the server.

---

## 0. What you'll need

| Requirement | Why |
|-------------|-----|
| Go 1.25+ | to build the CLI and server |
| PostgreSQL 16 + pgvector | the backend's datastore |
| A running backend (`cmd/server`) | the CLI has no local mode |
| A bearer token | every API call is authenticated |

If you just want to *see the commands* without a server, jump to
[Appendix A: explore without a server](#appendix-a-explore-the-cli-without-a-server).

---

## 1. Build the CLI

From the project root:

```bash
go build -o bin/pr-reviewer-cli ./cmd/cli
```

Verify it runs:

```bash
./bin/pr-reviewer-cli --help
./bin/pr-reviewer-cli --version
```

> Optional: put it on your PATH so you can type `pr-reviewer-cli` anywhere:
> ```bash
> sudo cp bin/pr-reviewer-cli /usr/local/bin/
> ```
> The rest of this guide assumes `./bin/pr-reviewer-cli`.

---

## 2. Start the backend

The CLI talks to the server defined in `.env` (`SERVER_PORT=8001` in this repo).

**a) Start Postgres** (skip if you already have one):

```bash
docker run -d \
  --name pr-reviewer-postgres \
  -e POSTGRES_USER=pr_reviewer \
  -e POSTGRES_PASSWORD=pr_reviewer \
  -e POSTGRES_DB=pr_reviewer \
  -p 5432:5432 \
  pgvector/pgvector:pg16-alpine
```

**b) Make sure `.env` has the required secrets** (see `docs/running-locally.md`
for the full list). Minimum for the CLI to work: `DATABASE_URL`, `JWT_SECRET`,
`ENCRYPTION_KEY`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `SERVER_PORT`.

**c) Run the server** (it auto-migrates the schema on startup):

```bash
go run ./cmd/server
```

**d) (Recommended) Seed demo data** so `reviews list`, `prs list`, etc. actually
return something:

```bash
make seed
```

**e) Confirm it's up:**

```bash
curl http://localhost:8001/health     # -> ok
curl http://localhost:8001/healthz    # -> {"status":"ok","db":"ok",...}
```

> ⚠️ **Port gotcha:** the CLI's built-in default server is `http://localhost:8080`,
> but this repo runs on **8001**. Always pass `--server http://localhost:8001`
> (or save it once with `auth login` — see step 4).

---

## 3. Get a token

Every CLI command sends `Authorization: Bearer <token>`. There are two kinds of
token, and you bootstrap the second from the first.

### 3a. Get a JWT by logging in via the browser

The backend authenticates humans through **GitHub OAuth**, and hands the JWT to
the frontend. Easiest path:

1. Start the frontend too (`cd web && npm install && npm run dev`) and open
   <http://localhost:3000>.
2. Log in with GitHub. After the OAuth round-trip the backend redirects to
   `http://localhost:3000/auth/callback?token=<JWT>`.
3. Grab that JWT — either copy it straight from the URL bar during the redirect,
   or open DevTools → Application → Local Storage and copy the stored token.

This JWT works as a bearer token immediately (it expires after `JWT_TTL_HOURS`,
default 24h).

### 3b. (Recommended) Mint a long-lived API token

JWTs expire. For a durable token (prefix `prt_`), use the JWT once to create an
API token. **Note:** token management is admin-only, so your user must have the
`owner`/`admin` role.

```bash
./bin/pr-reviewer-cli tokens create \
  --server http://localhost:8001 \
  --token <JWT-from-step-3a> \
  --name "my-laptop" \
  --scope readwrite
```

The raw `prt_…` token is printed **once** — copy it now. Use it everywhere the
JWT was used below.

---

## 4. Log in (save server + token)

Persist the server URL and token to `~/.config/pr-reviewer/config.json` so you
don't have to pass `--server`/`--token` on every command:

```bash
./bin/pr-reviewer-cli auth login \
  --server http://localhost:8001 \
  --token <prt_or_JWT>
```

It verifies the token against `/api/auth/me` and prints, e.g.:

```
Logged in to http://localhost:8001 as alice (owner)
```

Confirm any time with:

```bash
./bin/pr-reviewer-cli auth whoami
```

> **Alternative to a config file:** set environment variables instead.
> ```bash
> export PR_REVIEWER_SERVER=http://localhost:8001
> export PR_REVIEWER_TOKEN=prt_xxx
> ```
> Precedence is: `--flag` > env var > config file > default.

---

## 5. Run your first commands

Once logged in, drop the `--server`/`--token` flags:

```bash
# Who am I?
./bin/pr-reviewer-cli auth whoami

# Repositories the server is tracking
./bin/pr-reviewer-cli repos list

# Pull requests and their review history
./bin/pr-reviewer-cli prs list
./bin/pr-reviewer-cli prs get demo-org/api-service#42

# Reviews
./bin/pr-reviewer-cli reviews list
./bin/pr-reviewer-cli reviews get 1

# Aggregate stats
./bin/pr-reviewer-cli dashboard stats

# Configured AI providers + health
./bin/pr-reviewer-cli providers list
./bin/pr-reviewer-cli providers health
```

Trigger an actual AI re-review of a PR (needs a configured provider + GitHub
token in **Settings → Providers / GitHub App**):

```bash
./bin/pr-reviewer-cli prs re-review demo-org/api-service#42
```

---

## 6. Handy global flags

| Flag | Effect |
|------|--------|
| `--json` | Raw JSON instead of formatted tables (great for piping to `jq`) |
| `--server <url>` | Override the server for one command |
| `--token <tok>` | Override the token for one command |
| `--timeout 60s` | Bump the HTTP timeout for slow operations |
| `--config <path>` | Use a different config file |

Example — export reviews to CSV and inspect JSON:

```bash
./bin/pr-reviewer-cli reviews export --out reviews.csv
./bin/pr-reviewer-cli repos list --json | jq '.[].full_name'
```

---

## Command reference

| Group | Command | Description |
|-------|---------|-------------|
| **auth** | `login` / `whoami` / `logout` | authenticate and inspect identity |
| **repos** | `list` | list tracked repositories |
| | `enable <id>` / `disable <id>` | toggle reviewing for a repo |
| | `sync` | sync repos from the GitHub App installation |
| | `index <id>` | trigger a full RAG re-index |
| **prs** | `list` | list pull requests |
| | `get <owner/repo#N>` | show a PR + its review history |
| | `diff <owner/repo#N>` | print the PR's unified diff (JSON) |
| | `re-review <owner/repo#N>` | trigger a re-review |
| **reviews** | `list` | list reviews (most recent first) |
| | `get <id>` | show one review with its comments |
| | `export` | export reviews as CSV |
| **providers** | `list` / `test <id>` / `health` | manage AI providers |
| **dashboard** | `stats` | review + repo summary statistics |
| **tokens** | `list` / `create` / `revoke <id>` | manage API tokens (admin) |

Get help on any command with `--help`, e.g.:

```bash
./bin/pr-reviewer-cli prs re-review --help
```

---

## Troubleshooting

| Symptom | Cause / Fix |
|---------|-------------|
| `not authenticated: run auth login...` | No token resolved. Pass `--token`, set `PR_REVIEWER_TOKEN`, or `auth login`. |
| `server returned 401` | Token expired (JWT) or revoked. Re-login or mint a fresh `prt_` token. |
| `server returned 403` | Your user lacks the role for that action (e.g. `tokens` is admin-only). |
| Connection refused | Backend not running, or wrong port — remember it's **8001**, not 8080. |
| Empty lists everywhere | No data yet — run `make seed`, or connect a real repo + open a PR. |
| `token verification failed` | Wrong `--server`, or token minted against a different `JWT_SECRET`. |

---

## Appendix A: explore the CLI without a server

You can browse the entire command tree offline — only commands that hit the API
need a backend:

```bash
go run ./cmd/cli --help
go run ./cmd/cli prs --help
go run ./cmd/cli reviews --help
go run ./cmd/cli tokens create --help
```

This is the fastest way to see what's available before standing up Postgres and
the server.
