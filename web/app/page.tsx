"use client";

import { useEffect, Suspense } from "react";
import { useRouter } from "next/navigation";
import { getToken } from "@/lib/auth";
import { getSetupStatus } from "@/lib/api";

function Redirect() {
  const router = useRouter();

  useEffect(() => {
    getSetupStatus()
      .then((status) => {
        if (!status.setup_complete) {
          router.replace("/setup");
        } else if (getToken()) {
          router.replace("/dashboard");
        } else {
          router.replace("/login");
        }
      })
      .catch(() => {
        if (getToken()) {
          router.replace("/dashboard");
        } else {
          router.replace("/login");
        }
      });
  }, [router]);

  return <p className="p-8 text-muted-foreground">Redirecting…</p>;
}

export default function HomePage() {
  return (
    <Suspense>
      <Redirect />
    </Suspense>
  );
}
