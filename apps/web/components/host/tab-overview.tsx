"use client";

import { useState } from "react";
import Link from "next/link";
import { api, errorMessage } from "@/lib/api";
import type { EventView } from "@/lib/types";
import { Alert } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { EventForm } from "@/components/host/event-form";
import { formatDate, formatRange, statusLabel } from "@/lib/utils";

export function TabOverview({ event, onReload }: { event: EventView; onReload: () => Promise<void> }) {
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  function flash(message: string) {
    setNotice(message);
    setError(null);
  }

  async function lifecycle(action: "publish" | "unpublish" | "close" | "delete") {
    if (action === "delete" && !window.confirm("Delete this draft permanently? This cannot be undone.")) {
      return;
    }
    if (action === "unpublish" && !window.confirm("Return this event to draft? It will disappear from discovery.")) {
      return;
    }
    setBusy(action);
    setError(null);
    setNotice(null);
    try {
      if (action === "delete") {
        await api(`/events/${event.id}`, { method: "DELETE" });
        window.location.assign("/host");
        return;
      }
      await api(`/events/${event.id}/${action}`, { method: "POST" });
      await onReload();
      flash(
        action === "publish"
          ? "Event published."
          : action === "unpublish"
            ? "Event returned to draft."
            : "Event closed.",
      );
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-6">
      {error ? <Alert tone="danger">{error}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      <WindowCard title="STATUS & LIFECYCLE">
        <div className="space-y-5 p-5">
          <div className="flex flex-wrap items-center gap-3">
            <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
            <Badge tone={statusTone(event.judging_status)}>
              JUDGING {statusLabel(event.judging_status)}
            </Badge>
            <span className="font-mono text-sm text-muted">
              {event.registration_count} registered · {event.submission_count} submitted
            </span>
          </div>

          {event.status === "draft" ? (
            <Alert tone="info">
              Drafts are private. Publishing requires a start date, a registration deadline and a
              submission deadline.
            </Alert>
          ) : null}

          <dl className="grid gap-3 sm:grid-cols-2">
            <Meta label="When" value={formatRange(event.starts_at, event.ends_at)} />
            <Meta label="Registration closes" value={formatDate(event.registration_closes_at, true)} />
            <Meta label="Submission deadline" value={formatDate(event.submission_deadline, true)} />
            <Meta
              label="Format"
              value={
                event.participation_format === "both"
                  ? `Solo or teams up to ${event.max_team_size}`
                  : event.participation_format === "team"
                    ? `Teams up to ${event.max_team_size}`
                    : "Individual"
              }
            />
          </dl>

          <div className="flex flex-wrap gap-2 border-t-2 border-ink/20 pt-4">
            {event.status === "draft" ? (
              <Button onClick={() => void lifecycle("publish")} disabled={busy !== null}>
                {busy === "publish" ? "Publishing…" : "Publish event"}
              </Button>
            ) : null}
            {event.status === "published" ? (
              <>
                <Button variant="secondary" onClick={() => void lifecycle("close")} disabled={busy !== null}>
                  {busy === "close" ? "Closing…" : "Close registrations"}
                </Button>
                <Button
                  variant="ghost"
                  onClick={() => void lifecycle("unpublish")}
                  disabled={busy !== null || event.registration_count > 0}
                  title={
                    event.registration_count > 0
                      ? "Participants have registered, so this cannot return to draft."
                      : undefined
                  }
                >
                  Return to draft
                </Button>
              </>
            ) : null}
            {event.status === "results_published" ? (
              <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "secondary" })}>
                View public results
              </Link>
            ) : null}
            {event.status === "draft" ? (
              <Button variant="destructive" onClick={() => void lifecycle("delete")} disabled={busy !== null}>
                {busy === "delete" ? "Deleting…" : "Delete draft"}
              </Button>
            ) : null}
          </div>
        </div>
      </WindowCard>

      <WindowCard title="EVENT DETAILS">
        <div className="p-5">
          {editing ? (
            <EventForm
              initial={event}
              onSaved={() => {
                setEditing(false);
                void onReload().then(() => flash("Event details saved."));
              }}
            />
          ) : (
            <div className="space-y-4">
              <p className="whitespace-pre-wrap text-base">{event.description || "No description yet."}</p>
              <dl className="grid gap-3 sm:grid-cols-2">
                <Meta label="Category" value={event.category} />
                <Meta label="Location" value={event.location || "-"} />
                <Meta label="Contact" value={event.contact_email || "-"} />
                <Meta label="Host" value={`@${event.organizer_username || "you"}`} />
              </dl>
              {event.eligibility ? (
                <div>
                  <p className="font-mono text-sm uppercase tracking-wide text-muted">Eligibility</p>
                  <p className="mt-1 whitespace-pre-wrap text-base">{event.eligibility}</p>
                </div>
              ) : null}
              {event.rules ? (
                <div>
                  <p className="font-mono text-sm uppercase tracking-wide text-muted">Rules</p>
                  <p className="mt-1 whitespace-pre-wrap text-base">{event.rules}</p>
                </div>
              ) : null}
              <div className="flex flex-wrap gap-2 border-t-2 border-ink/20 pt-4">
                <Button variant="secondary" onClick={() => setEditing(true)}>
                  Edit event
                </Button>
                <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "ghost" })}>
                  Preview public page
                </Link>
              </div>
            </div>
          )}
        </div>
      </WindowCard>
    </div>
  );
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="font-mono text-xs uppercase tracking-wide text-muted">{label}</dt>
      <dd className="mt-0.5 text-base">{value}</dd>
    </div>
  );
}
