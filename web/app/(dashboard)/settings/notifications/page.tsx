"use client";

import { useEffect, useState } from "react";
import { useToken } from "@/hooks/useToken";
import {
  listNotificationConfigs,
  createNotificationConfig,
  updateNotificationConfig,
  deleteNotificationConfig,
  testNotificationConfig,
  triggerDigest,
  type NotificationConfig,
  type NotificationChannel,
  type SlackConfig,
  type EmailConfig,
  type WebhookConfig,
} from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { Bell, Plus, Trash2, FlaskConical, Hash, Mail, Webhook } from "lucide-react";

const ALL_EVENTS = ["assignment", "review_complete", "re_review", "score_below_threshold"];
const EVENT_LABELS: Record<string, string> = {
  assignment: "Reviewer assigned",
  review_complete: "Review complete",
  re_review: "Re-review requested",
  score_below_threshold: "Score below threshold",
};

function channelIcon(channel: NotificationChannel) {
  if (channel === "slack") return <Hash className="h-4 w-4" />;
  if (channel === "email") return <Mail className="h-4 w-4" />;
  return <Webhook className="h-4 w-4" />;
}

function defaultConfig(channel: NotificationChannel): SlackConfig | EmailConfig | WebhookConfig {
  if (channel === "slack") {
    return { webhook_url: "", events: ["assignment", "review_complete"], score_threshold: 0, template: "" };
  }
  if (channel === "email") {
    return { to: [], events: ["review_complete"], digest: "none", template: "", score_threshold: 0 };
  }
  return { url: "", secret: "", events: ["review_complete"], template: "", score_threshold: 0 };
}

function configSummary(cfg: NotificationConfig): string {
  const c = cfg.Config as unknown as Record<string, unknown>;
  if (cfg.Channel === "slack") return (c.webhook_url as string) || "No webhook URL";
  if (cfg.Channel === "email") {
    const to = c.to as string[];
    return to?.length ? to.join(", ") : "No recipients";
  }
  return (c.url as string) || "No URL";
}

// --- Slack form ---
function SlackForm({
  value,
  onChange,
}: {
  value: SlackConfig;
  onChange: (v: SlackConfig) => void;
}) {
  return (
    <div className="space-y-3">
      <div>
        <Label>Webhook URL</Label>
        <Input
          placeholder="https://hooks.slack.com/services/..."
          value={value.webhook_url}
          onChange={(e) => onChange({ ...value, webhook_url: e.target.value })}
        />
      </div>
      <EventCheckboxes
        events={value.events}
        onChange={(events) => onChange({ ...value, events })}
      />
      <ScoreThresholdField
        value={value.score_threshold}
        onChange={(score_threshold) => onChange({ ...value, score_threshold })}
      />
      <TemplateField value={value.template} onChange={(t) => onChange({ ...value, template: t })} />
    </div>
  );
}

// --- Email form ---
function EmailForm({
  value,
  onChange,
}: {
  value: EmailConfig;
  onChange: (v: EmailConfig) => void;
}) {
  const [toInput, setToInput] = useState(value.to?.join(", ") ?? "");
  return (
    <div className="space-y-3">
      <div>
        <Label>
          Additional recipients{" "}
          <span className="text-xs text-muted-foreground font-normal">(optional, comma-separated)</span>
        </Label>
        <Input
          placeholder="eng-reviews@example.com, manager@example.com"
          value={toInput}
          onChange={(e) => {
            setToInput(e.target.value);
            onChange({
              ...value,
              to: e.target.value.split(",").map((s) => s.trim()).filter(Boolean),
            });
          }}
        />
        <p className="text-xs text-muted-foreground mt-1">
          Assignment emails are sent to the assignee automatically (using their GitHub email).
          Add addresses here for extra recipients, or for review-complete / digest events that
          aren&apos;t tied to a specific person.
        </p>
      </div>
      <div>
        <Label>From address</Label>
        <Input
          placeholder="pr-reviewer@yourcompany.com"
          value={value.from ?? ""}
          onChange={(e) => onChange({ ...value, from: e.target.value })}
        />
      </div>
      <div className="rounded-md border p-3 space-y-3">
        <p className="text-xs text-muted-foreground">
          SMTP server — host, port and from address are required.
        </p>
        <div className="grid grid-cols-3 gap-2">
          <div className="col-span-2">
            <Label>SMTP host</Label>
            <Input
              placeholder="smtp.gmail.com"
              value={value.smtp_host ?? ""}
              onChange={(e) => onChange({ ...value, smtp_host: e.target.value })}
            />
          </div>
          <div>
            <Label>Port</Label>
            <Input
              type="number"
              placeholder="587"
              value={value.smtp_port ?? ""}
              onChange={(e) =>
                onChange({ ...value, smtp_port: e.target.value ? Number(e.target.value) : undefined })
              }
            />
          </div>
        </div>
        <div>
          <Label>Username</Label>
          <Input
            placeholder="apikey / you@gmail.com"
            value={value.smtp_username ?? ""}
            onChange={(e) => onChange({ ...value, smtp_username: e.target.value })}
          />
        </div>
        <div>
          <Label>Password</Label>
          <Input
            type="password"
            placeholder={value.smtp_password_set ? "•••••••• (leave blank to keep stored password)" : "SMTP password or app password"}
            value={value.smtp_password ?? ""}
            onChange={(e) => onChange({ ...value, smtp_password: e.target.value })}
          />
          {value.smtp_password_set && (
            <p className="text-xs text-muted-foreground mt-1">
              A password is stored (encrypted). Leave blank to keep it, or type a new one to replace it.
            </p>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          Port 465 uses implicit TLS; 587/25 use STARTTLS. Leave username &amp; password blank
          for relays that don&apos;t require authentication.
        </p>
      </div>
      <EventCheckboxes
        events={value.events}
        onChange={(events) => onChange({ ...value, events })}
      />
      <ScoreThresholdField
        value={value.score_threshold}
        onChange={(score_threshold) => onChange({ ...value, score_threshold })}
      />
      <div>
        <Label>Digest</Label>
        <Select
          value={value.digest ?? "none"}
          onValueChange={(v) => onChange({ ...value, digest: v as EmailConfig["digest"] })}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="none">None — email on each event</SelectItem>
            <SelectItem value="daily">Daily digest</SelectItem>
            <SelectItem value="weekly">Weekly digest</SelectItem>
          </SelectContent>
        </Select>
        <p className="text-xs text-muted-foreground mt-1">
          A digest sends one summary email per period instead of (or in addition to) per-event alerts.
        </p>
      </div>
      <TemplateField value={value.template} onChange={(t) => onChange({ ...value, template: t })} />
    </div>
  );
}

// --- Webhook form ---
function WebhookForm({
  value,
  onChange,
}: {
  value: WebhookConfig;
  onChange: (v: WebhookConfig) => void;
}) {
  return (
    <div className="space-y-3">
      <div>
        <Label>Endpoint URL</Label>
        <Input
          placeholder="https://your-service.com/hooks/pr-reviewer"
          value={value.url}
          onChange={(e) => onChange({ ...value, url: e.target.value })}
        />
      </div>
      <div>
        <Label>Secret (for HMAC-SHA256 signing — optional)</Label>
        <Input
          type="password"
          placeholder={value.secret_set ? "•••••••• (leave blank to keep stored secret)" : "your-webhook-secret"}
          value={value.secret ?? ""}
          onChange={(e) => onChange({ ...value, secret: e.target.value })}
        />
        {value.secret_set && (
          <p className="text-xs text-muted-foreground mt-1">
            A secret is stored (encrypted). Leave blank to keep it, or type a new one to replace it.
          </p>
        )}
      </div>
      <EventCheckboxes
        events={value.events}
        onChange={(events) => onChange({ ...value, events })}
      />
      <ScoreThresholdField
        value={value.score_threshold}
        onChange={(score_threshold) => onChange({ ...value, score_threshold })}
      />
    </div>
  );
}

// ScoreThresholdField configures the threshold used by the "Score below threshold"
// event. Only meaningful when that event is selected.
function ScoreThresholdField({
  value,
  onChange,
}: {
  value: number | undefined;
  onChange: (v: number) => void;
}) {
  return (
    <div>
      <Label>Score threshold</Label>
      <Input
        type="number"
        min={0}
        max={100}
        value={value ?? 0}
        onChange={(e) => onChange(Number(e.target.value))}
      />
      <p className="text-xs text-muted-foreground mt-1">
        Fires the &ldquo;Score below threshold&rdquo; event when a review scores below this value (0 = disabled).
      </p>
    </div>
  );
}

function EventCheckboxes({
  events,
  onChange,
}: {
  events: string[];
  onChange: (events: string[]) => void;
}) {
  return (
    <div>
      <Label className="mb-1 block">Events</Label>
      <div className="flex flex-wrap gap-3">
        {ALL_EVENTS.map((e) => (
          <label key={e} className="flex items-center gap-1.5 text-sm cursor-pointer">
            <input
              type="checkbox"
              checked={events.includes(e)}
              onChange={(ev) =>
                onChange(ev.target.checked ? [...events, e] : events.filter((x) => x !== e))
              }
            />
            {EVENT_LABELS[e]}
          </label>
        ))}
      </div>
    </div>
  );
}

function TemplateField({
  value,
  onChange,
}: {
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <div>
      <Label>
        Message template{" "}
        <span className="text-xs text-muted-foreground font-normal">
          (optional — uses default if blank)
        </span>
      </Label>
      <Textarea
        rows={3}
        placeholder="Use {{pr.title}}, {{pr.url}}, {{assignee}}, {{review.score}}, {{review.summary}}"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="font-mono text-xs"
      />
    </div>
  );
}

export default function NotificationsPage() {
  const { token } = useToken();
  const [configs, setConfigs] = useState<NotificationConfig[]>([]);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<NotificationConfig | null>(null);
  const [channel, setChannel] = useState<NotificationChannel>("slack");
  const [channelConfig, setChannelConfig] = useState<
    SlackConfig | EmailConfig | WebhookConfig
  >(defaultConfig("slack"));
  const [enabled, setEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState<number | null>(null);

  useEffect(() => {
    if (!token) return;
    listNotificationConfigs(token)
      .then(setConfigs)
      .catch(() => toast.error("Failed to load notification configs"))
      .finally(() => setLoading(false));
  }, [token]);

  function openCreate() {
    setEditTarget(null);
    setChannel("slack");
    setChannelConfig(defaultConfig("slack"));
    setEnabled(true);
    setOpen(true);
  }

  function openEdit(cfg: NotificationConfig) {
    setEditTarget(cfg);
    setChannel(cfg.Channel);
    setChannelConfig(cfg.Config as SlackConfig | EmailConfig | WebhookConfig);
    setEnabled(cfg.Enabled);
    setOpen(true);
  }

  async function handleSave() {
    if (!token) return;
    if (channel === "email") {
      const ec = channelConfig as EmailConfig;
      if (!ec.smtp_host?.trim() || !ec.smtp_port || !ec.from?.trim()) {
        toast.error("SMTP host, port and from address are required");
        return;
      }
    }
    setSaving(true);
    try {
      if (editTarget) {
        const updated = await updateNotificationConfig(token, editTarget.ID, {
          channel,
          config: channelConfig,
          enabled,
        });
        setConfigs((cs) => cs.map((c) => (c.ID === updated.ID ? updated : c)));
        toast.success("Notification config updated");
      } else {
        const created = await createNotificationConfig(token, {
          channel,
          config: channelConfig,
          enabled,
        });
        setConfigs((cs) => [...cs, created]);
        toast.success("Notification config created");
      }
      setOpen(false);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(id: number) {
    if (!token) return;
    try {
      await deleteNotificationConfig(token, id);
      setConfigs((cs) => cs.filter((c) => c.ID !== id));
      toast.success("Deleted");
    } catch {
      toast.error("Delete failed");
    }
  }

  async function handleTest(id: number) {
    if (!token) return;
    setTesting(id);
    try {
      const result = await testNotificationConfig(token, id);
      if (result.ok) {
        toast.success("Test notification sent successfully");
      } else {
        toast.error(`Test failed: ${result.error}`);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Test failed");
    } finally {
      setTesting(null);
    }
  }

  async function handleTriggerDigest() {
    if (!token) return;
    try {
      await triggerDigest(token, "daily");
      toast.success("Daily digest queued — recipients with digest=daily will receive a summary shortly.");
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to trigger digest");
    }
  }

  async function handleToggle(cfg: NotificationConfig) {
    if (!token) return;
    try {
      const updated = await updateNotificationConfig(token, cfg.ID, { enabled: !cfg.Enabled });
      setConfigs((cs) => cs.map((c) => (c.ID === updated.ID ? updated : c)));
    } catch {
      toast.error("Update failed");
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Notifications</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Configure Slack, email, and webhook alerts for review events.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={handleTriggerDigest}>
            Send digest now
          </Button>
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4 mr-2" />
            Add channel
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="space-y-3">
          {[0, 1].map((i) => (
            <Skeleton key={i} className="h-24 w-full" />
          ))}
        </div>
      ) : configs.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground">
            <Bell className="h-8 w-8 mx-auto mb-3 opacity-30" />
            <p>No notification channels configured yet.</p>
            <p className="text-sm mt-1">
              Add a Slack webhook, email recipients, or an outbound webhook to get started.
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {configs.map((cfg) => (
            <Card key={cfg.ID}>
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    {channelIcon(cfg.Channel)}
                    <CardTitle className="text-base capitalize">{cfg.Channel}</CardTitle>
                    <Badge variant={cfg.Enabled ? "default" : "secondary"}>
                      {cfg.Enabled ? "Enabled" : "Disabled"}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={cfg.Enabled}
                      onCheckedChange={() => handleToggle(cfg)}
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleTest(cfg.ID)}
                      disabled={testing === cfg.ID}
                    >
                      <FlaskConical className="h-3 w-3 mr-1" />
                      {testing === cfg.ID ? "Testing…" : "Test"}
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => openEdit(cfg)}>
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      onClick={() => handleDelete(cfg.ID)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-sm text-muted-foreground truncate">{configSummary(cfg)}</p>
                <div className="flex flex-wrap gap-1.5 mt-2">
                  {((cfg.Config as unknown as Record<string, unknown>).events as string[] | undefined)?.map((e) => (
                    <Badge key={e} variant="outline" className="text-xs">
                      {EVENT_LABELS[e] ?? e}
                    </Badge>
                  ))}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-lg max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>
              {editTarget ? "Edit notification channel" : "Add notification channel"}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            {!editTarget && (
              <div>
                <Label>Channel type</Label>
                <Select
                  value={channel}
                  onValueChange={(v) => {
                    const ch = v as NotificationChannel;
                    setChannel(ch);
                    setChannelConfig(defaultConfig(ch));
                  }}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="email">Email</SelectItem>
                    <SelectItem value="webhook">Outbound webhook</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}

            {channel === "slack" && (
              <SlackForm
                value={channelConfig as SlackConfig}
                onChange={setChannelConfig}
              />
            )}
            {channel === "email" && (
              <EmailForm
                value={channelConfig as EmailConfig}
                onChange={setChannelConfig}
              />
            )}
            {channel === "webhook" && (
              <WebhookForm
                value={channelConfig as WebhookConfig}
                onChange={setChannelConfig}
              />
            )}

            <div className="flex items-center gap-2">
              <Switch checked={enabled} onCheckedChange={setEnabled} />
              <Label>Enabled</Label>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSave} disabled={saving}>
              {saving ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
