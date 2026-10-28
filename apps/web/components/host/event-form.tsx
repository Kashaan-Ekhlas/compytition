"use client";

import { useState } from "react";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import type { Criterion, EventView } from "@/lib/types";
import { Alert } from "@/components/ui/feedback";
import { Button } from "@/components/ui/button";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { useAuth } from "@/components/auth-provider";
import { Checkbox } from "@/components/ui/input";
import { fromDateTimeInput, toDateTimeInput } from "@/lib/utils";

const categories = ["Hackathon", "Competition", "Contest", "Workshop", "Other"];
const formats = [
  { value: "both", label: "Solo or team" },
  { value: "team", label: "Team only" },
  { value: "individual", label: "Solo only" },
];

type CriterionDraft = Criterion;

export function EventForm({
  initial,
  onSaved,
}: {
  initial?: EventView;
  onSaved: (event: EventView) => void;
}) {
  const { user } = useAuth();
  const [title, setTitle] = useState(initial?.title ?? "");
  const [description, setDescription] = useState(initial?.description ?? "");
  const [category, setCategory] = useState(initial?.category ?? "Hackathon");
  const [format, setFormat] = useState<string>(initial?.participation_format ?? "both");
  const [eligibility, setEligibility] = useState(initial?.eligibility ?? "");
  const [rules, setRules] = useState(initial?.rules ?? "");
  const [contactEmail, setContactEmail] = useState(initial?.contact_email ?? "");
  const [isRemote, setIsRemote] = useState(initial?.is_remote ?? true);
  const [country, setCountry] = useState(initial?.country ?? "");
  const [state, setState] = useState(initial?.state ?? "");
  const [city, setCity] = useState(initial?.city ?? "");
  const [mergeForms, setMergeForms] = useState(initial?.merge_registration_submission ?? false);
  const [maxTeamSize, setMaxTeamSize] = useState(String(initial?.max_team_size ?? 4));
  const [registrationOpens, setRegistrationOpens] = useState(
    toDateTimeInput(initial?.registration_opens_at),
  );
  const [registrationCloses, setRegistrationCloses] = useState(
    toDateTimeInput(initial?.registration_closes_at),
  );
  const [startsAt, setStartsAt] = useState(toDateTimeInput(initial?.starts_at));
  const [endsAt, setEndsAt] = useState(toDateTimeInput(initial?.ends_at));
  const [deadline, setDeadline] = useState(toDateTimeInput(initial?.submission_deadline));
  const [criteria, setCriteria] = useState<CriterionDraft[]>(
    initial?.criteria?.length
      ? initial.criteria
      : [
          { name: "Originality", description: "Is the idea meaningfully new?" },
          { name: "Craft", description: "Is the execution considered?" },
        ],
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  function updateCriterion(index: number, patch: Partial<CriterionDraft>) {
    setCriteria((current) =>
      current.map((criterion, position) => (position === index ? { ...criterion, ...patch } : criterion)),
    );
  }

  function removeCriterion(index: number) {
    setCriteria((current) => current.filter((_, position) => position !== index));
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setFields({});
    setBusy(true);
    try {
      const individual = format === "individual";
      const payload = {
        title,
        description,
        category,
        participation_format: format,
        eligibility,
        rules,
        contact_email: contactEmail,
        is_remote: isRemote,
        country: isRemote ? "" : country,
        state: isRemote ? "" : state,
        city: isRemote ? "" : city,
        merge_registration_submission: mergeForms,
        min_team_size: 1,
        max_team_size: individual ? 1 : Number(maxTeamSize),
        registration_opens_at: fromDateTimeInput(registrationOpens),
        registration_closes_at: fromDateTimeInput(registrationCloses),
        starts_at: fromDateTimeInput(startsAt),
        ends_at: fromDateTimeInput(endsAt),
        submission_deadline: fromDateTimeInput(deadline),
        criteria: criteria.filter((criterion) => criterion.name.trim().length > 0),
      };
      const data = initial
        ? await api<{ event: EventView }>(`/events/${initial.id}`, { method: "PATCH", body: payload })
        : await api<{ event: EventView }>("/events", { method: "POST", body: payload });
      onSaved(data.event);
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="space-y-6" onSubmit={submit} noValidate>
      {error ? <Alert tone="danger">{error}</Alert> : null}

      <fieldset className="space-y-4">
        <legend className="mb-2 font-pixel text-lg">BASICS</legend>
        <Field label="Title" htmlFor="title" error={fields.title}>
          <Input
            id="title"
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            maxLength={160}
            required
            placeholder="Spring Design Jam 2026"
          />
        </Field>
        <Field
          label="Description"
          htmlFor="description"
          error={fields.description}
          hint="What is this event, who is it for, and what should people build?"
        >
          <Textarea
            id="description"
            value={description}
            onChange={(event) => setDescription(event.target.value)}
            maxLength={10_000}
            className="min-h-36"
          />
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Category" htmlFor="category" error={fields.category}>
            <Select id="category" value={category} onChange={(event) => setCategory(event.target.value)}>
              {categories.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Format" htmlFor="format" error={fields.participation_format}>
            <Select id="format" value={format} onChange={(event) => setFormat(event.target.value)}>
              {formats.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Where is it?" htmlFor="mode" error={fields.location}>
            <Select
              id="mode"
              value={isRemote ? "remote" : "in-person"}
              onChange={(event) => setIsRemote(event.target.value === "remote")}
            >
              <option value="remote">Remote / Online</option>
              <option value="in-person">In person</option>
            </Select>
          </Field>
          <Field
            label="Contact email"
            htmlFor="contact_email"
            error={fields.contact_email}
            hint={user ? "Your account email is the default contact." : undefined}
          >
            <div className="flex gap-2">
              <Input
                id="contact_email"
                type="email"
                value={contactEmail}
                onChange={(event) => setContactEmail(event.target.value)}
                placeholder="organizer@college.edu"
              />
              {user ? (
                <Button
                  type="button"
                  variant="secondary"
                  className="shrink-0 px-3"
                  onClick={() => setContactEmail(user.email)}
                >
                  Use mine
                </Button>
              ) : null}
            </div>
          </Field>
        </div>
        {!isRemote ? (
          <div className="grid gap-4 sm:grid-cols-3">
            <Field label="Country" htmlFor="country" error={fields.country}>
              <Input
                id="country"
                value={country}
                onChange={(event) => setCountry(event.target.value)}
                placeholder="India"
                autoComplete="country-name"
              />
            </Field>
            <Field label="State or region" htmlFor="state" error={fields.state}>
              <Input
                id="state"
                value={state}
                onChange={(event) => setState(event.target.value)}
                placeholder="Telangana"
                autoComplete="address-level1"
              />
            </Field>
            <Field label="City" htmlFor="city" error={fields.city}>
              <Input
                id="city"
                value={city}
                onChange={(event) => setCity(event.target.value)}
                placeholder="Hyderabad"
                autoComplete="address-level2"
              />
            </Field>
          </div>
        ) : (
          <p className="text-sm text-muted">
            Remote events show as online and can be searched from anywhere.
          </p>
        )}
      </fieldset>

      <div className="pixel-divider" />

      <fieldset className="space-y-4">
        <legend className="mb-2 font-pixel text-lg">DATES</legend>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Registration opens" htmlFor="registration_opens_at" error={fields.registration_opens_at}>
            <Input
              id="registration_opens_at"
              type="datetime-local"
              value={registrationOpens}
              onChange={(event) => setRegistrationOpens(event.target.value)}
            />
          </Field>
          <Field label="Registration closes" htmlFor="registration_closes_at" error={fields.registration_closes_at}>
            <Input
              id="registration_closes_at"
              type="datetime-local"
              value={registrationCloses}
              onChange={(event) => setRegistrationCloses(event.target.value)}
            />
          </Field>
          <Field label="Event starts" htmlFor="starts_at" error={fields.starts_at}>
            <Input
              id="starts_at"
              type="datetime-local"
              value={startsAt}
              onChange={(event) => setStartsAt(event.target.value)}
            />
          </Field>
          <Field label="Event ends" htmlFor="ends_at" error={fields.ends_at}>
            <Input
              id="ends_at"
              type="datetime-local"
              value={endsAt}
              onChange={(event) => setEndsAt(event.target.value)}
            />
          </Field>
          <Field
            label="Submission deadline"
            htmlFor="submission_deadline"
            error={fields.submission_deadline}
            hint="Must be after registration closes."
          >
            <Input
              id="submission_deadline"
              type="datetime-local"
              value={deadline}
              onChange={(event) => setDeadline(event.target.value)}
            />
          </Field>
        </div>
      </fieldset>

      <div className="pixel-divider" />

      <fieldset className="space-y-4">
        <legend className="mb-2 font-pixel text-lg">TEAMS</legend>
        <Field
          label="Maximum team size"
          htmlFor="max_team_size"
          error={fields.max_team_size}
          hint="Teams can be any size up to this limit."
        >
          <Input
            id="max_team_size"
            type="number"
            min={1}
            max={64}
            value={format === "individual" ? "1" : maxTeamSize}
            disabled={format === "individual"}
            onChange={(event) => setMaxTeamSize(event.target.value)}
          />
        </Field>
        <label className="flex items-start gap-3 border-2 border-ink bg-bg/40 p-3">
          <Checkbox
            checked={mergeForms}
            onChange={(event) => setMergeForms(event.target.checked)}
          />
          <span>
            <span className="block text-base font-semibold">Merge registration and submission forms</span>
            <span className="block text-sm text-muted">
              Participants answer submission questions while they register, so there is no separate
              submission step.
            </span>
          </span>
        </label>
      </fieldset>

      <div className="pixel-divider" />

      <fieldset className="space-y-4">
        <legend className="mb-2 font-pixel text-lg">RULES &amp; ELIGIBILITY</legend>
        <Field label="Who can enter?" htmlFor="eligibility" error={fields.eligibility}>
          <Textarea
            id="eligibility"
            value={eligibility}
            onChange={(event) => setEligibility(event.target.value)}
            className="min-h-24"
            placeholder="Open to students and recent graduates."
          />
        </Field>
        <Field label="Rules" htmlFor="rules" error={fields.rules}>
          <Textarea
            id="rules"
            value={rules}
            onChange={(event) => setRules(event.target.value)}
            className="min-h-24"
            placeholder="Teams of up to four. Work must be started during the event."
          />
        </Field>
      </fieldset>

      <div className="pixel-divider" />

      <fieldset className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <legend className="font-pixel text-lg">JUDGING CRITERIA</legend>
          <Button
            type="button"
            size="sm"
            variant="secondary"
            disabled={criteria.length >= 8}
            onClick={() =>
              setCriteria((current) => [...current, { name: "", description: "" }])
            }
          >
            Add criterion
          </Button>
        </div>
        <p className="text-sm text-muted">
          Judges compare two entries at a time. Criteria explain what to look for; they are never scored
          numerically.
        </p>
        {fields.criteria ? <Alert tone="danger">{fields.criteria}</Alert> : null}
        <ul className="space-y-3">
          {criteria.map((criterion, index) => (
            <li key={index} className="border-2 border-ink bg-bg/50 p-3">
              <div className="flex items-start gap-2">
                <span className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center border-2 border-ink bg-accent-3 font-pixel text-sm">
                  {index + 1}
                </span>
                <div className="min-w-0 flex-1 space-y-2">
                  <Input
                    aria-label={`Criterion ${index + 1} name`}
                    value={criterion.name}
                    maxLength={60}
                    onChange={(event) => updateCriterion(index, { name: event.target.value })}
                    placeholder="Originality"
                  />
                  <Input
                    aria-label={`Criterion ${index + 1} description`}
                    value={criterion.description}
                    maxLength={300}
                    onChange={(event) => updateCriterion(index, { description: event.target.value })}
                    placeholder="Is the idea meaningfully new?"
                  />
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="destructive"
                  onClick={() => removeCriterion(index)}
                  aria-label={`Remove criterion ${index + 1}`}
                >
                  ✕
                </Button>
              </div>
            </li>
          ))}
        </ul>
        {criteria.length === 0 ? (
          <p className="border-2 border-dashed border-ink p-4 text-base text-muted">
            No criteria yet. Judges will still compare entries, but context helps.
          </p>
        ) : null}
      </fieldset>

      <div className="flex flex-wrap items-center justify-between gap-3 border-t-2 border-ink pt-5">
        <p className="font-mono text-sm text-muted">
          Saving as a draft keeps the event hidden until you publish.
        </p>
        <Button type="submit" size="lg" disabled={busy || title.trim().length < 3}>
          {busy ? "Saving…" : initial ? "Save changes" : "Create draft"}
        </Button>
      </div>
    </form>
  );
}
