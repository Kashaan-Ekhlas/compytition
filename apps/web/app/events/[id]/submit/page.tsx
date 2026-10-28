"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { EventView, FormField, Registration, Submission } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { DynamicField } from "@/components/forms/dynamic-field";
import { formatDate } from "@/lib/utils";

export default function SubmitPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [event, setEvent] = useState<EventView | null>(null);
  const [submission, setSubmission] = useState<Submission | null>(null);
  const [registration, setRegistration] = useState<Registration | null>(null);
  const [form, setForm] = useState({
    title: "",
    description: "",
    repo_url: "",
    demo_url: "",
    file_url: "",
    status: "submitted",
  });
  const [formFields, setFormFields] = useState<FormField[]>([]);
  const [answers, setAnswers] = useState<Record<string, unknown>>({});
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!user) return;
    async function load() {
      try {
        const [eventData, subData, regData, formData] = await Promise.all([
          api<{ event: EventView }>(`/events/${params.id}`),
          api<{ submission: Submission | null }>(`/events/${params.id}/submissions/me`),
          api<{ registration: Registration | null }>(`/events/${params.id}/registrations/me`),
          api<{ fields: FormField[] }>(`/events/${params.id}/form`, { query: { kind: "submission" } }),
        ]);
        setEvent(eventData.event);
        setSubmission(subData.submission);
        setRegistration(regData.registration);
        const fields = formData.fields ?? [];
        setFormFields(fields);
        // Merged registration forms already collected the submission answers.
        // Keep only real submission field IDs so stale registration keys are
        // not posted back as unknown answers.
        const fieldIDs = new Set(fields.map((field) => field.id));
        const merged = {
          ...(regData.registration?.answers ?? {}),
          ...(subData.submission?.answers ?? {}),
        };
        setAnswers(
          Object.fromEntries(Object.entries(merged).filter(([key]) => fieldIDs.has(key))),
        );
        if (subData.submission) {
          const s = subData.submission;
          setForm({
            title: s.title,
            description: s.description,
            repo_url: s.repo_url ?? "",
            demo_url: s.demo_url ?? "",
            file_url: s.file_url ?? "",
            status: s.status,
          });
        }
      } catch (err) {
        setError(errorMessage(err));
      } finally {
        setLoading(false);
      }
    }
    void load();
  }, [params.id, user]);

  async function onSubmit(formEvent: React.FormEvent) {
    formEvent.preventDefault();
    setError(null);
    setFields({});
    setBusy(true);
    try {
      const fieldIDs = new Set(formFields.map((field) => field.id));
      const cleanAnswers = Object.fromEntries(
        Object.entries(answers).filter(([key]) => fieldIDs.has(key)),
      );
      const body = { ...form, answers: cleanAnswers };
      if (submission) {
        await api(`/submissions/${submission.id}`, { method: "PATCH", body });
      } else {
        await api(`/events/${params.id}/submissions`, { method: "POST", body });
      }
      router.push(`/events/${params.id}`);
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(false);
    }
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Loading submission" />
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

  if (!registration || registration.status !== "registered") {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <Alert tone="warning" title="Register first">
          You need a confirmed registration before submitting work.
        </Alert>
        <Link href={`/events/${event.id}/register`} className="chunky-link mt-4 inline-block font-semibold">
          Register for {event.title} →
        </Link>
      </div>
    );
  }

  const deadline = event.submission_deadline ? new Date(event.submission_deadline) : null;

  return (
    <div className="mx-auto max-w-reading px-4 py-10 sm:px-6">
      <Link href={`/events/${event.id}`} className="chunky-link font-mono text-sm">
        ← {event.title}
      </Link>
      <header className="mt-4">
        <p className="font-pixel text-sm text-muted">{submission ? "EDIT SUBMISSION" : "NEW SUBMISSION"}</p>
        <h1 className="mt-1 font-pixel text-3xl">{submission ? submission.title : "SHOW YOUR WORK"}</h1>
        <p className="mt-2 text-base text-muted">
          Deadline: {formatDate(event.submission_deadline, true)}
          {deadline && deadline.getTime() < Date.now() ? " · passed" : ""}
        </p>
        {registration.team_id ? (
          <p className="mt-1 font-mono text-sm text-muted">
            This submission represents your team ({registration.team_name || "your team"}).
          </p>
        ) : null}
      </header>

      {error ? <Alert tone="danger" className="mt-5">{error}</Alert> : null}

      <form onSubmit={onSubmit} className="mt-6 space-y-5 border-2 border-ink bg-surface p-5 shadow-hard sm:p-6">
        <Field label="Project title" htmlFor="title" error={fields.title}>
          <Input
            id="title"
            value={form.title}
            onChange={(e) => setForm({ ...form, title: e.target.value })}
            maxLength={160}
            required
          />
        </Field>
        <Field
          label="What did you build?"
          htmlFor="description"
          error={fields.description}
          hint="Judges read this before looking at the links. Keep it concrete."
        >
          <Textarea
            id="description"
            className="min-h-40"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
            maxLength={20_000}
          />
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Repository URL" htmlFor="repo" error={fields.repo_url} hint="Optional">
            <Input
              id="repo"
              type="url"
              placeholder="https://github.com/…"
              value={form.repo_url}
              onChange={(e) => setForm({ ...form, repo_url: e.target.value })}
            />
          </Field>
          <Field label="Live demo URL" htmlFor="demo" error={fields.demo_url} hint="Optional">
            <Input
              id="demo"
              type="url"
              placeholder="https://…"
              value={form.demo_url}
              onChange={(e) => setForm({ ...form, demo_url: e.target.value })}
            />
          </Field>
          <Field label="File or video URL" htmlFor="file" error={fields.file_url} hint="Optional">
            <Input
              id="file"
              type="url"
              placeholder="https://…"
              value={form.file_url}
              onChange={(e) => setForm({ ...form, file_url: e.target.value })}
            />
          </Field>
          <Field label="Visibility" htmlFor="status" error={fields.status} hint="Drafts are not judged.">
            <Select id="status" value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
              <option value="submitted">Submit for judging</option>
              <option value="draft">Keep as draft</option>
            </Select>
          </Field>
        </div>
        {formFields.length > 0 ? (
          <>
            <div className="pixel-divider" />
            {formFields.map((field) => (
              <DynamicField
                key={field.id}
                field={field}
                value={answers[field.id]}
                error={fields[field.id]}
                onChange={(value) =>
                  setAnswers((current) => ({ ...current, [field.id]: value }))
                }
              />
            ))}
          </>
        ) : null}
        <div className="flex flex-wrap items-center justify-between gap-3 border-t-2 border-ink pt-5">
          <Link href={`/events/${event.id}`} className="chunky-link font-mono text-sm">
            Cancel
          </Link>
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : submission ? "Save changes" : "Submit entry"}
          </Button>
        </div>
      </form>
    </div>
  );
}
