"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import { safeNext } from "@/lib/utils";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import { Alert, LoadingBlock, Progress } from "@/components/ui/feedback";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/input";

const intents = [
  { id: "find", label: "Find events to enter", detail: "Discover hackathons and jams." },
  { id: "host", label: "Host events", detail: "Run registrations, teams and judging." },
  { id: "judge", label: "Judge events", detail: "Compare submissions and shape results." },
];

const TOTAL_STEPS = 6;
const PLACEHOLDER_USERNAME = /^user_[a-f0-9]{6}$/;

export default function OnboardingPage() {
  const { user, profile, loading, refresh } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const router = useRouter();

  const [step, setStep] = useState(1);
  const [username, setUsername] = useState("");
  const [fullName, setFullName] = useState("");
  const [college, setCollege] = useState("");
  const [course, setCourse] = useState("");
  const [year, setYear] = useState("");
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [usernameState, setUsernameState] = useState<"idle" | "checking" | "ok" | "taken">("idle");

  useEffect(() => {
    if (!user) return;
    setUsername(user.username);
    setSelected(user.intents ?? []);
    if (profile) {
      setFullName(profile.full_name);
      setCollege(profile.college);
      setCourse(profile.course);
      setYear(profile.year_of_study ? String(profile.year_of_study) : "");
    }
    if (user.onboarding_completed) {
      const next = safeNext(new URLSearchParams(window.location.search).get("next"), "");
      router.replace(next || "/dashboard");
      return;
    }
    const placeholder = PLACEHOLDER_USERNAME.test(user.username);
    const resume = placeholder ? 1 : Math.min(Math.max(user.onboarding_step, 1), TOTAL_STEPS);
    setStep((current) => (current === 1 ? resume : current));
  }, [user, profile, router]);

  const canContinue = useMemo(() => {
    switch (step) {
      case 1:
        return (
          /^[a-zA-Z0-9_]{3,30}$/.test(username) &&
          !PLACEHOLDER_USERNAME.test(username) &&
          usernameState !== "taken"
        );
      case 2:
        return fullName.trim().length >= 2;
      case 3:
        return college.trim().length >= 2;
      case 4:
        return course.trim().length >= 1;
      case 5:
        return true;
      case 6:
        return selected.length > 0;
      default:
        return false;
    }
  }, [step, username, usernameState, fullName, college, course, selected]);

  async function checkUsername() {
    if (!/^[a-zA-Z0-9_]{3,30}$/.test(username)) {
      setUsernameState("idle");
      return;
    }
    setUsernameState("checking");
    try {
      const res = await api<{ available: boolean; reason?: string }>("/auth/username-available", {
        query: { username },
      });
      setUsernameState(res.available ? "ok" : "taken");
      if (!res.available && res.reason) setFields((f) => ({ ...f, username: res.reason! }));
    } catch {
      setUsernameState("idle");
    }
  }

  async function saveProfile(next: { full_name: string; college: string; course: string; year_of_study: number | null }) {
    await api("/profile", { method: "PUT", body: next });
  }

  async function next() {
    setError(null);
    setFields({});
    setBusy(true);
    try {
      if (step === 1) {
        await api("/auth/username", { method: "PATCH", body: { username } });
        await api("/auth/onboarding", { method: "PATCH", body: { step: 2, completed: false, intents: selected } });
        setStep(2);
      } else if (step === 2) {
        await saveProfile({ full_name: fullName, college, course, year_of_study: year ? Number(year) : null });
        await api("/auth/onboarding", { method: "PATCH", body: { step: 3, completed: false, intents: selected } });
        setStep(3);
      } else if (step === 3) {
        await saveProfile({ full_name: fullName, college, course, year_of_study: year ? Number(year) : null });
        await api("/auth/onboarding", { method: "PATCH", body: { step: 4, completed: false, intents: selected } });
        setStep(4);
      } else if (step === 4) {
        await saveProfile({ full_name: fullName, college, course, year_of_study: year ? Number(year) : null });
        await api("/auth/onboarding", { method: "PATCH", body: { step: 5, completed: false, intents: selected } });
        setStep(5);
      } else if (step === 5) {
        await saveProfile({ full_name: fullName, college, course, year_of_study: year ? Number(year) : null });
        await api("/auth/onboarding", { method: "PATCH", body: { step: 6, completed: false, intents: selected } });
        setStep(6);
      } else {
        await saveProfile({ full_name: fullName, college, course, year_of_study: year ? Number(year) : null });
        await api("/auth/onboarding", {
          method: "PATCH",
          body: { step: 6, completed: true, intents: selected },
        });
        await refresh();
        const next = safeNext(new URLSearchParams(window.location.search).get("next"), "");
        router.push(next || "/dashboard");
      }
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(false);
    }
  }

  function back() {
    setError(null);
    setFields({});
    setStep((current) => Math.max(1, current - 1));
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-reading px-4 py-16">
        <LoadingBlock label="Loading your setup" />
      </div>
    );
  }

  const titles: Record<number, string> = {
    1: "Pick a username",
    2: "What should we call you?",
    3: "Where do you study or work?",
    4: "What are you studying?",
    5: "Which year are you in?",
    6: "What brings you here?",
  };

  return (
    <div className="mx-auto max-w-reading px-4 py-10 sm:px-6 sm:py-14">
      <div className="flex items-center justify-between gap-3">
        <p className="font-pixel text-sm text-muted">
          SETUP · {step} / {TOTAL_STEPS}
        </p>
        <p className="font-mono text-sm text-muted">About two minutes</p>
      </div>
      <Progress value={step} max={TOTAL_STEPS} label="Onboarding" className="mt-2" />

      <section className="mt-8 border-2 border-ink bg-surface p-6 shadow-hard sm:p-8">
        <h1 className="font-pixel text-2xl sm:text-3xl">{titles[step]}</h1>
        <p className="mt-2 text-base text-muted">
          {step === 1
            ? "This is how teammates and judges will find you. Letters, numbers and underscores."
            : step === 6
              ? "Pick at least one. We will tune your dashboard around it."
              : "This autofills event registration forms later."}
        </p>

        {error ? (
          <Alert tone="danger" className="mt-5">
            {error}
          </Alert>
        ) : null}

        <div className="mt-6 space-y-5">
          {step === 1 ? (
            <Field
              label="Username"
              htmlFor="username"
              error={fields.username}
              hint={
                usernameState === "checking"
                  ? "Checking…"
                  : usernameState === "ok"
                    ? "Available."
                    : "3–30 characters. Letters, numbers, underscore."
              }
            >
              <Input
                id="username"
                value={username}
                onChange={(event) => {
                  setUsername(event.target.value.toLowerCase());
                  setUsernameState("idle");
                }}
                onBlur={() => void checkUsername()}
                autoComplete="username"
                autoFocus
              />
            </Field>
          ) : null}

          {step === 2 ? (
            <Field label="Full name" htmlFor="full-name" error={fields.full_name}>
              <Input
                id="full-name"
                value={fullName}
                onChange={(event) => setFullName(event.target.value)}
                autoComplete="name"
                autoFocus
              />
            </Field>
          ) : null}

          {step === 3 ? (
            <Field label="College or company" htmlFor="college" error={fields.college}>
              <Input
                id="college"
                value={college}
                onChange={(event) => setCollege(event.target.value)}
                autoComplete="organization"
                autoFocus
              />
            </Field>
          ) : null}

          {step === 4 ? (
            <Field label="Course or role" htmlFor="course" error={fields.course}>
              <Input
                id="course"
                value={course}
                onChange={(event) => setCourse(event.target.value)}
                placeholder="Computer Science"
                autoFocus
              />
            </Field>
          ) : null}

          {step === 5 ? (
            <Field label="Year of study" htmlFor="year" hint="Optional. Leave blank if it does not apply.">
              <Select id="year" value={year} onChange={(event) => setYear(event.target.value)}>
                <option value="">Not applicable</option>
                {[1, 2, 3, 4, 5, 6, 7, 8].map((n) => (
                  <option key={n} value={n}>
                    Year {n}
                  </option>
                ))}
              </Select>
            </Field>
          ) : null}

          {step === 6 ? (
            <fieldset className="space-y-3">
              <legend className="sr-only">Intent</legend>
              {intents.map((intent) => {
                const checked = selected.includes(intent.id);
                return (
                  <label
                    key={intent.id}
                    className={`flex cursor-pointer items-start gap-3 border-2 border-ink p-3 transition-colors ${
                      checked ? "bg-accent/40" : "bg-surface hover:bg-accent/10"
                    }`}
                  >
                    <input
                      type="checkbox"
                      className="mt-1 h-6 w-6 accent-[var(--accent)]"
                      checked={checked}
                      onChange={(event) =>
                        setSelected((current) =>
                          event.target.checked
                            ? [...current, intent.id]
                            : current.filter((id) => id !== intent.id),
                        )
                      }
                    />
                    <span>
                      <span className="block font-semibold">{intent.label}</span>
                      <span className="block text-sm text-muted">{intent.detail}</span>
                    </span>
                  </label>
                );
              })}
            </fieldset>
          ) : null}
        </div>

        <div className="mt-8 flex items-center justify-between gap-3">
          <Button variant="secondary" onClick={back} disabled={step === 1 || busy}>
            Back
          </Button>
          <Button onClick={() => void next()} disabled={!canContinue || busy}>
            {busy ? "Saving…" : step === TOTAL_STEPS ? "Finish setup" : "Continue"}
          </Button>
        </div>
      </section>
    </div>
  );
}
