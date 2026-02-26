"use client";

import { Fragment } from "react";
import Link from "next/link";
import { FiChevronDown, FiChevronUp } from "react-icons/fi";
import type { IndexCheckDTO, IndexCheckHistoryDTO } from "../../types/indexChecks";
import { IndexCheckHistoryCard } from "../IndexCheckHistoryCard";
import { Badge } from "../Badge";
import type { ActionGuard } from "../../features/queue-monitoring/services/actionGuards";
import { getIndexCheckStatusMeta } from "../../features/queue-monitoring/services/statusMeta";
import { TableStateRow } from "../../features/queue-monitoring/components/TableState";

export type IndexCheckSortKey =
  | "domain"
  | "check_date"
  | "status"
  | "attempts"
  | "is_indexed"
  | "last_attempt_at"
  | "next_retry_at";

export type IndexCheckSort = {
  key: IndexCheckSortKey;
  dir: "asc" | "desc";
};

export type IndexTableProps = {
  checks: IndexCheckDTO[];
  loading?: boolean;
  history: Record<string, IndexCheckHistoryDTO[]>;
  historyLoading: Record<string, boolean>;
  historyError: Record<string, string | null>;
  openHistory: Record<string, boolean>;
  onToggleHistory: (checkId: string) => void;
  formatDate: (value?: string | null) => string;
  formatDateTime: (value?: string | null) => string;
  sort: IndexCheckSort;
  onSortChange?: (sort: IndexCheckSort) => void;
  onRunNow?: (domainId: string) => void;
  runNowGuard?: (domainId: string) => ActionGuard;
};

/** Таблица проверок индексации с поддержкой истории. */
export function IndexTable({
  checks,
  loading,
  history,
  historyLoading,
  historyError,
  openHistory,
  onToggleHistory,
  formatDate,
  formatDateTime,
  sort,
  onSortChange,
  onRunNow,
  runNowGuard
}: IndexTableProps) {
  const handleSort = (key: IndexCheckSortKey) => {
    if (!onSortChange) {
      return;
    }
    if (sort.key === key) {
      onSortChange({ key, dir: sort.dir === "asc" ? "desc" : "asc" });
      return;
    }
    onSortChange({ key, dir: defaultDirForKey(key) });
  };

  return (
    <div className="space-y-3">
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-xs text-slate-500 dark:text-slate-400">
            <tr>
              <SortableTh label="Домен" sort={sort} sortKey="domain" onSort={handleSort} />
              <SortableTh label="Дата" sort={sort} sortKey="check_date" onSort={handleSort} />
              <SortableTh label="Статус" sort={sort} sortKey="status" onSort={handleSort} />
              <SortableTh label="Попытки" sort={sort} sortKey="attempts" onSort={handleSort} />
              <SortableTh label="В индексе" sort={sort} sortKey="is_indexed" onSort={handleSort} />
              <SortableTh label="Последняя попытка" sort={sort} sortKey="last_attempt_at" onSort={handleSort} />
              <SortableTh label="Следующий ретрай" sort={sort} sortKey="next_retry_at" onSort={handleSort} />
              <th className="text-left py-2 pr-3">Действия</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
            {loading || checks.length === 0 ? (
              <TableStateRow
                colSpan={8}
                loading={Boolean(loading)}
                empty={!loading && checks.length === 0}
                loadingText="Загрузка..."
                emptyText="Нет данных"
              />
            ) : (
              checks.map((check) => {
                const isOpen = Boolean(openHistory[check.id]);
                const historyList = history[check.id] || [];
                const historyIsLoading = Boolean(historyLoading[check.id]);
                const historyErr = historyError[check.id];
                const statusMeta = getIndexCheckStatusMeta(check.status);
                return (
                  <Fragment key={check.id}>
                    <tr className="align-top">
                      <td className="py-2 pr-3">
                        <Link href={`/domains/${check.domain_id}`} className="text-indigo-600 hover:underline">
                          {check.domain_url || "Домен"}
                        </Link>
                        {check.error_message && (
                          <div className="mt-1 text-[11px] text-red-600" title={check.error_message}>
                            {check.error_message}
                          </div>
                        )}
                      </td>
                      <td className="py-2 pr-3 whitespace-nowrap">{formatDate(check.check_date)}</td>
                      <td className="py-2 pr-3">
                        <span title={String(check.status || "")}>
                          <Badge label={statusMeta.label} tone={statusMeta.tone} className="text-xs" />
                        </span>
                      </td>
                      <td className="py-2 pr-3">{check.attempts}</td>
                      <td className="py-2 pr-3">
                        {check.is_indexed === null || check.is_indexed === undefined
                          ? "—"
                          : check.is_indexed
                            ? "Да"
                            : "Нет"}
                      </td>
                      <td className="py-2 pr-3 whitespace-nowrap">{formatDateTime(check.last_attempt_at)}</td>
                      <td className="py-2 pr-3 whitespace-nowrap">{formatDateTime(check.next_retry_at)}</td>
                      <td className="py-2 pr-3 whitespace-nowrap">
                        <div className="flex flex-wrap items-center gap-2">
                          {onRunNow && (
                            <button
                              type="button"
                              onClick={() => onRunNow(check.domain_id)}
                              disabled={runNowGuard?.(check.domain_id).disabled}
                              title={runNowGuard?.(check.domain_id).reason}
                              className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                            >
                              Запустить
                            </button>
                          )}
                          <button
                            type="button"
                            onClick={() => onToggleHistory(check.id)}
                            className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                          >
                            {isOpen ? <FiChevronUp /> : <FiChevronDown />}
                            {isOpen ? "Скрыть" : "История"}
                          </button>
                        </div>
                      </td>
                    </tr>
                    {isOpen && (
                      <tr>
                        <td colSpan={8} className="pb-4">
                          {historyIsLoading ? (
                            <div className="text-xs text-slate-500 dark:text-slate-400">
                              Загрузка истории...
                            </div>
                          ) : historyErr ? (
                            <div className="text-xs text-red-600 dark:text-red-300">{historyErr}</div>
                          ) : historyList.length === 0 ? (
                            <div className="text-xs text-slate-500 dark:text-slate-400">
                              История пустая
                            </div>
                          ) : (
                            <div className="grid gap-2">
                              {historyList.map((item) => (
                                <IndexCheckHistoryCard
                                  key={item.id}
                                  item={item}
                                  formatDateTime={formatDateTime}
                                />
                              ))}
                            </div>
                          )}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })
            )}
          </tbody>
        </table>
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
  sortKey: IndexCheckSortKey;
  sort: IndexCheckSort;
  onSort: (key: IndexCheckSortKey) => void;
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

function defaultDirForKey(key: IndexCheckSortKey): "asc" | "desc" {
  switch (key) {
    case "check_date":
    case "last_attempt_at":
    case "next_retry_at":
      return "desc";
    default:
      return "asc";
  }
}
