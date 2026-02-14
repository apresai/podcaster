"use client";

import { useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSession, signOut } from "next-auth/react";
import { Button } from "@/components/ui/button";
import { WaveformLogo } from "@/components/waveform-logo";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Menu } from "lucide-react";
import { canCreate } from "@/lib/auth";

const baseNavItems = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/docs", label: "Docs" },
];

const creatorNavItems = [
  { href: "/create", label: "Create" },
  { href: "/api-keys", label: "API Keys" },
  { href: "/usage", label: "Usage" },
];

const adminItems = [
  { href: "/admin/users", label: "Users" },
  { href: "/admin/usage", label: "All Usage" },
];

export function Nav() {
  const pathname = usePathname();
  const { data: session } = useSession();
  const [open, setOpen] = useState(false);

  if (!session) return null;

  const isAdmin = session.user?.role === "admin";
  const isCreator = canCreate(session.user?.role);

  const navItems = [
    ...baseNavItems,
    ...(isCreator ? creatorNavItems : []),
  ];

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

        {/* Desktop nav */}
        <div className="hidden md:flex items-center gap-1">
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

        {/* Desktop user info */}
        <div className="hidden md:flex ml-auto items-center gap-3">
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

        {/* Mobile hamburger */}
        <Sheet open={open} onOpenChange={setOpen}>
          <SheetTrigger asChild>
            <Button variant="ghost" size="icon" className="ml-auto md:hidden">
              <Menu className="size-5" />
              <span className="sr-only">Menu</span>
            </Button>
          </SheetTrigger>
          <SheetContent side="left" className="w-64 p-0">
            <SheetHeader className="border-b px-4 py-3">
              <SheetTitle className="flex items-center gap-2">
                <WaveformLogo size={18} />
                <span className="text-primary">Podcaster</span>
              </SheetTitle>
            </SheetHeader>
            <div className="flex flex-col h-[calc(100%-57px)]">
              <div className="flex-1 py-2">
                {navItems.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setOpen(false)}
                    className={`flex items-center min-h-[44px] px-4 text-sm font-medium transition-colors ${
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
                    <div className="mx-4 my-2 h-px bg-border" />
                    {adminItems.map((item) => (
                      <Link
                        key={item.href}
                        href={item.href}
                        onClick={() => setOpen(false)}
                        className={`flex items-center min-h-[44px] px-4 text-sm font-medium transition-colors ${
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
              <div className="border-t px-4 py-3 space-y-2">
                <p className="text-sm text-muted-foreground truncate">
                  {session.user?.name || session.user?.email}
                </p>
                <Button
                  variant="ghost"
                  size="sm"
                  className="w-full justify-start"
                  onClick={() => {
                    setOpen(false);
                    signOut({ callbackUrl: "/" });
                  }}
                >
                  Sign out
                </Button>
              </div>
            </div>
          </SheetContent>
        </Sheet>
      </div>
    </nav>
  );
}
