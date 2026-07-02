"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useToken } from "@/hooks/useToken";
import { listReviews, downloadReviewsCSV, downloadReviewsPDF, type ReviewList } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { ClipboardList } from "lucide-react";

function statusVariant(s: string): "default" | "destructive" | "secondary" {
  if (s === "APPROVE") return "default";
  if (s === "REQUEST_CHANGES") return "destructive";
  return "secondary";
}

function scoreColor(score: number): string {
  if (score >= 80) return "text-green-600 dark:text-green-400";
  if (score >= 60) return "text-yellow-600 dark:text-yellow-400";
  return "text-destructive";
}

export default function ReviewsPage() {
  const { token } = useToken();
  const [data, setData] = useState<ReviewList | null>(null);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [exportingCSV, setExportingCSV] = useState(false);
  const [exportingPDF, setExportingPDF] = useState(false);

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    listReviews(token, page).then(setData).finally(() => setLoading(false));
  }, [token, page]);

  if (loading) return <Skeleton className="h-64 w-full" />;

  const reviews = data?.reviews ?? [];
  const totalPages = Math.ceil((data?.total ?? 0) / (data?.per_page ?? 20));

  async function handleExportCSV() {
    if (!token) return;
    setExportingCSV(true);
    try {
      await downloadReviewsCSV(token);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "CSV export failed");
    } finally {
      setExportingCSV(false);
    }
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
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Reviews</h1>
        <div className="flex gap-2">
          <Button variant="outline" size="lg" onClick={handleExportCSV} disabled={exportingCSV}>{exportingCSV ? "Exporting…" : "Export CSV"}</Button>
          <Button variant="outline" size="lg" onClick={handleExportPDF} disabled={exportingPDF}>
            {exportingPDF ? "Generating…" : "Export PDF"}
          </Button>
        </div>
      </div>

      {reviews.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            <ClipboardList className="h-10 w-10 mx-auto mb-3 opacity-30" />
            <p className="text-base font-medium text-foreground">No reviews yet.</p>
            <p className="text-base mt-1">Reviews will appear here once the bot processes a pull request.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-base">
            <thead className="border-b bg-muted/50">
              <tr>
                <th className="px-5 py-3.5 text-left font-medium">ID</th>
                <th className="px-5 py-3.5 text-left font-medium">Status</th>
                <th className="px-5 py-3.5 text-left font-medium">Score</th>
                <th className="px-5 py-3.5 text-left font-medium">Date</th>
                <th className="px-5 py-3.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {reviews.map((r) => (
                <tr key={r.ID} className="hover:bg-muted/30 transition-colors">
                  <td className="px-5 py-4 font-mono">#{r.ID}</td>
                  <td className="px-5 py-4"><Badge variant={statusVariant(r.Status)}>{r.Status}</Badge></td>
                  <td className="px-5 py-4">
                    <span className={`font-mono font-semibold ${scoreColor(r.Score)}`}>{r.Score}/100</span>
                  </td>
                  <td className="px-5 py-4 text-muted-foreground">{new Date(r.CreatedAt).toLocaleString()}</td>
                  <td className="px-5 py-4 text-right">
                    <Link href={`/reviews/${r.ID}`}><Button variant="outline">View</Button></Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="flex items-center gap-2">
          <Button variant="outline" disabled={page === 1} onClick={() => setPage(p => p - 1)}>Previous</Button>
          <span className="px-3 py-2 text-sm text-muted-foreground">Page {page} of {totalPages}</span>
          <Button variant="outline" disabled={page === totalPages} onClick={() => setPage(p => p + 1)}>Next</Button>
        </div>
      )}
    </div>
  );
}
