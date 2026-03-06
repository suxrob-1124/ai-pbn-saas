"use client";

import { useState, useCallback, useEffect } from "react";
import type { EditorMode } from "../types/content-editor";

function getStorageKey(domainId: string) {
  return `editor-mode-${domainId}`;
}

function getDefaultMode(_role: string | undefined | null): EditorMode {
  return "content";
}

export function useContentMode(domainId: string, role: string | undefined | null) {
  const [mode, setModeState] = useState<EditorMode>(() => {
    if (typeof window === "undefined") return getDefaultMode(role);
    const stored = localStorage.getItem(getStorageKey(domainId));
    if (stored === "content" || stored === "code") return stored;
    return getDefaultMode(role);
  });

  useEffect(() => {
    if (!role) return;
    const stored = localStorage.getItem(getStorageKey(domainId));
    if (stored === "content" || stored === "code") return;
    setModeState(getDefaultMode(role));
  }, [role, domainId]);

  const setMode = useCallback(
    (next: EditorMode) => {
      setModeState(next);
      localStorage.setItem(getStorageKey(domainId), next);
    },
    [domainId],
  );

  return { mode, setMode };
}
