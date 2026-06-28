"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import { KeyboardShortcutsModal } from "@/components/keyboard-shortcuts-modal";
import { listPRs, type PRSummary, type PRStatus } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

const STATUS_FILTERS: { label: string; value: string }[] = [
  { label: "All", value: "" },
  { label: "Approved", value: "approved" },
  { label: "Changes Requested", value: "changes_requested" },
  { label: "Commented", value: "commented" },
  { label: "Pending", value: "pending" },
];

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

function scoreColor(score: number): string {
  if (score >= 80) return "text-green-600 dark:text-green-400";
  if (score >= 60) return "text-yellow-600 dark:text-yellow-400";
  return "text-destructive";
}

function timeAgo(dateStr: string | null): string {
  if (!dateStr) return "—";
  const diff = Date.now() - new Date(dateStr).getTime();
  const h = Math.floor(diff / 3_600_000);
  if (h < 1) return "< 1h ago";
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  if (d < 30) return `${d}d ago`;
  return `${Math.floor(d / 30)}mo ago`;
}

export default function PRsPage() {
  const { token } = useToken();
  const router = useRouter();
  const [data, setData] = useState<{ prs: PRSummary[]; total: number } | null>(null);
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(true);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    listPRs(token, { page, per_page: 25, status: status || undefined })
      .then(setData)
      .finally(() => setLoading(false));
  }, [token, page, status]);

  const prs = data?.prs ?? [];
  const totalPages = Math.ceil((data?.total ?? 0) / 25);

  useKeyboardShortcuts({
    onShowHelp: () => setShortcutsOpen(true),
    listItemSelector: "tbody tr[data-pr-href]",
    openItemCallback: (el) => {
      const href = (el as HTMLElement).dataset.prHref;
      if (href) router.push(href);
    },
  });

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Pull Requests</h1>

      <div className="flex gap-2 flex-wrap" role="group" aria-label="Filter by status">
        {STATUS_FILTERS.map((f) => (
          <Button
            key={f.value}
            size="sm"
            variant={status === f.value ? "default" : "outline"}
            onClick={() => { setStatus(f.value); setPage(1); }}
            aria-pressed={status === f.value}
            style={{ cursor: 'pointer' }}
          >
            {f.label}
          </Button>
        ))}
      </div>

      {loading ? (
        <Skeleton className="h-64 w-full" aria-label="Loading pull requests" />
      ) : prs.length === 0 ? (
        <p className="text-muted-foreground" role="status">No pull requests found.</p>
      ) : (
        <div className="rounded-lg border overflow-x-auto">
          <table className="w-full text-sm" aria-label="Pull requests list">
            <thead className="border-b bg-muted/50">
              <tr>
                <th className="px-4 py-3 text-left font-medium" scope="col">Pull Request</th>
                <th className="px-4 py-3 text-left font-medium" scope="col">Repo</th>
                <th className="px-4 py-3 text-left font-medium" scope="col">Status</th>
                <th className="px-4 py-3 text-left font-medium" scope="col">Score</th>
                <th className="px-4 py-3 text-left font-medium" scope="col">Reviews</th>
                <th className="px-4 py-3 text-left font-medium" scope="col">Last Review</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {prs.map((pr) => {
                const [owner, repo] = pr.repo.split("/");
                const href = `/prs/${owner}/${repo}/${pr.number}`;
                return (
                  <tr
                    key={pr.id}
                    className="hover:bg-muted/30 transition-colors focus-within:bg-muted/30 cursor-pointer"
                    data-pr-href={href}
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === "o") router.push(href);
                    }}
                    onClick={() => router.push(href)}
                    aria-label={`${pr.title || `PR #${pr.number}`} — ${prStatusLabel(pr.pr_status)}, score ${pr.review_count > 0 ? `${pr.current_score}/100` : "not reviewed"}`}
                  >
                    <td className="px-4 py-3">
                      <Link
                        href={href}
                        className="font-medium hover:underline focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring rounded-sm"
                        onClick={(e) => e.stopPropagation()}
                        tabIndex={-1}
                      >
                        {pr.title || `PR #${pr.number}`}
                      </Link>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        #{pr.number} · {pr.author}
                      </p>
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{pr.repo}</td>
                    <td className="px-4 py-3">
                      <Badge variant={prStatusVariant(pr.pr_status)}>
                        {prStatusLabel(pr.pr_status)}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      {pr.review_count > 0 ? (
                        <span className={`font-mono font-semibold ${scoreColor(pr.current_score)}`}>
                          {pr.current_score}/100
                        </span>
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{pr.review_count}</td>
                    <td className="px-4 py-3 text-muted-foreground text-xs">
                      <time dateTime={pr.last_reviewed_at ?? undefined}>{timeAgo(pr.last_reviewed_at)}</time>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <nav className="flex gap-2" aria-label="Pagination">
          <Button
            variant="outline"
            size="sm"
            disabled={page === 1}
            onClick={() => setPage((p) => p - 1)}
            aria-label="Previous page"
          >
            Previous
          </Button>
          <span className="px-3 py-2 text-sm text-muted-foreground" aria-current="page">
            Page {page} of {totalPages}
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={page === totalPages}
            onClick={() => setPage((p) => p + 1)}
            aria-label="Next page"
          >
            Next
          </Button>
        </nav>
      )}

      <KeyboardShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
    </div>
  );
}
