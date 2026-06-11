"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import { getGithubApp, putGithubApp, testGithubApp, deleteGithubApp, type GithubAppStatus } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { CheckCircle2, XCircle, FlaskConical, Pencil, Trash2 } from "lucide-react";

const emptyForm = { app_id: "", private_key: "", webhook_secret: "", github_token: "" };

export default function GithubAppPage() {
  const { token } = useToken();
  const [status, setStatus] = useState<GithubAppStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [editing, setEditing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [form, setForm] = useState(emptyForm);

  useEffect(() => {
    if (!token) return;
    getGithubApp(token).then(setStatus).finally(() => setLoading(false));
  }, [token]);

  const configured = !!status?.configured;

  function startEdit() {
    // Pre-fill the App ID; secrets stay blank and are only overwritten when re-entered.
    setForm({ ...emptyForm, app_id: status?.app_id ? String(status.app_id) : "" });
    setEditing(true);
  }

  function cancelEdit() {
    setForm(emptyForm);
    setEditing(false);
  }

  async function handleSave() {
    if (!token) return;
    const appId = Number(form.app_id);
    if (!appId) {
      toast.error("App ID is required");
      return;
    }
    // Private key is mandatory on first-time setup; when editing an existing
    // config it may be left blank to keep the stored key.
    if (!configured && !form.private_key.trim()) {
      toast.error("Private key is required");
      return;
    }
    setSaving(true);
    try {
      const updated = await putGithubApp(token, {
        app_id: appId,
        private_key: form.private_key.trim(),
        webhook_secret: form.webhook_secret.trim() || undefined,
        github_token: form.github_token.trim() || undefined,
      });
      setStatus(updated);
      setForm(emptyForm);
      setEditing(false);
      toast.success("GitHub App credentials saved");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!token) return;
    setDeleting(true);
    try {
      const updated = await deleteGithubApp(token);
      setStatus(updated);
      setForm(emptyForm);
      setEditing(false);
      setConfirmDelete(false);
      toast.success("GitHub App credentials deleted");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setDeleting(false);
    }
  }

  async function handleTest() {
    if (!token) return;
    setTesting(true);
    try {
      const res = await testGithubApp(token);
      toast[res.ok ? "success" : "error"](res.message);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setTesting(false);
    }
  }

  if (loading) return <Skeleton className="h-64 w-full" />;

  // Show the credentials form during first-time setup or when explicitly editing.
  const showForm = !configured || editing;

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold">GitHub App</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Configure your GitHub App credentials. These are stored encrypted in the database —
          no env vars needed after initial setup.
        </p>
      </div>

      {/* Status / current values */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center justify-between gap-2">
            <span className="flex items-center gap-2">
              Status
              {configured ? (
                <Badge variant="secondary" className="text-green-600">
                  <CheckCircle2 className="h-3 w-3 mr-1" />
                  Configured (App ID: {status?.app_id})
                </Badge>
              ) : (
                <Badge variant="outline" className="text-muted-foreground">
                  <XCircle className="h-3 w-3 mr-1" />
                  Not configured
                </Badge>
              )}
            </span>
            {configured && !editing && (
              <Button variant="outline" size="sm" onClick={startEdit}>
                <Pencil className="h-3 w-3 mr-1" />
                Edit
              </Button>
            )}
          </CardTitle>
        </CardHeader>
        {configured && (
          <CardContent className="pt-0 space-y-3">
            <div className="flex gap-4 text-sm text-muted-foreground">
              <span className="flex items-center gap-1">
                {status?.has_webhook_secret ? <CheckCircle2 className="h-3.5 w-3.5 text-green-500" /> : <XCircle className="h-3.5 w-3.5" />}
                Webhook secret
              </span>
              <span className="flex items-center gap-1">
                {status?.has_github_token ? <CheckCircle2 className="h-3.5 w-3.5 text-green-500" /> : <XCircle className="h-3.5 w-3.5" />}
                GitHub token
              </span>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" size="sm" onClick={handleTest} disabled={testing}>
                <FlaskConical className="h-3 w-3 mr-1" />
                {testing ? "Testing…" : "Test credentials"}
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="text-destructive hover:text-destructive"
                onClick={() => setConfirmDelete(true)}
                disabled={deleting}
              >
                <Trash2 className="h-3 w-3 mr-1" />
                Delete
              </Button>
            </div>
          </CardContent>
        )}
      </Card>

      {/* Credentials form — first-time setup or editing */}
      {showForm && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{editing ? "Edit credentials" : "Setup guide"}</CardTitle>
            <CardDescription>
              {editing
                ? "Update any field below and save. Leave secrets blank to keep the stored value."
                : "Create a GitHub App and fill in the fields below."}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {!editing && (
              <ol className="list-decimal list-inside space-y-2 text-sm text-muted-foreground">
                <li>Go to <strong>GitHub → Settings → Developer settings → GitHub Apps → New GitHub App</strong></li>
                <li>Set <strong>Webhook URL</strong> to <code className="bg-muted px-1 rounded text-xs">{"{YOUR_SERVER_URL}/webhooks"}</code></li>
                <li>Generate a random <strong>Webhook secret</strong> and paste it below <em>and</em> into the GitHub App settings</li>
                <li>
                  Grant permissions:
                  <ul className="list-disc list-inside ml-4 mt-1 space-y-0.5">
                    <li><strong>Pull requests</strong> — read &amp; write</li>
                    <li><strong>Contents</strong> — read</li>
                    <li><strong>Metadata</strong> — read (mandatory)</li>
                  </ul>
                </li>
                <li>
                  Subscribe to events:
                  <ul className="list-disc list-inside ml-4 mt-1 space-y-0.5">
                    <li><strong>Pull request</strong> — triggers reviews on open, update, ready for review</li>
                    <li><strong>Pull request review comment</strong> — enables replies to bot comments</li>
                    <li><strong>Installation target</strong> — registers the app installation in your database</li>
                  </ul>
                </li>
                <li>Note the <strong>App ID</strong> and generate a <strong>private key</strong></li>
                <li>Create a <strong>Personal Access Token</strong> at <code className="bg-muted px-1 rounded text-xs">github.com/settings/tokens</code> with <code className="bg-muted px-1 rounded text-xs">repo</code> + <code className="bg-muted px-1 rounded text-xs">read:org</code> scopes — paste below</li>
              </ol>
            )}

            <div className="space-y-4">
              <div className="space-y-1">
                <Label>App ID</Label>
                <Input
                  type="number"
                  placeholder="123456"
                  value={form.app_id}
                  onChange={(e) => setForm({ ...form, app_id: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>Private key (PEM)</Label>
                <Textarea
                  className="font-mono text-xs"
                  rows={8}
                  placeholder={editing
                    ? "leave blank to keep the stored private key"
                    : "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"}
                  value={form.private_key}
                  onChange={(e) => setForm({ ...form, private_key: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>
                  Webhook secret
                  <span className="text-muted-foreground font-normal ml-1 text-xs">— the random string you entered in GitHub App webhook settings</span>
                </Label>
                <Input
                  type="password"
                  placeholder="leave blank to keep existing value"
                  value={form.webhook_secret}
                  onChange={(e) => setForm({ ...form, webhook_secret: e.target.value })}
                />
              </div>
              <div className="space-y-1">
                <Label>
                  GitHub Personal Access Token
                  <span className="text-muted-foreground font-normal ml-1 text-xs">— used to fetch PR diffs and org data</span>
                </Label>
                <Input
                  type="password"
                  placeholder="ghp_… (leave blank to keep existing value)"
                  value={form.github_token}
                  onChange={(e) => setForm({ ...form, github_token: e.target.value })}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                All secrets are encrypted with AES-256-GCM before being stored. They are never exposed after saving.
              </p>
              <div className="flex gap-2">
                <Button onClick={handleSave} disabled={saving || !form.app_id || (!configured && !form.private_key.trim())}>
                  {saving ? "Saving…" : "Save credentials"}
                </Button>
                {editing && (
                  <Button variant="ghost" onClick={cancelEdit} disabled={saving}>
                    Cancel
                  </Button>
                )}
              </div>
            </div>
          </CardContent>
        </Card>
      )}

      <Dialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete GitHub App credentials?</DialogTitle>
            <DialogDescription>
              This permanently removes the stored App ID, private key, webhook secret, and
              GitHub token. Incoming webhooks will be rejected until you configure new
              credentials. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setConfirmDelete(false)} disabled={deleting}>
              Cancel
            </Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleting}>
              {deleting ? "Deleting…" : "Delete credentials"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
