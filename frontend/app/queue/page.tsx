"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { authFetch } from "../../lib/http";
import { useAuthGuard } from "../../lib/useAuth";
import { FiClock, FiPlay, FiCheck, FiAlertTriangle, FiRefreshCw, FiPause, FiX } from "react-icons/fi";

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
  useAuthGuard();
  const [items, setItems] = useState<Generation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState("all");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await authFetch<Generation[]>("/api/generations?limit=50&lite=1");
      setItems(Array.isArray(res) ? res : []);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить очередь");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    const timer = window.setInterval(load, 5000);
    return () => window.clearInterval(timer);
  }, [load]);

  const filtered = useMemo(() => {
    if (filter === "all") return items;
    return items.filter((i) => i.status === filter);
  }, [filter, items]);

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const i of items) {
      c[i.status] = (c[i.status] || 0) + 1;
    }
    return c;
  }, [items]);

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-2xl font-bold">Очередь генерации</h1>
            <p className="text-sm text-slate-500 dark:text-slate-400">
              Последние запуски по всем проектам. Фильтруй по статусу или переходи в карточку задачи.
            </p>
          </div>
          <div className="flex gap-2">
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
            <Link href="/projects" className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-500">
              ← К проектам
            </Link>
          </div>
        </div>
        {error && <div className="mt-2 text-red-500 text-sm">{error}</div>}

        <div className="mt-4 flex flex-wrap gap-2">
          <FilterButton label="Все" value="all" active={filter === "all"} onClick={() => setFilter("all")} count={items.length} />
          <FilterButton label="В очереди" value="pending" active={filter === "pending"} onClick={() => setFilter("pending")} count={counts["pending"] || 0} />
          <FilterButton label="В работе" value="processing" active={filter === "processing"} onClick={() => setFilter("processing")} count={counts["processing"] || 0} />
          <FilterButton label="Готово" value="success" active={filter === "success"} onClick={() => setFilter("success")} count={counts["success"] || 0} />
          <FilterButton label="Ошибка" value="error" active={filter === "error"} onClick={() => setFilter("error")} count={counts["error"] || 0} />
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="font-semibold">Запуски</h3>
          <span className="text-xs text-slate-500 dark:text-slate-400">Всего: {filtered.length}</span>
        </div>
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-slate-500 dark:text-slate-400 border-b border-slate-200 dark:border-slate-800">
                <th className="py-2 pr-4">ID</th>
                <th className="py-2 pr-4">Домен</th>
                <th className="py-2 pr-4">Статус</th>
                <th className="py-2 pr-4">Прогресс</th>
                <th className="py-2 pr-4">Обновлено</th>
                <th className="py-2 pr-4">Действия</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-800">
              {filtered.map((g) => (
                <tr key={g.id}>
                  <td className="py-3 pr-4 font-mono text-xs">{g.id.slice(0, 8)}</td>
                  <td className="py-3 pr-4">
                    {g.domain_url ? (
                      <Link href={`/domains/${g.domain_id}`} className="text-indigo-600 hover:underline">
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
                    <Link href={`/queue/${g.id}`} className="text-indigo-600 hover:underline">
                      Открыть
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    pending: { text: "В очереди", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiClock /> },
    processing: { text: "В работе", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPlay /> },
    pause_requested: { text: "Пауза запрошена", color: "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-200", icon: <FiPause /> },
    paused: { text: "Приостановлено", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPause /> },
    cancelling: { text: "Отмена...", color: "bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-200", icon: <FiX /> },
    cancelled: { text: "Отменено", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiX /> },
    success: { text: "Готово", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiCheck /> },
    error: { text: "Ошибка", color: "bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-200", icon: <FiAlertTriangle /> },
  };
  const cfg = map[status] || { text: status, color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiClock /> };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
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
