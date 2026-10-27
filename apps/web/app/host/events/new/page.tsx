"use client";

import { useRouter } from "next/navigation";
import Link from "next/link";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import { EventForm } from "@/components/host/event-form";
import { LoadingBlock } from "@/components/ui/feedback";
import type { EventView } from "@/lib/types";

export default function NewEventPage() {
  const router = useRouter();
  const { user } = useAuth();
  const { loading } = useRequireAuth();

  if (loading || !user) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Loading" />
      </div>
    );
  }

  function onSaved(event: EventView) {
    router.push(`/host/events/${event.id}`);
  }

  return (
    <div className="mx-auto max-w-reading px-4 py-8 sm:px-6 sm:py-12">
      <Link href="/host" className="chunky-link font-mono text-sm">
        ← Organizer home
      </Link>
      <header className="mt-4">
        <p className="font-pixel text-sm text-muted">NEW EVENT</p>
        <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">START WITH THE SHAPE</h1>
        <p className="mt-2 text-base text-muted">
          Everything here can be edited later. The event stays a private draft until you publish it.
        </p>
      </header>
      <div className="mt-8 border-2 border-ink bg-surface p-5 shadow-hard sm:p-7">
        <EventForm onSaved={onSaved} />
      </div>
    </div>
  );
}
