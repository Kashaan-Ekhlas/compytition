"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import type { MeResponse, Profile, User } from "@/lib/types";

interface AuthState {
  user: User | null;
  profile: Profile | null;
  googleEnabled: boolean;
  loading: boolean;
  refresh: () => Promise<MeResponse>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [profile, setProfile] = useState<Profile | null>(null);
  const [googleEnabled, setGoogleEnabled] = useState(false);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      const data = await api<MeResponse>("/auth/me");
      setUser(data.user);
      setProfile(data.profile);
      setGoogleEnabled(data.google_enabled);
      return data;
    } catch {
      setUser(null);
      setProfile(null);
      return { user: null, profile: null, google_enabled: false };
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const logout = useCallback(async () => {
    await api("/auth/logout", { method: "POST" }).catch(() => undefined);
    setUser(null);
    setProfile(null);
    router.push("/");
    router.refresh();
  }, [router]);

  const value = useMemo<AuthState>(
    () => ({ user, profile, googleEnabled, loading, refresh: load, logout }),
    [user, profile, googleEnabled, loading, load, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used inside AuthProvider");
  return context;
}

/** Redirect to login once the session check has finished. */
export function useRequireAuth() {
  const { user, loading } = useAuth();
  const router = useRouter();
  useEffect(() => {
    if (!loading && !user) {
      const next = typeof window === "undefined" ? "/" : window.location.pathname;
      router.replace(`/login?next=${encodeURIComponent(next)}`);
    }
  }, [loading, user, router]);
  return { user, loading };
}
