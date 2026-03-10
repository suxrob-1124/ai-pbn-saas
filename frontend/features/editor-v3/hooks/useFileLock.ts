"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { apiBase, refreshTokens } from "@/lib/http";

export type FileLockInfo = {
  file_path: string;
  locked_by: string;
  locked_at: string;
  expires_at: string;
  is_owner?: boolean;
};

const HEARTBEAT_INTERVAL = 60_000; // 60s — renew before 120s TTL expires
const POLL_INTERVAL = 30_000;      // 30s — refresh all domain locks

async function lockRequest(method: string, domainId: string, body?: object) {
  const doFetch = () =>
    fetch(`${apiBase()}/api/domains/${domainId}/files/locks`, {
      method,
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
  let res = await doFetch();
  if (res.status === 401) {
    await refreshTokens();
    res = await doFetch();
  }
  return res;
}

type Props = {
  domainId: string;
  currentFilePath: string | undefined;
  userEmail: string | undefined;
  readOnly?: boolean;
};

export function useFileLock({ domainId, currentFilePath, userEmail, readOnly }: Props) {
  // Map of filePath → lock info for all active locks in the domain
  const [domainLocks, setDomainLocks] = useState<Record<string, FileLockInfo>>({});
  const heartbeatRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const prevPathRef = useRef<string | undefined>(undefined);

  const fetchDomainLocks = useCallback(async () => {
    try {
      const res = await lockRequest("GET", domainId);
      if (!res.ok) return;
      const locks: FileLockInfo[] = await res.json();
      const map: Record<string, FileLockInfo> = {};
      for (const l of locks) map[l.file_path] = l;
      setDomainLocks(map);
    } catch { /* ignore */ }
  }, [domainId]);

  const acquireLock = useCallback(async (path: string) => {
    if (readOnly || !userEmail) return;
    try {
      await lockRequest("POST", domainId, { path });
      await fetchDomainLocks();
    } catch { /* ignore */ }
  }, [domainId, readOnly, userEmail, fetchDomainLocks]);

  const releaseLock = useCallback(async (path: string) => {
    if (readOnly || !userEmail) return;
    try {
      await lockRequest("DELETE", domainId, { path });
    } catch { /* ignore */ }
  }, [domainId, readOnly, userEmail]);

  // On file change: release previous, acquire new
  useEffect(() => {
    const prev = prevPathRef.current;
    prevPathRef.current = currentFilePath;

    if (prev && prev !== currentFilePath) {
      releaseLock(prev);
    }
    if (currentFilePath) {
      acquireLock(currentFilePath);
    }
  }, [currentFilePath, acquireLock, releaseLock]);

  // Heartbeat to renew current lock
  useEffect(() => {
    if (heartbeatRef.current) clearInterval(heartbeatRef.current);
    if (!currentFilePath || readOnly) return;
    heartbeatRef.current = setInterval(() => {
      acquireLock(currentFilePath);
    }, HEARTBEAT_INTERVAL);
    return () => {
      if (heartbeatRef.current) clearInterval(heartbeatRef.current);
    };
  }, [currentFilePath, readOnly, acquireLock]);

  // Poll domain locks to detect other users
  useEffect(() => {
    fetchDomainLocks();
    pollRef.current = setInterval(fetchDomainLocks, POLL_INTERVAL);
    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [fetchDomainLocks]);

  // Release on unmount
  useEffect(() => {
    return () => {
      if (prevPathRef.current) {
        releaseLock(prevPathRef.current);
      }
    };
  }, [releaseLock]);

  const currentLock = currentFilePath ? domainLocks[currentFilePath] : undefined;
  const isLockedByOther = Boolean(
    currentLock && userEmail && currentLock.locked_by !== userEmail
  );

  return { domainLocks, currentLock, isLockedByOther };
}
