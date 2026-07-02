"use client";

import { useEffect, useState, use } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { getReview, type ReviewDetail } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";

function severityVariant(s: string): "default" | "destructive" | "secondary" {
  if (s === "error") return "destructive";
  if (s === "warning") return "secondary";
  return "default";
}

function scoreColor(score: number): string {
  if (score >= 80) return "text-green-600 dark:text-green-400";
  if (score >= 60) return "text-yellow-600 dark:text-yellow-400";
  return "text-destructive";
}

export default function ReviewDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { token } = useToken();
  const router = useRouter();
  const [review, setReview] = useState<ReviewDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!token) return;
    getReview(token, Number(id)).then(setReview).finally(() => setLoading(false));
  }, [token, id]);

  if (loading) return <Skeleton className="h-64 w-full" />;
  if (!review) return <p className="text-muted-foreground">Review not found.</p>;

  const byFile = (review.Comments ?? []).reduce<Record<string, typeof review.Comments>>((acc, c) => {
    (acc[c.Path] ??= []).push(c);
    return acc;
  }, {});

  return (
    <div className="space-y-8 max-w-3xl">
      <div>
        <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>← Back</Button>
        <div className="flex items-center gap-3 flex-wrap">
          <h1 className="text-3xl font-bold">Review #{review.ID}</h1>
          <Badge variant={review.Status === "APPROVE" ? "default" : review.Status === "REQUEST_CHANGES" ? "destructive" : "secondary"}>
            {review.Status}
          </Badge>
          <span className={`font-mono font-semibold text-lg ${scoreColor(review.Score)}`}>{review.Score}/100</span>
        </div>
      </div>

      {review.Summary && (
        <Card>
          <CardHeader className="pb-3"><CardTitle className="text-lg">Summary</CardTitle></CardHeader>
          <CardContent><p className="text-base">{review.Summary}</p></CardContent>
        </Card>
      )}

      {Object.entries(byFile).map(([file, comments]) => (
        <Card key={file}>
          <CardHeader className="pb-3"><CardTitle className="font-mono text-sm">{file}</CardTitle></CardHeader>
          <CardContent>
            <ul className="space-y-4">
              {comments.map((c) => (
                <li key={c.ID} className="border-l-2 pl-3 border-muted">
                  <div className="flex items-center gap-2 mb-1.5">
                    <span className="text-sm text-muted-foreground">Line {c.Line}</span>
                    <Badge variant={severityVariant(c.Severity)} className="text-xs">{c.Severity}</Badge>
                  </div>
                  <p className="text-base">{c.Body}</p>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      ))}

      {(review.Comments ?? []).length === 0 && (
        <p className="text-base text-muted-foreground">No comments on this review.</p>
      )}
    </div>
  );
}
