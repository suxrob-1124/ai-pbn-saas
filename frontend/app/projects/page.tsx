"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuthGuard } from "../../lib/useAuth";
import { authFetch, post } from "../../lib/http";
import { FiFolder, FiPlay, FiPauseCircle, FiPlus, FiRefreshCw, FiClock } from "react-icons/fi";

export default function ProjectsPage() {
  useAuthGuard();
  const [projects, setProjects] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await authFetch<any[]>("/api/projects");
      setProjects(Array.isArray(data) ? data : []);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить проекты");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const createProject = async () => {
    if (!name.trim()) return;
    setLoading(true);
    setError(null);
    try {
      await post("/api/projects", {
        name,
        country,
        language
      });
      setName("");
      setCountry("");
      setLanguage("");
      await load();
    } catch (err: any) {
      setError(err?.message || "Не удалось создать проект");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <h2 className="text-xl font-semibold mb-2">Проекты</h2>
        <p className="text-sm text-slate-500 dark:text-slate-400">
          Заготовка под управление сгенерированными сайтами. Дальше здесь появятся CRUD, статусы и метрики.
        </p>
        <div className="mt-4 grid gap-3 md:grid-cols-4">
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Название"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Страна (SE)"
            value={country}
            onChange={(e) => setCountry(e.target.value)}
          />
          <input
            className="rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-100"
            placeholder="Язык (sv-SE)"
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          />
          <div className="flex gap-2">
            <button
              onClick={createProject}
              disabled={loading || !name.trim()}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-semibold text-white hover:bg-indigo-500 disabled:opacity-50"
            >
              <FiPlus /> Создать
            </button>
            <button
              onClick={load}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            >
              <FiRefreshCw /> Обновить
            </button>
          </div>
        </div>
        {error && <div className="text-red-500 text-sm mt-2">{error}</div>}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {loading && projects.length === 0 && <div className="text-slate-500">Загрузка...</div>}
        {!loading && projects.length === 0 && <div className="text-slate-500">Проектов пока нет.</div>}
        {projects.map((p) => (
          <Link key={p.id} href={`/projects/${p.id}`} className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow hover:border-indigo-500 block">
            <div className="flex items-center justify-between">
              <h3 className="font-semibold flex items-center gap-2">
                <FiFolder /> {p.name}
              </h3>
              <StatusBadge status={p.status || "draft"} />
            </div>
            <p className="text-sm text-slate-500 dark:text-slate-400 mt-2">
              {p.target_country ? `Страна: ${p.target_country}` : "Страна не задана"}
            </p>
            <div className="text-xs text-slate-500 dark:text-slate-400 mt-3 flex items-center gap-3">
              <span>Язык: {p.target_language || "—"}</span>
              <span className="flex items-center gap-1">
                <FiClock /> {p.updated_at ? new Date(p.updated_at).toLocaleDateString() : "—"}
              </span>
            </div>
          </Link>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h4 className="text-sm font-semibold mb-2">Очередь публикаций</h4>
          <div className="text-3xl font-bold">—</div>
          <div className="text-xs text-slate-500 dark:text-slate-400">подключим после воркера</div>
        </div>
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h4 className="text-sm font-semibold mb-2">Средний билд</h4>
          <div className="text-3xl font-bold">—</div>
          <div className="text-xs text-slate-500 dark:text-slate-400">данных пока нет</div>
        </div>
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h4 className="text-sm font-semibold mb-2">Uptime генератора</h4>
          <div className="text-3xl font-bold">—</div>
          <div className="text-xs text-slate-500 dark:text-slate-400">плейсхолдер под мониторинг</div>
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; color: string; icon: React.ReactNode }> = {
    Активен: { text: "Активен", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiPlay /> },
    active: { text: "Активен", color: "bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-200", icon: <FiPlay /> },
    Черновик: { text: "Черновик", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> },
    draft: { text: "Черновик", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> },
    "В разработке": { text: "В разработке", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPauseCircle /> },
    wip: { text: "В разработке", color: "bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-200", icon: <FiPauseCircle /> },
    paused: { text: "Приостановлено", color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> }
  };
  const cfg = map[status] || { text: status, color: "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200", icon: <FiPauseCircle /> };
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs font-semibold ${cfg.color}`}>
      {cfg.icon} {cfg.text}
    </span>
  );
}
