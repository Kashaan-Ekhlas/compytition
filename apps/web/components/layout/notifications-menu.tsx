"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import type { Notification } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export function NotificationsMenu({ initialUnread = 0 }: { initialUnread?: number }) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<Notification[]>([]);
  const [unread, setUnread] = useState(initialUnread);
  const [loading, setLoading] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api<{ notifications: Notification[]; unread: number }>("/notifications", {
        query: { limit: 20 },
      });
      setItems(data.notifications);
      setUnread(data.unread);
    } finally {
      setLoading(false);
    }
  }, []);

  // Keep the badge fresh without requiring the menu to be opened. The short
  // interval makes new invites and results feel immediate; a focus refresh
  // catches anything that arrived while the tab was in the background.
  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 5000);
    function refresh() {
      if (document.visibilityState === "visible") void load();
    }
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [load]);

  useEffect(() => {
    if (!open) return;
    void load();
    function onClick(event: MouseEvent) {
      if (panelRef.current && !panelRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [open, load]);

  async function markAll() {
    await api("/notifications/read-all", { method: "POST" }).catch(() => undefined);
    setItems((current) => current.map((item) => ({ ...item, read_at: item.read_at ?? new Date().toISOString() })));
    setUnread(0);
  }

  async function markOne(id: string) {
    await api(`/notifications/${id}/read`, { method: "POST" }).catch(() => undefined);
    setItems((current) =>
      current.map((item) => (item.id === id ? { ...item, read_at: item.read_at ?? new Date().toISOString() } : item)),
    );
    setUnread((current) => Math.max(0, current - 1));
  }

  return (
    <div className="relative" ref={panelRef}>
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="relative flex h-12 w-12 items-center justify-center border-2 border-ink bg-surface text-xl shadow-hard-sm transition-transform active:translate-x-[2px] active:translate-y-[2px] active:shadow-none"
        aria-label={`Notifications${unread ? `, ${unread} unread` : ""}`}
        aria-expanded={open}
      >
        <span aria-hidden="true">
          <svg viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M18 8a6 6 0 1 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" />
            <path d="M10 21h4" />
          </svg>
        </span>
        {unread > 0 ? (
          <span className="absolute -right-2 -top-2 flex h-6 min-w-6 items-center justify-center border-2 border-ink bg-accent-2 px-1 font-mono text-xs font-bold text-white">
            {unread > 9 ? "9+" : unread}
          </span>
        ) : null}
      </button>
      {open ? (
        <div className="absolute right-0 z-50 mt-2 w-80 border-2 border-ink bg-surface shadow-hard">
          <div className="flex items-center justify-between border-b-2 border-ink bg-accent-3 px-3 py-2">
            <span className="font-pixel text-sm">Notifications</span>
            <Button variant="link" size="sm" onClick={markAll} className="text-sm">
              Mark all read
            </Button>
          </div>
          <div className="max-h-96 overflow-y-auto">
            {loading && items.length === 0 ? (
              <p className="p-4 text-base text-muted">Loading…</p>
            ) : null}
            {!loading && items.length === 0 ? (
              <p className="p-4 text-base text-muted">Nothing here yet.</p>
            ) : null}
            {items.map((item) => {
              const body = (
                <div className={`border-b-2 border-ink/20 p-3 ${item.read_at ? "" : "bg-accent/20"}`}>
                  <div className="flex items-start justify-between gap-2">
                    <p className="font-semibold">{item.title}</p>
                    {!item.read_at ? <Badge tone="red">New</Badge> : null}
                  </div>
                  <p className="mt-1 text-sm text-muted">{item.body}</p>
                </div>
              );
              return item.link ? (
                <Link
                  key={item.id}
                  href={item.link}
                  onClick={() => {
                    void markOne(item.id);
                    setOpen(false);
                  }}
                  className="block hover:bg-accent/10"
                >
                  {body}
                </Link>
              ) : (
                <button key={item.id} type="button" className="block w-full text-left" onClick={() => void markOne(item.id)}>
                  {body}
                </button>
              );
            })}
          </div>
        </div>
      ) : null}
    </div>
  );
}
