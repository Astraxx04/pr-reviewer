# Trying the CLI (`prrev`)

A hands-on, step-by-step guide to building the CLI, signing in, and running your
first commands against a local server.

> **The CLI is a thin client over the HTTP API.** It does nothing on its own — it
> needs a running backend to talk to. So the flow is: start the backend → build
> the CLI → `auth login` (browser) → run commands.

> **Authentication is browser-only.** You sign in through GitHub in your browser;
> the CLI captures the resulting token and stores it in its config file. There is
> **no** token-paste login and **no** environment-variable login.

---

## 0. What you'll need

| Requirement | Why |
|-------------|-----|
| Go 1.25+ | to build the CLI and server |
| PostgreSQL 16 + pgvector | the backend's datastore |
| A running backend (`cmd/server`) | the CLI has no local mode |
| A GitHub OAuth App + a browser | the only way to authenticate |

If you just want to *see the commands* without a server, jump to
[Appendix A: explore without a server](#appendix-a-explore-the-cli-without-a-server).

---

## 1. Install the CLI (`prrev`)

Pick one:

**a) Homebrew (macOS):**

```bash
brew install Astraxx04/tap/prrev
```

**b) `go install`:**

```bash
go install github.com/Astraxx04/pr-reviewer/cmd/prrev@latest
```

**c) Release one-liner** — downloads the right prebuilt binary and installs
`prrev` (plus the server binaries) to `/usr/local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/Astraxx04/pr-reviewer/main/install.sh | sh
```

**d) Build from source (for development)** — from the project root:

```bash
go build -o bin/prrev ./cmd/prrev
sudo cp bin/prrev /usr/local/bin/    # optional: put it on your PATH
```

Verify it runs:

```bash
prrev --help
prrev --version
```

> If you didn't put it on your PATH, run it as `./bin/prrev` instead of `prrev`
> throughout this guide.

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
`ENCRYPTION_KEY`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `SERVER_PORT`,
`SERVER_URL`.

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

---

## 3. Sign in (browser OAuth — the only way)

Run `auth login`. The CLI prints a sign-in URL, opens your browser, you authorize
with GitHub, approve the consent screen, and the CLI captures the token
automatically over a localhost loopback and saves it to
`~/.config/pr-reviewer/config.json`. No copy-paste, no tokens, no env vars.

```bash
prrev auth login --server <SERVER_URL>
```

> ⚠️ **`--server` must match the server's `SERVER_URL`** — the address GitHub
> redirects back to — because the OAuth state cookie has to round-trip on a single
> domain. For plain local dev that's `http://localhost:8001`. **Behind a tunnel
> (ngrok) or a reverse proxy, use the public URL**, e.g.
> `--server https://your-tunnel.ngrok-free.dev`. Pointing at `localhost` while
> `SERVER_URL` is the tunnel causes an `invalid state` error (see Troubleshooting).

What happens:

1. The terminal prints the sign-in URL and tries to open your browser:
   ```
   To sign in with GitHub, open this URL in your browser:

       <SERVER_URL>/auth/github?cli_redirect=http://127.0.0.1:54321/callback

   Trying to open it automatically... opened.
   Waiting for authentication to complete (Ctrl-C to cancel)...
   ```
2. You authorize with GitHub (you'll only see GitHub's own login/consent if you're
   signed out or haven't authorized the app before).
3. **The app's consent screen** appears — *"Authorize sign-in — You're signing in
   as `<you>` — [Yes, continue]"*. Click **Yes, continue**.
4. The browser tab confirms success and the terminal prints, e.g.:
   ```
   Logged in to <SERVER_URL> as Astraxx04 (owner)
   ```

That's it — every later command works without any flags. Confirm with:

```bash
prrev auth whoami
```

### Good to know

- **The consent screen shows on *every* login**, even when GitHub silently
  re-approves an already-authorized app — it's the app's own explicit "yes, this
  is me" step. Nothing is created until you click it.
- **Reloading the consent page is safe** (it's valid for 5 minutes). Do **not**
  reload the GitHub *callback* URL — that's single-use and a refresh will fail with
  `invalid state`. The flow redirects you off it automatically.
- **CLI tokens last 7 days.** When yours expires, the next command tells you
  exactly what to do:
  ```
  Error: session expired — run: prrev auth login
  ```
  Just `auth login` again.
- **Security:** the server only ever returns the token to a **localhost** address
  (`127.0.0.1`/`localhost`/`::1`). A crafted redirect to any other host is
  rejected, so the token can't be exfiltrated off your machine.
- **No browser? (CI / headless / SSH)** This flow needs a browser. For headless
  automation, use an API token via the HTTP API directly (next section) rather than
  this CLI.

---

## 4. (Optional) API tokens for external tools

Need a long-lived credential for **CI or other programmatic clients** that call
the API directly? Mint an API token (prefix `prt_`). This is **not** used to log in
this CLI — it's for sending `Authorization: Bearer <token>` to the API from your
own scripts. Token management is admin-only (`owner`/`admin` role):

```bash
# You must be logged in (section 3) first.
prrev tokens create --name "ci-pipeline" --scope readwrite
```

The raw `prt_…` token is printed **once** — copy it and store it securely. Use it
in your own HTTP calls, e.g.:

```bash
curl -H "Authorization: Bearer prt_xxx" <SERVER_URL>/api/reviews
```

Manage them with `tokens list` and `tokens revoke <id>`.

---

## 5. Run your first commands

Once logged in, no flags are needed:

```bash
# Who am I?
prrev auth whoami

# Repositories the server is tracking
prrev repos list

# Pull requests and their review history
prrev prs list
prrev prs get demo-org/api-service#42

# Reviews
prrev reviews list
prrev reviews get 1

# Aggregate stats
prrev dashboard stats

# Configured AI providers + health
prrev providers list
prrev providers health
```

Trigger an actual AI re-review of a PR (needs a configured provider + GitHub token
in **Settings → Providers / GitHub App**):

```bash
prrev prs re-review demo-org/api-service#42
```

---

## 6. Handy global flags

| Flag | Effect |
|------|--------|
| `--json` | Raw JSON instead of formatted tables (great for piping to `jq`) |
| `--server <url>` | Server URL; on `auth login` it's saved to the config file |
| `--timeout 60s` | Bump the HTTP timeout for slow operations |
| `--config <path>` | Use a different config file |

Example — export reviews to CSV and inspect JSON:

```bash
prrev reviews export --out reviews.csv
prrev repos list --json | jq '.[].full_name'
```

---

## Command reference

| Group | Command | Description |
|-------|---------|-------------|
| **auth** | `login` | sign in via the browser (GitHub OAuth) |
| | `whoami` / `logout` | show identity / revoke session and clear stored token |
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
| **tokens** | `list` / `create` / `revoke <id>` | manage API tokens for external clients (admin) |

Get help on any command with `--help`, e.g.:

```bash
prrev prs re-review --help
```

---

## Troubleshooting

| Symptom | Cause / Fix |
|---------|-------------|
| `not authenticated: run prrev auth login` | No token in the config file — run `auth login`. |
| `session expired — run: prrev auth login` | Your 7-day CLI token expired or was revoked — log in again. |
| `invalid state` (in the browser) | You reloaded the single-use GitHub *callback* URL, **or** `--server` didn't match the server's `SERVER_URL` (cookie domain mismatch). Use the `SERVER_URL` host and don't refresh the callback. |
| `invalid or expired login request` (consent page) | The 5-minute pre-auth window lapsed — run `auth login` again. |
| ngrok "You are about to visit…" interstitial | Click **Visit Site**; the flow continues. |
| `server returned 403` | Your user lacks the role for that action (e.g. `tokens` is admin-only). |
| Connection refused | Backend not running, or wrong port — it's **8001**, not 8080. |
| Empty lists everywhere | No data yet — run `make seed`, or connect a real repo + open a PR. |

---

## Appendix A: explore the CLI without a server

You can browse the entire command tree offline — only commands that hit the API
need a backend:

```bash
go run ./cmd/prrev --help
go run ./cmd/prrev auth login --help
go run ./cmd/prrev prs --help
go run ./cmd/prrev reviews --help
```

This is the fastest way to see what's available before standing up Postgres and
the server.
