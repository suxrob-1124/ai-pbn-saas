'use client';

import { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import {
  RefreshCw,
  AlertTriangle,
  Clock,
  ExternalLink,
  ShieldAlert,
  ChevronDown,
  ChevronUp,
  RotateCw,
  Globe,
  CheckCheck,
  Eye,
  EyeOff,
} from 'lucide-react';

const DISMISSED_KEY = 'project_diagnostics_dismissed';

type Generation = {
  id: string;
  domain_id?: string;
  domain_url?: string | null;
  error?: string;
  created_at?: string;
  updated_at?: string;
  started_at?: string;
  finished_at?: string;
};

type DomainLite = {
  id: string;
  url: string;
};

type ProjectDiagnosticsSectionProps = {
  loading: boolean;
  error: string | null;
  items: Generation[];
  domainById: Record<string, DomainLite>;
  formatDateTime: (value?: string) => string;
  onRefresh: () => void;
  onRetry?: (domainId: string) => void;
};

// Вспомогательная функция для очистки текста ошибки от уникальных ID,
// чтобы одинаковые ошибки правильно группировались
function normalizeErrorMsg(err: string) {
  if (!err) return 'Неизвестная ошибка';
  return err
    .replace(/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/gi, '[ID]')
    .trim();
}

export function ProjectDiagnosticsSection({
  loading,
  error,
  items,
  domainById,
  formatDateTime,
  onRefresh,
  onRetry,
}: ProjectDiagnosticsSectionProps) {
  // Dismissed generation IDs (localStorage)
  const [dismissedIds, setDismissedIds] = useState<Set<string>>(() => {
    try {
      const raw = localStorage.getItem(DISMISSED_KEY);
      return new Set(raw ? JSON.parse(raw) : []);
    } catch {
      return new Set();
    }
  });
  const [showDismissed, setShowDismissed] = useState(false);

  useEffect(() => {
    try {
      localStorage.setItem(DISMISSED_KEY, JSON.stringify([...dismissedIds]));
    } catch {}
  }, [dismissedIds]);

  const dismissItems = (ids: string[]) => {
    setDismissedIds((prev) => new Set([...prev, ...ids]));
  };

  const restoreItems = (ids: string[]) => {
    setDismissedIds((prev) => {
      const next = new Set(prev);
      ids.forEach((id) => next.delete(id));
      return next;
    });
  };

  // Разделяем на активные и закрытые
  const activeItems = useMemo(() => items.filter((i) => !dismissedIds.has(i.id)), [items, dismissedIds]);
  const dismissedItems = useMemo(() => items.filter((i) => dismissedIds.has(i.id)), [items, dismissedIds]);

  // Группировка ошибок
  const groupedErrors = useMemo(() => {
    const source = showDismissed ? dismissedItems : activeItems;
    const groups: Record<string, Generation[]> = {};
    source.forEach((item) => {
      const msg = normalizeErrorMsg(item.error || 'Неизвестная ошибка');
      if (!groups[msg]) groups[msg] = [];
      groups[msg].push(item);
    });
    // Сортируем группы по количеству упавших доменов
    return Object.entries(groups).sort((a, b) => b[1].length - a[1].length);
  }, [activeItems, dismissedItems, showDismissed]);

  // Стейт для раскрытия/закрытия групп (по умолчанию открываем первую, если она есть)
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>(() => {
    const defaultState: Record<string, boolean> = {};
    if (groupedErrors.length > 0) defaultState[groupedErrors[0][0]] = true;
    return defaultState;
  });

  const toggleGroup = (msg: string) => {
    setExpandedGroups((prev) => ({ ...prev, [msg]: !prev[msg] }));
  };

  const handleRetryAll = (groupItems: Generation[]) => {
    if (!onRetry) return;
    const domainIds = Array.from(
      new Set(groupItems.map((i) => i.domain_id).filter(Boolean)),
    ) as string[];
    if (confirm(`Повторить генерацию для ${domainIds.length} доменов?`)) {
      domainIds.forEach((id) => onRetry(id));
    }
  };

  const handleDismissGroup = (groupItems: Generation[]) => {
    dismissItems(groupItems.map((i) => i.id));
  };

  return (
    <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm animate-in fade-in duration-300 overflow-hidden">
      {/* HEADER */}
      <div className="p-6 border-b border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-[#0a1020] flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h3 className="text-lg font-bold text-slate-900 dark:text-white flex items-center gap-2">
            <ShieldAlert className="w-5 h-5 text-red-500" /> Журнал сбоев и ошибок
          </h3>
          <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
            Ошибки сгруппированы по типу. Вы можете перезапустить сайты прямо отсюда.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {dismissedItems.length > 0 && (
            <button
              onClick={() => setShowDismissed((v) => !v)}
              className="inline-flex items-center gap-2 px-3 py-2.5 rounded-xl border border-slate-200 bg-white text-sm font-medium text-slate-500 hover:bg-slate-50 dark:border-slate-600 dark:bg-[#060d18] dark:text-slate-400 dark:hover:bg-slate-800 transition-all shadow-sm active:scale-95">
              {showDismissed ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              {showDismissed ? 'Скрыть закрытые' : `Закрытые (${dismissedItems.length})`}
            </button>
          )}
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl border border-slate-200 bg-white text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-600 dark:bg-[#060d18] dark:text-slate-300 dark:hover:bg-slate-800 transition-all shadow-sm active:scale-95">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Обновить лог
          </button>
        </div>
      </div>

      {/* СОСТОЯНИЯ */}
      {loading && items.length === 0 && (
        <div className="p-16 text-center text-slate-500 dark:text-slate-400 font-medium">
          Сканирование логов...
        </div>
      )}

      {!loading && error && (
        <div className="p-6 border-b border-red-100 dark:border-red-900/30 bg-red-50 dark:bg-red-900/10 text-sm font-medium text-red-600 dark:text-red-400">
          <AlertTriangle className="w-5 h-5 inline-block mr-2" /> {error}
        </div>
      )}

      {!loading && !error && activeItems.length === 0 && !showDismissed && (
        <div className="p-20 text-center flex flex-col items-center justify-center bg-slate-50/50 dark:bg-[#0a1020]">
          <div className="w-16 h-16 bg-emerald-50 border border-emerald-100 dark:border-emerald-900/50 dark:bg-emerald-900/20 text-emerald-500 rounded-full flex items-center justify-center mb-5 shadow-inner">
            <ShieldAlert className="w-8 h-8" />
          </div>
          <h4 className="text-xl font-bold text-slate-900 dark:text-white">
            Всё работает стабильно
          </h4>
          <p className="text-slate-500 dark:text-slate-400 mt-2 max-w-sm">
            Сбоев в работе генератора за последнее время не обнаружено.
          </p>
        </div>
      )}

      {/* ГРУППИРОВАННЫЙ СПИСОК ОШИБОК */}
      {groupedErrors.length > 0 && (
        <div className="divide-y divide-slate-100 dark:divide-slate-800/60 bg-slate-50/30 dark:bg-transparent">
          {groupedErrors.map(([errorMsg, groupItems], index) => {
            const isExpanded = expandedGroups[errorMsg];
            const uniqueDomainsCount = new Set(groupItems.map((i) => i.domain_id).filter(Boolean))
              .size;

            return (
              <div key={index} className="transition-colors">
                {/* ШАПКА ГРУППЫ */}
                <div
                  className={`p-5 cursor-pointer hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors flex flex-col lg:flex-row lg:items-center justify-between gap-4 ${isExpanded ? 'bg-slate-50 dark:bg-[#0a1020]' : ''}`}
                  onClick={() => toggleGroup(errorMsg)}>
                  <div className="flex items-start gap-3 min-w-0 flex-1">
                    <button className="mt-0.5 p-1 rounded-md text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors">
                      {isExpanded ? (
                        <ChevronUp className="w-4 h-4" />
                      ) : (
                        <ChevronDown className="w-4 h-4" />
                      )}
                    </button>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2 mb-1.5">
                        <span className="bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400 text-xs font-bold px-2.5 py-0.5 rounded-full border border-red-200 dark:border-red-800/50">
                          {groupItems.length}{' '}
                          {groupItems.length === 1
                            ? 'сбой'
                            : groupItems.length < 5
                              ? 'сбоя'
                              : 'сбоев'}
                        </span>
                        <span className="text-sm font-medium text-slate-500 dark:text-slate-400">
                          Затронуто сайтов: {uniqueDomainsCount}
                        </span>
                      </div>
                      <div
                        className="font-mono text-[13px] text-red-600 dark:text-red-300/90 truncate max-w-full font-medium"
                        title={errorMsg}>
                        {errorMsg}
                      </div>
                    </div>
                  </div>

                  {/* Массовые действия для группы */}
                  <div
                    className="flex items-center gap-2 flex-shrink-0 ml-9 lg:ml-0"
                    onClick={(e) => e.stopPropagation()}>
                    {onRetry && !showDismissed && (
                      <button
                        onClick={() => handleRetryAll(groupItems)}
                        className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-white border border-slate-200 text-slate-700 hover:bg-indigo-50 hover:text-indigo-700 hover:border-indigo-200 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-300 dark:hover:bg-indigo-900/30 dark:hover:text-indigo-300 dark:hover:border-indigo-800 transition-all text-sm font-semibold shadow-sm">
                        <RotateCw className="w-4 h-4" /> Перезапустить все
                      </button>
                    )}
                    {!showDismissed ? (
                      <button
                        onClick={() => handleDismissGroup(groupItems)}
                        className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-white border border-slate-200 text-slate-500 hover:bg-emerald-50 hover:text-emerald-700 hover:border-emerald-200 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-400 dark:hover:bg-emerald-900/30 dark:hover:text-emerald-300 dark:hover:border-emerald-800 transition-all text-sm font-semibold shadow-sm">
                        <CheckCheck className="w-4 h-4" /> Закрыть
                      </button>
                    ) : (
                      <button
                        onClick={() => restoreItems(groupItems.map((i) => i.id))}
                        className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl bg-white border border-slate-200 text-slate-500 hover:bg-amber-50 hover:text-amber-700 hover:border-amber-200 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-400 transition-all text-sm font-semibold shadow-sm">
                        <RotateCw className="w-4 h-4" /> Восстановить
                      </button>
                    )}
                  </div>
                </div>

                {/* РАСКРЫТЫЙ СПИСОК (Детали по доменам) */}
                {isExpanded && (
                  <div className="p-5 pt-2 pl-14 bg-slate-50 dark:bg-[#0a1020] border-t border-slate-100 dark:border-slate-800/40">
                    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      {groupItems.map((item) => {
                        const domain = item.domain_id ? domainById[item.domain_id] : undefined;
                        const label = domain?.url || item.domain_url || 'Неизвестный домен';
                        const when =
                          item.updated_at || item.finished_at || item.started_at || item.created_at;
                        const timeLabel = formatDateTime(when);

                        return (
                          <div
                            key={item.id}
                            className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 p-4 rounded-xl shadow-sm flex flex-col justify-between h-full group">
                            <div>
                              <div className="flex items-start justify-between gap-2 mb-2">
                                <div className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5 truncate">
                                  <Globe className="w-3.5 h-3.5 text-indigo-500 flex-shrink-0" />
                                  <span className="truncate" title={label}>
                                    {label}
                                  </span>
                                </div>
                                <Link
                                  href={`/queue/${item.id}`}
                                  className="text-slate-400 hover:text-indigo-500 transition-colors"
                                  title="Детальный лог">
                                  <ExternalLink className="w-4 h-4" />
                                </Link>
                              </div>
                              <div className="flex items-center gap-1.5 text-[11px] text-slate-500 dark:text-slate-400 font-medium">
                                <Clock className="w-3 h-3 opacity-70" /> {timeLabel}
                              </div>
                            </div>

                            <div className="mt-4 pt-3 border-t border-slate-100 dark:border-slate-800/60 flex gap-2">
                              {onRetry && item.domain_id && !showDismissed && (
                                <button
                                  onClick={() => onRetry(item.domain_id!)}
                                  className="flex-1 inline-flex items-center justify-center gap-2 py-1.5 rounded-lg text-xs font-semibold text-indigo-600 bg-indigo-50 hover:bg-indigo-100 dark:bg-indigo-500/10 dark:text-indigo-400 dark:hover:bg-indigo-500/20 transition-colors">
                                  <RotateCw className="w-3.5 h-3.5" /> Запустить заново
                                </button>
                              )}
                              {!showDismissed ? (
                                <button
                                  onClick={() => dismissItems([item.id])}
                                  className="flex-1 inline-flex items-center justify-center gap-2 py-1.5 rounded-lg text-xs font-semibold text-slate-500 bg-slate-100 hover:bg-emerald-50 hover:text-emerald-700 dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-emerald-900/20 dark:hover:text-emerald-400 transition-colors">
                                  <CheckCheck className="w-3.5 h-3.5" /> Закрыть
                                </button>
                              ) : (
                                <button
                                  onClick={() => restoreItems([item.id])}
                                  className="flex-1 inline-flex items-center justify-center gap-2 py-1.5 rounded-lg text-xs font-semibold text-slate-500 bg-slate-100 hover:bg-amber-50 hover:text-amber-700 dark:bg-slate-800 dark:text-slate-400 transition-colors">
                                  <RotateCw className="w-3.5 h-3.5" /> Восстановить
                                </button>
                              )}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
