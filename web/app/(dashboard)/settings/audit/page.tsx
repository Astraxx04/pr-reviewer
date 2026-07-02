"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { listAuditLogs, downloadAuditCSV, type AuditLogEntry } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { toast } from "sonner";
import { Download, ClipboardList } from "lucide-react";

const ENTITY_TYPES = ["repo", "provider", "user", "config", "sso", "api_token"];

function actionBadgeVariant(action: string): "default" | "secondary" | "destructive" | "outline" {
  if (action.includes("delete") || action.includes("disable") || action.includes("remove")) return "destructive";
  if (action.includes("create") || action.includes("enable") || action.includes("add")) return "default";
  return "secondary";
}

export default function AuditLogPage() {
  const { token } = useToken();
  const router = useRouter();
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [entityType, setEntityType] = useState("");
  const [actor, setActor] = useState("");
  const [since, setSince] = useState("");
  const [exportingCSV, setExportingCSV] = useState(false);

  const perPage = 50;

  useEffect(() => {
    if (!token) return;
    setLoading(true);
    listAuditLogs(token, { page, per_page: perPage, entity_type: entityType || undefined, actor: actor || undefined, since: since || undefined })
      .then((res) => {
        setLogs(res.logs ?? []);
        setTotal(res.total);
      })
      .catch(() => toast.error("Failed to load audit logs"))
      .finally(() => setLoading(false));
  }, [token, page, entityType, actor, since]);

  async function handleExport() {
    if (!token) return;
    setExportingCSV(true);
    try {
      await downloadAuditCSV(token);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "CSV export failed");
    } finally {
      setExportingCSV(false);
    }
  }

  const totalPages = Math.max(1, Math.ceil(total / perPage));

  return (
    <div className="space-y-8">
      <div className="flex items-start justify-between">
        <div>
          <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>← Back</Button>
          <h1 className="text-3xl font-bold">Audit Log</h1>
          <p className="text-base text-muted-foreground mt-1">
            All administrative actions with actor, timestamp, and change details.
          </p>
        </div>
        <Button size="lg" variant="outline" onClick={handleExport} disabled={exportingCSV}>
          <Download className="h-5 w-5 mr-2" />
          {exportingCSV ? "Exporting…" : "Export CSV"}
        </Button>
      </div>

      <div className="flex flex-wrap gap-3 items-end">
        <div className="flex-1 min-w-[160px] space-y-1.5">
          <Label className="text-sm">Actor login <span className="text-muted-foreground font-normal">(GitHub username)</span></Label>
          <Input
            placeholder="e.g. octocat"
            value={actor}
            onChange={(e) => { setActor(e.target.value); setPage(1); }}
          />
        </div>

        <div className="space-y-1.5">
          <Label className="text-sm">Entity type</Label>
          <div className="flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => { setEntityType(""); setPage(1); }}
              className={`rounded-full px-3 py-1 text-sm border transition-colors cursor-pointer ${
                entityType === ""
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-muted text-muted-foreground border-transparent hover:border-border hover:text-foreground"
              }`}
            >
              All
            </button>
            {ENTITY_TYPES.map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => { setEntityType(t); setPage(1); }}
                className={`rounded-full px-3 py-1 text-sm border transition-colors cursor-pointer ${
                  entityType === t
                    ? "bg-primary text-primary-foreground border-primary"
                    : "bg-muted text-muted-foreground border-transparent hover:border-border hover:text-foreground"
                }`}
              >
                {t}
              </button>
            ))}
          </div>
        </div>

        <div className="w-44 space-y-1.5">
          <Label className="text-sm">Since</Label>
          <Input
            type="date"
            value={since}
            onChange={(e) => { setSince(e.target.value); setPage(1); }}
          />
        </div>

      </div>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      ) : logs.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            <ClipboardList className="h-10 w-10 mx-auto mb-3 opacity-30" />
            <p className="text-base font-medium text-foreground">No audit log entries found.</p>
            <p className="text-base mt-1">Administrative actions will appear here once they occur.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="overflow-x-auto rounded-lg border">
          <table className="w-full text-base">
            <thead className="border-b bg-muted/50">
              <tr>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Time</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Actor</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Action</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Entity</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">ID</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">IP</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {logs.map((entry) => (
                <tr key={entry.ID} className="hover:bg-muted/30 transition-colors">
                  <td className="px-5 py-4 whitespace-nowrap text-muted-foreground">
                    <time dateTime={entry.CreatedAt}>
                      {new Date(entry.CreatedAt).toLocaleString()}
                    </time>
                  </td>
                  <td className="px-5 py-4 font-mono">{entry.ActorLogin}</td>
                  <td className="px-5 py-4">
                    <Badge variant={actionBadgeVariant(entry.Action)} className="text-xs font-mono">
                      {entry.Action}
                    </Badge>
                  </td>
                  <td className="px-5 py-4 text-muted-foreground">{entry.EntityType}</td>
                  <td className="px-5 py-4 font-mono text-sm text-muted-foreground">{entry.EntityID}</td>
                  <td className="px-5 py-4 text-sm text-muted-foreground">{entry.IPAddress}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>{total} entries</span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page === 1}
            >
              Previous
            </Button>
            <span className="px-2 py-1">
              {page} / {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page === totalPages}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
