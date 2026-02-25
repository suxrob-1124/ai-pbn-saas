"use client";

import { FiAlertTriangle, FiRefreshCw } from "react-icons/fi";
import type { IndexCheckDTO } from "../types/indexChecks";
import { getIndexCheckStatusMeta } from "../features/queue-monitoring/services/statusMeta";

export type FailedChecksAlertProps = {
  checks?: IndexCheckDTO[];
  failedCount?: number;
  loading?: boolean;
  error?: string | null;
  onRefresh?: () => void;
  onViewDetails?: () => void;
};

/** Алерт для проблемных проверок индексации. */
export function FailedChecksAlert({
  checks,
  failedCount,
  loading,
  error,
  onRefresh,
  onViewDetails
}: FailedChecksAlertProps) {
  const failedMeta = getIndexCheckStatusMeta("failed_investigation");

  if (loading) {
    return (
      <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-200">
        Загрузка проблемных проверок...
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
        {error}
      </div>
    );
  }

  const count = typeof failedCount === "number" ? failedCount : checks?.length || 0;
  if (!count) {
    return null;
  }

  return (
    <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-200">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 font-semibold">
          <FiAlertTriangle /> {failedMeta.label}: {count}
        </div>
        <div className="flex items-center gap-2">
          {onViewDetails && (
            <button
              type="button"
              onClick={onViewDetails}
              className="inline-flex items-center gap-1 rounded-full border border-amber-300 bg-white/60 px-2 py-0.5 text-[11px] font-semibold text-amber-800 hover:bg-white"
            >
              Показать детали
            </button>
          )}
          {onRefresh && (
            <button
              type="button"
              onClick={onRefresh}
              className="inline-flex items-center gap-1 rounded-full border border-amber-300 bg-white/60 px-2 py-0.5 text-[11px] font-semibold text-amber-800 hover:bg-white"
            >
              <FiRefreshCw /> Обновить
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
