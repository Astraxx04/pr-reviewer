"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { listAuditLogs, auditExportCSVUrl, type AuditLogEntry } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import { Download, Search, ClipboardList } from "lucide-react";

const ENTITY_TYPES = ["", "repo", "provider", "team_member", "user", "config", "sso", "api_token"];

function actionBadgeVariant(action: string): "default" | "secondary" | "destructive" | "outline" {
  if (action.includes("delete") || action.includes("disable") || action.includes("remove")) return "destructive";
  if (action.includes("create") || action.includes("enable") || action.includes("add")) return "default";
  return "secondary";
}

export default function AuditLogPage() {
  const { token } = useToken();
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [entityType, setEntityType] = useState("");
  const [actor, setActor] = useState("");
  const [since, setSince] = useState("");

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

  function handleSearch() {
    setPage(1);
  }

  function handleExport() {
    if (!token) return;
    window.open(auditExportCSVUrl() + `?token=${token}`, "_blank");
  }

  const totalPages = Math.max(1, Math.ceil(total / perPage));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Audit Log</h1>
          <p className="text-muted-foreground text-sm mt-1">
            All administrative actions with actor, timestamp, and change details.
          </p>
        </div>
        <Button variant="outline" onClick={handleExport}>
          <Download className="h-4 w-4 mr-2" />
          Export CSV
        </Button>
      </div>

      <div className="flex flex-wrap gap-3 items-end">
        <div className="flex-1 min-w-[160px]">
          <Label className="mb-1 block text-xs">Actor login</Label>
          <div className="flex gap-2">
            <Input
              placeholder="github-login"
              value={actor}
              onChange={(e) => setActor(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSearch()}
            />
          </div>
        </div>
        <div className="w-48">
          <Label className="mb-1 block text-xs">Entity type</Label>
          <Select value={entityType} onValueChange={(v) => { setEntityType(v && v !== "all" ? v : ""); setPage(1); }}>
            <SelectTrigger>
              <SelectValue placeholder="All types" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All types</SelectItem>
              {ENTITY_TYPES.filter(Boolean).map((t) => (
                <SelectItem key={t} value={t}>{t}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="w-44">
          <Label className="mb-1 block text-xs">Since</Label>
          <Input
            type="date"
            value={since}
            onChange={(e) => { setSince(e.target.value); setPage(1); }}
          />
        </div>
        <Button onClick={handleSearch} variant="secondary">
          <Search className="h-4 w-4 mr-1" />
          Search
        </Button>
      </div>

      {loading ? (
        <div className="space-y-2">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      ) : logs.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          <ClipboardList className="h-8 w-8 mx-auto mb-3 opacity-30" />
          <p>No audit log entries found.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-md border">
          <table className="w-full text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th scope="col" className="px-4 py-2 text-left font-medium">Time</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Actor</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Action</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Entity</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">ID</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">IP</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((entry) => (
                <tr key={entry.ID} className="border-t hover:bg-muted/30">
                  <td className="px-4 py-2 whitespace-nowrap text-muted-foreground">
                    <time dateTime={entry.CreatedAt}>
                      {new Date(entry.CreatedAt).toLocaleString()}
                    </time>
                  </td>
                  <td className="px-4 py-2 font-mono">{entry.ActorLogin}</td>
                  <td className="px-4 py-2">
                    <Badge variant={actionBadgeVariant(entry.Action)} className="text-xs font-mono">
                      {entry.Action}
                    </Badge>
                  </td>
                  <td className="px-4 py-2 text-muted-foreground">{entry.EntityType}</td>
                  <td className="px-4 py-2 font-mono text-xs text-muted-foreground">{entry.EntityID}</td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">{entry.IPAddress}</td>
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
