"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import type { EventView, FormField, Registration, ResultEntry, Submission, Team } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { formatDate, formatRange, relativeTime, statusLabel } from "@/lib/utils";

export default function EventDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const { user } = useAuth();
  const [event, setEvent] = useState<EventView | null>(null);
  const [fields, setFields] = useState<FormField[]>([]);
  const [registration, setRegistration] = useState<Registration | null>(null);
  const [submission, setSubmission] = useState<Submission | null>(null);
  const [team, setTeam] = useState<Team | null>(null);
  const [results, setResults] = useState<ResultEntry[] | null>(null);
  const [bookmarked, setBookmarked] = useState(false);
  const [bookmarkBusy, setBookmarkBusy] = useState(false);
  const [bookmarkNotice, setBookmarkNotice] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const eventData = await api<{ event: EventView }>(`/events/${id}`);
      setEvent(eventData.event);
      const formData = await api<{ fields: FormField[] }>(`/events/${id}/form`).catch(() => ({ fields: [] }));
      setFields(formData.fields ?? []);
      const resultsData = await api<{ results: ResultEntry[] }>(`/events/${id}/results`).catch(() => null);
      setResults(resultsData?.results ?? null);
      if (user) {
        const [reg, sub, myTeam] = await Promise.all([
          api<{ registration: Registration | null }>(`/events/${id}/registrations/me`).catch(() => ({ registration: null })),
          api<{ submission: Submission | null }>(`/events/${id}/submissions/me`).catch(() => ({ submission: null })),
          api<{ team: Team | null }>(`/events/${id}/teams/mine`).catch(() => ({ team: null })),
        ]);
        setRegistration(reg.registration);
        setSubmission(sub.submission);
        setTeam(myTeam.team);
        const saved = await api<{ bookmarks: Array<{ event_id: string }> }>("/bookmarks").catch(
          () => ({ bookmarks: [] }),
        );
        setBookmarked(saved.bookmarks.some((item) => item.event_id === id));
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [id, user]);

  useEffect(() => {
    void load();
  }, [load]);

  async function toggleBookmark() {
    setBookmarkBusy(true);
    setBookmarkNotice(null);
    try {
      if (bookmarked) {
        await api(`/events/${id}/bookmark`, { method: "DELETE" });
        setBookmarked(false);
        setBookmarkNotice("Removed from bookmarks.");
      } else {
        await api(`/events/${id}/bookmark`, { method: "POST" });
        setBookmarked(true);
        setBookmarkNotice("Saved to bookmarks.");
      }
    } catch (err) {
      setBookmarkNotice(errorMessage(err));
    } finally {
      setBookmarkBusy(false);
    }
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <LoadingBlock label="Loading event" />
      </div>
    );
  }

  if (error || !event) {
    return (
      <div className="mx-auto max-w-reading px-4 py-16 sm:px-6">
        <Alert tone="danger" title="Event not available">
          {error ?? "This event may have been removed."}
        </Alert>
        <Link href="/events" className={`${buttonVariants({ variant: "secondary" })} mt-4`}>
          Back to events
        </Link>
      </div>
    );
  }

  const now = Date.now();
  const closed = event.registration_closes_at ? new Date(event.registration_closes_at).getTime() < now : false;
  const notOpen = event.registration_opens_at ? new Date(event.registration_opens_at).getTime() > now : false;
  const registrationOpen = event.status === "published" && !closed && !notOpen;
  const canRegister = registrationOpen && (!registration || registration.status !== "registered");
  const deadlinePassed = event.submission_deadline ? new Date(event.submission_deadline).getTime() < now : false;
  const canSubmit = Boolean(registration && registration.status === "registered") && !deadlinePassed && event.status === "published";
  const isTeamEvent = event.participation_format !== "individual";

  return (
    <div className="mx-auto max-w-content px-4 py-8 sm:px-6 sm:py-12">
      <div className="flex flex-wrap items-center gap-2">
        <Link href="/events" className="chunky-link font-mono text-sm">
          ← All events
        </Link>
        <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
        <Badge tone="quiet">{event.category}</Badge>
        {event.is_judge ? <Badge tone="blue">Judging</Badge> : null}
        {event.can_manage ? <Badge tone="red">Organizer</Badge> : null}
      </div>

      <header className="mt-5 grid gap-6 lg:grid-cols-[1.35fr_0.65fr]">
        <div>
          <h1 className="font-pixel text-3xl leading-tight sm:text-4xl lg:text-5xl">{event.title}</h1>
          <p className="mt-3 max-w-reading text-lg text-muted">{event.description || "Details are being finalised."}</p>
          <div className="mt-6 flex flex-wrap gap-3">
            {canRegister ? (
              <Link href={`/events/${id}/register`} className={buttonVariants({ size: "lg" })}>
                Register
              </Link>
            ) : registration && registration.status === "registered" ? (
              <Link href={`/events/${id}/register`} className={buttonVariants({ variant: "secondary" })}>
                Edit registration
              </Link>
            ) : null}
            {canSubmit ? (
              <Link href={`/events/${id}/submit`} className={buttonVariants({ variant: "secondary" })}>
                {submission ? "Edit submission" : "Submit work"}
              </Link>
            ) : null}
            {isTeamEvent && registrationOpen ? (
              <Link href={`/events/${id}/team`} className={buttonVariants({ variant: "secondary" })}>
                {team ? team.name : "Find a team"}
              </Link>
            ) : null}
            {event.can_manage ? (
              <Link href={`/host/events/${id}`} className={buttonVariants({ variant: "secondary" })}>
                Manage
              </Link>
            ) : null}
            {event.is_judge ? (
              <Link href={`/judge/events/${id}`} className={buttonVariants({ variant: "secondary" })}>
                Judge event
              </Link>
            ) : null}
            {user ? (
              <Button
                variant="ghost"
                onClick={() => void toggleBookmark()}
                disabled={bookmarkBusy}
                aria-pressed={bookmarked}
              >
                {bookmarkBusy ? "Saving…" : bookmarked ? "★ Saved" : "☆ Save"}
              </Button>
            ) : null}
          </div>
          {bookmarkNotice ? (
            <p className="mt-3 font-mono text-sm text-muted" role="status">
              {bookmarkNotice}
            </p>
          ) : null}
          {registration && registration.status !== "registered" ? (
            <Alert tone="warning" className="mt-4">
              Your registration is marked <strong>{registration.status}</strong>.
            </Alert>
          ) : null}
          {event.status === "results_published" ? (
            <p className="mt-4 font-mono text-sm text-muted">
              Results published {relativeTime(event.results_published_at)}.
            </p>
          ) : null}
        </div>

        <aside className="border-2 border-ink bg-surface shadow-hard">
          <div className="border-b-2 border-ink bg-accent px-4 py-2 font-pixel text-sm">EVENT FILE</div>
          <dl className="divide-y-2 divide-ink/20">
            <Meta label="When" value={formatRange(event.starts_at, event.ends_at)} />
            <Meta label="Registration" value={`${formatDate(event.registration_opens_at)} → ${formatDate(event.registration_closes_at)}`} />
            <Meta label="Submission deadline" value={formatDate(event.submission_deadline, true)} />
            <Meta label="Format" value={
              event.participation_format === "both"
                ? `Solo or teams of ${event.min_team_size}–${event.max_team_size}`
                : event.participation_format === "team"
                  ? `Teams of ${event.min_team_size}–${event.max_team_size}`
                  : "Individual"
            } />
            <Meta label="Location" value={event.location || "Online"} />
            <Meta label="Host" value={event.organizer_username ? `@${event.organizer_username}` : "Community"} />
            <Meta label="Entries" value={`${event.registration_count} registered · ${event.submission_count} submitted`} />
          </dl>
        </aside>
      </header>

      <div className="mt-10 grid gap-6 lg:grid-cols-2">
        <WindowCard title="ABOUT">
          <div className="space-y-4 p-5">
            <p className="whitespace-pre-wrap text-base">{event.description || "No description yet."}</p>
            {event.eligibility ? (
              <div>
                <h3 className="font-pixel text-base">ELIGIBILITY</h3>
                <p className="mt-1 whitespace-pre-wrap text-base text-muted">{event.eligibility}</p>
              </div>
            ) : null}
          </div>
        </WindowCard>

        <WindowCard title="RULES">
          <div className="p-5">
            <p className="whitespace-pre-wrap text-base text-muted">{event.rules || "The organizer has not posted extra rules."}</p>
            {event.contact_email ? (
              <p className="mt-4 border-t-2 border-ink/20 pt-3 font-mono text-sm">
                Questions: <a className="chunky-link" href={`mailto:${event.contact_email}`}>{event.contact_email}</a>
              </p>
            ) : null}
          </div>
        </WindowCard>

        <WindowCard title="JUDGING CRITERIA">
          <div className="p-5">
            {event.criteria.length === 0 ? (
              <p className="text-base text-muted">Criteria are announced closer to judging.</p>
            ) : (
              <ol className="space-y-3">
                {event.criteria.map((criterion, index) => (
                  <li key={criterion.name} className="flex gap-3">
                    <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center border-2 border-ink bg-accent-3 font-pixel text-sm">
                      {index + 1}
                    </span>
                    <span>
                      <span className="block font-semibold">{criterion.name}</span>
                      <span className="block text-base text-muted">{criterion.description}</span>
                    </span>
                  </li>
                ))}
              </ol>
            )}
            <p className="mt-4 border-t-2 border-ink/20 pt-3 text-sm text-muted">
              Judges compare two entries at a time. There are no numeric scores or weights.
            </p>
          </div>
        </WindowCard>

        <WindowCard title="REGISTRATION FORM">
          <div className="p-5">
            {fields.length === 0 ? (
              <p className="text-base text-muted">This event has no extra questions — your profile is enough.</p>
            ) : (
              <ul className="space-y-2">
                {fields.map((field) => (
                  <li key={field.id} className="flex items-start justify-between gap-3 border-b-2 border-ink/15 pb-2">
                    <span>
                      <span className="block font-medium">{field.label}</span>
                      {field.help_text ? <span className="block text-sm text-muted">{field.help_text}</span> : null}
                    </span>
                    <Badge tone="quiet">{field.required ? "Required" : "Optional"}</Badge>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </WindowCard>
      </div>

      {results ? (
        <section className="mt-10">
          <div className="flex flex-wrap items-end justify-between gap-3">
            <div>
              <h2 className="font-pixel text-2xl">RESULTS</h2>
              <p className="mt-1 text-base text-muted">
                Aggregated from pairwise comparisons. Confidence is the model&apos;s edge over the next entry.
              </p>
            </div>
            <Badge tone="ink">PUBLISHED</Badge>
          </div>
          <div className="mt-4 overflow-x-auto border-2 border-ink bg-surface shadow-hard">
            <table className="w-full min-w-[640px] text-left">
              <thead className="border-b-2 border-ink bg-accent-3 font-pixel text-sm">
                <tr>
                  <th className="px-3 py-2">Rank</th>
                  <th className="px-3 py-2">Entry</th>
                  <th className="px-3 py-2">Team</th>
                  <th className="px-3 py-2">Score</th>
                  <th className="px-3 py-2">Confidence</th>
                </tr>
              </thead>
              <tbody>
                {results.map((row) => (
                  <tr key={row.submission_id} className="border-b-2 border-ink/15 last:border-0">
                    <td className="px-3 py-3 font-mono text-lg">#{row.rank}</td>
                    <td className="px-3 py-3">
                      <span className="block font-semibold">{row.title}</span>
                      <span className="block font-mono text-sm text-muted">@{row.creator_username}</span>
                    </td>
                    <td className="px-3 py-3 text-base">{row.team_name || "—"}</td>
                    <td className="px-3 py-3 font-mono">{row.score.toFixed(3)}</td>
                    <td className="px-3 py-3 font-mono">{Math.round(row.confidence * 100)}%</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      ) : null}
    </div>
  );
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="px-4 py-2.5">
      <dt className="font-mono text-xs uppercase tracking-wide text-muted">{label}</dt>
      <dd className="mt-0.5 text-base">{value}</dd>
    </div>
  );
}
