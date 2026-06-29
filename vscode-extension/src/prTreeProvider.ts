import * as vscode from "vscode";
import { PRSummary, PRComment } from "./api";

export class PRNode extends vscode.TreeItem {
  constructor(public readonly pr: PRSummary, public readonly findings: PRComment[]) {
    super(
      `#${pr.number} ${pr.title}`,
      findings.length > 0
        ? vscode.TreeItemCollapsibleState.Expanded
        : vscode.TreeItemCollapsibleState.Collapsed
    );
    this.description = `${pr.current_score}/100 · ${pr.pr_status}`;
    this.iconPath = new vscode.ThemeIcon("git-pull-request");
    this.contextValue = "pr";
    this.tooltip = new vscode.MarkdownString(`**#${pr.number}** ${pr.title}\n\nScore: ${pr.current_score}/100 · ${pr.pr_status} · @${pr.author}`);
  }
}

export class FindingNode extends vscode.TreeItem {
  constructor(public readonly comment: PRComment, folder: vscode.WorkspaceFolder) {
    const label = comment.body.length > 80 ? comment.body.slice(0, 77) + "…" : comment.body;
    super(label, vscode.TreeItemCollapsibleState.None);
    this.description = `${comment.path}:${comment.line}`;
    this.iconPath = iconForPriority(comment.priority);
    this.tooltip = comment.body;
    this.contextValue = "finding";
    this.command = {
      command: "vscode.open",
      title: "Go to finding",
      arguments: [
        vscode.Uri.joinPath(folder.uri, comment.path),
        {
          selection: new vscode.Range(
            Math.max(0, (comment.line || 1) - 1),
            0,
            Math.max(0, (comment.line || 1) - 1),
            0
          ),
        },
      ],
    };
  }
}

function iconForPriority(priority: string): vscode.ThemeIcon {
  switch ((priority || "").toLowerCase()) {
    case "p0":
    case "p1":
      return new vscode.ThemeIcon("error", new vscode.ThemeColor("list.errorForeground"));
    case "p2":
      return new vscode.ThemeIcon("warning", new vscode.ThemeColor("list.warningForeground"));
    default:
      return new vscode.ThemeIcon("info");
  }
}

type TreeNode = PRNode | FindingNode;

export class PRTreeProvider implements vscode.TreeDataProvider<TreeNode> {
  private readonly _onDidChangeTreeData = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this._onDidChangeTreeData.event;

  private nodes: PRNode[] = [];
  private folder: vscode.WorkspaceFolder | undefined;

  update(
    prs: PRSummary[],
    findingsByPR: Map<number, PRComment[]>,
    folder: vscode.WorkspaceFolder
  ): void {
    this.folder = folder;
    this.nodes = prs.map((pr) => new PRNode(pr, findingsByPR.get(pr.number) ?? []));
    this._onDidChangeTreeData.fire();
  }

  clear(): void {
    this.nodes = [];
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(node: TreeNode): vscode.TreeItem {
    return node;
  }

  getChildren(node?: TreeNode): TreeNode[] {
    if (!node) return this.nodes;
    if (node instanceof PRNode && this.folder) {
      return node.findings.map((f) => new FindingNode(f, this.folder!));
    }
    return [];
  }
}
