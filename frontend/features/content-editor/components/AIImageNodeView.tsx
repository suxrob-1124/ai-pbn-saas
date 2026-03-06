"use client";

import { NodeViewWrapper } from "@tiptap/react";
import { useState, useCallback } from "react";
import { FiImage, FiLoader } from "react-icons/fi";
import { generateEditorAsset } from "@/lib/fileApi";
import { apiBase } from "@/lib/http";
import { encodePath } from "@/features/editor-v3/services/editorPreviewUtils";
import { showToast } from "@/lib/toastStore";
import { contentEditorRu } from "../services/i18n-content-ru";

const t = contentEditorRu.imageGen;

export function AIImageNodeView(props: any) {
  const { node, updateAttributes, editor, deleteNode } = props;
  const { prompt, alt, src, generating } = node.attrs;
  const [localPrompt, setLocalPrompt] = useState(prompt || "");
  const [localAlt, setLocalAlt] = useState(alt || "");

  const domainId =
    editor?.extensionManager?.extensions?.find(
      (ext: any) => ext.name === "aiImageBlock",
    )?.options?.domainId || "";

  const handleGenerate = useCallback(async () => {
    if (!localPrompt.trim() || !domainId) return;

    updateAttributes({ generating: true, prompt: localPrompt, alt: localAlt });

    try {
      const path = `assets/content-img-${Date.now()}.webp`;
      const result = await generateEditorAsset(domainId, {
        path,
        prompt: localPrompt,
        mime_type: "image/webp",
      });

      if (result.status === "ok") {
        const imageUrl = `${apiBase()}/api/domains/${domainId}/files/${encodePath(path)}?raw=1`;
        updateAttributes({ src: imageUrl, generating: false });
      } else {
        updateAttributes({ generating: false });
        showToast({ type: "error", title: result.error_message || "Изображение не сгенерировано" });
      }
    } catch (err: any) {
      updateAttributes({ generating: false });
      showToast({
        type: "error",
        title: "Ошибка генерации",
        message: err?.message || String(err),
      });
    }
  }, [localPrompt, localAlt, domainId, updateAttributes]);

  if (src) {
    return (
      <NodeViewWrapper className="my-4">
        <figure className="rounded-xl border border-slate-200 bg-slate-50 p-2 dark:border-slate-700 dark:bg-slate-900">
          <img
            src={src}
            alt={alt || localAlt}
            className="mx-auto max-h-96 rounded-lg object-contain"
          />
          {(alt || localAlt) && (
            <figcaption className="mt-1 text-center text-xs text-slate-500 dark:text-slate-400">
              {alt || localAlt}
            </figcaption>
          )}
        </figure>
      </NodeViewWrapper>
    );
  }

  return (
    <NodeViewWrapper className="my-4">
      <div className="rounded-xl border-2 border-dashed border-slate-300 bg-slate-50 p-4 dark:border-slate-600 dark:bg-slate-900/50">
        <div className="mb-3 flex items-center gap-2 text-sm font-medium text-slate-600 dark:text-slate-300">
          <FiImage className="h-4 w-4" />
          AI-генерация изображения
        </div>

        <div className="space-y-2">
          <div>
            <label className="mb-0.5 block text-xs text-slate-500 dark:text-slate-400">
              {t.promptLabel}
            </label>
            <textarea
              value={localPrompt}
              onChange={(e) => setLocalPrompt(e.target.value)}
              placeholder={t.promptPlaceholder}
              rows={2}
              disabled={generating}
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
            />
          </div>

          <div>
            <label className="mb-0.5 block text-xs text-slate-500 dark:text-slate-400">
              {t.altLabel}
            </label>
            <input
              type="text"
              value={localAlt}
              onChange={(e) => setLocalAlt(e.target.value)}
              disabled={generating}
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-100"
            />
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={handleGenerate}
              disabled={generating || !localPrompt.trim()}
              className="inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {generating ? (
                <>
                  <FiLoader className="h-3.5 w-3.5 animate-spin" />
                  {t.generating}
                </>
              ) : (
                t.generate
              )}
            </button>
            <button
              type="button"
              onClick={() => deleteNode()}
              className="text-xs text-slate-400 hover:text-red-500 dark:hover:text-red-400"
            >
              Удалить
            </button>
          </div>
        </div>
      </div>
    </NodeViewWrapper>
  );
}
