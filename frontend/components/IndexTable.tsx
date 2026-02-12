"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import type { IndexCheckDTO, IndexCheckStatus } from "../types/indexChecks";

export type IndexTableProps = {
  checks: IndexCheckDTO[];
  loading?: boolean;
  error?: string | null;
  onRefresh?: () => void;
  onRunNow?: (domainId: string) => Promise<void> | void;
  domainOptions?: Array<{ id: string; label?: string }>;
  showFilters?: boolean;
  pageSizeOptions?: number[];
};

const DEFAULT_PAGE_SIZE = 20;
const DEFAULT_PAGE_OPTIONS = [10, 20, 50, 100];

const STATUS_OPTIONS: IndexCheckStatus[] = [
  "pending",
  "checking",
  "success",
  "failed_investigation"
];

type SortKey =
  | "domain"
  | "date"
  | "status"
  | "attempts"
  | "isIndexed"
  | "lastAttempt"
  | "nextRetry";

type SortState = { key: SortKey; dir: "asc" | "desc" };

/** Таблица проверок индексации с фильтрами, сортировкой и пагинацией. */
export function IndexTable({
  checks,
  loading,
  error,
  onRefresh,
  onRunNow,
  domainOptions,
  showFilters = true,
  pageSizeOptions = DEFAULT_PAGE_OPTIONS
}: IndexTableProps) {
  const [statusFilters, setStatusFilters] = useState<IndexCheckStatus[]>([]);
  const [domainFilter, setDomainFilter] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");
  const [isIndexed, setIsIndexed] = useState<"all" | "true" | "false">("all");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_PAGE_SIZE);
  const [sort, setSort] = useState<SortState>({ key: "date", dir: "desc" });
  const [expandedError, setExpandedError] = useState<Record<string, boolean>>({});

  const filtered = useMemo(() => {
    const term = domainFilter.trim().toLowerCase();
    const fromKey = fromDate.trim();
    const toKey = toDate.trim();
    const statuses = new Set(statusFilters);

    return checks.filter((check) => {
      if (statuses.size > 0 && !statuses.has(check.status as IndexCheckStatus)) {
        return false;
      }
      if (term) {
        const label = (check.domain_url || check.domain_id || "").toLowerCase();
        if (!label.includes(term)) {
          return false;
        }
      }
      if (isIndexed !== "all") {
        if (isIndexed === "true" && check.is_indexed !== true) {
          return false;
        }
        if (isIndexed === "false" && check.is_indexed !== false) {
          return false;
        }
      }
      const dateKey = toDateKey(check.check_date);
      if (fromKey && dateKey && dateKey < fromKey) {
        return false;
      }
      if (toKey && dateKey && dateKey > toKey) {
        return false;
      }
      return true;
    });
  }, [checks, domainFilter, fromDate, isIndexed, statusFilters, toDate]);

  const sorted = useMemo(() => {
    const list = [...filtered];
    list.sort((a, b) => compareChecks(a, b, sort));
    return list;
  }, [filtered, sort]);

  const totalPages = Math.max(1, Math.ceil(sorted.length / pageSize));
  const safePage = Math.min(page, totalPages);
  const startIdx = (safePage - 1) * pageSize;
  const visible = sorted.slice(startIdx, startIdx + pageSize);

  const toggleStatus = (status: IndexCheckStatus) => {
    setStatusFilters((prev) => {
      const exists = prev.includes(status);
      const next = exists ? prev.filter((item) => item !== status) : [...prev, status];
      setPage(1);
      return next;
    });
  };

  const handleSort = (key: SortKey) => {
    setSort((prev) => {
      if (prev.key === key) {
        return { key, dir: prev.dir === "asc" ? "desc" : "asc" };
      }
      return { key, dir: "asc" };
    });
  };

  const resetFilters = () => {
    setStatusFilters([]);
    setDomainFilter("");
    setFromDate("");
    setToDate("");
    setIsIndexed("all");
    setPage(1);
  };

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-semibold">Проверки индексации</h3>
        <div className="flex items-center gap-2">
          {onRefresh && (
            <button
              type="button"
              onClick={onRefresh}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              Обновить
            </button>
          )}
        </div>
      </div>

      {showFilters && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-3 space-y-3">
          <div className="grid grid-cols-1 gap-3 md:grid-cols-5">
            <div className="md:col-span-2">
              <label className="text-xs text-slate-500 dark:text-slate-400">Статусы</label>
              <div className="mt-2 grid grid-cols-2 gap-2">
                {STATUS_OPTIONS.map((status) => (
                  <label
                    key={status}
                    className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs text-slate-700 dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100"
                  >
                    <input
                      type="checkbox"
                      checked={statusFilters.includes(status)}
                      onChange={() => toggleStatus(status)}
                      disabled={loading}
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
                value={fromDate}
                onChange={(e) => {
                  setFromDate(e.target.value);
                  setPage(1);
                }}
                className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
                disabled={loading}
              />
            </div>
            <div>
              <label className="text-xs text-slate-500 dark:text-slate-400">По дату</label>
              <input
                type="date"
                value={toDate}
                onChange={(e) => {
                  setToDate(e.target.value);
                  setPage(1);
                }}
                className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
                disabled={loading}
              />
            </div>
            <div>
              <label className="text-xs text-slate-500 dark:text-slate-400">В индексе</label>
              <select
                value={isIndexed}
                onChange={(e) => {
                  setIsIndexed(e.target.value as "all" | "true" | "false");
                  setPage(1);
                }}
                className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
                disabled={loading}
              >
                <option value="all">Любой</option>
                <option value="true">Да</option>
                <option value="false">Нет</option>
              </select>
            </div>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
            <div>
              <label className="text-xs text-slate-500 dark:text-slate-400">Домен</label>
              <input
                type="text"
                list="index-table-domain-options"
                value={domainFilter}
                onChange={(e) => {
                  setDomainFilter(e.target.value);
                  setPage(1);
                }}
                placeholder="example.com"
                className="mt-2 w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-950"
                disabled={loading}
              />
              {domainOptions && domainOptions.length > 0 && (
                <datalist id="index-table-domain-options">
                  {domainOptions
                    .filter((option) => option.label)
                    .map((option) => (
                      <option key={option.id} value={option.label as string}>
                        {option.label}
                      </option>
                    ))}
                </datalist>
              )}
            </div>
            <div className="flex items-end gap-2">
              <button
                type="button"
                onClick={resetFilters}
                disabled={loading}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Сбросить
              </button>
            </div>
          </div>
        </div>
      )}

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
          {error}
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-xs text-slate-500 dark:text-slate-400">
            <tr>
              <SortableTh label="Домен" sort={sort} sortKey="domain" onSort={handleSort} />
              <SortableTh label="Дата" sort={sort} sortKey="date" onSort={handleSort} />
              <SortableTh label="Статус" sort={sort} sortKey="status" onSort={handleSort} />
              <SortableTh label="Попытки" sort={sort} sortKey="attempts" onSort={handleSort} />
              <SortableTh label="В индексе" sort={sort} sortKey="isIndexed" onSort={handleSort} />
              <SortableTh label="Последняя попытка" sort={sort} sortKey="lastAttempt" onSort={handleSort} />
              <SortableTh label="Следующий ретрай" sort={sort} sortKey="nextRetry" onSort={handleSort} />
              <th className="text-left py-2">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
            {loading ? (
              <tr>
                <td colSpan={8} className="py-6 text-center text-slate-500 dark:text-slate-400">
                  Загрузка...
                </td>
              </tr>
            ) : visible.length === 0 ? (
              <tr>
                <td colSpan={8} className="py-6 text-center text-slate-500 dark:text-slate-400">
                  Нет данных
                </td>
              </tr>
            ) : (
              visible.map((check) => {
                const label = check.domain_url || "Домен";
                const errorOpen = expandedError[check.id];
                return (
                  <tr key={check.id} className="align-top">
                    <td className="py-2 pr-3">
                      <Link href={`/domains/${check.domain_id}`} className="text-indigo-600 hover:underline">
                        {label}
                      </Link>
                      {check.error_message && (
                        <div className="mt-1 text-[11px] text-red-600" title={check.error_message}>
                          {errorOpen ? check.error_message : shorten(check.error_message)}
                        </div>
                      )}
                      {check.error_message && check.error_message.length > 60 && (
                        <button
                          type="button"
                          onClick={() =>
                            setExpandedError((prev) => ({ ...prev, [check.id]: !prev[check.id] }))
                          }
                          className="mt-1 text-[11px] text-slate-500 hover:text-slate-700"
                        >
                          {errorOpen ? "Скрыть" : "Показать"}
                        </button>
                      )}
                    </td>
                    <td className="py-2 pr-3 whitespace-nowrap">{formatDate(check.check_date)}</td>
                    <td className="py-2 pr-3">{check.status}</td>
                    <td className="py-2 pr-3">{check.attempts}</td>
                    <td className="py-2 pr-3">
                      {check.is_indexed === null || check.is_indexed === undefined
                        ? "—"
                        : check.is_indexed
                          ? "Да"
                          : "Нет"}
                    </td>
                    <td className="py-2 pr-3 whitespace-nowrap">
                      {formatDateTime(check.last_attempt_at)}
                    </td>
                    <td className="py-2 pr-3 whitespace-nowrap">
                      {formatDateTime(check.next_retry_at)}
                    </td>
                    <td className="py-2 whitespace-nowrap">
                      <div className="flex flex-wrap items-center gap-2">
                        <button
                          type="button"
                          onClick={() => onRunNow?.(check.domain_id)}
                          disabled={!onRunNow || loading}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        >
                          Запустить
                        </button>
                        <Link
                          href={`/domains/${check.domain_id}`}
                          className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                        >
                          Открыть домен
                        </Link>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500 dark:text-slate-400">
        <div>
          Показано {visible.length} из {sorted.length}
        </div>
        <div className="flex items-center gap-2">
          <span>Размер страницы</span>
          <select
            value={pageSize}
            onChange={(e) => {
              setPageSize(Number(e.target.value) || DEFAULT_PAGE_SIZE);
              setPage(1);
            }}
            className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-800 dark:bg-slate-950"
          >
            {pageSizeOptions.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            disabled={safePage <= 1}
            className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            Назад
          </button>
          <span>
            Страница {safePage} из {totalPages}
          </span>
          <button
            type="button"
            onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
            disabled={safePage >= totalPages}
            className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            Вперед
          </button>
        </div>
      </div>
    </div>
  );
}

function SortableTh({
  label,
  sortKey,
  sort,
  onSort
}: {
  label: string;
  sortKey: SortKey;
  sort: SortState;
  onSort: (key: SortKey) => void;
}) {
  const active = sort.key === sortKey;
  const arrow = !active ? "" : sort.dir === "asc" ? "↑" : "↓";
  return (
    <th className="text-left py-2 pr-3">
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className="inline-flex items-center gap-1 text-xs text-slate-500 hover:text-slate-700 dark:text-slate-400"
      >
        {label} {arrow}
      </button>
    </th>
  );
}

function compareChecks(a: IndexCheckDTO, b: IndexCheckDTO, sort: SortState) {
  const dir = sort.dir === "asc" ? 1 : -1;
  switch (sort.key) {
    case "domain":
      return dir * stringCompare(a.domain_url || a.domain_id, b.domain_url || b.domain_id);
    case "date":
      return dir * numberCompare(dateValue(a.check_date), dateValue(b.check_date));
    case "status":
      return dir * stringCompare(a.status, b.status);
    case "attempts":
      return dir * numberCompare(a.attempts, b.attempts);
    case "isIndexed":
      return dir * numberCompare(indexedValue(a.is_indexed), indexedValue(b.is_indexed));
    case "lastAttempt":
      return dir * numberCompare(dateValue(a.last_attempt_at), dateValue(b.last_attempt_at));
    case "nextRetry":
      return dir * numberCompare(dateValue(a.next_retry_at), dateValue(b.next_retry_at));
    default:
      return 0;
  }
}

function dateValue(value?: string | null) {
  if (!value) return 0;
  const time = new Date(value).getTime();
  return Number.isNaN(time) ? 0 : time;
}

function indexedValue(value?: boolean | null) {
  if (value === true) return 2;
  if (value === false) return 1;
  return 0;
}

function stringCompare(a?: string | null, b?: string | null) {
  return (a || "").localeCompare(b || "");
}

function numberCompare(a: number, b: number) {
  if (a === b) return 0;
  return a > b ? 1 : -1;
}

function toDateKey(value?: string | null): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return date.toISOString().slice(0, 10);
}

function formatDate(value?: string | null) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString();
}

function formatDateTime(value?: string | null) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function shorten(text: string, max = 60) {
  if (text.length <= max) return text;
  return `${text.slice(0, max)}...`;
}
