"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { AlertCircle, ShieldX, Mail, Building2 } from "lucide-react";

const REASONS: Record<string, { icon: React.ElementType; title: string; body: (p: URLSearchParams) => string }> = {
  invite_required: {
    icon: Mail,
    title: "Invitation required",
    body: () =>
      "This deployment is invite-only. Ask an admin to send you an invite link — you'll receive it by email.",
  },
  invite_invalid: {
    icon: AlertCircle,
    title: "Invite link expired or invalid",
    body: () =>
      "This invite link is no longer valid. It may have expired (links are valid for 7 days) or already been used. Ask an admin to resend your invite.",
  },
  email_mismatch: {
    icon: Mail,
    title: "Email address not verified on GitHub",
    body: (p) => {
      const email = p.get("email") ?? "";
      const login = p.get("login") ?? "";
      return `Your GitHub account (${login || "unknown"}) doesn't have ${email || "the invited email"} as a verified email address. Go to GitHub → Settings → Emails, add and verify that address, then click the invite link again.`;
    },
  },
  org_required: {
    icon: Building2,
    title: "Organization membership required",
    body: (p) => {
      const org = p.get("org") ?? "the required GitHub organization";
      return `You must be a member of ${org} to sign in. Ask an org admin to add you, then try again.`;
    },
  },
  suspended: {
    icon: ShieldX,
    title: "Account suspended",
    body: () =>
      "Your account has been suspended. Contact an admin if you believe this is a mistake.",
  },
};

function ErrorContent() {
  const params = useSearchParams();
  const reason = params.get("reason") ?? "unknown";
  const def = REASONS[reason] ?? {
    icon: AlertCircle,
    title: "Sign-in failed",
    body: () => `Something went wrong during sign-in (reason: ${reason}). Please try again or contact an admin.`,
  };
  const Icon = def.icon;

  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <div className="w-full max-w-md space-y-6 p-8 text-center">
        <div className="flex justify-center">
          <div className="rounded-full bg-destructive/10 p-4">
            <Icon className="h-8 w-8 text-destructive" />
          </div>
        </div>
        <div className="space-y-2">
          <h1 className="text-2xl font-bold">{def.title}</h1>
          <p className="text-muted-foreground text-sm leading-relaxed">{def.body(params)}</p>
        </div>
        <Link href="/login" className={cn(buttonVariants({ variant: "outline" }), "w-full")}>
          Back to sign in
        </Link>
      </div>
    </div>
  );
}

export default function AuthErrorPage() {
  return (
    <Suspense>
      <ErrorContent />
    </Suspense>
  );
}
