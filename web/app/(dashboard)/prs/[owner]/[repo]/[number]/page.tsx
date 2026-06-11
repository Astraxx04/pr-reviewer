"use client";

import { useEffect, useState, use, Fragment } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { getPR, getPRDiff, requestReReview, submitCommentFeedback, explainComment, type PRDetail, type PRStatus, type FileDiff, type PRComment } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { toast } from "sonner";
import { ThumbsUp, ThumbsDown, HelpCircle, ChevronDown, ChevronUp, Code } from "lucide-react";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, ReferenceLine,
} from "recharts";

// ---- Status helpers ----

function prStatusVariant(s: PRStatus): "default" | "destructive" | "secondary" | "outline" {
  if (s === "APPROVED") return "default";
  if (s === "CHANGES_REQUESTED") return "destructive";
  if (s === "COMMENTED") return "secondary";
  return "outline";
}

function prStatusLabel(s: PRStatus): string {
  if (s === "APPROVED") return "Approved";
  if (s === "CHANGES_REQUESTED") return "Changes Requested";
  if (s === "COMMENTED") return "Commented";
  return "Pending";
}

function priorityBorderColor(priority: string): string {
  if (priority === "p0") return "border-l-red-500";
  if (priority === "p1") return "border-l-orange-500";
  if (priority === "p2") return "border-l-yellow-500";
  return "border-l-muted-foreground/30";
}

function priorityLabel(priority: string): string {
  if (priority === "p0") return "P0 Critical";
  if (priority === "p1") return "P1 High";
  if (priority === "p2") return "P2 Medium";
  return "P3 Low";
}

function priorityVariant(priority: string): "default" | "destructive" | "secondary" | "outline" {
  if (priority === "p0" || priority === "p1") return "destructive";
  if (priority === "p2") return "secondary";
  return "outline";
}

// ---- Diff parser ----

interface DiffLine {
  type: "add" | "del" | "ctx" | "hdr";
  content: string;
  oldLine?: number;
  newLine?: number;
}

function parsePatch(patch: string): DiffLine[] {
  if (!patch) return [];
  const result: DiffLine[] = [];
  let oldLine = 0, newLine = 0;

  for (const raw of patch.split("\n")) {
    if (raw.startsWith("@@")) {
      const m = raw.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
      if (m) {
        oldLine = parseInt(m[1]) - 1;
        newLine = parseInt(m[2]) - 1;
      }
      result.push({ type: "hdr", content: raw });
    } else if (raw.startsWith("+")) {
      newLine++;
      result.push({ type: "add", content: raw.slice(1), newLine });
    } else if (raw.startsWith("-")) {
      oldLine++;
      result.push({ type: "del", content: raw.slice(1), oldLine });
    } else if (raw.startsWith(" ") || raw === "") {
      oldLine++;
      newLine++;
      result.push({ type: "ctx", content: raw.startsWith(" ") ? raw.slice(1) : raw, oldLine, newLine });
    }
  }
  return result;
}

// ---- Diff file view component ----

function DiffFileView({
  file,
  comments,
}: {
  file: FileDiff;
  comments: PRComment[];
}) {
  const [open, setOpen] = useState(true);
  const lines = parsePatch(file.patch);

  // Build a map from new-line number to comments.
  const commentsByLine = new Map<number, PRComment[]>();
  for (const c of comments) {
    if (!commentsByLine.has(c.line)) commentsByLine.set(c.line, []);
    commentsByLine.get(c.line)!.push(c);
  }

  const statusColor =
    file.status === "added" ? "text-green-600 dark:text-green-400" :
    file.status === "removed" ? "text-destructive" :
    "text-muted-foreground";

  return (
    <Card>
      <CardHeader className="pb-0">
        <button
          className="flex items-center justify-between gap-2 w-full text-left focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring rounded-sm"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-controls={`diff-${file.filename}`}
        >
          <div className="flex items-center gap-2 min-w-0">
            <Code className="h-4 w-4 flex-shrink-0 text-muted-foreground" aria-hidden="true" />
            <span className="font-mono text-sm truncate">{file.filename}</span>
            <span className={`text-xs flex-shrink-0 ${statusColor}`}>
              +{file.additions} -{file.deletions}
            </span>
            {comments.length > 0 && (
              <Badge variant="secondary" className="text-xs flex-shrink-0">
                {comments.length} {comments.length === 1 ? "comment" : "comments"}
              </Badge>
            )}
          </div>
          {open
            ? <ChevronUp className="h-4 w-4 flex-shrink-0 text-muted-foreground" aria-hidden="true" />
            : <ChevronDown className="h-4 w-4 flex-shrink-0 text-muted-foreground" aria-hidden="true" />
          }
        </button>
      </CardHeader>

      {open && (
        <CardContent className="p-0 pt-2" id={`diff-${file.filename}`}>
          {lines.length === 0 ? (
            <p className="px-4 py-2 text-xs text-muted-foreground italic">
              {file.status === "removed" ? "File deleted." : "No textual diff available."}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs font-mono border-collapse" aria-label={`Diff for ${file.filename}`}>
                <tbody>
                  {lines.map((line, i) => {
                    const lineComments = line.newLine ? commentsByLine.get(line.newLine) : undefined;
                    const rowBg =
                      line.type === "hdr" ? "bg-muted/50 text-muted-foreground" :
                      line.type === "add" ? "bg-green-500/10 dark:bg-green-900/20" :
                      line.type === "del" ? "bg-red-500/10 dark:bg-red-900/20" :
                      "";
                    const linePrefix =
                      line.type === "add" ? "+" :
                      line.type === "del" ? "-" :
                      line.type === "hdr" ? "" : " ";

                    return (
                      <Fragment key={i}>
                        <tr className={`${rowBg} leading-5`}>
                          {line.type !== "hdr" ? (
                            <>
                              <td className="select-none w-10 px-2 text-right text-muted-foreground border-r border-border">
                                {line.oldLine ?? ""}
                              </td>
                              <td className="select-none w-10 px-2 text-right text-muted-foreground border-r border-border">
                                {line.newLine ?? ""}
                              </td>
                              <td className="select-none w-4 px-1 text-center text-muted-foreground">
                                {linePrefix}
                              </td>
                            </>
                          ) : (
                            <td colSpan={3} />
                          )}
                          <td className="px-3 py-0.5 whitespace-pre">
                            {line.type === "hdr" ? (
                              <span className="text-muted-foreground">{line.content}</span>
                            ) : (
                              line.content || " "
                            )}
                          </td>
                        </tr>
                        {lineComments?.map((comment) => (
                          <tr key={`comment-${comment.id}`} className="bg-yellow-50/80 dark:bg-yellow-950/30">
                            <td colSpan={4} className="px-4 py-2">
                              <div className={`border-l-2 pl-3 ${priorityBorderColor(comment.priority)}`}>
                                <div className="flex items-center gap-2 mb-1 flex-wrap">
                                  <Badge variant={priorityVariant(comment.priority)} className="text-xs">
                                    {priorityLabel(comment.priority)}
                                  </Badge>
                                  {comment.has_reply && (
                                    <span className="text-xs text-muted-foreground">· replied</span>
                                  )}
                                </div>
                                <p className="text-xs leading-relaxed">{comment.body}</p>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </Fragment>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      )}
    </Card>
  );
}

// ---- Main page ----

export default function PRDetailPage({
  params,
}: {
  params: Promise<{ owner: string; repo: string; number: string }>;
}) {
  const { owner, repo, number } = use(params);
  const { token } = useToken();
  const router = useRouter();
  const [pr, setPR] = useState<PRDetail | null>(null);
  const [diffs, setDiffs] = useState<FileDiff[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [rereviewing, setRereviewing] = useState(false);
  const [feedback, setFeedback] = useState<Record<number, { up: number; down: number; myVote: 1 | -1 | 0 }>>({});
  const [explaining, setExplaining] = useState<number | null>(null);
  const [explanation, setExplanation] = useState<{ id: number; text: string } | null>(null);

  useEffect(() => {
    if (!token) return;
    Promise.all([
      getPR(token, owner, repo, Number(number)),
      getPRDiff(token, owner, repo, Number(number)).catch(() => null),
    ]).then(([prData, diffData]) => {
      setPR(prData);
      setDiffs(diffData);
    }).finally(() => setLoading(false));
  }, [token, owner, repo, number]);

  async function handleReReview() {
    if (!token) return;
    setRereviewing(true);
    try {
      await requestReReview(token, owner, repo, Number(number));
      toast.success("Re-review queued");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setRereviewing(false);
    }
  }

  async function handleFeedback(commentID: number, vote: 1 | -1) {
    if (!token) return;
    try {
      const result = await submitCommentFeedback(token, commentID, vote);
      setFeedback(prev => ({
        ...prev,
        [commentID]: { up: result.up, down: result.down, myVote: result.my_vote },
      }));
    } catch {
      toast.error("Failed to save feedback");
    }
  }

  async function handleExplain(commentID: number) {
    if (!token) return;
    setExplaining(commentID);
    try {
      const result = await explainComment(token, commentID);
      setExplanation({ id: commentID, text: result.explanation });
    } catch {
      toast.error("Could not generate explanation");
    } finally {
      setExplaining(null);
    }
  }

  if (loading) return (
    <div className="space-y-4 max-w-4xl" aria-busy="true" aria-label="Loading pull request details">
      <Skeleton className="h-10 w-full" />
      <Skeleton className="h-48 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );
  if (!pr) return <p className="text-muted-foreground">Pull request not found.</p>;

  const scoreHistory = pr.reviews.map((r) => ({
    date: new Date(r.created_at).toLocaleDateString(),
    score: r.score,
    id: r.id,
  }));

  const currentScore = pr.reviews.length > 0 ? pr.reviews[pr.reviews.length - 1].score : null;

  // Build comment map: file → comments (for diff overlay)
  const commentsByFile = new Map<string, PRComment[]>();
  for (const c of pr.latest_comments ?? []) {
    if (!commentsByFile.has(c.path)) commentsByFile.set(c.path, []);
    commentsByFile.get(c.path)!.push(c);
  }

  return (
    <div className="space-y-6 max-w-4xl">
      <div className="flex flex-wrap items-center gap-3">
        <Button variant="ghost" size="sm" onClick={() => router.push("/prs")} aria-label="Back to pull requests list">
          ← PRs
        </Button>
        <h1 className="text-xl font-bold flex-1">{pr.title || `PR #${pr.number}`}</h1>
        <Badge variant={prStatusVariant(pr.pr_status)}>{prStatusLabel(pr.pr_status)}</Badge>
        {currentScore !== null && (
          <span className="text-lg font-mono font-semibold" aria-label={`Score: ${currentScore} out of 100`}>
            {currentScore}/100
          </span>
        )}
        <Button size="sm" onClick={handleReReview} disabled={rereviewing} aria-label="Request re-review of this pull request">
          {rereviewing ? "Queuing…" : "Request Re-review"}
        </Button>
      </div>

      <dl className="text-sm text-muted-foreground flex gap-4 flex-wrap">
        <div><dt className="sr-only">PR number</dt><dd>#{pr.number}</dd></div>
        <div><dt className="sr-only">Author</dt><dd>by <strong>{pr.author}</strong></dd></div>
        <div><dt className="sr-only">Repository</dt><dd className="font-mono">{pr.repo}</dd></div>
        {pr.assignees?.length > 0 && (
          <div><dt className="sr-only">Assigned to</dt><dd>Assigned to: {pr.assignees.join(", ")}</dd></div>
        )}
      </dl>

      {scoreHistory.length > 1 && (
        <Card>
          <CardHeader><CardTitle>Score History</CardTitle></CardHeader>
          <CardContent>
            <ResponsiveContainer width="100%" height={180} aria-label="Score history chart">
              <LineChart data={scoreHistory}>
                <XAxis dataKey="date" tick={{ fontSize: 11 }} />
                <YAxis domain={[0, 100]} tick={{ fontSize: 11 }} />
                <Tooltip
                  formatter={(v) => [`${v}/100`, "Score"]}
                  contentStyle={{ background: "var(--popover)", border: "1px solid var(--border)", borderRadius: 8 }}
                  labelStyle={{ color: "var(--popover-foreground)" }}
                  itemStyle={{ color: "var(--popover-foreground)" }}
                />
                <ReferenceLine y={70} stroke="var(--muted-foreground)" strokeDasharray="4 2" />
                <Line
                  type="monotone"
                  dataKey="score"
                  stroke="var(--primary)"
                  strokeWidth={2}
                  dot={{ r: 4 }}
                />
              </LineChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      )}

      {pr.reviews.length > 0 && (
        <Card>
          <CardHeader><CardTitle>Review History</CardTitle></CardHeader>
          <CardContent>
            <ul className="divide-y" role="list" aria-label="Review history">
              {[...pr.reviews].reverse().map((r) => (
                <li key={r.id} className="py-3 flex items-start justify-between gap-4">
                  <div className="space-y-1 flex-1">
                    <div className="flex items-center gap-2 flex-wrap">
                      <Badge variant={r.status === "APPROVE" ? "default" : r.status === "REQUEST_CHANGES" ? "destructive" : "secondary"}>
                        {r.status}
                      </Badge>
                      <span className="font-mono text-sm" aria-label={`Score: ${r.score} out of 100`}>{r.score}/100</span>
                      <span className="text-xs text-muted-foreground">
                        {r.comment_count} comment{r.comment_count !== 1 ? "s" : ""}
                      </span>
                    </div>
                    {r.summary && <p className="text-sm text-muted-foreground line-clamp-2">{r.summary}</p>}
                  </div>
                  <time className="text-xs text-muted-foreground whitespace-nowrap" dateTime={r.created_at}>
                    {new Date(r.created_at).toLocaleString()}
                  </time>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      )}

      {/* Diff view with inline comment annotations */}
      {diffs && diffs.length > 0 && (
        <section aria-label="File diffs with inline review comments">
          <h2 className="text-lg font-semibold mb-4">
            Changed Files
            <span className="ml-2 text-sm font-normal text-muted-foreground">
              ({diffs.length} file{diffs.length !== 1 ? "s" : ""})
            </span>
          </h2>
          <div className="space-y-3">
            {diffs.map((file) => (
              <DiffFileView
                key={file.filename}
                file={file}
                comments={commentsByFile.get(file.filename) ?? []}
              />
            ))}
          </div>
        </section>
      )}

      {/* Fallback: file-by-file comment list when no diff is available */}
      {!diffs && Object.keys(Object.fromEntries(commentsByFile)).length > 0 && (
        <div className="space-y-4">
          <h2 className="text-lg font-semibold">Latest Review Comments</h2>
          {Array.from(commentsByFile.entries()).map(([file, comments]) => (
            <Card key={file}>
              <CardHeader className="pb-2">
                <CardTitle className="font-mono text-sm">{file}</CardTitle>
              </CardHeader>
              <CardContent>
                <ul className="space-y-3" role="list">
                  {comments.map((c) => (
                    <li key={c.id} className={`text-sm border-l-2 pl-3 ${priorityBorderColor(c.priority)}`}>
                      <div className="flex items-center gap-2 mb-1 flex-wrap">
                        <span className="text-xs text-muted-foreground">Line {c.line}</span>
                        <Badge variant={priorityVariant(c.priority)} className="text-xs">
                          {priorityLabel(c.priority)}
                        </Badge>
                        {c.has_reply && (
                          <span className="text-xs text-muted-foreground">· replied</span>
                        )}
                      </div>
                      <p className="mb-2">{c.body}</p>
                      <div className="flex items-center gap-2" role="group" aria-label="Comment feedback">
                        <button
                          onClick={() => handleFeedback(c.id, 1)}
                          className={`flex items-center gap-1 text-xs px-2 py-0.5 rounded border transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring ${
                            feedback[c.id]?.myVote === 1
                              ? "bg-green-100 border-green-400 text-green-700 dark:bg-green-900/30 dark:border-green-600 dark:text-green-400"
                              : "border-muted hover:border-muted-foreground text-muted-foreground"
                          }`}
                          aria-label={`Helpful finding, ${feedback[c.id]?.up ?? 0} upvotes`}
                          aria-pressed={feedback[c.id]?.myVote === 1}
                        >
                          <ThumbsUp className="h-3 w-3" aria-hidden="true" />
                          {feedback[c.id]?.up ?? 0}
                        </button>
                        <button
                          onClick={() => handleFeedback(c.id, -1)}
                          className={`flex items-center gap-1 text-xs px-2 py-0.5 rounded border transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring ${
                            feedback[c.id]?.myVote === -1
                              ? "bg-red-100 border-red-400 text-red-700 dark:bg-red-900/30 dark:border-red-600 dark:text-red-400"
                              : "border-muted hover:border-muted-foreground text-muted-foreground"
                          }`}
                          aria-label={`False positive, ${feedback[c.id]?.down ?? 0} downvotes`}
                          aria-pressed={feedback[c.id]?.myVote === -1}
                        >
                          <ThumbsDown className="h-3 w-3" aria-hidden="true" />
                          {feedback[c.id]?.down ?? 0}
                        </button>
                        <button
                          onClick={() => handleExplain(c.id)}
                          disabled={explaining === c.id}
                          className="flex items-center gap-1 text-xs px-2 py-0.5 rounded border border-muted hover:border-muted-foreground text-muted-foreground transition-colors disabled:opacity-50 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring"
                          aria-label="Explain this finding in more detail"
                        >
                          <HelpCircle className="h-3 w-3" aria-hidden="true" />
                          {explaining === c.id ? "Explaining…" : "Why?"}
                        </button>
                      </div>
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {pr.latest_comments?.length === 0 && pr.reviews.length > 0 && (
        <p className="text-muted-foreground text-sm" role="status">No comments on the latest review.</p>
      )}
      {pr.reviews.length === 0 && (
        <p className="text-muted-foreground text-sm" role="status">This PR has not been reviewed yet.</p>
      )}

      {explanation && (
        <Dialog open={!!explanation} onOpenChange={(open) => { if (!open) setExplanation(null); }}>
          <DialogContent className="max-w-lg max-h-[80vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle>Why is this a problem?</DialogTitle>
            </DialogHeader>
            <div className="text-sm whitespace-pre-wrap mt-2">{explanation.text}</div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}
