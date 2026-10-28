"use client";

import * as React from "react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

export const PasswordInput = React.forwardRef<
  HTMLInputElement,
  Omit<React.InputHTMLAttributes<HTMLInputElement>, "type">
>(({ className, ...props }, ref) => {
  const [visible, setVisible] = React.useState(false);
  return (
    <div className="relative">
      <Input
        ref={ref}
        type={visible ? "text" : "password"}
        className={cn("pr-14", className)}
        {...props}
      />
      <button
        type="button"
        onClick={() => setVisible((value) => !value)}
        aria-label={visible ? "Hide password" : "Show password"}
        aria-pressed={visible}
        className="absolute right-1.5 top-1/2 flex h-9 w-9 -translate-y-1/2 items-center justify-center border-2 border-transparent text-ink hover:border-ink hover:bg-accent/30"
      >
        {visible ? <EyeOffIcon /> : <EyeIcon />}
      </button>
    </div>
  );
});
PasswordInput.displayName = "PasswordInput";

function EyeIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M2 12s3.5-6 10-6 10 6 10 6-3.5 6-10 6S2 12 2 12Z" />
      <circle cx="12" cy="12" r="2.75" />
    </svg>
  );
}

function EyeOffIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M3 3l18 18" />
      <path d="M10.6 6.3A11 11 0 0 1 12 6c6.5 0 10 6 10 6a18 18 0 0 1-2.4 3.2" />
      <path d="M6.6 6.7C3.6 8.6 2 12 2 12s3.5 6 10 6a10 10 0 0 0 4.1-.9" />
    </svg>
  );
}

export const passwordRules = [
  { id: "length", label: "8 or more characters", test: (value: string) => value.length >= 8 },
  { id: "upper", label: "One uppercase letter", test: (value: string) => /[A-Z]/.test(value) },
  { id: "lower", label: "One lowercase letter", test: (value: string) => /[a-z]/.test(value) },
  { id: "number", label: "One number", test: (value: string) => /\d/.test(value) },
  {
    id: "special",
    label: "One special symbol",
    test: (value: string) => /[^A-Za-z0-9]/.test(value),
  },
] as const;

export function passwordIsStrong(value: string): boolean {
  return passwordRules.every((rule) => rule.test(value));
}

export function PasswordChecklist({ value }: { value: string }) {
  return (
    <ul className="mt-2 grid gap-1.5 sm:grid-cols-2" aria-label="Password requirements">
      {passwordRules.map((rule) => {
        const passed = rule.test(value);
        return (
          <li key={rule.id} className="flex items-center gap-2 text-sm">
            <span
              aria-hidden="true"
              className={cn(
                "flex h-5 w-5 shrink-0 items-center justify-center rounded-full border-2 border-ink text-xs font-bold",
                passed ? "bg-accent text-ink" : "bg-surface text-transparent",
              )}
            >
              ✓
            </span>
            <span className={passed ? "text-ink" : "text-muted"}>{rule.label}</span>
          </li>
        );
      })}
    </ul>
  );
}
