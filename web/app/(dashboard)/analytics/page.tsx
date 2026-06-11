"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { getAnalytics, getCostAnalytics, type AnalyticsData, type CostAnalytics } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, LineChart, Line } from "recharts";

const RANGES = [7, 14, 30, 90] as const;

function fmtNum(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
  return String(n);
}

export default function AnalyticsPage() {
  const { token, isAdmin } = useToken();
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [cost, setCost] = useState<CostAnalytics | null>(null);
  const [days, setDays] = useState<number>(30);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    // Cost analytics are admin-only (GET /api/analytics/cost); reviewers would
    // get a 403, so only fetch them for admins.
    const analytics = getAnalytics(token, days).then(setData);
    const costData = isAdmin
      ? getCostAnalytics(token, days).then(setCost)
      : Promise.resolve();
    Promise.all([analytics, costData])
      .catch((e) => toast.error(String(e)))
      .finally(() => setLoading(false));
  }, [token, days, isAdmin]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Analytics</h1>
        <div className="flex gap-2">
          {RANGES.map((d) => (
            <Button key={d} size="sm" variant={days === d ? "default" : "outline"} onClick={() => setDays(d)}>
              {d}d
            </Button>
          ))}
        </div>
      </div>

      {loading ? (
        <div className="space-y-4">
          <Skeleton className="h-64 w-full" />
          <Skeleton className="h-64 w-full" />
          <Skeleton className="h-32 w-full" />
        </div>
      ) : (
        <>
          <Card>
            <CardHeader><CardTitle>Daily Review Volume</CardTitle></CardHeader>
            <CardContent>
              {(data?.series?.length ?? 0) === 0 ? (
                <p className="py-16 text-center text-sm text-muted-foreground">
                  No reviews in the last {days} days yet.
                </p>
              ) : (
                <ResponsiveContainer width="100%" height={240}>
                  <BarChart data={data?.series ?? []}>
                    <XAxis dataKey="date" tick={{ fontSize: 12 }} />
                    <YAxis tick={{ fontSize: 12 }} />
                    <Tooltip
                      cursor={false}
                      contentStyle={{ background: "var(--popover)", border: "1px solid var(--border)", borderRadius: 8 }}
                      labelStyle={{ color: "var(--popover-foreground)" }}
                      itemStyle={{ color: "var(--popover-foreground)" }}
                    />
                    <Bar dataKey="count" fill="var(--primary)" radius={[4, 4, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle>Average Score Trend</CardTitle></CardHeader>
            <CardContent>
              {(data?.series?.length ?? 0) === 0 ? (
                <p className="py-16 text-center text-sm text-muted-foreground">
                  No score data in the last {days} days yet.
                </p>
              ) : (
                <ResponsiveContainer width="100%" height={240}>
                  <LineChart data={data?.series ?? []}>
                    <XAxis dataKey="date" tick={{ fontSize: 12 }} />
                    <YAxis domain={[0, 100]} tick={{ fontSize: 12 }} />
                    <Tooltip
                      contentStyle={{ background: "var(--popover)", border: "1px solid var(--border)", borderRadius: 8 }}
                      labelStyle={{ color: "var(--popover-foreground)" }}
                      itemStyle={{ color: "var(--popover-foreground)" }}
                    />
                    <Line type="monotone" dataKey="avg_score" stroke="var(--primary)" strokeWidth={2} dot={false} />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>

          {cost && (
            <Card>
              <CardHeader><CardTitle>AI Cost Tracker</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-4 sm:grid-cols-3">
                  <div>
                    <p className="text-xs text-muted-foreground">Input Tokens</p>
                    <p className="text-2xl font-bold">{fmtNum(cost.input_tokens)}</p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Output Tokens</p>
                    <p className="text-2xl font-bold">{fmtNum(cost.output_tokens)}</p>
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Estimated Cost</p>
                    <p className="text-2xl font-bold">${cost.est_cost_usd.toFixed(2)}</p>
                  </div>
                </div>
                {cost.by_repo.length > 0 && (
                  <div className="rounded-md border">
                    <table className="w-full text-sm">
                      <thead className="border-b bg-muted/50">
                        <tr>
                          <th className="px-4 py-2 text-left font-medium">Repository</th>
                          <th className="px-4 py-2 text-right font-medium">Input</th>
                          <th className="px-4 py-2 text-right font-medium">Output</th>
                          <th className="px-4 py-2 text-right font-medium">Est. Cost</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y">
                        {cost.by_repo.map((r) => (
                          <tr key={r.repo}>
                            <td className="px-4 py-2 font-mono text-xs">{r.repo}</td>
                            <td className="px-4 py-2 text-right">{fmtNum(r.input_tokens)}</td>
                            <td className="px-4 py-2 text-right">{fmtNum(r.output_tokens)}</td>
                            <td className="px-4 py-2 text-right">${r.est_cost_usd.toFixed(2)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                <p className="text-xs text-muted-foreground">
                  Cost estimates use rough average rates (~$3/M input, $15/M output). Actual costs vary by provider and model.
                </p>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
