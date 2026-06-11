"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import {
  listProviders, createProvider, updateProvider, deleteProvider, testProvider, getProviderHealth, listProviderModels,
  type Provider, type CreateProviderBody, type ProviderHealthEntry, PROVIDER_TYPE_META,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { Cpu, Trash2, FlaskConical, CheckCircle2, XCircle, AlertTriangle, ListChecks, Pencil } from "lucide-react";

const PROVIDER_TYPES = Object.keys(PROVIDER_TYPE_META);

const DEFAULT_FORM: CreateProviderBody = { name: "", type: "openai" };

function healthBadge(entry?: ProviderHealthEntry) {
  if (!entry || entry.status === "untested") return <span className="text-xs text-muted-foreground">Untested</span>;
  if (entry.status === "healthy") return <span className="flex items-center gap-1 text-xs text-green-600"><CheckCircle2 className="h-3 w-3" /> Healthy</span>;
  if (entry.status === "degraded") return <span className="flex items-center gap-1 text-xs text-yellow-600"><AlertTriangle className="h-3 w-3" /> Degraded</span>;
  return <span className="flex items-center gap-1 text-xs text-destructive"><XCircle className="h-3 w-3" /> Unreachable</span>;
}

export default function ProvidersPage() {
  const { token } = useToken();
  const [providers, setProviders] = useState<Provider[]>([]);
  const [health, setHealth] = useState<ProviderHealthEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<number | null>(null);
  const [form, setForm] = useState<CreateProviderBody>(DEFAULT_FORM);
  const [models, setModels] = useState<{ id: string; display_name?: string }[]>([]);
  const [fetchingModels, setFetchingModels] = useState(false);

  const meta = PROVIDER_TYPE_META[form.type] ?? PROVIDER_TYPE_META.openai_compatible;

  useEffect(() => {
    if (!token) return;
    Promise.all([listProviders(token), getProviderHealth(token)])
      .then(([p, h]) => { setProviders(p); setHealth(h); })
      .finally(() => setLoading(false));
  }, [token]);

  // When the provider type changes, pre-fill sensible defaults.
  function handleTypeChange(type: string | null) {
    if (!type) return;
    const m = PROVIDER_TYPE_META[type];
    setModels([]); // stale: belong to the previous provider
    setForm({
      ...form,
      type,
      base_url: m?.presetBaseUrl ?? (type === "ollama" ? "http://localhost:11434" : ""),
      default_model: m?.defaultModel ?? "",
    });
  }

  function openAdd() {
    setEditingId(null);
    setForm(DEFAULT_FORM);
    setModels([]);
    setOpen(true);
  }

  function openEdit(p: Provider) {
    setEditingId(p.id);
    setForm({
      name: p.name,
      type: p.type,
      base_url: p.base_url || undefined,
      default_model: p.default_model || undefined,
      supports_embeddings: p.supports_embeddings,
      embedding_model: p.embedding_model || undefined,
      api_key: "", // blank means "keep existing key"
    });
    setModels([]);
    setOpen(true);
  }

  async function handleFetchModels() {
    if (!token) return;
    setFetchingModels(true);
    try {
      const res = await listProviderModels(token, {
        // When editing, pass the provider id so the server reuses the stored
        // key if the API-key field was left blank.
        id: editingId ?? undefined,
        type: form.type,
        base_url: form.base_url || undefined,
        api_key: form.api_key || undefined,
      });
      if (!res.ok) {
        toast.error(res.message || "Could not fetch models");
        return;
      }
      setModels(res.models);
      toast.success(res.models.length ? `Found ${res.models.length} models` : "No models returned");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setFetchingModels(false);
    }
  }

  async function handleSave() {
    if (!token) return;
    setSaving(true);
    try {
      if (editingId != null) {
        await updateProvider(token, editingId, {
          name: form.name,
          base_url: form.base_url || undefined,
          default_model: form.default_model || undefined,
          supports_embeddings: form.supports_embeddings,
          embedding_model: form.embedding_model || undefined,
          // Only send the key if the user typed a new one; blank keeps the stored key.
          ...(form.api_key ? { api_key: form.api_key } : {}),
        });
      } else {
        await createProvider(token, form);
      }
      const fresh = await listProviders(token);
      setProviders(fresh);
      setOpen(false);
      setForm(DEFAULT_FORM);
      setEditingId(null);
      toast.success(editingId != null ? "Provider updated" : "Provider added");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: number) {
    if (!token) return;
    try {
      await deleteProvider(token, id);
      setProviders((prev) => prev.filter((p) => p.id !== id));
      toast.success("Provider removed");
    } catch (e) {
      toast.error(String(e));
    }
  }

  async function handleTest(id: number) {
    if (!token) return;
    setTesting(id);
    try {
      const res = await testProvider(token, id);
      toast[res.ok ? "success" : "error"](res.message);
      const fresh = await getProviderHealth(token);
      setHealth(fresh);
    } catch (e) {
      toast.error(String(e));
    } finally {
      setTesting(null);
    }
  }

  if (loading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">AI Providers</h1>
        <Button onClick={openAdd}>Add Provider</Button>
      </div>

      {providers.length === 0 ? (
        <p className="text-muted-foreground">No providers configured yet.</p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2">
          {providers.map((p) => {
            const typeMeta = PROVIDER_TYPE_META[p.type];
            const h = health.find((e) => e.provider_id === p.id);
            return (
              <Card key={p.id}>
                <CardHeader className="flex-row items-center gap-3 pb-2">
                  <Cpu className="h-5 w-5 text-muted-foreground" />
                  <CardTitle className="text-base">{p.name || p.type}</CardTitle>
                  <Badge variant="secondary" className="ml-auto">
                    {typeMeta?.label ?? p.type}
                  </Badge>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="text-sm text-muted-foreground space-y-1">
                    {p.default_model && <p>Model: <span className="font-mono">{p.default_model}</span></p>}
                    {p.base_url && <p>URL: <span className="font-mono text-xs">{p.base_url}</span></p>}
                    <div className="flex items-center gap-1.5">
                      {p.has_api_key
                        ? <><CheckCircle2 className="h-3.5 w-3.5 text-green-500" /> API key set</>
                        : <><XCircle className="h-3.5 w-3.5 text-destructive" /> No API key</>}
                    </div>
                    <div className="flex items-center gap-2">
                      {healthBadge(h)}
                      {h?.last_tested_at && (
                        <span className="text-xs text-muted-foreground">
                          {h.latency_ms != null ? `${h.latency_ms}ms · ` : ""}
                          {new Date(h.last_tested_at).toLocaleString()}
                        </span>
                      )}
                    </div>
                    {h?.error_msg && <p className="text-xs text-destructive">{h.error_msg}</p>}
                  </div>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={() => handleTest(p.id)} disabled={testing === p.id}>
                      <FlaskConical className="h-3 w-3 mr-1" />
                      {testing === p.id ? "Testing…" : "Test"}
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => openEdit(p)}>
                      <Pencil className="h-3 w-3 mr-1" />
                      Edit
                    </Button>
                    <Button variant="ghost" size="sm" className="text-destructive ml-auto" onClick={() => handleDelete(p.id)}>
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>{editingId != null ? "Edit AI Provider" : "Add AI Provider"}</DialogTitle></DialogHeader>
          <div className="space-y-4">
            <div className="space-y-1">
              <Label>Name</Label>
              <Input
                placeholder="e.g. My Anthropic"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
              />
            </div>

            <div className="space-y-1">
              <Label>Provider</Label>
              <Select value={form.type} onValueChange={handleTypeChange} disabled={editingId != null}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {PROVIDER_TYPES.map((t) => (
                    <SelectItem key={t} value={t}>{PROVIDER_TYPE_META[t].label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {editingId != null && (
                <p className="text-xs text-muted-foreground">Provider type can&apos;t be changed — delete and re-add to switch.</p>
              )}
              {meta.presetBaseUrl && (
                <p className="text-xs text-muted-foreground">
                  Endpoint: <span className="font-mono">{meta.presetBaseUrl}</span>
                </p>
              )}
            </div>

            {meta.needsApiKey && (
              <div className="space-y-1">
                <Label>API Key</Label>
                <Input
                  type="password"
                  placeholder={editingId != null ? "leave blank to keep existing key" : "sk-…"}
                  value={form.api_key ?? ""}
                  onChange={(e) => setForm({ ...form, api_key: e.target.value })}
                />
              </div>
            )}

            {meta.needsBaseUrl && !meta.presetBaseUrl && (
              <div className="space-y-1">
                <Label>Base URL</Label>
                <Input
                  placeholder="http://localhost:11434"
                  value={form.base_url ?? ""}
                  onChange={(e) => setForm({ ...form, base_url: e.target.value })}
                />
              </div>
            )}

            <div className="space-y-1">
              <div className="flex items-center justify-between">
                <Label>Default Model</Label>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={handleFetchModels}
                  disabled={fetchingModels || (editingId == null && meta.needsApiKey && !form.api_key)}
                >
                  <ListChecks className="h-3 w-3 mr-1" />
                  {fetchingModels ? "Fetching…" : "Fetch models"}
                </Button>
              </div>
              <Input
                list="provider-models"
                placeholder={meta.defaultModel ?? "model name…"}
                value={form.default_model ?? ""}
                onChange={(e) => setForm({ ...form, default_model: e.target.value })}
              />
              {models.length > 0 && (
                <datalist id="provider-models">
                  {models.map((m) => (
                    <option key={m.id} value={m.id}>{m.display_name || m.id}</option>
                  ))}
                </datalist>
              )}
              {models.length > 0 && (
                <p className="text-xs text-muted-foreground">
                  {models.length} models available — start typing to pick one.
                </p>
              )}
            </div>

            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="embeddings"
                checked={form.supports_embeddings ?? false}
                onChange={(e) => setForm({ ...form, supports_embeddings: e.target.checked })}
                className="h-4 w-4"
              />
              <Label htmlFor="embeddings" className="cursor-pointer font-normal">
                Use for embeddings (RAG)
              </Label>
            </div>

            {form.supports_embeddings && (() => {
              // Suggest embedding-looking models from the fetched list; fall back to
              // the full list if none match the "embed" heuristic.
              const embedModels = models.filter((m) => /embed/i.test(m.id));
              const suggestions = embedModels.length ? embedModels : models;
              return (
                <div className="space-y-1">
                  <div className="flex items-center justify-between">
                    <Label>Embedding model</Label>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-7 px-2 text-xs"
                      onClick={handleFetchModels}
                      disabled={fetchingModels || (editingId == null && meta.needsApiKey && !form.api_key)}
                    >
                      <ListChecks className="h-3 w-3 mr-1" />
                      {fetchingModels ? "Fetching…" : "Fetch models"}
                    </Button>
                  </div>
                  <Input
                    list="embedding-models"
                    placeholder={form.type === "ollama" ? "nomic-embed-text" : "text-embedding-3-small"}
                    value={form.embedding_model ?? ""}
                    onChange={(e) => setForm({ ...form, embedding_model: e.target.value })}
                  />
                  {suggestions.length > 0 && (
                    <datalist id="embedding-models">
                      {suggestions.map((m) => (
                        <option key={m.id} value={m.id}>{m.display_name || m.id}</option>
                      ))}
                    </datalist>
                  )}
                  <p className="text-xs text-muted-foreground">
                    A dedicated embeddings model — <strong>not</strong> a chat model. Using a chat
                    model (e.g. gpt-4o) here will fail. Leave blank to default to{" "}
                    <span className="font-mono">{form.type === "ollama" ? "nomic-embed-text" : "text-embedding-3-small"}</span>.
                  </p>
                </div>
              );
            })()}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={saving}>Cancel</Button>
            <Button onClick={handleSave} disabled={!form.type || saving}>
              {saving ? "Saving…" : editingId != null ? "Save changes" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
