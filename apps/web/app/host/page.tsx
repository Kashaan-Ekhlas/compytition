"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { EventView } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/illustrations";
import { formatDate, statusLabel } from "@/lib/utils";

export default function HostHomePage() {
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [events, setEvents] = useState<EventView[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api<{ events: EventView[] }>("/host/events");
      setEvents(data.events ?? []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <LoadingBlock label="Loading your events" />
      </div>
    );
  }

  const drafts = events.filter((event) => event.status === "draft");
  const active = events.filter((event) => event.status === "published" || event.status === "closed");
  const done = events.filter((event) => event.status === "results_published");

  return (
    <div className="mx-auto max-w-content px-4 py-8 sm:px-6 sm:py-12">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="font-pixel text-sm text-muted">ORGANIZER</p>
          <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">HOST AN EVENT</h1>
          <p className="mt-2 max-w-reading text-base text-muted">
            Set the dates, design the form, invite judges, then let pairwise comparisons do the ranking.
          </p>
        </div>
        <Link href="/host/events/new" className={buttonVariants({ size: "lg" })}>
          New event
        </Link>
      </header>

      {error ? (
        <Alert tone="danger" className="mt-5">
          {error}
        </Alert>
      ) : null}

      {events.length === 0 ? (
        <div className="mt-8">
          <EmptyState
            title="NO EVENTS YET"
            action={
              <Link href="/host/events/new" className={buttonVariants({ variant: "secondary" })}>
                Create your first event
              </Link>
            }
          >
            Drafts stay private. Publishing reveals the event in discovery and opens registration.
          </EmptyState>
        </div>
      ) : (
        <div className="mt-8 space-y-10">
          <HostSection title="DRAFTS" events={drafts} empty="No drafts in progress." />
          <HostSection title="RUNNING" events={active} empty="Nothing published right now." />
          <HostSection title="FINISHED" events={done} empty="No results published yet." />
        </div>
      )}
    </div>
  );
}

function HostSection({
  title,
  events,
  empty,
}: {
  title: string;
  events: EventView[];
  empty: string;
}) {
  return (
    <section>
      <div className="flex items-end justify-between gap-3">
        <h2 className="font-pixel text-xl">{title}</h2>
        <span className="font-mono text-sm text-muted">{events.length}</span>
      </div>
      {events.length === 0 ? (
        <p className="mt-2 border-2 border-dashed border-ink px-4 py-6 text-base text-muted">{empty}</p>
      ) : (
        <ul className="mt-3 space-y-3">
          {events.map((event) => (
            <li
              key={event.id}
              className="flex flex-wrap items-center justify-between gap-3 border-2 border-ink bg-surface p-4 shadow-hard-sm"
            >
              <div className="min-w-0">
                <Link href={`/host/events/${event.id}`} className="chunky-link font-pixel text-lg">
                  {event.title}
                </Link>
                <p className="mt-1 font-mono text-sm text-muted">
                  {formatDate(event.starts_at)} · {event.registration_count} registered ·{" "}
                  {event.submission_count} submitted · judging {event.judging_status}
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
                <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "ghost", size: "sm" })}>
                  Public page
                </Link>
                <Link
                  href={`/host/events/${event.id}`}
                  className={buttonVariants({ variant: "secondary", size: "sm" })}
                >
                  Manage
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
