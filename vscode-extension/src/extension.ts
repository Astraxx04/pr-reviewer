import * as vscode from "vscode";
import { Client, ApiError, PRSummary, PRComment } from "./api";
import { detectRepo, RepoRef } from "./git";

let diagnostics: vscode.DiagnosticCollection;

export function activate(context: vscode.ExtensionContext) {
  diagnostics = vscode.languages.createDiagnosticCollection("pr-reviewer");
  context.subscriptions.push(diagnostics);

  context.subscriptions.push(
    vscode.commands.registerCommand("prReviewer.showFindings", () => withErrors(showFindings)),
    vscode.commands.registerCommand("prReviewer.triggerReview", () => withErrors(triggerReview)),
    vscode.commands.registerCommand("prReviewer.clearFindings", () => {
      diagnostics.clear();
      vscode.window.showInformationMessage("PR Reviewer: findings cleared.");
    })
  );
}

export function deactivate() {
  diagnostics?.dispose();
}

// --- commands ---

async function showFindings() {
  const { client, folder, repo } = await context();
  const pr = await pickPR(client, repo);
  if (!pr) return;

  const detail = await client.getPR(repo.owner, repo.repo, pr.number);
  renderDiagnostics(folder, detail.latest_comments ?? []);

  const n = (detail.latest_comments ?? []).length;
  if (n === 0) {
    vscode.window.showInformationMessage(`PR Reviewer: no findings on #${pr.number}.`);
  } else {
    vscode.window.showInformationMessage(`PR Reviewer: ${n} finding(s) on #${pr.number} (see Problems panel).`);
  }
}

async function triggerReview() {
  const { client, repo } = await context();
  const pr = await pickPR(client, repo);
  if (!pr) return;
  await client.triggerReview(repo.owner, repo.repo, pr.number);
  vscode.window.showInformationMessage(`PR Reviewer: review queued for #${pr.number}.`);
}

// --- helpers ---

interface Context {
  client: Client;
  folder: vscode.WorkspaceFolder;
  repo: RepoRef;
}

async function context(): Promise<Context> {
  const cfg = vscode.workspace.getConfiguration("prReviewer");
  const serverUrl = (cfg.get<string>("serverUrl") ?? "").trim();
  const apiToken = (cfg.get<string>("apiToken") ?? "").trim();
  if (!serverUrl || !apiToken) {
    throw new ApiError("Set prReviewer.serverUrl and prReviewer.apiToken in Settings first.");
  }

  const folder = vscode.workspace.workspaceFolders?.[0];
  if (!folder) {
    throw new ApiError("Open a folder containing a git repository first.");
  }

  const repo = await detectRepo(folder.uri.fsPath);
  if (!repo) {
    throw new ApiError("Could not determine the GitHub repo from the git 'origin' remote.");
  }
  return { client: new Client(serverUrl, apiToken), folder, repo };
}

async function pickPR(client: Client, repo: RepoRef): Promise<PRSummary | undefined> {
  const { prs } = await client.listPRs(`${repo.owner}/${repo.repo}`);
  if (!prs || prs.length === 0) {
    vscode.window.showInformationMessage(`PR Reviewer: no PRs found for ${repo.owner}/${repo.repo}.`);
    return undefined;
  }
  const picked = await vscode.window.showQuickPick(
    prs.map((pr) => ({
      label: `#${pr.number} ${pr.title}`,
      description: `${pr.pr_status} · ${pr.current_score}/100 · @${pr.author}`,
      pr,
    })),
    { placeHolder: `Select a PR in ${repo.owner}/${repo.repo}` }
  );
  return picked?.pr;
}

function renderDiagnostics(folder: vscode.WorkspaceFolder, comments: PRComment[]) {
  diagnostics.clear();
  const byFile = new Map<string, vscode.Diagnostic[]>();
  for (const c of comments) {
    if (!c.path) continue;
    const line = Math.max(0, (c.line || 1) - 1);
    const range = new vscode.Range(line, 0, line, Number.MAX_SAFE_INTEGER);
    const diag = new vscode.Diagnostic(range, c.body, severityFor(c.priority));
    diag.source = `pr-reviewer${c.priority ? ` (${c.priority})` : ""}`;
    const list = byFile.get(c.path) ?? [];
    list.push(diag);
    byFile.set(c.path, list);
  }
  for (const [path, diags] of byFile) {
    const uri = vscode.Uri.joinPath(folder.uri, path);
    diagnostics.set(uri, diags);
  }
}

function severityFor(priority: string): vscode.DiagnosticSeverity {
  switch ((priority || "").toLowerCase()) {
    case "p0":
    case "p1":
      return vscode.DiagnosticSeverity.Error;
    case "p2":
      return vscode.DiagnosticSeverity.Warning;
    default:
      return vscode.DiagnosticSeverity.Information;
  }
}

async function withErrors(fn: () => Promise<void>) {
  try {
    await fn();
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    vscode.window.showErrorMessage(`PR Reviewer: ${msg}`);
  }
}
