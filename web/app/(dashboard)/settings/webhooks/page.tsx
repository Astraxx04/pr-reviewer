"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { listWebhookDeliveries, type WebhookDeliveryList } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";

function statusVariant(s: string): "default" | "destructive" | "secondary" | "outline" {
  if (s === "enqueued") return "default";
  if (s === "failed") return "destructive";
  if (s === "duplicate") return "outline";
  return "secondary";
}

export default function WebhooksPage() {
  const { token } = useToken();
  const [data, setData] = useState<WebhookDeliveryList | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    listWebhookDeliveries(token, page, 50).then(setData).finally(() => setLoading(false));
  }, [token, page]);

  if (loading) return <Skeleton className="h-64 w-full" />;

  const deliveries = data?.deliveries ?? [];
  const totalPages = Math.ceil((data?.total ?? 0) / (data?.per_page ?? 50));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Webhook Delivery Log</h1>
        <p className="text-sm text-muted-foreground">Retained for 7 days</p>
      </div>

      {deliveries.length === 0 ? (
        <p className="text-muted-foreground">No webhook deliveries recorded yet.</p>
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/50">
              <tr>
                <th className="px-4 py-3 text-left font-medium">Timestamp</th>
                <th className="px-4 py-3 text-left font-medium">Event</th>
                <th className="px-4 py-3 text-left font-medium">Repository</th>
                <th className="px-4 py-3 text-left font-medium">PR</th>
                <th className="px-4 py-3 text-left font-medium">Action</th>
                <th className="px-4 py-3 text-left font-medium">Status</th>
                <th className="px-4 py-3 text-left font-medium">Delivery ID</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {deliveries.map((d) => (
                <tr key={d.DeliveryID}>
                  <td className="px-4 py-3 text-muted-foreground whitespace-nowrap">
                    {new Date(d.ProcessedAt).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">{d.EventType || "—"}</td>
                  <td className="px-4 py-3 font-mono text-xs">
                    {d.Owner && d.Repo ? `${d.Owner}/${d.Repo}` : "—"}
                  </td>
                  <td className="px-4 py-3">{d.PRNumber > 0 ? `#${d.PRNumber}` : "—"}</td>
                  <td className="px-4 py-3 font-mono text-xs">{d.Action || "—"}</td>
                  <td className="px-4 py-3">
                    <Badge variant={statusVariant(d.Status)}>{d.Status || "—"}</Badge>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs truncate max-w-[120px]" title={d.DeliveryID}>
                    {d.DeliveryID}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="flex gap-2">
          <Button variant="outline" size="sm" disabled={page === 1} onClick={() => setPage((p) => p - 1)}>
            Previous
          </Button>
          <span className="px-3 py-2 text-sm text-muted-foreground">Page {page} of {totalPages}</span>
          <Button variant="outline" size="sm" disabled={page === totalPages} onClick={() => setPage((p) => p + 1)}>
            Next
          </Button>
        </div>
      )}
    </div>
  );
}
