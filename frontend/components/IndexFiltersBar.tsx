"use client";

import { useEffect, useMemo, useState } from "react";
import type { IndexCheckStatus } from "../types/indexChecks";
import {
  INDEX_CHECK_STATUS_KEYS,
  getIndexCheckStatusMeta,
  normalizeIndexCheckStatusList
} from "../features/queue-monitoring/services/statusMeta";
import { FilterDateInput } from "../features/queue-monitoring/components/FilterDateInput";
import { FilterSearchInput } from "../features/queue-monitoring/components/FilterSearchInput";
import { FilterSelect } from "../features/queue-monitoring/components/FilterSelect";

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

const DEFAULT_STATUS_OPTIONS: IndexCheckStatus[] = [...INDEX_CHECK_STATUS_KEYS];
const INDEXED_OPTIONS = [
  { value: "all", label: "Любой" },
  { value: "true", label: "Да" },
  { value: "false", label: "Нет" }
] as const;

const buildDefaultValue = (): IndexFiltersValue => ({
  statuses: [],
  from: "",
  to: "",
  domainId: "",
  isIndexed: "all",
  search: ""
});

/** Панель фильтров для мониторинга индексации. Без карточки — размещается внутри родительского контейнера. */
export function IndexFiltersBar({
  value,
  defaultValue,
  onApply,
  onReset,
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
    if (disabled) return;
    onApply({ ...draft, statuses: normalizeIndexCheckStatusList(draft.statuses) });
  };

  const handleReset = () => {
    if (disabled) return;
    setDraft(resolvedDefault);
    onReset(resolvedDefault);
  };

  return (
    <div className="space-y-3">
      {/* ── Inputs row ──────────────────────────────────────────────── */}
      <div className="flex flex-wrap items-end gap-3">
        <div className="flex flex-col gap-1 min-w-[150px]">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">С даты</span>
          <FilterDateInput
            value={draft.from}
            disabled={disabled}
            onChange={(v) => setDraft((p) => ({ ...p, from: v }))}
          />
        </div>
        <div className="flex flex-col gap-1 min-w-[150px]">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">По дату</span>
          <FilterDateInput
            value={draft.to}
            disabled={disabled}
            onChange={(v) => setDraft((p) => ({ ...p, to: v }))}
          />
        </div>
        <div className="flex flex-col gap-1 min-w-[140px]">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">В индексе</span>
          <FilterSelect
            value={draft.isIndexed}
            disabled={disabled}
            options={[...INDEXED_OPTIONS]}
            onChange={(v) => setDraft((p) => ({ ...p, isIndexed: v as "all" | "true" | "false" }))}
          />
        </div>
        <div className="flex flex-col gap-1 flex-1 min-w-[200px]">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Поиск (URL)</span>
          <FilterSearchInput
            value={draft.search}
            placeholder="example.com"
            disabled={disabled}
            onChange={(next) => {
              setDraft((p) => ({ ...p, search: next }));
              onSearchChange?.(next);
            }}
          />
        </div>
      </div>

      {/* ── Status chips ────────────────────────────────────────────── */}
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-xs font-medium text-slate-500 dark:text-slate-400 flex-shrink-0">Статус:</span>
        {DEFAULT_STATUS_OPTIONS.map((status) => {
          const checked = draft.statuses.includes(status);
          return (
            <label
              key={status}
              className={[
                "flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs cursor-pointer select-none transition-colors",
                checked
                  ? "border-indigo-300 bg-indigo-50 text-indigo-700 dark:border-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300"
                  : "border-slate-200 bg-white text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-400 dark:hover:bg-slate-800",
              ].join(" ")}
            >
              <input
                type="checkbox"
                checked={checked}
                onChange={() => toggleStatus(status)}
                disabled={disabled}
                className="sr-only"
              />
              {checked && (
                <span className="w-1.5 h-1.5 rounded-full bg-indigo-500 dark:bg-indigo-400 flex-shrink-0" />
              )}
              {getIndexCheckStatusMeta(status).label}
            </label>
          );
        })}
      </div>

      {/* ── Action buttons ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2 pt-0.5">
        <button
          type="button"
          onClick={handleApply}
          disabled={disabled || !dirty}
          className="inline-flex items-center gap-1.5 rounded-lg bg-indigo-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-indigo-500 disabled:opacity-40 transition-colors"
        >
          Применить
        </button>
        <button
          type="button"
          onClick={handleReset}
          disabled={disabled || !dirty}
          className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-40 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 transition-colors"
        >
          Сбросить
        </button>
      </div>
    </div>
  );
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
