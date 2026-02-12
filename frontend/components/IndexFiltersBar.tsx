"use client";

import { useEffect, useMemo, useState } from "react";
import type { IndexCheckStatus } from "../types/indexChecks";

export type IndexFiltersValue = {
  statuses: IndexCheckStatus[];
  from: string;
  to: string;
  domainId: string;
  isIndexed: "all" | "true" | "false";
  search: string;
};

export type IndexFiltersBarProps = {
  value: IndexFiltersValue;
  defaultValue?: IndexFiltersValue;
  onApply: (value: IndexFiltersValue) => void;
  onReset: (value: IndexFiltersValue) => void;
  onRefresh: () => void;
  onSearchChange?: (value: string) => void;
  domainOptions?: Array<{ id: string; label?: string }>;
  showDomain?: boolean;
  disabled?: boolean;
};

const DEFAULT_STATUS_OPTIONS: IndexCheckStatus[] = [
  "pending",
  "checking",
  "success",
  "failed_investigation"
];

const buildDefaultValue = (): IndexFiltersValue => ({
  statuses: [],
  from: "",
  to: "",
  domainId: "",
  isIndexed: "all",
  search: ""
});

/** Панель фильтров для мониторинга индексации. */
export function IndexFiltersBar({
  value,
  defaultValue,
  onApply,
  onReset,
  onRefresh,
  onSearchChange,
  disabled
}: IndexFiltersBarProps) {
  const resolvedDefault = defaultValue ?? buildDefaultValue();
  const [draft, setDraft] = useState<IndexFiltersValue>(value);

  useEffect(() => {
    setDraft(value);
  }, [value.domainId, value.from, value.isIndexed, value.search, value.to, value.statuses.join("|")]);

  const dirty = useMemo(() => !isSameValue(draft, value), [draft, value]);

  const toggleStatus = (status: IndexCheckStatus) => {
    setDraft((prev) => {
      const exists = prev.statuses.includes(status);
      const nextStatuses = exists
        ? prev.statuses.filter((item) => item !== status)
        : [...prev.statuses, status];
      return { ...prev, statuses: nextStatuses };
    });
  };

  const handleApply = () => {
    if (disabled) {
      return;
    }
    onApply({ ...draft, statuses: normalizeStatuses(draft.statuses) });
  };

  const handleReset = () => {
    if (disabled) {
      return;
    }
    setDraft(resolvedDefault);
    onReset(resolvedDefault);
  };

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="grid grid-cols-1 gap-3 md:grid-cols-6">
        <div className="md:col-span-2">
          <label className="text-xs text-slate-500 dark:text-slate-400">Статус</label>
          <div className="mt-2 grid grid-cols-2 gap-2">
            {DEFAULT_STATUS_OPTIONS.map((status) => (
              <label
                key={status}
                className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
              >
                <input
                  type="checkbox"
                  checked={draft.statuses.includes(status)}
                  onChange={() => toggleStatus(status)}
                  disabled={disabled}
                />
                {status}
              </label>
            ))}
          </div>
        </div>
        <div>
          <label className="text-xs text-slate-500 dark:text-slate-400">С даты</label>
          <input
            type="date"
            value={draft.from}
            onChange={(e) => setDraft((prev) => ({ ...prev, from: e.target.value }))}
            className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
            disabled={disabled}
          />
        </div>
        <div>
          <label className="text-xs text-slate-500 dark:text-slate-400">По дату</label>
          <input
            type="date"
            value={draft.to}
            onChange={(e) => setDraft((prev) => ({ ...prev, to: e.target.value }))}
            className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
            disabled={disabled}
          />
        </div>
        <div>
          <label className="text-xs text-slate-500 dark:text-slate-400">В индексе</label>
          <select
            value={draft.isIndexed}
            onChange={(e) => setDraft((prev) => ({ ...prev, isIndexed: e.target.value as "all" | "true" | "false" }))}
            className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
            disabled={disabled}
          >
            <option value="all">Любой</option>
            <option value="true">Да</option>
            <option value="false">Нет</option>
          </select>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <div>
          <label className="text-xs text-slate-500 dark:text-slate-400">Поиск (URL)</label>
          <input
            type="text"
            value={draft.search}
            onChange={(e) => {
              const next = e.target.value;
              setDraft((prev) => ({ ...prev, search: next }));
              onSearchChange?.(next);
            }}
            placeholder="example.com"
            className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
            disabled={disabled}
          />
        </div>
        <div className="flex items-end gap-2">
          <button
            type="button"
            onClick={handleApply}
            className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
            disabled={disabled || !dirty}
          >
            Применить
          </button>
          <button
            type="button"
            onClick={handleReset}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            disabled={disabled || !dirty}
          >
            Сбросить
          </button>
          <button
            type="button"
            onClick={onRefresh}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            disabled={disabled}
          >
            Обновить
          </button>
        </div>
      </div>
    </div>
  );
}

function normalizeStatuses(statuses: IndexCheckStatus[]): IndexCheckStatus[] {
  const trimmed = statuses.map((item) => item.trim()).filter(Boolean);
  return Array.from(new Set(trimmed));
}

function isSameValue(a: IndexFiltersValue, b: IndexFiltersValue): boolean {
  return (
    a.from === b.from &&
    a.to === b.to &&
    a.domainId === b.domainId &&
    a.isIndexed === b.isIndexed &&
    a.search === b.search &&
    sameStatuses(a.statuses, b.statuses)
  );
}

function sameStatuses(a: IndexCheckStatus[], b: IndexCheckStatus[]): boolean {
  if (a.length !== b.length) return false;
  const aset = new Set(a.map((item) => item.trim()));
  for (const item of b) {
    if (!aset.has(item.trim())) return false;
  }
  return true;
}
