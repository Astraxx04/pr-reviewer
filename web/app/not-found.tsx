"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";

export default function NotFound() {
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    const segments = pathname.split("/").filter(Boolean);
    const parent = segments.length > 1 ? "/" + segments.slice(0, -1).join("/") : "/";
    router.replace(parent);
  }, [pathname, router]);

  return null;
}
