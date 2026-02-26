"use client";

import { useCallback, useRef } from "react";

import { showToast } from "../../../lib/toastStore";

export function usePreviewNavigationGuard() {
  const previewGuardToastAtRef = useRef(0);

  const showPreviewNavigationBlockedToast = useCallback(() => {
    const now = Date.now();
    if (now - previewGuardToastAtRef.current < 1200) return;
    previewGuardToastAtRef.current = now;
    showToast({
      type: "error",
      title: "Переход в preview отключён",
      message: "Ссылки внутри предпросмотра заблокированы, чтобы не ломать редактор.",
    });
  }, []);

  const bindPreviewNavigationGuard = useCallback((iframe: HTMLIFrameElement | null) => {
    if (!iframe) return;
    const doc = iframe.contentDocument;
    if (!doc) return;
    if ((doc as unknown as { __editorPreviewGuardBound?: boolean }).__editorPreviewGuardBound) return;
    (doc as unknown as { __editorPreviewGuardBound?: boolean }).__editorPreviewGuardBound = true;

    doc.addEventListener(
      "click",
      (event) => {
        const target = event.target as HTMLElement | null;
        if (!target) return;
        const anchor = target.closest("a[href]") as HTMLAnchorElement | null;
        if (!anchor) return;
        const href = (anchor.getAttribute("href") || "").trim();
        if (!href) return;
        const isInPageAnchor = (() => {
          if (href.startsWith("#")) return true;
          if (!href.includes("#")) return false;
          if (/^(mailto:|tel:|javascript:|data:)/i.test(href)) return false;
          try {
            const url = new URL(href, "https://preview.local/");
            if (!url.hash || url.hash === "#") return false;
            // Allow hash-navigation inside preview for local/current-page links.
            if (!/^https?:$/i.test(url.protocol)) return false;
            return url.origin === "https://preview.local" || href.startsWith("http");
          } catch {
            return false;
          }
        })();
        if (isInPageAnchor) {
          event.preventDefault();
          event.stopPropagation();
          const hash = href.includes("#") ? href.slice(href.indexOf("#") + 1) : "";
          const anchorId = decodeURIComponent(hash).trim();
          if (!anchorId) return;
          const cssEscape = (globalThis as { CSS?: { escape?: (value: string) => string } }).CSS?.escape;
          const escapedId = typeof cssEscape === "function" ? cssEscape(anchorId) : anchorId;
          const targetEl =
            doc.getElementById(anchorId) ||
            doc.querySelector(`[id="${escapedId}"]`) ||
            doc.querySelector(`[name="${escapedId}"]`);
          if (targetEl && "scrollIntoView" in targetEl) {
            (targetEl as HTMLElement).scrollIntoView({ behavior: "smooth", block: "start" });
          }
          return;
        }
        event.preventDefault();
        event.stopPropagation();
        showPreviewNavigationBlockedToast();
      },
      true
    );
    doc.addEventListener(
      "submit",
      (event) => {
        event.preventDefault();
        event.stopPropagation();
        showPreviewNavigationBlockedToast();
      },
      true
    );
  }, [showPreviewNavigationBlockedToast]);

  return { bindPreviewNavigationGuard };
}
