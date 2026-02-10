"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { FiRefreshCw, FiTrash2 } from "react-icons/fi";
import { listQueue, deleteQueueItem, cleanupQueue } from "../../../../lib/queueApi";
import { authFetch } from "../../../../lib/http";
import { useAuthGuard } from "../../../../lib/useAuth";
import { showToast } from "../../../../lib/toastStore";
import type { QueueItemDTO } from "../../../../types/queue";

const statusOptions = ["all", "pending", "queued", "completed", "failed"];
const STATUS_LABELS: Record<string, string> = {
  all: "Все",
  pending: "Ожидает",
  queued: "В очереди",
  completed: "Завершено",
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
  const projectId = params?.id as string;

  const [items, setItems] = useState<QueueItemDTO[]>([]);
  const [domains, setDomains] = useState<Record<string, Domain>>({});
  const [loading, setLoading] = useState(false);
  const [cleaning, setCleaning] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [permissionDenied, setPermissionDenied] = useState(false);

  const [statusFilter, setStatusFilter] = useState("all");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  const load = async (opts?: { silent?: boolean }) => {
    if (!projectId) return;
    if (!opts?.silent) {
      setLoading(true);
    }
    setError(null);
    setPermissionDenied(false);
    try {
      const list = await listQueue(projectId);
      setItems(Array.isArray(list) ? list : []);
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

  useEffect(() => {
    load();
  }, [projectId]);

  const filtered = useMemo(() => {
    const fromDate = dateFrom ? new Date(`${dateFrom}T00:00:00`) : null;
    const toDate = dateTo ? new Date(`${dateTo}T23:59:59`) : null;
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
      return true;
    });
  }, [items, statusFilter, dateFrom, dateTo]);

  const handleRemove = async (item: QueueItemDTO) => {
    if (!confirm(`Удалить из очереди: ${item.id}?`)) return;
    setLoading(true);
    setError(null);
    try {
      await deleteQueueItem(item.id);
      showToast({
        type: "success",
        title: "Удалено из очереди",
        message: item.id
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
      await load({ silent: true });
    } finally {
      setRefreshing(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-semibold">Очередь проекта</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">ID проекта: {projectId}</p>
          </div>
          <div className="flex items-center gap-2">
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

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
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
      </div>

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
                {filtered.map((item) => {
                  const domain = domains[item.domain_id];
                  const domainLabel = item.domain_url || domain?.url || item.domain_id;
                  return (
                    <tr key={item.id}>
                      <td className="py-3 pr-4">
                        {domain || item.domain_url ? (
                          <Link href={`/domains/${domain?.id || item.domain_id}`} className="text-indigo-600 hover:underline">
                            {domainLabel}
                          </Link>
                        ) : (
                          <span className="text-slate-500 dark:text-slate-400">{item.domain_id}</span>
                        )}
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
      </div>
    </div>
  );
}
