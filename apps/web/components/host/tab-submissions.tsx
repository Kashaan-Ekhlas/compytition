"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api, errorMessage } from "@/lib/api";
import type { Submission } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Select } from "@/components/ui/input";
import { formatDate, statusLabel } from "@/lib/utils";

export function TabSubmissions({ eventId }: { eventId: string }) {
  const [rows, setRows] = useState<Submission[]>([]);
  const [filter, setFilter] = useState("");
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api<{ submissions: Submission[]; total: number }>(
        `/events/${eventId}/submissions`,
        { query: { status: filter, limit: 200 } },
      );
      setRows(data.submissions ?? []);
      setTotal(data.total ?? 0);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId, filter]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="font-pixel text-2xl">SUBMISSIONS</h2>
          <p className="mt-1 text-base text-muted">
            {total} entr{total === 1 ? "y" : "ies"}. Only submitted entries are compared during judging.
          </p>
        </div>
        <div className="flex items-end gap-2">
          <label className="font-mono text-sm" htmlFor="submission-filter">
            Status
          </label>
          <Select
            id="submission-filter"
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            className="w-44"
          >
            <option value="">All</option>
            <option value="submitted">Submitted</option>
            <option value="draft">Draft</option>
          </Select>
        </div>
      </div>

      {error ? <Alert tone="danger">{error}</Alert> : null}

      {loading ? (
        <LoadingBlock label="Loading submissions" />
      ) : rows.length === 0 ? (
        <p className="border-2 border-dashed border-ink px-5 py-10 text-center text-base text-muted">
          No submissions{filter ? ` with status ${filter}` : ""} yet.
        </p>
      ) : (
        <ul className="space-y-3">
          {rows.map((submission) => (
            <li key={submission.id} className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="font-pixel text-lg">{submission.title}</p>
                  <p className="font-mono text-sm text-muted">
                    {submission.team_name ? `${submission.team_name} · ` : ""}
                    {submission.creator_name || submission.creator_username || "Participant"} · updated{" "}
                    {formatDate(submission.updated_at, true)}
                  </p>
                </div>
                <Badge tone={statusTone(submission.status)}>{statusLabel(submission.status)}</Badge>
              </div>
              {submission.description ? (
                <p className="mt-2 line-clamp-3 text-base text-muted">{submission.description}</p>
              ) : null}
              <div className="mt-3 flex flex-wrap gap-2">
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
                <Link href={`/events/${eventId}`} className="chunky-link font-mono text-sm">
                  Event page →
                </Link>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
