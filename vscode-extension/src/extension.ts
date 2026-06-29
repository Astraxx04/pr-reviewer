import * as vscode from "vscode";
import { Client, ApiError, PRSummary, PRComment } from "./api";
import { detectRepo, getCurrentBranch, RepoRef } from "./git";
import { StatusBarManager } from "./statusBar";
import { PRTreeProvider, PRNode } from "./prTreeProvider";
import { FindingHoverProvider } from "./hoverProvider";
import { createDecorationTypes, applyGutterDecorations, clearDecorations, DecorationTypes } from "./decorations";
import { openPRWebview } from "./webviewPanel";

let diagnosticCollection: vscode.DiagnosticCollection;
let decorationTypes: DecorationTypes | undefined;

// Shared state updated by every refresh
let allComments: PRComment[] = [];
let currentFolder: vscode.WorkspaceFolder | undefined;
let currentRepo: RepoRef | undefined;
let currentClient: Client | undefined;

let pollTimer: ReturnType<typeof setInterval> | undefined;

interface Surfaces {
  statusBar: StatusBarManager;
  treeProvider: PRTreeProvider;
  hoverProvider: FindingHoverProvider;
}

export function activate(context: vscode.ExtensionContext) {
  diagnosticCollection = vscode.languages.createDiagnosticCollection("pr-reviewer");
  decorationTypes = createDecorationTypes(context.extensionPath);

  const statusBar = new StatusBarManager(context);
  const treeProvider = new PRTreeProvider();
  const hoverProvider = new FindingHoverProvider();
  const surfaces: Surfaces = { statusBar, treeProvider, hoverProvider };

  const treeView = vscode.window.createTreeView("prReviewer.prList", {
    treeDataProvider: treeProvider,
    showCollapseAll: true,
  });

  context.subscriptions.push(
    diagnosticCollection,
    treeView,
    vscode.languages.registerHoverProvider({ scheme: "file" }, hoverProvider),

    // Re-apply gutter decorations when switching files
    vscode.window.onDidChangeActiveTextEditor((editor) => {
      if (editor && decorationTypes && currentFolder) {
        applyGutterDecorations(editor, allComments, currentFolder, decorationTypes);
      }
    }),

    // --- Commands ---

    vscode.commands.registerCommand("prReviewer.showFindings", () =>
      withErrors(async () => {
        const ctx = await getContext();
        const pr = await pickPR(ctx.client, ctx.repo);
        if (!pr) return;
        const detail = await ctx.client.getPR(ctx.repo.owner, ctx.repo.repo, pr.number);
        const comments = detail.latest_comments ?? [];
        applyAllSurfaces(ctx.folder, ctx.repo, ctx.client, comments, [pr], surfaces);
        const n = comments.length;
        vscode.window.showInformationMessage(
          n === 0
            ? `PR Reviewer: no findings on #${pr.number}.`
            : `PR Reviewer: ${n} finding(s) on #${pr.number} (see Problems panel).`
        );
      })
    ),

    vscode.commands.registerCommand("prReviewer.triggerReview", (node?: unknown) =>
      withErrors(async () => {
        const ctx = await getContext();
        let prNumber: number;
        if (node instanceof PRNode) {
          prNumber = node.pr.number;
        } else {
          const pr = await pickPR(ctx.client, ctx.repo);
          if (!pr) return;
          prNumber = pr.number;
        }
        await ctx.client.triggerReview(ctx.repo.owner, ctx.repo.repo, prNumber);
        vscode.window.showInformationMessage(`PR Reviewer: review queued for #${prNumber}.`);
      })
    ),

    vscode.commands.registerCommand("prReviewer.clearFindings", () => {
      diagnosticCollection.clear();
      allComments = [];
      treeProvider.clear();
      hoverProvider.clear();
      statusBar.update(undefined, 0);
      if (decorationTypes) {
        for (const editor of vscode.window.visibleTextEditors) {
          clearDecorations(editor, decorationTypes);
        }
      }
      vscode.window.showInformationMessage("PR Reviewer: findings cleared.");
    }),

    vscode.commands.registerCommand("prReviewer.focusSidebar", () => {
      vscode.commands.executeCommand("prReviewer.prList.focus");
    }),

    vscode.commands.registerCommand("prReviewer.openWebview", (pr: PRSummary) =>
      withErrors(async () => {
        const ctx = await getContext();
        await openPRWebview(context, ctx.client, ctx.repo, pr);
      })
    ),

    vscode.commands.registerCommand("prReviewer.refresh", () =>
      withErrors(() => refreshAll(surfaces))
    ),

    { dispose: () => { if (pollTimer) clearInterval(pollTimer); } }
  );

  // Start background refresh (silently — no error toasts on startup)
  startPolling(surfaces);
}

export function deactivate() {
  diagnosticCollection?.dispose();
  if (pollTimer) clearInterval(pollTimer);
}

// --- Background refresh ---

function startPolling(surfaces: Surfaces): void {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = setInterval(() => refreshAll(surfaces).catch(() => {}), 60_000);
  refreshAll(surfaces).catch(() => {});
}

async function refreshAll(surfaces: Surfaces): Promise<void> {
  let ctx: Awaited<ReturnType<typeof getContext>>;
  try {
    ctx = await getContext();
  } catch {
    return; // not configured yet — stay silent
  }

  const { prs } = await ctx.client.listPRs(`${ctx.repo.owner}/${ctx.repo.repo}`);
  if (!prs || prs.length === 0) {
    surfaces.statusBar.update(undefined, 0);
    surfaces.treeProvider.clear();
    return;
  }

  const branch = await getCurrentBranch(ctx.folder.uri.fsPath);
  const activePR = (branch ? prs.find((p) => p.head_branch === branch) : undefined) ?? prs[0];

  const detail = await ctx.client.getPR(ctx.repo.owner, ctx.repo.repo, activePR.number);
  const comments = detail.latest_comments ?? [];

  applyAllSurfaces(ctx.folder, ctx.repo, ctx.client, comments, prs, surfaces, activePR);
}

// --- Surface orchestration ---

function applyAllSurfaces(
  folder: vscode.WorkspaceFolder,
  repo: RepoRef,
  client: Client,
  comments: PRComment[],
  prs: PRSummary[],
  surfaces: Surfaces,
  activePR?: PRSummary
): void {
  allComments = comments;
  currentFolder = folder;
  currentRepo = repo;
  currentClient = client;

  renderDiagnostics(folder, comments);

  const findingsByPR = new Map<number, PRComment[]>();
  if (activePR) findingsByPR.set(activePR.number, comments);
  surfaces.treeProvider.update(prs, findingsByPR, folder);
  surfaces.hoverProvider.update(comments, folder);
  surfaces.statusBar.update(activePR, comments.length);

  if (decorationTypes) {
    for (const editor of vscode.window.visibleTextEditors) {
      applyGutterDecorations(editor, comments, folder, decorationTypes);
    }
  }
}

// --- Helpers ---

interface Context {
  client: Client;
  folder: vscode.WorkspaceFolder;
  repo: RepoRef;
}

async function getContext(): Promise<Context> {
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

function renderDiagnostics(folder: vscode.WorkspaceFolder, comments: PRComment[]): void {
  diagnosticCollection.clear();
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
  for (const [filePath, diags] of byFile) {
    diagnosticCollection.set(vscode.Uri.joinPath(folder.uri, filePath), diags);
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

async function withErrors(fn: () => Promise<void>): Promise<void> {
  try {
    await fn();
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    vscode.window.showErrorMessage(`PR Reviewer: ${msg}`);
  }
}
