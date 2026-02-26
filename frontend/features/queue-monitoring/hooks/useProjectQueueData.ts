"use client";

import { useEffect, useMemo, useState } from "react";

import { authFetch } from "../../../lib/http";
import { deleteLinkTask, listLinkTasks, retryLinkTask } from "../../../lib/linkTasksApi";
import { cleanupQueue, deleteQueueItem, listQueue, listQueueHistory } from "../../../lib/queueApi";
import { showToast } from "../../../lib/toastStore";
import type { LinkTaskDTO } from "../../../types/linkTasks";
import type { QueueItemDTO } from "../../../types/queue";
import { useActionLocks } from "../../editor-v3/hooks/useActionLocks";
import { useFlowState } from "./useFlowState";
import { canDelete, canRun } from "../services/actionGuards";
import { matchesDateRange, matchesSearch } from "../services/filters";
import { queueMonitoringRu, toDiagnosticsText } from "../services/i18n-ru";
import { hasNextPageByPageSize } from "../services/primitives";
import {
  normalizeProjectQueueActiveStatus,
  normalizeProjectQueueHistoryStatus
} from "../services/statusMeta";
import { isLinkTaskInProgress, normalizeLinkTaskStatus } from "../../../lib/linkTaskStatus";

type Domain = {
  id: string;
  url: string;
};

const isPermissionError = (message: string) =>
  /permission|access denied|admin only|forbidden/i.test(message);

export function useProjectQueueData(projectId: string | undefined) {
  const [items, setItems] = useState<QueueItemDTO[]>([]);
  const [historyItems, setHistoryItems] = useState<QueueItemDTO[]>([]);
  const [domains, setDomains] = useState<Record<string, Domain>>({});
  const [projectName, setProjectName] = useState("");
  const [loading, setLoading] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errorDiagnostics, setErrorDiagnostics] = useState<string | null>(null);
  const [permissionDenied, setPermissionDenied] = useState(false);

  const [linkTasks, setLinkTasks] = useState<LinkTaskDTO[]>([]);
  const [linkLoading, setLinkLoading] = useState(false);
  const [linkRefreshing, setLinkRefreshing] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [linkErrorDiagnostics, setLinkErrorDiagnostics] = useState<string | null>(null);
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

  const { isLocked, lockReason, runLocked } = useActionLocks();
  const queueFlow = useFlowState();
  const linkFlow = useFlowState();

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
    setErrorDiagnostics(null);
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
        setError("Не удалось загрузить очередь");
        setErrorDiagnostics(toDiagnosticsText(err) || null);
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
    setErrorDiagnostics(null);
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
        setError("Не удалось загрузить историю очереди");
        setErrorDiagnostics(toDiagnosticsText(err) || null);
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
    setLinkErrorDiagnostics(null);
    setLinkPermissionDenied(false);
    try {
      const list = await listLinkTasks({
        projectId,
        limit: linkPageSize,
        page: linkPage,
        status:
          linkStatusFilter !== "all" ? normalizeLinkTaskStatus(linkStatusFilter) || linkStatusFilter : undefined,
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
        setLinkError("Не удалось загрузить задачи ссылок");
        setLinkErrorDiagnostics(toDiagnosticsText(err) || null);
      }
    } finally {
      if (!opts?.silent) {
        setLinkLoading(false);
      }
    }
  };

  useEffect(() => {
    void load();
  }, [projectId, genPage, search]);

  useEffect(() => {
    void loadHistory();
  }, [projectId, historyPage, historyStatusFilter, historyDateFrom, historyDateTo, historySearch]);

  useEffect(() => {
    void loadLinkTasks();
  }, [projectId, linkPage, linkStatusFilter, linkDateFrom, linkDateTo, linkSearch]);

  useEffect(() => {
    const hasActiveLinks = linkTasks.some((task) => isLinkTaskInProgress(task.status));
    if (!hasActiveLinks) {
      return;
    }
    const timer = window.setInterval(() => {
      void loadLinkTasks({ silent: true });
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
    queueFlow.validating("Проверяем возможность удаления из очереди");
    await runLocked(
      lockKey,
      async () => {
        queueFlow.sending(`Удаляем домен ${domainLabel} из очереди`);
        setLoading(true);
        setError(null);
        setErrorDiagnostics(null);
        try {
          await deleteQueueItem(item.id);
          showToast({
            type: "success",
            title: "Удалено из очереди",
            message: domainLabel
          });
          await load();
          queueFlow.done("Элемент очереди удалён");
        } catch (err: any) {
          const userMessage = "Не удалось удалить из очереди";
          setError(userMessage);
          setErrorDiagnostics(toDiagnosticsText(err) || null);
          queueFlow.fail(userMessage, err);
          showToast({ type: "error", title: "Ошибка удаления", message: userMessage });
        } finally {
          setLoading(false);
        }
      },
      queueMonitoringRu.lockReasons.deleteInFlight
    );
  };

  const handleLinkRetry = async (task: LinkTaskDTO) => {
    const domainLabel = domains[task.domain_id]?.url || "домен";
    if (!confirm(`Повторить задачу ссылки для домена ${domainLabel}?`)) return;
    const lockKey = linkRetryLockKey(task.id);
    linkFlow.validating("Проверяем возможность повтора задачи ссылки");
    await runLocked(
      lockKey,
      async () => {
        linkFlow.sending(`Повторяем задачу ссылки для ${domainLabel}`);
        setLinkLoading(true);
        setLinkError(null);
        setLinkErrorDiagnostics(null);
        try {
          await retryLinkTask(task.id);
          showToast({
            type: "success",
            title: "Повтор поставлен в очередь",
            message: domainLabel
          });
          await loadLinkTasks();
          linkFlow.done("Задача повторно поставлена в очередь");
        } catch (err: any) {
          const userMessage = "Не удалось повторить задачу ссылки";
          setLinkError(userMessage);
          setLinkErrorDiagnostics(toDiagnosticsText(err) || null);
          linkFlow.fail(userMessage, err);
          showToast({ type: "error", title: "Ошибка повтора", message: userMessage });
        } finally {
          setLinkLoading(false);
        }
      },
      queueMonitoringRu.lockReasons.retryInFlight
    );
  };

  const handleLinkDelete = async (task: LinkTaskDTO) => {
    const domainLabel = domains[task.domain_id]?.url || "домен";
    if (!confirm(`Удалить задачу ссылки для домена ${domainLabel}?`)) return;
    const lockKey = linkDeleteLockKey(task.id);
    linkFlow.validating("Проверяем возможность удаления задачи ссылки");
    await runLocked(
      lockKey,
      async () => {
        linkFlow.sending(`Удаляем задачу ссылки для ${domainLabel}`);
        setLinkLoading(true);
        setLinkError(null);
        setLinkErrorDiagnostics(null);
        try {
          await deleteLinkTask(task.id);
          showToast({
            type: "success",
            title: "Задача ссылки удалена",
            message: domainLabel
          });
          await loadLinkTasks();
          linkFlow.done("Задача ссылки удалена");
        } catch (err: any) {
          const userMessage = "Не удалось удалить задачу ссылки";
          setLinkError(userMessage);
          setLinkErrorDiagnostics(toDiagnosticsText(err) || null);
          linkFlow.fail(userMessage, err);
          showToast({ type: "error", title: "Ошибка удаления", message: userMessage });
        } finally {
          setLinkLoading(false);
        }
      },
      queueMonitoringRu.lockReasons.deleteInFlight
    );
  };

  const handleCleanup = async () => {
    if (!projectId) return;
    if (!confirm("Очистить устаревшие элементы очереди?")) return;
    queueFlow.validating("Проверяем возможность очистки очереди");
    await runLocked(
      cleanupLockKey,
      async () => {
        queueFlow.sending("Очищаем устаревшие элементы очереди");
        setCleaning(true);
        setError(null);
        setErrorDiagnostics(null);
        try {
          const res = await cleanupQueue(projectId);
          showToast({
            type: "success",
            title: "Очистка очереди",
            message: `Удалено: ${res?.removed ?? 0}`
          });
          await load();
          queueFlow.done("Очередь успешно очищена");
        } catch (err: any) {
          const userMessage = "Не удалось очистить очередь";
          setError(userMessage);
          setErrorDiagnostics(toDiagnosticsText(err) || null);
          queueFlow.fail(userMessage, err);
          showToast({ type: "error", title: "Ошибка очистки", message: userMessage });
        } finally {
          setCleaning(false);
        }
      },
      queueMonitoringRu.lockReasons.cleanupInFlight
    );
  };

  const handleRefresh = async () => {
    if (!projectId) return;
    queueFlow.validating("Проверяем доступность обновления очереди");
    await runLocked(
      refreshLockKey,
      async () => {
        queueFlow.sending("Обновляем активную очередь и историю запусков");
        setRefreshing(true);
        try {
          await Promise.all([load({ silent: true }), loadHistory({ silent: true })]);
          queueFlow.done("Данные очереди обновлены");
        } finally {
          setRefreshing(false);
        }
      },
      queueMonitoringRu.lockReasons.refreshInFlight
    );
  };

  const handleLinkRefresh = async () => {
    if (!projectId) return;
    linkFlow.validating("Проверяем доступность обновления задач ссылок");
    await runLocked(
      linkRefreshLockKey,
      async () => {
        linkFlow.sending("Обновляем задачи ссылок");
        setLinkRefreshing(true);
        try {
          await loadLinkTasks({ silent: true });
          linkFlow.done("Список задач ссылок обновлён");
        } finally {
          setLinkRefreshing(false);
        }
      },
      queueMonitoringRu.lockReasons.refreshInFlight
    );
  };

  return {
    projectName,
    domains,
    items,
    historyItems,
    linkTasks,
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
    loadHistory,
  };
}

