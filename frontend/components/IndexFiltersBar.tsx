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
    onApply({ ...draft, statuses: normalizeIndexCheckStatusList(draft.statuses) });
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
                {getIndexCheckStatusMeta(status).label}
              </label>
            ))}
          </div>
        </div>
        <FilterDateInput
          label="С даты"
          value={draft.from}
          disabled={disabled}
          onChange={(value) => setDraft((prev) => ({ ...prev, from: value }))}
        />
        <FilterDateInput
          label="По дату"
          value={draft.to}
          disabled={disabled}
          onChange={(value) => setDraft((prev) => ({ ...prev, to: value }))}
        />
        <FilterSelect
          label="В индексе"
          value={draft.isIndexed}
          disabled={disabled}
          options={[...INDEXED_OPTIONS]}
          onChange={(value) => setDraft((prev) => ({ ...prev, isIndexed: value as "all" | "true" | "false" }))}
        />
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
        <FilterSearchInput
          label="Поиск (URL)"
          value={draft.search}
          placeholder="example.com"
          disabled={disabled}
          onChange={(next) => {
            setDraft((prev) => ({ ...prev, search: next }));
            onSearchChange?.(next);
          }}
        />
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
