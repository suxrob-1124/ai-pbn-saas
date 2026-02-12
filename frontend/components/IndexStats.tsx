"use client";

import { useMemo } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis
} from "recharts";
import type { IndexCheckCalendarDayDTO, IndexCheckStatsDTO } from "../types/indexChecks";

export type IndexStatsProps = {
  stats?: IndexCheckStatsDTO | null;
  daily?: IndexCheckCalendarDayDTO[];
  loading?: boolean;
  period: PeriodKey;
  onPeriodChange: (period: PeriodKey) => void;
};

export type PeriodKey = "7d" | "30d" | "90d";

type ChartPoint = {
  date: string;
  total: number;
  indexed: number;
  percent: number;
};

/** Статистика и графики для мониторинга индексации. */
export function IndexStats({ stats, daily, loading, period, onPeriodChange }: IndexStatsProps) {
  const { chartData, percentIndexed, avgAttempts, failedWeek } = useMemo(() => {
    const list = daily || [];
    const statsFallback = stats;
    const percentIndexed =
      statsFallback && typeof statsFallback.percent_indexed === "number"
        ? statsFallback.percent_indexed
        : statsFallback && statsFallback.total_resolved > 0
          ? Math.round((statsFallback.indexed_true / statsFallback.total_resolved) * 100)
          : 0;
    const avgAttempts = statsFallback?.avg_attempts_to_success
      ? Number(statsFallback.avg_attempts_to_success.toFixed(1))
      : 0;
    const failedWeek = calcFailedWeek(list);
    const chartData = buildChartData(list);
    return { chartData, percentIndexed, avgAttempts, failedWeek };
  }, [daily, stats]);

  if (loading) {
    return (
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <SkeletonCard />
        <SkeletonCard />
        <SkeletonCard />
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="font-semibold">Статистика индексации</h3>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            Данные агрегированы по дням.
          </div>
        </div>
        <div className="flex items-center gap-2">
          {(["7d", "30d", "90d"] as PeriodKey[]).map((key) => (
            <button
              key={key}
              type="button"
              onClick={() => onPeriodChange(key)}
              className={`rounded-full border px-3 py-1 text-xs font-semibold transition ${
                period === key
                  ? "border-indigo-500 bg-indigo-500/10 text-indigo-600"
                  : "border-slate-200 bg-white text-slate-600 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-900"
              }`}
            >
              {key}
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <MetricCard label={`Процент индексации (${period})`} value={`${percentIndexed}%`} />
        <MetricCard label="Среднее попыток до успеха" value={avgAttempts.toString()} />
        <MetricCard label="failed_investigation за неделю" value={failedWeek.toString()} />
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartCard title="Процент индексации по дням">
          <ResponsiveContainer width="100%" height={240}>
            <LineChart data={chartData} margin={{ top: 10, right: 20, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis domain={[0, 100]} tickFormatter={(v) => `${v}%`} />
              <Tooltip formatter={(value) => `${value}%`} />
              <Line type="monotone" dataKey="percent" stroke="#6366f1" strokeWidth={2} dot={false} />
            </LineChart>
          </ResponsiveContainer>
        </ChartCard>
        <ChartCard title="Количество проверок в день">
          <ResponsiveContainer width="100%" height={240}>
            <BarChart data={chartData} margin={{ top: 10, right: 20, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis />
              <Tooltip />
              <Bar dataKey="total" fill="#0ea5e9" radius={[6, 6, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </ChartCard>
      </div>
    </div>
  );
}

function buildChartData(days: IndexCheckCalendarDayDTO[]): ChartPoint[] {
  if (!days || days.length === 0) {
    return [];
  }
  const sorted = [...days].sort((a, b) => a.date.localeCompare(b.date));
  return sorted.map((day) => {
    const total = day.total || 0;
    const indexed = day.indexed_true || 0;
    const percent = total > 0 ? Math.round((indexed / total) * 100) : 0;
    return {
      date: day.date,
      total,
      indexed,
      percent
    };
  });
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 p-4 shadow">
      <div className="text-xs uppercase tracking-wide text-slate-400">{label}</div>
      <div className="mt-2 text-2xl font-semibold text-slate-800 dark:text-slate-100">{value}</div>
    </div>
  );
}

function ChartCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 p-4 shadow">
      <div className="text-sm font-semibold text-slate-700 dark:text-slate-100 mb-3">{title}</div>
      {children}
    </div>
  );
}

function SkeletonCard() {
  return (
    <div className="rounded-xl border border-slate-200 dark:border-slate-800 bg-white/80 dark:bg-slate-900/60 p-4 shadow">
      <div className="h-3 w-24 rounded bg-slate-200 dark:bg-slate-800" />
      <div className="mt-3 h-7 w-16 rounded bg-slate-200 dark:bg-slate-800" />
    </div>
  );
}

function parseDate(value?: string | null): Date | null {
  if (!value) return null;
  const dt = new Date(value);
  return Number.isNaN(dt.getTime()) ? null : dt;
}

function startOfDay(date: Date) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
}

function endOfDay(date: Date) {
  return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate(), 23, 59, 59));
}

function addDays(date: Date, days: number) {
  const next = new Date(date.getTime());
  next.setUTCDate(next.getUTCDate() + days);
  return next;
}

function calcFailedWeek(days: IndexCheckCalendarDayDTO[]): number {
  if (!days || days.length === 0) {
    return 0;
  }
  const now = new Date();
  const weekAgo = startOfDay(addDays(now, -6));
  const toDate = endOfDay(now);
  let count = 0;
  for (const day of days) {
    const dt = parseDate(day.date);
    if (!dt) continue;
    if (dt >= weekAgo && dt <= toDate) {
      count += day.failed_investigation || 0;
    }
  }
  return count;
}
