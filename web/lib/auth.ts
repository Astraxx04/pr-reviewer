"use client";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8001";

const TOKEN_KEY = "pr_reviewer_token";

export function getToken(): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(/(?:^|;\s*)pr_reviewer_token=([^;]*)/);
  return match ? decodeURIComponent(match[1]) : null;
}

export function setToken(token: string): void {
  document.cookie = `${TOKEN_KEY}=${encodeURIComponent(token)}; path=/; max-age=${60 * 60 * 24 * 30}; samesite=lax`;
}

export function clearToken(): void {
  document.cookie = `${TOKEN_KEY}=; path=/; max-age=0`;
}

export function loginUrl(): string {
  return `${API_URL}/auth/github`;
}
