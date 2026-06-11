"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import {
  getJiraConfig,
  putJiraConfig,
  deleteJiraConfig,
  testJiraConfig,
  type JiraConfigStatus,
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
import { FlaskConical, Trash2, CheckCircle2 } from "lucide-react";

export default function JiraIntegrationPage() {
  const { token } = useToken();
  const [loading, setLoading] = useState(true);
  const [existing, setExisting] = useState<JiraConfigStatus | null>(null);
  const [baseUrl, setBaseUrl] = useState("");
  const [email, setEmail] = useState("");
  const [apiToken, setApiToken] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ display_name?: string; email?: string } | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!token) return;
    getJiraConfig(token)
      .then((cfg) => {
        setExisting(cfg);
        setBaseUrl(cfg.base_url ?? "");
        setEmail(cfg.email ?? "");
        setEnabled(cfg.enabled ?? true);
      })
      .catch(() => {
        // 404 = not yet configured, that's fine
      })
      .finally(() => setLoading(false));
  }, [token]);

  async function handleSave() {
    if (!token) return;
    if (!baseUrl || !email) {
      toast.error("Base URL and email are required");
      return;
    }
    if (!existing && !apiToken) {
      toast.error("API token is required for the first setup");
      return;
    }
    setSaving(true);
    try {
      await putJiraConfig(token, {
        base_url: baseUrl,
        email,
        api_token: apiToken || undefined,
        enabled,
      });
      toast.success("Jira integration saved");
      // Refresh state
      const updated = await getJiraConfig(token);
      setExisting(updated);
      setApiToken("");
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
      const result = await testJiraConfig(token);
      if (result.ok) {
        setTestResult({ display_name: result.display_name, email: result.email });
        toast.success(
          result.display_name
            ? `Connected to Jira as ${result.display_name}`
            : "Connection successful — Jira credentials are valid",
        );
      } else {
        setTestResult(null);
        toast.error(`Connection failed: ${result.error}`);
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
      await deleteJiraConfig(token);
      setExisting(null);
      setBaseUrl(""); setEmail(""); setApiToken(""); setEnabled(true);
      toast.success("Jira integration removed");
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
          <h1 className="text-2xl font-bold">Jira Integration</h1>
          <p className="text-muted-foreground text-sm mt-1">
            When a PR title or description references a Jira ticket (e.g.{" "}
            <code className="text-xs bg-muted px-1 rounded">PROJ-123</code>), the ticket
            summary and status are automatically included in the AI review prompt.
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
          <CardTitle className="text-base">Step-by-step setup</CardTitle>
          <CardDescription>For Jira Cloud (an <code className="text-xs">…atlassian.net</code> site). Atlassian authenticates with your email + an API token.</CardDescription>
        </CardHeader>
        <CardContent className="text-sm">
          <ol className="list-decimal space-y-4 pl-5">
            <li>
              <span className="font-medium">Get your Jira base URL.</span> It&apos;s your Atlassian site root —
              e.g. <code className="text-xs bg-muted px-1 rounded">https://yourcompany.atlassian.net</code> (everything
              before <code className="text-xs">/jira</code>). Paste it into <strong>Jira base URL</strong> below.
            </li>
            <li>
              <span className="font-medium">Use your account email.</span> The email you sign in to Jira with —
              paste it into <strong>Jira account email</strong>.
            </li>
            <li>
              <span className="font-medium">Create an API token.</span> Go to{" "}
              <a href="https://id.atlassian.com/manage-profile/security/api-tokens" target="_blank" rel="noopener noreferrer" className="underline">
                id.atlassian.com → Security → API tokens
              </a>{" "}
              → <strong>Create API token</strong>, label it (e.g. &ldquo;PR Reviewer&rdquo;), copy it, and paste into{" "}
              <strong>API token</strong>. Use a token, <strong>not</strong> your password — Atlassian Cloud REST auth is email + token.
            </li>
            <li>
              <span className="font-medium">Save, then test.</span> Click <strong>Save</strong>, then{" "}
              <strong>Test connection</strong> — it calls Jira&apos;s <code className="text-xs">/myself</code> and shows which
              account it authenticated as, confirming the credentials work.
            </li>
            <li>
              <span className="font-medium">Reference tickets in PRs.</span> Put an issue key like{" "}
              <code className="text-xs bg-muted px-1 rounded">PROJ-123</code> in a PR&apos;s title or description.
              On the next review the bot fetches that ticket and feeds its summary into the AI prompt — no other action needed.
            </li>
          </ol>
          <p className="mt-3 text-xs text-muted-foreground">
            Read-only access is enough — the integration only reads issues (<code className="text-xs">GET /rest/api/3/issue</code>).
            A token scoped to an account that can view the relevant projects is all you need.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Jira credentials</CardTitle>
          <CardDescription>
            Use a{" "}
            <a
              href="https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/"
              target="_blank"
              rel="noopener noreferrer"
              className="underline"
            >
              Jira API token
            </a>{" "}
            (not your password). The token is stored encrypted.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="jira-url">Jira base URL</Label>
            <Input
              id="jira-url"
              placeholder="https://yourcompany.atlassian.net"
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="jira-email">Jira account email</Label>
            <Input
              id="jira-email"
              type="email"
              placeholder="you@yourcompany.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          <div>
            <Label htmlFor="jira-token">
              API token{" "}
              {existing && (
                <span className="text-xs font-normal text-muted-foreground">
                  (leave blank to keep existing)
                </span>
              )}
            </Label>
            <Input
              id="jira-token"
              type="password"
              placeholder={existing ? "••••••••" : "your-jira-api-token"}
              value={apiToken}
              onChange={(e) => setApiToken(e.target.value)}
            />
          </div>

          <div className="flex items-center gap-3">
            <Switch id="jira-enabled" checked={enabled} onCheckedChange={setEnabled} />
            <Label htmlFor="jira-enabled">Enable Jira context injection</Label>
          </div>

          <div className="flex flex-wrap gap-2 pt-2">
            <Button onClick={handleSave} disabled={saving}>
              {saving ? "Saving…" : "Save"}
            </Button>
            {existing && (
              <Button
                variant="outline"
                onClick={handleTest}
                disabled={testing}
              >
                <FlaskConical className="h-4 w-4 mr-2" />
                {testing ? "Testing…" : "Test connection"}
              </Button>
            )}
            {existing && (
              <Button
                variant="ghost"
                className="text-destructive ml-auto"
                onClick={() => setDeleteOpen(true)}
              >
                <Trash2 className="h-4 w-4 mr-2" />
                Remove
              </Button>
            )}
          </div>

          {testResult && (
            <div className="rounded-md border bg-muted/40 p-3 text-sm space-y-1">
              <div className="flex items-center gap-1.5 font-medium text-green-600">
                <CheckCircle2 className="h-4 w-4" /> Connected to Jira
              </div>
              {testResult.display_name && <div>Account: <span className="font-medium">{testResult.display_name}</span></div>}
              {testResult.email && <div className="text-muted-foreground text-xs">{testResult.email}</div>}
            </div>
          )}
        </CardContent>
      </Card>

      {existing && (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">How ticket detection works</CardTitle>
          </CardHeader>
          <CardContent className="text-sm text-muted-foreground space-y-2">
            <p>
              When a PR is opened or updated, the reviewer scans the title and description for
              Jira issue keys — <strong>your</strong> project key, a hyphen, then the issue number,
              like <code className="bg-muted px-1 rounded text-xs">PROJ-123</code> or{" "}
              <code className="bg-muted px-1 rounded text-xs">ENG-4567</code>. The key is 2–10
              characters, starts with an uppercase letter, and may contain digits — &ldquo;PROJ&rdquo;/&ldquo;ENG&rdquo;
              are just examples, use whatever your Jira projects use.
            </p>
            <p>
              Up to 3 matching tickets are fetched and their summary, type, and status are added
              to the review prompt as context. The AI is instructed to use this only for context
              and not to repeat ticket details verbatim in comments.
            </p>
          </CardContent>
        </Card>
      )}

      <Dialog open={deleteOpen} onOpenChange={setDeleteOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove Jira integration?</DialogTitle>
            <DialogDescription>
              Ticket context will no longer be injected into reviews. This cannot be undone.
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
