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
import { useRouter } from "next/navigation";
import { CheckCircle2, XCircle, FlaskConical, Pencil, Trash2 } from "lucide-react";

const emptyForm = { app_id: "", private_key: "", webhook_secret: "" };

export default function GithubAppPage() {
  const { token } = useToken();
  const router = useRouter();
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
    <div className="space-y-8 max-w-2xl">
      <div>
        <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>
          ← Back
        </Button>
        <h1 className="text-3xl font-bold">GitHub App</h1>
        <p className="text-base text-muted-foreground mt-1">
          Configure your GitHub App credentials. These are stored encrypted in the database —
          no env vars needed after initial setup.
        </p>
      </div>

      {/* Status / current values */}
      <Card>
        <CardHeader className="">
          <CardTitle className="text-lg flex items-center justify-between gap-2">
            <span className="flex items-center gap-2">
              Status
              {configured ? (
                <Badge variant="secondary" className="text-green-600 text-sm">
                  <CheckCircle2 className="h-3.5 w-3.5 mr-1" />
                  Configured (App ID: {status?.app_id})
                </Badge>
              ) : (
                <Badge variant="outline" className="text-muted-foreground text-sm">
                  <XCircle className="h-3.5 w-3.5 mr-1" />
                  Not configured
                </Badge>
              )}
            </span>
            {configured && !editing && (
              <Button variant="outline" size="sm" onClick={startEdit}>
                <Pencil className="h-4 w-4 mr-1" />
                Edit
              </Button>
            )}
          </CardTitle>
        </CardHeader>
        {configured && (
          <CardContent className="pt-0 space-y-3">
            <div className="flex gap-4 text-base text-muted-foreground">
              <span className="flex items-center gap-1.5">
                {status?.has_webhook_secret ? <CheckCircle2 className="h-4 w-4 text-green-500" /> : <XCircle className="h-4 w-4" />}
                Webhook secret
              </span>
            </div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={handleTest} disabled={testing}>
                <FlaskConical className="h-4 w-4 mr-1.5" />
                {testing ? "Testing…" : "Test credentials"}
              </Button>
              <Button
                variant="outline"
                className="text-destructive hover:text-destructive"
                onClick={() => setConfirmDelete(true)}
                disabled={deleting}
              >
                <Trash2 className="h-4 w-4 mr-1.5" />
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
            <CardTitle className="text-lg">{editing ? "Edit credentials" : "Setup guide"}</CardTitle>
            <CardDescription className="text-sm">
              {editing
                ? "Update any field below and save. Leave secrets blank to keep the stored value."
                : "Create a GitHub App under your organization and fill in the fields below."}
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            {!editing && (
              <ol className="list-decimal pl-5 space-y-2.5 text-sm text-muted-foreground">
                <li>
                  On GitHub, open your <strong>organization page</strong> → <strong>Settings</strong> → <strong>Developer settings</strong> → <strong>GitHub Apps</strong> → <strong>New GitHub App</strong>
                </li>
                <li>
                  Under <strong>Webhook</strong>, enable it and set the URL to:
                  <div className="mt-1 rounded bg-muted px-3 py-1.5 font-mono text-xs">
                    {process.env.NEXT_PUBLIC_API_URL}/webhooks
                  </div>
                </li>
                <li>
                  Generate a random string for <strong>Webhook secret</strong>, paste it into GitHub, and also paste it in the field below
                </li>
                <li>
                  Under <strong>Permissions</strong>, grant:
                  <ul className="list-disc pl-4 mt-1.5 space-y-1">
                    <li><strong>Pull requests</strong> — Read &amp; write</li>
                    <li><strong>Contents</strong> — Read-only</li>
                    <li><strong>Members</strong> — Read-only (for org membership checks)</li>
                    <li><strong>Metadata</strong> — Read-only (mandatory)</li>
                  </ul>
                </li>
                <li>
                  Under <strong>Subscribe to events</strong>, check:
                  <ul className="list-disc pl-4 mt-1.5 space-y-1">
                    <li><strong>Pull request</strong> — triggers reviews on open, sync, ready for review</li>
                    <li><strong>Pull request review comment</strong> — enables replies to bot comments</li>
                    <li><strong>Installation target</strong> — registers the installation in your database</li>
                  </ul>
                </li>
                <li>
                  Set <strong>Where can this GitHub App be installed?</strong> to <strong>Only on this account</strong>
                </li>
                <li>
                  Click <strong>Create GitHub App</strong>, then note the <strong>App ID</strong> and generate a <strong>private key</strong> (downloads a <code className="text-xs bg-muted px-1 rounded">.pem</code> file)
                </li>
                <li>
                  Install the app on your organization: open the app page → <strong>Install App</strong> → select your org → grant access to the repositories you want reviewed
                </li>
              </ol>
            )}

            <div className="space-y-4">
              <div className="space-y-1.5">
                <Label className="text-sm">App ID</Label>
                <Input
                  type="number"
                  placeholder="123456"
                  value={form.app_id}
                  onChange={(e) => setForm({ ...form, app_id: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-sm">Private key (PEM)</Label>
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
              <div className="space-y-1.5">
                <Label className="text-sm">
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
              <p className="text-sm text-muted-foreground">
                All secrets are encrypted with AES-256-GCM before being stored. They are never exposed after saving.
              </p>
              <div className="flex gap-2">
                <Button size="lg" onClick={handleSave} disabled={saving || !form.app_id || (!configured && !form.private_key.trim())}>
                  {saving ? "Saving…" : "Save credentials"}
                </Button>
                {editing && (
                  <Button variant="ghost" size="lg" onClick={cancelEdit} disabled={saving}>
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
            <DialogTitle className="text-xl">Delete GitHub App credentials?</DialogTitle>
            <DialogDescription className="text-base">
              This permanently removes the stored App ID, private key, and webhook secret.
              Incoming webhooks will be rejected until you configure new credentials. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" size="lg" onClick={() => setConfirmDelete(false)} disabled={deleting}>
              Cancel
            </Button>
            <Button variant="destructive" size="lg" onClick={handleDelete} disabled={deleting}>
              {deleting ? "Deleting…" : "Delete credentials"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
