import * as React from "react";
import { cn } from "@/lib/utils";

const tones = {
  accent: "bg-accent text-ink",
  red: "bg-accent-2 text-white",
  blue: "bg-accent-3 text-ink",
  ink: "bg-ink text-surface",
  success: "bg-success text-white",
  quiet: "bg-surface text-ink",
} as const;

export function Badge({
  tone = "accent",
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement> & { tone?: keyof typeof tones }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 border-2 border-ink px-2 py-0.5 font-pixel text-sm uppercase leading-tight",
        tones[tone],
        className,
      )}
      {...props}
    />
  );
}

export function statusTone(status: string): keyof typeof tones {
  switch (status) {
    case "published":
    case "registered":
    case "live":
    case "submitted":
    case "accepted":
      return "success";
    case "draft":
    case "setup":
    case "waitlisted":
    case "invited":
      return "blue";
    case "closed":
    case "locked":
    case "pending":
      return "accent";
    case "results_published":
    case "judged":
      return "ink";
    case "cancelled":
    case "disqualified":
    case "rejected":
      return "red";
    default:
      return "quiet";
  }
}
