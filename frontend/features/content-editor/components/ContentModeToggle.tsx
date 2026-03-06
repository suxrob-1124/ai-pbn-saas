"use client";

import { FiEdit3, FiCode } from "react-icons/fi";
import type { EditorMode } from "../types/content-editor";
import { contentEditorRu } from "../services/i18n-content-ru";

type ContentModeToggleProps = {
  mode: EditorMode;
  setMode: (mode: EditorMode) => void;
};

const t = contentEditorRu.modeToggle;

export function ContentModeToggle({ mode, setMode }: ContentModeToggleProps) {
  return (
    <div className="inline-flex items-center rounded-lg border border-slate-200 bg-white/80 p-0.5 shadow-sm dark:border-slate-700 dark:bg-slate-800/80">
      <button
        type="button"
        onClick={() => setMode("content")}
        className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-semibold transition-colors ${
          mode === "content"
            ? "bg-indigo-600 text-white shadow-sm"
            : "text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
        }`}
      >
        <FiEdit3 className="h-3.5 w-3.5" />
        {t.content}
      </button>
      <button
        type="button"
        onClick={() => setMode("code")}
        className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-semibold transition-colors ${
          mode === "code"
            ? "bg-indigo-600 text-white shadow-sm"
            : "text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-200"
        }`}
      >
        <FiCode className="h-3.5 w-3.5" />
        {t.code}
      </button>
    </div>
  );
}
