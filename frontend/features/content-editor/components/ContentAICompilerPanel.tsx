"use client";

import { useRef, useState } from "react";
import { FiMonitor, FiTablet, FiSmartphone, FiCheck } from "react-icons/fi";
import { usePreviewNavigationGuard } from "@/features/editor-v3/hooks/usePreviewNavigationGuard";
import { contentEditorRu } from "../services/i18n-content-ru";

type CompilerResult = {
  html: string;
  path: string;
  previewSrcDoc: string;
};

type ContentAICompilerPanelProps = {
  result: CompilerResult | null;
  onApply: () => void;
  onCancel: () => void;
};

const t = contentEditorRu.compiler;

const VIEWPORT_OPTIONS = [
  { key: "desktop" as const, icon: FiMonitor, label: t.viewportDesktop, maxWidth: "100%" },
  { key: "tablet" as const, icon: FiTablet, label: t.viewportTablet, maxWidth: "820px" },
  { key: "mobile" as const, icon: FiSmartphone, label: t.viewportMobile, maxWidth: "390px" },
];

/**
 * Preview panel — only renders when there's a compile result.
 * The compile button itself is now in ContentSEOPanel (right sidebar).
 */
export function ContentAICompilerPanel({
  result,
  onApply,
  onCancel,
}: ContentAICompilerPanelProps) {
  const [viewport, setViewport] = useState<"desktop" | "tablet" | "mobile">("desktop");
  const previewRef = useRef<HTMLIFrameElement>(null);
  const { bindPreviewNavigationGuard } = usePreviewNavigationGuard();

  if (!result) return null;

  const viewportMaxWidth =
    VIEWPORT_OPTIONS.find((v) => v.key === viewport)?.maxWidth || "100%";

  return (
    <div className="rounded-xl border border-slate-200 bg-white/80 shadow dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-2.5 dark:border-slate-700">
        <h3 className="text-sm font-semibold text-slate-700 dark:text-slate-200">
          {t.preview}
        </h3>
        <div className="flex items-center gap-1">
          {VIEWPORT_OPTIONS.map((v) => (
            <button
              key={v.key}
              type="button"
              title={v.label}
              onClick={() => setViewport(v.key)}
              className={`rounded-md p-1.5 transition-colors ${
                viewport === v.key
                  ? "bg-indigo-100 text-indigo-700 dark:bg-indigo-500/20 dark:text-indigo-400"
                  : "text-slate-400 hover:text-slate-600 dark:hover:text-slate-300"
              }`}
            >
              <v.icon className="h-4 w-4" />
            </button>
          ))}
        </div>
      </div>

      <div className="flex justify-center bg-slate-50 p-4 dark:bg-slate-950/50">
        <iframe
          ref={previewRef}
          srcDoc={result.previewSrcDoc}
          className="h-[500px] w-full rounded-lg border border-slate-200 bg-white transition-all dark:border-slate-700"
          style={{ maxWidth: viewportMaxWidth }}
          sandbox="allow-same-origin"
          title={t.preview}
          onLoad={() => {
            if (previewRef.current) {
              bindPreviewNavigationGuard(previewRef.current);
            }
          }}
        />
      </div>

      <div className="flex items-center justify-end gap-2 border-t border-slate-200 px-4 py-3 dark:border-slate-700">
        <button
          type="button"
          onClick={onCancel}
          className="rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
        >
          {t.cancel}
        </button>
        <button
          type="button"
          onClick={onApply}
          className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-500 transition-colors"
        >
          <FiCheck className="h-4 w-4" />
          {t.apply}
        </button>
      </div>
    </div>
  );
}
