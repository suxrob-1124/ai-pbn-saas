'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useAuthGuard } from '@/lib/useAuth';
import { authFetchCached, post } from '@/lib/http';
import { Badge } from '@/components/Badge';
import {
  Plus,
  RefreshCw,
  Activity,
  Clock,
  AlertCircle,
  Globe2,
  ArrowRight,
  X,
  Play,
  PauseCircle,
} from 'lucide-react';

// --- ТИПЫ ---
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

// --- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ (Для красоты) ---

// 1. Уникальный градиент на основе строки (например, ID проекта)
function getProjectGradient(id: string) {
  const gradients = [
    'from-teal-400 to-emerald-500',
    'from-blue-400 to-indigo-500',
    'from-violet-400 to-purple-500',
    'from-rose-400 to-pink-500',
    'from-amber-400 to-orange-500',
    'from-cyan-400 to-blue-500',
  ];
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = id.charCodeAt(i) + ((hash << 5) - hash);
  }
  return gradients[Math.abs(hash) % gradients.length];
}

// 2. Две первые буквы для аватара
function getInitials(name: string) {
  return name.substring(0, 2).toUpperCase();
}

// 3. Человекочитаемое время ("2 часа назад")
function timeAgo(dateString: string) {
  const date = new Date(dateString);
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diffInSeconds < 60) return 'Только что';
  if (diffInSeconds < 3600) return `${Math.floor(diffInSeconds / 60)} мин. назад`;
  if (diffInSeconds < 86400) return `${Math.floor(diffInSeconds / 3600)} ч. назад`;
  if (diffInSeconds < 2592000) return `${Math.floor(diffInSeconds / 86400)} дн. назад`;
  return date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short', year: 'numeric' });
}

export default function ProjectsPage() {
  const { me } = useAuthGuard();

  const [projects, setProjects] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [name, setName] = useState('');
  const [country, setCountry] = useState('');
  const [language, setLanguage] = useState('');
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);

  const [genStats, setGenStats] = useState({
    pending: 0,
    processing: 0,
    error: 0,
    avgMinutes: null as number | null,
    avgSample: 0,
  });
  const [recentErrors, setRecentErrors] = useState<GenerationDTO[]>([]);

  const hasExtendedAccess = me?.role === 'admin';
  const canCreate = me?.role === 'admin' || me?.role === 'manager';

  const load = async (force = false) => {
    setLoading(true);
    setError(null);
    try {
      const dashboard = await authFetchCached<DashboardDTO>('/api/dashboard', undefined, {
        ttlMs: 15000,
        bypassCache: force,
      });
      setProjects(Array.isArray(dashboard?.projects) ? dashboard.projects : []);
      if (dashboard?.stats) {
        setGenStats({
          pending: dashboard.stats.pending || 0,
          processing: dashboard.stats.processing || 0,
          error: dashboard.stats.error || 0,
          avgMinutes: dashboard.stats.avg_minutes ?? null,
          avgSample: dashboard.stats.avg_sample || 0,
        });
      }
      setRecentErrors(Array.isArray(dashboard?.recent_errors) ? dashboard.recent_errors : []);
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить проекты');
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
    try {
      await post('/api/projects', { name, country, language });
      setName('');
      setCountry('');
      setLanguage('');
      setIsCreateModalOpen(false);
      await load(true);
    } catch (err: any) {
      setError(err?.message || 'Не удалось создать проект');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 md:p-8 lg:p-10 max-w-7xl mx-auto space-y-8">
      {/* HEADER */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold tracking-tight text-slate-900 dark:text-white">
            {canCreate ? 'Проекты' : 'Мои сайты'}
          </h1>
          <p className="text-slate-500 dark:text-slate-400 mt-1">
            {canCreate
              ? 'Рабочие пространства для управления сетками сайтов.'
              : 'Выберите рабочее пространство для управления контентом.'}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => load(true)}
            disabled={loading}
            className="p-2.5 rounded-lg border border-slate-200 bg-white text-slate-600 hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-950 dark:text-slate-300 dark:hover:bg-slate-900 transition-colors"
            title="Обновить данные">
            <RefreshCw className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} />
          </button>

          {canCreate && (
            <button
              onClick={() => setIsCreateModalOpen(true)}
              className="inline-flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-indigo-500 transition-colors shadow-sm">
              <Plus className="w-5 h-5" /> Новый проект
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="p-4 bg-red-50 text-red-700 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 flex items-center gap-3">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          {error}
        </div>
      )}

      {/* DASHBOARD METRICS (Vercel Style) */}
      {hasExtendedAccess && (
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <MetricCard
            title="Активные задачи"
            value={genStats.pending + genStats.processing}
            subtitle={`В работе: ${genStats.processing}`}
            icon={<Activity className="w-4 h-4" />}
          />
          <MetricCard
            title="Среднее время"
            value={genStats.avgMinutes != null ? `~${genStats.avgMinutes} мин` : '—'}
            subtitle={genStats.avgSample > 0 ? `Выборка: ${genStats.avgSample} запусков` : 'Нет полных запусков'}
            icon={<Clock className="w-4 h-4" />}
          />
          <MetricCard
            title="Ошибки генерации"
            value={genStats.error}
            subtitle="За последние 100 запусков"
            icon={<AlertCircle className="w-4 h-4" />}
            isError={genStats.error > 0}
          />
        </div>
      )}

      {/* PROJECTS GRID */}
      <div>
        {loading && projects.length === 0 && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[1, 2, 3].map((i) => (
              <div
                key={i}
                className="h-40 rounded-2xl bg-slate-100 dark:bg-slate-800/50 animate-pulse"
              />
            ))}
          </div>
        )}

        {!loading && projects.length === 0 && (
          <div className="flex flex-col items-center justify-center py-20 px-4 border-2 border-dashed border-slate-200 dark:border-slate-800 rounded-2xl text-slate-500 bg-white/50 dark:bg-slate-900/20">
            <Globe2 className="w-12 h-12 mb-4 text-slate-300 dark:text-slate-700" />
            <h3 className="text-lg font-medium text-slate-900 dark:text-white">Нет проектов</h3>
            <p className="mt-1 text-sm text-center max-w-sm">
              Создайте первый проект, чтобы начать добавлять домены и генерировать сайты.
            </p>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
          {projects.map((p) => (
            <Link
              key={p.id}
              href={`/projects/${p.id}`}
              className="group flex flex-col bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm hover:shadow-md hover:border-indigo-400 dark:hover:border-indigo-500/60 transition-all overflow-hidden">
              <div className="p-6 flex-1">
                <div className="flex items-start justify-between mb-4">
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className={`w-10 h-10 rounded-full flex items-center justify-center text-white font-bold shadow-inner bg-gradient-to-br ${getProjectGradient(p.id)} flex-shrink-0`}>
                      {getInitials(p.name)}
                    </div>
                    <div className="min-w-0">
                      <h3
                        className="font-semibold text-lg text-slate-900 dark:text-white truncate"
                        title={p.name}>
                        {p.name}
                      </h3>
                      <div className="flex items-center gap-2 mt-0.5 text-xs font-medium text-slate-500 dark:text-slate-400">
                        {p.target_country && (
                          <span className="bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
                            {p.target_country}
                          </span>
                        )}
                        {p.target_language && (
                          <span className="bg-slate-100 dark:bg-slate-800 px-2 py-0.5 rounded-md">
                            {p.target_language}
                          </span>
                        )}
                        {!p.target_country && !p.target_language && <span>Универсальный</span>}
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div className="px-6 py-3.5 bg-slate-50/50 dark:bg-slate-800/20 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between">
                <div className="flex items-center text-xs text-slate-500 dark:text-slate-400">
                  <Clock className="w-3.5 h-3.5 mr-1.5 opacity-70" />
                  {p.updated_at ? `Изменен ${timeAgo(p.updated_at)}` : 'Создан недавно'}
                </div>
                <div className="flex items-center text-xs font-medium text-indigo-600 dark:text-indigo-400 opacity-0 group-hover:opacity-100 transition-opacity">
                  Перейти <ArrowRight className="w-3.5 h-3.5 ml-1" />
                </div>
              </div>
            </Link>
          ))}
        </div>
      </div>

      {/* RECENT ERRORS LOG (Admin only) */}
      {hasExtendedAccess && recentErrors.length > 0 && (
        <div className="mt-12 space-y-4">
          <h3 className="text-lg font-semibold flex items-center gap-2 text-slate-900 dark:text-white">
            <AlertCircle className="w-5 h-5 text-red-500" />
            Требует внимания (Ошибки генерации)
          </h3>
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl overflow-hidden shadow-sm">
            <div className="divide-y divide-slate-100 dark:divide-slate-800/50">
              {recentErrors.map((g) => (
                <div
                  key={g.id}
                  className="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
                  <div className="min-w-0 flex-1">
                    <div className="font-medium text-slate-900 dark:text-slate-100 truncate">
                      {g.domain_url || 'Неизвестный домен'}
                    </div>
                    <div
                      className="text-sm text-slate-500 dark:text-slate-400 mt-1 line-clamp-1"
                      title={g.error || ''}>
                      <span className="text-red-600 dark:text-red-400 font-medium">Ошибка: </span>
                      {g.error || 'Неизвестный сбой'}
                    </div>
                  </div>
                  <div className="flex items-center gap-4 flex-shrink-0">
                    <span className="text-xs text-slate-400 hidden sm:block">
                      {g.updated_at ? timeAgo(g.updated_at) : ''}
                    </span>
                    <Link
                      href={`/queue/${g.id}`}
                      className="px-3 py-1.5 text-xs font-medium text-slate-700 bg-white border border-slate-200 rounded-lg hover:bg-slate-50 hover:text-indigo-600 dark:bg-slate-800 dark:border-slate-700 dark:text-slate-200 dark:hover:bg-slate-700 transition-colors">
                      Разбор
                    </Link>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* CREATE MODAL */}
      {isCreateModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40 backdrop-blur-sm">
          <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl p-6 shadow-2xl w-full max-w-md animate-in fade-in zoom-in-95 duration-200">
            <div className="flex justify-between items-center mb-6">
              <h3 className="text-xl font-bold text-slate-900 dark:text-white">Новый проект</h3>
              <button
                onClick={() => setIsCreateModalOpen(false)}
                className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 bg-slate-100 dark:bg-slate-800 p-1.5 rounded-full transition-colors">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-5">
              <div>
                <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
                  Название <span className="text-red-500">*</span>
                </label>
                <input
                  autoFocus
                  className="w-full rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-sm text-slate-900 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 transition-all"
                  placeholder="Например: PBN Health Network"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
                    Страна
                  </label>
                  <input
                    className="w-full rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-sm text-slate-900 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 transition-all"
                    placeholder="US, DE, FR..."
                    value={country}
                    onChange={(e) => setCountry(e.target.value)}
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-slate-700 dark:text-slate-300 mb-1.5">
                    Язык
                  </label>
                  <input
                    className="w-full rounded-xl border border-slate-300 bg-white px-4 py-2.5 text-sm text-slate-900 focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500/20 outline-none dark:border-slate-700 dark:bg-slate-950 dark:text-slate-100 transition-all"
                    placeholder="en-US, de-DE..."
                    value={language}
                    onChange={(e) => setLanguage(e.target.value)}
                  />
                </div>
              </div>
            </div>

            <div className="mt-8 flex justify-end gap-3 pt-4 border-t border-slate-100 dark:border-slate-800">
              <button
                onClick={() => setIsCreateModalOpen(false)}
                className="px-4 py-2.5 text-sm font-medium text-slate-600 hover:bg-slate-100 rounded-xl dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                Отмена
              </button>
              <button
                onClick={createProject}
                disabled={loading || !name.trim()}
                className="px-6 py-2.5 text-sm font-medium text-white bg-indigo-600 hover:bg-indigo-500 rounded-xl disabled:opacity-50 transition-all shadow-sm active:scale-95">
                {loading ? 'Создание...' : 'Создать'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// Вынесенный UI-компонент метрики
function MetricCard({ title, value, subtitle, icon, isError = false }: any) {
  return (
    <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-2xl p-5 shadow-sm">
      <div className="flex items-center gap-2 text-slate-500 dark:text-slate-400 mb-3">
        {icon}
        <h4 className="text-sm font-medium">{title}</h4>
      </div>
      <div className="flex items-baseline gap-3">
        <span
          className={`text-3xl font-bold tracking-tight ${isError ? 'text-red-500' : 'text-slate-900 dark:text-white'}`}>
          {value}
        </span>
      </div>
      <p className="text-xs text-slate-500 mt-1">{subtitle}</p>
    </div>
  );
}

