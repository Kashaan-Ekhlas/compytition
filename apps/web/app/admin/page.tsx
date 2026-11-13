"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type {
  AdminOverview,
  AdminResetResult,
  AdminSeedResult,
  AdminSeededEvent,
} from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Checkbox, Input, Label } from "@/components/ui/input";
import { WindowCard } from "@/components/ui/card";
import { formatDate, statusLabel } from "@/lib/utils";

const MAX_EVENTS = 10;

export default function AdminPage() {
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [overview, setOverview] = useState<AdminOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<"seed" | "reset" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [eventCount, setEventCount] = useState(MAX_EVENTS);
  const [resetFirst, setResetFirst] = useState(true);
  // Refresh runs can finish out of order, so only the newest request may write
  // state. Passing a spinner flag keeps refreshes from unmounting the controls.
  const requestRef = useRef(0);

  const load = useCallback(async (showSpinner = false) => {
    const requestID = requestRef.current + 1;
    requestRef.current = requestID;
    if (showSpinner) setLoading(true);
    setError(null);
    try {
      const next = await api<AdminOverview>("/admin/overview");
      if (requestRef.current === requestID) setOverview(next);
    } catch (err) {
      if (requestRef.current === requestID) setError(errorMessage(err));
    } finally {
      if (requestRef.current === requestID) setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (user?.is_admin) void load(true);
  }, [user, load]);

  if (authLoading) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <LoadingBlock label="Checking your access" />
      </div>
    );
  }

  if (!user) {
    return null;
  }

  if (!user.is_admin) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <Alert tone="danger" title="Admin only">
          This console is limited to admin accounts.{" "}
          <Link href="/" className="underline decoration-2 underline-offset-4">
            Go home
          </Link>
          .
        </Alert>
      </div>
    );
  }

  async function seed() {
    const message = resetFirst
      ? "Delete the previous seed data and generate a fresh set?"
      : "Add another set of seed data? Existing seed data is kept.";
    if (!window.confirm(message)) return;
    setBusy("seed");
    setError(null);
    setNotice(null);
    try {
      const result = await api<AdminSeedResult>("/admin/seed", {
        method: "POST",
        body: { events: eventCount, reset: resetFirst },
      });
      setNotice(
        `Seeded ${result.events} events, ${result.submissions} submissions and ${result.judgments} judgments.`,
      );
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  async function reset() {
    if (
      !window.confirm(
        "Delete every seed event, submission and seed account? This cannot be undone.",
      )
    ) {
      return;
    }
    setBusy("reset");
    setError(null);
    setNotice(null);
    try {
      const result = await api<AdminResetResult>("/admin/reset", { method: "POST" });
      setNotice(`Removed ${result.events_deleted} seed events and ${result.users_deleted} seed users.`);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="mx-auto max-w-content px-4 py-8 sm:px-6 sm:py-12">
      <header className="max-w-reading">
        <p className="font-pixel text-sm text-muted">ADMIN</p>
        <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">TEST DATA CONSOLE</h1>
        <p className="mt-2 text-lg text-muted">
          Generate varied events, submissions, judges and results so every public filter and
          judging mode has something to exercise.
        </p>
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

      {loading ? (
        <div className="mt-8">
          <LoadingBlock label="Loading admin overview" />
        </div>
      ) : overview ? (
        <>
          {!overview.seed_enabled ? (
            <Alert tone="info" title="Read-only in production" className="mt-8">
              Seeding and reset are disabled because this environment is production. You can still
              review totals and open the seeded events that already exist.
            </Alert>
          ) : null}

          <div className="mt-8 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            <Stat label="Users" value={overview.stats.users} />
            <Stat label="Events" value={overview.stats.events} />
            <Stat label="Submissions" value={overview.stats.submissions} />
            <Stat label="Judgments" value={overview.stats.judgments} />
            <Stat label="Uploads" value={overview.stats.uploads} />
          </div>

          <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_20rem]">
            <WindowCard title="SEED CONTROLS">
              <div className="space-y-5 p-5">
                <div className="grid gap-4 sm:grid-cols-2">
                  <div>
                    <Label htmlFor="events">Events to generate</Label>
                    <Input
                      id="events"
                      type="number"
                      min={1}
                      max={MAX_EVENTS}
                      value={eventCount}
                      disabled={!overview.seed_enabled}
                      onChange={(event) =>
                        setEventCount(
                          Math.min(MAX_EVENTS, Math.max(1, Number(event.target.value) || 1)),
                        )
                      }
                    />
                    <p className="mt-1 text-sm text-muted">
                      Templates cover categories, formats, locations, timelines and judging
                      modes. Maximum {MAX_EVENTS}.
                    </p>
                  </div>
                  <label className="flex items-start gap-3 border-2 border-ink bg-surface p-3">
                    <Checkbox
                      checked={resetFirst}
                      disabled={!overview.seed_enabled}
                      onChange={(event) => setResetFirst(event.target.checked)}
                    />
                    <span>
                      <span className="block font-pixel text-base">RESET SEED DATA FIRST</span>
                      <span className="mt-1 block text-sm text-muted">
                        Deletes previous seed events and seed accounts before creating a fresh set.
                      </span>
                    </span>
                  </label>
                </div>

                <div className="flex flex-wrap gap-3 border-t-2 border-dashed border-ink pt-4">
                  <Button
                    onClick={() => void seed()}
                    disabled={busy !== null || !overview.seed_enabled}
                  >
                    {busy === "seed" ? "Seeding…" : "Seed test data"}
                  </Button>
                  <Button
                    variant="destructive"
                    onClick={() => void reset()}
                    disabled={busy !== null || !overview.seed_enabled}
                  >
                    {busy === "reset" ? "Resetting…" : "Reset seed data"}
                  </Button>
                  <Button variant="ghost" onClick={() => void load()} disabled={busy !== null}>
                    Refresh
                  </Button>
                </div>

                <Alert tone="info" title="Seed accounts">
                  Every generated account uses the password <code>demo1234</code>. Participants are{" "}
                  <code>seed.user01@compytition.local</code> through{" "}
                  <code>seed.user16@compytition.local</code>; judges are{" "}
                  <code>seed.judge1@compytition.local</code> through{" "}
                  <code>seed.judge4@compytition.local</code>.
                </Alert>
              </div>
            </WindowCard>

            <WindowCard title="COVERAGE">
              <div className="space-y-3 p-5 text-base">
                <Coverage label="Seed events" value={overview.seed_events} />
                <Coverage label="Seed accounts" value={overview.seed_users} />
                <p className="text-sm text-muted">
                  Templates include individual, team and mixed formats; remote and in-person
                  locations across six countries; upcoming, open, closed and ended timelines; and
                  single-judge, multi-judge, live and published judging states.
                </p>
              </div>
            </WindowCard>
          </div>

          <WindowCard title="SEEDED EVENTS" className="mt-8">
            {overview.events.length === 0 ? (
              <p className="p-5 text-base text-muted">
                No seed events yet. Use the controls above to create a set.
              </p>
            ) : (
              <ul className="divide-y-2 divide-ink">
                {overview.events.map((event) => (
                  <SeedRow key={event.id} event={event} />
                ))}
              </ul>
            )}
          </WindowCard>
        </>
      ) : (
        <div className="mt-8">
          <Button onClick={() => void load(true)} disabled={busy !== null}>
            Retry loading the overview
          </Button>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
      <p className="font-pixel text-sm text-muted">{label.toUpperCase()}</p>
      <p className="mt-1 font-pixel text-3xl">{value}</p>
    </div>
  );
}

function Coverage({ label, value }: { label: string; value: number }) {
  return (
    <div className="flex items-center justify-between border-2 border-ink bg-bg px-3 py-2">
      <span className="text-muted">{label}</span>
      <span className="font-pixel text-xl">{value}</span>
    </div>
  );
}

function SeedRow({ event }: { event: AdminSeededEvent }) {
  const where = event.is_remote
    ? "Remote"
    : [event.city, event.state, event.country].filter(Boolean).join(", ");
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 p-4">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <Link href={`/events/${event.id}`} className="chunky-link font-pixel text-lg">
            {event.title}
          </Link>
          <Badge tone={statusTone(event.status)}>{statusLabel(event.status)}</Badge>
          <Badge tone="quiet">{event.judging_status.toUpperCase()}</Badge>
          {event.is_remote ? <Badge tone="blue">REMOTE</Badge> : null}
        </div>
        <p className="mt-1 font-mono text-sm text-muted">
          {event.category} · {event.participation_format} · {where || "Location TBD"}
        </p>
        <p className="mt-1 font-mono text-sm text-muted">
          Starts {formatDate(event.starts_at)} · deadline {formatDate(event.submission_deadline)} ·{" "}
          judging {event.judging_status}
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Link
          href={`/events/${event.id}`}
          className={buttonVariants({ variant: "ghost", size: "sm" })}
        >
          View
        </Link>
        <Link
          href={`/host/events/${event.id}`}
          className={buttonVariants({ variant: "secondary", size: "sm" })}
        >
          Manage
        </Link>
      </div>
    </li>
  );
}
