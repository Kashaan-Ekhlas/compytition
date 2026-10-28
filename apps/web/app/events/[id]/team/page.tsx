"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { api, errorMessage } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { EventView, JoinRequest, Team, TeamInvite, TeamInviteLink } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge } from "@/components/ui/badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { Checkbox, Field, Input } from "@/components/ui/input";
import { TeamChat } from "@/components/team/team-chat";
import { EmptyState } from "@/components/illustrations";
import { formatDate } from "@/lib/utils";

export default function TeamPage() {
  const params = useParams<{ id: string }>();
  const eventId = params.id;
  const { user } = useAuth();
  const { loading: authLoading } = useRequireAuth();
  const [event, setEvent] = useState<EventView | null>(null);
  const [team, setTeam] = useState<Team | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [invites, setInvites] = useState<TeamInvite[]>([]);
  const [requests, setRequests] = useState<JoinRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [newName, setNewName] = useState("");
  const [inviteUsername, setInviteUsername] = useState("");
  const [inviteLink, setInviteLink] = useState<TeamInviteLink | null>(null);
  const [joinMessage, setJoinMessage] = useState("");
  const [editName, setEditName] = useState("");
  const [looking, setLooking] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [eventData, mineData, teamsData, invitesData, requestsData] = await Promise.all([
        api<{ event: EventView }>(`/events/${eventId}`),
        api<{ team: Team | null }>(`/events/${eventId}/teams/mine`),
        api<{ teams: Team[] }>(`/events/${eventId}/teams`),
        api<{ invites: TeamInvite[] }>("/team-invites"),
        api<{ requests: JoinRequest[] }>("/team-join-requests"),
      ]);
      setEvent(eventData.event);
      setTeams(teamsData.teams ?? []);
      setInvites((invitesData.invites ?? []).filter((invite) => invite.event_id === eventId && invite.status === "pending"));
      setRequests((requestsData.requests ?? []).filter((request) => request.event_id === eventId && request.status === "pending"));
      if (mineData.team) {
        const detail = await api<{ team: Team }>(`/teams/${mineData.team.id}`);
        setTeam(detail.team);
        setEditName(detail.team.name);
        setLooking(detail.team.is_looking_for_members);
      } else {
        setTeam(null);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [eventId]);

  useEffect(() => {
    if (user) void load();
  }, [user, load]);

  async function act(label: string, fn: () => Promise<unknown>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
      setNotice(label);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Loading team" />
      </div>
    );
  }

  if (!event) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <Alert tone="danger">{error ?? "Event not found."}</Alert>
      </div>
    );
  }

  if (event.participation_format === "individual") {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <Alert tone="warning" title="This event is individual-only">
          No team needed - head back and register as yourself.
        </Alert>
        <Link href={`/events/${eventId}`} className={`${buttonVariants({ variant: "secondary" })} mt-4`}>
          Back to event
        </Link>
      </div>
    );
  }

  const full = team ? team.member_count >= (team.max_team_size ?? event.max_team_size) : false;

  return (
    <div className="mx-auto max-w-content px-4 py-10 sm:px-6">
      <Link href={`/events/${eventId}`} className="chunky-link font-mono text-sm">
        ← {event.title}
      </Link>
      <header className="mt-4 flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="font-pixel text-sm text-muted">TEAMS</p>
          <h1 className="mt-1 font-pixel text-3xl">BUILD YOUR CREW</h1>
          <p className="mt-2 text-base text-muted">
            Teams can have up to {event.max_team_size} members. Registering a team registers every current
            member.
          </p>
        </div>
        <Badge tone="blue">{teams.length} TEAMS</Badge>
      </header>

      {error ? <Alert tone="danger" className="mt-5">{error}</Alert> : null}
      {notice ? <Alert tone="success" className="mt-5">{notice}</Alert> : null}

      {team ? (
        <div className="mt-6 grid gap-6 lg:grid-cols-[1.2fr_0.8fr]">
          <div className="space-y-6">
            <WindowCard
              title={team.name.toUpperCase()}
              action={<Badge tone={full ? "red" : "success"}>{full ? "FULL" : `${team.member_count}/${team.max_team_size ?? event.max_team_size}`}</Badge>}
            >
              <div className="p-5">
                <ul className="space-y-2">
                  {team.members?.map((member) => (
                    <li key={member.user_id} className="flex items-center justify-between gap-3 border-b-2 border-ink/15 pb-2">
                      <span>
                        <span className="block font-semibold">{member.full_name || member.username}</span>
                        <span className="font-mono text-sm text-muted">@{member.username}</span>
                      </span>
                      <span className="flex items-center gap-2">
                        {member.is_owner ? <Badge tone="accent">Owner</Badge> : null}
                        {member.user_id === user?.id ? <Badge tone="quiet">You</Badge> : null}
                      </span>
                    </li>
                  ))}
                </ul>
                <p className="mt-3 font-mono text-sm text-muted">Created {formatDate(team.created_at)}</p>
                {team.is_owner ? (
                  <div className="mt-5 space-y-3 border-t-2 border-ink/20 pt-4">
                    <Field label="Team name" htmlFor="edit-name">
                      <Input id="edit-name" value={editName} onChange={(e) => setEditName(e.target.value)} maxLength={80} />
                    </Field>
                    <label className="flex items-center gap-2">
                      <Checkbox checked={looking} onChange={(e) => setLooking(e.target.checked)} />
                      <span>Looking for more members (shown in discovery)</span>
                    </label>
                    <div className="flex flex-wrap gap-2">
                      <Button
                        size="sm"
                        disabled={busy || editName.trim().length < 2}
                        onClick={() =>
                          void act("Team updated.", () =>
                            api(`/teams/${team.id}`, {
                              method: "PATCH",
                              body: { name: editName, is_looking_for_members: looking },
                            }),
                          )
                        }
                      >
                        Save team
                      </Button>
                      <Button
                        size="sm"
                        variant="destructive"
                        disabled={busy || (team.members?.length ?? 0) > 1}
                        onClick={() => {
                          if (!window.confirm("Dissolve this team? Your registration will be removed.")) return;
                          void act("Team dissolved.", () => api(`/teams/${team.id}`, { method: "DELETE" }));
                        }}
                      >
                        Dissolve
                      </Button>
                    </div>
                    {(team.members?.length ?? 0) > 1 ? (
                      <p className="text-sm text-muted">Other members must leave before the team can be dissolved.</p>
                    ) : null}
                  </div>
                ) : null}
                <div className="mt-5 flex flex-wrap gap-2 border-t-2 border-ink/20 pt-4">
                  <Link href={`/events/${eventId}/register`} className={buttonVariants({ variant: "primary", size: "sm" })}>
                    Register team
                  </Link>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={busy}
                    onClick={() => {
                      if (!window.confirm("Leave this team? Your team registration will be removed.")) return;
                      void act("You left the team.", () => api(`/teams/${team.id}/leave`, { method: "POST" }));
                    }}
                  >
                    Leave team
                  </Button>
                </div>
              </div>
            </WindowCard>

            {team.is_owner ? (
              <WindowCard title="INVITE & MANAGE">
                <div className="space-y-4 p-5">
                  <Field label="Invite by username" htmlFor="invite-username" hint="They must accept before joining.">
                    <div className="flex gap-2">
                      <Input
                        id="invite-username"
                        value={inviteUsername}
                        onChange={(e) => setInviteUsername(e.target.value)}
                        placeholder="username"
                      />
                      <Button
                        disabled={busy || !inviteUsername.trim() || full}
                        onClick={() =>
                          void act("Invite sent.", async () => {
                            await api(`/teams/${team.id}/invites`, {
                              method: "POST",
                              body: { username: inviteUsername.trim() },
                            });
                            setInviteUsername("");
                          })
                        }
                      >
                        Invite
                      </Button>
                    </div>
                  </Field>
                  {full ? <p className="text-sm text-muted">Team is full. Ask someone to leave before inviting.</p> : null}

                  <div className="border-t-2 border-ink/20 pt-4">
                    <h3 className="font-pixel text-base">INVITE LINK</h3>
                    <p className="mt-1 text-sm text-muted">
                      Share one link. People with an account join immediately; new people continue here
                      after signing up.
                    </p>
                    {inviteLink ? (
                      <div className="mt-3 flex flex-col gap-2 sm:flex-row">
                        <Input
                          readOnly
                          value={`${typeof window === "undefined" ? "" : window.location.origin}/invites/team/${inviteLink.token}`}
                          aria-label="Team invite link"
                          onFocus={(event) => event.target.select()}
                        />
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() => {
                            const url = `${window.location.origin}/invites/team/${inviteLink.token}`;
                            void navigator.clipboard
                              .writeText(url)
                              .then(() => setNotice("Invite link copied."))
                              .catch(() => setError("Could not copy the link. Select it and copy manually."));
                          }}
                        >
                          Copy
                        </Button>
                      </div>
                    ) : (
                      <Button
                        className="mt-3"
                        size="sm"
                        variant="secondary"
                        disabled={busy}
                        onClick={() =>
                          void act("Invite link ready.", async () => {
                            const data = await api<{ invite_link: TeamInviteLink }>(
                              `/teams/${team.id}/invite-link`,
                            );
                            setInviteLink(data.invite_link);
                          })
                        }
                      >
                        Create invite link
                      </Button>
                    )}
                  </div>

                  <div>
                    <h3 className="font-pixel text-base">PENDING INVITES</h3>
                    {team.pending_invites && team.pending_invites.length > 0 ? (
                      <ul className="mt-2 space-y-2">
                        {team.pending_invites.map((invite) => (
                          <li key={invite.id} className="flex items-center justify-between gap-2 border-b-2 border-ink/15 pb-2">
                            <span className="font-mono text-sm">@{invite.invited_by_username || invite.invited_user_id.slice(0, 8)}</span>
                            <Button
                              size="sm"
                              variant="destructive"
                              disabled={busy}
                              onClick={() => void act("Invite cancelled.", () => api(`/teams/${team.id}/invites/${invite.id}`, { method: "DELETE" }))}
                            >
                              Cancel
                            </Button>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p className="mt-1 text-sm text-muted">No pending invites.</p>
                    )}
                  </div>

                  <div>
                    <h3 className="font-pixel text-base">JOIN REQUESTS</h3>
                    {team.pending_requests && team.pending_requests.length > 0 ? (
                      <ul className="mt-2 space-y-2">
                        {team.pending_requests.map((request) => (
                          <li key={request.id} className="border-b-2 border-ink/15 pb-2">
                            <p className="font-semibold">{request.full_name || request.username}</p>
                            <p className="font-mono text-sm text-muted">@{request.username}</p>
                            {request.message ? <p className="mt-1 text-base">{request.message}</p> : null}
                            <div className="mt-2 flex gap-2">
                              <Button
                                size="sm"
                                disabled={busy || full}
                                onClick={() => void act("Request accepted.", () => api(`/team-join-requests/${request.id}/accept`, { method: "POST" }))}
                              >
                                Accept
                              </Button>
                              <Button
                                size="sm"
                                variant="secondary"
                                disabled={busy}
                                onClick={() => void act("Request declined.", () => api(`/team-join-requests/${request.id}/reject`, { method: "POST" }))}
                              >
                                Decline
                              </Button>
                            </div>
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <p className="mt-1 text-sm text-muted">No open requests.</p>
                    )}
                  </div>
                </div>
              </WindowCard>
            ) : null}
          </div>

          <div className="space-y-6">
            {user ? <TeamChat teamId={team.id} currentUserId={user.id} /> : null}
            <WindowCard title="READ THIS">
              <div className="space-y-2 p-5 text-base text-muted">
                <p>Registering the team registers every member with the same answers.</p>
                <p>Late joiners inherit the team registration automatically.</p>
                <p>Leaving removes your registration; the organizer can still see the team history.</p>
              </div>
            </WindowCard>
          </div>
        </div>
      ) : (
        <div className="mt-6 grid gap-6 lg:grid-cols-[0.8fr_1.2fr]">
          <div className="space-y-6">
            <WindowCard title="CREATE A TEAM">
              <div className="space-y-4 p-5">
                <Field label="Team name" htmlFor="team-name" hint="2–80 characters. Must be unique for this event.">
                  <Input id="team-name" value={newName} onChange={(e) => setNewName(e.target.value)} maxLength={80} />
                </Field>
                <Button
                  className="w-full"
                  disabled={busy || newName.trim().length < 2}
                  onClick={() =>
                    void act("Team created.", async () => {
                      await api(`/events/${eventId}/teams`, { method: "POST", body: { name: newName.trim() } });
                      setNewName("");
                    })
                  }
                >
                  Create team
                </Button>
              </div>
            </WindowCard>

            {invites.length > 0 ? (
              <WindowCard title="YOUR INVITES">
                <ul className="divide-y-2 divide-ink/15">
                  {invites.map((invite) => (
                    <li key={invite.id} className="space-y-2 p-4">
                      <p className="font-semibold">{invite.team_name}</p>
                      <p className="font-mono text-sm text-muted">from @{invite.invited_by_username}</p>
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          disabled={busy}
                          onClick={() => void act("Joined team.", () => api(`/team-invites/${invite.id}/accept`, { method: "POST" }))}
                        >
                          Accept
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={busy}
                          onClick={() => void act("Invite declined.", () => api(`/team-invites/${invite.id}/reject`, { method: "POST" }))}
                        >
                          Decline
                        </Button>
                      </div>
                    </li>
                  ))}
                </ul>
              </WindowCard>
            ) : null}

            {requests.length > 0 ? (
              <WindowCard title="YOUR REQUESTS">
                <ul className="divide-y-2 divide-ink/15">
                  {requests.map((request) => (
                    <li key={request.id} className="p-4">
                      <p className="font-semibold">{request.team_name}</p>
                      <p className="font-mono text-sm text-muted">Awaiting response</p>
                    </li>
                  ))}
                </ul>
              </WindowCard>
            ) : null}
          </div>

          <div>
            <div className="flex items-end justify-between gap-3">
              <h2 className="font-pixel text-2xl">OPEN TEAMS</h2>
              <span className="font-mono text-sm text-muted">{teams.length} listed</span>
            </div>
            {teams.length === 0 ? (
              <div className="mt-4">
                <EmptyState title="NO TEAMS YET">Be the first to start one.</EmptyState>
              </div>
            ) : (
              <ul className="mt-4 space-y-4">
                {teams.map((row) => (
                  <li key={row.id} className="border-2 border-ink bg-surface p-4 shadow-hard-sm">
                    <div className="flex flex-wrap items-start justify-between gap-2">
                      <div>
                        <p className="font-pixel text-lg">{row.name}</p>
                        <p className="font-mono text-sm text-muted">
                          @{row.owner_username} · {row.member_count}/{event.max_team_size}
                        </p>
                      </div>
                      <Badge tone={row.is_looking_for_members ? "success" : "quiet"}>
                        {row.is_looking_for_members ? "RECRUITING" : "CLOSED"}
                      </Badge>
                    </div>
                    <div className="mt-3">
                      {row.my_pending_invite ? (
                        <p className="text-sm text-muted">You have an invite - accept it on the left.</p>
                      ) : row.my_pending_request ? (
                        <p className="text-sm text-muted">Request sent. Hang tight.</p>
                      ) : row.member_count >= event.max_team_size ? (
                        <p className="text-sm text-muted">This team is full.</p>
                      ) : (
                        <div className="flex flex-col gap-2 sm:flex-row">
                          <Input
                            aria-label={`Message for ${row.name}`}
                            value={joinMessage}
                            onChange={(e) => setJoinMessage(e.target.value)}
                            placeholder="Optional note to the owner"
                            maxLength={500}
                          />
                          <Button
                            size="sm"
                            disabled={busy}
                            onClick={() =>
                              void act("Request sent.", async () => {
                                await api(`/teams/${row.id}/join-requests`, {
                                  method: "POST",
                                  body: { message: joinMessage },
                                });
                                setJoinMessage("");
                              })
                            }
                          >
                            Ask to join
                          </Button>
                        </div>
                      )}
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
