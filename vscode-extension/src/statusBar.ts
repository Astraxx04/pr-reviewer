import * as vscode from "vscode";
import { PRSummary } from "./api";

export class StatusBarManager {
  private item: vscode.StatusBarItem;

  constructor(context: vscode.ExtensionContext) {
    this.item = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    this.item.command = "prReviewer.focusSidebar";
    context.subscriptions.push(this.item);
    this.item.text = "$(git-pull-request) PR Reviewer";
    this.item.show();
  }

  update(pr: PRSummary | undefined, findingCount: number) {
    if (!pr) {
      this.item.text = "$(git-pull-request) PR Reviewer";
      this.item.tooltip = "No PR detected for current branch — click to open panel";
      return;
    }
    const icon = findingCount === 0 ? "$(check)" : "$(warning)";
    this.item.text = `$(git-pull-request) #${pr.number} · ${pr.current_score}/100 ${icon}`;
    const md = new vscode.MarkdownString(
      `**${pr.title}**\n\n${findingCount} finding(s) — click to open PR Reviewer panel`
    );
    md.isTrusted = true;
    this.item.tooltip = md;
  }
}
