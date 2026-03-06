"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import {
  extractPageMeta,
  defaultPageMeta,
  type PageMeta,
  type NavLink,
  type NavSection,
} from "../services/htmlMetaExtraction";

const NAV_KEY: Record<NavSection, "headerNav" | "footerNav" | "asideNav"> = {
  header: "headerNav",
  footer: "footerNav",
  aside: "asideNav",
};

export function usePageMeta(rawHtml: string) {
  const [meta, setMeta] = useState<PageMeta>({ ...defaultPageMeta });
  const [metaDirty, setMetaDirty] = useState(false);
  const lastRawRef = useRef("");

  useEffect(() => {
    if (rawHtml === lastRawRef.current) return;
    lastRawRef.current = rawHtml;
    setMeta(extractPageMeta(rawHtml));
    setMetaDirty(false);
  }, [rawHtml]);

  const updateMetaField = useCallback(
    <K extends keyof PageMeta>(key: K, value: PageMeta[K]) => {
      setMeta((prev) => ({ ...prev, [key]: value }));
      setMetaDirty(true);
    },
    [],
  );

  const updateNavLink = useCallback(
    (section: NavSection, index: number, field: keyof NavLink, value: string) => {
      const key = NAV_KEY[section];
      setMeta((prev) => {
        const updated = [...prev[key]];
        updated[index] = { ...updated[index], [field]: value };
        return { ...prev, [key]: updated };
      });
      setMetaDirty(true);
    },
    [],
  );

  const addNavLink = useCallback((section: NavSection) => {
    const key = NAV_KEY[section];
    setMeta((prev) => ({
      ...prev,
      [key]: [...prev[key], { label: "", href: "#" }],
    }));
    setMetaDirty(true);
  }, []);

  const removeNavLink = useCallback((section: NavSection, index: number) => {
    const key = NAV_KEY[section];
    setMeta((prev) => ({
      ...prev,
      [key]: prev[key].filter((_, i) => i !== index),
    }));
    setMetaDirty(true);
  }, []);

  return {
    meta,
    metaDirty,
    updateMetaField,
    updateNavLink,
    addNavLink,
    removeNavLink,
  };
}
