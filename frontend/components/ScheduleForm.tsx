"use client";

import { useEffect, useState } from "react";
import {
  buildScheduleConfig,
  ScheduleFormValue
} from "../lib/scheduleFormValidation";

type ScheduleFormProps = {
  value: ScheduleFormValue;
  loading: boolean;
  error?: string | null;
  title?: string;
  submitLabel?: string;
  timezone?: string;
  timezoneLabel?: string;
  onCancel?: () => void;
  onChange: (value: ScheduleFormValue) => void;
  onSubmit: (config: Record<string, unknown>) => void;
};

export function ScheduleForm({
  value,
  loading,
  error,
  title,
  submitLabel,
  timezone,
  timezoneLabel,
  onCancel,
  onChange,
  onSubmit
}: ScheduleFormProps) {
  const [localError, setLocalError] = useState<string | null>(null);
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const pad = (value: number) => value.toString().padStart(2, "0");
  const localTime = `${pad(now.getHours())}:${pad(now.getMinutes())}`;
  const utcTime = `${pad(now.getUTCHours())}:${pad(now.getUTCMinutes())}`;
  const offsetMinutes = -now.getTimezoneOffset();
  const offsetSign = offsetMinutes >= 0 ? "+" : "-";
  const offsetAbs = Math.abs(offsetMinutes);
  const offset = `${offsetSign}${pad(Math.floor(offsetAbs / 60))}:${pad(offsetAbs % 60)}`;
  const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  const tz = (timezone || "").trim() || browserZone;
  const tzLabel = (timezoneLabel || "").trim() || tz;
  const formatTimeForZone = (value: Date, zone: string) => {
    try {
      return new Intl.DateTimeFormat("ru-RU", {
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
        timeZone: zone
      }).format(value);
    } catch {
      return value.toLocaleTimeString("ru-RU", { hour: "2-digit", minute: "2-digit", hour12: false });
    }
  };
  const tzTime = formatTimeForZone(now, tz);
  const timeHint = `Сейчас: ${tzTime} (${tzLabel}), локальное: ${localTime} (UTC${offset}), UTC: ${utcTime}`;

  const handleSubmit = () => {
    const result = buildScheduleConfig(value);
    if (!result.ok) {
      setLocalError(result.error);
      return;
    }
    setLocalError(null);
    onSubmit(result.config);
  };

  const showDaily = value.strategy === "daily";
  const showWeekly = value.strategy === "weekly";
  const showCustom = value.strategy === "custom";

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <h3 className="font-semibold">{title || "Новое расписание"}</h3>
      {error && <div className="text-sm text-red-500">{error}</div>}
      {localError && <div className="text-sm text-red-500">{localError}</div>}
      <div className="grid gap-3 md:grid-cols-2">
        <input
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
          placeholder="Название"
          value={value.name}
          onChange={(e) => onChange({ ...value, name: e.target.value })}
        />
        <select
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
          value={value.strategy}
          onChange={(e) => onChange({ ...value, strategy: e.target.value })}
        >
          <option value="immediate">Сразу</option>
          <option value="daily">Ежедневно</option>
          <option value="weekly">Еженедельно</option>
          <option value="custom">CRON</option>
        </select>
        <input
          className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 md:col-span-2"
          placeholder="Описание"
          value={value.description}
          onChange={(e) => onChange({ ...value, description: e.target.value })}
        />
        {showDaily && (
          <>
            <input
              type="number"
              min={1}
              className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              placeholder="Лимит (день)"
              value={value.dailyLimit}
              onChange={(e) => onChange({ ...value, dailyLimit: e.target.value })}
            />
            <input
              type="time"
              className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={value.dailyTime}
              onChange={(e) => onChange({ ...value, dailyTime: e.target.value })}
            />
            <div className="text-xs text-slate-500 dark:text-slate-400 md:col-span-2">
              {timeHint}. Время расписания интерпретируется как {tzLabel}.
            </div>
          </>
        )}
        {showWeekly && (
          <>
            <input
              type="number"
              min={1}
              className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              placeholder="Лимит (неделя)"
              value={value.weeklyLimit}
              onChange={(e) => onChange({ ...value, weeklyLimit: e.target.value })}
            />
            <select
              className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={value.weeklyDay}
              onChange={(e) => onChange({ ...value, weeklyDay: e.target.value })}
            >
              <option value="mon">Пн</option>
              <option value="tue">Вт</option>
              <option value="wed">Ср</option>
              <option value="thu">Чт</option>
              <option value="fri">Пт</option>
              <option value="sat">Сб</option>
              <option value="sun">Вс</option>
            </select>
            <input
              type="time"
              className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 md:col-span-2"
              value={value.weeklyTime}
              onChange={(e) => onChange({ ...value, weeklyTime: e.target.value })}
            />
            <div className="text-xs text-slate-500 dark:text-slate-400 md:col-span-2">
              {timeHint}. Время расписания интерпретируется как {tzLabel}.
            </div>
          </>
        )}
        {showCustom && (
          <div className="md:col-span-2 space-y-1">
            <input
              className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100 w-full"
              placeholder="CRON выражение (например: 0 9 * * *)"
              value={value.customCron}
              onChange={(e) => onChange({ ...value, customCron: e.target.value })}
            />
            <div className="text-xs text-slate-500 dark:text-slate-400">
              Формат cron: минута час день месяц день_недели
            </div>
          </div>
        )}
        <label className="flex items-center gap-2 text-sm text-slate-600 dark:text-slate-300">
          <input
            type="checkbox"
            checked={value.isActive}
            onChange={(e) => onChange({ ...value, isActive: e.target.checked })}
          />
          Активно
        </label>
      </div>
      <div className="flex flex-wrap gap-2">
        <button
          onClick={handleSubmit}
          disabled={loading || !value.name.trim()}
          className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
        >
          {submitLabel || "Создать расписание"}
        </button>
        {onCancel && (
          <button
            onClick={onCancel}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            Отмена
          </button>
        )}
      </div>
    </div>
  );
}
