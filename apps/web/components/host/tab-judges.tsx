"use client";

import { useCallback, useEffect, useState } from "react";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import type { StaffMember } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/input";
import { WindowCard } from "@/components/ui/card";

export function TabJudges({ eventId }: { eventId: string }) {
  const [judges, setJudges] = useState<StaffMember[]>([]);
  const [organizers, setOrganizers] = useState<StaffMember[]>([]);
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("judge");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [judgeData, organizerData] = await Promise.all([
        api<{ staff: StaffMember[] }>(`/events/${eventId}/staff`, { query: { role: "judge" } }),
        api<{ staff: StaffMember[] }>(`/events/${eventId}/staff`, {
          query: { role: "co_organizer" },
        }),
      ]);
      setJudges(judgeData.staff ?? []);
      setOrganizers(organizerData.staff ?? []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function invite(event: React.FormEvent) {
    event.preventDefault();
    setBusy("invite");
    setError(null);
    setNotice(null);
    setFields({});
    try {
      const data = await api<{ staff: StaffMember }>(`/events/${eventId}/staff`, {
        method: "POST",
        body: { email, role },
      });
      setEmail("");
      setNotice(
        data.staff.accept_url
          ? "Invite created. Share the acceptance link with them."
          : "Staff updated.",
      );
      await load();
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(null);
    }
  }

  async function remove(member: StaffMember) {
    if (!window.confirm(`Remove ${member.email} from this event?`)) return;
    setBusy(member.id);
    setError(null);
    setNotice(null);
    try {
      await api(`/events/${eventId}/staff/${member.id}`, { method: "DELETE" });
      setNotice(`${member.email} removed.`);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  }

  async function copy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      setNotice("Acceptance link copied.");
    } catch {
      setError("Could not copy. Select the link manually.");
    }
  }

  return (
    <div className="space-y-6">
      {error ? <Alert tone="danger">{error}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      <WindowCard title="INVITE SOMEONE">
        <form className="space-y-4 p-5" onSubmit={invite} noValidate>
          <p className="text-base text-muted">
            Judges compare submissions. Co-organizers can manage the event but do not judge unless also
            invited as a judge. Existing accounts join immediately. If the account does not exist yet,
            the invite link works after they sign up.
          </p>
          <div className="grid gap-4 sm:grid-cols-[1fr_200px_auto] sm:items-end">
            <Field
              label="Email or username"
              htmlFor="staff-email"
              error={fields.email ?? fields.username}
              hint="Type an email address or an existing username."
            >
              <Input
                id="staff-email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="judge@college.edu or @username"
              />
            </Field>
            <Field label="Role" htmlFor="staff-role">
              <Select id="staff-role" value={role} onChange={(event) => setRole(event.target.value)}>
                <option value="judge">Judge</option>
                <option value="co_organizer">Co-organizer</option>
              </Select>
            </Field>
            <Button type="submit" disabled={busy !== null || email.trim().length < 2}>
              {busy === "invite" ? "Inviting…" : "Invite"}
            </Button>
          </div>
        </form>
      </WindowCard>

      {loading ? (
        <LoadingBlock label="Loading staff" />
      ) : (
        <>
          <StaffList
            title="JUDGES"
            empty="No judges invited yet. At least one accepted judge is required before assigning comparisons."
            members={judges}
            busy={busy}
            onRemove={remove}
            onCopy={copy}
          />
          <StaffList
            title="CO-ORGANIZERS"
            empty="No co-organizers. You are managing this event alone."
            members={organizers}
            busy={busy}
            onRemove={remove}
            onCopy={copy}
          />
        </>
      )}
    </div>
  );
}

function StaffList({
  title,
  empty,
  members,
  busy,
  onRemove,
  onCopy,
}: {
  title: string;
  empty: string;
  members: StaffMember[];
  busy: string | null;
  onRemove: (member: StaffMember) => void;
  onCopy: (value: string) => void;
}) {
  return (
    <WindowCard title={title}>
      {members.length === 0 ? (
        <p className="p-5 text-base text-muted">{empty}</p>
      ) : (
        <ul className="divide-y-2 divide-ink/15">
          {members.map((member) => {
            const link =
              member.accept_url ||
              (member.invite_token ? `/invites/staff/${member.invite_token}` : undefined);
            return (
              <li key={member.id} className="space-y-2 p-4">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="font-semibold">
                      {member.username ? `@${member.username}` : member.email}
                    </p>
                    {member.username ? (
                      <p className="font-mono text-sm text-muted">{member.email}</p>
                    ) : null}
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge tone={statusTone(member.status)}>{member.status.toUpperCase()}</Badge>
                    <Button
                      size="sm"
                      variant="destructive"
                      disabled={busy !== null}
                      onClick={() => onRemove(member)}
                    >
                      Remove
                    </Button>
                  </div>
                </div>
                {member.status === "invited" && link ? (
                  <div className="flex flex-wrap items-center gap-2 border-2 border-ink/20 bg-bg/50 p-2">
                    <code className="min-w-0 flex-1 truncate font-mono text-xs">{link}</code>
                    <Button size="sm" variant="secondary" onClick={() => onCopy(link)}>
                      Copy link
                    </Button>
                  </div>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </WindowCard>
  );
}
