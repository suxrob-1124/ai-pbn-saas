"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { apiBase, authFetch, refreshTokens } from "./http";

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

  useEffect(() => {
    let timer: NodeJS.Timeout | null = null;
    const init = async () => {
      await refreshTokens().catch(() => {});
      await fetchMe().catch(() => {});
      timer = setInterval(() => {
        refreshTokens().catch(() => {});
      }, 10 * 60 * 1000); // каждые 10 минут (access TTL 15m)
    };
    init();
    return () => {
      if (timer) clearInterval(timer);
    };
  }, [fetchMe]);

  return { me, loading, error, refresh: fetchMe };
}
