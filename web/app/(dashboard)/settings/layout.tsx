"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useToken } from "@/hooks/useToken";

// Settings is admin/owner only. The API enforces this server-side (RequireRole);
// this guard keeps non-admins from landing on a page that would just 403.
export default function SettingsLayout({ children }: { children: React.ReactNode }) {
  const { isAdmin, isLoading } = useToken();
  const router = useRouter();

  useEffect(() => {
    if (!isLoading && !isAdmin) router.replace("/dashboard");
  }, [isAdmin, isLoading, router]);

  if (isLoading || !isAdmin) return null;
  return <>{children}</>;
}
