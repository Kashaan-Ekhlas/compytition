"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api, errorMessage } from "@/lib/api";
import type { EventView, JudgingPreview, JudgingReview } from "@/lib/types";
import { Alert, LoadingBlock, Progress } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { statusLabel } from "@/lib/utils";

export function TabJudging({
  event,
  onReload,
}: {
  event: EventView;
  onReload: () => Promise<void>;
}) {
  const [preview, setPreview] = useState<JudgingPreview | null>(null);
  const [review, setReview] = useState<JudgingReview | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const reviewData = await api<JudgingReview>(`/events/${event.id}/judging/review`);
      setReview({
        ...reviewData,
        rankings: reviewData.rankings ?? [],
        judges: reviewData.judges ?? [],
        disagreements: reviewData.disagreements ?? [],
        warnings: reviewData.warnings ?? [],
      });
      if (event.judging_status === "setup") {
        const previewData = await api<JudgingPreview>(`/events/${event.id}/judging/preview`);
        setPreview({
          ...previewData,
          submissions: previewData.submissions ?? [],
          judges: previewData.judges ?? [],
          warnings: previewData.warnings ?? [],
          plan: {
            ...previewData.plan,
            pairs: previewData.plan?.pairs ?? [],
            judge_stats: previewData.plan?.judge_stats ?? [],
            overlap: previewData.plan?.overlap ?? [],
            anchors: previewData.plan?.anchors ?? [],
            coverage: previewData.plan?.coverage ?? {},
          },
        });
      } else {
        setPreview(null);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [event.id, event.judging_status]);

  useEffect(() => {
    void load();
  }, [load]);

  async function assign() {
    setBusy("assign");
    setError(null);
    setNotice(null);
    try {
      const data = await api<{ total_pairs: number }>(`/events/${event.id}/judging/assign`, {
        method: "POST",
      });
      setNotice(`Generated ${data.total_pairs} comparisons.`);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  async function start() {
    if (!window.confirm("Start judging? Comparisons are locked once judges begin.")) return;
    setBusy("start");
    setError(null);
    setNotice(null);
    try {
      await api(`/events/${event.id}/judging/start`, { method: "POST" });
      await onReload();
      await load();
      setNotice("Judging is live. Judges can start comparing.");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  async function publish(force: boolean) {
    if (
      !window.confirm(
        force
          ? "Publish anyway? Pending comparisons will be skipped and cannot be judged afterwards."
          : "Publish results? This locks judging and writes the public ranking.",
      )
    ) {
      return;
    }
    setBusy("publish");
    setError(null);
    setNotice(null);
    try {
      await api(`/events/${event.id}/judging/publish`, { method: "POST", body: { force } });
      await onReload();
      await load();
      setNotice("Results published.");
    } catch (err) {
      const message = errorMessage(err);
      if (!force && message.toLowerCase().includes("pending")) {
        setError(message);
        if (window.confirm("Publish anyway with pending comparisons skipped?")) {
          setBusy(null);
          await publish(true);
          return;
        }
      } else {
        setError(message);
      }
    } finally {
      setBusy(null);
    }
  }

  if (loading) return <LoadingBlock label="Loading judging" />;

  const titleById = new Map<string, string>();
  for (const submission of preview?.submissions ?? []) {
    titleById.set(submission.id, submission.title);
  }
  for (const entry of review?.rankings ?? []) {
    titleById.set(entry.submission_id, entry.title);
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="font-pixel text-2xl">JUDGING</h2>
          <p className="mt-1 max-w-reading text-base text-muted">
            Preview the assignment, start judging, then review reliability before publishing.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Badge tone={statusTone(event.judging_status)}>
            {statusLabel(event.judging_status)}
          </Badge>
          <Button variant="ghost" size="sm" onClick={() => void load()} disabled={busy !== null}>
            Refresh
          </Button>
        </div>
      </div>

      {error ? <Alert tone="danger">{error}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      {review ? (
        <WindowCard title="COMPARISON PROGRESS">
          <div className="space-y-4 p-5">
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <Metric label="Pairs" value={String(review.total_pairs)} />
              <Metric label="Done" value={String(review.done_pairs)} />
              <Metric label="Pending" value={String(review.pending_pairs)} />
              <Metric label="Judgments" value={String(review.total_judgments)} />
            </div>
            <Progress
              value={review.done_pairs}
              max={Math.max(review.total_pairs, 1)}
              label="Comparisons complete"
            />
            <p className="font-mono text-sm text-muted">
              Coverage per entry: {review.coverage_min}–{review.coverage_max} comparisons.
            </p>
            {review.warnings.length > 0 ? (
              <Alert tone="warning" title="Before you publish">
                <ul className="list-disc space-y-1 pl-5">
                  {review.warnings.map((warning) => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              </Alert>
            ) : (
              <Alert tone="success">No coverage or reliability warnings.</Alert>
            )}
          </div>
        </WindowCard>
      ) : null}

      {event.judging_status === "setup" ? (
        <WindowCard title="ASSIGNMENT PREVIEW">
          <div className="space-y-4 p-5">
            {preview ? (
              <>
                <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                  <Metric label="Submissions" value={String(preview.submissions.length)} />
                  <Metric label="Judges" value={String(preview.judges.length)} />
                  <Metric label="Pairs" value={String(preview.plan.total_pairs)} />
                  <Metric
                    label="Connected"
                    value={preview.plan.connected ? "yes" : "no"}
                  />
                </div>
                <p className="font-mono text-sm text-muted">
                  Target coverage {preview.plan.target_coverage} · actual {preview.plan.coverage_min}–
                  {preview.plan.coverage_max} · {preview.plan.anchors.length} shared anchors
                </p>

                {preview.warnings.length > 0 ? (
                  <Alert tone="warning" title="Plan warnings">
                    <ul className="list-disc space-y-1 pl-5">
                      {preview.warnings.map((warning) => (
                        <li key={warning}>{warning}</li>
                      ))}
                    </ul>
                  </Alert>
                ) : null}

                <div className="overflow-x-auto">
                  <table className="w-full min-w-[520px] text-left">
                    <caption className="sr-only">Pair load per judge</caption>
                    <thead className="border-b-2 border-ink bg-accent-3 font-pixel text-sm">
                      <tr>
                        <th className="px-3 py-2">Judge</th>
                        <th className="px-3 py-2">Comparisons</th>
                        <th className="px-3 py-2">Entries seen</th>
                      </tr>
                    </thead>
                    <tbody>
                      {preview.judges.map((judge) => {
                        const stat = preview.plan.judge_stats.find((item) => item.judge_id === judge.id);
                        return (
                          <tr key={judge.id} className="border-b-2 border-ink/15 last:border-0">
                            <td className="px-3 py-2">
                              <span className="font-semibold">{judge.username || judge.email}</span>
                              <span className="block font-mono text-xs text-muted">{judge.email}</span>
                            </td>
                            <td className="px-3 py-2 font-mono">{stat?.comparisons ?? 0}</td>
                            <td className="px-3 py-2 font-mono">{stat?.submissions_seen ?? 0}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>

                {preview.plan.overlap.length > 0 ? (
                  <div>
                    <h3 className="font-pixel text-base">JUDGE OVERLAP</h3>
                    <ul className="mt-2 grid gap-2 sm:grid-cols-2">
                      {preview.plan.overlap.map((overlap) => (
                        <li
                          key={`${overlap.judge_a}-${overlap.judge_b}`}
                          className="border-2 border-ink/20 bg-bg/40 px-3 py-2 font-mono text-sm"
                        >
                          {nameFor(preview, overlap.judge_a)} × {nameFor(preview, overlap.judge_b)}:{" "}
                          {overlap.shared} shared
                        </li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </>
            ) : (
              <p className="text-base text-muted">No preview available.</p>
            )}

            <div className="flex flex-wrap gap-2 border-t-2 border-ink/20 pt-4">
              <Button variant="secondary" onClick={() => void assign()} disabled={busy !== null}>
                {busy === "assign" ? "Generating…" : "Generate comparisons"}
              </Button>
              <Button
                onClick={() => void start()}
                disabled={busy !== null || (review?.total_pairs ?? 0) === 0}
                title={
                  (review?.total_pairs ?? 0) === 0
                    ? "Generate comparisons first."
                    : undefined
                }
              >
                {busy === "start" ? "Starting…" : "Start judging"}
              </Button>
            </div>
            <p className="font-mono text-sm text-muted">
              Generating replaces the current plan. Start judging only once judges are ready.
            </p>
          </div>
        </WindowCard>
      ) : null}

      {review && review.judging_status !== "setup" ? (
        <>
          <WindowCard title="RANKING PREVIEW">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[760px] text-left">
                <thead className="border-b-2 border-ink bg-accent-3 font-pixel text-sm">
                  <tr>
                    <th className="px-3 py-2">#</th>
                    <th className="px-3 py-2">Entry</th>
                    <th className="px-3 py-2">Coverage</th>
                    <th className="px-3 py-2">W / L</th>
                    <th className="px-3 py-2">Score</th>
                    <th className="px-3 py-2">Confidence</th>
                  </tr>
                </thead>
                <tbody>
                  {review.rankings.map((entry) => (
                    <tr key={entry.submission_id} className="border-b-2 border-ink/15 last:border-0">
                      <td className="px-3 py-2 font-mono">{entry.rank}</td>
                      <td className="px-3 py-2">
                        <span className="block font-semibold">{entry.title}</span>
                        <span className="block font-mono text-xs text-muted">
                          {entry.team_name || `@${entry.creator_username}`}
                        </span>
                      </td>
                      <td className="px-3 py-2 font-mono">{entry.coverage}</td>
                      <td className="px-3 py-2 font-mono">
                        {entry.wins} / {entry.losses}
                      </td>
                      <td className="px-3 py-2 font-mono">{entry.score.toFixed(3)}</td>
                      <td className="px-3 py-2 font-mono">
                        {Math.round(entry.confidence * 100)}%
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </WindowCard>

          <div className="grid gap-6 lg:grid-cols-2">
            <WindowCard title="JUDGE RELIABILITY">
              {review.judges.length === 0 ? (
                <p className="p-5 text-base text-muted">No judgments yet.</p>
              ) : (
                <ul className="divide-y-2 divide-ink/15">
                  {review.judges.map((judge) => (
                    <li key={judge.id} className="space-y-2 p-4">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span className="font-semibold">
                          {judge.username || judge.email || judge.id.slice(0, 8)}
                        </span>
                        <span className="font-mono text-sm">
                          {(judge.reliability * 100).toFixed(0)}% reliable
                        </span>
                      </div>
                      <Progress
                        value={Math.round(judge.reliability * 100)}
                        max={100}
                        label={`${judge.comparisons} judgments · α ${judge.alpha.toFixed(1)} / β ${judge.beta.toFixed(1)}`}
                      />
                      <p className="font-mono text-xs text-muted">
                        {judge.done} of {judge.assigned} assigned comparisons done
                      </p>
                    </li>
                  ))}
                </ul>
              )}
            </WindowCard>

            <WindowCard title="SPLIT DECISIONS">
              {review.disagreements.length === 0 ? (
                <p className="p-5 text-base text-muted">
                  No pair has a split decision yet. Judges agree on every shared comparison.
                </p>
              ) : (
                <ul className="divide-y-2 divide-ink/15">
                  {review.disagreements.map((item) => (
                    <li key={item.pair_id} className="p-4">
                      <p className="font-semibold">
                        {item.title_a} <span className="text-muted">vs</span> {item.title_b}
                      </p>
                      <ul className="mt-1 space-y-0.5 font-mono text-sm text-muted">
                        {Object.entries(item.winner_counts).map(([submissionId, count]) => (
                          <li key={submissionId}>
                            {titleById.get(submissionId) ?? submissionId.slice(0, 8)}: {count} vote
                            {count === 1 ? "" : "s"}
                          </li>
                        ))}
                      </ul>
                    </li>
                  ))}
                </ul>
              )}
            </WindowCard>
          </div>

          <WindowCard title="PUBLISH">
            <div className="space-y-4 p-5">
              {event.judging_status === "published" ? (
                <>
                  <Alert tone="success">
                    Results were published{" "}
                    {event.results_published_at
                      ? new Date(event.results_published_at).toLocaleString()
                      : "already"}
                    . Judging is locked.
                  </Alert>
                  <Link href={`/events/${event.id}`} className={buttonVariants({ variant: "secondary" })}>
                    View public results
                  </Link>
                </>
              ) : (
                <>
                  <p className="text-base text-muted">
                    Publishing writes the ranking, snapshots judge reliability and locks judging. This
                    cannot be undone from the interface.
                  </p>
                  {review.pending_pairs > 0 ? (
                    <Alert tone="warning">
                      {review.pending_pairs} of {review.total_pairs} comparisons are still pending.
                    </Alert>
                  ) : null}
                  <div className="flex flex-wrap gap-2">
                    <Button
                      onClick={() => void publish(false)}
                      disabled={busy !== null || review.total_judgments === 0}
                    >
                      {busy === "publish" ? "Publishing…" : "Publish results"}
                    </Button>
                    {review.pending_pairs > 0 ? (
                      <Button
                        variant="destructive"
                        onClick={() => void publish(true)}
                        disabled={busy !== null || review.total_judgments === 0}
                      >
                        Publish with pending skipped
                      </Button>
                    ) : null}
                  </div>
                </>
              )}
            </div>
          </WindowCard>
        </>
      ) : null}
    </div>
  );
}

function nameFor(preview: JudgingPreview, judgeId: string): string {
  const judge = preview.judges.find((item) => item.id === judgeId);
  return judge?.username || judge?.email || judgeId.slice(0, 8);
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-2 border-ink bg-bg/40 p-3">
      <p className="font-pixel text-2xl">{value}</p>
      <p className="mt-0.5 font-mono text-xs uppercase tracking-wide text-muted">{label}</p>
    </div>
  );
}
