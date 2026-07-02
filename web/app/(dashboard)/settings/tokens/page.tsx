"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import {
  listAPITokens,
  createAPIToken,
  revokeAPIToken,
  type APIToken,
  type CreatedAPIToken,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { Plus, Trash2, Copy, KeyRound, Eye, Pencil } from "lucide-react";

const EXPIRY_PRESETS = [
  { label: "30 days", days: 30 },
  { label: "90 days", days: 90 },
  { label: "1 year",  days: 365 },
  { label: "No expiry", days: 0 },
] as const;

function addDays(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() + n);
  return d.toISOString().slice(0, 10);
}

export default function APITokensPage() {
  const { token } = useToken();
  const router = useRouter();
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [scope, setScope] = useState<"read" | "readwrite">("read");
  const [expiresAt, setExpiresAt] = useState("");
  const [saving, setSaving] = useState(false);
  const [newToken, setNewToken] = useState<CreatedAPIToken | null>(null);
  const [confirmRevokeId, setConfirmRevokeId] = useState<number | null>(null);
  const [revoking, setRevoking] = useState(false);

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

  async function handleRevoke() {
    if (!token || confirmRevokeId == null) return;
    setRevoking(true);
    try {
      await revokeAPIToken(token, confirmRevokeId);
      setTokens((ts) => ts.filter((t) => t.ID !== confirmRevokeId));
      setConfirmRevokeId(null);
      toast.success("Token revoked");
    } catch {
      toast.error("Revoke failed");
    } finally {
      setRevoking(false);
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text).then(() => toast.success("Copied to clipboard"));
  }

  return (
    <div className="space-y-8">
      <div className="flex items-start justify-between">
        <div>
          <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>← Back</Button>
          <h1 className="text-3xl font-bold">API Tokens</h1>
          <p className="text-base text-muted-foreground mt-1">
            Generate long-lived tokens for CLI and automation. Raw values are shown once.
          </p>
        </div>
        <Button size="lg" onClick={() => setCreateOpen(true)}>
          <Plus className="h-5 w-5 mr-2" />
          Generate token
        </Button>
      </div>

      {loading ? (
        <div className="space-y-3">
          {[0, 1].map((i) => <Skeleton key={i} className="h-16 w-full" />)}
        </div>
      ) : tokens.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            <KeyRound className="h-10 w-10 mx-auto mb-3 opacity-30" />
            <p className="text-base font-medium text-foreground">No API tokens yet.</p>
            <p className="text-base mt-1">Generate a token to use with the CLI or external integrations.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-base">
            <thead className="border-b bg-muted/50">
              <tr>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Name</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Scope</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Prefix</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Last used</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Expires</th>
                <th scope="col" className="px-5 py-3.5 text-left font-medium">Created</th>
                <th scope="col" className="px-5 py-3.5"></th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {tokens.map((t) => (
                <tr key={t.ID} className="hover:bg-muted/30 transition-colors">
                  <td className="px-5 py-4 font-medium">{t.Name}</td>
                  <td className="px-5 py-4">
                    <Badge variant={t.Scope === "readwrite" ? "default" : "secondary"} className="text-xs">
                      {t.Scope}
                    </Badge>
                  </td>
                  <td className="px-5 py-4 font-mono text-sm text-muted-foreground">{t.Prefix}…</td>
                  <td className="px-5 py-4 text-muted-foreground text-sm">
                    {t.LastUsedAt ? new Date(t.LastUsedAt).toLocaleDateString() : "Never"}
                  </td>
                  <td className="px-5 py-4 text-muted-foreground text-sm">
                    {t.ExpiresAt ? new Date(t.ExpiresAt).toLocaleDateString() : "Never"}
                  </td>
                  <td className="px-5 py-4 text-muted-foreground text-sm">
                    <time dateTime={t.CreatedAt}>{new Date(t.CreatedAt).toLocaleDateString()}</time>
                  </td>
                  <td className="px-5 py-4 text-right">
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive cursor-pointer"
                      onClick={() => setConfirmRevokeId(t.ID)}
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
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-xl">Generate API token</DialogTitle>
            <DialogDescription>Create a token for CLI access or external integrations.</DialogDescription>
          </DialogHeader>
          <div className="space-y-5 py-2">
            <div className="space-y-1.5">
              <Label className="text-sm">Token name</Label>
              <Input
                placeholder="e.g. CI pipeline, local dev"
                value={name}
                onChange={(e) => setName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && handleCreate()}
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-sm">Scope</Label>
              <div className="grid grid-cols-2 gap-2">
                {([
                  { value: "read",      icon: <Eye className="h-5 w-5" />,    label: "Read-only",    desc: "GET endpoints only" },
                  { value: "readwrite", icon: <Pencil className="h-5 w-5" />, label: "Read & write", desc: "Trigger reviews & manage settings" },
                ] as const).map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    onClick={() => setScope(opt.value)}
                    className={`flex flex-col items-center gap-1.5 rounded-lg border p-3 text-center transition-colors cursor-pointer ${
                      scope === opt.value
                        ? "border-primary bg-primary/5 text-primary"
                        : "border-border text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground"
                    }`}
                  >
                    {opt.icon}
                    <span className="text-sm font-medium">{opt.label}</span>
                    <span className="text-xs">{opt.desc}</span>
                  </button>
                ))}
              </div>
            </div>

            <div className="space-y-1.5">
              <Label className="text-sm">Expiry <span className="text-muted-foreground font-normal">(optional)</span></Label>
              <div className="flex flex-wrap gap-2">
                {EXPIRY_PRESETS.map((p) => {
                  const val = p.days === 0 ? "" : addDays(p.days);
                  const selected = expiresAt === val;
                  return (
                    <button
                      key={p.label}
                      type="button"
                      onClick={() => setExpiresAt(val)}
                      className={`rounded-full px-3 py-1 text-sm border transition-colors cursor-pointer ${
                        selected
                          ? "bg-primary text-primary-foreground border-primary"
                          : "bg-muted text-muted-foreground border-transparent hover:border-border hover:text-foreground"
                      }`}
                    >
                      {p.label}
                    </button>
                  );
                })}
                {expiresAt !== "" && !EXPIRY_PRESETS.some((p) => expiresAt === (p.days === 0 ? "" : addDays(p.days))) && (
                  <span className="rounded-full px-3 py-1 text-sm border bg-primary text-primary-foreground border-primary">
                    Custom
                  </span>
                )}
              </div>
              <Input
                type="date"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="ghost" size="lg" onClick={() => setCreateOpen(false)}>Cancel</Button>
            <Button size="lg" onClick={handleCreate} disabled={saving || !name.trim()}>
              {saving ? "Generating…" : "Generate"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* New token reveal dialog */}
      <Dialog open={!!newToken} onOpenChange={() => setNewToken(null)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-xl">Token created — copy it now</DialogTitle>
            <DialogDescription>This is the only time the raw token will be shown. Store it somewhere safe.</DialogDescription>
          </DialogHeader>
          <div className="py-2">
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-muted rounded-md px-3 py-2.5 text-sm font-mono break-all select-all">
                {newToken?.token}
              </code>
              <Button
                size="icon"
                variant="outline"
                className="cursor-pointer shrink-0"
                onClick={() => newToken && copyToClipboard(newToken.token)}
                aria-label="Copy token"
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button size="lg" onClick={() => setNewToken(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Confirm revoke dialog */}
      <Dialog open={confirmRevokeId != null} onOpenChange={(o) => { if (!o) setConfirmRevokeId(null); }}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="text-xl">Revoke token?</DialogTitle>
            <DialogDescription className="text-base">
              <strong>{tokens.find((t) => t.ID === confirmRevokeId)?.Name || "This token"}</strong> will
              be permanently revoked. Any integrations using it will stop working immediately.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" size="lg" onClick={() => setConfirmRevokeId(null)}>Cancel</Button>
            <Button variant="destructive" size="lg" onClick={handleRevoke} disabled={revoking}>
              {revoking ? "Revoking…" : "Revoke"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
