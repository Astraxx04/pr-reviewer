import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Cpu, GitBranch, Bell, Webhook, ClipboardList, KeyRound, Database, ShieldCheck, Puzzle, MessageSquare } from "lucide-react";

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
    title: "Single Sign-On",
    description: "Configure OIDC SSO for Okta, Azure AD, Google Workspace, or any OIDC IdP.",
    href: "/settings/sso",
    icon: ShieldCheck,
    label: "Configure SSO",
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
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Settings</h1>
      <div className="grid gap-4 sm:grid-cols-2">
        {sections.map(({ title, description, href, icon: Icon, label }) => (
          <Card key={href}>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-base">
                <Icon className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
                {title}
              </CardTitle>
              <CardDescription>{description}</CardDescription>
            </CardHeader>
            <CardContent>
              <Link href={href}>
                <Button variant="outline" size="sm" style={{ cursor: 'pointer' }}>{label}</Button>
              </Link>
            </CardContent>
          </Card>
        ))}
      </div>
    </div>
  );
}
