"use client";

import { useState, useCallback, useRef } from "react";
import { aiCreatePage, saveFile, createFileOrDir } from "@/lib/fileApi";
import { showToast } from "@/lib/toastStore";
import { useAIFlowState } from "@/features/editor-v3/hooks/useAIFlowState";
import {
  injectRuntimeAssets,
  rewriteHtmlAssetRefs,
  normalizeGeneratedHtmlResourcePaths,
} from "@/features/editor-v3/services/editorPreviewUtils";
import { apiBase } from "@/lib/http";
import type { SEOData } from "../types/content-editor";
import { applyPageMeta, type PageMeta } from "../services/htmlMetaExtraction";
import { contentEditorRu } from "../services/i18n-content-ru";

const t = contentEditorRu.compiler;

type CompilerResult = {
  html: string;
  path: string;
  previewSrcDoc: string;
};

export function useAICompiler(
  domainId: string,
  stylePreview: string,
  scriptPreview: string,
) {
  const flow = useAIFlowState();
  const [result, setResult] = useState<CompilerResult | null>(null);
  const [busy, setBusy] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const compile = useCallback(
    async (params: {
      targetPath: string;
      wysiwygHtml: string;
      seo: SEOData;
      meta?: PageMeta;
      existingHtml: string;
    }) => {
      const { targetPath, wysiwygHtml, seo, meta, existingHtml } = params;

      if (!wysiwygHtml.trim()) {
        showToast({ type: "error", title: "Нечего верстать — напишите контент" });
        return;
      }

      setBusy(true);
      setResult(null);
      flow.start(t.statusAnalyzing, "validating");

      const abortController = new AbortController();
      abortRef.current = abortController;

      try {
        const seoBlock = [
          seo.title && `Title: ${seo.title}`,
          seo.description && `Description: ${seo.description}`,
          seo.ogTitle && `OG Title: ${seo.ogTitle}`,
          seo.ogDescription && `OG Description: ${seo.ogDescription}`,
        ]
          .filter(Boolean)
          .join("\n");

        const metaBlock = meta
          ? [
              meta.favicon && `Favicon: ${meta.favicon}`,
              meta.logo && `Logo image src: ${meta.logo}`,
              meta.headerNav.length > 0 &&
                `Header navigation:\n${meta.headerNav.map((l: { label: string; href: string }) => `  - "${l.label}" → ${l.href}`).join("\n")}`,
              meta.footerNav.length > 0 &&
                `Footer navigation:\n${meta.footerNav.map((l: { label: string; href: string }) => `  - "${l.label}" → ${l.href}`).join("\n")}`,
            ]
              .filter(Boolean)
              .join("\n")
          : "";

        const instruction = [
          "Ты — frontend-интегратор. Я даю тебе исходный HTML страницы и новый контент.",
          "Твоя задача — заменить старый контент на новый, СТРОГО сохранив все существующие CSS-классы, обёртки, div-ы и структуру документа.",
          "Если переданы изменения навигации, логотипа или фавикона — примени их в соответствующих местах.",
          "Сохрани все ссылки на style.css и script.js.",
          "",
          existingHtml
            ? `Текущий HTML страницы (сохрани его структуру, классы, обёртки):\n\`\`\`html\n${existingHtml.slice(0, 8000)}\n\`\`\``
            : "Это новая страница — используй стили и структуру, соответствующие дизайну сайта.",
          "",
          `Новый контент для вставки:\n${wysiwygHtml}`,
          "",
          seoBlock && `SEO-метатеги (вставь в <head>):\n${seoBlock}`,
          metaBlock && `Мета-данные страницы (обнови в HTML):\n${metaBlock}`,
        ]
          .filter(Boolean)
          .join("\n");

        flow.setStatus("sending", t.statusDesigning);

        const response = await aiCreatePage(
          domainId,
          {
            instruction,
            target_path: targetPath,
            with_assets: false,
            context_mode: "auto",
          },
          { signal: abortController.signal },
        );

        flow.setStatus("parsing", t.statusOptimizing);

        const mainFile = response.files.find((f) => f.path === targetPath) || response.files[0];
        if (!mainFile) {
          flow.fail("AI не вернул файлов", "Ошибка генерации");
          return;
        }

        let normalized = normalizeGeneratedHtmlResourcePaths(mainFile.content, mainFile.path);

        // Применяем мета-данные (favicon, logo, nav) если были изменены
        if (meta) {
          normalized = applyPageMeta(normalized, meta);
        }

        let previewSrcDoc = normalized;
        if (stylePreview) {
          previewSrcDoc = injectRuntimeAssets(previewSrcDoc, stylePreview, scriptPreview);
        }
        previewSrcDoc = rewriteHtmlAssetRefs(previewSrcDoc, domainId);

        // Добавляем <base> для корректного разрешения относительных путей в preview
        const baseTag = `<base href="${apiBase()}/api/domains/${domainId}/files/">`;
        previewSrcDoc = previewSrcDoc.replace(/<head([^>]*)>/i, `<head$1>${baseTag}`);

        const compileResult: CompilerResult = {
          html: normalized,
          path: mainFile.path,
          previewSrcDoc,
        };

        setResult(compileResult);
        flow.finish(t.statusReady, "ready");

        return compileResult;
      } catch (err: any) {
        if (err instanceof DOMException && err.name === "AbortError") {
          flow.reset();
          return;
        }
        flow.fail(err, t.error);
        showToast({
          type: "error",
          title: t.error,
          message: err?.message || String(err),
        });
      } finally {
        setBusy(false);
        abortRef.current = null;
      }
    },
    [domainId, flow, stylePreview, scriptPreview],
  );

  const apply = useCallback(
    async (path: string, html: string, version?: number) => {
      setBusy(true);
      flow.setStatus("applying", "Сохраняем...");
      try {
        try {
          await saveFile(domainId, path, html, "Content mode publish", {
            expectedVersion: version,
            source: "ai",
          });
        } catch (err: any) {
          if (err?.status === 404) {
            await createFileOrDir(domainId, {
              kind: "file",
              path,
              content: html,
              mime_type: "text/html",
            });
          } else {
            throw err;
          }
        }

        flow.finish(t.success, "done");
        showToast({ type: "success", title: t.success });
        setResult(null);
        return true;
      } catch (err: any) {
        flow.fail(err, "Ошибка сохранения");
        showToast({
          type: "error",
          title: "Ошибка сохранения",
          message: err?.message || String(err),
        });
        return false;
      } finally {
        setBusy(false);
      }
    },
    [domainId, flow],
  );

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    setResult(null);
    flow.reset();
    setBusy(false);
  }, [flow]);

  return { compile, apply, cancel, result, busy, flow };
}
