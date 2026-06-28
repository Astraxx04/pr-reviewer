"use client";

import { useEffect, Suspense } from "react";
import { useRouter } from "next/navigation";
import Image from "next/image";
import { Button } from "@/components/ui/button";
import { getSetupStatus } from "@/lib/api";

const loginUrl = `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/auth/github`;

function SetupGuard() {
  const router = useRouter();

  useEffect(() => {
    getSetupStatus()
      .then((status) => {
        if (!status.setup_complete) {
          router.replace("/setup");
        }
      })
      .catch(() => {});
  }, [router]);

  return null;
}

export default function LoginPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background">
      <Suspense>
        <SetupGuard />
      </Suspense>
      <div className="w-full max-w-sm space-y-8 p-8">
        <div className="text-center">
          <div className="flex justify-center mb-4">
            <Image src="/logo.png" alt="PR Reviewer" width={1024} height={1024} style={{ height: '10rem', width: 'auto' }} priority />
          </div>
          <p className="mt-2 text-muted-foreground">AI-powered pull request analysis</p>
        </div>
        <a href={loginUrl} className="block">
          <Button className="w-full gap-2" size="lg">
            <svg viewBox="0 0 24 24" className="h-5 w-5 fill-current">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
            </svg>
            Sign in with GitHub
          </Button>
        </a>
      </div>
    </div>
  );
}
