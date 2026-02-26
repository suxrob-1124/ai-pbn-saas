"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useParams, useSearchParams } from "next/navigation";
import Link from "next/link";
import { FiAlertTriangle, FiCheck, FiClock, FiRefreshCw, FiRotateCw, FiTrash2 } from "react-icons/fi";
import { cleanupQueue, deleteQueueItem, listQueue, listQueueHistory } from "../../../../lib/queueApi";
import { deleteLinkTask, listLinkTasks, retryLinkTask } from "../../../../lib/linkTasksApi";
import { authFetch } from "../../../../lib/http";
import { useAuthGuard } from "../../../../lib/useAuth";
import { showToast } from "../../../../lib/toastStore";
import { useActionLocks } from "../../../../features/editor-v3/hooks/useActionLocks";
import type { QueueItemDTO } from "../../../../types/queue";
import type { LinkTaskDTO } from "../../../../types/linkTasks";
import { Badge } from "../../../../components/Badge";
import { canRetryLinkTask, getLinkTaskStatusMeta, isLinkTaskInProgress, normalizeLinkTaskStatus } from "../../../../lib/linkTaskStatus";
import { FilterDateInput } from "../../../../features/queue-monitoring/components/FilterDateInput";
import { FilterSearchInput } from "../../../../features/queue-monitoring/components/FilterSearchInput";
import { FilterSelect } from "../../../../features/queue-monitoring/components/FilterSelect";
import { PaginationControls } from "../../../../features/queue-monitoring/components/PaginationControls";
import { TableState } from "../../../../features/queue-monitoring/components/TableState";
import { canDelete, canRetry, canRun } from "../../../../features/queue-monitoring/services/actionGuards";
import { matchesDateRange, matchesSearch } from "../../../../features/queue-monitoring/services/filters";
import {
  hasNextPageByPageSize,
  resolveQueueTab
} from "../../../../features/queue-monitoring/services/primitives";
import {
  LINK_QUEUE_FILTER_KEYS,
  getLinkQueueStatusLabel,
  getProjectQueueActiveStatusLabel,
  getProjectQueueHistoryStatusLabel,
  normalizeProjectQueueActiveStatus,
  normalizeProjectQueueHistoryStatus
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

type Domain = {
  id: string;
  url: string;
};

const isPermissionError = (message: string) =>
  /permission|access denied|admin only|forbidden/i.test(message);

export default function ProjectQueuePage() {
  useAuthGuard();
  const params = useParams();
  const searchParams = useSearchParams();
  const paramId = params?.id as string | undefined;
  const queryId = searchParams.get("id") || undefined;
  const projectId = paramId && paramId !== "[id]" ? paramId : queryId;

  const [items, setItems] = useState<QueueItemDTO[]>([]);
  const [historyItems, setHistoryItems] = useState<QueueItemDTO[]>([]);
  const [domains, setDomains] = useState<Record<string, Domain>>({});
  const [projectName, setProjectName] = useState("");
  const [loading, setLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [permissionDenied, setPermissionDenied] = useState(false);

  const [linkTasks, setLinkTasks] = useState<LinkTaskDTO[]>([]);
  const [linkLoading, setLinkLoading] = useState(false);
  const [linkRefreshing, setLinkRefreshing] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [linkPermissionDenied, setLinkPermissionDenied] = useState(false);

  const [statusFilter, setStatusFilter] = useState("all");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [search, setSearch] = useState("");
  const [genPage, setGenPage] = useState(1);
  const genPageSize = 20;
  const [historyStatusFilter, setHistoryStatusFilter] = useState("all");
  const [historyDateFrom, setHistoryDateFrom] = useState("");
  const [historyDateTo, setHistoryDateTo] = useState("");
  const [historySearch, setHistorySearch] = useState("");
  const [historyPage, setHistoryPage] = useState(1);
  const historyPageSize = 20;

  const [linkStatusFilter, setLinkStatusFilter] = useState("all");
  const [linkDateFrom, setLinkDateFrom] = useState("");
  const [linkDateTo, setLinkDateTo] = useState("");
  const [linkSearch, setLinkSearch] = useState("");
  const [linkPage, setLinkPage] = useState(1);
  const linkPageSize = 20;
  const activeTab = resolveQueueTab(searchParams.get("tab"));
  const { isLocked, lockReason, runLocked } = useActionLocks();

  const scopeId = projectId || "unknown";
  const cleanupLockKey = `project:${scopeId}:queue:cleanup`;
  const refreshLockKey = `project:${scopeId}:queue:refresh`;
  const linkRefreshLockKey = `project:${scopeId}:queue:links:refresh`;
  const removeQueueItemLockKey = (itemId: string) => `project:${scopeId}:queue:item:${itemId}:delete`;
  const linkRetryLockKey = (taskId: string) => `project:${scopeId}:queue:link:${taskId}:retry`;
  const linkDeleteLockKey = (taskId: string) => `project:${scopeId}:queue:link:${taskId}:delete`;

  useEffect(() => {
    let cancelled = false;
    if (!projectId) {
      setProjectName("");
      return;
    }
    authFetch<{ project?: { name?: string } }>(`/api/projects/${projectId}/summary`)
      .then((data) => {
        if (!cancelled) {
          setProjectName((data?.project?.name || "").trim());
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProjectName("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  const loadDomainsMap = async () => {
    if (!projectId) {
      return;
    }
    try {
      const domainList = await authFetch<Domain[]>(`/api/projects/${projectId}/domains`);
      const map: Record<string, Domain> = {};
      (Array.isArray(domainList) ? domainList : []).forEach((d) => {
        map[d.id] = d;
      });
      setDomains(map);
    } catch {
      // ignore domain mapping errors
    }
  };

  const load = async (opts?: { silent?: boolean }) => {
    if (!projectId) return;
    if (!opts?.silent) {
      setLoading(true);
    }
    setError(null);
    setPermissionDenied(false);
    try {
      const list = await listQueue(projectId, {
        limit: genPageSize,
        page: genPage,
        search: search.trim() ? search.trim() : undefined
      });
      setItems(Array.isArray(list) ? list : []);
      await loadDomainsMap();
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить очередь";
      if (isPermissionError(msg)) {
        setPermissionDenied(true);
      } else {
        setError(msg);
      }
    } finally {
      if (!opts?.silent) {
        setLoading(false);
      }
    }
  };

  const loadHistory = async (opts?: { silent?: boolean }) => {
    if (!projectId) return;
    if (!opts?.silent) {
      setHistoryLoading(true);
    }
    setError(null);
    setPermissionDenied(false);
    try {
      const list = await listQueueHistory(projectId, {
        limit: historyPageSize,
        page: historyPage,
        search: historySearch.trim() ? historySearch.trim() : undefined,
        status: historyStatusFilter as "all" | "completed" | "failed",
        dateFrom: historyDateFrom || undefined,
        dateTo: historyDateTo || undefined
      });
      setHistoryItems(Array.isArray(list) ? list : []);
      await loadDomainsMap();
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить историю очереди";
      if (isPermissionError(msg)) {
        setPermissionDenied(true);
      } else {
        setError(msg);
      }
    } finally {
      if (!opts?.silent) {
        setHistoryLoading(false);
      }
    }
  };

  const loadLinkTasks = async (opts?: { silent?: boolean }) => {
    if (!projectId) return;
    if (!opts?.silent) {
      setLinkLoading(true);
    }
    setLinkError(null);
    setLinkPermissionDenied(false);
    try {
      const list = await listLinkTasks({
        projectId,
        limit: linkPageSize,
        page: linkPage,
        status: linkStatusFilter !== "all" ? (normalizeLinkTaskStatus(linkStatusFilter) || linkStatusFilter) : undefined,
        search: linkSearch.trim() ? linkSearch.trim() : undefined,
        scheduledFrom: linkDateFrom ? new Date(`${linkDateFrom}T00:00:00`) : undefined,
        scheduledTo: linkDateTo ? new Date(`${linkDateTo}T23:59:59`) : undefined
      });
      setLinkTasks(Array.isArray(list) ? list : []);
    } catch (err: any) {
      const msg = err?.message || "Не удалось загрузить задачи ссылок";
      if (isPermissionError(msg)) {
        setLinkPermissionDenied(true);
      } else {
        setLinkError(msg);
      }
    } finally {
      if (!opts?.silent) {
        setLinkLoading(false);
      }
    }
  };

  useEffect(() => {
    load();
  }, [projectId, genPage, search]);

  useEffect(() => {
    loadHistory();
  }, [projectId, historyPage, historyStatusFilter, historyDateFrom, historyDateTo, historySearch]);

  useEffect(() => {
    loadLinkTasks();
  }, [projectId, linkPage, linkStatusFilter, linkDateFrom, linkDateTo, linkSearch]);

  useEffect(() => {
    const hasActiveLinks = linkTasks.some((task) => isLinkTaskInProgress(task.status));
    if (!hasActiveLinks) {
      return;
    }
    const timer = window.setInterval(() => {
      loadLinkTasks({ silent: true });
    }, 5000);
    return () => window.clearInterval(timer);
  }, [projectId, linkTasks]);

  const filtered = useMemo(() => {
    return items.filter((item) => {
      const normalizedStatus = normalizeProjectQueueActiveStatus(item.status) || item.status;
      if (statusFilter !== "all" && normalizedStatus !== statusFilter) {
        return false;
      }
      if (!matchesDateRange(item.scheduled_for, { from: dateFrom, to: dateTo })) {
        return false;
      }
      const domain = domains[item.domain_id];
      return matchesSearch(item.domain_url || domain?.url || item.domain_id, search);
    });
  }, [items, statusFilter, dateFrom, dateTo, search, domains]);

  const filteredLinks = useMemo(() => {
    return linkTasks.filter((task) => {
      const normalizedStatus = normalizeLinkTaskStatus(task.status) || task.status;
      if (linkStatusFilter !== "all" && normalizedStatus !== linkStatusFilter) {
        return false;
      }
      if (!matchesDateRange(task.scheduled_for, { from: linkDateFrom, to: linkDateTo })) {
        return false;
      }
      return matchesSearch(domains[task.domain_id]?.url || task.domain_id, linkSearch);
    });
  }, [linkTasks, linkStatusFilter, linkDateFrom, linkDateTo, linkSearch, domains]);

  const filteredHistory = useMemo(() => {
    return historyItems.filter((item) => {
      const normalizedStatus = normalizeProjectQueueHistoryStatus(item.status) || item.status;
      if (historyStatusFilter !== "all" && normalizedStatus !== historyStatusFilter) {
        return false;
      }
      if (!matchesDateRange(item.scheduled_for, { from: historyDateFrom, to: historyDateTo })) {
        return false;
      }
      const domain = domains[item.domain_id];
      return matchesSearch(item.domain_url || domain?.url || item.domain_id, historySearch);
    });
  }, [historyItems, historyStatusFilter, historyDateFrom, historyDateTo, historySearch, domains]);

  useEffect(() => {
    setGenPage(1);
  }, [statusFilter, dateFrom, dateTo, search]);

  useEffect(() => {
    setHistoryPage(1);
  }, [historyStatusFilter, historyDateFrom, historyDateTo, historySearch]);

  useEffect(() => {
    setLinkPage(1);
  }, [linkStatusFilter, linkDateFrom, linkDateTo, linkSearch]);

  const genHasNext = hasNextPageByPageSize(items.length, genPageSize);
  const historyHasNext = hasNextPageByPageSize(historyItems.length, historyPageSize);
  const linkHasNext = hasNextPageByPageSize(linkTasks.length, linkPageSize);
  const visibleItems = filtered;
  const visibleHistoryItems = filteredHistory;
  const visibleLinks = filteredLinks;
  const cleanupGuard = canDelete({
    busy: cleaning || isLocked(cleanupLockKey),
    busyReason: lockReason(cleanupLockKey),
    allowed: true
  });
  const refreshGuard = canRun({
    busy: refreshing || isLocked(refreshLockKey),
    busyReason: lockReason(refreshLockKey)
  });
  const linkRefreshGuard = canRun({
    busy: linkRefreshing || isLocked(linkRefreshLockKey),
    busyReason: lockReason(linkRefreshLockKey)
  });

  const handleRemove = async (item: QueueItemDTO) => {
    const domainLabel = item.domain_url || domains[item.domain_id]?.url || "домен";
    if (!confirm(`Удалить из очереди домен ${domainLabel}?`)) return;
    const lockKey = removeQueueItemLockKey(item.id);
    await runLocked(
      lockKey,
      async () => {
        setLoading(true);
        setError(null);
        try {
          await deleteQueueItem(item.id);
          showToast({
            type: "success",
            title: "Удалено из очереди",
            message: domainLabel
          });
          await load();
        } catch (err: any) {
          const msg = err?.message || "Не удалось удалить из очереди";
          setError(msg);
          showToast({ type: "error", title: "Ошибка удаления", message: msg });
        } finally {
          setLoading(false);
        }
      },
      "Удаление уже выполняется."
    );
  };

  const handleLinkRetry = async (task: LinkTaskDTO) => {
    const domainLabel = domains[task.domain_id]?.url || "домен";
    if (!confirm(`Повторить задачу ссылки для домена ${domainLabel}?`)) return;
    const lockKey = linkRetryLockKey(task.id);
    await runLocked(
      lockKey,
      async () => {
        setLinkLoading(true);
        setLinkError(null);
        try {
          await retryLinkTask(task.id);
          showToast({
            type: "success",
            title: "Повтор поставлен в очередь",
            message: domainLabel
          });
          await loadLinkTasks();
        } catch (err: any) {
          const msg = err?.message || "Не удалось повторить задачу ссылки";
          setLinkError(msg);
          showToast({ type: "error", title: "Ошибка повтора", message: msg });
        } finally {
          setLinkLoading(false);
        }
      },
      "Повтор уже выполняется."
    );
  };

  const handleLinkDelete = async (task: LinkTaskDTO) => {
    const domainLabel = domains[task.domain_id]?.url || "домен";
    if (!confirm(`Удалить задачу ссылки для домена ${domainLabel}?`)) return;
    const lockKey = linkDeleteLockKey(task.id);
    await runLocked(
      lockKey,
      async () => {
        setLinkLoading(true);
        setLinkError(null);
        try {
          await deleteLinkTask(task.id);
          showToast({
            type: "success",
            title: "Задача ссылки удалена",
            message: domainLabel
          });
          await loadLinkTasks();
        } catch (err: any) {
          const msg = err?.message || "Не удалось удалить задачу ссылки";
          setLinkError(msg);
          showToast({ type: "error", title: "Ошибка удаления", message: msg });
        } finally {
          setLinkLoading(false);
        }
      },
      "Удаление уже выполняется."
    );
  };

  const handleCleanup = async () => {
    if (!projectId) return;
    if (!confirm("Очистить устаревшие элементы очереди?")) return;
    await runLocked(
      cleanupLockKey,
      async () => {
        setCleaning(true);
        setError(null);
        try {
          const res = await cleanupQueue(projectId);
          showToast({
            type: "success",
            title: "Очистка очереди",
            message: `Удалено: ${res?.removed ?? 0}`
          });
          await load();
        } catch (err: any) {
          const msg = err?.message || "Не удалось очистить очередь";
          setError(msg);
          showToast({ type: "error", title: "Ошибка очистки", message: msg });
        } finally {
          setCleaning(false);
        }
      },
      "Очистка уже выполняется."
    );
  };

  const handleRefresh = async () => {
    if (!projectId) return;
    await runLocked(
      refreshLockKey,
      async () => {
        setRefreshing(true);
        try {
          await Promise.all([load({ silent: true }), loadHistory({ silent: true })]);
        } finally {
          setRefreshing(false);
        }
      },
      "Обновление уже выполняется."
    );
  };

  const handleLinkRefresh = async () => {
    if (!projectId) return;
    await runLocked(
      linkRefreshLockKey,
      async () => {
        setLinkRefreshing(true);
        try {
          await loadLinkTasks({ silent: true });
        } finally {
          setLinkRefreshing(false);
        }
      },
      "Обновление уже выполняется."
    );
  };

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
        {permissionDenied && (
          <div className="text-sm text-amber-600 dark:text-amber-400 mt-2">
            Недостаточно прав для просмотра очереди.
          </div>
        )}
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
            <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {filteredLinks.length}</span>
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
            empty={!linkLoading && !linkError && filteredLinks.length === 0}
            emptyText="Очередь ссылок пуста."
          />
        )}
        {!linkLoading && !linkError && !linkPermissionDenied && filteredLinks.length > 0 && (
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
        {filteredLinks.length > 0 && (
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
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {filtered.length}</span>
        </div>
        {!permissionDenied && (
          <TableState
            loading={loading}
            empty={!loading && filtered.length === 0}
            emptyText="Очередь пуста."
          />
        )}
        {!loading && !permissionDenied && filtered.length > 0 && (
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
        {filtered.length > 0 && (
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
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {filteredHistory.length}</span>
        </div>
        {!permissionDenied && (
          <TableState
            loading={historyLoading}
            empty={!historyLoading && filteredHistory.length === 0}
            emptyText="История запусков пуста."
          />
        )}
        {!historyLoading && !permissionDenied && filteredHistory.length > 0 && (
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
        {filteredHistory.length > 0 && (
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
