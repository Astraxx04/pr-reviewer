"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { CheckCircle2, Circle, X, ChevronDown, ChevronUp } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { getGithubApp, listProviders, listRepos, listTeam, listNotificationConfigs } from "@/lib/api";

interface ChecklistItem {
  id: string;
  label: string;
  href: string;
  done: boolean;
}

const DISMISSED_KEY = "pr_reviewer_onboarding_dismissed";

export function OnboardingChecklist({ token }: { token: string }) {
  const [items, setItems] = useState<ChecklistItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [dismissed, setDismissed] = useState(false);
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    if (typeof window !== "undefined") {
      setDismissed(localStorage.getItem(DISMISSED_KEY) === "1");
    }
  }, []);

  useEffect(() => {
    if (!token || dismissed) return;

    Promise.allSettled([
      getGithubApp(token),
      listProviders(token),
      listRepos(token),
      listTeam(token),
      listNotificationConfigs(token),
    ]).then(([ghApp, providers, repos, team, notifs]) => {
      const ghDone = ghApp.status === "fulfilled" && ghApp.value.configured;
      const providerDone = providers.status === "fulfilled" && providers.value.length > 0;
      const repoDone = repos.status === "fulfilled" && repos.value.some((r) => r.Enabled);
      const teamDone = team.status === "fulfilled" && team.value.length > 0;
      const notifDone = notifs.status === "fulfilled" && notifs.value.length > 0;

      setItems([
        { id: "github", label: "Connect a GitHub App", href: "/settings/github-app", done: ghDone },
        { id: "provider", label: "Add an AI provider", href: "/settings/providers", done: providerDone },
        { id: "repo", label: "Enable your first repo", href: "/repos", done: repoDone },
        { id: "team", label: "Add a team member", href: "/team", done: teamDone },
        { id: "notifications", label: "Configure notifications", href: "/settings/notifications", done: notifDone },
      ]);
      setLoading(false);
    });
  }, [token, dismissed]);

  function dismiss() {
    localStorage.setItem(DISMISSED_KEY, "1");
    setDismissed(true);
  }

  if (dismissed || loading) return null;
  const allDone = items.every((i) => i.done);
  if (allDone) return null;

  const doneCount = items.filter((i) => i.done).length;

  return (
    <Card className="border-primary/20 bg-primary/5">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base flex items-center gap-2">
            Getting started
            <span className="text-xs font-normal text-muted-foreground">
              {doneCount}/{items.length} complete
            </span>
          </CardTitle>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground"
              onClick={() => setCollapsed((c) => !c)}
              aria-label={collapsed ? "Expand checklist" : "Collapse checklist"}
              aria-expanded={!collapsed}
              style={{ cursor: 'pointer' }}
            >
              {collapsed ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 text-muted-foreground"
              onClick={dismiss}
              aria-label="Dismiss onboarding checklist"
              style={{ cursor: 'pointer' }}
            >
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </CardHeader>
      {!collapsed && (
        <CardContent>
          <ul className="space-y-2" role="list" aria-label="Setup steps">
            {items.map((item) => (
              <li key={item.id} className="flex items-center gap-3 text-sm">
                {item.done ? (
                  <CheckCircle2 className="h-4 w-4 flex-shrink-0 text-green-500" aria-hidden="true" />
                ) : (
                  <Circle className="h-4 w-4 flex-shrink-0 text-muted-foreground" aria-hidden="true" />
                )}
                {item.done ? (
                  <span className="line-through text-muted-foreground">{item.label}</span>
                ) : (
                  <Link
                    href={item.href}
                    className="hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring rounded-sm"
                  >
                    {item.label}
                  </Link>
                )}
                {item.done && <span className="sr-only">(complete)</span>}
              </li>
            ))}
          </ul>
        </CardContent>
      )}
    </Card>
  );
}
