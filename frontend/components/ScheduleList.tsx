'use client';

import { useState } from 'react';
import { Edit3, RefreshCw, Trash2, Power, PowerOff, Clock, Play } from 'lucide-react';
import type { ScheduleDTO } from '../types/schedules';
import { ScheduleTrigger } from './ScheduleTrigger';
import { canCancel, canDelete, canRun } from '../features/queue-monitoring/services/actionGuards';
import {
  getScheduleActivityLabel,
  getScheduleStrategyLabel,
  getScheduleToggleLabel,
} from '../features/queue-monitoring/services/statusMeta';

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
  onDelete,
}: ScheduleListProps) {
  const [pendingDelete, setPendingDelete] = useState<ScheduleDTO | null>(null);
  const refreshGuard = canRun({ busy: loading });
  const tz = (timezone || '').trim();

  const formatDateTime = (value?: string) => {
    if (!value) return '—';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return '—';
    if (tz) {
      try {
        return new Intl.DateTimeFormat('ru-RU', {
          dateStyle: 'short',
          timeStyle: 'short',
          timeZone: tz,
        }).format(date);
      } catch {}
    }
    return date.toLocaleString();
  };

  const handleConfirmDelete = () => {
    if (!pendingDelete) return;
    onDelete(pendingDelete);
    setPendingDelete(null);
  };

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between px-1">
        <h3 className="text-sm font-bold text-slate-700 dark:text-slate-300">
          {title || 'Список расписаний'}
        </h3>
        <button
          onClick={onRefresh}
          disabled={refreshGuard.disabled}
          title={refreshGuard.reason}
          className="p-1.5 rounded-md text-slate-400 hover:text-slate-600 hover:bg-slate-200 dark:hover:bg-slate-800 dark:hover:text-slate-200 transition-colors">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {loading && schedules.length === 0 && (
        <div className="text-sm text-slate-400 p-4 border border-dashed border-slate-200 dark:border-slate-800 rounded-xl">
          Загрузка...
        </div>
      )}
      {!loading && permissionDenied && (
        <div className="text-sm text-amber-500 p-4 border border-amber-200 rounded-xl">
          Нет прав для просмотра.
        </div>
      )}
      {!loading && error && (
        <div className="text-sm text-red-500 p-4 border border-red-200 rounded-xl">{error}</div>
      )}
      {!loading && !permissionDenied && !error && schedules.length === 0 && (
        <div className="text-sm text-slate-500 dark:text-slate-400 p-6 text-center border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-2xl bg-white/50 dark:bg-slate-900/20">
          Активных расписаний нет. Создайте новое выше.
        </div>
      )}

      {schedules.length > 0 && (
        <div className="grid gap-4 md:grid-cols-2">
          {schedules.map((sched) => {
            const runGuard = canRun({ busy: loading });
            const editGuard = canRun({ busy: loading });
            const toggleGuard = sched.isActive
              ? canCancel({ busy: loading, allowed: true })
              : canRun({ busy: loading });
            const deleteGuard = canDelete({ busy: loading, allowed: true });

            const nextRun = sched.nextRunAt || (sched as any).next_run_at;
            const updated =
              (sched as any).updatedAt || (sched as any).updated_at || sched.createdAt;

            return (
              <div
                key={sched.id}
                className={`flex flex-col justify-between p-5 rounded-2xl border transition-all shadow-sm ${sched.isActive ? 'bg-white dark:bg-[#0f1523] border-slate-200 dark:border-slate-700' : 'bg-slate-50 dark:bg-[#0a1020] border-slate-100 dark:border-slate-800 opacity-75'}`}>
                <div>
                  <div className="flex items-start justify-between mb-3">
                    <div>
                      <h4 className="font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        {sched.name}
                        {sched.isActive ? (
                          <span className="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.4)]"></span>
                        ) : (
                          <span className="w-2 h-2 rounded-full bg-slate-300 dark:bg-slate-600"></span>
                        )}
                      </h4>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                        {getScheduleStrategyLabel(sched.strategy)}
                      </p>
                    </div>

                    {/* Кнопка Вкл/Выкл (Toggle) */}
                    <button
                      onClick={() => onToggle(sched)}
                      disabled={toggleGuard.disabled}
                      title={toggleGuard.reason}
                      className={`p-2 rounded-lg transition-colors ${sched.isActive ? 'bg-emerald-50 text-emerald-600 hover:bg-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-400 dark:hover:bg-emerald-500/20' : 'bg-slate-100 text-slate-500 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-400'}`}>
                      {sched.isActive ? (
                        <Power className="w-4 h-4" />
                      ) : (
                        <PowerOff className="w-4 h-4" />
                      )}
                    </button>
                  </div>

                  <div className="flex items-center gap-2 text-xs font-medium text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-slate-800/50 p-2 rounded-lg">
                    <Clock className="w-3.5 h-3.5 opacity-70" />
                    <span>
                      След. запуск:{' '}
                      <strong className="text-slate-700 dark:text-slate-200">
                        {formatDateTime(nextRun)}
                      </strong>
                    </span>
                  </div>
                </div>

                {/* Подвал с кнопками */}
                <div className="flex items-center justify-between mt-5 pt-4 border-t border-slate-100 dark:border-slate-800">
                  <div className="flex gap-2">
                    <ScheduleTrigger
                      onClick={() => onTrigger(sched)}
                      disabled={runGuard.disabled}
                      disabledReason={runGuard.reason}
                    />
                    <button
                      onClick={() => onEdit(sched)}
                      disabled={editGuard.disabled}
                      className="p-2 rounded-lg bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700 transition-colors"
                      title="Редактировать">
                      <Edit3 className="w-4 h-4" />
                    </button>
                  </div>
                  <button
                    onClick={() => setPendingDelete(sched)}
                    disabled={deleteGuard.disabled}
                    className="p-2 rounded-lg text-slate-400 hover:text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 transition-colors"
                    title="Удалить">
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Модалка удаления */}
      {pendingDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 backdrop-blur-sm p-4">
          <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-2xl w-full max-w-sm p-6 animate-in fade-in zoom-in-95">
            <h4 className="text-lg font-bold text-slate-900 dark:text-white">
              Удалить расписание?
            </h4>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
              Это действие нельзя отменить. Расписание «{pendingDelete.name}» будет безвозвратно
              удалено.
            </p>
            <div className="mt-6 flex justify-end gap-3">
              <button
                onClick={() => setPendingDelete(null)}
                className="px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                Отмена
              </button>
              <button
                onClick={handleConfirmDelete}
                className="px-5 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-500 rounded-xl shadow-sm">
                Удалить
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
