"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { apiBase, authFetch, refreshTokens } from "./http";

const MAX_CONSECUTIVE_REFRESH_FAILURES = 3;

export type Me = {
  email: string;
  name?: string | null;
  avatarUrl?: string | null;
  role?: string | null;
  isApproved?: boolean;
  verified?: boolean;
  apiKeyUpdatedAt?: string | null;
  hasApiKey?: boolean;
  apiKeyPrefix?: string;
};

export function useOptionalMe() {
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let mounted = true;
    const load = async () => {
      setLoading(true);
      try {
        await refreshTokens().catch(() => {});
        const res = await fetch(`${apiBase()}/api/me`, { credentials: "include" });
        if (!mounted) return;
        if (!res.ok) {
          setMe(null);
          return;
        }
        const data = (await res.json()) as Me;
        setMe(data);
      } catch {
        if (mounted) setMe(null);
      } finally {
        if (mounted) setLoading(false);
      }
    };
    load();
    return () => {
      mounted = false;
    };
  }, []);

  return { me, loading, isAuthed: Boolean(me) };
}

export function useAuthGuard() {
  const router = useRouter();
  const [me, setMe] = useState<Me | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const lastRefreshAtRef = useRef(0);
  const refreshInFlightRef = useRef<Promise<boolean> | null>(null);
  const lastActivityCheckRef = useRef(0);
  const consecutiveFailuresRef = useRef(0);

  const fetchMe = useCallback(async () => {
    setLoading(true);
    try {
      const res = await authFetch<Me>("/api/me");
      setMe(res);
      setError(null);
    } catch (err: any) {
      setError(err?.message || "unauthorized");
      router.replace("/login");
      throw err;
    } finally {
      setLoading(false);
    }
  }, [router]);

  const refreshNow = useCallback(async () => {
    if (consecutiveFailuresRef.current >= MAX_CONSECUTIVE_REFRESH_FAILURES) {
      router.replace("/login");
      return false;
    }
    if (refreshInFlightRef.current) {
      return refreshInFlightRef.current;
    }
    const req = refreshTokens()
      .then((ok) => {
        if (ok) {
          lastRefreshAtRef.current = Date.now();
          consecutiveFailuresRef.current = 0;
        } else {
          consecutiveFailuresRef.current += 1;
          if (consecutiveFailuresRef.current >= MAX_CONSECUTIVE_REFRESH_FAILURES) {
            router.replace("/login");
          }
        }
        return ok;
      })
      .finally(() => {
        refreshInFlightRef.current = null;
      });
    refreshInFlightRef.current = req;
    return req;
  }, [router]);

  useEffect(() => {
    let timer: NodeJS.Timeout | null = null;
    const refreshIntervalMs =
      Number(process.env.NEXT_PUBLIC_REFRESH_INTERVAL_MS) || 10 * 60 * 1000;
    const activityRefreshMinMs =
      Number(process.env.NEXT_PUBLIC_REFRESH_ON_ACTIVITY_MS) ||
      Math.max(60_000, Math.floor(refreshIntervalMs / 2));
    const onActivity = () => {
      const now = Date.now();
      if (now - lastActivityCheckRef.current < 15_000) {
        return;
      }
      lastActivityCheckRef.current = now;
      if (now - lastRefreshAtRef.current >= activityRefreshMinMs) {
        refreshNow().catch(() => {});
      }
    };
    const onFocus = () => {
      refreshNow().catch(() => {});
    };
    const onVisibility = () => {
      if (document.visibilityState === "visible") {
        refreshNow().catch(() => {});
      }
    };
    const init = async () => {
      await refreshNow().catch(() => {});
      await fetchMe().catch(() => {});
      timer = setInterval(() => {
        refreshNow().catch(() => {});
      }, refreshIntervalMs);
    };
    init();
    window.addEventListener("focus", onFocus);
    window.addEventListener("online", onFocus);
    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("mousemove", onActivity);
    window.addEventListener("keydown", onActivity);
    window.addEventListener("touchstart", onActivity);
    window.addEventListener("scroll", onActivity);
    return () => {
      if (timer) clearInterval(timer);
      window.removeEventListener("focus", onFocus);
      window.removeEventListener("online", onFocus);
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("mousemove", onActivity);
      window.removeEventListener("keydown", onActivity);
      window.removeEventListener("touchstart", onActivity);
      window.removeEventListener("scroll", onActivity);
    };
  }, [fetchMe, refreshNow]);

  return { me, loading, error, refresh: fetchMe };
}
