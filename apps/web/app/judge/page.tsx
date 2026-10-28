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

interface JudgeEvent extends EventView {
  progress?: { total: number; done: number } | null;
}

export default function JudgeHomePage() {
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [events, setEvents] = useState<JudgeEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api<{ events: EventView[] }>("/judge/events");
      const list = data.events ?? [];
      const withProgress = await Promise.all(
        list.map(async (event) => {
          if (event.judging_status !== "live") return { ...event, progress: null };
          try {
            const progress = await api<{ total: number; done: number }>(
              `/judge/events/${event.id}/progress`,
            );
            return { ...event, progress };
          } catch {
            return { ...event, progress: null };
          }
        }),
      );
      setEvents(withProgress);
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
        <LoadingBlock label="Loading judging queue" />
      </div>
    );
  }

  const ready = events.filter((event) => event.judging_status === "live");
  const waiting = events.filter((event) => event.judging_status !== "live");

  return (
    <div className="mx-auto max-w-content px-4 py-8 sm:px-6 sm:py-12">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="font-pixel text-sm text-muted">JUDGE</p>
          <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">YOUR JUDGING QUEUE</h1>
          <p className="mt-2 max-w-reading text-base text-muted">
            Compare two entries at a time. Pick the stronger one - no scores, no weights, no tie-breaking
            spreadsheets.
          </p>
        </div>
        <Link href="/events" className={buttonVariants({ variant: "secondary" })}>
          Browse events
        </Link>
      </header>

      {error ? (
        <Alert tone="danger" className="mt-5">
          {error}
        </Alert>
      ) : null}

      {events.length === 0 ? (
        <div className="mt-8">
          <EmptyState title="NO JUDGING ASSIGNMENTS">
            When an organizer invites you as a judge, accept the email invite and the event appears here.
          </EmptyState>
        </div>
      ) : (
        <div className="mt-8 space-y-8">
          <section>
            <div className="flex items-end justify-between gap-3">
              <h2 className="font-pixel text-xl">READY TO JUDGE</h2>
              <span className="font-mono text-sm text-muted">{ready.length}</span>
            </div>
            {ready.length === 0 ? (
              <p className="mt-2 border-2 border-dashed border-ink px-4 py-6 text-base text-muted">
                Nothing is live right now. Organizers start judging after comparisons are assigned.
              </p>
            ) : (
              <ul className="mt-3 grid gap-4 sm:grid-cols-2">
                {ready.map((event) => {
                  const total = event.progress?.total ?? 0;
                  const done = event.progress?.done ?? 0;
                  const complete = total > 0 && done >= total;
                  return (
                    <li key={event.id} className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <p className="font-pixel text-lg">{event.title}</p>
                          <p className="font-mono text-sm text-muted">
                            {event.category} · {formatDate(event.starts_at)}
                          </p>
                        </div>
                        <Badge tone={complete ? "success" : "accent"}>
                          {complete ? "DONE" : "LIVE"}
                        </Badge>
                      </div>
                      <p className="mt-3 font-mono text-sm">
                        {done} / {total} comparisons complete
                      </p>
                      <div className="mt-2 flex h-4 gap-1" aria-hidden="true">
                        {Array.from({ length: 16 }).map((_, index) => {
                          const filled = total > 0 && index < Math.round((done / total) * 16);
                          return (
                            <span
                              key={index}
                              className={`h-full flex-1 border-2 border-ink ${
                                filled ? "bg-accent" : "bg-surface"
                              }`}
                            />
                          );
                        })}
                      </div>
                      <Link
                        href={`/judge/events/${event.id}`}
                        className={`${buttonVariants({ size: "sm" })} mt-4`}
                      >
                        {complete ? "Review your calls" : done > 0 ? "Resume judging" : "Start judging"}
                      </Link>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          {waiting.length > 0 ? (
            <section>
              <div className="flex items-end justify-between gap-3">
                <h2 className="font-pixel text-xl">WAITING</h2>
                <span className="font-mono text-sm text-muted">{waiting.length}</span>
              </div>
              <ul className="mt-3 space-y-3">
                {waiting.map((event) => (
                  <li
                    key={event.id}
                    className="flex flex-wrap items-center justify-between gap-3 border-2 border-ink bg-surface p-4"
                  >
                    <div>
                      <p className="font-pixel text-lg">{event.title}</p>
                      <p className="font-mono text-sm text-muted">
                        {formatDate(event.starts_at)} · judging {statusLabel(event.judging_status)}
                      </p>
                    </div>
                    <Badge tone={statusTone(event.judging_status)}>
                      {statusLabel(event.judging_status)}
                    </Badge>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
        </div>
      )}
    </div>
  );
}
