'use client';

import { Suspense, type ReactNode } from 'react';
import { useParams, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import {
  AlertTriangle,
  Clock,
  RefreshCw,
  RotateCw,
  Trash2,
  ListFilter,
  Database,
  ChevronRight,
  Activity,
  Archive,
  Check,
} from 'lucide-react';
import { useAuthGuard } from '@/lib/useAuth';
import type { QueueItemDTO } from '@/types/queue';
import { Badge } from '@/components/Badge';
import {
  canRetryLinkTask,
  getLinkTaskStatusMeta,
  normalizeLinkTaskStatus,
} from '@/lib/linkTaskStatus';

// Импорты компонентов фильтрации
import { FilterDateInput } from '@/features/queue-monitoring/components/FilterDateInput';
import { FlowStateBanner } from '@/features/queue-monitoring/components/FlowStateBanner';
import { FilterSearchInput } from '@/features/queue-monitoring/components/FilterSearchInput';
import { useProjectQueueData } from '@/features/queue-monitoring/hooks/useProjectQueueData';
import { FilterSelect } from '@/features/queue-monitoring/components/FilterSelect';
import { PaginationControls } from '@/features/queue-monitoring/components/PaginationControls';
import { TableState } from '@/features/queue-monitoring/components/TableState';
import { canDelete, canRetry } from '@/features/queue-monitoring/services/actionGuards';
import { queueMonitoringRu } from '@/features/queue-monitoring/services/i18n-ru';
import { resolveQueueTab } from '@/features/queue-monitoring/services/primitives';
import {
  LINK_QUEUE_FILTER_KEYS,
  getLinkQueueStatusLabel,
  getProjectQueueActiveStatusLabel,
  getProjectQueueHistoryStatusLabel,
} from '@/features/queue-monitoring/services/statusMeta';

const statusOptions = ['all', 'pending', 'queued'];
const STATUS_FILTER_OPTIONS = statusOptions.map((value) => ({
  value,
  label: getProjectQueueActiveStatusLabel(value),
}));

const historyStatusOptions = ['all', 'completed', 'failed'];
const HISTORY_STATUS_FILTER_OPTIONS = historyStatusOptions.map((value) => ({
  value,
  label: getProjectQueueHistoryStatusLabel(value),
}));

const linkStatusOptions = [...LINK_QUEUE_FILTER_KEYS];
const LINK_STATUS_FILTER_OPTIONS = linkStatusOptions.map((value) => ({
  value,
  label: getLinkQueueStatusLabel(value),
}));

export default function ProjectQueuePage() {
  return (
    <Suspense
      fallback={
        <div className="p-6 text-sm text-slate-500 dark:text-slate-400">Загрузка очереди...</div>
      }>
      <ProjectQueuePageContent />
    </Suspense>
  );
}

function ProjectQueuePageContent() {
  useAuthGuard();
  const params = useParams();
  const searchParams = useSearchParams();
  const paramId = params?.id as string | undefined;
  const queryId = searchParams.get('id') || undefined;
  const projectId = paramId && paramId !== '[id]' ? paramId : queryId;

  const activeTab = resolveQueueTab(searchParams.get('tab'));

  const {
    projectName,
    domains,
    loading,
    historyLoading,
    cleaning,
    refreshing,
    error,
    errorDiagnostics,
    permissionDenied,
    linkLoading,
    linkRefreshing,
    linkError,
    linkErrorDiagnostics,
    linkPermissionDenied,
    statusFilter,
    setStatusFilter,
    dateFrom,
    setDateFrom,
    dateTo,
    setDateTo,
    search,
    setSearch,
    genPage,
    setGenPage,
    historyStatusFilter,
    setHistoryStatusFilter,
    historyDateFrom,
    setHistoryDateFrom,
    historyDateTo,
    setHistoryDateTo,
    historySearch,
    setHistorySearch,
    historyPage,
    setHistoryPage,
    linkStatusFilter,
    setLinkStatusFilter,
    linkDateFrom,
    setLinkDateFrom,
    linkDateTo,
    setLinkDateTo,
    linkSearch,
    setLinkSearch,
    linkPage,
    setLinkPage,
    queueFlow,
    linkFlow,
    genHasNext,
    historyHasNext,
    linkHasNext,
    visibleItems,
    visibleHistoryItems,
    visibleLinks,
    cleanupGuard,
    refreshGuard,
    linkRefreshGuard,
    handleRemove,
    handleLinkRetry,
    handleLinkDelete,
    handleCleanup,
    handleRefresh,
    handleLinkRefresh,
    removeQueueItemLockKey,
    linkRetryLockKey,
    linkDeleteLockKey,
    isLocked,
    lockReason,
  } = useProjectQueueData(projectId);

  // Стили для таблиц в едином дизайне проекта
  const tableWrapperClass =
    'bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden animate-in fade-in';
  const tableHeaderClass =
    'text-left text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 bg-white dark:bg-[#0f1117] border-b border-slate-200 dark:border-slate-800/80';
  const tableRowClass =
    'border-b border-slate-100 dark:border-slate-800/40 hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors';

  // Умное скрытие баннеров (показываем только если грузится или ошибка)
  const showQueueBanner = queueFlow.flow.status !== 'idle' && queueFlow.flow.status !== 'done';
  const showLinkBanner = linkFlow.flow.status !== 'idle' && linkFlow.flow.status !== 'done';

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER: Хлебные крошки и переключатель табов */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <Link href="/projects" className="hover:text-indigo-600 transition-colors">
                Проекты
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <Link
                href={projectId ? `/projects/${projectId}` : '/projects'}
                className="hover:text-indigo-600 transition-colors">
                {projectName || 'Проект'}
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <span className="text-slate-900 dark:text-slate-200 font-medium">Очереди</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              Центр очередей
            </h1>
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={handleCleanup}
              disabled={cleanupGuard.disabled}
              title={cleanupGuard.reason}
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl border border-amber-200 bg-amber-50 text-sm font-semibold text-amber-700 hover:bg-amber-100 dark:bg-amber-500/10 dark:border-amber-500/30 dark:text-amber-400 transition-colors">
              <Trash2 className={`w-4 h-4 ${cleaning ? 'animate-pulse' : ''}`} />
              <span className="hidden sm:inline">
                {cleaning ? 'Очистка...' : 'Удалить зависшие'}
              </span>
            </button>
            <button
              onClick={activeTab === 'links' ? handleLinkRefresh : handleRefresh}
              disabled={activeTab === 'links' ? linkRefreshGuard.disabled : refreshGuard.disabled}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-500 transition-all shadow-sm active:scale-95">
              <RefreshCw
                className={`w-4 h-4 ${refreshing || linkRefreshing ? 'animate-spin' : ''}`}
              />
              Обновить
            </button>
          </div>
        </div>

        {/* TABS (Встроены в шапку) */}
        <div className="max-w-7xl mx-auto mt-6 flex items-center gap-6 border-b border-slate-200 dark:border-slate-800">
          <TabLink
            href={projectId ? `/projects/${projectId}/queue?tab=domains` : '/projects'}
            label="Генерация сайтов"
            icon={<ListFilter />}
            active={activeTab === 'domains'}
          />
          <TabLink
            href={projectId ? `/projects/${projectId}/queue?tab=links` : '/projects'}
            label="Очередь ссылок"
            icon={<Database />}
            active={activeTab === 'links'}
          />
        </div>
      </header>

      {/* CONTENT AREA */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-6">
          {/* ГЛОБАЛЬНЫЕ ОШИБКИ И БАННЕРЫ */}
          {error && (
            <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 flex flex-col gap-2">
              <div className="flex items-center gap-2 font-bold">
                <AlertTriangle className="w-5 h-5" /> {error}
              </div>
              {errorDiagnostics && (
                <code className="text-xs opacity-80 block p-2 bg-black/5 dark:bg-black/30 rounded-lg">
                  {errorDiagnostics}
                </code>
              )}
            </div>
          )}
          {permissionDenied && (
            <div className="p-4 bg-amber-50 text-amber-700 rounded-xl text-sm border border-amber-200 dark:bg-amber-900/20 dark:border-amber-800">
              Недостаточно прав для просмотра очереди.
            </div>
          )}

          {/* Плавающие уведомления (Только во время работы) */}
          {(showQueueBanner || showLinkBanner) && (
            <div className="grid gap-4 md:grid-cols-2">
              {showQueueBanner && (
                <FlowStateBanner title={queueMonitoringRu.flowTitles.queue} flow={queueFlow.flow} />
              )}
              {showLinkBanner && (
                <FlowStateBanner title={queueMonitoringRu.flowTitles.links} flow={linkFlow.flow} />
              )}
            </div>
          )}

          {/* ========================================= */}
          {/* ВКЛАДКА: ГЕНЕРАЦИЯ (Домены)               */}
          {/* ========================================= */}
          {activeTab === 'domains' && (
            <div className="space-y-8 animate-in fade-in duration-300">
              {/* 1. АКТИВНАЯ ОЧЕРЕДЬ */}
              <div className={tableWrapperClass}>
                <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex flex-col xl:flex-row xl:items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-lg bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                      <Activity className="w-4 h-4" />
                    </div>
                    <div>
                      <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        Активная очередь{' '}
                        <Badge label={visibleItems.length.toString()} tone="indigo" />
                      </h3>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Сайты, ожидающие генерации или находящиеся в работе.
                      </p>
                    </div>
                  </div>
                  {/* ФИЛЬТРЫ */}
                  <div className="flex flex-wrap items-center gap-2">
                    <FilterSelect
                      value={statusFilter}
                      options={STATUS_FILTER_OPTIONS}
                      onChange={setStatusFilter}
                    />
                    <FilterDateInput value={dateFrom} onChange={setDateFrom} placeholder="С" />
                    <FilterDateInput value={dateTo} onChange={setDateTo} placeholder="По" />
                    <FilterSearchInput value={search} onChange={setSearch} placeholder="Поиск..." />
                  </div>
                </div>

                {!permissionDenied && (
                  <TableState
                    loading={loading}
                    empty={!loading && visibleItems.length === 0}
                    emptyText="Активных задач нет."
                    className='py-3 px-5'
                  />
                )}

                {!loading && !permissionDenied && visibleItems.length > 0 && (
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead>
                        <tr className={tableHeaderClass}>
                          <th className="py-3 px-5">Сайт</th>
                          <th className="py-3 px-5">Запланировано</th>
                          <th className="py-3 px-5">Статус</th>
                          <th className="py-3 px-5">Приоритет</th>
                          <th className="py-3 px-5 text-right">Действия</th>
                        </tr>
                      </thead>
                      <tbody>
                        {visibleItems.map((item) => {
                          const domain = domains[item.domain_id];
                          const domainLabel = item.domain_url || domain?.url || 'Домен';
                          const removeGuard = canDelete({
                            busy: loading || isLocked(removeQueueItemLockKey(item.id)),
                            allowed: true,
                          });
                          return (
                            <tr key={item.id} className={tableRowClass}>
                              <td className="py-3 px-5 font-semibold">
                                <Link
                                  href={`/domains/${domain?.id || item.domain_id}`}
                                  className="text-indigo-600 dark:text-indigo-400 hover:underline">
                                  {domainLabel}
                                </Link>
                              </td>
                              <td className="py-3 px-5 text-slate-500 text-xs">
                                <div>{new Date(item.scheduled_for).toLocaleString()}</div>
                                {item.processed_at && (
                                  <div className="mt-1 text-indigo-500">
                                    Запуск: {new Date(item.processed_at).toLocaleString()}
                                  </div>
                                )}
                              </td>
                              <td className="py-3 px-5">
                                <Badge
                                  label={getProjectQueueActiveStatusLabel(item.status)}
                                  tone="amber"
                                />
                              </td>
                              <td className="py-3 px-5 text-slate-500">{item.priority}</td>
                              <td className="py-3 px-5 text-right">
                                <button
                                  onClick={() => handleRemove(item)}
                                  disabled={removeGuard.disabled}
                                  className="p-2 text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-lg transition-colors"
                                  title="Удалить из очереди">
                                  <Trash2 className="w-4 h-4" />
                                </button>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
                {visibleItems.length > 0 && (
                  <div className="p-4 border-t border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/10">
                    <PaginationControls
                      page={genPage}
                      hasNext={genHasNext}
                      onPrev={() => setGenPage((p) => Math.max(1, p - 1))}
                      onNext={() => setGenPage((p) => p + 1)}
                    />
                  </div>
                )}
              </div>

              {/* 2. ИСТОРИЯ ЗАПУСКОВ */}
              <div className={tableWrapperClass}>
                <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex flex-col xl:flex-row xl:items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-lg bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-slate-600 dark:text-slate-300">
                      <Archive className="w-4 h-4" />
                    </div>
                    <div>
                      <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                        История запусков{' '}
                        <Badge label={visibleHistoryItems.length.toString()} tone="slate" />
                      </h3>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Архив завершенных или упавших генераций.
                      </p>
                    </div>
                  </div>
                  {/* ФИЛЬТРЫ */}
                  <div className="flex flex-wrap items-center gap-2">
                    <FilterSelect
                      value={historyStatusFilter}
                      options={HISTORY_STATUS_FILTER_OPTIONS}
                      onChange={setHistoryStatusFilter}
                    />
                    <FilterDateInput
                      value={historyDateFrom}
                      onChange={setHistoryDateFrom}
                      placeholder="С"
                    />
                    <FilterDateInput
                      value={historyDateTo}
                      onChange={setHistoryDateTo}
                      placeholder="По"
                    />
                    <FilterSearchInput
                      value={historySearch}
                      onChange={setHistorySearch}
                      placeholder="Поиск..."
                    />
                  </div>
                </div>

                {!permissionDenied && (
                  <TableState
                    loading={historyLoading}
                    empty={!historyLoading && visibleHistoryItems.length === 0}
                    emptyText="История пуста."
                    className='py-3 px-5'
                  />
                )}

                {!historyLoading && !permissionDenied && visibleHistoryItems.length > 0 && (
                  <div className="overflow-x-auto">
                    <table className="min-w-full text-sm">
                      <thead>
                        <tr className={tableHeaderClass}>
                          <th className="py-3 px-5">Сайт</th>
                          <th className="py-3 px-5">Тайминг</th>
                          <th className="py-3 px-5">Статус</th>
                          <th className="py-3 px-5">Детали (Ошибки)</th>
                        </tr>
                      </thead>
                      <tbody>
                        {visibleHistoryItems.map((item) => {
                          const domain = domains[item.domain_id];
                          const domainLabel = item.domain_url || domain?.url || 'Домен';
                          const isError = item.status === 'failed';
                          return (
                            <tr key={item.id} className={tableRowClass}>
                              <td className="py-3 px-5 font-semibold">
                                <Link
                                  href={`/domains/${domain?.id || item.domain_id}`}
                                  className="text-indigo-600 dark:text-indigo-400 hover:underline">
                                  {domainLabel}
                                </Link>
                              </td>
                              <td className="py-3 px-5 text-slate-500 text-xs space-y-1">
                                <div>План: {new Date(item.scheduled_for).toLocaleString()}</div>
                                {item.processed_at && (
                                  <div>
                                    Завершено: {new Date(item.processed_at).toLocaleString()}
                                  </div>
                                )}
                              </td>
                              <td className="py-3 px-5">
                                <Badge
                                  label={getProjectQueueHistoryStatusLabel(item.status)}
                                  tone={isError ? 'red' : 'green'}
                                />
                              </td>
                              <td
                                className={`py-3 px-5 max-w-sm truncate ${isError ? 'text-red-500 font-mono text-[11px]' : 'text-slate-500 text-xs'}`}
                                title={formatQueueHistoryDetails(item)}>
                                {formatQueueHistoryDetails(item)}
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
                {visibleHistoryItems.length > 0 && (
                  <div className="p-4 border-t border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/10">
                    <PaginationControls
                      page={historyPage}
                      hasNext={historyHasNext}
                      onPrev={() => setHistoryPage((p) => Math.max(1, p - 1))}
                      onNext={() => setHistoryPage((p) => p + 1)}
                    />
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ========================================= */}
          {/* ВКЛАДКА: ССЫЛКИ                           */}
          {/* ========================================= */}
          {activeTab === 'links' && (
            <div className={`${tableWrapperClass} animate-in fade-in duration-300`}>
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex flex-col xl:flex-row xl:items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-lg bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                    <Database className="w-4 h-4" />
                  </div>
                  <div>
                    <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                      Очередь ссылок <Badge label={visibleLinks.length.toString()} tone="indigo" />
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Задачи на вставку и удаление ссылок (Link Flow).
                    </p>
                  </div>
                </div>
                {/* ФИЛЬТРЫ */}
                <div className="flex flex-wrap items-center gap-2">
                  <FilterSelect
                    value={linkStatusFilter}
                    options={LINK_STATUS_FILTER_OPTIONS}
                    onChange={setLinkStatusFilter}
                  />
                  <FilterDateInput
                    value={linkDateFrom}
                    onChange={setLinkDateFrom}
                    placeholder="С"
                  />
                  <FilterDateInput value={linkDateTo} onChange={setLinkDateTo} placeholder="По" />
                  <FilterSearchInput
                    value={linkSearch}
                    onChange={setLinkSearch}
                    placeholder="Поиск..."
                  />
                </div>
              </div>

              {linkPermissionDenied && (
                <div className="p-6 text-amber-600 text-sm">Недостаточно прав.</div>
              )}
              {!linkPermissionDenied && (
                <TableState
                  loading={linkLoading}
                  error={linkError}
                  empty={!linkLoading && visibleLinks.length === 0}
                  emptyText="Очередь ссылок пуста."
                />
              )}

              {!linkLoading && !linkError && !linkPermissionDenied && visibleLinks.length > 0 && (
                <div className="overflow-x-auto">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className={tableHeaderClass}>
                        <th className="py-3 px-5">Сайт / Действие</th>
                        <th className="py-3 px-5">Тайминг</th>
                        <th className="py-3 px-5">Статус</th>
                        <th className="py-3 px-5">Событие (Лог)</th>
                        <th className="py-3 px-5 text-right">Действия</th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleLinks.map((task) => {
                        const domain = domains[task.domain_id];
                        const domainLabel = domain?.url || 'Домен';
                        const actionLabel =
                          (task.action || 'insert') === 'remove' ? 'Удаление' : 'Вставка';
                        const normalizedStatus =
                          normalizeLinkTaskStatus(task.status) || task.status;
                        const canRetryByStatus = canRetryLinkTask(task.status);
                        const retryGuard = canRetry({
                          busy: linkLoading || isLocked(linkRetryLockKey(task.id)),
                          allowed: canRetryByStatus,
                        });
                        const deleteGuard = canDelete({
                          busy: linkLoading || isLocked(linkDeleteLockKey(task.id)),
                          status: task.status,
                        });
                        const lastLog = task.log_lines?.length
                          ? task.log_lines[task.log_lines.length - 1]
                          : '';
                        const eventText = task.error_message || lastLog || '—';
                        const isError = normalizedStatus === 'failed';

                        return (
                          <tr key={task.id} className={tableRowClass}>
                            <td className="py-3 px-5">
                              <Link
                                href={`/domains/${domain?.id || task.domain_id}`}
                                className="font-semibold text-indigo-600 dark:text-indigo-400 hover:underline block mb-1">
                                {domainLabel}
                              </Link>
                              <Badge
                                label={actionLabel}
                                tone={task.action === 'remove' ? 'red' : 'blue'}
                                className="text-[10px] px-1.5 py-0"
                              />
                            </td>
                            <td className="py-3 px-5 text-slate-500 text-xs space-y-1">
                              <div>{new Date(task.scheduled_for).toLocaleString()}</div>
                              <div className="opacity-70">Попыток: {task.attempts}</div>
                            </td>
                            <td className="py-3 px-5">
                              <LinkTaskStatusBadge status={normalizedStatus} />
                            </td>
                            <td
                              className={`py-3 px-5 max-w-[250px] truncate ${isError ? 'text-red-500 font-mono text-[11px]' : 'text-slate-500 text-xs'}`}
                              title={eventText}>
                              {eventText}
                            </td>
                            <td className="py-3 px-5 text-right">
                              <div className="flex items-center justify-end gap-2">
                                <Link
                                  href={`/links/${task.id}`}
                                  className="px-3 py-1.5 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg dark:bg-slate-800 dark:text-slate-300 transition-colors">
                                  Детали
                                </Link>
                                {retryGuard.enabled && (
                                  <button
                                    onClick={() => handleLinkRetry(task)}
                                    disabled={retryGuard.disabled}
                                    className="p-2 text-amber-500 hover:bg-amber-50 dark:hover:bg-amber-900/20 rounded-lg transition-colors"
                                    title="Повторить">
                                    <RotateCw className="w-4 h-4" />
                                  </button>
                                )}
                                <button
                                  onClick={() => handleLinkDelete(task)}
                                  disabled={deleteGuard.disabled}
                                  className="p-2 text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-colors"
                                  title="Удалить">
                                  <Trash2 className="w-4 h-4" />
                                </button>
                              </div>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
              {visibleLinks.length > 0 && (
                <div className="p-4 border-t border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/10">
                  <PaginationControls
                    page={linkPage}
                    hasNext={linkHasNext}
                    onPrev={() => setLinkPage((p) => Math.max(1, p - 1))}
                    onNext={() => setLinkPage((p) => p + 1)}
                  />
                </div>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

// --- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---

function formatQueueHistoryDetails(item: QueueItemDTO): string {
  const text = (item.error_message || '').trim();
  if (!text) return '—';
  if (item.status === 'completed' && text.toLowerCase() === 'generation enqueued')
    return 'Поставлено в генерацию';
  return text;
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon =
    meta.icon === 'refresh' ? (
      <RefreshCw className="w-3 h-3" />
    ) : meta.icon === 'check' ? (
      <Check className="w-3 h-3" />
    ) : meta.icon === 'alert' ? (
      <AlertTriangle className="w-3 h-3" />
    ) : (
      <Clock className="w-3 h-3" />
    );
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

function TabLink({
  href,
  label,
  icon,
  active,
}: {
  href: any;
  label: string;
  icon: ReactNode;
  active: boolean;
}) {
  return (
    <Link
      href={href}
      className={`flex items-center gap-2 pb-4 px-1 text-sm font-medium border-b-2 transition-all ${active ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400' : 'border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'}`}>
      {icon && <span className="flex-shrink-0 opacity-80 [&>svg]:w-4 [&>svg]:h-4">{icon}</span>}
      {label}
    </Link>
  );
}
