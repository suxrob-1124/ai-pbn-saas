"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { extractContentAndTemplate, rewriteEditorImageUrls } from "../services/htmlTextExtraction";
import type { ClassMap } from "../services/cssClassMap";
import { apiBase } from "@/lib/http";

/**
 * Хук извлечения текстового контента из HTML и отслеживания dirty-состояния.
 *
 * Проблема: TipTap нормализует HTML при setContent (добавляет <p>, меняет пробелы).
 * Из-за этого onUpdate срабатывает сразу после загрузки, даже если пользователь
 * ничего не менял → contentDirty ложно становится true.
 *
 * Решение: храним «baseline» — первую нормализованную версию HTML от TipTap.
 * Dirty = true только когда текущий контент отличается от baseline.
 */
export function useHtmlTextExtractor(rawHtml: string, domainId: string) {
  const [editorContent, setEditorContent] = useState("");
  const [contentDirty, setContentDirty] = useState(false);
  const lastRawRef = useRef("");
  const templateRef = useRef("");
  const classMapRef = useRef<ClassMap>({});
  // baseline — нормализованная версия контента (после TipTap обработки).
  // Первый onChange после setContent устанавливает baseline.
  const baselineRef = useRef<string | null>(null);
  // Флаг: ждём первый onChange от TipTap после setContent
  const awaitingBaselineRef = useRef(false);

  useEffect(() => {
    if (rawHtml === lastRawRef.current) return;
    lastRawRef.current = rawHtml;
    const { content, template, classMap } = extractContentAndTemplate(rawHtml);
    templateRef.current = template;
    classMapRef.current = classMap;
    let extracted = content;
    if (domainId) {
      extracted = rewriteEditorImageUrls(extracted, apiBase(), domainId);
    }
    baselineRef.current = null;
    awaitingBaselineRef.current = true;
    setEditorContent(extracted);
    setContentDirty(false);
  }, [rawHtml, domainId]);

  const handleContentChange = useCallback((html: string) => {
    // Первый onChange после установки контента — TipTap нормализовал HTML.
    // Сохраняем как baseline и НЕ помечаем dirty.
    if (awaitingBaselineRef.current) {
      baselineRef.current = html;
      awaitingBaselineRef.current = false;
      setEditorContent(html);
      return;
    }

    setEditorContent(html);

    // Dirty только если контент реально отличается от baseline
    if (baselineRef.current !== null) {
      setContentDirty(html !== baselineRef.current);
    } else {
      setContentDirty(true);
    }
  }, []);

  const resetContent = useCallback(() => {
    const { content, template, classMap } = extractContentAndTemplate(lastRawRef.current);
    templateRef.current = template;
    classMapRef.current = classMap;
    let extracted = content;
    if (domainId) {
      extracted = rewriteEditorImageUrls(extracted, apiBase(), domainId);
    }
    baselineRef.current = null;
    awaitingBaselineRef.current = true;
    setEditorContent(extracted);
    setContentDirty(false);
  }, [domainId]);

  return {
    editorContent,
    setEditorContent: handleContentChange,
    contentDirty,
    resetContent,
    template: templateRef.current,
    classMap: classMapRef.current,
  };
}
