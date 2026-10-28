"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { EventView, FormField, Registration, Team } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Button } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import { DynamicField } from "@/components/forms/dynamic-field";
import { formatDate } from "@/lib/utils";

export default function RegisterPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const { user, profile } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [event, setEvent] = useState<EventView | null>(null);
  const [fields, setFields] = useState<FormField[]>([]);
  const [team, setTeam] = useState<Team | null>(null);
  const [registration, setRegistration] = useState<Registration | null>(null);
  const [answers, setAnswers] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!user) return;
    async function load() {
      try {
        const [eventData, formData, submissionFormData, regData, teamData] = await Promise.all([
          api<{ event: EventView }>(`/events/${params.id}`),
          api<{ fields: FormField[] }>(`/events/${params.id}/form`),
          api<{ fields: FormField[] }>(`/events/${params.id}/form`, { query: { kind: "submission" } }),
          api<{ registration: Registration | null }>(`/events/${params.id}/registrations/me`),
          api<{ team: Team | null }>(`/events/${params.id}/teams/mine`),
        ]);
        setEvent(eventData.event);
        setFields([
          ...(formData.fields ?? []),
          ...(eventData.event.merge_registration_submission ? submissionFormData.fields ?? [] : []),
        ]);
        setRegistration(regData.registration);
        setTeam(teamData.team);
        const base: Record<string, unknown> = {
          full_name: profile?.full_name ?? "",
          college: profile?.college ?? "",
          course: profile?.course ?? "",
          year_of_study: profile?.year_of_study ?? null,
          email: user?.email ?? "",
          ...(regData.registration?.answers ?? {}),
        };
        setAnswers(base);
      } catch (err) {
        setError(errorMessage(err));
      } finally {
        setLoading(false);
      }
    }
    void load();
  }, [params.id, user, profile]);

  function setValue(key: string, value: unknown) {
    setAnswers((current) => ({ ...current, [key]: value }));
  }

  async function onSubmit(formEvent: React.FormEvent) {
    formEvent.preventDefault();
    setError(null);
    setErrors({});
    if (!event) return;
    setBusy(true);
    try {
      const payload = {
        team_id: team?.is_member ? team.id : null,
        answers,
      };
      if (registration && registration.status !== "cancelled") {
        await api(`/events/${event.id}/registrations/me`, { method: "PATCH", body: { answers } });
      } else {
        await api(`/events/${event.id}/registrations`, { method: "POST", body: payload });
      }
      router.push(`/events/${event.id}`);
    } catch (err) {
      setError(errorMessage(err));
      setErrors(fieldErrors(err));
    } finally {
      setBusy(false);
    }
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Loading form" />
      </div>
    );
  }

  if (!event) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <Alert tone="danger">{error ?? "Event not found."}</Alert>
      </div>
    );
  }

  const editing = registration && registration.status !== "cancelled";
  const teamRequired = event.participation_format === "team" && !team?.is_member;

  return (
    <div className="mx-auto max-w-reading px-4 py-10 sm:px-6">
      <Link href={`/events/${event.id}`} className="chunky-link font-mono text-sm">
        ← {event.title}
      </Link>
      <header className="mt-4">
        <p className="font-pixel text-sm text-muted">{editing ? "EDIT REGISTRATION" : "REGISTRATION"}</p>
        <h1 className="mt-1 font-pixel text-3xl">{event.title}</h1>
        <p className="mt-2 text-base text-muted">
          Closes {formatDate(event.registration_closes_at, true)}. You can edit until then.
        </p>
        {event.merge_registration_submission ? (
          <p className="mt-1 text-base text-muted">
            Registration and submission are one step for this event. Your project answers are saved with
            your registration.
          </p>
        ) : null}
      </header>

      {teamRequired ? (
        <Alert tone="warning" className="mt-5" title="This event is team-only">
          Join or create a team before registering.{" "}
          <Link className="chunky-link font-semibold" href={`/events/${event.id}/team`}>
            Go to teams
          </Link>
          .
        </Alert>
      ) : null}

      {team?.is_member ? (
        <Alert tone="info" className="mt-5">
          You are registering with <strong>{team.name}</strong>. Every current teammate will be registered
          with these answers.
        </Alert>
      ) : null}

      {error ? (
        <Alert tone="danger" className="mt-5">
          {error}
        </Alert>
      ) : null}

      <form onSubmit={onSubmit} className="mt-6 space-y-6 border-2 border-ink bg-surface p-5 shadow-hard sm:p-6">
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label="Full name" htmlFor="full_name" error={errors.full_name}>
            <Input
              id="full_name"
              value={String(answers.full_name ?? "")}
              onChange={(e) => setValue("full_name", e.target.value)}
              required
            />
          </Field>
          <Field label="College or company" htmlFor="college" error={errors.college}>
            <Input
              id="college"
              value={String(answers.college ?? "")}
              onChange={(e) => setValue("college", e.target.value)}
            />
          </Field>
          <Field label="Course or role" htmlFor="course" error={errors.course}>
            <Input
              id="course"
              value={String(answers.course ?? "")}
              onChange={(e) => setValue("course", e.target.value)}
            />
          </Field>
          <Field label="Email" htmlFor="email" hint="Used for event updates.">
            <Input id="email" type="email" value={String(answers.email ?? "")} readOnly className="bg-bg" />
          </Field>
        </div>

        {fields.length > 0 ? <div className="pixel-divider" /> : null}

        {fields.map((field) => (
          <DynamicField
            key={field.id}
            field={field}
            value={answers[field.id]}
            error={errors[field.id]}
            onChange={(value) => setValue(field.id, value)}
          />
        ))}

        <div className="flex flex-wrap items-center justify-between gap-3 border-t-2 border-ink pt-5">
          <Link href={`/events/${event.id}`} className="chunky-link font-mono text-sm">
            Cancel
          </Link>
          <Button type="submit" disabled={busy || teamRequired}>
            {busy ? "Saving…" : editing ? "Save registration" : "Complete registration"}
          </Button>
        </div>
      </form>
    </div>
  );
}

