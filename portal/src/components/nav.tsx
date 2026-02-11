"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession, signOut } from "next-auth/react";
import { Button } from "@/components/ui/button";
import { WaveformLogo } from "@/components/waveform-logo";

const navItems = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/create", label: "Create" },
  { href: "/api-keys", label: "API Keys" },
  { href: "/usage", label: "Usage" },
  { href: "/docs", label: "Docs" },
];

const adminItems = [
  { href: "/admin/users", label: "Users" },
  { href: "/admin/usage", label: "All Usage" },
];

export function Nav() {
  const pathname = usePathname();
  const { data: session } = useSession();

  if (!session) return null;

  const isAdmin = session.user?.role === "admin";

  return (
    <nav className="sticky top-0 z-50 border-b border-border/40 bg-card/80 backdrop-blur-md">
      <div className="mx-auto flex h-14 max-w-6xl items-center px-4 gap-6">
        <Link
          href="/dashboard"
          className="flex items-center gap-2"
        >
          <WaveformLogo size={20} />
          <span className="font-semibold text-lg text-primary">Podcaster</span>
        </Link>
        <div className="flex items-center gap-1">
          {navItems.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={`rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                pathname.startsWith(item.href)
                  ? "bg-primary/10 text-primary font-semibold"
                  : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
              }`}
            >
              {item.label}
            </Link>
          ))}
          {isAdmin && (
            <>
              <div className="mx-2 h-4 w-px bg-border" />
              {adminItems.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                    pathname.startsWith(item.href)
                      ? "bg-primary/10 text-primary font-semibold"
                      : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
                  }`}
                >
                  {item.label}
                </Link>
              ))}
            </>
          )}
        </div>
        <div className="ml-auto flex items-center gap-3">
          <span className="text-sm text-muted-foreground">
            {session.user?.name || session.user?.email}
          </span>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => signOut({ callbackUrl: "/" })}
          >
            Sign out
          </Button>
        </div>
      </div>
    </nav>
  );
}
