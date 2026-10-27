"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { api, errorMessage, fieldErrors } from "@/lib/api";
import { useAuth, useRequireAuth } from "@/components/auth-provider";
import type { Bookmark, Profile } from "@/lib/types";
import { Alert, LoadingBlock } from "@/components/ui/feedback";
import { Badge, statusTone } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { WindowCard } from "@/components/ui/card";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { formatDate, statusLabel } from "@/lib/utils";

const years = [1, 2, 3, 4, 5, 6, 7, 8];

export default function SettingsPage() {
  const { user, profile, loading, refresh, logout } = useAuth();
  const { loading: authLoading } = useRequireAuth();

  const [fullName, setFullName] = useState("");
  const [college, setCollege] = useState("");
  const [course, setCourse] = useState("");
  const [year, setYear] = useState("");
  const [username, setUsername] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [subject, setSubject] = useState("");
  const [feedbackBody, setFeedbackBody] = useState("");
  const [bookmarks, setBookmarks] = useState<Bookmark[]>([]);

  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!user) return;
    setUsername(user.username.startsWith("user_") ? "" : user.username);
  }, [user]);

  useEffect(() => {
    if (!profile) return;
    setFullName(profile.full_name ?? "");
    setCollege(profile.college ?? "");
    setCourse(profile.course ?? "");
    setYear(profile.year_of_study ? String(profile.year_of_study) : "");
  }, [profile]);

  const loadBookmarks = useCallback(async () => {
    try {
      const data = await api<{ bookmarks: Bookmark[] }>("/bookmarks");
      setBookmarks(data.bookmarks ?? []);
    } catch {
      setBookmarks([]);
    }
  }, []);

  useEffect(() => {
    if (user) void loadBookmarks();
  }, [user, loadBookmarks]);

  function begin(section: string) {
    setBusy(section);
    setError(null);
    setNotice(null);
    setFields({});
  }

  function finish(section: string, message: string) {
    setBusy(null);
    setNotice(message);
    void section;
  }

  async function saveProfile(event: React.FormEvent) {
    event.preventDefault();
    begin("profile");
    try {
      await api<{ user: unknown; profile: Profile }>("/profile", {
        method: "PUT",
        body: {
          full_name: fullName,
          college,
          course,
          year_of_study: year ? Number(year) : null,
        },
      });
      await refresh();
      finish("profile", "Profile saved.");
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(null);
    }
  }

  async function saveUsername(event: React.FormEvent) {
    event.preventDefault();
    begin("username");
    try {
      await api("/auth/username", { method: "PATCH", body: { username } });
      await refresh();
      finish("username", "Username updated.");
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(null);
    }
  }

  async function savePassword(event: React.FormEvent) {
    event.preventDefault();
    if (newPassword !== confirmPassword) {
      setFields({ confirm_password: "Passwords do not match." });
      return;
    }
    begin("password");
    try {
      await api("/auth/password", {
        method: "POST",
        body: { current_password: currentPassword, new_password: newPassword },
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      finish("password", "Password changed. Other sessions were signed out.");
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(null);
    }
  }

  async function sendFeedback(event: React.FormEvent) {
    event.preventDefault();
    begin("feedback");
    try {
      await api("/feedback", { method: "POST", body: { subject, body: feedbackBody } });
      setSubject("");
      setFeedbackBody("");
      finish("feedback", "Thanks — we read every note.");
    } catch (err) {
      setError(errorMessage(err));
      setFields(fieldErrors(err));
    } finally {
      setBusy(null);
    }
  }

  async function removeBookmark(bookmark: Bookmark) {
    setBookmarks((current) => current.filter((item) => item.event_id !== bookmark.event_id));
    await api(`/events/${bookmark.event_id}/bookmark`, { method: "DELETE" }).catch(() => {
      void loadBookmarks();
    });
  }

  if (authLoading || loading) {
    return (
      <div className="mx-auto max-w-reading px-4 py-12">
        <LoadingBlock label="Loading settings" />
      </div>
    );
  }

  if (!user) return null;

  return (
    <div className="mx-auto max-w-reading px-4 py-8 sm:px-6 sm:py-12">
      <header>
        <p className="font-pixel text-sm text-muted">ACCOUNT</p>
        <h1 className="mt-1 font-pixel text-3xl sm:text-4xl">SETTINGS</h1>
        <p className="mt-2 text-base text-muted">
          Signed in as <span className="font-mono text-ink">{user.email}</span>.
          {user.is_admin ? <Badge tone="red" className="ml-2">Admin</Badge> : null}
        </p>
      </header>

      {error ? (
        <Alert tone="danger" className="mt-5">
          {error}
        </Alert>
      ) : null}
      {notice ? (
        <Alert tone="success" className="mt-5">
          {notice}
        </Alert>
      ) : null}

      <div className="mt-6 space-y-6">
        <WindowCard title="PROFILE">
          <form className="space-y-4 p-5" onSubmit={saveProfile} noValidate>
            <Field label="Full name" htmlFor="full_name" error={fields.full_name}>
              <Input
                id="full_name"
                value={fullName}
                onChange={(event) => setFullName(event.target.value)}
                autoComplete="name"
              />
            </Field>
            <Field label="College or company" htmlFor="college" error={fields.college}>
              <Input
                id="college"
                value={college}
                onChange={(event) => setCollege(event.target.value)}
                autoComplete="organization"
              />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="Course or role" htmlFor="course" error={fields.course}>
                <Input
                  id="course"
                  value={course}
                  onChange={(event) => setCourse(event.target.value)}
                  placeholder="Computer Science"
                />
              </Field>
              <Field label="Year of study" htmlFor="year" error={fields.year_of_study}>
                <Select id="year" value={year} onChange={(event) => setYear(event.target.value)}>
                  <option value="">Not applicable</option>
                  {years.map((value) => (
                    <option key={value} value={value}>
                      Year {value}
                    </option>
                  ))}
                </Select>
              </Field>
            </div>
            <p className="text-sm text-muted">
              These values autofill registration forms. Event submissions never expose them publicly.
            </p>
            <Button type="submit" disabled={busy !== null}>
              {busy === "profile" ? "Saving…" : "Save profile"}
            </Button>
          </form>
        </WindowCard>

        <WindowCard title="USERNAME">
          <form className="space-y-4 p-5" onSubmit={saveUsername} noValidate>
            <Field
              label="Username"
              htmlFor="username"
              error={fields.username}
              hint="3–30 characters. Letters, numbers and underscores. Links to your profile use this."
            >
              <Input
                id="username"
                value={username}
                onChange={(event) => setUsername(event.target.value.toLowerCase())}
                autoComplete="username"
              />
            </Field>
            <Button type="submit" disabled={busy !== null || username.length < 3}>
              {busy === "username" ? "Saving…" : "Update username"}
            </Button>
          </form>
        </WindowCard>

        <WindowCard title="PASSWORD">
          <form className="space-y-4 p-5" onSubmit={savePassword} noValidate>
            <Field label="Current password" htmlFor="current_password" error={fields.current_password}>
              <Input
                id="current_password"
                type="password"
                value={currentPassword}
                onChange={(event) => setCurrentPassword(event.target.value)}
                autoComplete="current-password"
              />
            </Field>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label="New password" htmlFor="new_password" error={fields.new_password}>
                <Input
                  id="new_password"
                  type="password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  autoComplete="new-password"
                />
              </Field>
              <Field label="Confirm new password" htmlFor="confirm_password" error={fields.confirm_password}>
                <Input
                  id="confirm_password"
                  type="password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  autoComplete="new-password"
                />
              </Field>
            </div>
            <Button
              type="submit"
              variant="secondary"
              disabled={busy !== null || !currentPassword || newPassword.length < 8}
            >
              {busy === "password" ? "Updating…" : "Change password"}
            </Button>
          </form>
        </WindowCard>

        <WindowCard title="BOOKMARKS">
          <div className="p-5">
            {bookmarks.length === 0 ? (
              <p className="text-base text-muted">
                Save an event from its detail page to keep it here.
              </p>
            ) : (
              <ul className="divide-y-2 divide-ink/15">
                {bookmarks.map((bookmark) => (
                  <li key={bookmark.event_id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                    <div>
                      <Link href={`/events/${bookmark.event_id}`} className="chunky-link font-semibold">
                        {bookmark.title}
                      </Link>
                      <p className="font-mono text-sm text-muted">
                        {formatDate(bookmark.starts_at)} · saved {formatDate(bookmark.bookmarked_at)}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge tone={statusTone(bookmark.status)}>{statusLabel(bookmark.status)}</Badge>
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={() => void removeBookmark(bookmark)}
                        aria-label={`Remove bookmark for ${bookmark.title}`}
                      >
                        Remove
                      </Button>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </WindowCard>

        <WindowCard title="SEND FEEDBACK">
          <form className="space-y-4 p-5" onSubmit={sendFeedback} noValidate>
            <Field label="Subject" htmlFor="subject" error={fields.subject}>
              <Input
                id="subject"
                value={subject}
                onChange={(event) => setSubject(event.target.value)}
                maxLength={160}
                placeholder="Something is broken, or could be better"
              />
            </Field>
            <Field label="Details" htmlFor="feedback" error={fields.body}>
              <Textarea
                id="feedback"
                value={feedbackBody}
                onChange={(event) => setFeedbackBody(event.target.value)}
                maxLength={5000}
                className="min-h-36"
                placeholder="What happened, and what did you expect?"
              />
            </Field>
            <Button type="submit" disabled={busy !== null || subject.trim().length < 3 || feedbackBody.trim().length < 3}>
              {busy === "feedback" ? "Sending…" : "Send feedback"}
            </Button>
          </form>
        </WindowCard>

        <WindowCard title="SESSION">
          <div className="flex flex-wrap items-center justify-between gap-3 p-5">
            <p className="text-base text-muted">
              Signing out clears this browser session. Your account and data stay put.
            </p>
            <Button variant="destructive" onClick={() => void logout()} disabled={busy !== null}>
              Log out
            </Button>
          </div>
        </WindowCard>
      </div>
    </div>
  );
}
