export function CrtIllustration({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 320 260"
      role="img"
      aria-label="A beige retro computer monitor showing two cards being compared"
      className={className}
    >
      <rect x="18" y="12" width="284" height="192" rx="6" fill="#D9D2C2" stroke="var(--ink)" strokeWidth="4" />
      <rect x="40" y="32" width="240" height="150" fill="#C7CCF2" stroke="var(--ink)" strokeWidth="4" />
      <rect x="52" y="48" width="88" height="54" fill="#F4EFE3" stroke="var(--ink)" strokeWidth="3" />
      <rect x="60" y="58" width="60" height="6" fill="var(--ink)" />
      <rect x="60" y="70" width="44" height="4" fill="var(--ink)" opacity="0.55" />
      <rect x="60" y="80" width="52" height="4" fill="var(--ink)" opacity="0.55" />
      <rect x="180" y="48" width="88" height="54" fill="#FFD83D" stroke="var(--ink)" strokeWidth="3" />
      <rect x="188" y="58" width="60" height="6" fill="var(--ink)" />
      <rect x="188" y="70" width="44" height="4" fill="var(--ink)" opacity="0.55" />
      <rect x="188" y="80" width="52" height="4" fill="var(--ink)" opacity="0.55" />
      <rect x="132" y="66" width="24" height="18" fill="var(--ink)" />
      <text x="138" y="80" fontFamily="Silkscreen, monospace" fontSize="11" fill="#FFD83D">
        VS
      </text>
      <rect x="52" y="118" width="88" height="46" fill="#F4EFE3" stroke="var(--ink)" strokeWidth="3" />
      <rect x="60" y="128" width="52" height="6" fill="var(--ink)" />
      <rect x="60" y="140" width="40" height="4" fill="var(--ink)" opacity="0.55" />
      <rect x="180" y="118" width="88" height="46" fill="#F4EFE3" stroke="var(--ink)" strokeWidth="3" />
      <rect x="188" y="128" width="52" height="6" fill="var(--ink)" />
      <rect x="188" y="140" width="40" height="4" fill="var(--ink)" opacity="0.55" />
      <rect x="72" y="204" width="176" height="14" fill="#C9C1AF" stroke="var(--ink)" strokeWidth="4" />
      <rect x="96" y="218" width="128" height="14" fill="#B8AF9C" stroke="var(--ink)" strokeWidth="4" />
      <rect x="238" y="204" width="36" height="8" fill="var(--ink)" />
      <circle cx="58" cy="198" r="3" fill="var(--ink)" />
      <circle cx="68" cy="198" r="3" fill="var(--ink)" />
      <path d="M262 228 l10 18 5-8 9 3 -7-10 8-4z" fill="var(--ink)" />
    </svg>
  );
}

export function FloppyIllustration({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 120 120" role="img" aria-label="A retro floppy disk" className={className}>
      <rect x="12" y="12" width="96" height="96" rx="2" fill="#C7CCF2" stroke="var(--ink)" strokeWidth="4" />
      <rect x="34" y="16" width="52" height="34" fill="#14110F" />
      <rect x="42" y="22" width="30" height="8" fill="#F4EFE3" />
      <rect x="30" y="60" width="60" height="40" fill="#F4EFE3" stroke="var(--ink)" strokeWidth="3" />
      <rect x="38" y="68" width="44" height="4" fill="#14110F" />
      <rect x="38" y="78" width="34" height="4" fill="#14110F" opacity="0.5" />
      <rect x="38" y="88" width="40" height="4" fill="#14110F" opacity="0.5" />
    </svg>
  );
}

export function EmptyState({
  title,
  children,
  action,
}: {
  title: string;
  children?: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="halftone flex flex-col items-center justify-center gap-4 border-2 border-dashed border-ink px-6 py-12 text-center">
      <FloppyIllustration className="h-20 w-20" />
      <h3 className="font-pixel text-xl">{title}</h3>
      {children ? <div className="max-w-reading text-base text-muted">{children}</div> : null}
      {action}
    </div>
  );
}
