"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import {
  getSlackApp,
  putSlackApp,
  deleteSlackApp,
  testSlackApp,
  type SlackAppStatus,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog";
import { toast } from "sonner";
import { FlaskConical, Trash2, CheckCircle2, XCircle, ExternalLink, Copy } from "lucide-react";

function CopyableURL({ url }: { url: string }) {
  return (
    <span className="mt-1 flex items-center gap-2">
      <code className="flex-1 bg-muted px-2 py-1 rounded text-xs break-all">{url}</code>
      <Button
        size="icon"
        variant="outline"
        className="h-7 w-7 shrink-0"
        onClick={() => navigator.clipboard.writeText(url).then(() => toast.success("Copied to clipboard"))}
        aria-label="Copy URL"
      >
        <Copy className="h-3.5 w-3.5" />
      </Button>
    </span>
  );
}

function StatusRow({ label, ok, okText, badText }: { label: string; ok?: boolean; okText: string; badText: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-muted-foreground">{label}</span>
      <span className={`flex items-center gap-1.5 ${ok ? "text-green-600" : "text-muted-foreground"}`}>
        {ok ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
        {ok ? okText : badText}
      </span>
    </div>
  );
}

export default function SlackIntegrationPage() {
  const { token } = useToken();
  const [loading, setLoading] = useState(true);
  const [existing, setExisting] = useState<SlackAppStatus | null>(null);
  const [signingSecret, setSigningSecret] = useState("");
  const [botToken, setBotToken] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ team?: string; user?: string; url?: string } | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  // The server's own public base URL (SERVER_URL) is what Slack must reach.
  const [serverBase, setServerBase] = useState<string>(process.env.NEXT_PUBLIC_API_URL ?? "");

  useEffect(() => {
    if (!token) return;
    getSlackApp(token)
      .then((cfg) => {
        if (cfg.server_url) setServerBase(cfg.server_url);
        if (cfg.configured) {
          setExisting(cfg);
          setEnabled(cfg.enabled ?? true);
        }
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [token]);

  async function handleSave() {
    if (!token) return;
    if (!existing && !signingSecret) {
      toast.error("Signing secret is required for the first setup");
      return;
    }
    setSaving(true);
    try {
      await putSlackApp(token, {
        signing_secret: signingSecret || undefined,
        bot_token: botToken || undefined,
        enabled,
      });
      toast.success("Slack bot saved");
      const updated = await getSlackApp(token);
      if (updated.configured) setExisting(updated);
      setSigningSecret("");
      setBotToken("");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleTest() {
    if (!token) return;
    setTesting(true);
    try {
      const result = await testSlackApp(token);
      if (result.ok) {
        setTestResult({ team: result.team, user: result.user, url: result.url });
        toast.success(
          result.team
            ? `Connected to ${result.team}${result.user ? ` as @${result.user}` : ""}`
            : "Bot token is valid",
        );
      } else {
        setTestResult(null);
        toast.error(`Test failed: ${result.error}`);
      }
    } catch (err) {
      setTestResult(null);
      toast.error(err instanceof Error ? err.message : "Test failed");
    } finally {
      setTesting(false);
    }
  }

  async function handleDelete() {
    if (!token) return;
    setDeleting(true);
    try {
      await deleteSlackApp(token);
      setExisting(null);
      setSigningSecret(""); setBotToken(""); setEnabled(true);
      toast.success("Slack bot removed");
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
          <h1 className="text-2xl font-bold">Slack Bot</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Trigger and check PR reviews from Slack. See how the commands work below.
          </p>
        </div>
        {existing && (
          <Badge variant={existing.enabled ? "default" : "secondary"}>
            {existing.enabled ? "Active" : "Disabled"}
          </Badge>
        )}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">How to use it</CardTitle>
          <CardDescription>
            Every command takes a pull-request reference written as{" "}
            <code className="text-xs bg-muted px-1 rounded">owner/repo#number</code> — the GitHub owner
            (org or username), the repository name, and the PR number. For the PR at{" "}
            <code className="text-xs">github.com/acme/api/pull/42</code> you&apos;d write{" "}
            <code className="text-xs bg-muted px-1 rounded">acme/api#42</code>.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <div>
            <code className="text-xs bg-muted px-1.5 py-0.5 rounded">/review acme/api#42</code>
            <p className="text-muted-foreground mt-1">
              Runs a fresh AI review of that PR. The bot replies that it&apos;s queued, then posts the
              review as comments on the GitHub PR and a summary back into this Slack channel.
            </p>
          </div>
          <div>
            <code className="text-xs bg-muted px-1.5 py-0.5 rounded">/review-status acme/api#42</code>
            <p className="text-muted-foreground mt-1">
              Shows the most recent review (score + summary) for that PR — without starting a new one.
            </p>
          </div>
          <div>
            <code className="text-xs bg-muted px-1.5 py-0.5 rounded">@PR Reviewer acme/api#42</code>
            <p className="text-muted-foreground mt-1">
              Mentioning the bot (in a channel it&apos;s been invited to) with a PR reference re-reviews
              that PR. Invite it first with <code className="text-xs">/invite @PR Reviewer</code>.
            </p>
          </div>
          <p className="text-xs text-muted-foreground border-t pt-2">
            Typing <code className="text-xs">/review</code> shows the slug under it (e.g. &ldquo;pr-reviewer&rdquo;) — that&apos;s
            your Slack app, not an argument. If you see <strong>several</strong> <code className="text-xs">/review</code> entries, the command
            was registered more than once (or another app uses the same name) — see the setup notes below to fix it.
          </p>
        </CardContent>
      </Card>

      {existing && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Connection details</CardTitle>
            <CardDescription>Current Slack bot configuration. Run a test to verify it against Slack.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <StatusRow label="Bot" ok={existing.enabled} okText="Enabled" badText="Disabled" />
            <StatusRow label="Signing secret" ok={existing.has_signing_key} okText="Set" badText="Missing" />
            <StatusRow label="Bot token" ok={existing.has_bot_token} okText="Set" badText="Not set (slash commands only)" />
            {existing.updated_at && (
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Last updated</span>
                <span>{new Date(existing.updated_at).toLocaleString()}</span>
              </div>
            )}
            {testResult ? (
              <div className="mt-2 rounded-md border bg-muted/40 p-3 space-y-1">
                <div className="flex items-center gap-1.5 font-medium text-green-600">
                  <CheckCircle2 className="h-4 w-4" /> Verified with Slack
                </div>
                {testResult.team && <div>Workspace: <span className="font-medium">{testResult.team}</span></div>}
                {testResult.user && <div>Bot user: <span className="font-mono">@{testResult.user}</span></div>}
                {testResult.url && (
                  <a href={testResult.url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-xs underline">
                    {testResult.url} <ExternalLink className="h-3 w-3" />
                  </a>
                )}
              </div>
            ) : (
              existing.has_bot_token && (
                <p className="text-xs text-muted-foreground pt-1">
                  Not yet verified this session — click <strong>Test bot token</strong> below to confirm the bot can reach Slack.
                </p>
              )
            )}
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Slack App credentials</CardTitle>
          <CardDescription>
            Create a{" "}
            <a href="https://api.slack.com/apps" target="_blank" rel="noopener noreferrer" className="underline">
              Slack App
            </a>
            , add slash commands and the <code className="text-xs bg-muted px-1 rounded">app_mention</code>{" "}
            event subscription, then paste the signing secret and bot token below. Both are stored encrypted.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="slack-signing">
              Signing secret{" "}
              {existing && (
                <span className="text-xs font-normal text-muted-foreground">(leave blank to keep existing)</span>
              )}
            </Label>
            <Input
              id="slack-signing"
              type="password"
              placeholder={existing?.has_signing_key ? "••••••••" : "from Basic Information → App Credentials"}
              value={signingSecret}
              onChange={(e) => setSigningSecret(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="slack-bot">
              Bot token (xoxb-){" "}
              {existing && (
                <span className="text-xs font-normal text-muted-foreground">(leave blank to keep existing)</span>
              )}
            </Label>
            <Input
              id="slack-bot"
              type="password"
              placeholder={existing?.has_bot_token ? "••••••••" : "xoxb-..."}
              value={botToken}
              onChange={(e) => setBotToken(e.target.value)}
            />
            <p className="text-xs text-muted-foreground mt-1">
              Required for @mention replies. Slash commands work without it.
            </p>
          </div>

          <div className="flex items-center gap-3">
            <Switch id="slack-enabled" checked={enabled} onCheckedChange={setEnabled} />
            <Label htmlFor="slack-enabled">Enable Slack bot</Label>
          </div>

          <div className="flex flex-wrap gap-2 pt-2">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? "Saving…" : "Save"}
            </Button>
            {existing && existing.has_bot_token && (
              <Button variant="outline" onClick={handleTest} disabled={testing}>
                <FlaskConical className="h-4 w-4 mr-2" />
                {testing ? "Testing…" : "Test bot token"}
              </Button>
            )}
            {existing && (
              <Button variant="ghost" className="text-destructive ml-auto" onClick={() => setDeleteOpen(true)}>
                <Trash2 className="h-4 w-4 mr-2" />
                Remove
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Step-by-step setup</CardTitle>
          <CardDescription>
            Do these in the Slack App dashboard. The request URLs below use this server&apos;s configured
            public URL (<code className="text-xs">SERVER_URL</code>).
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm">
          {(!serverBase || serverBase.includes("localhost") || serverBase.includes("127.0.0.1")) && (
            <div className="mb-4 rounded-md border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-700 dark:text-amber-400">
              <strong>Slack can&apos;t reach this URL.</strong> It points at{" "}
              <code>{serverBase || "(unset)"}</code>, which is only reachable on this machine. Slack needs a
              public HTTPS URL — set the server&apos;s <code>SERVER_URL</code> env var to your public address
              (e.g. your ngrok URL <code>https://…ngrok-free.dev</code>) and restart, then the URLs below update
              automatically. Use that same host when pasting into Slack.
            </div>
          )}
          <ol className="list-decimal space-y-4 pl-5">
            <li>
              <span className="font-medium">Create the app.</span> Go to{" "}
              <a href="https://api.slack.com/apps" target="_blank" rel="noopener noreferrer" className="underline">api.slack.com/apps</a>{" "}
              → <strong>Create New App</strong> → <strong>From scratch</strong>. Name it (e.g. &ldquo;PR Reviewer&rdquo;) and pick your workspace.
            </li>
            <li>
              <span className="font-medium">Copy the signing secret.</span> <strong>Basic Information → App Credentials → Signing Secret</strong> →
              paste it into <strong>Signing secret</strong> above and click <strong>Save</strong>. (Slack uses it to verify the request URLs in the next steps.)
            </li>
            <li>
              <span className="font-medium">Add bot scopes &amp; install.</span> <strong>OAuth &amp; Permissions → Scopes → Bot Token Scopes</strong>, add:
              <span className="mt-1 flex flex-wrap gap-1.5">
                <code className="bg-muted px-1.5 py-0.5 rounded text-xs">commands</code>
                <code className="bg-muted px-1.5 py-0.5 rounded text-xs">app_mentions:read</code>
                <code className="bg-muted px-1.5 py-0.5 rounded text-xs">chat:write</code>
              </span>
              <span className="mt-1 block">
                Then <strong>Install to Workspace</strong> → <strong>Allow</strong>, copy the{" "}
                <strong>Bot User OAuth Token</strong> (<code className="text-xs">xoxb-…</code>) into <strong>Bot token</strong> above, and <strong>Save</strong>.
              </span>
            </li>
            <li>
              <span className="font-medium">Add slash commands.</span> <strong>Slash Commands → Create New Command</strong>. Make two,
              both with this Request URL:
              <CopyableURL url={`${serverBase}/slack/commands`} />
              <span className="mt-1 block text-muted-foreground text-xs">
                Set Command to <code>/review</code> (description e.g. &ldquo;Review a PR&rdquo;, usage hint{" "}
                <code>owner/repo#number</code>), then create a second one named <code>/review-status</code>.
                Create each command <strong>once</strong> — a command name must be unique within the app, and registering it
                twice (or having another installed app use <code>/review</code>) is what produces duplicate suggestions while
                typing. To fix duplicates, delete the extra entries on this Slash Commands page.
              </span>
            </li>
            <li>
              <span className="font-medium">Enable event subscriptions.</span> <strong>Event Subscriptions</strong> → toggle <strong>On</strong> → set Request URL:
              <CopyableURL url={`${serverBase}/slack/events`} />
              <span className="mt-1 block text-muted-foreground text-xs">
                Slack verifies this instantly (the signing secret must be saved here first). Under{" "}
                <strong>Subscribe to bot events</strong>, add <code>app_mention</code>, then <strong>Save Changes</strong>.
              </span>
            </li>
            <li>
              <span className="font-medium">Reinstall if prompted.</span> Slack asks you to reinstall after scope/event changes — do it, then come back here and click{" "}
              <strong>Test bot token</strong> to confirm the connection.
            </li>
          </ol>
        </CardContent>
      </Card>

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove Slack bot?</DialogTitle>
            <DialogDescription>
              Slash commands and mentions will stop working. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteOpen(false)}>Cancel</Button>
            <Button variant="destructive" onClick={handleDelete} disabled={deleting}>
              {deleting ? "Removing…" : "Remove"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
