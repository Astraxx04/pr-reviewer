"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useToken } from "@/hooks/useToken";
import { listReviews, exportReviewsCSVUrl, downloadReviewsPDF, type ReviewList } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";

function statusVariant(s: string): "default" | "destructive" | "secondary" {
  if (s === "APPROVE") return "default";
  if (s === "REQUEST_CHANGES") return "destructive";
  return "secondary";
}

export default function ReviewsPage() {
  const { token } = useToken();
  const [data, setData] = useState<ReviewList | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [exportingPDF, setExportingPDF] = useState(false);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    listReviews(token, page).then(setData).finally(() => setLoading(false));
  }, [token, page]);

  if (loading) return <Skeleton className="h-64 w-full" />;

  const reviews = data?.reviews ?? [];
  const totalPages = Math.ceil((data?.total ?? 0) / (data?.per_page ?? 20));

  function handleExport() {
    const url = exportReviewsCSVUrl(token ?? "");
    window.open(url, "_blank");
  }

  async function handleExportPDF() {
    if (!token) return;
    setExportingPDF(true);
    try {
      await downloadReviewsPDF(token);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "PDF export failed");
    } finally {
      setExportingPDF(false);
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Reviews</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={handleExport} style={{ cursor: 'pointer' }}>Export CSV</Button>
          <Button variant="outline" size="sm" onClick={handleExportPDF} disabled={exportingPDF} style={{ cursor: 'pointer' }}>
            {exportingPDF ? "Generating…" : "Export PDF"}
          </Button>
        </div>
      </div>
      <div className="rounded-lg border">
        <table className="w-full text-sm">
          <thead className="border-b bg-muted/50">
            <tr>
              <th className="px-4 py-3 text-left font-medium">ID</th>
              <th className="px-4 py-3 text-left font-medium">Status</th>
              <th className="px-4 py-3 text-left font-medium">Score</th>
              <th className="px-4 py-3 text-left font-medium">Date</th>
              <th className="px-4 py-3 text-right font-medium">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {reviews.map((r) => (
              <tr key={r.ID}>
                <td className="px-4 py-3 font-mono">#{r.ID}</td>
                <td className="px-4 py-3"><Badge variant={statusVariant(r.Status)}>{r.Status}</Badge></td>
                <td className="px-4 py-3">{r.Score}/100</td>
                <td className="px-4 py-3 text-muted-foreground">{new Date(r.CreatedAt).toLocaleString()}</td>
                <td className="px-4 py-3 text-right">
                  <Link href={`/reviews/${r.ID}`}><Button variant="outline" size="sm">View</Button></Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {totalPages > 1 && (
        <div className="flex gap-2">
          <Button variant="outline" size="sm" disabled={page === 1} onClick={() => setPage(p => p - 1)}>Previous</Button>
          <span className="px-3 py-2 text-sm text-muted-foreground">Page {page} of {totalPages}</span>
          <Button variant="outline" size="sm" disabled={page === totalPages} onClick={() => setPage(p => p + 1)}>Next</Button>
        </div>
      )}
    </div>
  );
}
