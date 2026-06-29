"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { API_URL } from "@/lib/auth";

function Consent() {
  const params = useSearchParams();
  const token = params.get("t") ?? "";
  const login = params.get("u") ?? "";

  if (!token) {
    return (
      <p className="text-muted-foreground">
        Invalid or expired sign-in request. Please start over.
      </p>
    );
  }

  return (
    <Card className="w-full max-w-sm">
      <CardHeader className="text-center">
        <CardTitle>Authorize sign-in</CardTitle>
        <CardDescription>
          {login ? (
            <>
              You&apos;re signing in to PR Reviewer as{" "}
              <span className="font-semibold text-foreground">{login}</span>.
            </>
          ) : (
            "Confirm sign-in to PR Reviewer."
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {/* Plain form POST (full navigation) so the browser follows the backend's
            redirect to /auth/callback with the real token. The session and token
            are only created once this is submitted. */}
        <form method="POST" action={`${API_URL}/auth/github/continue`}>
          <input type="hidden" name="t" value={token} />
          <Button
            type="submit"
            size="lg"
            className="w-full"
            style={{ cursor: "pointer" }}
          >
            Yes, continue
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}

export default function ConsentPage() {
  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-8">
      <Suspense>
        <Consent />
      </Suspense>
    </div>
  );
}
