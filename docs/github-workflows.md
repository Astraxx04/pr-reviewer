# GitHub Workflows

This project uses two GitHub Actions workflows, both defined under `.github/workflows/`:

| File | Name | Triggers on | Purpose |
|------|------|-------------|---------|
| `ci.yml` | **CI** | push to `main`, any pull request | Lint, test, and build every change |
| `release.yml` | **Release** | push of a `v*` tag | Cut a release with GoReleaser (binaries + Docker images) |

---

## `ci.yml` — Continuous Integration

Runs on every push to `main` and on every pull request. It is made up of five independent jobs that run in parallel on `ubuntu-latest`.

### Triggers

```yaml
on:
  push:
    branches: [main]
  pull_request:
```

### Jobs

#### 1. `lint` — Go static analysis
- Checks out the repo and installs Go using the version pinned in `go.mod` (`go-version-file: go.mod`), with module caching enabled.
- Runs **golangci-lint** via `golangci/golangci-lint-action@v8`, pinned to **`v2.12.2`**.
- The action version (`v8`) and the linter version (`v2.x`) must match the `.golangci.yml` config format (`version: 2`). The pinned `v2.12.2` binary is built with Go 1.26, which must be **≥ the Go version targeted in `go.mod`** (`go 1.25.6`) — otherwise golangci-lint refuses to run.
- Enabled linters (from `.golangci.yml`): `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`.

#### 2. `test` — Unit tests
- Spins up a **PostgreSQL service container** (`pgvector/pgvector:pg16`) with a health check, exposed on port `5432`.
- Provides the DB connection string via the `DATABASE_URL` env var.
- Runs `go test -race -count=1 ./...` — the full test suite with the race detector and no test caching.

#### 3. `build` — Go binaries
- Compiles the two main binaries with `CGO_ENABLED=0`, discarding the output (`-o /dev/null`) — this is a compile check, not an artifact build:
  - `./cmd/server`
  - `./cmd/migrate`

#### 4. `build-web` — Frontend build
- Runs inside the `web/` directory (`defaults.run.working-directory: web`).
- Uses Node.js 22, then `npm ci` (installs from the lockfile; fails if `package-lock.json` is out of date).
- Runs `npm run build` to verify the Next.js app compiles.

> The Go integration tests (`go test -tags=integration ./internal/integration/...`) still live
> in the repo and can be run locally with a Postgres `DATABASE_URL`, but they are no longer run
> in CI.

---

## `release.yml` — Release

Builds and publishes a release whenever a version tag is pushed.

### Triggers

```yaml
on:
  push:
    tags:
      - "v*"
```

Any tag starting with `v` (e.g. `v1.2.3`) kicks off a release.

### Concurrency

```yaml
concurrency:
  group: release
  cancel-in-progress: false
```

All release runs share a single concurrency group, so **only one release runs at a time**. `cancel-in-progress: false` means an in-flight release is **never cancelled** by a newer one — a second tag pushed during a release waits its turn rather than interrupting a publish that's already partway through uploading assets.

### Permissions

```yaml
permissions:
  contents: write   # create the GitHub Release and upload assets
  packages: write   # push Docker images to GHCR
```

### Job: `release` (GoReleaser)

Runs on `ubuntu-latest`. Steps, in order:

1. **Checkout** with `fetch-depth: 0` — full git history and all tags are required so GoReleaser can build a changelog and resolve the tag.

2. **Verify tag is on main** — a guard step:
   ```bash
   git fetch origin main
   if ! git merge-base --is-ancestor "$GITHUB_SHA" origin/main; then
     echo "::error::Tag ... is not an ancestor of main. Refusing to release."
     exit 1
   fi
   ```
   Uses `git merge-base --is-ancestor` to confirm the tagged commit is reachable from `origin/main`. If a tag was accidentally pushed from a feature branch, the job fails fast and **no release is published**. (Assumes the release workflow expects all releases to originate from `main`.)

3. **Set up Go** from `go.mod`, with caching.

4. **Set up QEMU** (`docker/setup-qemu-action@v3`) — enables multi-architecture Docker builds via emulation.

5. **Set up Docker Buildx** (`docker/setup-buildx-action@v3`) — the builder backend for multi-arch images.

6. **Log in to GitHub Container Registry** (`docker/login-action@v3`) — authenticates to `ghcr.io` as `${{ github.actor }}` using the automatic `GITHUB_TOKEN`.

7. **Run GoReleaser** (`goreleaser/goreleaser-action@v6`):
   - `distribution: goreleaser`, `version: "~> v2"` (GoReleaser v2.x).
   - `args: release --clean` — builds the release and cleans the `dist/` directory first.
   - Env: `GITHUB_TOKEN`, `GITHUB_REPOSITORY_OWNER`, `GITHUB_REPOSITORY`.
   - The actual build/package/publish behavior (binaries, archives, Docker images, checksums, changelog) is defined in **`.goreleaser.yml`** at the repo root.

---

## How a change flows through the pipeline

1. **Open a PR** → `ci.yml` runs `lint`, `test`, `build`, and `build-web`.
2. **Merge to `main`** → the same four jobs run on `main`.
3. **Push a `v*` tag** (from a commit on `main`) → `release.yml` verifies the tag is on `main`, then runs GoReleaser to publish binaries and Docker images.

## Related files

- `.golangci.yml` — golangci-lint configuration (config format `version: 2`).
- `.goreleaser.yml` — GoReleaser configuration (what gets built and published).
- `go.mod` — the Go toolchain version, consumed by `setup-go` and by the lint Go-version check.
- `web/package.json` / `web/package-lock.json` — frontend dependencies used by `build-web`.
