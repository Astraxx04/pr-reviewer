"use client";

import { useEffect, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { setToken } from "@/lib/auth";

function AuthCallback() {
  const router = useRouter();
  const params = useSearchParams();

  useEffect(() => {
    const token = params.get("token");
    if (token) {
      setToken(token);
    }
    router.replace("/dashboard");
  }, [router, params]);

  return <p className="p-8 text-muted-foreground">Signing in…</p>;
}

export default function AuthCallbackPage() {
  return (
    <Suspense>
      <AuthCallback />
    </Suspense>
  );
}
