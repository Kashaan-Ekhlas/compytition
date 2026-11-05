import { cn, initials } from "@/lib/utils";

const sizes = {
  sm: "h-8 w-8 text-xs",
  md: "h-12 w-12 text-sm",
  lg: "h-20 w-20 text-xl",
} as const;

export function Avatar({
  src,
  name,
  size = "md",
  className,
}: {
  src?: string | null;
  name: string;
  size?: keyof typeof sizes;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden border-2 border-ink bg-accent-3 font-pixel",
        sizes[size],
        className,
      )}
    >
      {src ? (
        // Uploaded avatars are served from our own origin; next/image would
        // need a loader for the API rewrite, so a plain img is intentional.
        // eslint-disable-next-line @next/next/no-img-element
        <img src={src} alt="" className="h-full w-full object-cover" />
      ) : (
        <span aria-hidden="true">{initials(name) || "?"}</span>
      )}
    </span>
  );
}
