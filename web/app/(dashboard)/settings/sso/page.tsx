"use client";

import { useEffect, useState } from "react";
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

const DEFAULT_ROLE_MAPPING = JSON.stringify({ admin: "admin", developer: "reviewer" }, null, 2);

export default function SSOPage() {
  const { token } = useToken();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [existing, setExisting] = useState<SSOConfig | null>(null);
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

  useEffect(() => {
    if (!token) return;
    getSSOConfig(token)
      .then((cfg) => {
        setExisting(cfg);
        setIssuer(cfg.Issuer ?? "");
        setClientId(cfg.ClientID ?? "");
        setRedirectUrl(cfg.RedirectURL ?? "");
        setRoleMappingJson(JSON.stringify(cfg.RoleMapping ?? {}, null, 2));
        setEnforced(cfg.Enforced ?? false);
        setEnabled(cfg.Enabled ?? true);
      })
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
      // Refresh to get ID etc.
      const updated = await getSSOConfig(token);
      setExisting(updated);
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
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Single Sign-On</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Configure OIDC SSO for Okta, Azure AD, Google Workspace, or any OIDC-compliant IdP.
          </p>
        </div>
        {existing && (
          <Badge variant={existing.Enabled ? "default" : "secondary"}>
            <ShieldCheck className="h-3 w-3 mr-1" />
            {existing.Enabled ? "Active" : "Disabled"}
          </Badge>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>OIDC configuration</CardTitle>
          <CardDescription>
            Your IdP must support OIDC discovery at{" "}
            <code className="text-xs">{"{issuer}"}/.well-known/openid-configuration</code>.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="sso-issuer">Issuer URL</Label>
            <Input
              id="sso-issuer"
              placeholder="https://accounts.google.com"
              value={issuer}
              onChange={(e) => setIssuer(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="sso-client-id">Client ID</Label>
            <Input
              id="sso-client-id"
              placeholder="your-client-id"
              value={clientId}
              onChange={(e) => setClientId(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="sso-client-secret">
              Client secret{" "}
              {existing && (
                <span className="text-xs font-normal text-muted-foreground">
                  (leave blank to keep existing)
                </span>
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
          <div>
            <Label htmlFor="sso-redirect">
              Redirect URL{" "}
              <span className="text-xs font-normal text-muted-foreground">(optional — auto-derived from host)</span>
            </Label>
            <Input
              id="sso-redirect"
              placeholder="https://your-domain.com/auth/oidc/callback"
              value={redirectUrl}
              onChange={(e) => setRedirectUrl(e.target.value)}
            />
          </div>

          <div>
            <Label htmlFor="sso-role-mapping">
              Role mapping{" "}
              <span className="text-xs font-normal text-muted-foreground">
                (IdP group → platform role: admin or reviewer)
              </span>
            </Label>
            <Textarea
              id="sso-role-mapping"
              rows={5}
              className="font-mono text-xs mt-1"
              value={roleMappingJson}
              onChange={(e) => setRoleMappingJson(e.target.value)}
            />
            {roleMappingError && (
              <p className="text-xs text-destructive mt-1">{roleMappingError}</p>
            )}
          </div>

          <div className="flex items-center gap-3">
            <Switch id="sso-enabled" checked={enabled} onCheckedChange={setEnabled} />
            <Label htmlFor="sso-enabled">SSO enabled</Label>
          </div>

          <div className="flex items-center gap-3">
            <Switch id="sso-enforced" checked={enforced} onCheckedChange={setEnforced} />
            <Label htmlFor="sso-enforced">
              Enforce SSO{" "}
              <span className="text-xs font-normal text-muted-foreground">
                (blocks GitHub OAuth login when enabled)
              </span>
            </Label>
          </div>

          <div className="flex gap-3 pt-2">
            <Button onClick={handleSave} disabled={saving || !issuer || !clientId}>
              {saving ? "Saving…" : "Save configuration"}
            </Button>
            {existing && (
              <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
                Remove SSO
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {existing && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Login URL</CardTitle>
            <CardDescription>Share this link with users to initiate SSO login.</CardDescription>
          </CardHeader>
          <CardContent>
            <code className="text-sm bg-muted rounded px-3 py-2 block">
              {typeof window !== "undefined" ? window.location.origin : ""}/auth/oidc
            </code>
          </CardContent>
        </Card>
      )}

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove SSO configuration?</DialogTitle>
            <DialogDescription>
              Users will fall back to GitHub OAuth. This cannot be undone from this dialog.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>Cancel</Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? "Removing…" : "Remove"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
