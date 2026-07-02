"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useToken } from "@/hooks/useToken";
import { listRepos, updateRepo, syncRepos, triggerRepoIndex, getGithubApp, type Repo } from "@/lib/api";
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
  return <span className={`text-sm font-medium ${className}`}>{label}</span>;
}

export default function ReposPage() {
  const { token, isAdmin } = useToken();
  const [repos, setRepos] = useState<Repo[]>([]);
  const [ghAppConfigured, setGhAppConfigured] = useState(false);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [indexing, setIndexing] = useState<Record<number, boolean>>({});

  useEffect(() => {
    if (!token) return;
    Promise.all([listRepos(token), getGithubApp(token)])
      .then(([r, app]) => { setRepos(r); setGhAppConfigured(!!app.configured); })
      .finally(() => setLoading(false));
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
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="text-3xl font-bold">Repositories</h1>
        {isAdmin && (
          <Button variant="outline" onClick={handleSync} disabled={syncing}>
            <RefreshCw className={`h-5 w-5 mr-2 ${syncing ? "animate-spin" : ""}`} />
            {syncing ? "Syncing…" : "Sync from GitHub"}
          </Button>
        )}
      </div>

      {repos.length === 0 ? (
        isAdmin ? (
          <div className="rounded-lg border border-dashed p-8 text-muted-foreground">
            {ghAppConfigured ? (
              <>
                <p className="text-center text-base font-medium text-foreground">No repositories connected</p>
                <p className="mt-1 text-center text-base">
                  Your GitHub App is configured. Click{" "}
                  <button onClick={handleSync} disabled={syncing} className="underline hover:text-foreground disabled:opacity-50">
                    {syncing ? "Syncing…" : "Sync from GitHub"}
                  </button>{" "}
                  to pull in the repositories the App can access.
                </p>
              </>
            ) : (
              <>
                <p className="text-center text-base font-medium text-foreground">No repositories connected</p>
                <p className="mt-1 text-center text-base">
                  Set up your GitHub App first in{" "}
                  <Link href="/settings/github-app" className="underline hover:text-foreground">
                    Settings → GitHub App
                  </Link>
                  , then come back and sync.
                </p>
              </>
            )}
          </div>
        ) : (
          <div className="rounded-lg border border-dashed p-8 text-center text-muted-foreground">
            <p className="text-base font-medium text-foreground">No repositories yet</p>
            <p className="mt-1 text-base">
              Repositories you have access to will show up here once an admin connects them.
              Check back later or ask your admin to add the repositories you need to review.
            </p>
          </div>
        )
      ) : (
        <div className="rounded-lg border">
          <table className="w-full text-base">
            <thead className="border-b bg-muted/50">
              <tr>
                <th className="px-5 py-3.5 text-left font-medium">Repository</th>
                <th className="px-5 py-3.5 text-left font-medium">Connected</th>
                <th className="px-5 py-3.5 text-left font-medium">RAG Index</th>
                <th className="px-5 py-3.5 text-left font-medium">Enabled</th>
                <th className="px-5 py-3.5 text-right font-medium">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y">
              {repos.map((repo) => (
                <tr key={repo.ID}>
                  <td className="px-5 py-4 font-medium">{repo.Owner}/{repo.Name}</td>
                  <td className="px-5 py-4 text-muted-foreground">
                    {new Date(repo.CreatedAt).toLocaleDateString()}
                  </td>
                  <td className="px-5 py-4">
                    <div className="flex items-center gap-2">
                      <IndexingBadge status={repo.IndexingStatus ?? "idle"} />
                      {isAdmin && repo.Enabled && (
                        <button
                          onClick={() => handleIndex(repo)}
                          // Only block on an in-flight click, not on persisted "indexing"
                          // status — otherwise a repo stuck on "indexing" (e.g. a failed
                          // run) could never be re-triggered to recover.
                          disabled={indexing[repo.ID]}
                          className="cursor-pointer text-muted-foreground hover:text-foreground disabled:opacity-40"
                          title={repo.IndexingStatus === "indexing" ? "Restart indexing (recover a stuck run)" : "Re-index this repository"}
                        >
                          <Database className="h-4 w-4" />
                        </button>
                      )}
                    </div>
                  </td>
                  <td className="px-5 py-4">
                    {isAdmin ? (
                      <Switch checked={repo.Enabled} onCheckedChange={() => toggle(repo)} className="cursor-pointer" />
                    ) : (
                      <span className="text-sm text-muted-foreground">{repo.Enabled ? "Enabled" : "Disabled"}</span>
                    )}
                  </td>
                  <td className="px-5 py-4 text-right">
                    {isAdmin && (
                      <Link href={`/repos/${repo.ID}`}>
                        <Button variant="outline">Configure</Button>
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
