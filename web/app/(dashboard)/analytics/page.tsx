"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { getAnalytics, getCostAnalytics, type AnalyticsData, type CostAnalytics } from "@/lib/api";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, LineChart, Line } from "recharts";
import { BarChart2, TrendingUp } from "lucide-react";

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
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Analytics</h1>
        <div className="flex gap-2">
          {RANGES.map((d) => (
            <Button key={d} variant={days === d ? "default" : "outline"} className="cursor-pointer" onClick={() => setDays(d)}>
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
            <CardHeader><CardTitle className="text-lg">Daily Review Volume</CardTitle></CardHeader>
            <CardContent>
              {(data?.series?.length ?? 0) === 0 ? (
                <div className="py-16 flex flex-col items-center gap-3 text-muted-foreground">
                  <BarChart2 className="h-10 w-10 opacity-30" />
                  <p className="text-base">No reviews in the last {days} days yet.</p>
                </div>
              ) : (
                <ResponsiveContainer width="100%" height={240}>
                  <BarChart data={data?.series ?? []}>
                    <XAxis dataKey="date" tick={{ fontSize: 13 }} />
                    <YAxis tick={{ fontSize: 13 }} />
                    <Tooltip
                      cursor={false}
                      contentStyle={{ background: "var(--popover)", border: "1px solid var(--border)", borderRadius: 8 }}
                      labelStyle={{ color: "var(--popover-foreground)" }}
                      itemStyle={{ color: "var(--popover-foreground)" }}
                    />
                    <Bar dataKey="count" fill="var(--primary)" radius={[4, 4, 0, 0]} cursor="pointer" />
                  </BarChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader><CardTitle className="text-lg">Average Score Trend</CardTitle></CardHeader>
            <CardContent>
              {(data?.series?.length ?? 0) === 0 ? (
                <div className="py-16 flex flex-col items-center gap-3 text-muted-foreground">
                  <TrendingUp className="h-10 w-10 opacity-30" />
                  <p className="text-base">No score data in the last {days} days yet.</p>
                </div>
              ) : (
                <ResponsiveContainer width="100%" height={240}>
                  <LineChart data={data?.series ?? []}>
                    <XAxis dataKey="date" tick={{ fontSize: 13 }} />
                    <YAxis domain={[0, 100]} tick={{ fontSize: 13 }} />
                    <Tooltip
                      contentStyle={{ background: "var(--popover)", border: "1px solid var(--border)", borderRadius: 8 }}
                      labelStyle={{ color: "var(--popover-foreground)" }}
                      itemStyle={{ color: "var(--popover-foreground)" }}
                    />
                    <Line type="monotone" dataKey="avg_score" stroke="var(--primary)" strokeWidth={2} dot={false} cursor="pointer" />
                  </LineChart>
                </ResponsiveContainer>
              )}
            </CardContent>
          </Card>

          {cost && (
            <Card>
              <CardHeader><CardTitle className="text-lg">AI Cost Tracker</CardTitle></CardHeader>
              <CardContent className="space-y-4">
                <div className="grid gap-4 sm:grid-cols-3">
                  {[
                    { label: "Input Tokens",   value: fmtNum(cost.input_tokens) },
                    { label: "Output Tokens",  value: fmtNum(cost.output_tokens) },
                    { label: "Estimated Cost", value: `$${cost.est_cost_usd.toFixed(2)}` },
                  ].map((stat) => (
                    <div key={stat.label} className="rounded-lg border p-4">
                      <p className="text-sm text-muted-foreground">{stat.label}</p>
                      <p className="text-2xl font-bold mt-1">{stat.value}</p>
                    </div>
                  ))}
                </div>
                {cost.by_repo.length > 0 && (
                  <div className="rounded-md border">
                    <table className="w-full text-base">
                      <thead className="border-b bg-muted/50">
                        <tr>
                          <th className="px-5 py-4 text-left font-medium">Repository</th>
                          <th className="px-5 py-4 text-right font-medium">Input</th>
                          <th className="px-5 py-4 text-right font-medium">Output</th>
                          <th className="px-5 py-4 text-right font-medium">Est. Cost</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y">
                        {cost.by_repo.map((r) => (
                          <tr key={r.repo}>
                            <td className="px-5 py-4 font-mono text-sm">{r.repo}</td>
                            <td className="px-5 py-4 text-right">{fmtNum(r.input_tokens)}</td>
                            <td className="px-5 py-4 text-right">{fmtNum(r.output_tokens)}</td>
                            <td className="px-5 py-4 text-right">${r.est_cost_usd.toFixed(2)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
                <p className="text-sm text-muted-foreground">
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
