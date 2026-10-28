"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams, usePathname } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import type { Team, TeamInviteLink } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";

export default function AcceptTeamInvitePage() {
  const params = useParams<{ token: string }>();
  const pathname = usePathname();
  const { user, loading } = useAuth();
  const [preview, setPreview] = useState<TeamInviteLink | null>(null);
  const [team, setTeam] = useState<Team | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    api<{ invite_link: TeamInviteLink }>("/team-invites/preview", {
      query: { token: params.token },
    })
      .then((data) => {
        if (active) setPreview(data.invite_link);
      })
      .catch((err) => {
        if (active) setError(errorMessage(err));
      });
    return () => {
      active = false;
    };
  }, [params.token]);

  const accept = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const data = await api<{ team: Team }>("/team-invites/accept-link", {
        method: "POST",
        body: { token: params.token },
      });
      setTeam(data.team);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }, [params.token]);

  // Existing accounts join as soon as the page loads. New accounts come back
  // here after registration through the `next` link, so the same effect runs.
  useEffect(() => {
    if (user && preview && !team && !busy && !error) {
      void accept();
    }
  }, [user, preview, team, busy, error, accept]);

  if (loading || (!preview && !error)) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Checking the invite" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-reading px-4 py-12 sm:px-6">
      <WindowCard title="TEAM INVITE">
        <div className="space-y-4 p-5">
          {preview ? (
            <>
              <p className="font-mono text-sm text-muted">{preview.event_title}</p>
              <h1 className="font-pixel text-2xl">{preview.team_name}</h1>
            </>
          ) : null}

          {!user ? (
            <>
              <p className="text-base">
                Sign in to join this team. If you are new, create an account and you will come straight back
                here.
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
            </>
          ) : error ? (
            <>
              <Alert tone="danger">{error}</Alert>
              <Button onClick={() => void accept()} disabled={busy}>
                {busy ? "Trying again…" : "Try again"}
              </Button>
            </>
          ) : team ? (
            <>
              <Alert tone="success">You are on {team.name}.</Alert>
              <div className="flex flex-wrap gap-2">
                <Link
                  href={`/events/${team.event_id}/team`}
                  className={buttonVariants({ variant: "primary" })}
                >
                  Open team
                </Link>
                <Link
                  href={`/events/${team.event_id}`}
                  className={buttonVariants({ variant: "secondary" })}
                >
                  View event
                </Link>
              </div>
            </>
          ) : (
            <p className="text-base text-muted">Joining…</p>
          )}
        </div>
      </WindowCard>
    </div>
  );
}
