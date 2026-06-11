"use client";

import { useEffect, useRef } from "react";
import { API_URL } from "@/lib/auth";

export interface SSEEvent {
  type: string;
  data: unknown;
}

export function useSSE(token: string | null, onEvent: (event: SSEEvent) => void) {
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    if (!token) return;

    const url = `${API_URL}/api/events?token=${encodeURIComponent(token)}&ngrok-skip-browser-warning=true`;
    const es = new EventSource(url);

    es.onmessage = (e: MessageEvent) => {
      try {
        const event = JSON.parse(e.data as string) as SSEEvent;
        onEventRef.current(event);
      } catch {
        // ignore malformed events
      }
    };

    es.onerror = () => {
      // EventSource auto-reconnects on error; nothing to do here
    };

    return () => es.close();
  }, [token]);
}
