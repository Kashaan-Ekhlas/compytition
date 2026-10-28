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

const colleges = [
  "Indian Institute of Technology",
  "National Institute of Technology",
  "BITS Pilani",
  "Delhi University",
  "Anna University",
  "Vellore Institute of Technology",
  "University of Hyderabad",
  "Independent / Self-taught",
  "Working professional",
];

const courses = [
  "Computer Science",
  "Information Technology",
  "Electronics",
  "Design",
  "Business",
  "Mathematics",
  "Physics",
  "Self-taught builder",
];

const TOTAL_STEPS = 7;
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
  const [country, setCountry] = useState("");
  const [state, setState] = useState("");
  const [city, setCity] = useState("");
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
      setCountry(profile.country ?? "");
      setState(profile.state ?? "");
      setCity(profile.city ?? "");
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
          usernameState !== "taken" &&
          usernameState !== "checking"
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
        return true;
      case 7:
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
    setFields((current) => ({ ...current, username: "" }));
    try {
      const res = await api<{ available: boolean; reason?: string }>("/auth/username-available", {
        query: { username },
      });
      setUsernameState(res.available ? "ok" : "taken");
      if (!res.available && res.reason) setFields((current) => ({ ...current, username: res.reason! }));
    } catch {
      setUsernameState("idle");
    }
  }

  async function saveProfile() {
    await api("/profile", {
      method: "PUT",
      body: {
        full_name: fullName,
        college,
        course,
        country,
        state,
        city,
        year_of_study: year ? Number(year) : null,
      },
    });
  }

  async function next() {
    setError(null);
    setFields({});
    setBusy(true);
    try {
      if (step === 1) {
        await api("/auth/username", { method: "PATCH", body: { username } });
      } else if (step >= 2) {
        await saveProfile();
      }
      if (step < TOTAL_STEPS) {
        const nextStep = step + 1;
        await api("/auth/onboarding", {
          method: "PATCH",
          body: { step: nextStep, completed: false, intents: selected },
        });
        setStep(nextStep);
      } else {
        await api("/auth/onboarding", {
          method: "PATCH",
          body: { step: TOTAL_STEPS, completed: true, intents: selected },
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
    6: "Where are you based?",
    7: "What brings you here?",
  };

  const subtitles: Record<number, string> = {
    1: "This is how teammates and judges will find you. Letters, numbers and underscores.",
    2: "This autofills event registration forms later.",
    3: "Pick from the list or type your own. Students and working professionals are both welcome.",
    4: "Pick a suggestion or write your own course or role.",
    5: "Optional. Leave blank if it does not apply.",
    6: "Optional. Fill in as much as you like, country first.",
    7: "Pick at least one. We will tune your dashboard around it.",
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
        <p className="mt-2 text-base text-muted">{subtitles[step]}</p>

        {error ? (
          <Alert tone="danger" className="mt-5">
            {error}
          </Alert>
        ) : null}

        <div className="mt-6 space-y-5">
          {step === 1 ? (
            <>
              <Field
                label="Username"
                htmlFor="username"
                error={fields.username}
                hint={
                  usernameState === "checking"
                    ? "Checking…"
                    : usernameState === "ok"
                      ? "Available."
                      : usernameState === "taken"
                        ? "That username is taken."
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
              <Button type="button" variant="secondary" size="sm" onClick={() => void checkUsername()}>
                Check availability
              </Button>
            </>
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
            <Field
              label="College or company"
              htmlFor="college"
              error={fields.college}
              hint="Choose a suggestion or type your own."
            >
              <Input
                id="college"
                list="college-options"
                value={college}
                onChange={(event) => setCollege(event.target.value)}
                autoComplete="organization"
                autoFocus
              />
              <datalist id="college-options">
                {colleges.map((item) => (
                  <option key={item} value={item} />
                ))}
              </datalist>
            </Field>
          ) : null}

          {step === 4 ? (
            <Field
              label="Course or role"
              htmlFor="course"
              error={fields.course}
              hint="Choose a suggestion or type your own."
            >
              <Input
                id="course"
                list="course-options"
                value={course}
                onChange={(event) => setCourse(event.target.value)}
                placeholder="Computer Science"
                autoFocus
              />
              <datalist id="course-options">
                {courses.map((item) => (
                  <option key={item} value={item} />
                ))}
              </datalist>
            </Field>
          ) : null}

          {step === 5 ? (
            <Field label="Year of study" htmlFor="year" hint="Optional.">
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
            <div className="grid gap-5 sm:grid-cols-3">
              <Field label="Country" htmlFor="country" error={fields.location}>
                <Input
                  id="country"
                  value={country}
                  onChange={(event) => setCountry(event.target.value)}
                  autoComplete="country-name"
                  placeholder="India"
                  autoFocus
                />
              </Field>
              <Field label="State or region" htmlFor="state" hint={!country ? "Choose a country first." : undefined}>
                <Input
                  id="state"
                  value={state}
                  onChange={(event) => setState(event.target.value)}
                  autoComplete="address-level1"
                  placeholder="Telangana"
                  disabled={!country.trim()}
                />
              </Field>
              <Field label="City" htmlFor="city" hint={!state ? "Choose a state first." : undefined}>
                <Input
                  id="city"
                  value={city}
                  onChange={(event) => setCity(event.target.value)}
                  autoComplete="address-level2"
                  placeholder="Hyderabad"
                  disabled={!state.trim()}
                />
              </Field>
            </div>
          ) : null}

          {step === 7 ? (
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
