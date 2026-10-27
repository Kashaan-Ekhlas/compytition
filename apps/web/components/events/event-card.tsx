import Link from "next/link";
import { Badge, statusTone } from "@/components/ui/badge";
import { buttonVariants } from "@/components/ui/button";
import { formatDate, formatRange } from "@/lib/utils";
import type { EventView } from "@/lib/types";

export function EventCard({ event }: { event: EventView }) {
  const format =
    event.participation_format === "team"
      ? "Team"
      : event.participation_format === "individual"
        ? "Solo"
        : "Solo or team";
  return (
    <article className="flex h-full flex-col border-2 border-ink bg-surface shadow-hard transition-transform hover:-translate-y-0.5">
      <div className="flex items-center justify-between gap-2 border-b-2 border-ink bg-accent-3 px-3 py-1.5">
        <span className="truncate font-pixel text-sm">{event.category.toUpperCase()}</span>
        <Badge tone={statusTone(event.status)}>{event.status.replace("_", " ")}</Badge>
      </div>
      <div className="flex flex-1 flex-col p-4">
        <h3 className="font-pixel text-xl leading-snug">{event.title}</h3>
        <p className="mt-1 font-mono text-sm text-muted">
          {event.organizer_username ? `@${event.organizer_username}` : "Community event"}
          {event.location ? ` · ${event.location}` : ""}
        </p>
        <p className="mt-3 line-clamp-3 text-base text-muted">{event.description || "Details coming soon."}</p>
        <dl className="mt-4 grid grid-cols-2 gap-2 border-t-2 border-ink/20 pt-3 font-mono text-sm">
          <div>
            <dt className="text-muted">When</dt>
            <dd>{formatRange(event.starts_at, event.ends_at)}</dd>
          </div>
          <div>
            <dt className="text-muted">Register by</dt>
            <dd>{formatDate(event.registration_closes_at)}</dd>
          </div>
        </dl>
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <Badge tone="quiet">{format}</Badge>
          {event.registration_count > 0 ? (
            <span className="font-mono text-sm text-muted">{event.registration_count} registered</span>
          ) : null}
        </div>
        <div className="mt-4 flex-1" />
        <Link
          href={`/events/${event.id}`}
          className={`${buttonVariants({ variant: "primary", size: "sm" })} w-full`}
          aria-label={`View ${event.title}`}
        >
          View event
        </Link>
      </div>
    </article>
  );
}
