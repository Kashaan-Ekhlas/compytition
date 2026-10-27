"use client";

import * as React from "react";
import { cn } from "@/lib/utils";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type = "text", ...props }, ref) => (
    <input
      ref={ref}
      type={type}
      className={cn(
        "min-h-12 w-full rounded border-2 border-ink bg-surface px-3 py-2 font-sans text-lg text-ink placeholder:text-muted/70 focus-visible:outline-3 focus-visible:outline-offset-2 disabled:opacity-60",
        className,
      )}
      {...props}
    />
  ),
);
Input.displayName = "Input";

export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(({ className, ...props }, ref) => (
  <textarea
    ref={ref}
    className={cn(
      "min-h-28 w-full rounded border-2 border-ink bg-surface px-3 py-2 font-sans text-lg text-ink placeholder:text-muted/70 focus-visible:outline-3 focus-visible:outline-offset-2",
      className,
    )}
    {...props}
  />
));
Textarea.displayName = "Textarea";

export const Select = React.forwardRef<
  HTMLSelectElement,
  React.SelectHTMLAttributes<HTMLSelectElement>
>(({ className, children, ...props }, ref) => (
  <select
    ref={ref}
    className={cn(
      "min-h-12 w-full rounded border-2 border-ink bg-surface px-3 py-2 pr-10 font-sans text-lg text-ink focus-visible:outline-3 focus-visible:outline-offset-2",
      className,
    )}
    {...props}
  >
    {children}
  </select>
));
Select.displayName = "Select";

export function Label({ className, ...props }: React.LabelHTMLAttributes<HTMLLabelElement>) {
  return (
    <label
      className={cn("mb-1.5 block font-mono text-[15px] font-semibold uppercase tracking-wide", className)}
      {...props}
    />
  );
}

export function Field({
  label,
  htmlFor,
  error,
  hint,
  children,
  className,
}: {
  label: string;
  htmlFor?: string;
  error?: string;
  hint?: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("space-y-1", className)}>
      <Label htmlFor={htmlFor}>{label}</Label>
      {children}
      {hint && !error ? <p className="text-sm text-muted">{hint}</p> : null}
      {error ? (
        <p className="flex items-start gap-1 text-sm font-medium text-accent-2" role="alert">
          <span aria-hidden="true">▲</span>
          {error}
        </p>
      ) : null}
    </div>
  );
}

export const Checkbox = React.forwardRef<
  HTMLInputElement,
  React.InputHTMLAttributes<HTMLInputElement>
>(({ className, ...props }, ref) => (
  <input
    ref={ref}
    type="checkbox"
    className={cn("h-6 w-6 shrink-0 accent-[var(--accent)]", className)}
    {...props}
  />
));
Checkbox.displayName = "Checkbox";
