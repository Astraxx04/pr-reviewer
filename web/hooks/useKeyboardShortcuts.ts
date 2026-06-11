"use client";

import { useEffect, useRef } from "react";
import { useRouter } from "next/navigation";

export interface ShortcutDefinition {
  keys: string; // display string e.g. "g d", "?"
  description: string;
}

export const SHORTCUTS: ShortcutDefinition[] = [
  { keys: "g d", description: "Go to Dashboard" },
  { keys: "g r", description: "Go to Reviews" },
  { keys: "g p", description: "Go to Pull Requests" },
  { keys: "g a", description: "Go to Analytics" },
  { keys: "j", description: "Next item in list" },
  { keys: "k", description: "Previous item in list" },
  { keys: "o / Enter", description: "Open selected item" },
  { keys: "?", description: "Show keyboard shortcuts" },
];

interface UseKeyboardShortcutsOptions {
  onShowHelp: () => void;
  listItemSelector?: string;
  openItemCallback?: (el: Element) => void;
}

export function useKeyboardShortcuts({
  onShowHelp,
  listItemSelector,
  openItemCallback,
}: UseKeyboardShortcutsOptions) {
  const router = useRouter();
  const gPressed = useRef(false);
  const selectedIndex = useRef(-1);

  useEffect(() => {
    let gTimer: ReturnType<typeof setTimeout> | null = null;

    function getItems(): NodeListOf<Element> | Element[] {
      if (!listItemSelector) return [];
      return document.querySelectorAll(listItemSelector);
    }

    function setSelected(index: number) {
      const items = getItems();
      if (!items.length) return;
      const len = Array.from(items).length;
      const next = Math.max(0, Math.min(len - 1, index));
      selectedIndex.current = next;
      const el = Array.from(items)[next];
      if (el instanceof HTMLElement) {
        el.focus();
        el.scrollIntoView({ block: "nearest" });
      }
    }

    function handleKeyDown(e: KeyboardEvent) {
      // Don't fire shortcuts when typing in inputs/textareas.
      const tag = (e.target as HTMLElement).tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
      if ((e.target as HTMLElement).isContentEditable) return;

      const key = e.key;

      // Handle "g ?" chord shortcuts.
      if (key === "g" && !e.ctrlKey && !e.metaKey && !e.altKey) {
        gPressed.current = true;
        if (gTimer) clearTimeout(gTimer);
        gTimer = setTimeout(() => { gPressed.current = false; }, 800);
        return;
      }

      if (gPressed.current) {
        gPressed.current = false;
        if (gTimer) clearTimeout(gTimer);
        switch (key) {
          case "d": router.push("/dashboard"); return;
          case "r": router.push("/reviews"); return;
          case "p": router.push("/prs"); return;
          case "a": router.push("/analytics"); return;
        }
        return;
      }

      switch (key) {
        case "?":
          onShowHelp();
          break;
        case "j": {
          const items = getItems();
          if (Array.from(items).length) {
            setSelected(selectedIndex.current + 1);
          }
          break;
        }
        case "k": {
          const items = getItems();
          if (Array.from(items).length) {
            setSelected(Math.max(0, selectedIndex.current - 1));
          }
          break;
        }
        case "o":
        case "Enter": {
          if (key === "Enter") break; // let native Enter behaviour through
          const items = Array.from(getItems());
          const el = items[selectedIndex.current];
          if (el) {
            if (openItemCallback) {
              openItemCallback(el);
            } else {
              const link = el.querySelector("a") ?? (el instanceof HTMLAnchorElement ? el : null);
              if (link) (link as HTMLElement).click();
            }
          }
          break;
        }
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      if (gTimer) clearTimeout(gTimer);
    };
  }, [router, onShowHelp, listItemSelector, openItemCallback]);
}
