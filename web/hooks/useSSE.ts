"use client";

import { useEffect, useRef } from "react";
import { API_URL } from "@/lib/auth";

export interface SSEEvent {
  type: string;
  data: unknown;
}

const isDev = process.env.NODE_ENV === "development";
const SSE_BASE = isDev ? "http://localhost:8001" : API_URL;
const INITIAL_RETRY_MS = 1_000;
const MAX_RETRY_MS = 30_000;

export function useSSE(token: string | null, onEvent: (event: SSEEvent) => void) {
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    if (!token) return;

    let es: EventSource | null = null;
    let retryMs = INITIAL_RETRY_MS;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;
    let stopped = false;

    function connect() {
      const params = new URLSearchParams({ token: token! });

      es = new EventSource(`${SSE_BASE}/api/events?${params}`);

      es.onopen = () => {
        retryMs = INITIAL_RETRY_MS;
      };

      es.onmessage = (e: MessageEvent) => {
        retryMs = INITIAL_RETRY_MS;
        try {
          const event = JSON.parse(e.data as string) as SSEEvent;
          onEventRef.current(event);
        } catch {
          // ignore malformed events
        }
      };

      es.onerror = () => {
        es?.close();
        es = null;
        if (stopped) return;
        retryTimer = setTimeout(() => {
          if (!stopped) connect();
        }, retryMs);
        retryMs = Math.min(retryMs * 2, MAX_RETRY_MS);
      };
    }

    connect();

    return () => {
      stopped = true;
      if (retryTimer !== null) clearTimeout(retryTimer);
      es?.close();
    };
  }, [token]);
}
