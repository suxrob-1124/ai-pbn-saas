"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import type { SEOData } from "../types/content-editor";
import { parseSeoFromHtml } from "../services/seoExtraction";

export function useSeoParser(rawHtml: string) {
  const [seo, setSeo] = useState<SEOData>({
    title: "",
    description: "",
    ogTitle: "",
    ogDescription: "",
  });
  const [seoDirty, setSeoDirty] = useState(false);
  const lastRawRef = useRef("");

  useEffect(() => {
    if (rawHtml === lastRawRef.current) return;
    lastRawRef.current = rawHtml;
    const parsed = parseSeoFromHtml(rawHtml);
    setSeo(parsed);
    setSeoDirty(false);
  }, [rawHtml]);

  const updateSeoField = useCallback(<K extends keyof SEOData>(key: K, value: string) => {
    setSeo((prev) => ({ ...prev, [key]: value }));
    setSeoDirty(true);
  }, []);

  const resetSeo = useCallback(() => {
    const parsed = parseSeoFromHtml(lastRawRef.current);
    setSeo(parsed);
    setSeoDirty(false);
  }, []);

  return { seo, updateSeoField, seoDirty, resetSeo };
}
