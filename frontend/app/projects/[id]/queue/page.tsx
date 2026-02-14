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
import type { QueueItemDTO } from "../../../../types/queue";
import type { LinkTaskDTO } from "../../../../types/linkTasks";
import { Badge } from "../../../../components/Badge";
import { canDeleteLinkTask, canRetryLinkTask, getLinkTaskStatusMeta, isLinkTaskInProgress, normalizeLinkTaskStatus } from "../../../../lib/linkTaskStatus";

const statusOptions = ["all", "pending", "queued"];
const STATUS_LABELS: Record<string, string> = {
  all: "Все",
  pending: "Ожидает",
  queued: "В очереди"
};

const historyStatusOptions = ["all", "completed", "failed"];
const HISTORY_STATUS_LABELS: Record<string, string> = {
  all: "Все",
  completed: "Обработано",
  failed: "Ошибка"
};

const linkStatusOptions = [
  "all",
  "pending",
  "searching",
  "removing",
  "inserted",
  "generated",
  "removed",
  "failed"
];

const LINK_STATUS_LABELS: Record<string, string> = {
  all: "Все",
  pending: "Ожидает",
  searching: "Поиск",
  removing: "Удаление",
  inserted: "Вставлено",
  generated: "Вставлено (ген. текст)",
  removed: "Удалено",
  failed: "Ошибка"
};

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
  const tabParam = (searchParams.get("tab") || "domains").toLowerCase();
  const activeTab = tabParam === "links" ? "links" : "domains";

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
    const fromDate = dateFrom ? new Date(`${dateFrom}T00:00:00`) : null;
    const toDate = dateTo ? new Date(`${dateTo}T23:59:59`) : null;
    const term = search.trim().toLowerCase();
    return items.filter((item) => {
      if (statusFilter !== "all" && item.status !== statusFilter) {
        return false;
      }
      const scheduled = new Date(item.scheduled_for);
      if (fromDate && scheduled < fromDate) {
        return false;
      }
      if (toDate && scheduled > toDate) {
        return false;
      }
      if (!term) {
        return true;
      }
      const domain = domains[item.domain_id];
      const label = (item.domain_url || domain?.url || item.domain_id || "").toLowerCase();
      return label.includes(term);
    });
  }, [items, statusFilter, dateFrom, dateTo, search, domains]);

  const filteredLinks = useMemo(() => {
    const fromDate = linkDateFrom ? new Date(`${linkDateFrom}T00:00:00`) : null;
    const toDate = linkDateTo ? new Date(`${linkDateTo}T23:59:59`) : null;
    const term = linkSearch.trim().toLowerCase();
    return linkTasks.filter((task) => {
      const normalizedStatus = normalizeLinkTaskStatus(task.status) || task.status;
      if (linkStatusFilter !== "all" && normalizedStatus !== linkStatusFilter) {
        return false;
      }
      const scheduled = new Date(task.scheduled_for);
      if (fromDate && scheduled < fromDate) {
        return false;
      }
      if (toDate && scheduled > toDate) {
        return false;
      }
      if (!term) {
        return true;
      }
      const label = (domains[task.domain_id]?.url || task.domain_id || "").toLowerCase();
      return label.includes(term);
    });
  }, [linkTasks, linkStatusFilter, linkDateFrom, linkDateTo, linkSearch, domains]);

  const filteredHistory = useMemo(() => {
    const fromDate = historyDateFrom ? new Date(`${historyDateFrom}T00:00:00`) : null;
    const toDate = historyDateTo ? new Date(`${historyDateTo}T23:59:59`) : null;
    const term = historySearch.trim().toLowerCase();
    return historyItems.filter((item) => {
      if (historyStatusFilter !== "all" && item.status !== historyStatusFilter) {
        return false;
      }
      const scheduled = new Date(item.scheduled_for);
      if (fromDate && scheduled < fromDate) {
        return false;
      }
      if (toDate && scheduled > toDate) {
        return false;
      }
      if (!term) {
        return true;
      }
      const domain = domains[item.domain_id];
      const label = (item.domain_url || domain?.url || item.domain_id || "").toLowerCase();
      return label.includes(term);
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

  const genHasNext = items.length === genPageSize;
  const historyHasNext = historyItems.length === historyPageSize;
  const linkHasNext = linkTasks.length === linkPageSize;
  const visibleItems = filtered;
  const visibleHistoryItems = filteredHistory;
  const visibleLinks = filteredLinks;

  const handleRemove = async (item: QueueItemDTO) => {
    const domainLabel = item.domain_url || domains[item.domain_id]?.url || "домен";
    if (!confirm(`Удалить из очереди домен ${domainLabel}?`)) return;
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
  };

  const handleLinkRetry = async (task: LinkTaskDTO) => {
    const domainLabel = domains[task.domain_id]?.url || "домен";
    if (!confirm(`Повторить задачу ссылки для домена ${domainLabel}?`)) return;
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
  };

  const handleLinkDelete = async (task: LinkTaskDTO) => {
    const domainLabel = domains[task.domain_id]?.url || "домен";
    if (!confirm(`Удалить задачу ссылки для домена ${domainLabel}?`)) return;
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
  };

  const handleCleanup = async () => {
    if (!projectId) return;
    if (!confirm("Очистить устаревшие элементы очереди?")) return;
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
  };

  const handleRefresh = async () => {
    if (!projectId) return;
    setRefreshing(true);
    try {
      await Promise.all([load({ silent: true }), loadHistory({ silent: true })]);
    } finally {
      setRefreshing(false);
    }
  };

  const handleLinkRefresh = async () => {
    if (!projectId) return;
    setLinkRefreshing(true);
    try {
      await loadLinkTasks({ silent: true });
    } finally {
      setLinkRefreshing(false);
    }
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
              disabled={cleaning}
              className="inline-flex items-center gap-2 rounded-lg border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-900/40 dark:bg-slate-800 dark:text-amber-200"
            >
              <FiRefreshCw className={cleaning ? "animate-spin" : ""} />
              {cleaning ? "Очищаю..." : "Очистить очередь"}
            </button>
            <button
              onClick={handleRefresh}
              disabled={refreshing}
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
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по статусу</span>
            <select
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
            >
              {statusOptions.map((opt) => (
                <option key={opt} value={opt}>
                  {STATUS_LABELS[opt] || opt}
                </option>
              ))}
            </select>
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по дате (от)</span>
            <input
              type="date"
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={dateFrom}
              onChange={(e) => setDateFrom(e.target.value)}
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по дате (до)</span>
            <input
              type="date"
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={dateTo}
              onChange={(e) => setDateTo(e.target.value)}
            />
          </label>
        </div>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Поиск по домену"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        />
      </div>
      )}

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="text-sm font-semibold">Фильтры истории запусков</div>
        <div className="grid gap-3 md:grid-cols-3">
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по статусу</span>
            <select
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={historyStatusFilter}
              onChange={(e) => setHistoryStatusFilter(e.target.value)}
            >
              {historyStatusOptions.map((opt) => (
                <option key={opt} value={opt}>
                  {HISTORY_STATUS_LABELS[opt] || opt}
                </option>
              ))}
            </select>
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по дате (от)</span>
            <input
              type="date"
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={historyDateFrom}
              onChange={(e) => setHistoryDateFrom(e.target.value)}
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по дате (до)</span>
            <input
              type="date"
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={historyDateTo}
              onChange={(e) => setHistoryDateTo(e.target.value)}
            />
          </label>
        </div>
        <input
          type="search"
          value={historySearch}
          onChange={(e) => setHistorySearch(e.target.value)}
          placeholder="Поиск по домену в истории"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
        />
      </div>
      )}

      {activeTab === "links" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="text-sm font-semibold">Фильтры ссылок</div>
        <div className="grid gap-3 md:grid-cols-3">
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по статусу</span>
            <select
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkStatusFilter}
              onChange={(e) => setLinkStatusFilter(e.target.value)}
            >
              {linkStatusOptions.map((opt) => (
                <option key={opt} value={opt}>
                  {LINK_STATUS_LABELS[opt] || opt}
                </option>
              ))}
            </select>
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по дате (от)</span>
            <input
              type="date"
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkDateFrom}
              onChange={(e) => setLinkDateFrom(e.target.value)}
            />
          </label>
          <label className="text-sm space-y-1">
            <span className="text-slate-600 dark:text-slate-300">Фильтр по дате (до)</span>
            <input
              type="date"
              className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
              value={linkDateTo}
              onChange={(e) => setLinkDateTo(e.target.value)}
            />
          </label>
        </div>
        <input
          type="search"
          value={linkSearch}
          onChange={(e) => setLinkSearch(e.target.value)}
          placeholder="Поиск по домену"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
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
              disabled={linkRefreshing}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw className={linkRefreshing ? "animate-spin" : ""} /> Обновить
            </button>
          </div>
        </div>
        {linkError && <div className="text-sm text-red-500">{linkError}</div>}
        {linkPermissionDenied && (
          <div className="text-sm text-amber-600 dark:text-amber-400">Недостаточно прав для просмотра задач ссылок.</div>
        )}
        {linkLoading && <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
        {!linkLoading && !linkPermissionDenied && filteredLinks.length === 0 && (
          <div className="text-sm text-slate-500 dark:text-slate-400">Очередь ссылок пуста.</div>
        )}
        {!linkLoading && !linkPermissionDenied && filteredLinks.length > 0 && (
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
                {visibleLinks.map((task) => {
                  const domain = domains[task.domain_id];
                  const domainLabel = domain?.url || "Домен";
                  const domainHref = `/domains/${domain?.id || task.domain_id}`;
                  const actionLabel = (task.action || "insert") === "remove" ? "Удаление" : "Вставка";
                  const normalizedStatus = normalizeLinkTaskStatus(task.status) || task.status;
                  const canRetry = canRetryLinkTask(task.status);
                  const canDelete = canDeleteLinkTask(task.status);
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
                          {canRetry && (
                            <button
                              onClick={() => handleLinkRetry(task)}
                              disabled={linkLoading}
                              className="inline-flex items-center gap-1 rounded-lg border border-amber-200 bg-white px-2 py-1 text-xs font-semibold text-amber-700 hover:bg-amber-50 dark:border-amber-800 dark:bg-slate-800 dark:text-amber-200"
                            >
                              <FiRotateCw /> Повтор
                            </button>
                          )}
                          <button
                            onClick={() => handleLinkDelete(task)}
                            disabled={linkLoading || !canDelete}
                            title={!canDelete ? "Удаление недоступно для активных задач (ожидает/поиск/удаление)." : undefined}
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
          <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
            <span>Страница {linkPage}</span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setLinkPage((p) => Math.max(1, p - 1))}
                disabled={linkPage <= 1}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Назад
              </button>
              <button
                onClick={() => setLinkPage((p) => p + 1)}
                disabled={!linkHasNext}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Вперёд
              </button>
            </div>
          </div>
        )}
      </div>
      )}

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Очередь генераций</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {filtered.length}</span>
        </div>
        {loading && <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
        {!loading && !permissionDenied && filtered.length === 0 && (
          <div className="text-sm text-slate-500 dark:text-slate-400">Очередь пуста.</div>
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
                {visibleItems.map((item) => {
                  const domain = domains[item.domain_id];
                  const domainLabel = item.domain_url || domain?.url || "Домен";
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
                      <td className="py-3 pr-4">{STATUS_LABELS[item.status] || item.status}</td>
                      <td className="py-3 pr-4">{item.priority}</td>
                      <td className="py-3 pr-4 text-right">
                        <button
                          onClick={() => handleRemove(item)}
                          disabled={loading}
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
          <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
            <span>Страница {genPage}</span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setGenPage((p) => Math.max(1, p - 1))}
                disabled={genPage <= 1}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Назад
              </button>
              <button
                onClick={() => setGenPage((p) => p + 1)}
                disabled={!genHasNext}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Вперёд
              </button>
            </div>
          </div>
        )}
      </div>
      )}

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">История запусков</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {filteredHistory.length}</span>
        </div>
        {historyLoading && <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
        {!historyLoading && !permissionDenied && filteredHistory.length === 0 && (
          <div className="text-sm text-slate-500 dark:text-slate-400">История запусков пуста.</div>
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
                {visibleHistoryItems.map((item) => {
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
                      <td className="py-3 pr-4">{HISTORY_STATUS_LABELS[item.status] || item.status}</td>
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
          <div className="flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
            <span>Страница {historyPage}</span>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setHistoryPage((p) => Math.max(1, p - 1))}
                disabled={historyPage <= 1}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Назад
              </button>
              <button
                onClick={() => setHistoryPage((p) => p + 1)}
                disabled={!historyHasNext}
                className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              >
                Вперёд
              </button>
            </div>
          </div>
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
