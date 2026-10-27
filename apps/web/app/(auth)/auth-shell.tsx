import Link from "next/link";

export default function AuthShell({
  title,
  subtitle,
  children,
  footer,
}: {
  title: string;
  subtitle: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
}) {
  return (
    <div className="mx-auto grid max-w-content gap-8 px-4 py-12 sm:px-6 lg:grid-cols-[1fr_420px] lg:py-20">
      <div className="hidden lg:block">
        <p className="font-pixel text-4xl leading-[1.3]">
          SMALL
          <br />
          COMPARISONS.
          <br />
          <span className="sticker mt-2 inline-block">BIG</span>
          <br />
          SIGNAL.
        </p>
        <p className="mt-6 max-w-reading text-lg text-muted">
          One account covers hosting, competing and judging. Your profile autofills registration forms, so
          you never type your college name twice.
        </p>
        <div className="pixel-divider mt-10 max-w-sm" />
      </div>
      <div className="border-2 border-ink bg-surface shadow-hard">
        <div className="border-b-2 border-ink bg-accent-3 px-5 py-3">
          <div className="flex items-center gap-2">
            <span aria-hidden="true" className="flex gap-1">
              <span className="block h-2.5 w-2.5 border border-ink bg-accent-2" />
              <span className="block h-2.5 w-2.5 border border-ink bg-accent" />
              <span className="block h-2.5 w-2.5 border border-ink bg-surface" />
            </span>
            <h1 className="font-pixel text-lg">{title}</h1>
          </div>
        </div>
        <div className="space-y-4 p-5">
          <p className="text-base text-muted">{subtitle}</p>
          {children}
        </div>
        {footer ? <div className="border-t-2 border-ink bg-bg/60 p-5 text-base">{footer}</div> : null}
      </div>
      <div className="text-center lg:hidden">
        <Link href="/events" className="chunky-link font-mono text-sm">
          Browse events without an account →
        </Link>
      </div>
    </div>
  );
}
