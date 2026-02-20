"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { signOut } from "next-auth/react";
import { Button } from "@/components/ui/button";
import { WaveformLogo } from "@/components/waveform-logo";

export function PublicNav({ isLoggedIn }: { isLoggedIn: boolean }) {
  const pathname = usePathname();

  return (
    <nav className="sticky top-0 z-50 border-b border-border/40 bg-card/80 backdrop-blur-md">
      <div className="mx-auto flex h-14 max-w-6xl items-center px-4 gap-6">
        <Link href="/" className="flex items-center gap-2">
          <WaveformLogo size={20} />
          <span className="font-semibold text-lg text-primary">Podcaster</span>
        </Link>

        <div className="flex items-center gap-1">
          <Link
            href="/podcasts"
            className={`rounded-md px-3 py-2 text-sm font-medium transition-colors ${
              pathname === "/podcasts"
                ? "bg-primary/10 text-primary font-semibold"
                : "text-muted-foreground hover:text-foreground hover:bg-accent/50"
            }`}
          >
            Podcasts
          </Link>
        </div>

        <div className="ml-auto flex items-center gap-3">
          {isLoggedIn ? (
            <>
              <Link href="/dashboard">
                <Button variant="ghost" size="sm">
                  Dashboard
                </Button>
              </Link>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => signOut({ callbackUrl: "/" })}
              >
                Sign out
              </Button>
            </>
          ) : (
            <Link href="/login">
              <Button size="sm">Sign in</Button>
            </Link>
          )}
        </div>
      </div>
    </nav>
  );
}
