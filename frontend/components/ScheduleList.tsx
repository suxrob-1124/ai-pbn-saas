"use client";

import { useState } from "react";
import { FiEdit2, FiRefreshCw, FiTrash2 } from "react-icons/fi";
import type { ScheduleDTO } from "../types/schedules";
import { ScheduleTrigger } from "./ScheduleTrigger";

const STRATEGY_LABELS: Record<string, string> = {
  immediate: "Сразу",
  daily: "Ежедневно",
  weekly: "Еженедельно",
  custom: "CRON"
};

type ScheduleListProps = {
  schedules: ScheduleDTO[];
  loading: boolean;
  error?: string | null;
  permissionDenied?: boolean;
  title?: string;
  timezone?: string;
  onRefresh: () => void;
  onTrigger: (schedule: ScheduleDTO) => void;
  onToggle: (schedule: ScheduleDTO) => void;
  onEdit: (schedule: ScheduleDTO) => void;
  onDelete: (schedule: ScheduleDTO) => void;
};

export function ScheduleList({
  schedules,
  loading,
  error,
  permissionDenied,
  title,
  timezone,
  onRefresh,
  onTrigger,
  onToggle,
  onEdit,
  onDelete
}: ScheduleListProps) {
  const [pendingDelete, setPendingDelete] = useState<ScheduleDTO | null>(null);
  const tz = (timezone || "").trim();
  const formatDateTime = (value?: string) => {
    if (!value) return "—";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "—";
    if (tz) {
      try {
        return new Intl.DateTimeFormat("ru-RU", {
          dateStyle: "short",
          timeStyle: "medium",
          timeZone: tz
        }).format(date);
      } catch {
        // fallback to local
      }
    }
    return date.toLocaleString();
  };

  const handleConfirmDelete = () => {
    if (!pendingDelete) return;
    onDelete(pendingDelete);
    setPendingDelete(null);
  };

  return (
    <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="font-semibold">{title || "Список расписаний"}</h3>
        <button
          onClick={onRefresh}
          disabled={loading}
          className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
        >
          <FiRefreshCw /> Обновить
        </button>
      </div>
      {loading && (
        <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка расписаний...</div>
      )}
      {!loading && permissionDenied && (
        <div className="text-sm text-amber-600 dark:text-amber-400">
          Недостаточно прав для просмотра расписаний.
        </div>
      )}
      {!loading && !permissionDenied && error && (
        <div className="text-sm text-red-500">{error}</div>
      )}
      {!loading && !permissionDenied && !error && schedules.length === 0 && (
        <div className="text-sm text-slate-500 dark:text-slate-400">Расписаний пока нет.</div>
      )}
      {!loading && !permissionDenied && !error && schedules.length > 0 && (
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-4">Название</th>
                <th className="py-2 pr-4">Стратегия</th>
                <th className="py-2 pr-4">Активно</th>
                <th className="py-2 pr-4">Следующий запуск</th>
                <th className="py-2 pr-4">Обновлено</th>
                <th className="py-2 pr-4 text-right">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
              {schedules.map((sched) => (
                <tr key={sched.id}>
                  <td className="py-3 pr-4 font-medium">{sched.name}</td>
                  <td className="py-3 pr-4">{STRATEGY_LABELS[sched.strategy] || sched.strategy}</td>
                  <td className="py-3 pr-4">{sched.isActive ? "Да" : "Нет"}</td>
                  <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                    {(() => {
                      const nextRun = sched.nextRunAt || (sched as { next_run_at?: string }).next_run_at;
                      return formatDateTime(nextRun);
                    })()}
                  </td>
                  <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                    {(() => {
                      const updated =
                        (sched as { updatedAt?: string }).updatedAt ||
                        (sched as { updated_at?: string }).updated_at ||
                        sched.createdAt;
                      return formatDateTime(updated);
                    })()}
                  </td>
                  <td className="py-3 pr-4 text-right space-x-2">
                    <ScheduleTrigger onClick={() => onTrigger(sched)} disabled={loading} />
                    <button
                      onClick={() => onEdit(sched)}
                      disabled={loading}
                      className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      <FiEdit2 /> Редактировать
                    </button>
                    <button
                      onClick={() => onToggle(sched)}
                      disabled={loading}
                      className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                    >
                      {sched.isActive ? "Пауза" : "Активировать"}
                    </button>
                    <button
                      onClick={() => setPendingDelete(sched)}
                      disabled={loading}
                      className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                    >
                      <FiTrash2 /> Удалить
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {pendingDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="w-full max-w-sm rounded-xl bg-white p-5 shadow-xl dark:bg-slate-900">
            <h4 className="text-base font-semibold">Удалить расписание?</h4>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
              Подтвердить удаление &laquo;{pendingDelete.name}&raquo;.
            </p>
            <div className="mt-4 flex justify-end gap-2">
              <button
                onClick={() => setPendingDelete(null)}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Отмена
              </button>
              <button
                onClick={handleConfirmDelete}
                className="inline-flex items-center gap-2 rounded-lg bg-red-600 px-3 py-2 text-xs font-semibold text-white hover:bg-red-500"
              >
                Подтвердить удаление
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
