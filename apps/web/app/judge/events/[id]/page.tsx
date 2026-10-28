"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { EventView, FormField, JudgingNext, JudgingSubmission } from "@/lib/types";
import { SubmissionAnswers } from "@/components/forms/submission-answers";
import { Alert, LoadingBlock, Progress } from "@/components/ui/feedback";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { statusLabel } from "@/lib/utils";

export default function JudgeEventPage() {
  const params = useParams<{ id: string }>();
  const eventId = params.id;
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();

  const [event, setEvent] = useState<EventView | null>(null);
  const [formFields, setFormFields] = useState<FormField[]>([]);
  const [next, setNext] = useState<JudgingNext | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [lastPairId, setLastPairId] = useState<string | null>(null);

  const fetchNext = useCallback(async () => {
    const data = await api<JudgingNext>(`/judge/events/${eventId}/next`);
    setNext(data);
    setLastPairId(data.progress?.last_pair_id ?? null);
    return data;
  }, [eventId]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [eventData, formData] = await Promise.all([
        api<{ event: EventView }>(`/events/${eventId}`),
        api<{ fields: FormField[] }>(`/events/${eventId}/form`, {
          query: { kind: "submission" },
        }),
      ]);
      setEvent(eventData.event);
      setFormFields(formData.fields ?? []);
      if (eventData.event.judging_status === "live") {
        await fetchNext();
      } else {
        setNext(null);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId, fetchNext]);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  async function decide(winner: "a" | "b" | "skip" | "conflict") {
    const pair = next?.pair;
    if (!pair || busy) return;
    if (winner === "conflict") {
      if (
        !window.confirm(
          "Flag a conflict with these entries? This pair and every remaining pair involving either entry will be skipped for you.",
        )
      ) {
        return;
      }
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const body =
        winner === "skip"
          ? { skipped: true }
          : winner === "conflict"
            ? { conflict: true }
            : { winner };
      await api(`/judge/events/${eventId}/comparisons/${pair.id}`, { method: "POST", body });
      setLastPairId(pair.id);
      if (winner === "skip") setNotice("Skipped. This pair will not come back.");
      else if (winner === "conflict") setNotice("Conflict flagged. Related pairs were skipped.");
      else setNotice("Saved.");
      await fetchNext();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  async function undo() {
    if (!lastPairId || busy) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await api(`/judge/events/${eventId}/comparisons/${lastPairId}/undo`, { method: "POST" });
      setLastPairId(null);
      setNotice("Last call undone. The pair is back in your queue.");
      await fetchNext();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  const pair = next?.pair ?? null;
  const leftIsA = pair ? pair.left_submission === pair.a.id : true;
  const left = pair ? (leftIsA ? pair.a : pair.b) : null;
  const right = pair ? (leftIsA ? pair.b : pair.a) : null;
  const leftWinner: "a" | "b" = leftIsA ? "a" : "b";
  const rightWinner: "a" | "b" = leftIsA ? "b" : "a";

  useEffect(() => {
    function onKeyDown(keyEvent: KeyboardEvent) {
      if (!pair || busy) return;
      const target = keyEvent.target as HTMLElement | null;
      const tag = target?.tagName?.toLowerCase();
      if (tag === "input" || tag === "textarea" || tag === "select" || target?.isContentEditable) {
        return;
      }
      if (keyEvent.metaKey || keyEvent.ctrlKey || keyEvent.altKey) return;
      if (keyEvent.key === "1") {
        keyEvent.preventDefault();
        void decide(leftWinner);
      } else if (keyEvent.key === "2") {
        keyEvent.preventDefault();
        void decide(rightWinner);
      }
    }
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  });

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-content px-4 py-12 sm:px-6">
        <LoadingBlock label="Loading your queue" />
      </div>
    );
  }

  if (!event) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12 sm:px-6">
        <Alert tone="danger" title="Event not available">
          {error ?? "You are not assigned to judge this event."}
        </Alert>
        <Link href="/judge" className={`${buttonVariants({ variant: "secondary" })} mt-4`}>
          Back to judging queue
        </Link>
      </div>
    );
  }

  const progress = next?.progress;
  const done = progress ? progress.total > 0 && progress.done >= progress.total : false;

  return (
    <div className="mx-auto max-w-content px-4 py-6 sm:px-6 sm:py-8">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <Link href="/judge" className="chunky-link font-mono text-sm">
            ← Judging queue
          </Link>
          <h1 className="mt-2 font-pixel text-2xl leading-tight sm:text-3xl">{event.title}</h1>
        </div>
        <Badge tone={done ? "success" : "accent"}>
          {event.judging_status === "live" ? (done ? "COMPLETE" : "LIVE") : statusLabel(event.judging_status)}
        </Badge>
      </div>

      {progress ? (
        <div className="mt-5 border-2 border-ink bg-surface p-4 shadow-hard-sm">
          <Progress
            value={progress.done}
            max={Math.max(progress.total, 1)}
            label="Your comparisons"
          />
        </div>
      ) : null}

      {error ? (
        <Alert tone="danger" className="mt-4">
          {error}
        </Alert>
      ) : null}
      {notice ? (
        <Alert tone="success" className="mt-4">
          {notice}
        </Alert>
      ) : null}

      {event.judging_status !== "live" ? (
        <div className="mt-6 border-2 border-ink bg-surface p-6 shadow-hard">
          <h2 className="font-pixel text-2xl">NOT LIVE YET</h2>
          <p className="mt-2 text-base text-muted">
            The organizer has not started judging. You will be able to compare entries once they do.
          </p>
          <Link href={`/events/${event.id}`} className={`${buttonVariants({ variant: "secondary" })} mt-4`}>
            View event
          </Link>
        </div>
      ) : pair && left && right ? (
        <div className="mt-6 grid gap-5 lg:grid-cols-[1fr_260px]">
          <div className="grid gap-4 md:grid-cols-2">
            <SubmissionPanel
              label="ENTRY 1"
              shortcut="1"
              submission={left}
              fields={formFields}
              onPick={() => void decide(leftWinner)}
              disabled={busy}
            />
            <SubmissionPanel
              label="ENTRY 2"
              shortcut="2"
              submission={right}
              fields={formFields}
              onPick={() => void decide(rightWinner)}
              disabled={busy}
            />
          </div>

          <aside className="space-y-4">
            <WindowCard title="CRITERIA">
              <div className="p-4">
                {next?.criteria?.length ? (
                  <ol className="space-y-3">
                    {next.criteria.map((criterion, index) => (
                      <li key={`${criterion.name}-${index}`}>
                        <p className="font-semibold">
                          <span className="mr-1 font-pixel text-sm text-accent-2">{index + 1}.</span>
                          {criterion.name}
                        </p>
                        {criterion.description ? (
                          <p className="text-sm text-muted">{criterion.description}</p>
                        ) : null}
                      </li>
                    ))}
                  </ol>
                ) : (
                  <p className="text-base text-muted">
                    No criteria posted. Compare on overall strength and usefulness.
                  </p>
                )}
                <p className="mt-4 border-t-2 border-ink/20 pt-3 text-sm text-muted">
                  There is no right answer beyond which entry you believe is stronger. Your call is saved
                  immediately.
                </p>
              </div>
            </WindowCard>

            <div className="space-y-2 border-2 border-ink bg-surface p-4 shadow-hard-sm">
              <p className="font-pixel text-sm">SHORTCUTS</p>
              <p className="font-mono text-sm text-muted">1 / 2 pick entry</p>
              <p className="font-mono text-sm text-muted">Every choice is final until you undo.</p>
              <div className="space-y-2 pt-2">
                <Button
                  variant="secondary"
                  className="w-full"
                  size="sm"
                  disabled={busy}
                  onClick={() => void decide("skip")}
                >
                  Skip this pair
                </Button>
                <Button
                  variant="ghost"
                  className="w-full"
                  size="sm"
                  disabled={busy}
                  onClick={() => void decide("conflict")}
                >
                  I know these entries
                </Button>
                {lastPairId ? (
                  <Button
                    variant="ghost"
                    className="w-full"
                    size="sm"
                    disabled={busy}
                    onClick={() => void undo()}
                  >
                    Undo last call
                  </Button>
                ) : null}
              </div>
            </div>
          </aside>
        </div>
      ) : (
        <div className="mt-6 border-2 border-ink bg-surface p-6 shadow-hard">
          <h2 className="font-pixel text-2xl">QUEUE CLEAR</h2>
          <p className="mt-2 max-w-reading text-base text-muted">
            You have no pending comparisons for this event. If the organizer adds more, they will appear
            here.
          </p>
          <div className="mt-4 flex flex-wrap gap-2">
            {lastPairId ? (
              <Button variant="secondary" onClick={() => void undo()} disabled={busy}>
                {busy ? "Undoing…" : "Undo last call"}
              </Button>
            ) : null}
            <Link href="/judge" className={buttonVariants({ variant: "secondary" })}>
              Back to queue
            </Link>
            <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "ghost" })}>
              View event
            </Link>
          </div>
        </div>
      )}
    </div>
  );
}

function SubmissionPanel({
  label,
  shortcut,
  submission,
  fields,
  onPick,
  disabled,
}: {
  label: string;
  shortcut: string;
  submission: JudgingSubmission;
  fields: FormField[];
  onPick: () => void;
  disabled: boolean;
}) {
  return (
    <article className="flex flex-col border-2 border-ink bg-surface shadow-hard">
      <div className="flex items-center justify-between gap-2 border-b-2 border-ink bg-accent-3 px-4 py-2">
        <span className="font-pixel text-sm">{label}</span>
        <span className="flex h-7 w-7 items-center justify-center border-2 border-ink bg-surface font-pixel text-sm">
          {shortcut}
        </span>
      </div>
      <div className="flex flex-1 flex-col p-4">
        <h2 className="font-pixel text-2xl leading-tight">{submission.title}</h2>
        <p className="mt-1 font-mono text-sm text-muted">
          {submission.team_name ? `${submission.team_name} · ` : ""}
          @{submission.creator_username}
        </p>
        <p className="mt-3 max-h-72 overflow-y-auto whitespace-pre-wrap text-base">
          {submission.description || "No description provided."}
        </p>
        <div className="mt-4 flex flex-wrap gap-2">
          {submission.repo_url ? (
            <a
              href={submission.repo_url}
              target="_blank"
              rel="noreferrer noopener"
              className="chunky-link font-mono text-sm"
            >
              Repository ↗
            </a>
          ) : null}
          {submission.demo_url ? (
            <a
              href={submission.demo_url}
              target="_blank"
              rel="noreferrer noopener"
              className="chunky-link font-mono text-sm"
            >
              Demo ↗
            </a>
          ) : null}
          {submission.file_url ? (
            <a
              href={submission.file_url}
              target="_blank"
              rel="noreferrer noopener"
              className="chunky-link font-mono text-sm"
            >
              File ↗
            </a>
          ) : null}
        </div>
        <SubmissionAnswers
          fields={fields}
          answers={submission.answers}
          className="mt-4 border-t-2 border-ink/20 pt-3"
        />
        <div className="mt-4 flex-1" />
        <Button className="mt-4 w-full" size="lg" onClick={onPick} disabled={disabled}>
          Choose this entry
        </Button>
      </div>
    </article>
  );
}
