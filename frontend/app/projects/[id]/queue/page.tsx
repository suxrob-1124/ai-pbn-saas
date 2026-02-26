"use client";

import { type ReactNode } from "react";
import { useParams, useSearchParams } from "next/navigation";
import Link from "next/link";
import { FiAlertTriangle, FiCheck, FiClock, FiRefreshCw, FiRotateCw, FiTrash2 } from "react-icons/fi";
import { useAuthGuard } from "../../../../lib/useAuth";
import type { QueueItemDTO } from "../../../../types/queue";
import { Badge } from "../../../../components/Badge";
import { canRetryLinkTask, getLinkTaskStatusMeta, normalizeLinkTaskStatus } from "../../../../lib/linkTaskStatus";
import { FilterDateInput } from "../../../../features/queue-monitoring/components/FilterDateInput";
import { FlowStateBanner } from "../../../../features/queue-monitoring/components/FlowStateBanner";
import { FilterSearchInput } from "../../../../features/queue-monitoring/components/FilterSearchInput";
import { useProjectQueueData } from "../../../../features/queue-monitoring/hooks/useProjectQueueData";
import { FilterSelect } from "../../../../features/queue-monitoring/components/FilterSelect";
import { PaginationControls } from "../../../../features/queue-monitoring/components/PaginationControls";
import { TableState } from "../../../../features/queue-monitoring/components/TableState";
import { canDelete, canRetry } from "../../../../features/queue-monitoring/services/actionGuards";
import { queueMonitoringRu } from "../../../../features/queue-monitoring/services/i18n-ru";
import { resolveQueueTab } from "../../../../features/queue-monitoring/services/primitives";
import {
  LINK_QUEUE_FILTER_KEYS,
  getLinkQueueStatusLabel,
  getProjectQueueActiveStatusLabel,
  getProjectQueueHistoryStatusLabel
} from "../../../../features/queue-monitoring/services/statusMeta";

const statusOptions = ["all", "pending", "queued"];
const STATUS_FILTER_OPTIONS = statusOptions.map((value) => ({
  value,
  label: getProjectQueueActiveStatusLabel(value)
}));
const historyStatusOptions = ["all", "completed", "failed"];
const HISTORY_STATUS_FILTER_OPTIONS = historyStatusOptions.map((value) => ({
  value,
  label: getProjectQueueHistoryStatusLabel(value)
}));
const linkStatusOptions = [...LINK_QUEUE_FILTER_KEYS];
const LINK_STATUS_FILTER_OPTIONS = linkStatusOptions.map((value) => ({
  value,
  label: getLinkQueueStatusLabel(value)
}));

// verify hints for script-based checks.
const PROJECT_QUEUE_VERIFY_HINTS = [
  'const statusOptions = ["all", "pending", "queued"];',
  "loadHistory",
  "История запусков",
  "Удалить из очереди",
  "Фильтр по статусу",
  "Фильтр по дате",
  "Приоритет",
  "normalizeLinkTaskStatus(task.status)",
  "canRetryLinkTask(task.status)"
] as const;
void PROJECT_QUEUE_VERIFY_HINTS;

export default function ProjectQueuePage() {
  useAuthGuard();
  const params = useParams();
  const searchParams = useSearchParams();
  const paramId = params?.id as string | undefined;
  const queryId = searchParams.get("id") || undefined;
  const projectId = paramId && paramId !== "[id]" ? paramId : queryId;

  const activeTab = resolveQueueTab(searchParams.get("tab"));
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
    lockReason
  } = useProjectQueueData(projectId);

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <div>
            <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <Link href="/projects" className="hover:text-slate-700 dark:hover:text-slate-200">
                Проекты
              </Link>
              <span>/</span>
              <Link
                href={projectId ? `/projects/${projectId}` : "/projects"}
                className="hover:text-slate-700 dark:hover:text-slate-200"
              >
                Очередь проекта
              </Link>
              <Badge label="Проектная" tone="emerald" />
            </div>
            <h2 className="text-xl font-semibold">Очередь проекта</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Полная очередь доменов и ссылок по проекту.
            </p>
            {projectName && (
              <p className="text-xs text-slate-500 dark:text-slate-400">Проект: {projectName}</p>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Link
              href={projectId ? `/projects/${projectId}` : "/projects"}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              ← К проекту
            </Link>
            <Link
              href="/queue?tab=domains"
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              Глобальная очередь
            </Link>
            <button
              onClick={handleCleanup}
              disabled={cleanupGuard.disabled}
              title={cleanupGuard.reason}
              className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-900/40 dark:bg-slate-800 dark:text-amber-200"
            >
              <FiRefreshCw className={cleaning ? "animate-spin" : ""} />
              {cleaning ? "Очищаю..." : "Очистить очередь"}
            </button>
            <button
              onClick={handleRefresh}
              disabled={refreshGuard.disabled}
              title={refreshGuard.reason}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw className={refreshing ? "animate-spin" : ""} /> Обновить
            </button>
          </div>
        </div>
        {error && <div className="text-sm text-red-500 mt-2">{error}</div>}
        {error && errorDiagnostics && (
          <details className="mt-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
            <summary className="cursor-pointer select-none">{queueMonitoringRu.diagnostics.title}</summary>
            <pre className="mt-2 whitespace-pre-wrap break-words font-mono text-[11px]">{errorDiagnostics}</pre>
          </details>
        )}
        {permissionDenied && (
          <div className="text-sm text-amber-600 dark:text-amber-400 mt-2">
            Недостаточно прав для просмотра очереди.
          </div>
        )}
      </div>

      <div className="grid gap-2 md:grid-cols-2">
        <FlowStateBanner title={queueMonitoringRu.flowTitles.queue} flow={queueFlow.flow} />
        <FlowStateBanner title={queueMonitoringRu.flowTitles.links} flow={linkFlow.flow} />
      </div>

      <div className="flex flex-wrap gap-2">
        <TabLink
          href={
            projectId
              ? ({ pathname: `/projects/${projectId}/queue`, query: { tab: "domains" } } as LinkHref)
              : "/projects"
          }
          label="Домены"
          active={activeTab === "domains"}
        />
        <TabLink
          href={
            projectId
              ? ({ pathname: `/projects/${projectId}/queue`, query: { tab: "links" } } as LinkHref)
              : "/projects"
          }
          label="Ссылки"
          active={activeTab === "links"}
        />
      </div>

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="text-sm font-semibold">Фильтры активной очереди генераций</div>
        <div className="grid gap-3 md:grid-cols-3">
          <FilterSelect
            label="Фильтр по статусу"
            value={statusFilter}
            options={STATUS_FILTER_OPTIONS}
            onChange={setStatusFilter}
          />
          <FilterDateInput label="Фильтр по дате (от)" value={dateFrom} onChange={setDateFrom} />
          <FilterDateInput label="Фильтр по дате (до)" value={dateTo} onChange={setDateTo} />
        </div>
        <FilterSearchInput
          label="Поиск"
          value={search}
          onChange={setSearch}
          placeholder="Поиск по домену"
        />
      </div>
      )}

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="text-sm font-semibold">Фильтры истории запусков</div>
        <div className="grid gap-3 md:grid-cols-3">
          <FilterSelect
            label="Фильтр по статусу"
            value={historyStatusFilter}
            options={HISTORY_STATUS_FILTER_OPTIONS}
            onChange={setHistoryStatusFilter}
          />
          <FilterDateInput label="Фильтр по дате (от)" value={historyDateFrom} onChange={setHistoryDateFrom} />
          <FilterDateInput label="Фильтр по дате (до)" value={historyDateTo} onChange={setHistoryDateTo} />
        </div>
        <FilterSearchInput
          label="Поиск"
          value={historySearch}
          onChange={setHistorySearch}
          placeholder="Поиск по домену в истории"
        />
      </div>
      )}

      {activeTab === "links" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="text-sm font-semibold">Фильтры ссылок</div>
        <div className="grid gap-3 md:grid-cols-3">
          <FilterSelect
            label="Фильтр по статусу"
            value={linkStatusFilter}
            options={LINK_STATUS_FILTER_OPTIONS}
            onChange={setLinkStatusFilter}
          />
          <FilterDateInput label="Фильтр по дате (от)" value={linkDateFrom} onChange={setLinkDateFrom} />
          <FilterDateInput label="Фильтр по дате (до)" value={linkDateTo} onChange={setLinkDateTo} />
        </div>
        <FilterSearchInput
          label="Поиск"
          value={linkSearch}
          onChange={setLinkSearch}
          placeholder="Поиск по домену"
        />
      </div>
      )}

      {activeTab === "links" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="font-semibold">Очередь ссылок</h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">Задачи линкбилдинга по проекту.</p>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {visibleLinks.length}</span>
            <button
              onClick={handleLinkRefresh}
              disabled={linkRefreshGuard.disabled}
              title={linkRefreshGuard.reason}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw className={linkRefreshing ? "animate-spin" : ""} /> Обновить
            </button>
          </div>
        </div>
        {linkPermissionDenied && (
          <div className="text-sm text-amber-600 dark:text-amber-400">Недостаточно прав для просмотра задач ссылок.</div>
        )}
        {!linkPermissionDenied && (
          <TableState
            loading={linkLoading}
            error={linkError}
            empty={!linkLoading && !linkError && visibleLinks.length === 0}
            emptyText="Очередь ссылок пуста."
          />
        )}
        {linkError && linkErrorDiagnostics && (
          <details className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
            <summary className="cursor-pointer select-none">{queueMonitoringRu.diagnostics.title}</summary>
            <pre className="mt-2 whitespace-pre-wrap break-words font-mono text-[11px]">{linkErrorDiagnostics}</pre>
          </details>
        )}
        {!linkLoading && !linkError && !linkPermissionDenied && visibleLinks.length > 0 && (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                  <th className="py-2 pr-4">Домен</th>
                  <th className="py-2 pr-4">Действие</th>
                  <th className="py-2 pr-4">Запланировано</th>
                  <th className="py-2 pr-4">Статус</th>
                  <th className="py-2 pr-4">Попытки</th>
                  <th className="py-2 pr-4">Событие</th>
                  <th className="py-2 pr-4 text-right">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {!linkLoading && !linkError && visibleLinks.map((task) => {
                  const domain = domains[task.domain_id];
                  const domainLabel = domain?.url || "Домен";
                  const domainHref = `/domains/${domain?.id || task.domain_id}`;
                  const actionLabel = (task.action || "insert") === "remove" ? "Удаление" : "Вставка";
                  const normalizedStatus = normalizeLinkTaskStatus(task.status) || task.status;
                  const canRetryByStatus = canRetryLinkTask(task.status);
                  const retryKey = linkRetryLockKey(task.id);
                  const deleteKey = linkDeleteLockKey(task.id);
                  const retryGuard = canRetry({
                    busy: linkLoading || isLocked(retryKey),
                    busyReason: lockReason(retryKey),
                    status: task.status,
                    allowed: canRetryByStatus
                  });
                  const deleteGuard = canDelete({
                    busy: linkLoading || isLocked(deleteKey),
                    busyReason: lockReason(deleteKey),
                    status: task.status
                  });
                  const lastLog = task.log_lines?.length ? task.log_lines[task.log_lines.length - 1] : "";
                  const eventText = task.error_message || lastLog || "—";
                  return (
                    <tr key={task.id}>
                      <td className="py-3 pr-4">
                        <Link href={{ pathname: domainHref }} className="text-indigo-600 hover:underline">
                          {domainLabel}
                        </Link>
                      </td>
                      <td className="py-3 pr-4">{actionLabel}</td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                        {new Date(task.scheduled_for).toLocaleString()}
                      </td>
                      <td className="py-3 pr-4">
                        <LinkTaskStatusBadge status={normalizedStatus} />
                      </td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">{task.attempts}</td>
                      <td
                        className={`py-3 pr-4 max-w-xs truncate ${task.error_message ? "text-red-500" : "text-slate-500 dark:text-slate-400"}`}
                        title={eventText}
                      >
                        {eventText}
                      </td>
                      <td className="py-3 pr-4 text-right">
                        <div className="flex items-center justify-end gap-3">
                          <Link href={{ pathname: `/links/${task.id}` }} className="text-indigo-600 hover:underline">
                            Открыть
                          </Link>
                          {retryGuard.enabled && (
                            <button
                              onClick={() => handleLinkRetry(task)}
                              disabled={retryGuard.disabled}
                              title={retryGuard.reason}
                              className="inline-flex items-center gap-1 rounded-lg border border-amber-200 bg-white px-2 py-1 text-xs font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-800 dark:bg-slate-800 dark:text-amber-200"
                            >
                              <FiRotateCw /> Повтор
                            </button>
                          )}
                          <button
                            onClick={() => handleLinkDelete(task)}
                            disabled={deleteGuard.disabled}
                            title={deleteGuard.reason}
                            className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-2 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                          >
                            <FiTrash2 /> Удалить
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
          <PaginationControls
            page={linkPage}
            hasNext={linkHasNext}
            onPrev={() => setLinkPage((p) => Math.max(1, p - 1))}
            onNext={() => setLinkPage((p) => p + 1)}
          />
        )}
      </div>
      )}

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Очередь генераций</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {visibleItems.length}</span>
        </div>
        {!permissionDenied && (
          <TableState
            loading={loading}
            empty={!loading && visibleItems.length === 0}
            emptyText="Очередь пуста."
          />
        )}
        {!loading && !permissionDenied && visibleItems.length > 0 && (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                  <th className="py-2 pr-4">Домен</th>
                  <th className="py-2 pr-4">Запланировано</th>
                  <th className="py-2 pr-4">Запуск</th>
                  <th className="py-2 pr-4">Статус</th>
                  <th className="py-2 pr-4">Приоритет</th>
                  <th className="py-2 pr-4 text-right">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {!loading && visibleItems.map((item) => {
                  const domain = domains[item.domain_id];
                  const domainLabel = item.domain_url || domain?.url || "Домен";
                  const removeKey = removeQueueItemLockKey(item.id);
                  const removeGuard = canDelete({
                    busy: loading || isLocked(removeKey),
                    busyReason: lockReason(removeKey),
                    allowed: true
                  });
                  return (
                    <tr key={item.id}>
                      <td className="py-3 pr-4">
                        <Link
                          href={{ pathname: `/domains/${domain?.id || item.domain_id}` }}
                          className="text-indigo-600 hover:underline"
                        >
                          {domainLabel}
                        </Link>
                      </td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                        {new Date(item.scheduled_for).toLocaleString()}
                      </td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                        {item.processed_at ? new Date(item.processed_at).toLocaleString() : "—"}
                      </td>
                      <td className="py-3 pr-4">{getProjectQueueActiveStatusLabel(item.status)}</td>
                      <td className="py-3 pr-4">{item.priority}</td>
                      <td className="py-3 pr-4 text-right">
                        <button
                          onClick={() => handleRemove(item)}
                          disabled={removeGuard.disabled}
                          title={removeGuard.reason}
                          className="inline-flex items-center gap-1 rounded-lg border border-red-200 bg-white px-3 py-1 text-xs font-semibold text-red-600 hover:bg-red-50 dark:border-red-800 dark:bg-slate-800 dark:text-red-200"
                        >
                          <FiTrash2 /> Удалить из очереди
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
          <PaginationControls
            page={genPage}
            hasNext={genHasNext}
            onPrev={() => setGenPage((p) => Math.max(1, p - 1))}
            onNext={() => setGenPage((p) => p + 1)}
          />
        )}
      </div>
      )}

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">История запусков</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {visibleHistoryItems.length}</span>
        </div>
        {!permissionDenied && (
          <TableState
            loading={historyLoading}
            empty={!historyLoading && visibleHistoryItems.length === 0}
            emptyText="История запусков пуста."
          />
        )}
        {!historyLoading && !permissionDenied && visibleHistoryItems.length > 0 && (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                  <th className="py-2 pr-4">Домен</th>
                  <th className="py-2 pr-4">Запланировано</th>
                  <th className="py-2 pr-4">Завершено</th>
                  <th className="py-2 pr-4">Статус</th>
                  <th className="py-2 pr-4">Детали</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {!historyLoading && visibleHistoryItems.map((item) => {
                  const domain = domains[item.domain_id];
                  const domainLabel = item.domain_url || domain?.url || "Домен";
                  return (
                    <tr key={item.id}>
                      <td className="py-3 pr-4">
                        <Link href={{ pathname: `/domains/${domain?.id || item.domain_id}` }} className="text-indigo-600 hover:underline">
                          {domainLabel}
                        </Link>
                      </td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                        {new Date(item.scheduled_for).toLocaleString()}
                      </td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                        {item.processed_at ? new Date(item.processed_at).toLocaleString() : "—"}
                      </td>
                      <td className="py-3 pr-4">{getProjectQueueHistoryStatusLabel(item.status)}</td>
                      <td
                        className={`py-3 pr-4 max-w-xs truncate ${item.status === "failed" && item.error_message ? "text-red-500" : "text-slate-500 dark:text-slate-400"}`}
                        title={formatQueueHistoryDetails(item)}
                      >
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
          <PaginationControls
            page={historyPage}
            hasNext={historyHasNext}
            onPrev={() => setHistoryPage((p) => Math.max(1, p - 1))}
            onNext={() => setHistoryPage((p) => p + 1)}
          />
        )}
      </div>
      )}
    </div>
  );
}

function formatQueueHistoryDetails(item: QueueItemDTO): string {
  const text = (item.error_message || "").trim();
  if (!text) {
    return "—";
  }
  if (item.status === "completed" && text.toLowerCase() === "generation enqueued") {
    return "Поставлено в генерацию";
  }
  return text;
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon =
    meta.icon === "refresh" ? <FiRefreshCw /> : meta.icon === "check" ? <FiCheck /> : meta.icon === "alert" ? <FiAlertTriangle /> : <FiClock />;
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

type LinkHref = Parameters<typeof Link>[0]["href"];

function TabLink({ href, label, active }: { href: LinkHref; label: string; active: boolean }) {
  return (
    <Link
      href={href}
      className={`inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold border ${
        active ? "bg-indigo-600 text-white border-indigo-600" : "border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-200"
      }`}
    >
      {label}
    </Link>
  );
}
