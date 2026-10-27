"use client";

import { useCallback, useEffect, useState } from "react";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import type { FormField } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Button } from "@/components/ui/button";
import { Checkbox, Field, Input, Select } from "@/components/ui/input";

interface DraftField {
  key: string;
  label: string;
  field_type: FormField["field_type"];
  required: boolean;
  options: string;
  help_text: string;
}

const fieldTypes: Array<{ value: FormField["field_type"]; label: string }> = [
  { value: "text", label: "Short text" },
  { value: "long_text", label: "Long text" },
  { value: "select", label: "Single choice" },
  { value: "multi_select", label: "Multiple choice" },
  { value: "number", label: "Number" },
  { value: "url", label: "URL" },
  { value: "file", label: "File link" },
];

function toDraft(field: FormField, index: number): DraftField {
  return {
    key: field.id || `field-${index}`,
    label: field.label,
    field_type: field.field_type,
    required: field.required,
    options: (field.options ?? []).join(", "),
    help_text: field.help_text ?? "",
  };
}

let counter = 0;

export function TabForm({ eventId }: { eventId: string }) {
  const [fields, setFields] = useState<DraftField[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [validation, setValidation] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api<{ fields: FormField[] }>(`/events/${eventId}/form`);
      setFields((data.fields ?? []).map(toDraft));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId]);

  useEffect(() => {
    void load();
  }, [load]);

  function addField() {
    counter += 1;
    setFields((current) => [
      ...current,
      {
        key: `new-${counter}`,
        label: "",
        field_type: "text",
        required: false,
        options: "",
        help_text: "",
      },
    ]);
  }

  function update(index: number, patch: Partial<DraftField>) {
    setFields((current) =>
      current.map((field, position) => (position === index ? { ...field, ...patch } : field)),
    );
  }

  function move(index: number, direction: -1 | 1) {
    setFields((current) => {
      const target = index + direction;
      if (target < 0 || target >= current.length) return current;
      const next = [...current];
      const value = next[index];
      const swap = next[target];
      if (!value || !swap) return current;
      next[index] = swap;
      next[target] = value;
      return next;
    });
  }

  async function save() {
    setBusy(true);
    setError(null);
    setNotice(null);
    setValidation({});
    try {
      const payload = {
        fields: fields.map((field) => ({
          label: field.label,
          field_type: field.field_type,
          required: field.required,
          options:
            field.field_type === "select" || field.field_type === "multi_select"
              ? field.options
                  .split(",")
                  .map((option) => option.trim())
                  .filter(Boolean)
              : [],
          help_text: field.help_text,
        })),
      };
      const data = await api<{ fields: FormField[] }>(`/events/${eventId}/form`, {
        method: "PUT",
        body: payload,
      });
      setFields((data.fields ?? []).map(toDraft));
      setNotice("Registration form saved.");
    } catch (err) {
      setError(errorMessage(err));
      setValidation(fieldErrors(err));
    } finally {
      setBusy(false);
    }
  }

  if (loading) return <LoadingBlock label="Loading form" />;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="font-pixel text-2xl">REGISTRATION FORM</h2>
          <p className="mt-1 max-w-reading text-base text-muted">
            These questions are added after the profile autofill. Profile fields (name, college, course,
            email) are always collected automatically.
          </p>
        </div>
        <Button variant="secondary" onClick={addField} disabled={fields.length >= 30}>
          Add field
        </Button>
      </div>

      {error ? <Alert tone="danger">{error}</Alert> : null}
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      {fields.length === 0 ? (
        <div className="border-2 border-dashed border-ink bg-surface px-5 py-10 text-center">
          <p className="font-pixel text-lg">NO EXTRA QUESTIONS</p>
          <p className="mt-2 text-base text-muted">
            Your profile is enough. Add a field only when the event needs it.
          </p>
          <Button className="mt-4" onClick={addField}>
            Add the first field
          </Button>
        </div>
      ) : (
        <ul className="space-y-4">
          {fields.map((field, index) => {
            const choice = field.field_type === "select" || field.field_type === "multi_select";
            return (
              <li key={field.key} className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
                <div className="flex items-start justify-between gap-2">
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center border-2 border-ink bg-accent font-pixel text-sm">
                    {index + 1}
                  </span>
                  <div className="flex flex-wrap gap-1">
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => move(index, -1)}
                      disabled={index === 0}
                      aria-label={`Move field ${index + 1} up`}
                    >
                      ↑
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => move(index, 1)}
                      disabled={index === fields.length - 1}
                      aria-label={`Move field ${index + 1} down`}
                    >
                      ↓
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() =>
                        setFields((current) => current.filter((_, position) => position !== index))
                      }
                      aria-label={`Delete field ${index + 1}`}
                    >
                      ✕
                    </Button>
                  </div>
                </div>

                <div className="mt-3 grid gap-4 sm:grid-cols-[1fr_200px]">
                  <Field
                    label="Question"
                    htmlFor={`label-${field.key}`}
                    error={validation[`form_fields[${index}]`]}
                  >
                    <Input
                      id={`label-${field.key}`}
                      value={field.label}
                      maxLength={120}
                      onChange={(event) => update(index, { label: event.target.value })}
                      placeholder="What are you hoping to build?"
                    />
                  </Field>
                  <Field label="Type" htmlFor={`type-${field.key}`}>
                    <Select
                      id={`type-${field.key}`}
                      value={field.field_type}
                      onChange={(event) =>
                        update(index, { field_type: event.target.value as DraftField["field_type"] })
                      }
                    >
                      {fieldTypes.map((type) => (
                        <option key={type.value} value={type.value}>
                          {type.label}
                        </option>
                      ))}
                    </Select>
                  </Field>
                </div>

                {choice ? (
                  <div className="mt-4">
                    <Field
                      label="Options"
                      htmlFor={`options-${field.key}`}
                      hint="Comma separated. At least two."
                    >
                      <Input
                        id={`options-${field.key}`}
                        value={field.options}
                        onChange={(event) => update(index, { options: event.target.value })}
                        placeholder="First hackathon, A few, Veteran"
                      />
                    </Field>
                  </div>
                ) : null}

                <div className="mt-4">
                  <Field label="Helper text" htmlFor={`help-${field.key}`} hint="Optional.">
                    <Input
                      id={`help-${field.key}`}
                      value={field.help_text}
                      onChange={(event) => update(index, { help_text: event.target.value })}
                      placeholder="A sentence or two is plenty."
                    />
                  </Field>
                </div>

                <label className="mt-3 flex items-center gap-2">
                  <Checkbox
                    checked={field.required}
                    onChange={(event) => update(index, { required: event.target.checked })}
                  />
                  <span className="text-base">Required</span>
                </label>
              </li>
            );
          })}
        </ul>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3 border-t-2 border-ink pt-5">
        <p className="font-mono text-sm text-muted">
          Saving replaces the whole form. Existing answers keep their keys only when labels stay stable.
        </p>
        <Button onClick={() => void save()} disabled={busy || fields.some((field) => !field.label.trim())}>
          {busy ? "Saving…" : "Save form"}
        </Button>
      </div>
    </div>
  );
}
