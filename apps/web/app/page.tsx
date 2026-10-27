"use client";

import Link from "next/link";
import { useAuth } from "@/components/auth-provider";
import { CrtIllustration } from "@/components/illustrations";
import { Badge } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const steps = [
  {
    n: "01",
    title: "Host",
    body: "Publish an event, design the registration form, and invite judges by email.",
  },
  {
    n: "02",
    title: "Compare",
    body: "Judges see two submissions at a time and pick the stronger one. No scores, no rubrics.",
  },
  {
    n: "03",
    title: "Publish",
    body: "CrowdBT weighs every comparison — and every judge's reliability — into one ranking.",
  },
];

export default function HomePage() {
  const { user, loading } = useAuth();
  const needsOnboarding = user && !user.onboarding_completed;

  return (
    <>
      <section className="relative overflow-hidden border-b-2 border-ink">
        <div className="mx-auto grid max-w-content items-center gap-10 px-4 py-14 sm:px-6 lg:grid-cols-[1.15fr_0.85fr] lg:py-20">
          <div>
            <Badge tone="blue">PAIRWISE JUDGING</Badge>
            <h1 className="mt-5 font-pixel text-4xl leading-[1.25] sm:text-5xl lg:text-6xl">
              COMPYTE.
              <br />
              <span className="sticker mt-2 inline-block">COMPARE.</span>
              <br />
              DECIDE.
            </h1>
            <p className="mt-6 max-w-reading text-lg text-muted sm:text-xl">
              A calm home for hackathons, design jams and science fairs. Judges answer one question at a
              time — <strong className="text-ink">which of these two is stronger?</strong> — and the
              ranking falls out of the comparisons.
            </p>
            <div className="mt-8 flex flex-col gap-3 sm:flex-row">
              {loading ? null : needsOnboarding ? (
                <Link href="/onboarding" className={buttonVariants({ size: "lg" })}>
                  Finish setting up
                </Link>
              ) : user ? (
                <Link href="/dashboard" className={buttonVariants({ size: "lg" })}>
                  Go to dashboard
                </Link>
              ) : (
                <Link href="/register" className={buttonVariants({ size: "lg" })}>
                  Start free
                </Link>
              )}
              <Link href="/events" className={buttonVariants({ variant: "secondary", size: "lg" })}>
                Browse events
              </Link>
            </div>
            <p className="mt-4 font-mono text-sm text-muted">
              No ads. No entry fees. No scores out of ten.
            </p>
          </div>
          <div className="halftone relative border-2 border-ink bg-accent-3/50 p-5 shadow-hard">
            <CrtIllustration className="h-auto w-full" />
            <div className="mt-3 flex items-center justify-between border-t-2 border-ink pt-3 font-mono text-sm">
              <span>JUDGE 2 / 3</span>
              <span aria-hidden="true" className="animate-blink">
                ▌
              </span>
            </div>
          </div>
        </div>
      </section>

      <section className="mx-auto max-w-content px-4 py-14 sm:px-6">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h2 className="font-pixel text-2xl sm:text-3xl">HOW IT WORKS</h2>
            <p className="mt-2 max-w-reading text-lg text-muted">
              Three steps, one accent color, zero spreadsheets.
            </p>
          </div>
          <Link href="/events" className={buttonVariants({ variant: "secondary" })}>
            See open events
          </Link>
        </div>
        <div className="mt-8 grid gap-5 md:grid-cols-3">
          {steps.map((step) => (
            <article key={step.n} className="border-2 border-ink bg-surface p-5 shadow-hard">
              <p className="font-pixel text-3xl text-accent-2">{step.n}</p>
              <h3 className="mt-2 font-pixel text-xl">{step.title.toUpperCase()}</h3>
              <p className="mt-2 text-base text-muted">{step.body}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="border-y-2 border-ink bg-accent">
        <div className="halftone mx-auto flex max-w-content flex-col items-start gap-4 px-4 py-10 sm:flex-row sm:items-center sm:justify-between sm:px-6">
          <div>
            <h2 className="font-pixel text-2xl">JUDGING WITHOUT THE GUESSWORK</h2>
            <p className="mt-1 max-w-reading text-base">
              Pairwise choices are lower effort for judges and more reliable for organizers. Reliability is
              learned per judge, not assumed.
            </p>
          </div>
          <Link
            href="/host/events/new"
            className={cn(buttonVariants({ variant: "secondary", size: "lg" }), "shrink-0")}
          >
            Host an event
          </Link>
        </div>
      </section>
    </>
  );
}
