"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import { Alert } from "@/components/ui/feedback";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import AuthShell from "@/app/(auth)/auth-shell";

export default function RegisterPage() {
  const router = useRouter();
  const { refresh, googleEnabled } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setFields({});
    if (password !== confirm) {
      setFields({ password: "Passwords do not match." });
      return;
    }
    setBusy(true);
    try {
      await api("/auth/register", { method: "POST", body: { email, password } });
      await refresh();
      const next = new URLSearchParams(window.location.search).get("next");
      router.push(
        next && next.startsWith("/") ? `/onboarding?next=${encodeURIComponent(next)}` : "/onboarding",
      );
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <AuthShell
      title="CREATE ACCOUNT"
      subtitle="One account for hosting, entering and judging. You can change everything later."
      footer={
        <>
          Already registered?{" "}
          <Link href="/login" className="font-semibold chunky-link">
            Sign in
          </Link>
        </>
      }
    >
      <form className="space-y-4" onSubmit={onSubmit} noValidate>
        {error ? <Alert tone="danger">{error}</Alert> : null}
        <Field label="Email" htmlFor="email" error={fields.email}>
          <Input
            id="email"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="you@college.edu"
          />
        </Field>
        <Field
          label="Password"
          htmlFor="password"
          error={fields.password}
          hint="At least 8 characters. A passphrase works well."
        >
          <Input
            id="password"
            type="password"
            autoComplete="new-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </Field>
        <Field label="Confirm password" htmlFor="confirm">
          <Input
            id="confirm"
            type="password"
            autoComplete="new-password"
            required
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
          />
        </Field>
        <Button type="submit" className="w-full" disabled={busy}>
          {busy ? "Creating…" : "Create account"}
        </Button>
        {googleEnabled ? (
          <a
            href="/api/v1/auth/google/start?redirect=/onboarding"
            className={`${buttonVariants({ variant: "secondary" })} w-full`}
          >
            Sign up with Google
          </a>
        ) : null}
        <p className="text-sm text-muted">
          By joining you agree to be a decent human in the chat.
        </p>
      </form>
    </AuthShell>
  );
}
