"use client";

import { useEffect } from "react";
import { cn } from "@/lib/utils";

/**
 * A short-lived popup for authentication and form failures. It sits above the
 * page chrome, stays visible for a few seconds, and can be dismissed with the
 * close button.
 */
export function ErrorToast({
  message,
  onClose,
  className,
  duration = 6000,
}: {
  message: string | null;
  onClose: () => void;
  className?: string;
  duration?: number;
}) {
  useEffect(() => {
    if (!message) return;
    const timer = window.setTimeout(onClose, duration);
    return () => window.clearTimeout(timer);
  }, [message, duration, onClose]);

  if (!message) return null;

  return (
    <div
      role="alert"
      aria-live="assertive"
      className={cn(
        "fixed right-4 top-24 z-[60] w-[min(24rem,calc(100vw-2rem))] border-2 border-ink bg-accent-2 p-4 text-white shadow-hard",
        className,
      )}
    >
      <div className="flex items-start gap-3">
        <span aria-hidden="true" className="mt-0.5 text-xl font-bold">
          !
        </span>
        <p className="flex-1 text-base font-semibold">{message}</p>
        <button
          type="button"
          onClick={onClose}
          aria-label="Dismiss error"
          className="-mr-1 -mt-1 flex h-9 w-9 shrink-0 items-center justify-center border-2 border-white/70 text-lg font-bold hover:bg-white/15"
        >
          ×
        </button>
      </div>
    </div>
  );
}
