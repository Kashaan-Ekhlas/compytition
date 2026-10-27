"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { safeNext } from "@/lib/utils";
import { useAuth } from "@/components/auth-provider";
import { Alert } from "@/components/ui/feedback";
import { Button, buttonVariants } from "@/components/ui/button";
import { Field, Input } from "@/components/ui/input";
import AuthShell from "@/app/(auth)/auth-shell";

function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const { refresh, googleEnabled } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await api("/auth/login", { method: "POST", body: { email, password } });
      const me = await refresh();
      const next = safeNext(params.get("next"), "");
      if (me.user && !me.user.onboarding_completed) {
        router.push(next ? `/onboarding?next=${encodeURIComponent(next)}` : "/onboarding");
      } else {
        router.push(next || "/dashboard");
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="space-y-4" onSubmit={onSubmit} noValidate>
      {error ? <Alert tone="danger">{error}</Alert> : null}
      <Field label="Email" htmlFor="email">
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
      <Field label="Password" htmlFor="password">
        <Input
          id="password"
          type="password"
          autoComplete="current-password"
          required
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder="••••••••"
        />
      </Field>
      <Button type="submit" className="w-full" disabled={busy}>
        {busy ? "Signing in…" : "Sign in"}
      </Button>
      {googleEnabled ? (
        <a
          href="/api/v1/auth/google/start?redirect=/dashboard"
          className={`${buttonVariants({ variant: "secondary" })} w-full`}
        >
          Continue with Google
        </a>
      ) : null}
      <p className="text-sm text-muted">
        No account?{" "}
        <Link href="/register" className="chunky-link font-semibold">
          Create one
        </Link>
        .
      </p>
    </form>
  );
}

export default function LoginPage() {
  return (
    <AuthShell
      title="SIGN IN"
      subtitle="Pick up where you left off — registrations, teams and judging queues are waiting."
      footer={
        <>
          New here?{" "}
          <Link href="/register" className="font-semibold chunky-link">
            Create an account
          </Link>
        </>
      }
    >
      <Suspense fallback={<p className="font-mono text-base">Loading…</p>}>
        <LoginForm />
      </Suspense>
    </AuthShell>
  );
}
