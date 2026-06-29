# VS Code Extension

The extension (`vscode-extension/`) surfaces AI review findings as inline diagnostics and lets you trigger fresh reviews without leaving the editor.

---

## How it works

### Three commands (Command Palette: `Cmd/Ctrl+Shift+P`)

| Command | What it does |
|---------|-------------|
| `PR Reviewer: Show Findings for a PR` | Lists all PRs for the current repo, fetches the latest review's comments, and renders them as squiggles in the editor + entries in the **Problems** panel |
| `PR Reviewer: Trigger Review` | Queues a fresh AI review for a selected PR on the server |
| `PR Reviewer: Clear Findings` | Removes all squiggles/diagnostics |

### Data flow

```
VS Code command
    │
    ▼
git.ts: detectRepo()
    reads `git remote get-url origin`
    parses owner/repo from SSH or HTTPS URL
    │
    ▼
api.ts: Client
    GET /api/prs?repo=owner/repo          → pick a PR (QuickPick)
    GET /api/prs/{owner}/{repo}/{number}  → fetch findings
    POST /api/prs/{owner}/{repo}/{number}/re-review  → trigger
    │  (all requests send Authorization: Bearer prt_...)
    │
    ▼
extension.ts: renderDiagnostics()
    maps each PRComment { path, line, priority, body }
    → vscode.Diagnostic with severity:
        P0/P1 → Error (red squiggle)
        P2    → Warning (yellow squiggle)
        P3+   → Information (blue squiggle)
    attaches to the file URI inside the workspace folder
```

The extension **never talks to GitHub directly** — all data flows through your PR Reviewer server using the same long-lived API token (`prt_...`) as the CLI.

---

## Setup

### 1. Generate an API token

In the PR Reviewer web UI: **Settings → API Tokens → Create**.  
The token starts with `prt_`.

### 2. Configure VS Code settings

Open **Settings** (`Cmd+,`) and search for `prReviewer`, or add to your `settings.json`:

```json
{
  "prReviewer.serverUrl": "https://pr-reviewer.yourco.com",
  "prReviewer.apiToken": "prt_xxxxxxxxxxxxxxxx"
}
```

| Setting | Description |
|---------|-------------|
| `prReviewer.serverUrl` | Base URL of your self-hosted PR Reviewer server (no trailing slash) |
| `prReviewer.apiToken` | API token generated in the Settings UI |

### 3. Open your repo and run a command

- Open the repository folder in VS Code (the folder must have a `.git/` directory with a GitHub `origin` remote).
- Open the Command Palette (`Cmd/Ctrl+Shift+P`) and run one of the `PR Reviewer: …` commands.

---

## Development

```bash
cd vscode-extension
npm install
npm run compile      # one-shot TypeScript build
# or
npm run watch        # incremental watch mode
```

Then press **F5** in VS Code to launch the **Extension Development Host** — a fresh VS Code window with the extension loaded.

### Source layout

| File | Responsibility |
|------|---------------|
| `src/extension.ts` | Activation, command registration, diagnostic rendering |
| `src/api.ts` | `Client` class — typed wrappers for every API endpoint |
| `src/git.ts` | `detectRepo()` — parses GitHub owner/repo from the `origin` remote |
| `package.json` | Extension manifest: commands, configuration schema, VS Code engine version |
| `tsconfig.json` | TypeScript config targeting CommonJS for the VS Code host |

### To add a new command

1. Register it in `package.json` under `contributes.commands`.
2. Add `vscode.commands.registerCommand("prReviewer.yourCommand", ...)` in `extension.ts`.
3. Add any new API calls to `src/api.ts`.

### Publishing

```bash
npm install -g @vscode/vsce
vsce package          # produces pr-reviewer-vscode-0.1.0.vsix
vsce publish          # requires a publisher PAT
```

Or install the `.vsix` locally: **Extensions → … → Install from VSIX**.
