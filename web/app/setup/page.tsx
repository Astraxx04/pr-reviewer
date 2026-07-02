"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { getSetupStatus, completeSetup, type SetupStatus } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { CheckCircle2, Circle, XCircle, Database, GitBranch, Rocket } from "lucide-react";
import { toast } from "sonner";

const STEPS = [
  { id: "db", label: "Database", icon: Database },
  { id: "github", label: "GitHub", icon: GitBranch },
  { id: "done", label: "Launch", icon: Rocket },
] as const;

function StepIndicator({
  current,
  status,
  serverDown,
  onStepClick,
}: {
  current: number;
  status: SetupStatus | null;
  serverDown: boolean;
  onStepClick: (i: number) => void;
}) {
  const states = [
    !serverDown,
    status?.github_configured ?? false,
    false,
  ];

  return (
    <div className="flex items-center justify-center mb-10">
      {STEPS.map((step, i) => (
        <div key={step.id} className="flex items-center">
          <button
            onClick={() => onStepClick(i)}
            style={{ cursor: 'pointer' }}
            className={`flex items-center gap-2 px-4 py-2 rounded-full text-base font-medium transition-colors ${
              i === current
                ? "bg-primary text-primary-foreground"
                : states[i]
                ? "text-green-600 hover:bg-green-50 dark:hover:bg-green-950/20"
                : "text-muted-foreground hover:bg-muted"
            }`}
          >
            {states[i] && i !== current ? (
              <CheckCircle2 className="h-4 w-4" />
            ) : (
              <Circle className="h-4 w-4" />
            )}
            {step.label}
          </button>
          {i < STEPS.length - 1 && (
            <div className={`h-px w-12 mx-2 ${states[i] ? "bg-green-400" : "bg-border"}`} />
          )}
        </div>
      ))}
    </div>
  );
}

export default function SetupPage() {
  const router = useRouter();
  const [step, setStep] = useState(0);
  const [status, setStatus] = useState<SetupStatus | null>(null);
  const [serverDown, setServerDown] = useState(false);
  const [loading, setLoading] = useState(true);
  const [completing, setCompleting] = useState(false);

  function loadStatus() {
    setLoading(true);
    setServerDown(false);
    getSetupStatus()
      .then((s) => {
        setStatus(s);
        setLoading(false);
        if (!s.github_configured) setStep(1);
        else setStep(2);
      })
      .catch(() => {
        setServerDown(true);
        setStatus(null);
        setLoading(false);
        setStep(0);
      });
  }

  useEffect(() => { loadStatus(); }, []);

  async function finish() {
    setCompleting(true);
    try {
      await completeSetup();
      toast.success("Setup complete! Welcome to PR Reviewer.");
      router.replace("/login");
    } catch {
      toast.error("Failed to save setup state.");
    } finally {
      setCompleting(false);
    }
  }

  if (loading) {
    return <div className="flex items-center justify-center h-screen text-muted-foreground">Loading…</div>;
  }

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-8">
      <div className="w-full max-w-2xl">
        <div className="mb-8 text-center">
          <div className="flex justify-center mb-4">
            <Image src="/logo.png" alt="PR Reviewer" width={1024} height={1024} style={{ height: '6rem', width: 'auto' }} priority />
          </div>
          <p className="text-base text-muted-foreground">Let's get your self-hosted instance configured.</p>
        </div>

        <StepIndicator
          current={step}
          status={status}
          serverDown={serverDown}
          onStepClick={setStep}
        />

        {step === 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-xl flex items-center gap-2">
                <Database className="h-5 w-5" />
                Database
              </CardTitle>
              <CardDescription className="text-sm">Checks that the backend is reachable and the database is connected.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              {serverDown ? (
                <>
                  <div className="rounded-lg border border-destructive/40 bg-destructive/5 p-4 space-y-3">
                    <div className="flex items-center gap-2 text-destructive font-semibold text-base">
                      <XCircle className="h-5 w-5 shrink-0" />
                      Backend server not reachable
                    </div>
                    <p className="text-sm text-muted-foreground">
                      The setup wizard could not connect to the API. Check the following:
                    </p>
                    <ul className="text-sm text-muted-foreground space-y-1.5 list-disc pl-5">
                      <li>Is the backend container / process running? (<code className="text-xs bg-muted px-1 rounded">docker compose up</code>)</li>
                      <li>Is <code className="text-xs bg-muted px-1 rounded">DATABASE_URL</code> set and the DB reachable?</li>
                      <li>Check server logs for migration or startup errors.</li>
                      <li>Confirm the API port is not blocked by a firewall.</li>
                    </ul>
                  </div>
                  <Button variant="outline" onClick={loadStatus}>Retry connection</Button>
                </>
              ) : (
                <>
                  <div className="space-y-2.5">
                    <div className="flex items-center gap-2.5 text-green-600 font-medium text-base">
                      <CheckCircle2 className="h-5 w-5 shrink-0" />
                      Backend server reachable
                    </div>
                    <div className="flex items-center gap-2.5 text-green-600 font-medium text-base">
                      <CheckCircle2 className="h-5 w-5 shrink-0" />
                      Database connection healthy
                    </div>
                    <div className="flex items-center gap-2.5 text-green-600 font-medium text-base">
                      <CheckCircle2 className="h-5 w-5 shrink-0" />
                      Migrations applied
                    </div>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    The backend is running and the database is ready. You can proceed to configure GitHub OAuth.
                  </p>
                  <Button size="lg" onClick={() => setStep(1)}>Next: GitHub OAuth →</Button>
                </>
              )}
            </CardContent>
          </Card>
        )}

        {step === 1 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-xl flex items-center gap-2">
                <GitBranch className="h-5 w-5" />
                GitHub OAuth
              </CardTitle>
              <CardDescription className="text-sm">
                Required — users sign in via GitHub. You need a GitHub OAuth App for this instance.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              {status?.github_configured ? (
                <>
                  <div className="space-y-2.5">
                    <div className="flex items-center gap-2.5 text-green-600 font-medium text-base">
                      <CheckCircle2 className="h-5 w-5 shrink-0" />
                      <code className="text-sm font-mono">GITHUB_CLIENT_ID</code> is set
                    </div>
                    <div className="flex items-center gap-2.5 text-green-600 font-medium text-base">
                      <CheckCircle2 className="h-5 w-5 shrink-0" />
                      <code className="text-sm font-mono">GITHUB_CLIENT_SECRET</code> is set
                    </div>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    GitHub OAuth is configured. Users will be able to sign in with their GitHub account.
                  </p>
                  <Button size="lg" onClick={() => setStep(2)}>Next: Launch →</Button>
                </>
              ) : (
                <>
                  <div className="rounded-lg border border-amber-300 bg-amber-50 dark:bg-amber-950/20 p-4 space-y-3">
                    <div className="flex items-center gap-2.5 font-semibold text-base text-amber-800 dark:text-amber-300">
                      <XCircle className="h-5 w-5 shrink-0" />
                      GitHub OAuth credentials not set
                    </div>
                    <p className="text-sm text-muted-foreground">
                      Set <code className="text-xs bg-muted px-1 rounded">GITHUB_CLIENT_ID</code> and{" "}
                      <code className="text-xs bg-muted px-1 rounded">GITHUB_CLIENT_SECRET</code> in your environment, then restart the server.
                    </p>
                  </div>

                  <div className="space-y-2">
                    <p className="text-sm font-medium">How to create a GitHub OAuth App under your organization:</p>
                    <ol className="text-sm text-muted-foreground space-y-2 list-decimal pl-5">
                      <li>
                        On GitHub, go to your <strong>organization page</strong> → <strong>Settings</strong> → <strong>Developer settings</strong> → <strong>OAuth Apps</strong> → <strong>New OAuth App</strong>
                      </li>
                      <li>
                        Set <strong>Authorization callback URL</strong> to:
                        <div className="mt-1 rounded bg-muted px-3 py-1.5 font-mono text-xs">
                          {process.env.NEXT_PUBLIC_API_URL}/auth/github/callback
                        </div>
                      </li>
                      <li>Click <strong>Register application</strong>, then copy the Client ID and generate a Client Secret</li>
                      <li>Add both to your <code className="text-xs bg-muted px-1 rounded">.env</code> and restart the server</li>
                    </ol>
                  </div>

                  <Button variant="outline" onClick={() => { loadStatus(); toast.info("Status refreshed"); }}>
                    Refresh status
                  </Button>
                </>
              )}
            </CardContent>
          </Card>
        )}

        {step === 2 && (
          <Card>
            <CardHeader>
              <CardTitle className="text-xl flex items-center gap-2">
                <Rocket className="h-5 w-5" />
                Ready to launch
              </CardTitle>
              <CardDescription className="text-sm">Review your setup summary before completing.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="rounded-lg border bg-muted/30 p-4 space-y-3">
                <p className="text-sm font-medium text-muted-foreground uppercase tracking-wide">Setup summary</p>
                <div className="space-y-2.5 text-base">
                  <div className="flex items-center gap-2.5">
                    {!serverDown
                      ? <CheckCircle2 className="h-5 w-5 shrink-0 text-green-500" />
                      : <XCircle className="h-5 w-5 shrink-0 text-destructive" />}
                    <span>Database — {serverDown ? <span className="text-destructive">server not reachable</span> : "connected"}</span>
                  </div>
                  <div className="flex items-center gap-2.5">
                    {status?.github_configured
                      ? <CheckCircle2 className="h-5 w-5 shrink-0 text-green-500" />
                      : <XCircle className="h-5 w-5 shrink-0 text-destructive" />}
                    <span>GitHub OAuth — {status?.github_configured ? "configured" : <span className="text-destructive">not configured</span>}</span>
                  </div>
                </div>
              </div>

              {!status?.github_configured && (
                <div className="rounded-lg border border-amber-300 bg-amber-50 dark:bg-amber-950/20 p-3 text-sm text-amber-800 dark:text-amber-300">
                  GitHub OAuth is not configured. Go back to step 2 and set it up before completing — users will not be able to sign in without it.
                </div>
              )}

              <div className="space-y-2">
                <p className="text-sm font-medium">What happens next</p>
                <ul className="text-sm text-muted-foreground space-y-1.5 list-disc pl-5">
                  <li>You'll be taken to the login page — sign in with your GitHub account to become the <strong>owner</strong></li>
                  <li>Connect a <strong>GitHub App</strong> in Settings to enable PR review webhooks</li>
                  <li>Add an <strong>AI provider</strong> (OpenAI, Anthropic, etc.) in Settings</li>
                  <li>Invite your team members from the <strong>Team</strong> page</li>
                </ul>
              </div>

              <Button
                size="lg"
                onClick={finish}
                disabled={completing || !status?.github_configured || serverDown}
              >
                {completing ? "Finishing…" : "Complete setup & go to login"}
              </Button>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  );
}
