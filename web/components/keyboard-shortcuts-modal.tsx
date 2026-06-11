"use client";

import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { SHORTCUTS } from "@/hooks/useKeyboardShortcuts";

interface KeyboardShortcutsModalProps {
  open: boolean;
  onClose: () => void;
}

export function KeyboardShortcutsModal({ open, onClose }: KeyboardShortcutsModalProps) {
  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Keyboard Shortcuts</DialogTitle>
        </DialogHeader>
        <table className="w-full text-sm" role="table">
          <tbody>
            {SHORTCUTS.map((s) => (
              <tr key={s.keys} className="border-b last:border-0">
                <td className="py-2 pr-4 font-mono">
                  {s.keys.split(" / ").map((k, i) => (
                    <span key={k}>
                      {i > 0 && <span className="text-muted-foreground mx-1">/</span>}
                      <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 text-xs">
                        {k}
                      </kbd>
                    </span>
                  ))}
                </td>
                <td className="py-2 text-muted-foreground">{s.description}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <p className="text-xs text-muted-foreground">
          Shortcuts are disabled when focus is inside an input field.
        </p>
      </DialogContent>
    </Dialog>
  );
}
