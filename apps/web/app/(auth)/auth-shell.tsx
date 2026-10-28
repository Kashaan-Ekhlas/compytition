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
    <div className="mx-auto max-w-content px-4 py-12 sm:px-6 lg:py-20">
      <div className="mx-auto w-full max-w-[560px]">
        <p className="mb-5 text-center text-lg text-muted">
          One account covers hosting, competing and judging.
        </p>
        <div className="border-2 border-ink bg-surface shadow-hard">
          <div className="border-b-2 border-ink bg-accent-3 px-6 py-4">
            <div className="flex items-center gap-2">
              <span aria-hidden="true" className="flex gap-1">
                <span className="block h-2.5 w-2.5 border border-ink bg-accent-2" />
                <span className="block h-2.5 w-2.5 border border-ink bg-accent" />
                <span className="block h-2.5 w-2.5 border border-ink bg-surface" />
              </span>
              <h1 className="font-pixel text-xl">{title}</h1>
            </div>
          </div>
          <div className="space-y-4 p-6">
            <p className="text-base text-muted">{subtitle}</p>
            {children}
          </div>
          {footer ? <div className="border-t-2 border-ink bg-bg/60 p-6 text-base">{footer}</div> : null}
        </div>
      </div>
    </div>
  );
}
