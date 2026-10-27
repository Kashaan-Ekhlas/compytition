import Link from "next/link";

export function SiteFooter() {
  return (
    <footer className="mt-20 border-t-2 border-ink">
      <div className="mx-auto flex max-w-content flex-col gap-4 px-4 py-8 sm:flex-row sm:items-center sm:justify-between sm:px-6">
        <div>
          <p className="font-pixel text-sm">COMPYTITION</p>
          <p className="mt-1 max-w-reading text-sm text-muted">
            Pairwise judging for events that care about good work, not good guesses.
          </p>
        </div>
        <nav aria-label="Footer" className="flex flex-wrap gap-4 font-mono text-sm">
          <Link className="chunky-link" href="/events">
            Events
          </Link>
          <Link className="chunky-link" href="/host/events/new">
            Host
          </Link>
          <Link className="chunky-link" href="/settings">
            Feedback
          </Link>
        </nav>
      </div>
    </footer>
  );
}
