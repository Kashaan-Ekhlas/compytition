"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";

export default function AcceptStaffInvitePage() {
  const params = useParams<{ token: string }>();
  const pathname = usePathname();
  const { user, loading } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<{ event_id: string; role: string } | null>(null);

  useEffect(() => {
    setError(null);
  }, [params.token]);

  async function accept() {
    setBusy(true);
    setError(null);
    try {
      const data = await api<{ event_id: string; role: string }>(
        `/invites/staff/${params.token}/accept`,
        { method: "POST" },
      );
      setResult(data);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Checking your session" />
      </div>
    );
  }

  if (!user) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12 sm:px-6">
        <WindowCard title="STAFF INVITE">
          <div className="space-y-4 p-5">
            <p className="text-base">
              You have been invited to help run an event. Sign in with the email address that received the
              invite, then open this link again.
            </p>
            <div className="flex flex-wrap gap-2">
              <Link
                href={`/login?next=${encodeURIComponent(pathname)}`}
                className={buttonVariants({ variant: "primary" })}
              >
                Sign in
              </Link>
              <Link
                href={`/register?next=${encodeURIComponent(pathname)}`}
                className={buttonVariants({ variant: "secondary" })}
              >
                Create account
              </Link>
            </div>
            <p className="font-mono text-sm text-muted">
              Invites are tied to the email they were sent to.
            </p>
          </div>
        </WindowCard>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-reading px-4 py-12 sm:px-6">
      <WindowCard title="STAFF INVITE">
        <div className="space-y-4 p-5">
          {result ? (
            <>
              <Badge tone="success">{result.role.replace("_", " ").toUpperCase()}</Badge>
              <h1 className="font-pixel text-2xl">YOU ARE IN</h1>
              <p className="text-base text-muted">
                The invite has been accepted for <span className="font-mono">{user.email}</span>.
              </p>
              <div className="flex flex-wrap gap-2">
                {result.role === "judge" ? (
                  <Link
                    href={`/judge/events/${result.event_id}`}
                    className={buttonVariants({ variant: "primary" })}
                  >
                    Open judging
                  </Link>
                ) : (
                  <Link
                    href={`/host/events/${result.event_id}`}
                    className={buttonVariants({ variant: "primary" })}
                  >
                    Open event manager
                  </Link>
                )}
                <Link
                  href={`/events/${result.event_id}`}
                  className={buttonVariants({ variant: "secondary" })}
                >
                  View event
                </Link>
              </div>
            </>
          ) : (
            <>
              <h1 className="font-pixel text-2xl">ACCEPT INVITE</h1>
              <p className="text-base text-muted">
                Signed in as <span className="font-mono">{user.email}</span>. Accepting adds this event to
                your account. The invite must have been sent to this email address.
              </p>
              {error ? <Alert tone="danger">{error}</Alert> : null}
              <div className="flex flex-wrap gap-2">
                <Button onClick={() => void accept()} disabled={busy}>
                  {busy ? "Accepting…" : "Accept invitation"}
                </Button>
                <Link href="/dashboard" className={buttonVariants({ variant: "ghost" })}>
                  Not now
                </Link>
              </div>
            </>
          )}
        </div>
      </WindowCard>
    </div>
  );
}
