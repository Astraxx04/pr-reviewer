"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { getSSOConfig, putSSOConfig, deleteSSOConfig, type SSOConfig } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { ShieldCheck } from "lucide-react";

const DEFAULT_ROLE_MAPPING = JSON.stringify({ owner: ["owners"], admin: ["admins"], reviewer: ["*"] }, null, 2);

type FormSnapshot = {
  issuer: string;
  clientId: string;
  redirectUrl: string;
  roleMappingJson: string;
  enforced: boolean;
  enabled: boolean;
};

function makeSnapshot(cfg: SSOConfig): FormSnapshot {
  return {
    issuer: cfg.issuer ?? "",
    clientId: cfg.client_id ?? "",
    redirectUrl: cfg.redirect_url ?? "",
    roleMappingJson: JSON.stringify(cfg.role_mapping ?? {}, null, 2),
    enforced: cfg.enforced ?? false,
    enabled: cfg.enabled ?? true,
  };
}

export default function SSOPage() {
  const { token } = useToken();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [existing, setExisting] = useState<SSOConfig | null>(null);
  const [savedSnapshot, setSavedSnapshot] = useState<FormSnapshot | null>(null);
  const [issuer, setIssuer] = useState("");
  const [clientId, setClientId] = useState("");
  const [clientSecret, setClientSecret] = useState("");
  const [redirectUrl, setRedirectUrl] = useState("");
  const [roleMappingJson, setRoleMappingJson] = useState(DEFAULT_ROLE_MAPPING);
  const [roleMappingError, setRoleMappingError] = useState("");
  const [enforced, setEnforced] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const isDirty =
    clientSecret !== "" ||
    issuer !== (savedSnapshot?.issuer ?? "") ||
    clientId !== (savedSnapshot?.clientId ?? "") ||
    redirectUrl !== (savedSnapshot?.redirectUrl ?? "") ||
    roleMappingJson !== (savedSnapshot?.roleMappingJson ?? DEFAULT_ROLE_MAPPING) ||
    enforced !== (savedSnapshot?.enforced ?? false) ||
    enabled !== (savedSnapshot?.enabled ?? true);

  function applyConfig(cfg: SSOConfig) {
    const snap = makeSnapshot(cfg);
    setExisting(cfg);
    setSavedSnapshot(snap);
    setIssuer(snap.issuer);
    setClientId(snap.clientId);
    setRedirectUrl(snap.redirectUrl);
    setRoleMappingJson(snap.roleMappingJson);
    setEnforced(snap.enforced);
    setEnabled(snap.enabled);
  }

  useEffect(() => {
    if (!token) return;
    getSSOConfig(token)
      .then(applyConfig)
      .catch(() => {
        // 404 means not configured yet — that's fine
      })
      .finally(() => setLoading(false));
  }, [token]);

  function validateRoleMapping(): Record<string, string> | null {
    try {
      const parsed = JSON.parse(roleMappingJson);
      setRoleMappingError("");
      return parsed;
    } catch {
      setRoleMappingError("Invalid JSON");
      return null;
    }
  }

  async function handleSave() {
    if (!token) return;
    const roleMapping = validateRoleMapping();
    if (roleMapping === null) return;
    setSaving(true);
    try {
      await putSSOConfig(token, {
        issuer,
        client_id: clientId,
        client_secret: clientSecret || undefined,
        redirect_url: redirectUrl || undefined,
        role_mapping: roleMapping,
        enforced,
        enabled,
      });
      toast.success("SSO configuration saved");
      const updated = await getSSOConfig(token);
      applyConfig(updated);
      setClientSecret("");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!token) return;
    setDeleting(true);
    try {
      await deleteSSOConfig(token);
      setExisting(null);
      setSavedSnapshot(null);
      setIssuer(""); setClientId(""); setClientSecret("");
      setRedirectUrl(""); setRoleMappingJson(DEFAULT_ROLE_MAPPING);
      setEnforced(false); setEnabled(true);
      toast.success("SSO configuration removed");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
      setDeleteOpen(false);
    }
  }

  if (loading) {
    return (
      <div className="space-y-4 max-w-2xl">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-8 max-w-2xl">
      <div className="flex items-start justify-between">
        <div>
          <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>← Back</Button>
          <h1 className="text-3xl font-bold">Single Sign-On</h1>
          <p className="text-base text-muted-foreground mt-1">
            Configure OIDC SSO for Okta, Azure AD, Google Workspace, or any OIDC-compliant IdP.
          </p>
        </div>
        {existing && (
          <Badge variant={existing.enabled ? "default" : "secondary"} className="mt-10 shrink-0">
            <ShieldCheck className="h-3.5 w-3.5 mr-1" />
            {existing.enabled ? "Active" : "Disabled"}
          </Badge>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">OIDC configuration</CardTitle>
          <CardDescription className="text-base">
            Your IdP must support OIDC discovery at{" "}
            <code className="text-xs">{"{issuer}"}/.well-known/openid-configuration</code>.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="space-y-1.5">
            <Label className="text-sm" htmlFor="sso-issuer">Issuer URL</Label>
            <Input
              id="sso-issuer"
              placeholder="https://accounts.google.com"
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-sm" htmlFor="sso-client-id">Client ID</Label>
            <Input
              id="sso-client-id"
              placeholder="your-client-id"
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-sm" htmlFor="sso-client-secret">
              Client secret{" "}
              {existing && (
                <span className="font-normal text-muted-foreground">(leave blank to keep existing)</span>
              )}
            </Label>
            <Input
              id="sso-client-secret"
              type="password"
              placeholder={existing ? "••••••••" : "your-client-secret"}
              value={clientSecret}
              onChange={(e) => setClientSecret(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-sm" htmlFor="sso-redirect">
              Redirect URL{" "}
              <span className="font-normal text-muted-foreground">(optional — auto-derived from host)</span>
            </Label>
            <Input
              id="sso-redirect"
              placeholder="https://your-domain.com/auth/oidc/callback"
              value={redirectUrl}
              onChange={(e) => setRedirectUrl(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label className="text-sm" htmlFor="sso-role-mapping">
              Role mapping{" "}
              <span className="font-normal text-muted-foreground">(IdP group → platform role: admin or reviewer)</span>
            </Label>
            <Textarea
              id="sso-role-mapping"
              rows={5}
              className="font-mono text-xs"
              value={roleMappingJson}
              onChange={(e) => setRoleMappingJson(e.target.value)}
            />
            {roleMappingError && (
              <p className="text-sm text-destructive">{roleMappingError}</p>
            )}
          </div>

          <div className="flex items-center justify-between rounded-lg border p-4">
            <div>
              <p className="text-base font-medium">SSO enabled</p>
              <p className="text-sm text-muted-foreground mt-0.5">Allow users to sign in via OIDC</p>
            </div>
            <Switch id="sso-enabled" checked={enabled} onCheckedChange={setEnabled} className="cursor-pointer" />
          </div>

          <div className="flex items-center justify-between rounded-lg border p-4">
            <div>
              <p className="text-base font-medium">Enforce SSO</p>
              <p className="text-sm text-muted-foreground mt-0.5">Blocks GitHub OAuth login when enabled</p>
            </div>
            <Switch id="sso-enforced" checked={enforced} onCheckedChange={setEnforced} className="cursor-pointer" />
          </div>

          <div className="flex gap-3 pt-1">
            <Button size="lg" onClick={handleSave} disabled={saving || !isDirty || !issuer || !clientId}>
              {saving ? "Saving…" : "Save configuration"}
            </Button>
            {existing && (
              <Button size="lg" variant="destructive" onClick={() => setDeleteOpen(true)}>
                Remove SSO
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {existing && (
        <Card>
          <CardHeader>
            <CardTitle className="text-lg">Login URL</CardTitle>
            <CardDescription className="text-base">Share this link with users to initiate SSO login.</CardDescription>
          </CardHeader>
          <CardContent>
            <code className="text-sm bg-muted rounded-lg px-4 py-3 block break-all">
              {process.env.NEXT_PUBLIC_API_URL ?? window.location.origin}/auth/oidc
            </code>
          </CardContent>
        </Card>
      )}

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="text-xl">Remove SSO configuration?</DialogTitle>
            <DialogDescription className="text-base">
              Users will fall back to GitHub OAuth. This cannot be undone from this dialog.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button size="lg" variant="ghost" onClick={() => setDeleteOpen(false)}>Cancel</Button>
            <Button size="lg" variant="destructive" onClick={handleDelete} disabled={deleting}>
              {deleting ? "Removing…" : "Remove"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
