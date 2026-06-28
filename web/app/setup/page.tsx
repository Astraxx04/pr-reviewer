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
            className={`flex items-center gap-2 px-4 py-2 rounded-full text-sm font-medium transition-colors ${
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
          <p className="text-muted-foreground">Let's get your self-hosted instance configured.</p>
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
              <CardTitle className="flex items-center gap-2">
                <Database className="h-5 w-5" />
                Database
              </CardTitle>
              <CardDescription>Verifies the backend is reachable and the database is connected.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {serverDown ? (
                <div className="rounded-lg border border-destructive/40 bg-destructive/5 p-4 space-y-2">
                  <div className="flex items-center gap-2 text-destructive font-medium">
                    <XCircle className="h-5 w-5" />
                    Server not reachable
                  </div>
                  <p className="text-sm text-muted-foreground">
                    Make sure the backend server is running and the API is accessible before continuing.
                  </p>
                  <Button variant="outline" size="sm" onClick={loadStatus} style={{ cursor: 'pointer' }}>
                    Retry
                  </Button>
                </div>
              ) : (
                <>
                  <div className="flex items-center gap-2 text-green-600 font-medium">
                    <CheckCircle2 className="h-5 w-5" />
                    Database connected
                  </div>
                  <p className="text-sm text-muted-foreground">
                    The backend is running and the database connection is healthy.
                  </p>
                  <Button onClick={() => setStep(1)} style={{ cursor: 'pointer' }}>Next: GitHub →</Button>
                </>
              )}
            </CardContent>
          </Card>
        )}

        {step === 1 && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <GitBranch className="h-5 w-5" />
                GitHub OAuth
              </CardTitle>
              <CardDescription>
                Required — without this, users cannot sign in.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {status?.github_configured ? (
                <>
                  <div className="flex items-center gap-2 text-green-600 font-medium">
                    <CheckCircle2 className="h-5 w-5" />
                    GitHub OAuth configured
                  </div>
                  <Button onClick={() => setStep(2)} style={{ cursor: 'pointer' }}>Next: Launch →</Button>
                </>
              ) : (
                <>
                  <div className="rounded-lg border border-amber-300 bg-amber-50 dark:bg-amber-950/20 p-4 text-sm space-y-3">
                    <div className="flex items-center gap-2 font-medium text-amber-800 dark:text-amber-300">
                      <XCircle className="h-4 w-4" />
                      GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET are not set
                    </div>
                    <ol className="list-decimal list-inside space-y-1 text-muted-foreground">
                      <li>Go to GitHub → Settings → Developer settings → OAuth Apps → New OAuth App</li>
                      <li>Set <strong>Authorization callback URL</strong> to <code>{"{SERVER_URL}/auth/github/callback"}</code></li>
                      <li>Copy the Client ID + Secret into your environment and restart the server</li>
                    </ol>
                  </div>
                  <Button variant="outline" onClick={() => { loadStatus(); toast.info("Status refreshed"); }} style={{ cursor: 'pointer' }}>
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
              <CardTitle className="flex items-center gap-2">
                <Rocket className="h-5 w-5" />
                Ready to launch
              </CardTitle>
              <CardDescription>Review your setup and complete the wizard.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-3 text-sm">
                <div className="flex items-center gap-2">
                  {!serverDown
                    ? <CheckCircle2 className="h-4 w-4 text-green-500" />
                    : <XCircle className="h-4 w-4 text-destructive" />}
                  Database {serverDown ? "— server not reachable" : "connected"}
                </div>
                <div className="flex items-center gap-2">
                  {status?.github_configured
                    ? <CheckCircle2 className="h-4 w-4 text-green-500" />
                    : <XCircle className="h-4 w-4 text-destructive" />}
                  GitHub OAuth {status?.github_configured ? "configured" : "— not configured, users cannot sign in"}
                </div>
              </div>
              {!status?.github_configured && (
                <p className="text-sm text-amber-700 dark:text-amber-400">
                  GitHub OAuth is not configured. Go back and set it up before completing.
                </p>
              )}
              <p className="text-sm text-muted-foreground">
                Add an AI provider and configure your GitHub App in <strong>Settings</strong> after logging in.
              </p>
              <Button
                onClick={finish}
                disabled={completing || !status?.github_configured || serverDown}
                style={{ cursor: 'pointer' }}
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
