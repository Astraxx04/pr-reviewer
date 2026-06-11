# PR Reviewer — VS Code Extension

Surface AI code-review findings inside VS Code and trigger reviews without leaving the editor.

## Features

- **PR Reviewer: Show Findings for a PR** — detects the GitHub repo from your `origin`
  remote, lists the repo's PRs, and renders the latest review's comments as diagnostics
  (squiggles + Problems panel), color-coded by priority (P0/P1 → error, P2 → warning, P3 → info).
- **PR Reviewer: Trigger Review** — queues a fresh review for a selected PR.
- **PR Reviewer: Clear Findings** — clears the diagnostics.

## Setup

1. Generate an API token in the PR Reviewer web UI: **Settings → API Tokens**.
2. In VS Code settings, set:
   - `prReviewer.serverUrl` — your server, e.g. `https://pr-reviewer.yourco.com`
   - `prReviewer.apiToken` — the `prt_...` token
3. Open the repository folder and run a command from the Command Palette (`Cmd/Ctrl+Shift+P`).

## Develop

```bash
npm install
npm run compile      # or: npm run watch
# Press F5 in VS Code to launch the Extension Development Host.
```

Auth uses the same long-lived API token scheme as the CLI. The extension only reads
findings and triggers reviews through the platform API; it never talks to GitHub directly.
