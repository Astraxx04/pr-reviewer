"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { useTheme } from "next-themes";
import { cn } from "@/lib/utils";
import { useToken } from "@/hooks/useToken";
import {
  LayoutDashboard, GitPullRequest, Database, Users, BarChart2, Settings,
  LogOut, GitMerge, Moon, Sun, X,
} from "lucide-react";
import { Button } from "@/components/ui/button";

const nav = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/repos", label: "Repositories", icon: Database },
  { href: "/prs", label: "Pull Requests", icon: GitMerge },
  { href: "/reviews", label: "Reviews", icon: GitPullRequest },
  { href: "/team", label: "Team", icon: Users, adminOnly: true },
  { href: "/analytics", label: "Analytics", icon: BarChart2 },
  // Settings is admin/owner only — its endpoints are gated server-side too.
  { href: "/settings", label: "Settings", icon: Settings, adminOnly: true },
];

interface SidebarProps {
  open?: boolean;
  onClose?: () => void;
}

export function Sidebar({ open, onClose }: SidebarProps) {
  const pathname = usePathname();
  const { logout, isAdmin } = useToken();
  const visibleNav = nav.filter((item) => !item.adminOnly || isAdmin);
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  const isMobileControlled = open !== undefined;

  return (
    <>
      {/* Mobile overlay backdrop */}
      {isMobileControlled && open && (
        <div
          className="fixed inset-0 z-30 bg-black/40 md:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          "flex h-screen w-56 flex-col border-r bg-card px-3 py-4",
          // Desktop: always visible
          "md:relative md:translate-x-0 md:flex",
          // Mobile: slide in/out
          isMobileControlled
            ? cn(
                "fixed inset-y-0 left-0 z-40 transition-transform duration-200 md:static md:z-auto",
                open ? "translate-x-0" : "-translate-x-full"
              )
            : "hidden md:flex"
        )}
        aria-label="Main navigation"
      >
        <div className="mb-6 flex items-center justify-between px-3">
          <Link href="/dashboard" className="text-lg font-bold focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring rounded-sm">
            PR Reviewer
          </Link>
          {isMobileControlled && (
            <Button
              variant="ghost"
              size="icon"
              className="md:hidden h-7 w-7"
              onClick={onClose}
              aria-label="Close navigation"
            >
              <X className="h-4 w-4" />
            </Button>
          )}
        </div>

        <nav className="flex flex-1 flex-col gap-1" aria-label="Site navigation">
          {visibleNav.map(({ href, label, icon: Icon }) => (
            <Link
              key={href}
              href={href}
              className={cn(
                "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
                pathname === href
                  ? "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              )}
              aria-current={pathname === href ? "page" : undefined}
              onClick={onClose}
            >
              <Icon className="h-4 w-4" aria-hidden="true" />
              {label}
            </Link>
          ))}
        </nav>

        <div className="flex items-center gap-1 border-t pt-3 mt-2">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 text-muted-foreground"
            onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
            aria-label={mounted ? (resolvedTheme === "dark" ? "Switch to light mode" : "Switch to dark mode") : "Toggle theme"}
            suppressHydrationWarning
          >
            {mounted && resolvedTheme === "dark" ? (
              <Sun className="h-4 w-4" aria-hidden="true" />
            ) : (
              <Moon className="h-4 w-4" aria-hidden="true" />
            )}
          </Button>
          <Button
            variant="ghost"
            className="flex-1 justify-start gap-3 text-muted-foreground"
            onClick={logout}
            aria-label="Sign out"
          >
            <LogOut className="h-4 w-4" aria-hidden="true" />
            Sign out
          </Button>
        </div>
      </aside>
    </>
  );
}
