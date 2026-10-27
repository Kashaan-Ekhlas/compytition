import * as React from "react";
import { cn } from "@/lib/utils";

const tones = {
  info: "border-ink bg-accent-3",
  warning: "border-ink bg-accent",
  danger: "border-accent-2 bg-surface",
  success: "border-success bg-surface",
} as const;

export function Alert({
  tone = "info",
  title,
  children,
  className,
}: {
  tone?: keyof typeof tones;
  title?: string;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={cn("border-2 p-4 shadow-hard-sm", tones[tone], className)}
    >
      {title ? <p className="font-pixel text-base">{title}</p> : null}
      {children ? <div className="mt-1 text-base">{children}</div> : null}
    </div>
  );
}

export function Progress({
  value,
  max,
  label,
  className,
}: {
  value: number;
  max: number;
  label?: string;
  className?: string;
}) {
  const safeMax = Math.max(max, 1);
  const ratio = Math.min(Math.max(value / safeMax, 0), 1);
  const totalBlocks = 20;
  const filled = Math.round(ratio * totalBlocks);
  return (
    <div className={cn("space-y-1", className)}>
      <div
        className="flex h-5 gap-1"
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        aria-label={label ?? "Progress"}
      >
        {Array.from({ length: totalBlocks }).map((_, index) => (
          <span
            key={index}
            className={cn(
              "h-full flex-1 border-2 border-ink",
              index < filled ? "bg-accent" : "bg-surface",
            )}
          />
        ))}
      </div>
      {label ? (
        <p className="font-mono text-sm text-muted">
          {label} · {value} / {max}
        </p>
      ) : null}
    </div>
  );
}

export function Skeleton({ className }: { className?: string }) {
  return <div aria-hidden="true" className={cn("animate-pulse border-2 border-ink/30 bg-ink/10", className)} />;
}

export function LoadingBlock({ label = "Loading" }: { label?: string }) {
  return (
    <div className="flex items-center gap-3 py-10 font-mono text-base text-muted" role="status">
      <span className="inline-block h-4 w-3 animate-blink bg-ink" aria-hidden="true" />
      {label}…
    </div>
  );
}
