'use client';

import { useEffect, useState } from 'react';
import { Clock, Info } from 'lucide-react';
import { buildScheduleConfig, ScheduleFormValue } from '../lib/scheduleFormValidation';
import type { SchedulePreviewSection } from '../types/schedules';

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
  projectId?: string;
  scheduleType?: 'generation' | 'link';
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
  onSubmit,
  projectId,
  scheduleType,
}: ScheduleFormProps) {
  const [localError, setLocalError] = useState<string | null>(null);
  const [now, setNow] = useState(() => new Date());
  const [preview, setPreview] = useState<SchedulePreviewSection | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [showPreview, setShowPreview] = useState(false);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  const pad = (value: number) => value.toString().padStart(2, '0');
  const localTime = `${pad(now.getHours())}:${pad(now.getMinutes())}`;
  const utcTime = `${pad(now.getUTCHours())}:${pad(now.getUTCMinutes())}`;

  const offsetMinutes = -now.getTimezoneOffset();
  const offsetSign = offsetMinutes >= 0 ? '+' : '-';
  const offsetAbs = Math.abs(offsetMinutes);
  const offset = `${offsetSign}${pad(Math.floor(offsetAbs / 60))}:${pad(offsetAbs % 60)}`;

  const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  const tz = (timezone || '').trim() || browserZone;
  const tzLabel = (timezoneLabel || '').trim() || tz;

  const formatTimeForZone = (value: Date, zone: string) => {
    try {
      return new Intl.DateTimeFormat('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
        timeZone: zone,
      }).format(value);
    } catch {
      return value.toLocaleTimeString('ru-RU', {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      });
    }
  };

  const tzTime = formatTimeForZone(now, tz);
  const timeHint = `Текущее время: ${tzTime} (${tzLabel})`;

  const handleSubmit = () => {
    const result = buildScheduleConfig(value);
    if (!result.ok) {
      setLocalError(result.error);
      return;
    }
    setLocalError(null);
    onSubmit(result.config);
  };

  const handlePreview = async () => {
    if (!projectId) return;
    setPreviewLoading(true);
    setShowPreview(true);
    setPreview(null);
    try {
      const { getSchedulesPreview } = await import('../lib/linkSchedulesApi');
      const data = await getSchedulesPreview(projectId);
      setPreview(scheduleType === 'link' ? data.link : data.generation);
    } catch {
      setPreview(null);
    } finally {
      setPreviewLoading(false);
    }
  };

  const showDaily = value.strategy === 'daily';
  const showWeekly = value.strategy === 'weekly';
  const showCustom = value.strategy === 'custom';

  const inputBaseClass =
    'w-full bg-white dark:bg-[#060d18] border border-slate-200 dark:border-slate-700 px-3 py-2 text-sm rounded-lg outline-none focus:ring-2 focus:ring-indigo-500/20 focus:border-indigo-500 dark:text-slate-100 transition-all';

  return (
    <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm overflow-hidden animate-in fade-in duration-300">
      <div className="p-5 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-[#0a1020]">
        <h3 className="text-sm font-bold text-slate-900 dark:text-white flex items-center gap-2">
          {title || 'Новое расписание'}
        </h3>
      </div>

      <div className="p-5 space-y-4">
        {error && (
          <div className="p-3 bg-red-50 text-red-600 rounded-lg text-xs border border-red-100 dark:bg-red-950/30 dark:border-red-900/50">
            {error}
          </div>
        )}
        {localError && (
          <div className="p-3 bg-red-50 text-red-600 rounded-lg text-xs border border-red-100 dark:bg-red-950/30 dark:border-red-900/50">
            {localError}
          </div>
        )}

        {projectId && (
          <div>
            <button
              type="button"
              onClick={handlePreview}
              disabled={previewLoading}
              className="text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:underline disabled:opacity-50">
              {previewLoading ? 'Загрузка превью...' : 'Просмотреть следующий запуск →'}
            </button>
            {showPreview && preview && !previewLoading && (
              <div className="mt-2 border border-slate-200 dark:border-slate-700 rounded-xl overflow-hidden text-sm">
                <div className="px-3 py-2 bg-slate-50 dark:bg-slate-800/50 flex items-center justify-between">
                  <span className="font-semibold text-slate-700 dark:text-slate-300">
                    Следующий запуск {preview.next_run_at ? `: ${new Date(preview.next_run_at).toLocaleString('ru-RU')}` : ''}
                  </span>
                  <button onClick={() => setShowPreview(false)} className="text-slate-400 hover:text-slate-600 text-xs">✕</button>
                </div>
                <div className="divide-y divide-slate-100 dark:divide-slate-800 max-h-48 overflow-y-auto">
                  {preview.eligible_domains.map((d) => (
                    <div key={d.id} className="flex items-center gap-2 px-3 py-1.5 text-xs">
                      <span className="text-emerald-500">✓</span>
                      <span className="text-slate-700 dark:text-slate-300 truncate">{d.url}</span>
                    </div>
                  ))}
                  {preview.would_skip.map((s, i) => (
                    <div key={i} className="flex items-start gap-2 px-3 py-1.5 text-xs bg-red-50/40 dark:bg-red-900/10">
                      <span className="text-red-400 mt-0.5">✗</span>
                      <div className="min-w-0">
                        <span className="block text-slate-700 dark:text-slate-300 truncate">{s.domain_url}</span>
                        <span className="text-red-500 dark:text-red-400">{s.reason}</span>
                      </div>
                    </div>
                  ))}
                  {preview.eligible_domains.length === 0 && preview.would_skip.length === 0 && (
                    <div className="px-3 py-3 text-slate-400 text-center">Нет доменов</div>
                  )}
                </div>
                {(preview.eligible_domains.length > 0 || preview.would_skip.length > 0) && (
                  <div className="px-3 py-1.5 bg-slate-50 dark:bg-slate-800/50 text-xs text-slate-500 flex gap-3">
                    <span className="text-emerald-600">✓ {preview.would_enqueue} запустятся</span>
                    {preview.would_skip.length > 0 && (
                      <span className="text-red-500">✗ {preview.would_skip.length} пропускаются</span>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        <div className="grid gap-4 md:grid-cols-[1fr_200px]">
          <div>
            <label className="block text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1.5">
              Название и описание
            </label>
            <div className="space-y-2">
              <input
                className={inputBaseClass}
                placeholder="Название расписания"
                value={value.name}
                onChange={(e) => onChange({ ...value, name: e.target.value })}
              />
              <input
                className={inputBaseClass}
                placeholder="Описание (опционально)"
                value={value.description}
                onChange={(e) => onChange({ ...value, description: e.target.value })}
              />
            </div>
          </div>

          <div>
            <label className="block text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1.5">
              Стратегия запуска
            </label>
            <select
              className={inputBaseClass}
              value={value.strategy}
              onChange={(e) => onChange({ ...value, strategy: e.target.value })}>
              <option value="immediate">Запустить один раз сейчас</option>
              <option value="daily">Каждый день</option>
              <option value="weekly">Раз в неделю</option>
              <option value="custom">По Cron-выражению</option>
            </select>
          </div>
        </div>

        {/* НАСТРОЙКИ СТРАТЕГИИ */}
        {(showDaily || showWeekly || showCustom) && (
          <div className="p-4 bg-slate-50/50 dark:bg-slate-800/30 rounded-xl border border-slate-100 dark:border-slate-700/50">
            {showDaily && (
              <div className="flex flex-wrap items-center gap-3">
                <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                  Каждый день брать по
                </span>
                <input
                  type="number"
                  min={1}
                  className={`${inputBaseClass} w-20`}
                  value={value.dailyLimit}
                  onChange={(e) => onChange({ ...value, dailyLimit: e.target.value })}
                />
                <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                  доменов в
                </span>
                <input
                  type="time"
                  className={`${inputBaseClass} w-28`}
                  value={value.dailyTime}
                  onChange={(e) => onChange({ ...value, dailyTime: e.target.value })}
                />
              </div>
            )}

            {showWeekly && (
              <div className="flex flex-wrap items-center gap-3">
                <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                  Каждую(ый)
                </span>
                <select
                  className={`${inputBaseClass} w-32`}
                  value={value.weeklyDay}
                  onChange={(e) => onChange({ ...value, weeklyDay: e.target.value })}>
                  <option value="mon">Понедельник</option>
                  <option value="tue">Вторник</option>
                  <option value="wed">Среду</option>
                  <option value="thu">Четверг</option>
                  <option value="fri">Пятницу</option>
                  <option value="sat">Субботу</option>
                  <option value="sun">Воскресенье</option>
                </select>
                <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                  брать по
                </span>
                <input
                  type="number"
                  min={1}
                  className={`${inputBaseClass} w-20`}
                  value={value.weeklyLimit}
                  onChange={(e) => onChange({ ...value, weeklyLimit: e.target.value })}
                />
                <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                  доменов в
                </span>
                <input
                  type="time"
                  className={`${inputBaseClass} w-28`}
                  value={value.weeklyTime}
                  onChange={(e) => onChange({ ...value, weeklyTime: e.target.value })}
                />
              </div>
            )}

            {showCustom && (
              <div className="space-y-2">
                <input
                  className={inputBaseClass}
                  placeholder="CRON выражение (например: 0 9 * * *)"
                  value={value.customCron}
                  onChange={(e) => onChange({ ...value, customCron: e.target.value })}
                />
                <p className="text-[11px] text-slate-500 font-mono">
                  Формат: минута час день месяц день_недели
                </p>
              </div>
            )}

            <div className="mt-3 flex flex-wrap items-center gap-3">
              <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                Задержка между запусками
              </span>
              <input
                type="number"
                min={1}
                max={60}
                className={`${inputBaseClass} w-20`}
                value={value.delayMinutes}
                onChange={(e) => onChange({ ...value, delayMinutes: e.target.value })}
              />
              <span className="text-sm font-medium text-slate-700 dark:text-slate-300">мин</span>
            </div>

            <div className="mt-3 flex items-center gap-2 text-[11px] text-slate-500 dark:text-slate-400">
              <Clock className="w-3 h-3" />
              <span>
                {timeHint} (ваша таймзона: {tzLabel})
              </span>
            </div>
          </div>
        )}

        <div className="flex items-center justify-between pt-2">
          <label className="flex items-center gap-2 cursor-pointer group">
            <div className="relative flex items-center">
              <input
                type="checkbox"
                checked={value.isActive}
                onChange={(e) => onChange({ ...value, isActive: e.target.checked })}
                className="sr-only peer"
              />
              <div className="w-9 h-5 bg-slate-200 peer-focus:outline-none rounded-full peer dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-slate-300 after:border after:rounded-full after:h-4 after:w-4 after:transition-all dark:border-slate-600 peer-checked:bg-indigo-600"></div>
            </div>
            <span className="text-sm font-medium text-slate-700 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white transition-colors">
              Активно
            </span>
          </label>

          <div className="flex items-center gap-2">
            {onCancel && (
              <button
                onClick={onCancel}
                disabled={loading}
                className="px-4 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-lg dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                Отмена
              </button>
            )}
            <button
              onClick={handleSubmit}
              disabled={loading || !value.name.trim()}
              className="px-5 py-2 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-lg disabled:opacity-50 transition-all shadow-sm active:scale-95">
              {loading ? 'Сохранение...' : submitLabel || 'Создать'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
