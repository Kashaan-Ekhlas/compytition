"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { DashboardData, TeamInvite } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { EmptyState } from "@/components/illustrations";
import { formatDate, relativeTime, statusLabel } from "@/lib/utils";

export default function DashboardPage() {
  const { user, profile } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await api<DashboardData>("/dashboard");
      setData(result);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  async function respond(invite: TeamInvite, action: "accept" | "reject") {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await api(`/team-invites/${invite.id}/${action}`, { method: "POST" });
      setNotice(action === "accept" ? "You joined the team." : "Invite declined.");
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <LoadingBlock label="Loading your dashboard" />
      </div>
    );
  }

  if (!data) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12 sm:px-6">
        <Alert tone="danger" title="Dashboard unavailable">
          {error ?? "Try refreshing the page."}
        </Alert>
        <Button variant="secondary" className="mt-4" onClick={() => void load()}>
          Retry
        </Button>
      </div>
    );
  }

  const name = profile?.full_name?.split(" ")[0] || user?.username;
  const openRegistrations = data.registrations.filter((r) => r.status === "registered");
  const drafts = data.submissions.filter((s) => s.status === "draft");

  return (
    <div className="mx-auto max-w-content px-4 py-8 sm:px-6 sm:py-12">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="font-pixel text-sm text-muted">YOUR DESK</p>
          <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">HELLO, {name?.toUpperCase()}</h1>
          <p className="mt-2 max-w-reading text-base text-muted">
            One place for the events you are entering, hosting and judging.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link href="/events" className={buttonVariants({ variant: "secondary" })}>
            Find an event
          </Link>
          <Link href="/host/events/new" className={buttonVariants()}>
            Host an event
          </Link>
        </div>
      </header>

      {error ? (
        <Alert tone="danger" className="mt-5">
          {error}
        </Alert>
      ) : null}
      {notice ? (
        <Alert tone="success" className="mt-5">
          {notice}
        </Alert>
      ) : null}

      <section className="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Stat label="Registered" value={openRegistrations.length} href="#my-events" />
        <Stat label="Teams" value={data.teams.length} href="#my-teams" />
        <Stat label="Hosting" value={data.hosted_events.length} href="#hosting" />
        <Stat label="Judging" value={data.judge_events.length} href="#judging" />
      </section>

      {data.team_invites.length > 0 ? (
        <WindowCard title="TEAM INVITES" className="mt-8">
          <ul className="divide-y-2 divide-ink/15">
            {data.team_invites.map((invite) => (
              <li key={invite.id} className="flex flex-wrap items-center justify-between gap-3 p-4">
                <div>
                  <p className="font-semibold">{invite.team_name}</p>
                  <p className="font-mono text-sm text-muted">
                    {invite.event_title ? `${invite.event_title} · ` : ""}
                    from @{invite.invited_by_username || invite.invited_by.slice(0, 8)}
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button size="sm" disabled={busy} onClick={() => void respond(invite, "accept")}>
                    Accept
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={busy}
                    onClick={() => void respond(invite, "reject")}
                  >
                    Decline
                  </Button>
                </div>
              </li>
            ))}
          </ul>
        </WindowCard>
      ) : null}

      {drafts.length > 0 ? (
        <Alert tone="warning" className="mt-6" title="Unsubmitted drafts">
          <ul className="space-y-1">
            {drafts.map((draft) => (
              <li key={draft.id}>
                <Link href={`/events/${draft.event_id}/submit`} className="chunky-link font-semibold">
                  {draft.title || "Untitled entry"}
                </Link>{" "}
                <span className="text-muted">
                  for {draft.event_title || "an event"} · saved {relativeTime(draft.updated_at)}
                </span>
              </li>
            ))}
          </ul>
        </Alert>
      ) : null}

      <div id="my-events" className="mt-10 scroll-mt-24">
        <SectionHeading title="MY EVENTS" subtitle="Registrations, not bookmarks.">
          <Link href="/events" className="chunky-link font-mono text-sm">
            Browse more →
          </Link>
        </SectionHeading>
        {data.registrations.length === 0 ? (
          <EmptyState
            title="NO REGISTRATIONS YET"
            action={
              <Link href="/events" className={buttonVariants({ variant: "secondary" })}>
                Find your first event
              </Link>
            }
          >
            Once you register, the event and your submission status will live here.
          </EmptyState>
        ) : (
          <div className="mt-4 overflow-x-auto border-2 border-ink bg-surface shadow-hard">
            <table className="w-full min-w-[720px] text-left">
              <thead className="border-b-2 border-ink bg-accent-3 font-pixel text-sm">
                <tr>
                  <th className="px-3 py-2">Event</th>
                  <th className="px-3 py-2">Starts</th>
                  <th className="px-3 py-2">Team</th>
                  <th className="px-3 py-2">Status</th>
                  <th className="px-3 py-2">Entry</th>
                </tr>
              </thead>
              <tbody>
                {data.registrations.map((registration) => (
                  <tr key={registration.id} className="border-b-2 border-ink/15 last:border-0">
                    <td className="px-3 py-3">
                      <Link
                        href={`/events/${registration.event_id}`}
                        className="chunky-link font-semibold"
                      >
                        {registration.event_title || "Event"}
                      </Link>
                    </td>
                    <td className="px-3 py-3 font-mono text-sm">
                      {formatDate(registration.starts_at)}
                    </td>
                    <td className="px-3 py-3 text-base">{registration.team_name || "Solo"}</td>
                    <td className="px-3 py-3">
                      <Badge tone={statusTone(registration.status)}>
                        {statusLabel(registration.status)}
                      </Badge>
                      {registration.event_status ? (
                        <span className="mt-1 block font-mono text-xs text-muted">
                          event {statusLabel(registration.event_status)}
                        </span>
                      ) : null}
                    </td>
                    <td className="px-3 py-3">
                      {registration.has_submission ? (
                        <Badge tone="success">Submitted</Badge>
                      ) : (
                        <Link
                          href={`/events/${registration.event_id}/submit`}
                          className="chunky-link font-mono text-sm"
                        >
                          Add entry →
                        </Link>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div id="my-teams" className="mt-10 scroll-mt-24">
        <SectionHeading title="MY TEAMS" subtitle="Shared registrations and team chat." />
        {data.teams.length === 0 ? (
          <EmptyState title="NO TEAMS YET">
            Team events let you register everyone at once. Create or join one from an event page.
          </EmptyState>
        ) : (
          <ul className="mt-4 grid gap-4 sm:grid-cols-2">
            {data.teams.map((team) => (
              <li key={team.id} className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <p className="font-pixel text-lg">{team.name}</p>
                    <p className="font-mono text-sm text-muted">
                      {team.event_title || "Event"} · {team.member_count}
                      {team.max_team_size ? `/${team.max_team_size}` : ""} members
                    </p>
                  </div>
                  {team.is_owner ? <Badge tone="accent">Owner</Badge> : null}
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  <Link
                    href={`/events/${team.event_id}/team`}
                    className={buttonVariants({ variant: "secondary", size: "sm" })}
                  >
                    Open team
                  </Link>
                  <Link
                    href={`/events/${team.event_id}`}
                    className={buttonVariants({ variant: "ghost", size: "sm" })}
                  >
                    Event
                  </Link>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div id="hosting" className="mt-10 scroll-mt-24">
        <SectionHeading title="HOSTING" subtitle="Draft, publish and run your events.">
          <Link href="/host" className="chunky-link font-mono text-sm">
            Organizer home →
          </Link>
        </SectionHeading>
        {data.hosted_events.length === 0 ? (
          <EmptyState
            title="NOT HOSTING ANYTHING"
            action={
              <Link href="/host/events/new" className={buttonVariants({ variant: "secondary" })}>
                Create an event
              </Link>
            }
          >
            Publishing takes about five minutes once the dates are set.
          </EmptyState>
        ) : (
          <ul className="mt-4 space-y-3">
            {data.hosted_events.map((event) => (
              <li
                key={event.id}
                className="flex flex-wrap items-center justify-between gap-3 border-2 border-ink bg-surface p-4 shadow-hard-sm"
              >
                <div>
                  <Link href={`/host/events/${event.id}`} className="chunky-link font-pixel text-lg">
                    {event.title}
                  </Link>
                  <p className="mt-1 font-mono text-sm text-muted">
                    {event.registration_count} registered · {event.submission_count} submitted ·{" "}
                    {event.is_judge ? "judging live" : "judging " + event.judging_status}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
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
      </div>

      <div id="judging" className="mt-10 scroll-mt-24">
        <SectionHeading title="JUDGING" subtitle="Comparisons waiting for your call.">
          <Link href="/judge" className="chunky-link font-mono text-sm">
            Judging home →
          </Link>
        </SectionHeading>
        {data.judge_events.length === 0 ? (
          <EmptyState title="NO JUDGING ASSIGNMENTS">
            Organizers invite judges by email. Accept the invite and the event shows up here.
          </EmptyState>
        ) : (
          <ul className="mt-4 grid gap-4 sm:grid-cols-2">
            {data.judge_events.map((event) => (
              <li key={event.id} className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <p className="font-pixel text-lg">{event.title}</p>
                    <p className="font-mono text-sm text-muted">
                      {formatDate(event.starts_at)} · {statusLabel(event.judging_status)}
                    </p>
                  </div>
                  <Badge tone={statusTone(event.judging_status)}>
                    {statusLabel(event.judging_status)}
                  </Badge>
                </div>
                <Link
                  href={`/judge/events/${event.id}`}
                  className={`${buttonVariants({ size: "sm" })} mt-3`}
                >
                  Open judging
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function Stat({ label, value, href }: { label: string; value: number; href: string }) {
  return (
    <a
      href={href}
      className="border-2 border-ink bg-surface p-4 shadow-hard-sm transition-transform hover:-translate-y-0.5"
    >
      <span className="block font-pixel text-3xl">{value}</span>
      <span className="mt-1 block font-mono text-sm uppercase tracking-wide text-muted">{label}</span>
    </a>
  );
}

function SectionHeading({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 className="font-pixel text-2xl">{title}</h2>
        <p className="mt-1 text-base text-muted">{subtitle}</p>
      </div>
      {children}
    </div>
  );
}
