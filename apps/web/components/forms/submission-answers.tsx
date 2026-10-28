import type { FormField } from "@/lib/types";

function displayValue(value: unknown) {
  if (Array.isArray(value)) return value.join(", ");
  if (value === null || value === undefined || value === "") return "";
  return String(value);
}

/**
 * Read-only custom submission answers. Labels come from the event's submission
 * form so organizers and judges see the same words participants saw.
 */
export function SubmissionAnswers({
  fields,
  answers,
  className,
}: {
  fields: FormField[];
  answers: Record<string, unknown> | undefined;
  className?: string;
}) {
  const rows = fields
    .map((field) => ({ field, value: displayValue(answers?.[field.id]) }))
    .filter((row) => row.value !== "");
  if (rows.length === 0) return null;
  return (
    <dl className={className}>
      {rows.map(({ field, value }) => (
        <div key={field.id} className="border-t-2 border-ink/20 py-2 first:border-t-0 first:pt-0">
          <dt className="font-mono text-xs uppercase tracking-wide text-muted">{field.label}</dt>
          <dd className="mt-0.5 whitespace-pre-wrap break-words text-base">{value}</dd>
        </div>
      ))}
    </dl>
  );
}
