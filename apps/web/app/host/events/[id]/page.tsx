"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { EventView } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { TabOverview } from "@/components/host/tab-overview";
import { TabForm } from "@/components/host/tab-form";
import { TabParticipants } from "@/components/host/tab-participants";
import { TabSubmissions } from "@/components/host/tab-submissions";
import { TabJudges } from "@/components/host/tab-judges";
import { TabJudging } from "@/components/host/tab-judging";
import { cn, statusLabel } from "@/lib/utils";

const tabs = [
  { id: "overview", label: "Overview" },
  { id: "form", label: "Form" },
  { id: "participants", label: "Participants" },
  { id: "submissions", label: "Submissions" },
  { id: "judges", label: "Judges" },
  { id: "judging", label: "Judging" },
] as const;

type TabId = (typeof tabs)[number]["id"];

export default function ManageEventPage() {
  const params = useParams<{ id: string }>();
  const eventId = params.id;
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [event, setEvent] = useState<EventView | null>(null);
  const [tab, setTab] = useState<TabId>("overview");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await api<{ event: EventView }>(`/events/${eventId}/manage`);
      setEvent(data.event);
      setError(null);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId]);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <LoadingBlock label="Loading event" />
      </div>
    );
  }

  if (!event) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12 sm:px-6">
        <Alert tone="danger" title="Event not available">
          {error ?? "You may not have permission to manage this event."}
        </Alert>
        <Link href="/host" className={`${buttonVariants({ variant: "secondary" })} mt-4`}>
          Back to organizer home
        </Link>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-content px-4 py-8 sm:px-6 sm:py-10">
      <Link href="/host" className="chunky-link font-mono text-sm">
        ← Organizer home
      </Link>

      <header className="mt-4 flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
            <Badge tone={statusTone(event.judging_status)}>
              JUDGING {statusLabel(event.judging_status)}
            </Badge>
            <span className="font-mono text-sm text-muted">
              {event.registration_count} registered · {event.submission_count} submitted
            </span>
          </div>
          <h1 className="mt-2 font-pixel text-3xl leading-tight sm:text-4xl">{event.title}</h1>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "secondary", size: "sm" })}>
            Public page
          </Link>
          <Link
            href={`/events/${event.id}/submit`}
            className={buttonVariants({ variant: "ghost", size: "sm" })}
          >
            Submission form
          </Link>
        </div>
      </header>

      <nav aria-label="Event sections" className="mt-6 overflow-x-auto border-b-2 border-ink">
        <ul className="flex min-w-max gap-1">
          {tabs.map((item) => {
            const active = tab === item.id;
            return (
              <li key={item.id}>
                <button
                  type="button"
                  onClick={() => setTab(item.id)}
                  aria-current={active ? "page" : undefined}
                  className={cn(
                    "min-h-12 border-2 border-b-0 border-ink px-4 py-2 font-pixel text-sm",
                    active ? "bg-accent" : "bg-surface hover:bg-accent/20",
                  )}
                >
                  {item.label}
                </button>
              </li>
            );
          })}
        </ul>
      </nav>

      <div className="mt-6">
        {tab === "overview" ? <TabOverview event={event} onReload={load} /> : null}
        {tab === "form" ? <TabForm event={event} /> : null}
        {tab === "participants" ? <TabParticipants eventId={event.id} /> : null}
        {tab === "submissions" ? <TabSubmissions eventId={event.id} /> : null}
        {tab === "judges" ? <TabJudges eventId={event.id} /> : null}
        {tab === "judging" ? <TabJudging event={event} onReload={load} /> : null}
      </div>
    </div>
  );
}
