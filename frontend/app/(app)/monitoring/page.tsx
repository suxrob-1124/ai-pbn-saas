'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useAuthGuard } from '@/lib/useAuth';
import { authFetch, apiBase } from '@/lib/http';
import {
  Activity,
  CheckCircle2,
  AlertCircle,
  Server,
  Database,
  Zap,
  Cpu,
  Clock,
  ExternalLink,
  RefreshCw,
  BarChart3,
  LineChart,
  AlertTriangle,
  Loader2,
  FolderGit2,
  Timer,
} from 'lucide-react';

type DashboardStats = {
  pending: number;
  processing: number;
  error: number;
  avg_minutes?: number | null;
  avg_sample: number;
};

type RecentError = {
  id: string;
  domain_id: string;
  domain_url?: string | null;
  status: string;
  error?: string | null;
  updated_at: string;
};

type DashboardData = {
  stats: DashboardStats;
  recent_errors: RecentError[];
  projects: { id: string; name: string }[];
};

type ServiceStatus = 'checking' | 'operational' | 'idle' | 'down' | 'unknown';

type Service = {
  name: string;
  icon: ReactNode;
  status: ServiceStatus;
  ping: string;
};

export default function MonitoringPage() {
  useAuthGuard();

  const [loading, setLoading] = useState(true);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [apiStatus, setApiStatus] = useState<'checking' | 'up' | 'down'>('checking');
  const [dashboard, setDashboard] = useState<DashboardData | null>(null);
  const [fetchError, setFetchError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setFetchError(null);
    try {
      // Проверяем доступность API напрямую (без авторизации)
      try {
        const healthRes = await fetch(`${apiBase()}/healthz`);
        setApiStatus(healthRes.ok ? 'up' : 'down');
      } catch {
        setApiStatus('down');
      }

      // Загружаем реальные данные с дашборда
      const data = await authFetch<DashboardData>('/api/dashboard?limit=50');
      setDashboard(data);
      setLastUpdated(new Date());
    } catch (err: any) {
      setFetchError(err?.message || 'Не удалось загрузить данные мониторинга');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  const stats = dashboard?.stats;
  const recentErrors = dashboard?.recent_errors ?? [];
  const projectCount = dashboard?.projects?.length ?? 0;

  const services: Service[] = [
    {
      name: 'HTTP API (Backend)',
      icon: <Server className="w-4 h-4" />,
      status:
        apiStatus === 'checking' ? 'checking' : apiStatus === 'up' ? 'operational' : 'down',
      ping: apiStatus === 'checking' ? '...' : apiStatus === 'up' ? 'OK' : 'ERR',
    },
    {
      name: 'PostgreSQL Database',
      icon: <Database className="w-4 h-4" />,
      status:
        apiStatus === 'up' ? 'operational' : apiStatus === 'checking' ? 'checking' : 'unknown',
      ping: apiStatus === 'up' ? 'via API' : '—',
    },
    {
      name: 'Redis Cache & Queue',
      icon: <Zap className="w-4 h-4" />,
      status:
        apiStatus === 'up' ? 'operational' : apiStatus === 'checking' ? 'checking' : 'unknown',
      ping: apiStatus === 'up' ? 'via API' : '—',
    },
    {
      name: 'Background Workers',
      icon: <Cpu className="w-4 h-4" />,
      status:
        stats == null
          ? 'checking'
          : stats.processing > 0
            ? 'operational'
            : apiStatus === 'up'
              ? 'idle'
              : 'unknown',
      ping:
        stats != null
          ? stats.processing > 0
            ? `${stats.processing} активно`
            : 'Ожидает задач'
          : '...',
    },
  ];

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <span className="text-slate-900 dark:text-slate-200 font-medium">Мониторинг</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
              Статус систем
            </h1>
            {lastUpdated && !loading && (
              <p className="text-xs text-slate-400 dark:text-slate-500 mt-0.5">
                Обновлено: {lastUpdated.toLocaleTimeString()}
              </p>
            )}
          </div>
          <button
            onClick={load}
            disabled={loading}
            className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl border border-slate-200 bg-white text-sm font-medium text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-[#060d18] dark:text-slate-300 dark:hover:bg-slate-800 transition-all shadow-sm active:scale-95 disabled:opacity-60">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </header>

      {/* CONTENT */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-6">
          {/* Ошибка загрузки */}
          {fetchError && (
            <div className="p-4 bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-900/50 rounded-xl text-sm text-red-600 dark:text-red-400 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 flex-shrink-0" />
              {fetchError}
            </div>
          )}

          {/* БЫСТРЫЕ ССЫЛКИ */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Link
              href="/monitoring/indexing"
              className="group p-5 rounded-2xl bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 shadow-sm hover:border-indigo-400 dark:hover:border-indigo-500/60 transition-all flex items-center gap-4">
              <div className="w-12 h-12 rounded-full bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400 group-hover:scale-110 transition-transform flex-shrink-0">
                <BarChart3 className="w-5 h-5" />
              </div>
              <div>
                <h4 className="font-bold text-slate-900 dark:text-white">Индексация</h4>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Трекинг и статистика поисковиков
                </p>
              </div>
            </Link>

            <Link
              href="/monitoring/llm-usage"
              className="group p-5 rounded-2xl bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 shadow-sm hover:border-indigo-400 dark:hover:border-indigo-500/60 transition-all flex items-center gap-4">
              <div className="w-12 h-12 rounded-full bg-emerald-50 dark:bg-emerald-900/30 flex items-center justify-center text-emerald-600 dark:text-emerald-400 group-hover:scale-110 transition-transform flex-shrink-0">
                <LineChart className="w-5 h-5" />
              </div>
              <div>
                <h4 className="font-bold text-slate-900 dark:text-white">LLM Usage</h4>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Расход токенов и биллинг API
                </p>
              </div>
            </Link>

            <a
              href="http://localhost:3001"
              target="_blank"
              rel="noopener noreferrer"
              className="group p-5 rounded-2xl bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 shadow-sm hover:border-orange-400 dark:hover:border-orange-500/60 transition-all flex items-center gap-4">
              <div className="w-12 h-12 rounded-full bg-orange-50 dark:bg-orange-900/30 flex items-center justify-center text-orange-600 dark:text-orange-400 group-hover:scale-110 transition-transform flex-shrink-0">
                <Activity className="w-5 h-5" />
              </div>
              <div className="flex-1 min-w-0">
                <h4 className="font-bold text-slate-900 dark:text-white flex items-center gap-1.5">
                  Grafana <ExternalLink className="w-3 h-3 opacity-50" />
                </h4>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Метрики CPU, RAM и Prometheus
                </p>
              </div>
            </a>
          </div>

          {/* СТАТИСТИКА (4 карточки с реальными данными) */}
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <StatCard
              label="Проектов"
              value={loading ? null : projectCount}
              icon={<FolderGit2 className="w-5 h-5" />}
              color="indigo"
            />
            <StatCard
              label="В очереди"
              value={loading ? null : (stats?.pending ?? 0)}
              icon={<Clock className="w-5 h-5" />}
              color="amber"
            />
            <StatCard
              label="Генерируется"
              value={loading ? null : (stats?.processing ?? 0)}
              icon={<Activity className="w-5 h-5" />}
              color="blue"
              pulse={stats != null && stats.processing > 0}
            />
            <StatCard
              label="Ошибок"
              value={loading ? null : (stats?.error ?? 0)}
              icon={<AlertCircle className="w-5 h-5" />}
              color={stats != null && stats.error > 0 ? 'red' : 'slate'}
            />
          </div>

          {/* СЛУЖБЫ + ПОСЛЕДНИЕ ОШИБКИ */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* СТАТУС СЛУЖБ */}
            <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm overflow-hidden">
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                <h3 className="font-bold text-slate-900 dark:text-white">Службы платформы</h3>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                  API проверяется в реальном времени. Прочие — по статусу API.
                </p>
              </div>
              <div className="divide-y divide-slate-100 dark:divide-slate-800/60 p-2">
                {services.map((service, idx) => (
                  <div
                    key={idx}
                    className="flex items-center justify-between p-4 hover:bg-slate-50 dark:hover:bg-slate-800/30 rounded-xl transition-colors">
                    <div className="flex items-center gap-3">
                      <div className="text-slate-400 dark:text-slate-500 flex-shrink-0">
                        {service.icon}
                      </div>
                      <span className="text-sm font-medium text-slate-900 dark:text-slate-200">
                        {service.name}
                      </span>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="font-mono text-xs text-slate-400 dark:text-slate-500 hidden sm:block">
                        {service.ping}
                      </span>
                      <ServiceStatusBadge status={service.status} />
                    </div>
                  </div>
                ))}
              </div>

              {/* Среднее время генерации */}
              {!loading && stats?.avg_minutes != null && (
                <div className="p-4 border-t border-slate-100 dark:border-slate-800/60 bg-slate-50/30 dark:bg-slate-800/10 flex items-center gap-2.5">
                  <Timer className="w-4 h-4 text-slate-400 flex-shrink-0" />
                  <span className="text-xs text-slate-500 dark:text-slate-400">
                    Среднее время генерации:{' '}
                    <span className="font-bold text-slate-700 dark:text-slate-300 font-mono">
                      ~{stats.avg_minutes} мин
                    </span>{' '}
                    (выборка: {stats.avg_sample} задач)
                  </span>
                </div>
              )}
            </div>

            {/* ПОСЛЕДНИЕ ОШИБКИ ГЕНЕРАЦИИ */}
            <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm overflow-hidden">
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex items-center justify-between">
                <div>
                  <h3 className="font-bold text-slate-900 dark:text-white">Последние ошибки</h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                    Неудачные генерации за последний период.
                  </p>
                </div>
                {recentErrors.length > 0 && (
                  <span className="text-xs font-bold px-2 py-0.5 rounded-full bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-400 border border-red-100 dark:border-red-800/40">
                    {recentErrors.length}
                  </span>
                )}
              </div>

              <div className="divide-y divide-slate-100 dark:divide-slate-800/60">
                {loading ? (
                  <div className="p-8 flex items-center justify-center gap-2 text-slate-400 dark:text-slate-500 text-sm">
                    <Loader2 className="w-4 h-4 animate-spin" />
                    Загрузка...
                  </div>
                ) : recentErrors.length === 0 ? (
                  <div className="p-8 text-center">
                    <CheckCircle2 className="w-8 h-8 text-emerald-400 mx-auto mb-2" />
                    <p className="text-sm font-medium text-slate-600 dark:text-slate-400">
                      Ошибок нет
                    </p>
                    <p className="text-xs text-slate-400 dark:text-slate-500 mt-1">
                      Все генерации проходят успешно
                    </p>
                  </div>
                ) : (
                  recentErrors.map((err) => (
                    <div
                      key={err.id}
                      className="p-4 hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                      <div className="flex items-start gap-3">
                        <AlertCircle className="w-4 h-4 text-red-500 flex-shrink-0 mt-0.5" />
                        <div className="min-w-0 flex-1">
                          <p className="text-sm font-semibold text-slate-900 dark:text-slate-200 truncate">
                            {err.domain_url || err.domain_id}
                          </p>
                          {err.error && (
                            <p
                              className="text-xs text-red-500 font-mono mt-0.5 truncate"
                              title={err.error}>
                              {err.error}
                            </p>
                          )}
                          <p className="text-xs text-slate-400 dark:text-slate-500 mt-1">
                            {new Date(err.updated_at).toLocaleString()}
                          </p>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

// --- Компоненты ---

type StatColor = 'indigo' | 'amber' | 'blue' | 'red' | 'slate';

function StatCard({
  label,
  value,
  icon,
  color,
  pulse = false,
}: {
  label: string;
  value: number | null;
  icon: ReactNode;
  color: StatColor;
  pulse?: boolean;
}) {
  const colorMap: Record<StatColor, { bg: string; text: string }> = {
    indigo: {
      bg: 'bg-indigo-50 dark:bg-indigo-900/30',
      text: 'text-indigo-600 dark:text-indigo-400',
    },
    amber: {
      bg: 'bg-amber-50 dark:bg-amber-900/30',
      text: 'text-amber-600 dark:text-amber-400',
    },
    blue: {
      bg: 'bg-blue-50 dark:bg-blue-900/30',
      text: 'text-blue-600 dark:text-blue-400',
    },
    red: {
      bg: 'bg-red-50 dark:bg-red-900/30',
      text: 'text-red-600 dark:text-red-400',
    },
    slate: {
      bg: 'bg-slate-100 dark:bg-slate-800/50',
      text: 'text-slate-500 dark:text-slate-400',
    },
  };
  const c = colorMap[color];

  return (
    <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700 rounded-2xl shadow-sm p-5 flex items-center gap-4">
      <div
        className={`w-12 h-12 rounded-xl flex items-center justify-center flex-shrink-0 ${c.bg} ${c.text} ${pulse ? 'animate-pulse' : ''}`}>
        {icon}
      </div>
      <div className="min-w-0">
        <p className="text-[11px] font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
          {label}
        </p>
        {value === null ? (
          <div className="h-8 w-12 bg-slate-100 dark:bg-slate-800 rounded-lg animate-pulse mt-1" />
        ) : (
          <p className={`text-3xl font-bold font-mono leading-none mt-1 ${c.text}`}>{value}</p>
        )}
      </div>
    </div>
  );
}

function ServiceStatusBadge({ status }: { status: ServiceStatus }) {
  const map: Record<ServiceStatus, { label: string; className: string; dot: string }> = {
    operational: {
      label: 'Работает',
      className:
        'bg-emerald-50 text-emerald-700 border-emerald-200/50 dark:bg-emerald-500/10 dark:text-emerald-400 dark:border-emerald-500/20',
      dot: 'bg-emerald-500',
    },
    idle: {
      label: 'Ожидает',
      className:
        'bg-amber-50 text-amber-700 border-amber-200/50 dark:bg-amber-500/10 dark:text-amber-400 dark:border-amber-500/20',
      dot: 'bg-amber-400',
    },
    checking: {
      label: 'Проверка...',
      className:
        'bg-slate-50 text-slate-500 border-slate-200 dark:bg-slate-800/50 dark:text-slate-400 dark:border-slate-700',
      dot: 'bg-slate-400 animate-pulse',
    },
    down: {
      label: 'Недоступен',
      className:
        'bg-red-50 text-red-700 border-red-200/50 dark:bg-red-500/10 dark:text-red-400 dark:border-red-500/20',
      dot: 'bg-red-500',
    },
    unknown: {
      label: 'Неизвестно',
      className:
        'bg-slate-50 text-slate-500 border-slate-200 dark:bg-slate-800/50 dark:text-slate-400 dark:border-slate-700',
      dot: 'bg-slate-400',
    },
  };
  const s = map[status];
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-xs font-bold ${s.className}`}>
      <span className={`w-1.5 h-1.5 rounded-full flex-shrink-0 ${s.dot}`} />
      {s.label}
    </span>
  );
}
