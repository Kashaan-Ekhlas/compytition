"use client";

import { useCallback, useEffect, useState } from "react";
import { api, errorMessage } from "@/lib/api";
import type { FormField, Registration } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/input";
import { formatDate, statusLabel } from "@/lib/utils";

const statuses = ["registered", "waitlisted", "cancelled", "disqualified"] as const;

export function TabParticipants({ eventId }: { eventId: string }) {
  const [rows, setRows] = useState<Registration[]>([]);
  const [fields, setFields] = useState<FormField[]>([]);
  const [filter, setFilter] = useState("");
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [list, form] = await Promise.all([
        api<{ registrations: Registration[]; total: number }>(`/events/${eventId}/registrations`, {
          query: { status: filter, limit: 200 },
        }),
        api<{ fields: FormField[] }>(`/events/${eventId}/form`),
      ]);
      setRows(list.registrations ?? []);
      setTotal(list.total ?? 0);
      setFields(form.fields ?? []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId, filter]);

  useEffect(() => {
    void load();
  }, [load]);

  async function setStatus(registration: Registration, status: string) {
    setBusy(registration.id);
    setError(null);
    setNotice(null);
    try {
      await api(`/registrations/${registration.id}/status`, {
        method: "PATCH",
        body: { status },
      });
      setNotice(`${registration.full_name || registration.username || "Participant"} marked ${status}.`);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  const answerRows = (registration: Registration) => {
    const answers = (registration.answers ?? {}) as Record<string, unknown>;
    const reserved: Array<[string, string]> = [
      ["full_name", "Name"],
      ["college", "College"],
      ["course", "Course"],
      ["year_of_study", "Year"],
      ["email", "Email"],
    ];
    const items: Array<{ label: string; value: string }> = [];
    for (const [key, label] of reserved) {
      const value = answers[key];
      if (value !== undefined && value !== null && value !== "") {
        items.push({ label, value: String(value) });
      }
    }
    for (const field of fields) {
      const value = answers[field.id];
      if (value === undefined || value === null || value === "") continue;
      items.push({
        label: field.label,
        value: Array.isArray(value) ? value.join(", ") : String(value),
      });
    }
    return items;
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="font-pixel text-2xl">PARTICIPANTS</h2>
          <p className="mt-1 text-base text-muted">
            {total} registration{total === 1 ? "" : "s"}. Waitlist, cancel or disqualify without deleting
            answers.
          </p>
        </div>
        <div className="flex items-end gap-2">
          <label className="font-mono text-sm" htmlFor="participant-filter">
            Status
          </label>
          <Select
            id="participant-filter"
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            className="w-44"
          >
            <option value="">All</option>
            {statuses.map((status) => (
              <option key={status} value={status}>
                {statusLabel(status)}
              </option>
            ))}
          </Select>
        </div>
      </div>

      {error ? <Alert tone="danger">{error}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      {loading ? (
        <LoadingBlock label="Loading participants" />
      ) : rows.length === 0 ? (
        <p className="border-2 border-dashed border-ink px-5 py-10 text-center text-base text-muted">
          No registrations{filter ? ` with status ${filter}` : ""} yet.
        </p>
      ) : (
        <ul className="space-y-3">
          {rows.map((registration) => (
            <li key={registration.id} className="border-2 border-ink bg-surface shadow-hard-sm">
              <div className="flex flex-wrap items-start justify-between gap-3 p-4">
                <div>
                  <p className="font-pixel text-lg">
                    {registration.full_name || registration.username || "Participant"}
                  </p>
                  <p className="font-mono text-sm text-muted">
                    {registration.username ? `@${registration.username} · ` : ""}
                    {registration.email}
                  </p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge tone={statusTone(registration.status)}>{statusLabel(registration.status)}</Badge>
                  {registration.has_submission ? <Badge tone="success">Submitted</Badge> : null}
                  <Select
                    aria-label={`Status for ${registration.full_name || registration.username}`}
                    value={registration.status}
                    disabled={busy !== null}
                    className="w-44"
                    onChange={(event) => void setStatus(registration, event.target.value)}
                  >
                    {statuses.map((status) => (
                      <option key={status} value={status}>
                        {statusLabel(status)}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>
              <div className="grid gap-3 border-t-2 border-ink/15 bg-bg/40 p-4 sm:grid-cols-2 lg:grid-cols-3">
                <Detail label="Team" value={registration.team_name || "Solo"} />
                <Detail label="Registered" value={formatDate(registration.created_at, true)} />
                {answerRows(registration).map((item, index) => (
                  <Detail key={`${item.label}-${index}`} label={item.label} value={item.value} />
                ))}
              </div>
            </li>
          ))}
        </ul>
      )}

      {rows.length > 0 ? (
        <p className="font-mono text-sm text-muted">
          Showing {rows.length} of {total}. Use the status filter for older entries.
        </p>
      ) : null}
    </div>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="font-mono text-xs uppercase tracking-wide text-muted">{label}</p>
      <p className="mt-0.5 break-words text-base">{value}</p>
    </div>
  );
}
