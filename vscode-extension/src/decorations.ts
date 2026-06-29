import * as vscode from "vscode";
import * as path from "path";
import { PRComment } from "./api";

function svgUri(extensionPath: string, name: string): vscode.Uri {
  return vscode.Uri.file(path.join(extensionPath, "assets", `${name}.svg`));
}

export interface DecorationTypes {
  error: vscode.TextEditorDecorationType;
  warning: vscode.TextEditorDecorationType;
  info: vscode.TextEditorDecorationType;
}

export function createDecorationTypes(extensionPath: string): DecorationTypes {
  return {
    error: vscode.window.createTextEditorDecorationType({
      gutterIconPath: svgUri(extensionPath, "error"),
      gutterIconSize: "contain",
      overviewRulerColor: new vscode.ThemeColor("editorError.foreground"),
      overviewRulerLane: vscode.OverviewRulerLane.Right,
    }),
    warning: vscode.window.createTextEditorDecorationType({
      gutterIconPath: svgUri(extensionPath, "warning"),
      gutterIconSize: "contain",
      overviewRulerColor: new vscode.ThemeColor("editorWarning.foreground"),
      overviewRulerLane: vscode.OverviewRulerLane.Right,
    }),
    info: vscode.window.createTextEditorDecorationType({
      gutterIconPath: svgUri(extensionPath, "info"),
      gutterIconSize: "contain",
      overviewRulerColor: new vscode.ThemeColor("editorInfo.foreground"),
      overviewRulerLane: vscode.OverviewRulerLane.Right,
    }),
  };
}

export function applyGutterDecorations(
  editor: vscode.TextEditor,
  comments: PRComment[],
  folder: vscode.WorkspaceFolder,
  types: DecorationTypes
): void {
  const relPath = vscode.workspace.asRelativePath(editor.document.uri, false);
  const mine = comments.filter((c) => c.path === relPath);

  const errors: vscode.DecorationOptions[] = [];
  const warnings: vscode.DecorationOptions[] = [];
  const infos: vscode.DecorationOptions[] = [];

  for (const c of mine) {
    const line = Math.max(0, (c.line || 1) - 1);
    const range = new vscode.Range(line, 0, line, 0);
    const hoverMessage = new vscode.MarkdownString(
      `**PR Reviewer — ${c.priority?.toUpperCase() ?? "INFO"}**\n\n${c.body}`
    );
    const opt: vscode.DecorationOptions = { range, hoverMessage };
    const p = (c.priority || "").toLowerCase();
    if (p === "p0" || p === "p1") errors.push(opt);
    else if (p === "p2") warnings.push(opt);
    else infos.push(opt);
  }

  editor.setDecorations(types.error, errors);
  editor.setDecorations(types.warning, warnings);
  editor.setDecorations(types.info, infos);
}

export function clearDecorations(editor: vscode.TextEditor, types: DecorationTypes): void {
  editor.setDecorations(types.error, []);
  editor.setDecorations(types.warning, []);
  editor.setDecorations(types.info, []);
}
