import * as React from "react";
import { cn } from "@/lib/utils";

export function Card({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("border-2 border-ink bg-surface shadow-hard", className)} {...props} />;
}

export function CardHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("space-y-1 border-b-2 border-ink p-5", className)} {...props} />;
}

export function CardTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
  return <h2 className={cn("font-pixel text-2xl", className)} {...props} />;
}

export function CardDescription({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn("text-base text-muted", className)} {...props} />;
}

export function CardContent({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-5", className)} {...props} />;
}

export function CardFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("flex flex-wrap items-center gap-3 border-t-2 border-ink p-5", className)} {...props} />;
}

/** Card with the retro window title bar and three square controls. */
export function WindowCard({
  title,
  className,
  children,
  action,
}: {
  title: string;
  className?: string;
  children: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className={cn("border-2 border-ink bg-surface shadow-hard", className)}>
      <div className="flex items-center justify-between gap-3 border-b-2 border-ink bg-accent-3 px-4 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <span aria-hidden="true" className="flex gap-1">
            <span className="block h-2.5 w-2.5 border border-ink bg-accent-2" />
            <span className="block h-2.5 w-2.5 border border-ink bg-accent" />
            <span className="block h-2.5 w-2.5 border border-ink bg-surface" />
          </span>
          <h2 className="truncate font-pixel text-base sm:text-lg">{title}</h2>
        </div>
        {action}
      </div>
      {children}
    </div>
  );
}
