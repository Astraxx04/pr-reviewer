"use client";

import { useEffect, useState, useCallback } from "react";
import { getToken, clearToken } from "@/lib/auth";

function decodeRole(token: string): string {
  try {
    const payload = JSON.parse(atob(token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/")));
    return (payload.role as string) ?? "viewer";
  } catch {
    return "viewer";
  }
}

export function useToken() {
  const [token, setToken] = useState<string | null>(null);

  useEffect(() => {
    setToken(getToken());
  }, []);

  const logout = useCallback(() => {
    clearToken();
    setToken(null);
    window.location.href = "/login";
  }, []);

  const role = token ? decodeRole(token) : null;
  const isAdmin = role === "owner" || role === "admin";

  return { token, logout, isLoading: token === null, role, isAdmin };
}
