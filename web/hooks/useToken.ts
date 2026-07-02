"use client";

import { useEffect, useLayoutEffect, useState, useCallback } from "react";
import { getToken, clearToken } from "@/lib/auth";

// useLayoutEffect on the client so the token is read before first paint,
// preventing a flash where admin-only nav items are briefly hidden on refresh.
const useIsomorphicLayoutEffect = typeof window !== "undefined" ? useLayoutEffect : useEffect;

function decodePayload(token: string): { role: string; userId: number } {
  try {
    const payload = JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));
    return { role: (payload.role as string) ?? "viewer", userId: (payload.sub as number) ?? 0 };
  } catch {
    return { role: "viewer", userId: 0 };
  }
}

export function useToken() {
  const [token, setToken] = useState<string | null>(null);

  useIsomorphicLayoutEffect(() => {
    setToken(getToken());
  }, []);

  const logout = useCallback(() => {
    clearToken();
    setToken(null);
    window.location.href = "/login";
  }, []);

  const { role, userId } = token ? decodePayload(token) : { role: null, userId: 0 };
  const isAdmin = role === "owner" || role === "admin";

  return { token, logout, isLoading: token === null, role, isAdmin, userId };
}
