# PR Reviewer — VS Code Extension

AI-powered pull request reviews surfaced directly in your editor — no context switching, no copy-pasting findings from a browser tab.

---

## Features

### Sidebar Panel
A dedicated **PR Reviewer** panel in the Activity Bar shows all open PRs for the current repo. Each PR expands to list its findings, color-coded by priority. Clicking any finding jumps straight to the file and line.

### Status Bar
The bottom status bar always shows the active PR's number and score (e.g. `#42 · 78/100 ⚠`), auto-matched to your current git branch. Click it to focus the sidebar.

### Inline Editor Diagnostics
Findings appear as squiggles in the editor and entries in the **Problems** panel (`Cmd+Shift+M`), color-coded by priority:
- 🔴 **P0 / P1** → Error (red squiggle)
- 🟡 **P2** → Warning (yellow squiggle)
- 🔵 **P3** → Info (blue squiggle)

### Gutter Icons & Overview Ruler
Priority icons appear in the editor left margin and as marks on the scrollbar minimap — so findings are visible even when you're not looking at the Problems panel.

### Hover Cards
Hover over any finding line for a rich tooltip showing the full finding body and its priority.

### PR Summary Webview
Click the preview icon on any PR in the sidebar to open a full summary tab: error/warning/info counts at a glance, and a complete sortable table of every finding with file locations.

### Auto-Refresh
The extension polls every 60 seconds and automatically matches your current git branch to the right PR — findings update without any manual action.

---

## Commands

All commands are available from the Command Palette (`Cmd/Ctrl+Shift+P`):

| Command | Description |
|---|---|
| `PR Reviewer: Show Findings for a PR` | Pick a PR from a list and load its findings |
| `PR Reviewer: Trigger Review` | Queue a fresh AI review for a selected PR |
| `PR Reviewer: Refresh` | Manually refresh the sidebar and findings |
| `PR Reviewer: Clear Findings` | Remove all squiggles and diagnostics |
| `PR Reviewer: Open PR Summary` | Open the full webview for a PR |

The sidebar also has **Refresh** and **Clear** buttons in its toolbar, and each PR row has inline **Open Summary** and **Trigger Review** icons on hover.

---

## Setup

**1. Generate an API token**

In the PR Reviewer web UI: **Settings → API Tokens → Create**. The token starts with `prt_`.

**2. Configure VS Code**

Open Settings (`Cmd+,`), search for `prReviewer`, and set:

| Setting | Description |
|---|---|
| `prReviewer.serverUrl` | Base URL of your PR Reviewer server, e.g. `https://pr-reviewer.yourco.com` |
| `prReviewer.apiToken` | Your `prt_...` API token |

Or add directly to `settings.json`:
```json
{
  "prReviewer.serverUrl": "https://pr-reviewer.yourco.com",
  "prReviewer.apiToken": "prt_xxxxxxxxxxxxxxxx"
}
```

**3. Open your repo**

Open any folder that has a `.git/` directory with a GitHub `origin` remote. The extension activates automatically and starts loading findings for the current branch's PR.

---

## Requirements

- VS Code 1.85+
- A running PR Reviewer server
- A git repository with a GitHub `origin` remote

---

## Development

### Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Node.js | 20 | Use `nvm use` — `.nvmrc` is included |
| npm | 10+ | Bundled with Node 20 |
| VS Code | 1.85+ | Required to run the Extension Development Host |

### Clone & install

```bash
git clone https://github.com/Astraxx04/pr-reviewer.git
cd pr-reviewer/vscode-extension

nvm use          # switches to Node 20 per .nvmrc
npm install
```

### Build

```bash
npm run compile   # one-shot TypeScript → out/
npm run watch     # incremental watch mode (run this during active development)
```

Output lands in `out/`. The `tsconfig.json` targets ES2021 CommonJS — do not change the module format, VS Code's extension host requires it.

### Run & Debug

Press **F5** in VS Code (with this folder open) to launch the **Extension Development Host** — a fresh VS Code window with the extension loaded from source. You can set breakpoints in any `src/` file and they will hit in the host window.

To test against a real server, configure `prReviewer.serverUrl` and `prReviewer.apiToken` in the Development Host's settings (`Cmd+,`).

### Source layout

| File | Responsibility |
|---|---|
| `src/extension.ts` | Activation, command registration, polling loop, surface orchestration |
| `src/api.ts` | Typed API client — all HTTP calls to the PR Reviewer server |
| `src/git.ts` | `detectRepo()` and `getCurrentBranch()` — reads the git `origin` remote |
| `src/statusBar.ts` | Bottom status bar item showing active PR score |
| `src/prTreeProvider.ts` | Sidebar tree data provider — PR list and findings tree |
| `src/decorations.ts` | Gutter icon decorations and overview ruler marks |
| `src/hoverProvider.ts` | Hover card shown when the cursor is on a finding line |
| `src/webviewPanel.ts` | Full HTML PR summary tab |
| `assets/logo.png` | Extension icon (shown in marketplace and extension list) |
| `assets/sidebar-icon.svg` | Activity Bar panel icon |
| `assets/error.svg` / `warning.svg` / `info.svg` | Gutter decoration icons |

### Adding a new command

1. Add the command entry to `package.json` under `contributes.commands` (and `contributes.menus` if it needs a toolbar or context menu slot).
2. Register it in `src/extension.ts` with `vscode.commands.registerCommand("prReviewer.yourCommand", ...)`.
3. Add any new API calls to `src/api.ts`.

### Auth & API

The extension authenticates to the PR Reviewer server using a long-lived API token (`prt_...`) sent as a `Bearer` header — the same scheme used by the CLI. All data flows through the server; the extension never calls the GitHub API directly.

The server URL and token are read from VS Code settings (`prReviewer.serverUrl`, `prReviewer.apiToken`) on every command invocation, so changes take effect immediately without reloading.

### Packaging

```bash
npx @vscode/vsce package --no-dependencies
# produces pr-reviewer-vscode-0.1.0.vsix
```

Install locally: Extensions sidebar → `⋯` → **Install from VSIX**.

To publish to the marketplace, a publisher PAT is required:
```bash
npx @vscode/vsce publish
```
