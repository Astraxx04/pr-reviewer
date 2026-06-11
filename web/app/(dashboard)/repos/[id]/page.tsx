"use client";

import { useEffect, useState, use } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";
import { getRepoConfig, putRepoConfig, listProviders, listProviderModels, type Provider, PROVIDER_TYPE_META } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { ListChecks } from "lucide-react";

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
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="sm" onClick={() => router.back()}>← Back</Button>
        <h1 className="text-2xl font-bold">Repo AI Config</h1>
      </div>

      <p className="text-sm text-muted-foreground">
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
                    <CardTitle className="text-base">{agent.label}</CardTitle>
                    <CardDescription>{agent.description}</CardDescription>
                  </div>
                  {agent.optional && (
                    <Switch
                      checked={active}
                      onCheckedChange={(v) => updateAgent(agent.id, "enabled", v)}
                      aria-label={`Enable ${agent.label}`}
                    />
                  )}
                </div>
              </CardHeader>
              {active && (
              <CardContent className="space-y-3">
                <div className="space-y-1">
                  <Label>Provider</Label>
                  {providers.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                      No providers configured — <a href="/settings/providers" className="underline">add one first</a>.
                    </p>
                  ) : (
                    <Select
                      value={cfg.provider_id}
                      onValueChange={(v) => updateAgent(agent.id, "provider_id", v ?? "")}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Installation default">
                          {(value) => {
                            const p = providers.find((pr) => String(pr.id) === value);
                            return p
                              ? `${p.name || p.type} (${PROVIDER_TYPE_META[p.type]?.label ?? p.type})`
                              : "Installation default";
                          }}
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="">Installation default</SelectItem>
                        {providers.map((p) => (
                          <SelectItem key={p.id} value={String(p.id)}>
                            {p.name || p.type} ({PROVIDER_TYPE_META[p.type]?.label ?? p.type})
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  )}
                </div>
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label>Model override</Label>
                    {cfg.provider_id && (
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        className="h-7 px-2 text-xs"
                        onClick={() => fetchModels(cfg.provider_id)}
                        disabled={fetchingFor === cfg.provider_id}
                      >
                        <ListChecks className="h-3 w-3 mr-1" />
                        {fetchingFor === cfg.provider_id ? "Fetching…" : "Fetch models"}
                      </Button>
                    )}
                  </div>
                  <Input
                    list={`models-${agent.id}`}
                    placeholder={selectedProvider?.default_model ?? "e.g. gpt-4o"}
                    value={cfg.model}
                    onChange={(e) => updateAgent(agent.id, "model", e.target.value)}
                  />
                  {(modelsByProvider[cfg.provider_id]?.length ?? 0) > 0 && (
                    <>
                      <datalist id={`models-${agent.id}`}>
                        {modelsByProvider[cfg.provider_id].map((m) => (
                          <option key={m.id} value={m.id}>{m.display_name || m.id}</option>
                        ))}
                      </datalist>
                      <p className="text-xs text-muted-foreground">
                        {modelsByProvider[cfg.provider_id].length} models available — start typing to pick one.
                      </p>
                    </>
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
          <CardTitle className="text-base">Branch protection</CardTitle>
          <CardDescription>
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
            />
            <Label htmlFor="commit-status-enabled">Post commit status on each review</Label>
          </div>
          <div className="space-y-1">
            <Label htmlFor="min-score">Minimum score to pass</Label>
            <Input
              id="min-score"
              type="number"
              min={0}
              max={100}
              disabled={!commitStatus.enabled}
              value={commitStatus.min_score}
              onChange={(e) => setCommitStatus((c) => ({ ...c, min_score: Number(e.target.value) }))}
            />
            <p className="text-xs text-muted-foreground">
              Reviews scoring below this fail the check (state <code className="text-xs">failure</code>);
              at or above it pass (<code className="text-xs">success</code>).
            </p>
          </div>

          <div className="rounded-md border bg-muted/30 p-3 text-xs text-muted-foreground space-y-2">
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

      <Button onClick={save} disabled={saving}>{saving ? "Saving…" : "Save config"}</Button>
    </div>
  );
}
