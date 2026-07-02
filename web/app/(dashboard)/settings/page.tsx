import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Cpu, GitBranch, Bell, Webhook, ClipboardList, KeyRound, Database, Puzzle, MessageSquare } from "lucide-react";

const sections = [
  {
    title: "AI Providers",
    description: "Configure OpenAI, Anthropic, Ollama, or custom providers.",
    href: "/settings/providers",
    icon: Cpu,
    label: "Manage Providers",
  },
  {
    title: "GitHub App",
    description: "Manage your GitHub App installation and webhook credentials.",
    href: "/settings/github-app",
    icon: GitBranch,
    label: "Configure",
  },
  {
    title: "Notifications",
    description: "Configure Slack, email, and webhook alerts for review events.",
    href: "/settings/notifications",
    icon: Bell,
    label: "Manage",
  },
  {
    title: "Webhooks",
    description: "View outbound webhook delivery history.",
    href: "/settings/webhooks",
    icon: Webhook,
    label: "View deliveries",
  },
  {
    title: "API Tokens",
    description: "Generate long-lived tokens for the CLI and external integrations.",
    href: "/settings/tokens",
    icon: KeyRound,
    label: "Manage tokens",
  },
  {
    title: "Audit Log",
    description: "Review all administrative actions with actor, timestamp, and change details.",
    href: "/settings/audit",
    icon: ClipboardList,
    label: "View log",
  },
  {
    title: "Data Retention",
    description: "Set review retention policies and handle GDPR erasure requests.",
    href: "/settings/retention",
    icon: Database,
    label: "Configure",
  },

  {
    title: "Jira",
    description: "Inject Jira ticket context into reviews when PRs reference ticket keys.",
    href: "/settings/integrations/jira",
    icon: Puzzle,
    label: "Configure",
  },
  {
    title: "Slack Bot",
    description: "Trigger reviews and check status from Slack with /review and @mentions.",
    href: "/settings/integrations/slack",
    icon: MessageSquare,
    label: "Configure",
  },
];

export default function SettingsPage() {
  return (
    <div className="space-y-8">
      <h1 className="text-3xl font-bold">Settings</h1>
      <div className="grid gap-5 sm:grid-cols-2">
        {sections.map(({ title, description, href, icon: Icon, label }) => (
          <Card key={href}>
            <CardHeader className="pb-3">
              <CardTitle className="flex items-center gap-2 text-lg">
                <Icon className="h-5 w-5 text-muted-foreground" aria-hidden="true" />
                {title}
              </CardTitle>
              <CardDescription className="text-sm">{description}</CardDescription>
            </CardHeader>
            <CardContent>
              <Link href={href}>
                <Button variant="outline">{label}</Button>
              </Link>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
