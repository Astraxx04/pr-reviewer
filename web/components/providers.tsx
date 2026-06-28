"use client";

import { ThemeProvider } from "next-themes";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    // To re-enable theme toggle: replace forcedTheme="dark" with defaultTheme="system" enableSystem
    <ThemeProvider attribute="class" forcedTheme="dark" disableTransitionOnChange>
      {children}
    </ThemeProvider>
  );
}
