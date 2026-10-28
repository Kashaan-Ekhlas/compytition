"use client";

import { Suspense, useCallback, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import { safeNext } from "@/lib/utils";
import { useAuth } from "@/components/auth-provider";
import { ErrorToast } from "@/components/ui/toast";
import { PasswordChecklist, PasswordInput, passwordIsStrong } from "@/components/ui/password-input";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import AuthShell from "@/app/(auth)/auth-shell";

function AuthSwitchFooter() {
  const params = useSearchParams();
  const next = safeNext(params.get("next"), "");
  return (
    <>
      Already registered?{" "}
      <Link
        href={next ? `/login?next=${encodeURIComponent(next)}` : "/login"}
        className="font-semibold chunky-link"
      >
        Sign in
      </Link>
    </>
  );
}

export default function RegisterPage() {
  const router = useRouter();
  const { refresh, googleEnabled } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const clearError = useCallback(() => setError(null), []);
  const strong = passwordIsStrong(password);
  const matches = confirm.length > 0 && confirm === password;
  const canSubmit = !busy && strong && matches && email.trim().length > 0;

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setFields({});
    if (password !== confirm) {
      setFields({ password: "Passwords do not match." });
      return;
    }
    if (!passwordIsStrong(password)) {
      setFields({ password: "Meet every password requirement first." });
      return;
    }
    setBusy(true);
    try {
      await api("/auth/register", { method: "POST", body: { email, password } });
      await refresh();
      const next = safeNext(new URLSearchParams(window.location.search).get("next"), "");
      router.push(next ? `/onboarding?next=${encodeURIComponent(next)}` : "/onboarding");
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
        <Suspense fallback="Already registered?">
          <AuthSwitchFooter />
        </Suspense>
      }
    >
      <form className="space-y-4" onSubmit={onSubmit} noValidate>
        <ErrorToast message={error} onClose={clearError} />
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
          hint="Use a mix you can remember. The checklist below must be complete."
        >
          <PasswordInput
            id="password"
            autoComplete="new-password"
            required
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <PasswordChecklist value={password} />
        </Field>
        <Field
          label="Confirm password"
          htmlFor="confirm"
          error={confirm.length > 0 && !matches ? "Passwords do not match." : undefined}
        >
          <PasswordInput
            id="confirm"
            autoComplete="new-password"
            required
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
          />
        </Field>
        <Button type="submit" className="w-full" disabled={!canSubmit}>
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
