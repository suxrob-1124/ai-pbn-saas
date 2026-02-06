"use client";

import { useMemo, useRef, useState } from "react";
import type { DragEventHandler } from "react";
import type { LinkTaskImportItem } from "../types/linkTasks";

type CSVImportProps = {
  loading: boolean;
  error?: string | null;
  onImport: (items: LinkTaskImportItem[]) => void;
};

type ParsedRow = {
  line: number;
  anchorText: string;
  targetUrl: string;
  scheduledFor?: string;
  valid: boolean;
  error?: string;
};

const isValidUrl = (value: string) => {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
};

const toISOIfValid = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = new Date(trimmed);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }
  return parsed.toISOString();
};

const parseCsv = (raw: string): ParsedRow[] => {
  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  const rows: ParsedRow[] = [];
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    if (i === 0 && /anchor_text/i.test(line) && /target_url/i.test(line)) {
      continue;
    }
    const parts = line.split(",");
    const anchorText = (parts[0] || "").trim();
    const targetUrl = (parts[1] || "").trim();
    const scheduledRaw = parts.slice(2).join(",").trim();
    let error: string | undefined;
    if (!anchorText || !targetUrl) {
      error = "anchor_text и target_url обязательны";
    } else if (!isValidUrl(targetUrl)) {
      error = "URL должен начинаться с http:// или https://";
    }
    const scheduledFor = scheduledRaw ? toISOIfValid(scheduledRaw) : undefined;
    if (scheduledRaw && scheduledFor === null) {
      error = "Неверный формат scheduled_for";
    }
    rows.push({
      line: i + 1,
      anchorText,
      targetUrl,
      scheduledFor: scheduledFor ?? undefined,
      valid: !error,
      error
    });
  }
  return rows;
};

export function CSVImport({ loading, error, onImport }: CSVImportProps) {
  const [text, setText] = useState("");
  const [dragActive, setDragActive] = useState(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const rows = useMemo(() => parseCsv(text), [text]);
  const validRows = rows.filter((row) => row.valid);
  const invalidRows = rows.filter((row) => !row.valid);

  const applyText = (value: string) => {
    setText(value);
  };

  const readFile = async (file: File) => {
    const content = await file.text();
    applyText(content);
  };

  const handleFiles = (files: FileList | null) => {
    if (!files || files.length === 0) {
      return;
    }
    void readFile(files[0]);
  };

  const handleDrop: DragEventHandler<HTMLDivElement> = (event) => {
    event.preventDefault();
    event.stopPropagation();
    setDragActive(false);
    handleFiles(event.dataTransfer.files);
  };

  const handleDragOver: DragEventHandler<HTMLDivElement> = (event) => {
    event.preventDefault();
    event.stopPropagation();
    setDragActive(true);
  };

  const handleDragLeave: DragEventHandler<HTMLDivElement> = (event) => {
    event.preventDefault();
    event.stopPropagation();
    setDragActive(false);
  };

  const openFilePicker = () => {
    fileInputRef.current?.click();
  };

  const handleSubmit = () => {
    if (loading || validRows.length === 0) {
      return;
    }
    const items: LinkTaskImportItem[] = validRows.map((row) => ({
      anchorText: row.anchorText,
      targetUrl: row.targetUrl,
      scheduledFor: row.scheduledFor
    }));
    onImport(items);
  };

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">CSV импорт</h3>
        <button
          onClick={() => applyText("")}
          disabled={loading || text.length === 0}
          className="text-xs text-slate-500 hover:text-slate-700 disabled:opacity-50"
        >
          Очистить
        </button>
      </div>
      {error && <div className="text-sm text-red-500">{error}</div>}
      <div
        className={`cursor-pointer rounded-lg border-2 border-dashed p-4 text-sm transition ${
          dragActive
            ? "border-indigo-400 bg-indigo-50 text-indigo-700 dark:border-indigo-500 dark:bg-indigo-500/10 dark:text-indigo-200"
            : "border-slate-200 bg-white text-slate-500 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-400"
        }`}
        onClick={openFilePicker}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        role="button"
        tabIndex={0}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            openFilePicker();
          }
        }}
      >
        Перетащите CSV файл сюда или нажмите для выбора.
        <div className="mt-1 text-xs text-slate-400 dark:text-slate-500">
          Формат: anchor_text,target_url,scheduled_for
        </div>
      </div>
      <input
        ref={fileInputRef}
        type="file"
        accept=".csv,text/csv"
        className="hidden"
        onChange={(event) => handleFiles(event.target.files)}
      />
      <textarea
        className="w-full min-h-[120px] rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        placeholder="anchor_text,target_url,scheduled_for"
        value={text}
        onChange={(event) => applyText(event.target.value)}
      />
      <div className="text-xs text-slate-500 dark:text-slate-400">
        Валидных: {validRows.length} · Ошибок: {invalidRows.length}
      </div>
      <div className="rounded-lg border border-slate-200 dark:border-slate-800">
        <div className="px-3 py-2 text-xs font-semibold text-slate-600 dark:text-slate-300 border-b border-slate-200 dark:border-slate-800">
          Предпросмотр
        </div>
        {rows.length === 0 ? (
          <div className="px-3 py-3 text-xs text-slate-400">Нет данных для просмотра.</div>
        ) : (
          <div className="max-h-48 overflow-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-slate-500 dark:text-slate-400">
                  <th className="py-1 px-3">#</th>
                  <th className="py-1 px-3">Анкор</th>
                  <th className="py-1 px-3">URL</th>
                  <th className="py-1 px-3">Запланировано</th>
                  <th className="py-1 px-3">Статус</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {rows.map((row) => (
                  <tr key={`${row.line}-${row.anchorText}`}>
                    <td className="py-1 px-3 text-slate-400">{row.line}</td>
                    <td className="py-1 px-3">{row.anchorText || "—"}</td>
                    <td className="py-1 px-3">{row.targetUrl || "—"}</td>
                    <td className="py-1 px-3">{row.scheduledFor || "—"}</td>
                    <td className="py-1 px-3">
                      {row.valid ? (
                        <span className="text-emerald-600">ОК</span>
                      ) : (
                        <span className="text-red-500">{row.error}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
      <button
        onClick={handleSubmit}
        disabled={loading || validRows.length === 0}
        className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
      >
        Импортировать все
      </button>
    </div>
  );
}
