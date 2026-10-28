"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import { Badge, statusTone } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { formatDate, formatRange, statusLabel } from "@/lib/utils";
import type { EventView } from "@/lib/types";
import { cn } from "@/lib/utils";

export function EventCard({ event }: { event: EventView }) {
  const { user } = useAuth();
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const format =
    event.participation_format === "team"
      ? "Team"
      : event.participation_format === "individual"
        ? "Solo"
        : "Solo or team";

  useEffect(() => {
    if (!user) {
      setSaved(false);
      return;
    }
    let active = true;
    api<{ bookmarked: boolean }>(`/events/${event.id}/bookmark`)
      .then((data) => {
        if (active) setSaved(data.bookmarked);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [event.id, user]);

  const toggleSave = useCallback(async () => {
    if (!user) {
      window.location.href = `/login?next=${encodeURIComponent("/events")}`;
      return;
    }
    setBusy(true);
    try {
      if (saved) {
        await api(`/events/${event.id}/bookmark`, { method: "DELETE" });
        setSaved(false);
        setNotice("Removed from saved events");
      } else {
        await api(`/events/${event.id}/bookmark`, { method: "POST" });
        setSaved(true);
        setNotice("Saved to your events");
      }
    } catch {
      setNotice("Could not update saved events");
    } finally {
      setBusy(false);
      window.setTimeout(() => setNotice(null), 2500);
    }
  }, [event.id, saved, user]);

  async function share() {
    const url = `${window.location.origin}/events/${event.id}`;
    try {
      if (navigator.share) {
        await navigator.share({ title: event.title, url });
      } else {
        await navigator.clipboard.writeText(url);
        setNotice("Event link copied");
        window.setTimeout(() => setNotice(null), 2500);
      }
    } catch {
      // Sharing was dismissed or the clipboard was blocked. Nothing to report.
    }
  }

  return (
    <article className="flex h-full flex-col border-2 border-ink bg-surface shadow-hard transition-transform hover:-translate-y-0.5">
      <div className="flex items-center justify-between gap-2 border-b-2 border-ink bg-accent-3 px-4 py-2">
        <span className="truncate font-pixel text-sm">{event.category.toUpperCase()}</span>
        <div className="flex shrink-0 items-center gap-1.5">
          <button
            type="button"
            onClick={() => void toggleSave()}
            disabled={busy}
            aria-label={saved ? `Remove ${event.title} from saved events` : `Save ${event.title}`}
            aria-pressed={saved}
            title={saved ? "Remove from saved events" : "Save event"}
            className={cn(
              "flex h-10 w-10 items-center justify-center border-2 border-ink transition-transform active:translate-x-[2px] active:translate-y-[2px]",
              saved ? "bg-accent" : "bg-surface hover:bg-accent/30",
            )}
          >
            <BookmarkIcon filled={saved} />
          </button>
          <button
            type="button"
            onClick={() => void share()}
            aria-label={`Share ${event.title}`}
            title="Share event"
            className="flex h-10 w-10 items-center justify-center border-2 border-ink bg-surface transition-transform hover:bg-accent/30 active:translate-x-[2px] active:translate-y-[2px]"
          >
            <ShareIcon />
          </button>
        </div>
      </div>

      <div className="flex flex-1 flex-col p-5">
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
          <Badge tone="quiet">{format}</Badge>
          {event.registration_status === "open" ? <Badge tone="success">Registration open</Badge> : null}
        </div>
        <h3 className="mt-3 font-pixel text-2xl leading-snug">{event.title}</h3>
        <p className="mt-1 font-mono text-sm text-muted">
          {event.organizer_username ? `@${event.organizer_username}` : "Community event"}
          {event.location ? ` · ${event.location}` : ""}
        </p>
        <p className="mt-3 line-clamp-3 text-base text-muted">
          {event.description || "Details coming soon."}
        </p>

        <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 border-t-2 border-ink/20 pt-4 text-sm">
          <div>
            <dt className="font-mono uppercase tracking-wide text-muted">When</dt>
            <dd className="mt-0.5 font-semibold">{formatRange(event.starts_at, event.ends_at)}</dd>
          </div>
          <div>
            <dt className="font-mono uppercase tracking-wide text-muted">Register by</dt>
            <dd className="mt-0.5 font-semibold">{formatDate(event.registration_closes_at)}</dd>
          </div>
          <div>
            <dt className="font-mono uppercase tracking-wide text-muted">Entries</dt>
            <dd className="mt-0.5 font-semibold">
              {event.submission_count} submission{event.submission_count === 1 ? "" : "s"}
            </dd>
          </div>
          <div>
            <dt className="font-mono uppercase tracking-wide text-muted">Registered</dt>
            <dd className="mt-0.5 font-semibold">
              {event.registration_count} {event.registration_count === 1 ? "person" : "people"}
            </dd>
          </div>
        </dl>

        <div className="mt-4 flex-1" />
        {notice ? (
          <p role="status" className="mb-2 border-2 border-ink bg-accent px-2 py-1 font-mono text-xs">
            {notice}
          </p>
        ) : null}
        <Link
          href={`/events/${event.id}`}
          className={`${buttonVariants({ variant: "primary", size: "sm" })} w-full`}
          aria-label={`View ${event.title}`}
        >
          View event
        </Link>
      </div>
    </article>
  );
}

function BookmarkIcon({ filled }: { filled: boolean }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      className="h-5 w-5"
      fill={filled ? "currentColor" : "none"}
      stroke="currentColor"
      strokeWidth="2"
    >
      <path d="M6 3h12v18l-6-4-6 4V3Z" />
    </svg>
  );
}

function ShareIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="18" cy="5" r="3" />
      <circle cx="6" cy="12" r="3" />
      <circle cx="18" cy="19" r="3" />
      <path d="m8.6 10.6 6.8-4.2M8.6 13.4l6.8 4.2" />
    </svg>
  );
}
