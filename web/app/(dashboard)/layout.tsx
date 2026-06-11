"use client";

import { useState } from "react";
import { Menu } from "lucide-react";
import { Sidebar } from "@/components/layout/sidebar";
import { Button } from "@/components/ui/button";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Mobile header bar */}
      <div className="fixed top-0 left-0 right-0 z-20 flex items-center gap-3 border-b bg-card px-4 py-3 md:hidden">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={() => setSidebarOpen(true)}
          aria-label="Open navigation menu"
          aria-expanded={sidebarOpen}
          aria-controls="sidebar"
        >
          <Menu className="h-5 w-5" aria-hidden="true" />
        </Button>
        <span className="font-semibold">PR Reviewer</span>
      </div>

      {/* Sidebar — hidden on mobile until hamburger is tapped */}
      <Sidebar open={sidebarOpen} onClose={() => setSidebarOpen(false)} />

      {/* Main content — top padding on mobile to clear the fixed header */}
      <main
        className="flex-1 overflow-y-auto p-8 pt-20 md:pt-8"
        id="main-content"
        tabIndex={-1}
      >
        {children}
      </main>
    </div>
  );
}
