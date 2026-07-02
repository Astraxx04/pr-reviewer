"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
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
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "sonner";
import { Bell, Plus, Trash2, FlaskConical, Hash, Mail, Webhook, Zap, CalendarDays, Calendar } from "lucide-react";

const ALL_EVENTS = ["assignment", "review_complete", "re_review", "score_below_threshold"];
const EVENT_LABELS: Record<string, string> = {
  assignment: "Reviewer assigned",
  review_complete: "Review complete",
  re_review: "Re-review requested",
  score_below_threshold: "Score below threshold",
};

function channelIcon(channel: NotificationChannel) {
  if (channel === "slack") return <Hash className="h-5 w-5" />;
  if (channel === "email") return <Mail className="h-5 w-5" />;
  return <Webhook className="h-5 w-5" />;
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
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label className="text-sm">Webhook URL</Label>
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
      {value.events.includes("score_below_threshold") && (
        <ScoreThresholdField
          value={value.score_threshold}
          onChange={(score_threshold) => onChange({ ...value, score_threshold })}
        />
      )}
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
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label className="text-sm">
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
        <p className="text-xs text-muted-foreground">
          Assignment emails are sent to the assignee automatically (using their GitHub email).
          Add addresses here for extra recipients, or for review-complete / digest events that
          aren&apos;t tied to a specific person.
        </p>
      </div>
      <div className="space-y-1.5">
        <Label className="text-sm">From address</Label>
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
          <div className="col-span-2 space-y-1.5">
            <Label className="text-sm">SMTP host</Label>
            <Input
              placeholder="smtp.gmail.com"
              value={value.smtp_host ?? ""}
              onChange={(e) => onChange({ ...value, smtp_host: e.target.value })}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-sm">Port</Label>
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
        <div className="space-y-1.5">
          <Label className="text-sm">Username</Label>
          <Input
            placeholder="apikey / you@gmail.com"
            value={value.smtp_username ?? ""}
            onChange={(e) => onChange({ ...value, smtp_username: e.target.value })}
          />
        </div>
        <div className="space-y-1.5">
          <Label className="text-sm">Password</Label>
          <Input
            type="password"
            placeholder={value.smtp_password_set ? "•••••••• (leave blank to keep stored password)" : "SMTP password or app password"}
            value={value.smtp_password ?? ""}
            onChange={(e) => onChange({ ...value, smtp_password: e.target.value })}
          />
          {value.smtp_password_set && (
            <p className="text-xs text-muted-foreground">
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
      {value.events.includes("score_below_threshold") && (
        <ScoreThresholdField
          value={value.score_threshold}
          onChange={(score_threshold) => onChange({ ...value, score_threshold })}
        />
      )}
      <div className="space-y-1.5">
        <Label className="text-sm">Digest</Label>
        <div className="grid grid-cols-3 gap-2">
          {([
            { id: "none",   icon: <Zap className="h-5 w-5" />,         label: "None",   desc: "Email per event" },
            { id: "daily",  icon: <CalendarDays className="h-5 w-5" />, label: "Daily",  desc: "One summary a day" },
            { id: "weekly", icon: <Calendar className="h-5 w-5" />,     label: "Weekly", desc: "One summary a week" },
          ] as const).map((opt) => {
            const selected = (value.digest ?? "none") === opt.id;
            return (
              <button
                key={opt.id}
                type="button"
                onClick={() => onChange({ ...value, digest: opt.id })}
                className={`flex flex-col items-center gap-1.5 rounded-lg border p-3 text-center transition-colors cursor-pointer ${
                  selected
                    ? "border-primary bg-primary/5 text-primary"
                    : "border-border text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground"
                }`}
              >
                {opt.icon}
                <span className="text-sm font-medium">{opt.label}</span>
                <span className="text-xs">{opt.desc}</span>
              </button>
            );
          })}
        </div>
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
    <div className="space-y-4">
      <div className="space-y-1.5">
        <Label className="text-sm">Endpoint URL</Label>
        <Input
          placeholder="https://your-service.com/hooks/pr-reviewer"
          value={value.url}
          onChange={(e) => onChange({ ...value, url: e.target.value })}
        />
      </div>
      <div className="space-y-1.5">
        <Label className="text-sm">Secret (for HMAC-SHA256 signing — optional)</Label>
        <Input
          type="password"
          placeholder={value.secret_set ? "•••••••• (leave blank to keep stored secret)" : "your-webhook-secret"}
          value={value.secret ?? ""}
          onChange={(e) => onChange({ ...value, secret: e.target.value })}
        />
        {value.secret_set && (
          <p className="text-xs text-muted-foreground">
            A secret is stored (encrypted). Leave blank to keep it, or type a new one to replace it.
          </p>
        )}
      </div>
      <EventCheckboxes
        events={value.events}
        onChange={(events) => onChange({ ...value, events })}
      />
      {value.events.includes("score_below_threshold") && (
        <ScoreThresholdField
          value={value.score_threshold}
          onChange={(score_threshold) => onChange({ ...value, score_threshold })}
        />
      )}
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
    <div className="space-y-1.5">
      <Label className="text-sm">Score threshold</Label>
      <Input
        type="number"
        min={0}
        max={100}
        value={value ?? 0}
        onChange={(e) => onChange(Number(e.target.value))}
      />
      <p className="text-xs text-muted-foreground">
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
    <div className="space-y-1.5">
      <Label className="text-sm">Events</Label>
      <div className="flex flex-wrap gap-2">
        {ALL_EVENTS.map((e) => {
          const selected = events.includes(e);
          return (
            <button
              key={e}
              type="button"
              onClick={() => onChange(selected ? events.filter((x) => x !== e) : [...events, e])}
              className={`rounded-full px-3 py-1 text-sm border transition-colors cursor-pointer ${
                selected
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-muted text-muted-foreground border-transparent hover:border-border hover:text-foreground"
              }`}
            >
              {EVENT_LABELS[e]}
            </button>
          );
        })}
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
    <details className="group">
      <summary className="flex cursor-pointer items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground select-none list-none">
        <span className="transition-transform group-open:rotate-90">▶</span>
        Advanced — custom message template
      </summary>
      <div className="mt-2 space-y-1.5">
        <Textarea
          rows={3}
          placeholder="Use {{pr.title}}, {{pr.url}}, {{assignee}}, {{review.score}}, {{review.summary}}"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="font-mono text-xs"
        />
        <p className="text-xs text-muted-foreground">Leave blank to use the default template.</p>
      </div>
    </details>
  );
}

export default function NotificationsPage() {
  const { token } = useToken();
  const router = useRouter();
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
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null);

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

  async function handleDelete() {
    if (!token || confirmDeleteId == null) return;
    try {
      await deleteNotificationConfig(token, confirmDeleteId);
      setConfigs((cs) => cs.filter((c) => c.ID !== confirmDeleteId));
      setConfirmDeleteId(null);
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
    <div className="space-y-8">
      <div className="flex items-start justify-between">
        <div>
          <Button variant="ghost" size="sm" className="-ml-2 mb-3 text-muted-foreground" onClick={() => router.back()}>← Back</Button>
          <h1 className="text-3xl font-bold">Notifications</h1>
          <p className="text-muted-foreground text-base mt-1">
            Configure Slack, email, and webhook alerts for review events.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="lg" onClick={handleTriggerDigest}>
            Send digest now
          </Button>
          <Button size="lg" onClick={openCreate}>
            <Plus className="h-5 w-5 mr-2" />
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
            <Bell className="h-10 w-10 mx-auto mb-3 opacity-30" />
            <p className="text-base font-medium text-foreground">No notification channels configured yet.</p>
            <p className="text-base mt-1">
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
                    <CardTitle className="text-lg capitalize">{cfg.Channel}</CardTitle>
                    <Badge variant={cfg.Enabled ? "default" : "secondary"}>
                      {cfg.Enabled ? "Enabled" : "Disabled"}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-2">
                    <Switch
                      checked={cfg.Enabled}
                      onCheckedChange={() => handleToggle(cfg)}
                      className="cursor-pointer"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => handleTest(cfg.ID)}
                      disabled={testing === cfg.ID}
                    >
                      <FlaskConical className="h-4 w-4 mr-1" />
                      {testing === cfg.ID ? "Testing…" : "Test"}
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => openEdit(cfg)}>
                      Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive"
                      onClick={() => setConfirmDeleteId(cfg.ID)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-base text-muted-foreground truncate">{configSummary(cfg)}</p>
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
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle className="text-xl">
              {editTarget ? "Edit notification channel" : "Add notification channel"}
            </DialogTitle>
            <DialogDescription>
              {editTarget
                ? "Update the configuration for this notification channel."
                : "Choose a channel type and configure where to send review event alerts."}
            </DialogDescription>
          </DialogHeader>

          <div className="overflow-y-auto max-h-[60vh] space-y-5 py-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            {!editTarget && (
              <div className="space-y-1.5">
                <Label className="text-sm">Channel type</Label>
                <div className="grid grid-cols-3 gap-2">
                  {([
                    { id: "slack",   icon: <Hash className="h-5 w-5" />,    label: "Slack",   desc: "Post to a channel" },
                    { id: "email",   icon: <Mail className="h-5 w-5" />,    label: "Email",   desc: "Send via SMTP" },
                    { id: "webhook", icon: <Webhook className="h-5 w-5" />, label: "Webhook", desc: "HTTP POST payload" },
                  ] as const).map((ch) => (
                    <button
                      key={ch.id}
                      type="button"
                      onClick={() => { setChannel(ch.id); setChannelConfig(defaultConfig(ch.id)); }}
                      className={`flex flex-col items-center gap-1.5 rounded-lg border p-3 text-center transition-colors cursor-pointer ${
                        channel === ch.id
                          ? "border-primary bg-primary/5 text-primary"
                          : "border-border text-muted-foreground hover:border-muted-foreground/50 hover:text-foreground"
                      }`}
                    >
                      {ch.icon}
                      <span className="text-sm font-medium">{ch.label}</span>
                      <span className="text-xs">{ch.desc}</span>
                    </button>
                  ))}
                </div>
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

            <div className="flex items-center justify-between rounded-lg border px-4 py-3">
              <div>
                <p className="text-sm font-medium">Enabled</p>
                <p className="text-sm text-muted-foreground">Send notifications for this channel</p>
              </div>
              <Switch checked={enabled} onCheckedChange={setEnabled} className="cursor-pointer" />
            </div>
          </div>

          <DialogFooter>
            <Button variant="ghost" size="lg" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button size="lg" onClick={handleSave} disabled={saving}>
              {saving ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={confirmDeleteId != null} onOpenChange={(o) => { if (!o) setConfirmDeleteId(null); }}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="text-xl">Remove channel?</DialogTitle>
            <DialogDescription className="text-base">
              <strong className="capitalize">{configs.find((c) => c.ID === confirmDeleteId)?.Channel || "This channel"}</strong> will
              be permanently removed and will stop sending notifications.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" size="lg" onClick={() => setConfirmDeleteId(null)}>Cancel</Button>
            <Button variant="destructive" size="lg" onClick={handleDelete}>Remove</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
