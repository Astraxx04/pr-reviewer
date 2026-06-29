"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useToken } from "@/hooks/useToken";
import { listRepos, updateRepo, syncRepos, triggerRepoIndex, type Repo } from "@/lib/api";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { RefreshCw, Database } from "lucide-react";

function IndexingBadge({ status }: { status: Repo["IndexingStatus"] }) {
  const map: Record<Repo["IndexingStatus"], { label: string; className: string }> = {
    idle:     { label: "Not indexed",  className: "text-muted-foreground" },
    indexing: { label: "Indexing…",    className: "text-blue-500 animate-pulse" },
    indexed:  { label: "Indexed",      className: "text-green-600" },
    error:    { label: "Index error",  className: "text-destructive" },
  };
  const { label, className } = map[status] ?? map.idle;
  return <span className={`text-xs font-medium ${className}`}>{label}</span>;
}

export default function ReposPage() {
  const { token, isAdmin } = useToken();
  const [repos, setRepos] = useState<Repo[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [indexing, setIndexing] = useState<Record<number, boolean>>({});

  useEffect(() => {
    if (!token) return;
    listRepos(token).then(setRepos).finally(() => setLoading(false));
  }, [token]);

  async function toggle(repo: Repo) {
    if (!token) return;
    try {
      const updated = await updateRepo(token, repo.ID, { enabled: !repo.Enabled });
      setRepos((prev) => prev.map((r) => (r.ID === repo.ID ? updated : r)));
      toast.success(`${repo.Name} ${!repo.Enabled ? "enabled" : "disabled"}`);
    } catch (e) {
      toast.error(String(e));
    }
  }

  async function handleSync() {
    if (!token) return;
    setSyncing(true);
    try {
      const result = await syncRepos(token);
      const fresh = await listRepos(token);
      setRepos(fresh);
      toast.success(`Synced ${result.synced} repos — ${result.added} new`);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setSyncing(false);
    }
  }

  async function handleIndex(repo: Repo) {
    if (!token) return;
    setIndexing((prev) => ({ ...prev, [repo.ID]: true }));
    try {
      await triggerRepoIndex(token, repo.ID);
      setRepos((prev) =>
        prev.map((r) => (r.ID === repo.ID ? { ...r, IndexingStatus: "indexing" } : r))
      );
      toast.success(`Indexing started for ${repo.Owner}/${repo.Name}`);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setIndexing((prev) => ({ ...prev, [repo.ID]: false }));
    }
  }

  if (loading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Repositories</h1>
        {isAdmin && (
          <Button variant="outline" size="sm" onClick={handleSync} disabled={syncing}>
            <RefreshCw className={`h-4 w-4 mr-2 ${syncing ? "animate-spin" : ""}`} />
            {syncing ? "Syncing…" : "Sync from GitHub"}
          </Button>
        )}
      </div>

      {repos.length === 0 ? (
        isAdmin ? (
          <div className="rounded-lg border border-dashed p-8 text-muted-foreground">
            <p className="text-center font-medium text-foreground">No repositories connected</p>
            <p className="mt-1 text-center text-sm">
              Repositories appear here once the GitHub App is installed on your organisation
              and synced. Follow these steps:
            </p>
            <ol className="mx-auto mt-4 max-w-lg list-decimal space-y-2 pl-5 text-sm">
              <li>
                Make sure your App credentials (App ID + private key) are saved in{" "}
                <Link href="/settings/github-app" className="underline hover:text-foreground">
                  Settings → GitHub App
                </Link>
                .
              </li>
              <li>
                On GitHub, open{" "}
                <a
                  href="https://github.com/settings/apps"
                  target="_blank"
                  rel="noreferrer"
                  className="underline hover:text-foreground"
                >
                  Settings → Developer settings → GitHub Apps
                </a>
                , select your app, and click <strong>Install App</strong> in the sidebar.
              </li>
              <li>
                Choose your organisation (or personal account), grant access to{" "}
                <strong>All repositories</strong> or a selected set, and confirm the install.
              </li>
              <li>
                Come back to this page and click <strong>Sync from GitHub</strong> (top right) to
                pull in the repositories the App can access.
              </li>
            </ol>
          </div>
        ) : (
          <div className="rounded-lg border border-dashed p-8 text-center text-muted-foreground">
            <p className="font-medium text-foreground">No repositories yet</p>
            <p className="mt-1 text-sm">
              Repositories you have access to will show up here once an admin connects them.
              Check back later or ask your admin to add the repositories you need to review.
            </p>
          </div>
        )
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-sm">
            <thead className="border-b bg-muted/50">
              <tr>
                <th className="px-4 py-3 text-left font-medium">Repository</th>
                <th className="px-4 py-3 text-left font-medium">Connected</th>
                <th className="px-4 py-3 text-left font-medium">RAG Index</th>
                <th className="px-4 py-3 text-left font-medium">Enabled</th>
                <th className="px-4 py-3 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {repos.map((repo) => (
                <tr key={repo.ID}>
                  <td className="px-4 py-3 font-medium">{repo.Owner}/{repo.Name}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {new Date(repo.CreatedAt).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <IndexingBadge status={repo.IndexingStatus ?? "idle"} />
                      {isAdmin && repo.Enabled && (
                        <button
                          onClick={() => handleIndex(repo)}
                          // Only block on an in-flight click, not on persisted "indexing"
                          // status — otherwise a repo stuck on "indexing" (e.g. a failed
                          // run) could never be re-triggered to recover.
                          disabled={indexing[repo.ID]}
                          className="text-muted-foreground hover:text-foreground disabled:opacity-40"
                          title={repo.IndexingStatus === "indexing" ? "Restart indexing (recover a stuck run)" : "Re-index this repository"}
                        >
                          <Database className="h-3.5 w-3.5" />
                        </button>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    {isAdmin ? (
                      <Switch checked={repo.Enabled} onCheckedChange={() => toggle(repo)} />
                    ) : (
                      <span className="text-xs text-muted-foreground">{repo.Enabled ? "Enabled" : "Disabled"}</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {isAdmin && (
                      <Link href={`/repos/${repo.ID}`}>
                        <Button variant="outline" size="sm">Configure</Button>
                      </Link>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
