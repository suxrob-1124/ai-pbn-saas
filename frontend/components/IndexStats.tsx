"use client";

import { useMemo, useState } from "react";
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
import type { IndexCheckDTO } from "../types/indexChecks";

export type IndexStatsProps = {
  checks: IndexCheckDTO[];
  loading?: boolean;
};

type PeriodKey = "7d" | "30d" | "90d";

type ChartPoint = {
  date: string;
  total: number;
  indexed: number;
  percent: number;
};

/** Статистика и графики для мониторинга индексации. */
export function IndexStats({ checks, loading }: IndexStatsProps) {
  const [period, setPeriod] = useState<PeriodKey>("30d");

  const { chartData, percentIndexed, avgAttempts, failedWeek } = useMemo(() => {
    const days = periodToDays(period);
    const now = new Date();
    const fromDate = startOfDay(addDays(now, -days + 1));
    const toDate = endOfDay(now);

    const filtered = checks.filter((check) => {
      const dt = parseDate(check.check_date);
      if (!dt) return false;
      return dt >= fromDate && dt <= toDate;
    });

    const stats = buildChartData(filtered, fromDate, days);
    const resolved = filtered.filter((check) => check.status === "success" && check.is_indexed !== null);
    const totalResolved = resolved.length;
    const indexedTrue = resolved.filter((check) => check.is_indexed === true).length;
    const percentIndexed = totalResolved > 0 ? Math.round((indexedTrue / totalResolved) * 100) : 0;

    const successAttempts = filtered
      .filter((check) => check.status === "success")
      .map((check) => check.attempts)
      .filter((value) => typeof value === "number");
    const avgAttempts = successAttempts.length
      ? Number((successAttempts.reduce((a, b) => a + b, 0) / successAttempts.length).toFixed(1))
      : 0;

    const weekAgo = startOfDay(addDays(now, -6));
    const failedWeek = checks.filter((check) => {
      const dt = parseDate(check.check_date);
      return dt && dt >= weekAgo && dt <= toDate && check.status === "failed_investigation";
    }).length;

    return { chartData: stats, percentIndexed, avgAttempts, failedWeek };
  }, [checks, period]);

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
        <h3 className="font-semibold">Index Stats</h3>
        <div className="flex items-center gap-2">
          {(["7d", "30d", "90d"] as PeriodKey[]).map((key) => (
            <button
              key={key}
              type="button"
              onClick={() => setPeriod(key)}
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
        <MetricCard label="Процент индексации (30д)" value={`${percentIndexed}%`} />
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

function buildChartData(checks: IndexCheckDTO[], startDate: Date, days: number): ChartPoint[] {
  const map = new Map<string, ChartPoint>();
  for (let i = 0; i < days; i++) {
    const day = addDays(startDate, i);
    const key = day.toISOString().slice(0, 10);
    map.set(key, { date: key, total: 0, indexed: 0, percent: 0 });
  }
  for (const check of checks) {
    const key = toDateKey(check.check_date);
    if (!key || !map.has(key)) {
      continue;
    }
    const entry = map.get(key)!;
    entry.total += 1;
    if (check.status === "success" && check.is_indexed === true) {
      entry.indexed += 1;
    }
  }
  map.forEach((entry) => {
    entry.percent = entry.total > 0 ? Math.round((entry.indexed / entry.total) * 100) : 0;
  });
  return Array.from(map.values());
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

function periodToDays(period: PeriodKey) {
  switch (period) {
    case "7d":
      return 7;
    case "90d":
      return 90;
    default:
      return 30;
  }
}

function parseDate(value?: string | null): Date | null {
  if (!value) return null;
  const dt = new Date(value);
  return Number.isNaN(dt.getTime()) ? null : dt;
}

function toDateKey(value?: string | null): string {
  if (!value) return "";
  const dt = new Date(value);
  if (Number.isNaN(dt.getTime())) return "";
  return dt.toISOString().slice(0, 10);
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
