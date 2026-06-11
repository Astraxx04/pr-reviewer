"use client";

import { useCallback, useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { useSSE, type SSEEvent } from "@/hooks/useSSE";
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts";
import {
  getDashboardStats, listReviews, getSystemMetrics,
  type DashboardStats, type ReviewSummary, type SystemMetrics,
} from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { OnboardingChecklist } from "@/components/onboarding-checklist";
import { KeyboardShortcutsModal } from "@/components/keyboard-shortcuts-modal";
import { toast } from "sonner";

function statusVariant(s: string): "default" | "destructive" | "secondary" | "outline" {
  if (s === "APPROVE") return "default";
  if (s === "REQUEST_CHANGES") return "destructive";
  return "secondary";
}

function fmtMs(ms: number): string {
  if (!ms) return "—";
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${Math.round(ms)}ms`;
}

export default function DashboardPage() {
  const { token, isAdmin } = useToken();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [recent, setRecent] = useState<ReviewSummary[]>([]);
  const [sysMetrics, setSysMetrics] = useState<SystemMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);

  function fetchAll(t: string) {
    const core = Promise.all([
      getDashboardStats(t),
      listReviews(t, 1, 5),
    ]).then(([s, r]) => {
      setStats(s);
      setRecent(r.reviews);
    });
    // System Health metrics are admin-only (GET /api/metrics/system); reviewers
    // would get a 403, so only fetch them for admins.
    const metrics = isAdmin
      ? getSystemMetrics(t).then(setSysMetrics)
      : Promise.resolve();
    return Promise.all([core, metrics]);
  }

  useEffect(() => {
    if (!token) return;
    fetchAll(token)
      .catch((e) => toast.error(String(e)))
      .finally(() => setLoading(false));
    const id = setInterval(() => {
      fetchAll(token).catch(() => {});
    }, 30_000);
    return () => clearInterval(id);
  }, [token, isAdmin]);

  // SSE: real-time review completion notifications.
  const handleSSEEvent = useCallback((event: SSEEvent) => {
    if (event.type === "review_complete" && token) {
      const d = event.data as { owner: string; repo: string; number: number; score: number; status: string };
      toast.success(`Review complete: ${d.owner}/${d.repo} #${d.number}`, {
        description: `Score: ${d.score}/100 · ${d.status}`,
      });
      // Refresh recent reviews to show the new one.
      fetchAll(token);
    }
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  useSSE(token, handleSSEEvent);

  useKeyboardShortcuts({ onShowHelp: () => setShortcutsOpen(true) });

  if (loading) return (
    <div className="space-y-4" aria-busy="true" aria-label="Loading dashboard">
      <Skeleton className="h-32 w-full" />
      <Skeleton className="h-32 w-full" />
      <Skeleton className="h-64 w-full" />
    </div>
  );

  const errorRate = sysMetrics && sysMetrics.webhook_total_24h > 0
    ? ((sysMetrics.webhook_errors_24h / sysMetrics.webhook_total_24h) * 100).toFixed(1)
    : "0.0";

  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      {token && isAdmin && <OnboardingChecklist token={token} />}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Total Reviews" value={stats?.total_reviews ?? 0} />
        <StatCard title="Avg Score" value={`${Math.round(stats?.avg_score ?? 0)}/100`} />
        <StatCard title="Approvals" value={stats?.approvals ?? 0} />
        <StatCard title="Changes Requested" value={stats?.request_changes ?? 0} />
        <StatCard title="Comments" value={stats?.comments ?? 0} />
        <StatCard title="Enabled Repos" value={`${stats?.enabled_repos ?? 0} / ${stats?.total_repos ?? 0}`} />
      </div>

      {sysMetrics && (
        <Card>
          <CardHeader><CardTitle>System Health</CardTitle></CardHeader>
          <CardContent>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <HealthMetric label="Queue Depth" value={String(sysMetrics.queue_depth)} />
              <HealthMetric label="Reviews Today" value={String(sysMetrics.reviews_today)} />
              <HealthMetric label="Reviews This Week" value={String(sysMetrics.reviews_week)} />
              <HealthMetric label="Reviews This Month" value={String(sysMetrics.reviews_month)} />
              <HealthMetric label="Latency p50" value={fmtMs(sysMetrics.latency_p50_ms)} />
              <HealthMetric label="Latency p95" value={fmtMs(sysMetrics.latency_p95_ms)} />
              <HealthMetric label="Latency p99" value={fmtMs(sysMetrics.latency_p99_ms)} />
              <HealthMetric
                label="Webhook Errors (24h)"
                value={`${sysMetrics.webhook_errors_24h} / ${sysMetrics.webhook_total_24h} (${errorRate}%)`}
                highlight={parseFloat(errorRate) > 5}
              />
            </div>
            <p className="mt-3 text-xs text-muted-foreground" aria-live="polite">
              Auto-refreshes every 30 s
            </p>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader><CardTitle>Recent Reviews</CardTitle></CardHeader>
        <CardContent>
          {recent.length === 0 ? (
            <p className="text-muted-foreground text-sm">No reviews yet.</p>
          ) : (
            <ul className="divide-y" role="list" aria-label="Recent reviews">
              {recent.map((r) => (
                <li key={r.ID} className="flex items-center justify-between py-3">
                  <div>
                    <p className="text-sm font-medium">Review #{r.ID}</p>
                    <p className="text-xs text-muted-foreground">
                      <time dateTime={r.CreatedAt}>{new Date(r.CreatedAt).toLocaleString()}</time>
                    </p>
                  </div>
                  <div className="flex items-center gap-3">
                    <span className="text-sm font-mono" aria-label={`Score: ${r.Score} out of 100`}>
                      {r.Score}/100
                    </span>
                    <Badge variant={statusVariant(r.Status)}>{r.Status}</Badge>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <KeyboardShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
    </div>
  );
}

function StatCard({ title, value }: { title: string; value: string | number }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium text-muted-foreground">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-3xl font-bold" aria-label={`${title}: ${value}`}>{value}</p>
      </CardContent>
    </Card>
  );
}

function HealthMetric({ label, value, highlight }: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className="space-y-1">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p
        className={`text-lg font-semibold ${highlight ? "text-destructive" : ""}`}
        aria-label={`${label}: ${value}`}
      >
        {value}
      </p>
    </div>
  );
}
