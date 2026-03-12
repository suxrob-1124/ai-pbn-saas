'use client';

import { useCallback, useEffect, useState } from 'react';
import { ChevronDown, ChevronUp, Clock, XCircle, AlertTriangle } from 'lucide-react';
import { getScheduleRunLogs } from '../../../lib/linkSchedulesApi';
import type { ScheduleRunLog } from '../../../types/schedules';

const REASON_LABELS: Record<string, string> = {
  already_inserted: 'Ссылка уже вставлена',
  not_published: 'Сайт не опубликован',
  no_link_settings: 'Не настроен анкор/акцептор',
  active_task_running: 'Задача уже выполняется',
  link_ready_at_future: 'Ещё не готов (link_ready_at)',
  not_generated: 'Сайт не сгенерирован',
  already_queued: 'Уже в очереди',
};

function formatDate(iso: string) {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

function RunRow({ log }: { log: ScheduleRunLog }) {
  const [expanded, setExpanded] = useState(false);
  const hasSkips = log.skip_details && log.skip_details.length > 0;
  const typeLabel = log.schedule_type === 'link' ? 'Ссылки' : 'Генерация';
  const typeBg = log.schedule_type === 'link'
    ? 'bg-indigo-50 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300'
    : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300';

  return (
    <div className="border-b border-slate-100 dark:border-slate-800/60 last:border-0">
      <button
        onClick={() => hasSkips && setExpanded((v) => !v)}
        className={`w-full flex items-center gap-3 px-4 py-3 text-sm text-left transition-colors ${hasSkips ? 'hover:bg-slate-50 dark:hover:bg-slate-800/30 cursor-pointer' : 'cursor-default'}`}
      >
        <span className={`text-[10px] font-bold px-1.5 py-0.5 rounded uppercase ${typeBg}`}>
          {typeLabel}
        </span>
        <span className="text-slate-500 dark:text-slate-400 text-xs shrink-0">
          {formatDate(log.run_at)}
        </span>
        <span className="flex-1 text-slate-700 dark:text-slate-300 font-medium">
          {log.enqueued_count > 0 ? (
            <span className="text-emerald-600 dark:text-emerald-400">
              Добавлено в очередь: {log.enqueued_count}
            </span>
          ) : (
            <span className="text-slate-400">Ничего не добавлено</span>
          )}
          {log.skipped_count > 0 && (
            <span className="text-red-500 ml-2">· пропущено {log.skipped_count}</span>
          )}
          {log.total_domains > 0 && (
            <span className="text-slate-400 text-xs ml-2">из {log.total_domains}</span>
          )}
        </span>
        {log.next_run_at && (
          <span className="text-xs text-slate-400 shrink-0 hidden md:block">
            Следующий: {formatDate(log.next_run_at)}
          </span>
        )}
        {log.error_message && (
          <AlertTriangle className="w-4 h-4 text-red-400 shrink-0" />
        )}
        {hasSkips && (
          expanded ? <ChevronUp className="w-4 h-4 text-slate-400 shrink-0" /> : <ChevronDown className="w-4 h-4 text-slate-400 shrink-0" />
        )}
      </button>

      {expanded && hasSkips && (
        <div className="px-4 pb-3 space-y-1.5">
          {log.error_message && (
            <p className="text-xs text-red-500 font-mono mb-2 p-2 bg-red-50 dark:bg-red-900/20 rounded-lg">
              {log.error_message}
            </p>
          )}
          {log.skip_details.map((s, i) => (
            <div key={i} className="flex items-start gap-2 py-1.5 text-sm bg-red-50/40 dark:bg-red-900/10 rounded-lg px-3">
              <XCircle className="w-4 h-4 text-red-400 flex-shrink-0 mt-0.5" />
              <div className="min-w-0">
                <span className="text-slate-700 dark:text-slate-300 block truncate">{s.domain_url}</span>
                <span className="text-xs text-red-500 dark:text-red-400">
                  {REASON_LABELS[s.reason] ?? s.reason}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

type Props = {
  projectId: string;
  isScheduleActive?: boolean;
};

export function ScheduleRunHistory({ projectId, isScheduleActive }: Props) {
  const [logs, setLogs] = useState<ScheduleRunLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!projectId) return;
    setLoading(true);
    setError(null);
    try {
      const data = await getScheduleRunLogs(projectId);
      setLogs(Array.isArray(data) ? data : []);
    } catch (e: any) {
      setError(e?.message || 'Не удалось загрузить историю');
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => {
    void load();
  }, [load]);

  // Auto-refresh when schedule is active
  useEffect(() => {
    if (!isScheduleActive) return;
    const id = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(id);
  }, [isScheduleActive, load]);

  if (loading && logs.length === 0) {
    return (
      <div className="border border-slate-200 dark:border-slate-800 rounded-2xl p-4 text-sm text-slate-500 animate-pulse">
        Загрузка истории запусков...
      </div>
    );
  }

  if (error) {
    return (
      <div className="border border-red-200 dark:border-red-900/50 rounded-2xl p-4 text-sm text-red-500">
        {error}
      </div>
    );
  }

  if (logs.length === 0) return null;

  return (
    <div className="border border-slate-200 dark:border-slate-800 rounded-2xl overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 bg-slate-50 dark:bg-slate-900/40 border-b border-slate-200 dark:border-slate-800">
        <div className="flex items-center gap-2">
          <Clock className="w-4 h-4 text-slate-400" />
          <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">
            История запусков
          </span>
          <span className="text-xs text-slate-400">({logs.length})</span>
        </div>
        <button
          onClick={load}
          disabled={loading}
          className="text-xs text-slate-500 hover:text-indigo-600 transition-colors disabled:opacity-50">
          {loading ? 'Обновление...' : 'Обновить'}
        </button>
      </div>
      <div>
        {logs.map((log) => (
          <RunRow key={log.id} log={log} />
        ))}
      </div>
    </div>
  );
}
