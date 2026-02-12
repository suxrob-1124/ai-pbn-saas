"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { useAuthGuard } from "../../lib/useAuth";
import { authFetchCached, post } from "../../lib/http";
import { FiFolder, FiPlay, FiPauseCircle, FiPlus, FiRefreshCw, FiClock, FiAlertCircle } from "react-icons/fi";
import { Badge } from "../../components/Badge";

type GenerationDTO = {
  id: string;
  domain_id: string;
  domain_url?: string | null;
  status: string;
  error?: string | null;
  started_at?: string | null;
  finished_at?: string | null;
  updated_at?: string | null;
};

type DashboardDTO = {
  projects: any[];
  stats: {
    pending: number;
    processing: number;
    error: number;
    avg_minutes?: number | null;
    avg_sample: number;
  };
  recent_errors: GenerationDTO[];
};

export default function ProjectsPage() {
  useAuthGuard();
  const [projects, setProjects] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [country, setCountry] = useState("");
  const [language, setLanguage] = useState("");
  const [genStats, setGenStats] = useState({
    pending: 0,
    processing: 0,
    error: 0,
    avgMinutes: null as number | null,
    avgSample: 0
  });
  const [recentErrors, setRecentErrors] = useState<GenerationDTO[]>([]);
  const [recentErrorsError, setRecentErrorsError] = useState<string | null>(null);

  const load = async (force = false) => {
    setLoading(true);
    setError(null);
    try {
      const dashboard = await authFetchCached<DashboardDTO>("/api/dashboard", undefined, {
        ttlMs: 15000,
        bypassCache: force
      });
      setProjects(Array.isArray(dashboard?.projects) ? dashboard.projects : []);
      if (dashboard?.stats) {
        setGenStats({
          pending: dashboard.stats.pending || 0,
          processing: dashboard.stats.processing || 0,
          error: dashboard.stats.error || 0,
          avgMinutes: dashboard.stats.avg_minutes ?? null,
          avgSample: dashboard.stats.avg_sample || 0
        });
      } else {
        setGenStats({ pending: 0, processing: 0, error: 0, avgMinutes: null, avgSample: 0 });
      }
      if (Array.isArray(dashboard?.recent_errors)) {
        setRecentErrors(dashboard.recent_errors);
        setRecentErrorsError(null);
      } else {
        setRecentErrors([]);
        setRecentErrorsError("Не удалось загрузить ошибки генерации");
      }
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
      await load(true);
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
          Создавайте проекты, управляйте доменами и следите за генерацией и ошибками в одном месте.
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
              onClick={() => load(true)}
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
          <h4 className="text-sm font-semibold mb-2">Очередь генераций</h4>
          <div className="text-3xl font-bold">{genStats.pending + genStats.processing}</div>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            В очереди: {genStats.pending} · В работе: {genStats.processing}
          </div>
        </div>
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h4 className="text-sm font-semibold mb-2">Средняя длительность</h4>
          <div className="text-3xl font-bold">{genStats.avgMinutes ? `${genStats.avgMinutes} мин` : "—"}</div>
          <div className="text-xs text-slate-500 dark:text-slate-400">
            По успешным генерациям: {genStats.avgSample || "нет данных"}
          </div>
        </div>
        <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
          <h4 className="text-sm font-semibold mb-2">Ошибки генератора</h4>
          <div className="text-3xl font-bold">{genStats.error}</div>
          <div className="text-xs text-slate-500 dark:text-slate-400">по последним 100 запускам</div>
        </div>
      </div>

      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
        <div className="flex items-center justify-between">
          <h4 className="text-sm font-semibold">Последние ошибки генерации</h4>
          <button
            onClick={() => load(true)}
            disabled={loading}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
          >
            <FiRefreshCw /> Обновить
          </button>
        </div>
        {recentErrorsError && <div className="mt-2 text-sm text-red-500">{recentErrorsError}</div>}
        {!recentErrorsError && recentErrors.length === 0 && (
          <div className="mt-3 text-sm text-slate-500 dark:text-slate-400">Ошибок пока нет.</div>
        )}
        <div className="mt-3 space-y-3">
          {recentErrors.map((g) => {
            const label = g.domain_url || "Неизвестный домен";
            const when = g.updated_at || g.finished_at || g.started_at;
            const timeLabel = when ? new Date(when).toLocaleString() : "—";
            const message = (g.error || "Ошибка не указана").trim();
            const shortMessage = message.length > 140 ? `${message.slice(0, 140)}…` : message;
            return (
              <div
                key={g.id}
                className="flex flex-col gap-3 rounded-lg border border-slate-200 bg-white/80 px-3 py-2 text-sm dark:border-slate-800 dark:bg-slate-900/60 md:flex-row md:items-center md:justify-between"
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2 font-semibold text-slate-900 dark:text-slate-100">
                    <FiAlertCircle className="text-red-500" /> {label}
                  </div>
                  <div className="text-xs text-slate-500 dark:text-slate-400">
                    {timeLabel} · {shortMessage}
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Link
                    href={`/queue/${g.id}`}
                    className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-white px-3 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                  >
                    Открыть
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { text: string; tone: "green" | "slate" | "amber"; icon: ReactNode }> = {
    Активен: { text: "Активен", tone: "green", icon: <FiPlay /> },
    active: { text: "Активен", tone: "green", icon: <FiPlay /> },
    Черновик: { text: "В подготовке", tone: "slate", icon: <FiClock /> },
    draft: { text: "В подготовке", tone: "slate", icon: <FiClock /> },
    "В разработке": { text: "В работе", tone: "amber", icon: <FiPauseCircle /> },
    wip: { text: "В работе", tone: "amber", icon: <FiPauseCircle /> },
    paused: { text: "Пауза", tone: "slate", icon: <FiPauseCircle /> },
  };
  const cfg = map[status] || { text: status, tone: "slate" as const, icon: <FiPauseCircle /> };
  return <Badge label={cfg.text} tone={cfg.tone} icon={cfg.icon} className="text-xs" />;
}
