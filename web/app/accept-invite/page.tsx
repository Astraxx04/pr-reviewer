"use client";

import { useEffect, useState, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { validateInviteToken } from "@/lib/api";
import { AlertCircle, CheckCircle2 } from "lucide-react";

import { API_URL as API_BASE } from "@/lib/auth";

function AcceptInviteContent() {
  const params = useSearchParams();
  const token = params.get("token") ?? "";
  const [state, setState] = useState<"loading" | "valid" | "invalid">("loading");
  const [info, setInfo] = useState<{ email: string; role: string; invited_by: string } | null>(null);

  useEffect(() => {
    if (!token) {
      setState("invalid");
      return;
    }
    validateInviteToken(token)
      .then((data) => {
        setInfo(data);
        setState("valid");
      })
      .catch(() => setState("invalid"));
  }, [token]);

  if (state === "loading") {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <Skeleton className="h-48 w-80" />
      </div>
    );
  }

  if (state === "invalid") {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background">
        <div className="w-full max-w-md space-y-4 p-8 text-center">
          <div className="flex justify-center">
            <div className="rounded-full bg-destructive/10 p-4">
              <AlertCircle className="h-8 w-8 text-destructive" />
            </div>
          </div>
          <h1 className="text-2xl font-bold">Invite link expired or invalid</h1>
          <p className="text-muted-foreground text-sm">
            This invite link is no longer valid. It may have expired (links are valid for 7 days)
            or already been used. Ask an admin to resend your invite.
          </p>
        </div>
      </div>
    );
  }

  const loginUrl = `${API_BASE}/auth/github?invite=${encodeURIComponent(token)}`;

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-md space-y-6 p-8 text-center">
        <div className="flex justify-center">
          <div className="rounded-full bg-green-500/10 p-4">
            <CheckCircle2 className="h-8 w-8 text-green-500" />
          </div>
        </div>
        <div className="space-y-2">
          <h1 className="text-2xl font-bold">You&apos;ve been invited</h1>
          {info && (
            <p className="text-muted-foreground text-sm">
              <strong>{info.invited_by}</strong> invited <strong>{info.email}</strong> to join as{" "}
              <strong>{info.role}</strong>.
            </p>
          )}
          <p className="text-muted-foreground text-sm mt-2">
            Sign in with the GitHub account that has <strong>{info?.email}</strong> as a verified
            email address to accept.
          </p>
        </div>
        <a href={loginUrl} className="block">
          <Button className="w-full gap-2" size="lg">
            <svg viewBox="0 0 24 24" className="h-5 w-5 fill-current">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
            </svg>
            Accept invitation with GitHub
          </Button>
        </a>
      </div>
    </div>
  );
}

export default function AcceptInvitePage() {
  return (
    <Suspense>
      <AcceptInviteContent />
    </Suspense>
  );
}
