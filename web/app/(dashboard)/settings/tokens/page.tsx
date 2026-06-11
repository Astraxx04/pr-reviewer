"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import {
  listAPITokens,
  createAPIToken,
  revokeAPIToken,
  type APIToken,
  type CreatedAPIToken,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "sonner";
import { Plus, Trash2, Copy, KeyRound } from "lucide-react";

export default function APITokensPage() {
  const { token } = useToken();
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [scope, setScope] = useState<"read" | "readwrite">("read");
  const [expiresAt, setExpiresAt] = useState("");
  const [saving, setSaving] = useState(false);
  const [newToken, setNewToken] = useState<CreatedAPIToken | null>(null);
  const [revoking, setRevoking] = useState<number | null>(null);

  useEffect(() => {
    if (!token) return;
    listAPITokens(token)
      .then(setTokens)
      .catch(() => toast.error("Failed to load API tokens"))
      .finally(() => setLoading(false));
  }, [token]);

  async function handleCreate() {
    if (!token || !name.trim()) return;
    setSaving(true);
    try {
      const created = await createAPIToken(token, {
        name: name.trim(),
        scope,
        expires_at: expiresAt || undefined,
      });
      setTokens((ts) => [...ts, created]);
      setNewToken(created);
      setCreateOpen(false);
      setName("");
      setScope("read");
      setExpiresAt("");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Create failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleRevoke(id: number) {
    if (!token) return;
    setRevoking(id);
    try {
      await revokeAPIToken(token, id);
      setTokens((ts) => ts.filter((t) => t.ID !== id));
      toast.success("Token revoked");
    } catch {
      toast.error("Revoke failed");
    } finally {
      setRevoking(null);
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => toast.success("Copied to clipboard"));
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">API Tokens</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Generate long-lived tokens for CLI and automation. Raw values are shown once.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4 mr-2" />
          Generate token
        </Button>
      </div>

      {loading ? (
        <div className="space-y-3">
          {[0, 1].map((i) => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      ) : tokens.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground border rounded-md">
          <KeyRound className="h-8 w-8 mx-auto mb-3 opacity-30" />
          <p>No API tokens yet.</p>
          <p className="text-sm mt-1">Generate a token to use with the CLI or external integrations.</p>
        </div>
      ) : (
        <div className="rounded-md border overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th scope="col" className="px-4 py-2 text-left font-medium">Name</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Scope</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Prefix</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Last used</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Expires</th>
                <th scope="col" className="px-4 py-2 text-left font-medium">Created</th>
                <th scope="col" className="px-4 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {tokens.map((t) => (
                <tr key={t.ID} className="border-t hover:bg-muted/30">
                  <td className="px-4 py-2 font-medium">{t.Name}</td>
                  <td className="px-4 py-2">
                    <Badge variant={t.Scope === "readwrite" ? "default" : "secondary"} className="text-xs">
                      {t.Scope}
                    </Badge>
                  </td>
                  <td className="px-4 py-2 font-mono text-xs text-muted-foreground">{t.Prefix}…</td>
                  <td className="px-4 py-2 text-muted-foreground text-xs">
                    {t.LastUsedAt ? new Date(t.LastUsedAt).toLocaleDateString() : "Never"}
                  </td>
                  <td className="px-4 py-2 text-muted-foreground text-xs">
                    {t.ExpiresAt ? new Date(t.ExpiresAt).toLocaleDateString() : "Never"}
                  </td>
                  <td className="px-4 py-2 text-muted-foreground text-xs">
                    <time dateTime={t.CreatedAt}>{new Date(t.CreatedAt).toLocaleDateString()}</time>
                  </td>
                  <td className="px-4 py-2 text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      onClick={() => handleRevoke(t.ID)}
                      disabled={revoking === t.ID}
                      aria-label={`Revoke token ${t.Name}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Create dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Generate API token</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div>
              <Label>Token name</Label>
              <Input
                placeholder="e.g. CI pipeline, local dev"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleCreate()}
              />
            </div>
            <div>
              <Label>Scope</Label>
              <Select value={scope} onValueChange={(v) => setScope(v as "read" | "readwrite")}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="read">Read-only</SelectItem>
                  <SelectItem value="readwrite">Read &amp; write</SelectItem>
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground mt-1">
                Read-only tokens can only access GET endpoints. Read &amp; write tokens can trigger re-reviews and manage settings.
              </p>
            </div>
            <div>
              <Label>Expiry date <span className="text-muted-foreground font-normal">(optional)</span></Label>
              <Input
                type="date"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={saving || !name.trim()}>
              {saving ? "Generating…" : "Generate"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* New token reveal dialog */}
      <Dialog open={!!newToken} onOpenChange={() => setNewToken(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Token created — copy it now</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              This is the only time the raw token will be shown. Store it somewhere safe.
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-muted rounded px-3 py-2 text-sm font-mono break-all select-all">
                {newToken?.token}
              </code>
              <Button
                size="icon"
                variant="outline"
                onClick={() => newToken && copyToClipboard(newToken.token)}
                aria-label="Copy token"
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setNewToken(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
