"use client";

import { Suspense, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { authFetch } from "../../lib/http";
import { useAuthGuard } from "../../lib/useAuth";
import { FiClock, FiPlay, FiCheck, FiAlertTriangle, FiRefreshCw, FiPause, FiRotateCw, FiTrash2, FiX } from "react-icons/fi";
import { deleteLinkTask, listLinkTasks, retryLinkTask } from "../../lib/linkTasksApi";
import { showToast } from "../../lib/toastStore";
import type { LinkTaskDTO } from "../../types/linkTasks";
import { Badge } from "../../components/Badge";
import { getLinkTaskStatusMeta, isLinkTaskInProgress, normalizeLinkTaskStatus } from "../../lib/linkTaskStatus";
import { PaginationControls } from "../../features/queue-monitoring/components/PaginationControls";
import { canDelete, canRetry, canRun } from "../../features/queue-monitoring/services/actionGuards";
import {
  QUEUE_LINK_STATUS_LABELS,
  hasNextPageByPageSize,
  resolveQueueTab
} from "../../features/queue-monitoring/services/primitives";

type Generation = {
  id: string;
  domain_id: string;
  domain_url?: string;
  status: string;
  progress: number;
  created_at?: string;
  updated_at?: string;
};

export default function QueuePage() {
  return (
    <Suspense fallback={<div className="p-4 text-sm text-slate-500 dark:text-slate-400">Загрузка очереди...</div>}>
      <QueuePageContent />
    </Suspense>
  );
}

type ProjectDTO = { id: string; name?: string | null };

function QueuePageContent() {
  const { me } = useAuthGuard();
  const [items, setItems] = useState<Generation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("all");
  const [search, setSearch] = useState("");
  const [genPage, setGenPage] = useState(1);
  const genPageSize = 20;

  const [linkTasks, setLinkTasks] = useState<LinkTaskDTO[]>([]);
  const [linkLoading, setLinkLoading] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [linkFilter, setLinkFilter] = useState("all");
  const [linkSearch, setLinkSearch] = useState("");
  const [linkPage, setLinkPage] = useState(1);
  const linkPageSize = 20;
  const [linkDomains, setLinkDomains] = useState<Record<string, string>>({});
  const searchParams = useSearchParams();
  const activeTab = resolveQueueTab(searchParams.get("tab"));

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams();
      params.set("limit", String(genPageSize));
      params.set("page", String(genPage));
      params.set("lite", "1");
      if (search.trim()) {
        params.set("search", search.trim());
      }
      const res = await authFetch<Generation[]>(`/api/generations?${params.toString()}`);
      setItems(Array.isArray(res) ? res : []);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить очередь");
    } finally {
      setLoading(false);
    }
  }, [genPage, genPageSize, search]);

  const loadLinks = useCallback(async () => {
    setLinkLoading(true);
    setLinkError(null);
    try {
      const params = {
        limit: linkPageSize,
        page: linkPage,
        status: linkFilter !== "all" ? linkFilter : undefined,
        search: linkSearch.trim() ? linkSearch.trim() : undefined
      };
      let list: LinkTaskDTO[] = [];
      try {
        const res = await listLinkTasks(params);
        list = Array.isArray(res) ? res : [];
      } catch (err: any) {
        const msg = String(err?.message || "");
        const isAdminOnly = msg.toLowerCase().includes("admin only");
        const isAdmin = (me?.role || "").toLowerCase() === "admin";
        if (!isAdmin && isAdminOnly) {
          const projects = await authFetch<ProjectDTO[]>("/api/projects");
          const ids = (Array.isArray(projects) ? projects : [])
            .map((project) => project.id)
            .filter((id) => id);
          const perProjectLimit = Math.max(200, linkPageSize * linkPage);
          const allTasks: LinkTaskDTO[] = [];
          for (const projectId of ids) {
            const res = await listLinkTasks({
              ...params,
              projectId,
              limit: perProjectLimit,
              page: 1
            });
            if (Array.isArray(res)) {
              allTasks.push(...res);
            }
          }
          allTasks.sort((a, b) => {
            const aTime = new Date(a.scheduled_for).getTime();
            const bTime = new Date(b.scheduled_for).getTime();
            return aTime - bTime;
          });
          const offset = (linkPage - 1) * linkPageSize;
          const slice = allTasks.slice(offset, offset + linkPageSize + 1);
          list = slice.slice(0, linkPageSize);
        } else {
          throw err;
        }
      }
      setLinkTasks(list);
      const ids = Array.from(new Set(list.map((task) => task.domain_id).filter(Boolean))).slice(0, 200);
      if (ids.length === 0) {
        setLinkDomains({});
      } else {
        try {
          const params = new URLSearchParams();
          params.set("ids", ids.join(","));
          const domainList = await authFetch<{ id: string; url: string }[]>(`/api/domains?${params.toString()}`);
          const map: Record<string, string> = {};
          (Array.isArray(domainList) ? domainList : []).forEach((d) => {
            if (d?.id && d?.url) {
              map[d.id] = d.url;
            }
          });
          setLinkDomains(map);
        } catch {
          setLinkDomains({});
        }
      }
    } catch (err: any) {
      setLinkError(err?.message || "Не удалось загрузить задачи ссылок");
    } finally {
      setLinkLoading(false);
    }
  }, [linkFilter, linkPage, linkPageSize, linkSearch, me]);

  const handleRefresh = useCallback(async () => {
    await Promise.all([load(), loadLinks()]);
  }, [load, loadLinks]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    loadLinks();
  }, [loadLinks]);

  useEffect(() => {
    const hasActiveGenerations = items.some((i) =>
      ["pending", "processing", "pause_requested", "cancelling"].includes(i.status)
    );
    const hasActiveLinks = linkTasks.some((t) => isLinkTaskInProgress(t.status));
    if (!hasActiveGenerations && !hasActiveLinks) {
      return;
    }
    const timer = window.setInterval(() => {
      if (hasActiveGenerations) {
        load();
      }
      if (hasActiveLinks) {
        loadLinks();
      }
    }, 5000);
    return () => window.clearInterval(timer);
  }, [items, linkTasks, load, loadLinks]);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    return items.filter((i) => {
      if (filter !== "all" && i.status !== filter) {
        return false;
      }
      if (!term) {
        return true;
      }
      const label = (i.domain_url || "").toLowerCase();
      return label.includes(term);
    });
  }, [filter, items, search]);

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const i of items) {
      c[i.status] = (c[i.status] || 0) + 1;
    }
    return c;
  }, [items]);

  const filteredLinks = useMemo(() => {
    const term = linkSearch.trim().toLowerCase();
    return linkTasks.filter((task) => {
      const normalizedStatus = normalizeLinkTaskStatus(task.status) || task.status;
      if (linkFilter !== "all" && normalizedStatus !== linkFilter) {
        return false;
      }
      if (!term) {
        return true;
      }
      const label = (linkDomains[task.domain_id] || "").toLowerCase();
      return label.includes(term);
    });
  }, [linkFilter, linkTasks, linkSearch, linkDomains]);

  const linkCounts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const t of linkTasks) {
      const normalizedStatus = normalizeLinkTaskStatus(t.status) || t.status;
      c[normalizedStatus] = (c[normalizedStatus] || 0) + 1;
    }
    return c;
  }, [linkTasks]);

  useEffect(() => {
    setGenPage(1);
  }, [filter, search]);

  useEffect(() => {
    setLinkPage(1);
  }, [linkFilter, linkSearch]);

  const genHasNext = hasNextPageByPageSize(items.length, genPageSize);
  const linkHasNext = hasNextPageByPageSize(linkTasks.length, linkPageSize);
  const visibleGenerations = filtered;
  const visibleLinks = filteredLinks;
  const genIndexBase = (genPage - 1) * genPageSize;
  const linkIndexBase = (linkPage - 1) * linkPageSize;
  const refreshGuard = canRun({ busy: loading || linkLoading });

  const handleLinkRetry = async (task: LinkTaskDTO) => {
    const domainLabel = linkDomains[task.domain_id] || "домен";
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
      await loadLinks();
    } catch (err: any) {
      const msg = err?.message || "Не удалось повторить задачу ссылки";
      setLinkError(msg);
      showToast({ type: "error", title: "Ошибка повтора", message: msg });
    } finally {
      setLinkLoading(false);
    }
  };

  const handleLinkDelete = async (task: LinkTaskDTO) => {
    const domainLabel = linkDomains[task.domain_id] || "домен";
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
      await loadLinks();
    } catch (err: any) {
      const msg = err?.message || "Не удалось удалить задачу ссылки";
      setLinkError(msg);
      showToast({ type: "error", title: "Ошибка удаления", message: msg });
    } finally {
      setLinkLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
            <Link href="/projects" className="hover:text-slate-700 dark:hover:text-slate-200">
              Проекты
            </Link>
            <span>/</span>
            <span>Глобальная очередь</span>
            <Badge label="Глобальная" tone="indigo" />
          </div>
          <h1 className="text-2xl font-bold">Глобальная очередь</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">
            Генерации и задачи ссылок по всем проектам. Поиск работает по всем страницам.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            onClick={handleRefresh}
            disabled={refreshGuard.disabled}
            title={refreshGuard.reason}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRefreshCw /> Обновить всё
          </button>
          <Link
            href="/projects"
            className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500"
          >
            ← К проектам
          </Link>
        </div>
      </div>
      {error && <div className="text-red-500 text-sm">{error}</div>}

      <div className="flex flex-wrap gap-2">
        <TabLink
          href={({ pathname: "/queue", query: { tab: "domains" } } as LinkHref)}
          label={`Домены (${items.length})`}
          active={activeTab === "domains"}
        />
        <TabLink
          href={({ pathname: "/queue", query: { tab: "links" } } as LinkHref)}
          label={`Ссылки (${linkTasks.length})`}
          active={activeTab === "links"}
        />
      </div>

      {activeTab === "domains" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 className="font-semibold">Генерации</h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Фильтры — для текущей страницы, поиск — по всем страницам.
            </p>
          </div>
          <span className="text-xs text-slate-500 dark:text-slate-400">Показано: {filtered.length}</span>
        </div>
        <div className="flex flex-wrap gap-2">
          <FilterButton label="Все" value="all" active={filter === "all"} onClick={() => setFilter("all")} count={items.length} />
          <FilterButton label="В очереди" value="pending" active={filter === "pending"} onClick={() => setFilter("pending")} count={counts["pending"] || 0} />
          <FilterButton label="В работе" value="processing" active={filter === "processing"} onClick={() => setFilter("processing")} count={counts["processing"] || 0} />
          <FilterButton label="Готово" value="success" active={filter === "success"} onClick={() => setFilter("success")} count={counts["success"] || 0} />
          <FilterButton label="Ошибка" value="error" active={filter === "error"} onClick={() => setFilter("error")} count={counts["error"] || 0} />
        </div>
        <input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Поиск по домену"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
        />
        {loading && <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
        {!loading && filtered.length === 0 && (
          <div className="text-sm text-slate-500 dark:text-slate-400">Запусков пока нет.</div>
        )}
        {!loading && filtered.length > 0 && (
          <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-4">№</th>
                <th className="py-2 pr-4">Домен</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Прогресс</th>
                <th className="py-2 pr-4">Обновлено</th>
                <th className="py-2 pr-4">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {visibleGenerations.map((g, idx) => (
                <tr key={g.id}>
                  <td className="py-3 pr-4 text-xs text-slate-500 dark:text-slate-400">
                    {genIndexBase + idx + 1}
                  </td>
                  <td className="py-3 pr-4">
                    {g.domain_url ? (
                      <Link href={{ pathname: `/domains/${g.domain_id}` }} className="text-indigo-600 hover:underline">
                        {g.domain_url}
                      </Link>
                    ) : (
                      <span className="text-slate-500 dark:text-slate-400">—</span>
                    )}
                  </td>
                  <td className="py-3 pr-4">
                    <StatusBadge status={g.status} />
                  </td>
                  <td className="py-3 pr-4">{g.progress}%</td>
                  <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                    {g.updated_at ? new Date(g.updated_at).toLocaleString() : "—"}
                  </td>
                  <td className="py-3 pr-4">
                    <Link href={{ pathname: `/queue/${g.id}` }} className="text-indigo-600 hover:underline">
                      Открыть
                    </Link>
                  </td>
                </tr>
              ))}
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

      {activeTab === "links" && (
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-4">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h3 className="font-semibold">Очередь ссылок</h3>
            <p className="text-xs text-slate-500 dark:text-slate-400">
              Фильтры — для текущей страницы, поиск — по всем страницам.
            </p>
          </div>
          <span className="text-xs text-slate-500 dark:text-slate-400">Показано: {filteredLinks.length}</span>
        </div>
        <div className="flex flex-wrap gap-2">
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.all} value="all" active={linkFilter === "all"} onClick={() => setLinkFilter("all")} count={linkTasks.length} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.pending} value="pending" active={linkFilter === "pending"} onClick={() => setLinkFilter("pending")} count={linkCounts["pending"] || 0} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.searching} value="searching" active={linkFilter === "searching"} onClick={() => setLinkFilter("searching")} count={linkCounts["searching"] || 0} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.removing} value="removing" active={linkFilter === "removing"} onClick={() => setLinkFilter("removing")} count={linkCounts["removing"] || 0} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.inserted} value="inserted" active={linkFilter === "inserted"} onClick={() => setLinkFilter("inserted")} count={linkCounts["inserted"] || 0} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.generated} value="generated" active={linkFilter === "generated"} onClick={() => setLinkFilter("generated")} count={linkCounts["generated"] || 0} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.removed} value="removed" active={linkFilter === "removed"} onClick={() => setLinkFilter("removed")} count={linkCounts["removed"] || 0} />
          <FilterButton label={QUEUE_LINK_STATUS_LABELS.failed} value="failed" active={linkFilter === "failed"} onClick={() => setLinkFilter("failed")} count={linkCounts["failed"] || 0} />
        </div>
        <input
          type="search"
          value={linkSearch}
          onChange={(e) => setLinkSearch(e.target.value)}
          placeholder="Поиск по домену"
          className="w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 placeholder:text-slate-400 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-100"
        />
        {linkError && <div className="text-sm text-red-500">{linkError}</div>}
        {linkLoading && <div className="text-sm text-slate-500 dark:text-slate-400">Загрузка...</div>}
        {!linkLoading && filteredLinks.length === 0 && (
          <div className="text-sm text-slate-500 dark:text-slate-400">Задач ссылок нет.</div>
        )}
        {!linkLoading && filteredLinks.length > 0 && (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                  <th className="py-2 pr-4">№</th>
                  <th className="py-2 pr-4">Домен</th>
                  <th className="py-2 pr-4">Действие</th>
                  <th className="py-2 pr-4">Запланировано</th>
                  <th className="py-2 pr-4">Статус</th>
                  <th className="py-2 pr-4">Попытки</th>
                  <th className="py-2 pr-4">Событие</th>
                  <th className="py-2 pr-4">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
                {visibleLinks.map((task, idx) => {
                  const actionLabel = (task.action || "insert") === "remove" ? "Удаление" : "Вставка";
                  const lastLog = task.log_lines?.length ? task.log_lines[task.log_lines.length - 1] : "";
                  const eventText = task.error_message || lastLog || "—";
                  const domainLabel = linkDomains[task.domain_id] || "Домен";
                  const retryGuard = canRetry({ busy: linkLoading, status: task.status });
                  const deleteGuard = canDelete({ busy: linkLoading, status: task.status });
                  return (
                    <tr key={task.id}>
                      <td className="py-3 pr-4 text-xs text-slate-500 dark:text-slate-400">
                        {linkIndexBase + idx + 1}
                      </td>
                      <td className="py-3 pr-4">
                        <Link href={{ pathname: `/domains/${task.domain_id}` }} className="text-indigo-600 hover:underline">
                          {domainLabel}
                        </Link>
                      </td>
                      <td className="py-3 pr-4">{actionLabel}</td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">
                        {new Date(task.scheduled_for).toLocaleString()}
                      </td>
                      <td className="py-3 pr-4">
                        <LinkTaskStatusBadge status={task.status} />
                      </td>
                      <td className="py-3 pr-4 text-slate-500 dark:text-slate-400">{task.attempts}</td>
                      <td
                        className={`py-3 pr-4 max-w-xs truncate ${task.error_message ? "text-red-500" : "text-slate-500 dark:text-slate-400"}`}
                        title={eventText}
                      >
                        {eventText}
                      </td>
                      <td className="py-3 pr-4">
                        <div className="flex flex-wrap items-center gap-3">
                          <Link href={{ pathname: `/links/${task.id}` }} className="text-indigo-600 hover:underline">
                            Открыть
                          </Link>
                          {(normalizeLinkTaskStatus(task.status) || task.status) === "failed" && (
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
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; tone: "amber" | "yellow" | "slate" | "orange" | "red" | "green"; icon: ReactNode }> = {
    pending: { text: "В очереди", tone: "amber", icon: <FiClock /> },
    processing: { text: "В работе", tone: "amber", icon: <FiPlay /> },
    pause_requested: { text: "Пауза запрошена", tone: "yellow", icon: <FiPause /> },
    paused: { text: "Приостановлено", tone: "slate", icon: <FiPause /> },
    cancelling: { text: "Отмена...", tone: "orange", icon: <FiX /> },
    cancelled: { text: "Отменено", tone: "red", icon: <FiX /> },
    success: { text: "Готово", tone: "green", icon: <FiCheck /> },
    error: { text: "Ошибка", tone: "red", icon: <FiAlertTriangle /> },
  };
  const cfg = map[status] || { text: status, tone: "slate" as const, icon: <FiClock /> };
  return <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />;
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon: ReactNode = (() => {
    if (meta.icon === "refresh") return <FiRefreshCw />;
    if (meta.icon === "check") return <FiCheck />;
    if (meta.icon === "alert") return <FiAlertTriangle />;
    return <FiClock />;
  })();
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

function FilterButton({ label, value, active, onClick, count }: { label: string; value: string; active: boolean; onClick: () => void; count: number }) {
  return (
    <button
      onClick={onClick}
      className={`inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold border ${
        active ? "bg-indigo-600 text-white border-indigo-600" : "border-slate-200 dark:border-slate-700 text-slate-700 dark:text-slate-200"
      }`}
    >
      {label} {count > 0 && <span className="text-[11px] opacity-80">({count})</span>}
    </button>
  );
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
