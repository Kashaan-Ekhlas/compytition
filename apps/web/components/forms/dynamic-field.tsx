"use client";

import type { FormField } from "@/lib/types";
import { Checkbox, Field, Input, Label, Select, Textarea } from "@/components/ui/input";

export function DynamicField({
  field,
  value,
  error,
  onChange,
}: {
  field: FormField;
  value: unknown;
  error?: string;
  onChange: (value: unknown) => void;
}) {
  const id = field.id;
  switch (field.field_type) {
    case "long_text":
      return (
        <Field label={field.label} htmlFor={id} error={error} hint={field.help_text}>
          <Textarea
            id={id}
            value={String(value ?? "")}
            onChange={(e) => onChange(e.target.value)}
            required={field.required}
          />
        </Field>
      );
    case "select":
      return (
        <Field label={field.label} htmlFor={id} error={error} hint={field.help_text}>
          <Select
            id={id}
            value={String(value ?? "")}
            onChange={(e) => onChange(e.target.value)}
            required={field.required}
          >
            <option value="">Choose…</option>
            {field.options.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </Select>
        </Field>
      );
    case "multi_select": {
      const current = Array.isArray(value) ? (value as string[]) : [];
      return (
        <fieldset>
          <Label>{field.label}</Label>
          {field.help_text ? <p className="mb-2 text-sm text-muted">{field.help_text}</p> : null}
          <div className="space-y-2">
            {field.options.map((option) => (
              <label key={option} className="flex items-center gap-2 border-2 border-ink/30 p-2">
                <Checkbox
                  checked={current.includes(option)}
                  onChange={(e) =>
                    onChange(
                      e.target.checked ? [...current, option] : current.filter((item) => item !== option),
                    )
                  }
                />
                <span>{option}</span>
              </label>
            ))}
          </div>
          {error ? <p className="mt-1 text-sm text-accent-2">▲ {error}</p> : null}
        </fieldset>
      );
    }
    case "checkbox":
      return (
        <label className="flex items-start gap-3 border-2 border-ink/30 p-3">
          <Checkbox
            checked={Boolean(value)}
            onChange={(e) => onChange(e.target.checked)}
            required={field.required}
          />
          <span>
            <span className="block font-medium">{field.label}</span>
            {field.help_text ? <span className="block text-sm text-muted">{field.help_text}</span> : null}
          </span>
        </label>
      );
    case "number":
      return (
        <Field label={field.label} htmlFor={id} error={error} hint={field.help_text}>
          <Input
            id={id}
            type="number"
            value={String(value ?? "")}
            onChange={(e) => onChange(e.target.value)}
            required={field.required}
          />
        </Field>
      );
    case "url":
    case "file":
      return (
        <Field label={field.label} htmlFor={id} error={error} hint={field.help_text ?? "Paste a link."}>
          <Input
            id={id}
            type="url"
            value={String(value ?? "")}
            onChange={(e) => onChange(e.target.value)}
            placeholder="https://"
            required={field.required}
          />
        </Field>
      );
    default:
      return (
        <Field label={field.label} htmlFor={id} error={error} hint={field.help_text}>
          <Input
            id={id}
            value={String(value ?? "")}
            onChange={(e) => onChange(e.target.value)}
            required={field.required}
          />
        </Field>
      );
  }
}
