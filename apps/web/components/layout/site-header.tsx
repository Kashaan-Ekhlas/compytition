"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { NotificationsMenu } from "@/components/layout/notifications-menu";
import { Button, buttonVariants } from "@/components/ui/button";
import { cn, initials } from "@/lib/utils";

const nav = [
  { href: "/events", label: "Events", authOnly: false },
  { href: "/dashboard", label: "Dashboard", authOnly: true },
  { href: "/host", label: "Host", authOnly: true },
  { href: "/judge", label: "Judge", authOnly: true },
];

export function SiteHeader() {
  const { user, profile, loading, logout } = useAuth();
  const pathname = usePathname();
  const [menuOpen, setMenuOpen] = useState(false);
  const visibleNav = nav.filter((item) => !item.authOnly || user);

  return (
    <header className="sticky top-0 z-40 border-b-2 border-ink bg-bg/95 backdrop-blur-[2px]">
      <div className="mx-auto flex min-h-[72px] max-w-content items-center gap-4 px-4 py-2 sm:px-6">
        <Link href="/" className="flex items-center gap-2" aria-label="Compytition home">
          <span className="flex h-9 w-9 items-center justify-center border-2 border-ink bg-accent font-pixel text-lg shadow-hard-sm">
            C
          </span>
          <span className="font-pixel text-base tracking-tight sm:text-lg">COMPYTITION</span>
        </Link>

        <nav aria-label="Main" className="ml-4 hidden translate-y-[2px] items-center gap-1 md:flex">
          {visibleNav.map((item) => {
            const active = pathname === item.href || pathname.startsWith(`${item.href}/`);
            return (
              <Link
                key={item.href}
                href={item.href}
                className={cn(
                  "border-b-4 px-3 py-2 font-pixel text-sm",
                  active ? "border-accent bg-accent/30" : "border-transparent hover:bg-accent/15",
                )}
                aria-current={active ? "page" : undefined}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          {loading ? (
            <span className="hidden font-mono text-sm text-muted sm:inline">…</span>
          ) : user ? (
            <>
              <div className="hidden sm:block">
                <NotificationsMenu />
              </div>
              <Link
                href="/settings"
                className="hidden items-center gap-2 border-2 border-ink bg-surface px-2 py-1 shadow-hard-sm sm:flex"
                title={user.email}
              >
                <span className="flex h-8 w-8 items-center justify-center border-2 border-ink bg-accent-3 font-pixel text-xs">
                  {initials(profile?.full_name || user.username)}
                </span>
                <span className="max-w-28 truncate font-mono text-sm">{user.username}</span>
              </Link>
              <Button variant="ghost" size="sm" className="hidden lg:inline-flex" onClick={() => void logout()}>
                Log out
              </Button>
            </>
          ) : (
            <div className="hidden items-center gap-2 sm:flex">
              <Link href="/login" className={buttonVariants({ variant: "secondary", size: "sm" })}>
                Log in
              </Link>
              <Link href="/register" className={buttonVariants({ variant: "primary", size: "sm" })}>
                Join
              </Link>
            </div>
          )}
          <button
            type="button"
            className="flex h-12 w-12 items-center justify-center border-2 border-ink bg-accent shadow-hard-sm md:hidden"
            onClick={() => setMenuOpen((value) => !value)}
            aria-expanded={menuOpen}
            aria-controls="mobile-nav"
            aria-label="Menu"
          >
            <span aria-hidden="true" className="font-pixel text-lg">
              {menuOpen ? "×" : "≡"}
            </span>
          </button>
        </div>
      </div>

      {menuOpen ? (
        <nav id="mobile-nav" aria-label="Mobile" className="border-t-2 border-ink bg-surface px-4 py-3 md:hidden">
          <ul className="space-y-1">
            {visibleNav.map((item) => (
              <li key={item.href}>
                <Link
                  href={item.href}
                  onClick={() => setMenuOpen(false)}
                  className="block min-h-12 border-b-2 border-ink/20 px-2 py-2 font-pixel text-base"
                >
                  {item.label}
                </Link>
              </li>
            ))}
            {user ? (
              <>
                <li>
                  <Link
                    href="/settings"
                    onClick={() => setMenuOpen(false)}
                    className="block min-h-12 border-b-2 border-ink/20 px-2 py-2 font-pixel text-base"
                  >
                    Settings
                  </Link>
                </li>
                <li className="pt-2">
                  <Button variant="secondary" className="w-full" onClick={() => void logout()}>
                    Log out
                  </Button>
                </li>
              </>
            ) : (
              <li className="flex gap-2 pt-2">
                <Link href="/login" className={cn(buttonVariants({ variant: "secondary" }), "flex-1")}>
                  Log in
                </Link>
                <Link href="/register" className={cn(buttonVariants({ variant: "primary" }), "flex-1")}>
                  Join
                </Link>
              </li>
            )}
          </ul>
        </nav>
      ) : null}
    </header>
  );
}
