import { useCallback, useRef, useState } from "react";

type LockMap = Record<string, string>;

export function useActionLocks() {
  const locksRef = useRef<Map<string, string>>(new Map());
  const [locks, setLocks] = useState<LockMap>({});

  const isLocked = useCallback((key: string) => {
    if (!key) return false;
    return locksRef.current.has(key);
  }, []);

  const lockReason = useCallback(
    (key: string) => {
      if (!key) return undefined;
      return locks[key];
    },
    [locks]
  );

  const runLocked = useCallback(async <T>(key: string, fn: () => Promise<T>, reason = "Выполняется") => {
    const normalized = key.trim();
    if (!normalized) {
      return fn();
    }
    if (locksRef.current.has(normalized)) {
      return undefined;
    }

    locksRef.current.set(normalized, reason);
    setLocks((prev) => ({ ...prev, [normalized]: reason }));
    try {
      return await fn();
    } finally {
      locksRef.current.delete(normalized);
      setLocks((prev) => {
        if (!(normalized in prev)) return prev;
        const next = { ...prev };
        delete next[normalized];
        return next;
      });
    }
  }, []);

  return { isLocked, runLocked, lockReason };
}
