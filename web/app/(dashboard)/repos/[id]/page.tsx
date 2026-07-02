"use client";

import { useEffect, useState, use } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { getRepoConfig, putRepoConfig, listProviders, listProviderModels, type Provider, PROVIDER_TYPE_META } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { RefreshCw } from "lucide-react";

interface AgentCfg {
  provider_id: string;
  model: string;
  enabled?: boolean;
}

type RepoAgentConfig = Record<string, AgentCfg>;

interface CommitStatusCfg {
  enabled: boolean;
  min_score: number;
}

const DEFAULT_COMMIT_STATUS: CommitStatusCfg = { enabled: false, min_score: 60 };

const AGENTS = [
  { id: "code-review", label: "Code Review Agent", description: "General code quality and correctness", optional: false },
  { id: "security", label: "Security Agent", description: "Security vulnerabilities and best practices", optional: false },
  { id: "performance", label: "Performance Agent", description: "Algorithmic complexity, N+1 queries, allocations, and hot-path I/O", optional: true },
  { id: "database", label: "Database Agent", description: "Migration safety, indexes, transactions, and query patterns", optional: true },
];

export default function RepoConfigPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { token } = useToken();
  const router = useRouter();
  const [config, setConfig] = useState<RepoAgentConfig>({});
  const [commitStatus, setCommitStatus] = useState<CommitStatusCfg>(DEFAULT_COMMIT_STATUS);
  const [extraConfig, setExtraConfig] = useState<Record<string, unknown>>({});
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  // Fetched models keyed by provider id, so a provider's list is shared across agents.
  const [modelsByProvider, setModelsByProvider] = useState<Record<string, { id: string; display_name?: string }[]>>({});
  const [fetchingFor, setFetchingFor] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    Promise.all([
      getRepoConfig(token, Number(id)),
      listProviders(token),
    ]).then(([cfg, prvs]) => {
      const raw = (cfg.config ?? {}) as Record<string, unknown>;
      if (raw.agents && typeof raw.agents === "object") {
        // Nested format: { agents, commit_status, ...other section-8 settings }.
        const { agents, commit_status, ...rest } = raw;
        setConfig((agents as RepoAgentConfig) ?? {});
        setCommitStatus({ ...DEFAULT_COMMIT_STATUS, ...(commit_status as CommitStatusCfg) });
        setExtraConfig(rest);
      } else {
        // Legacy flat format: the whole object is the agents map.
        setConfig(raw as RepoAgentConfig);
      }
      setProviders(prvs);
    }).finally(() => setLoading(false));
  }, [token, id]);

  function updateAgent(agentId: string, field: keyof AgentCfg, value: string | boolean) {
    setConfig((prev) => ({
      ...prev,
      [agentId]: { ...(prev[agentId] ?? { provider_id: "", model: "" }), [field]: value },
    }));
  }

  async function fetchModels(providerId: string) {
    if (!token || !providerId) return;
    setFetchingFor(providerId);
    try {
      const res = await listProviderModels(token, { id: Number(providerId) });
      if (!res.ok) {
        toast.error(res.message || "Could not fetch models");
        return;
      }
      setModelsByProvider((prev) => ({ ...prev, [providerId]: res.models }));
      toast.success(res.models.length ? `Found ${res.models.length} models` : "No models returned");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setFetchingFor(null);
    }
  }

  async function save() {
    if (!token) return;
    setSaving(true);
    try {
      // Always persist the nested format so section-8 settings round-trip cleanly.
      await putRepoConfig(token, Number(id), {
        ...extraConfig,
        agents: config,
        commit_status: commitStatus,
      });
      toast.success("Config saved");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-8 max-w-2xl">
      <div>
        <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>← Back</Button>
        <h1 className="text-3xl font-bold">Repo AI Config</h1>
      </div>

      <p className="text-base text-muted-foreground">
        Code Review and Security run on every review. Performance and Database are optional —
        toggle them on per repo. For any active agent, override the AI provider and model, or leave
        blank to use the installation default.
      </p>

      <div className="space-y-4">
        {AGENTS.map((agent) => {
          const cfg = config[agent.id] ?? { provider_id: "", model: "" };
          const selectedProvider = providers.find((p) => String(p.id) === cfg.provider_id || p.name === cfg.provider_id);
          const active = !agent.optional || (cfg.enabled ?? false);
          return (
            <Card key={agent.id}>
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between gap-4">
                  <div className="space-y-1.5">
                    <CardTitle className="text-lg">{agent.label}</CardTitle>
                    <CardDescription className="text-sm">{agent.description}</CardDescription>
                  </div>
                  {agent.optional && (
                    <Switch
                      checked={active}
                      onCheckedChange={(v) => updateAgent(agent.id, "enabled", v)}
                      aria-label={`Enable ${agent.label}`}
                      className="cursor-pointer"
                    />
                  )}
                </div>
              </CardHeader>
              {active && (
              <CardContent className="space-y-4">
                <div className="space-y-1.5">
                  <Label className="text-sm">Provider</Label>
                  {providers.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      No providers configured — <a href="/settings/providers" className="underline">add one first</a>.
                    </p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        onClick={() => updateAgent(agent.id, "provider_id", "")}
                        className={`rounded-lg border px-3 py-2 text-sm transition-colors cursor-pointer ${
                          !cfg.provider_id
                            ? "border-primary bg-primary/5 text-primary"
                            : "border-border text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground"
                        }`}
                      >
                        Installation default
                      </button>
                      {providers.map((p) => (
                        <button
                          key={p.id}
                          type="button"
                          onClick={() => updateAgent(agent.id, "provider_id", String(p.id))}
                          className={`rounded-lg border px-3 py-2 text-sm transition-colors cursor-pointer ${
                            cfg.provider_id === String(p.id)
                              ? "border-primary bg-primary/5 text-primary"
                              : "border-border text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground"
                          }`}
                        >
                          <span className="font-medium">{p.name || p.type}</span>
                          <span className="ml-1.5 text-xs opacity-60">{PROVIDER_TYPE_META[p.type]?.label ?? p.type}</span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
                <div className="space-y-1.5">
                  <Label className="text-sm">Model override</Label>
                  <div className="flex gap-2">
                    <Input
                      placeholder={selectedProvider?.default_model ?? "e.g. gpt-4o"}
                      value={cfg.model}
                      onChange={(e) => updateAgent(agent.id, "model", e.target.value)}
                      className="flex-1"
                    />
                    {cfg.provider_id && (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        className="shrink-0"
                        onClick={() => fetchModels(cfg.provider_id)}
                        disabled={fetchingFor === cfg.provider_id}
                        title="Fetch available models"
                      >
                        <RefreshCw className={`h-4 w-4 ${fetchingFor === cfg.provider_id ? "animate-spin" : ""}`} />
                      </Button>
                    )}
                  </div>
                  {(modelsByProvider[cfg.provider_id]?.length ?? 0) > 0 && (
                    <div className="overflow-y-auto max-h-32 rounded-md border p-2 flex flex-wrap gap-1.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                      {modelsByProvider[cfg.provider_id].map((m) => (
                        <button
                          key={m.id}
                          type="button"
                          onClick={() => updateAgent(agent.id, "model", m.id)}
                          className={`rounded-full px-2.5 py-0.5 text-xs border transition-colors cursor-pointer ${
                            cfg.model === m.id
                              ? "bg-primary text-primary-foreground border-primary"
                              : "bg-muted text-muted-foreground border-transparent hover:border-border hover:text-foreground"
                          }`}
                        >
                          {m.display_name || m.id}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </CardContent>
              )}
            </Card>
          );
        })}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-lg">Branch protection</CardTitle>
          <CardDescription className="text-sm">
            Post a GitHub commit status (<code className="text-xs bg-muted px-1 rounded">pr-reviewer</code>)
            so you can require it in branch protection rules and block merges below a score threshold.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-3">
            <Switch
              id="commit-status-enabled"
              checked={commitStatus.enabled}
              onCheckedChange={(v) => setCommitStatus((c) => ({ ...c, enabled: v }))}
              className="cursor-pointer"
            />
            <Label htmlFor="commit-status-enabled" className="text-base cursor-pointer">Post commit status on each review</Label>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="min-score" className="text-sm">Minimum score to pass</Label>
            <Input
              id="min-score"
              type="number"
              min={0}
              max={100}
              disabled={!commitStatus.enabled}
              value={commitStatus.min_score}
              onChange={(e) => setCommitStatus((c) => ({ ...c, min_score: Number(e.target.value) }))}
            />
            <p className="text-sm text-muted-foreground">
              Reviews scoring below this fail the check (state <code className="text-xs">failure</code>);
              at or above it pass (<code className="text-xs">success</code>).
            </p>
          </div>

          <div className="rounded-md border bg-muted/30 p-3 text-sm text-muted-foreground space-y-2">
            <p className="font-medium text-foreground">To actually block merges, enforce it on GitHub</p>
            <p>
              Enabling this only <em>posts</em> the{" "}
              <code className="text-xs bg-muted px-1 rounded">pr-reviewer</code> check — it does not
              block merges until you require it in a GitHub branch protection rule:
            </p>
            <ol className="list-decimal space-y-1 pl-4">
              <li>
                Trigger one review first (open or push to a PR) so the{" "}
                <code className="text-xs bg-muted px-1 rounded">pr-reviewer</code> check has reported
                once — GitHub only lists checks it has seen.
              </li>
              <li>
                On GitHub, go to the repo&apos;s{" "}
                <strong>Settings → Branches</strong> and add (or edit) a branch protection rule for
                your default branch (e.g. <code className="text-xs bg-muted px-1 rounded">main</code>).
                {" "}On newer repos use <strong>Settings → Rules → Rulesets</strong> instead.
              </li>
              <li>
                Enable <strong>Require status checks to pass before merging</strong>, search for{" "}
                <code className="text-xs bg-muted px-1 rounded">pr-reviewer</code>, and select it.
              </li>
              <li>Save the rule. Merging is now blocked whenever the score is below the threshold above.</li>
            </ol>
            <p>
              To cover many repos at once, create the ruleset at the{" "}
              <strong>organisation</strong> level (Org → Settings → Rules → Rulesets) and target repos
              by name. Note: repo admins can bypass a failing check unless you also disallow bypassing.
            </p>
            <p>
              <a
                href="https://docs.github.com/articles/about-protected-branches"
                target="_blank"
                rel="noreferrer"
                className="underline hover:text-foreground"
              >
                GitHub docs: about protected branches →
              </a>
            </p>
          </div>
        </CardContent>
      </Card>

      <Button size="lg" onClick={save} disabled={saving}>{saving ? "Saving…" : "Save config"}</Button>
    </div>
  );
}
