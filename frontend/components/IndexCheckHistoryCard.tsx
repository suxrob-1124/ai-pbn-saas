"use client";

import type { IndexCheckHistoryDTO } from "../types/indexChecks";
import { Badge } from "./Badge";

export type IndexCheckHistoryCardProps = {
  item: IndexCheckHistoryDTO;
  formatDateTime: (value?: string | null) => string;
};

/** Карточка истории попытки проверки индексации. */
export function IndexCheckHistoryCard({ item, formatDateTime }: IndexCheckHistoryCardProps) {
  const resultTone = item.result === "success" ? "green" : "amber";
  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-white/60 dark:bg-slate-900/40 p-3">
      <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
        <Badge label={item.result || "—"} tone={resultTone} />
        <span>Попытка: {item.attempt_number}</span>
        <span>Длительность: {item.duration_ms ?? "—"} ms</span>
        <span>{formatDateTime(item.created_at)}</span>
      </div>
      {item.error_message && (
        <div className="mt-1 text-xs text-red-600 dark:text-red-300">{item.error_message}</div>
      )}
    </div>
  );
}
