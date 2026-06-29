import * as vscode from "vscode";
import { PRComment } from "./api";

export class FindingHoverProvider implements vscode.HoverProvider {
  private comments: PRComment[] = [];
  private folder: vscode.WorkspaceFolder | undefined;

  update(comments: PRComment[], folder: vscode.WorkspaceFolder): void {
    this.comments = comments;
    this.folder = folder;
  }

  clear(): void {
    this.comments = [];
    this.folder = undefined;
  }

  provideHover(
    document: vscode.TextDocument,
    position: vscode.Position
  ): vscode.Hover | undefined {
    if (!this.folder) return;
    const relPath = vscode.workspace.asRelativePath(document.uri, false);
    const match = this.comments.find(
      (c) => c.path === relPath && Math.max(0, (c.line || 1) - 1) === position.line
    );
    if (!match) return;

    const md = new vscode.MarkdownString();
    md.isTrusted = true;
    md.appendMarkdown(`### PR Reviewer — \`${match.priority?.toUpperCase() ?? "INFO"}\`\n\n`);
    md.appendMarkdown(match.body);
    return new vscode.Hover(md);
  }
}
