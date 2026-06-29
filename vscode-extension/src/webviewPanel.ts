import * as vscode from "vscode";
import { Client, PRDetail, PRSummary } from "./api";
import { RepoRef } from "./git";

export async function openPRWebview(
  context: vscode.ExtensionContext,
  client: Client,
  repo: RepoRef,
  pr: PRSummary
): Promise<void> {
  const detail = await client.getPR(repo.owner, repo.repo, pr.number);
  const panel = vscode.window.createWebviewPanel(
    "prReviewer.detail",
    `PR #${detail.number}`,
    vscode.ViewColumn.Beside,
    { enableScripts: false, retainContextWhenHidden: true }
  );
  panel.webview.html = buildHtml(detail);
}

function buildHtml(detail: PRDetail): string {
  const comments = detail.latest_comments ?? [];
  const p01 = comments.filter((c) => ["p0", "p1"].includes((c.priority || "").toLowerCase())).length;
  const p2 = comments.filter((c) => (c.priority || "").toLowerCase() === "p2").length;
  const p3 = comments.length - p01 - p2;

  const rows = comments
    .map(
      (c) => `<tr>
      <td><span class="badge badge-${(c.priority || "p3").toLowerCase()}">${escHtml(c.priority ?? "—")}</span></td>
      <td class="loc">${escHtml(c.path)}:${c.line}</td>
      <td>${escHtml(c.body)}</td>
    </tr>`
    )
    .join("\n");

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline';">
<title>PR #${detail.number}</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: var(--vscode-font-family);
    font-size: var(--vscode-font-size);
    color: var(--vscode-foreground);
    background: var(--vscode-editor-background);
    padding: 20px 24px;
    line-height: 1.5;
  }
  h1 { font-size: 1.2rem; margin-bottom: 4px; }
  .meta { color: var(--vscode-descriptionForeground); font-size: 0.85rem; margin-bottom: 20px; }
  .stats { display: flex; gap: 24px; margin-bottom: 24px; padding: 12px 0; border-bottom: 1px solid var(--vscode-editorWidget-border, #454545); }
  .stat-value { font-size: 1.8rem; font-weight: 700; line-height: 1; }
  .stat-label { font-size: 0.7rem; color: var(--vscode-descriptionForeground); text-transform: uppercase; letter-spacing: 0.06em; margin-top: 2px; }
  .c-error { color: var(--vscode-errorForeground, #f14c4c); }
  .c-warn { color: var(--vscode-editorWarning-foreground, #cca700); }
  .c-info { color: var(--vscode-editorInfo-foreground, #3794ff); }
  table { width: 100%; border-collapse: collapse; }
  th {
    text-align: left; padding: 6px 10px;
    border-bottom: 1px solid var(--vscode-editorWidget-border, #454545);
    font-size: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em;
    color: var(--vscode-descriptionForeground);
  }
  td { padding: 8px 10px; border-bottom: 1px solid var(--vscode-editorWidget-border, #2d2d2d); vertical-align: top; }
  .loc { font-family: var(--vscode-editor-font-family, monospace); font-size: 0.83em; white-space: nowrap; color: var(--vscode-descriptionForeground); }
  .badge {
    display: inline-block; padding: 1px 7px; border-radius: 3px;
    font-size: 0.72rem; font-weight: 700; font-family: monospace; letter-spacing: 0.03em;
  }
  .badge-p0, .badge-p1 { background: rgba(241,76,76,0.15); color: var(--vscode-errorForeground, #f14c4c); }
  .badge-p2 { background: rgba(204,167,0,0.15); color: var(--vscode-editorWarning-foreground, #cca700); }
  .badge-p3, .badge-— { background: rgba(55,148,255,0.12); color: var(--vscode-editorInfo-foreground, #3794ff); }
  .empty { color: var(--vscode-descriptionForeground); padding: 32px; text-align: center; }
</style>
</head>
<body>
  <h1>PR #${detail.number} — ${escHtml(detail.title)}</h1>
  <div class="meta">${escHtml(detail.repo)}</div>
  <div class="stats">
    <div>
      <div class="stat-value c-error">${p01}</div>
      <div class="stat-label">P0/P1 Errors</div>
    </div>
    <div>
      <div class="stat-value c-warn">${p2}</div>
      <div class="stat-label">P2 Warnings</div>
    </div>
    <div>
      <div class="stat-value c-info">${p3}</div>
      <div class="stat-label">P3 Info</div>
    </div>
    <div>
      <div class="stat-value">${comments.length}</div>
      <div class="stat-label">Total</div>
    </div>
  </div>
  ${
    comments.length === 0
      ? `<div class="empty">No findings for this PR.</div>`
      : `<table>
    <thead><tr><th>Priority</th><th>Location</th><th>Finding</th></tr></thead>
    <tbody>${rows}</tbody>
  </table>`
  }
</body>
</html>`;
}

function escHtml(s: string): string {
  return (s ?? "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
