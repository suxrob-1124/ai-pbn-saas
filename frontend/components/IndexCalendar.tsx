"use client";

import type { IndexCheckDTO } from "../types/indexChecks";

export type IndexCalendarProps = {
  checks: IndexCheckDTO[];
  selectedDate?: string;
  onSelectDate?: (date: string) => void;
  baseDate?: string;
  loading?: boolean;
};

type DaySummary = {
  total: number;
  indexedTrue: number;
  indexedFalse: number;
  pending: number;
  failed: number;
  domains: string[];
};

const WEEK_DAYS = ["Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"];

/** Календарь проверок индексации с индикаторами и подсказкой. */
export function IndexCalendar({
  checks,
  selectedDate,
  onSelectDate,
  baseDate,
  loading
}: IndexCalendarProps) {
  const reference = baseDate ? new Date(baseDate) : new Date();
  const month = reference.getMonth();
  const year = reference.getFullYear();

  const firstDay = new Date(year, month, 1);
  const startOffset = (firstDay.getDay() + 6) % 7;
  const daysInMonth = new Date(year, month + 1, 0).getDate();

  const summaries = buildSummaries(checks, year, month);

  const cells = Array.from({ length: startOffset + daysInMonth }, (_, index) => {
    if (index < startOffset) {
      return null;
    }
    const day = index - startOffset + 1;
    const key = dayKey(year, month, day);
    return { day, key, summary: summaries[key] };
  });

  return (
    <div className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 p-4 shadow">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">Календарь проверок</h3>
        <span className="text-xs text-slate-500 dark:text-slate-400">
          {reference.toLocaleString("ru-RU", { month: "long", year: "numeric" })}
        </span>
      </div>
      <div className="mt-3 grid grid-cols-7 gap-2 text-center text-xs text-slate-500 dark:text-slate-400">
        {WEEK_DAYS.map((day) => (
          <div key={day}>{day}</div>
        ))}
      </div>
      <div className="mt-2 grid grid-cols-7 gap-2">
        {cells.map((cell, idx) => {
          if (!cell) {
            return <div key={`empty-${idx}`} className="h-10" />;
          }
          const summary = cell.summary;
          const tone = summary ? toneForSummary(summary) : "slate";
          const isSelected = selectedDate === cell.key;
          const tooltip = summary ? buildTooltip(cell.key, summary) : undefined;
          return (
            <button
              key={`day-${cell.day}`}
              type="button"
              className={`h-10 rounded-lg border border-slate-200 dark:border-slate-800 flex flex-col items-center justify-center text-xs font-semibold transition ${
                loading ? "bg-slate-100 dark:bg-slate-800/50" : "bg-white/70 dark:bg-slate-950/40"
              } ${isSelected ? "ring-2 ring-indigo-400" : ""}`}
              onClick={() => onSelectDate?.(cell.key)}
              title={tooltip}
              disabled={loading}
            >
              <span className="text-slate-700 dark:text-slate-200">{cell.day}</span>
              {summary && <span className={`mt-0.5 h-1.5 w-6 rounded-full ${toneClass(tone)}`} />}
            </button>
          );
        })}
      </div>
      {loading && (
        <div className="mt-3 text-xs text-slate-500 dark:text-slate-400">Загрузка календаря...</div>
      )}
    </div>
  );
}

function dayKey(year: number, month: number, day: number) {
  return `${year}-${String(month + 1).padStart(2, "0")}-${String(day).padStart(2, "0")}`;
}

function buildSummaries(checks: IndexCheckDTO[], year: number, month: number) {
  const summaries: Record<string, DaySummary> = {};
  for (const check of checks) {
    const dateKey = toDateKey(check.check_date);
    if (!dateKey) {
      continue;
    }
    const date = new Date(check.check_date);
    if (Number.isNaN(date.getTime())) {
      continue;
    }
    if (date.getFullYear() !== year || date.getMonth() !== month) {
      continue;
    }
    if (!summaries[dateKey]) {
      summaries[dateKey] = {
        total: 0,
        indexedTrue: 0,
        indexedFalse: 0,
        pending: 0,
        failed: 0,
        domains: []
      };
    }
    const summary = summaries[dateKey];
    summary.total += 1;
    if (check.status === "failed_investigation") {
      summary.failed += 1;
    } else if (check.status === "checking" || check.status === "pending") {
      summary.pending += 1;
    } else if (check.status === "success") {
      if (check.is_indexed === true) {
        summary.indexedTrue += 1;
      } else if (check.is_indexed === false) {
        summary.indexedFalse += 1;
      }
    }
    const label = check.domain_url || check.domain_id;
    if (label && summary.domains.length < 4 && !summary.domains.includes(label)) {
      summary.domains.push(label);
    }
  }
  return summaries;
}

function toneForSummary(summary: DaySummary) {
  if (summary.failed > 0) return "gray";
  if (summary.pending > 0) return "yellow";
  if (summary.indexedFalse > 0) return "red";
  if (summary.indexedTrue > 0) return "green";
  return "slate";
}

function toneClass(tone: "green" | "red" | "yellow" | "gray" | "slate") {
  switch (tone) {
    case "green":
      return "bg-emerald-500/70";
    case "red":
      return "bg-red-500/70";
    case "yellow":
      return "bg-amber-500/70";
    case "gray":
      return "bg-slate-400/70";
    default:
      return "bg-slate-300/70 dark:bg-slate-700/70";
  }
}

function buildTooltip(dateKey: string, summary: DaySummary) {
  const lines = [
    `Дата: ${dateKey}`,
    `Всего: ${summary.total}`,
    `В индексе: ${summary.indexedTrue}`,
    `Не в индексе: ${summary.indexedFalse}`,
    `В работе: ${summary.pending}`,
    `Проблемы: ${summary.failed}`
  ];
  if (summary.domains.length > 0) {
    lines.push(`Домены: ${summary.domains.join(", ")}`);
  }
  return lines.join("\n");
}

function toDateKey(value?: string | null): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toISOString().slice(0, 10);
}
