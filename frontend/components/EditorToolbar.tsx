"use client";

import { FiDownload, FiRotateCcw, FiSave } from "react-icons/fi";

type EditorToolbarProps = {
  currentPath?: string;
  dirty: boolean;
  saving: boolean;
  canSave: boolean;
  readOnly: boolean;
  description: string;
  onDescriptionChange: (value: string) => void;
  onSave: () => void;
  onRevert: () => void;
  onDownload: () => void;
};

export function EditorToolbar({
  currentPath,
  dirty,
  saving,
  canSave,
  readOnly,
  description,
  onDescriptionChange,
  onSave,
  onRevert,
  onDownload
}: EditorToolbarProps) {
  return (
    <div className="space-y-3 rounded-xl border border-slate-200 bg-white/80 p-3 dark:border-slate-800 dark:bg-slate-900/60">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-sm text-slate-600 dark:text-slate-300">
          Файл: <span className="font-semibold">{currentPath || "—"}</span>
          {dirty && <span className="ml-2 text-amber-600 dark:text-amber-300">(несохраненные изменения)</span>}
          {!dirty && <span className="ml-2 text-emerald-600 dark:text-emerald-300">(сохранено)</span>}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={onSave}
            disabled={!canSave || saving}
            className="inline-flex items-center gap-2 rounded-lg bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-500 disabled:opacity-50"
          >
            <FiSave /> {saving ? "Сохранение..." : "Сохранить"}
          </button>
          <button
            type="button"
            onClick={onRevert}
            disabled={!dirty || saving}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRotateCcw /> Откатить
          </button>
          <button
            type="button"
            onClick={onDownload}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiDownload /> Скачать
          </button>
        </div>
      </div>

      <div className="grid gap-2 md:grid-cols-[1fr_auto] md:items-center">
        <input
          value={description}
          onChange={(e) => onDescriptionChange(e.target.value)}
          disabled={readOnly}
          placeholder="Комментарий к изменению (опционально)"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        />
        {readOnly && (
          <span className="text-xs text-slate-500 dark:text-slate-400">Режим только чтение (viewer)</span>
        )}
      </div>
    </div>
  );
}
