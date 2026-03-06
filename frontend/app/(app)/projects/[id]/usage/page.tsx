'use client';

import Link from 'next/link';
import { useParams } from 'next/navigation';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { DollarSign, RefreshCw, ChevronRight, Activity, Database, Zap, Search } from 'lucide-react';
import { listProjectLLMUsageEvents, listProjectLLMUsageStats } from '@/lib/llmUsageApi';
import { authFetchCached } from '@/lib/http';
import { useAuthGuard } from '@/lib/useAuth';
import type { LLMUsageEventDTO, LLMUsageFilters, LLMUsageStatsDTO } from '@/types/llmUsage';
import { UsageCostValue } from '@/features/llm-usage/components/UsageCostValue';
import { UsageTokenSourceBadge } from '@/features/llm-usage/components/UsageTokenSourceBadge';

// Переиспользуем наши обновленные фильтры
import { FilterDateInput } from '@/features/queue-monitoring/components/FilterDateInput';
import { FilterSelect } from '@/features/queue-monitoring/components/FilterSelect';
import { FilterSearchInput } from '@/features/queue-monitoring/components/FilterSearchInput';

const DEFAULT_LIMIT = 50;

type ProjectSummaryResponse = {
  project?: { id?: string; name?: string };
  domains?: Array<{ id?: string; url?: string }>;
};

export default function ProjectLLMUsagePage() {
  useAuthGuard();
  const params = useParams();
  const projectId = String(params?.id || '').trim();

  const [projectName, setProjectName] = useState('');
  const [projectDomains, setProjectDomains] = useState<Array<{ id: string; url: string }>>([]);
  const [items, setItems] = useState<LLMUsageEventDTO[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<LLMUsageStatsDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [page, setPage] = useState(1);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [model, setModel] = useState('all');
  const [operation, setOperation] = useState('all');
  const [status, setStatus] = useState('all');
  const [userEmail, setUserEmail] = useState('');
  const [domainQuery, setDomainQuery] = useState('');

  const totalPages = Math.max(1, Math.ceil(total / DEFAULT_LIMIT));

  const domainLookup = useMemo(() => {
    const map = new Map<string, string>();
    for (const domain of projectDomains) {
      map.set(domain.id, domain.url);
    }
    return map;
  }, [projectDomains]);

  const resolvedDomainId = useMemo(() => {
    const query = domainQuery.trim().toLowerCase();
    if (!query) return '';
    const exact = projectDomains.find((item) => item.url.toLowerCase() === query);
    return exact?.id || '';
  }, [domainQuery, projectDomains]);

  const modelOptions = useMemo(() => {
    const values = new Set<string>([
      'gemini-2.5-flash',
      'gemini-2.5-pro',
      'gemini-2.5-flash-image',
    ]);
    for (const item of items) {
      if (item.model?.trim()) values.add(item.model.trim());
    }
    return [
      { value: 'all', label: 'Все модели' },
      ...Array.from(values)
        .sort()
        .map((v) => ({ value: v, label: v })),
    ];
  }, [items]);

  const operationOptions = useMemo(() => {
    const values = new Set<string>([
      'generation_step/competitor_analysis',
      'generation_step/content_generation',
      'editor_ai_suggest',
      'editor_ai_create_page',
    ]);
    for (const item of items) {
      if (item.operation?.trim()) values.add(item.operation.trim());
    }
    return [
      { value: 'all', label: 'Все операции' },
      ...Array.from(values)
        .sort()
        .map((v) => ({ value: v, label: v.split('/').pop() || v })),
    ];
  }, [items]);

  const statusOptions = [
    { value: 'all', label: 'Все статусы' },
    { value: 'success', label: 'Успех' },
    { value: 'error', label: 'Ошибка' },
  ];

  const visibleItems = useMemo(() => {
    const query = domainQuery.trim().toLowerCase();
    if (!query) return items;
    return items.filter((item) => {
      const label = item.domain_id ? domainLookup.get(item.domain_id) || item.domain_id : '';
      return label.toLowerCase().includes(query);
    });
  }, [domainLookup, domainQuery, items]);

  const filters = useMemo<LLMUsageFilters>(() => {
    const value: LLMUsageFilters = { page, limit: DEFAULT_LIMIT };
    if (from.trim()) value.from = from.trim();
    if (to.trim()) value.to = to.trim();
    if (model !== 'all') value.model = model;
    if (operation !== 'all') value.operation = operation;
    if (status !== 'all') value.status = status;
    if (userEmail.trim()) value.userEmail = userEmail.trim();
    if (resolvedDomainId) value.domainId = resolvedDomainId;
    return value;
  }, [from, model, operation, page, resolvedDomainId, status, to, userEmail]);

  const load = useCallback(async () => {
    if (!projectId) return;
    setLoading(true);
    setError(null);
    try {
      const [eventsRes, statsRes] = await Promise.all([
        listProjectLLMUsageEvents(projectId, filters),
        listProjectLLMUsageStats(projectId, filters),
      ]);
      setItems(Array.isArray(eventsRes.items) ? eventsRes.items : []);
      setTotal(eventsRes.total || 0);
      setStats(statsRes);
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить usage проекта');
    } finally {
      setLoading(false);
    }
  }, [filters, projectId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    let active = true;
    if (!projectId) {
      setProjectName('');
      return;
    }
    authFetchCached<ProjectSummaryResponse>(
      `/api/projects/${encodeURIComponent(projectId)}/summary`,
    )
      .then((summary) => {
        if (!active) return;
        setProjectName((summary?.project?.name || '').trim());
        const domains = Array.isArray(summary?.domains)
          ? summary.domains
              .map((item) => ({
                id: String(item?.id || '').trim(),
                url: String(item?.url || '').trim(),
              }))
              .filter((item) => item.id && item.url)
          : [];
        setProjectDomains(domains);
      })
      .catch(() => {
        if (!active) return;
        setProjectName('');
        setProjectDomains([]);
      });
    return () => {
      active = false;
    };
  }, [projectId]);

  // Стили таблиц
  const tableWrapperClass =
    'bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden animate-in fade-in';
  const tableHeaderClass =
    'text-left text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 bg-white dark:bg-[#0f1117] border-b border-slate-200 dark:border-slate-800/80';
  const tableRowClass =
    'border-b border-slate-100 dark:border-slate-800/40 hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors';

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <Link href="/projects" className="hover:text-indigo-600 transition-colors">
                Проекты
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <Link
                href={projectId ? `/projects/${projectId}` : '/projects'}
                className="hover:text-indigo-600 transition-colors">
                {projectName || 'Проект'}
              </Link>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <span className="text-slate-900 dark:text-slate-200 font-medium">LLM Usage</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              Расход токенов (LLM)
            </h1>
          </div>

          <button
            onClick={load}
            disabled={loading}
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-500 transition-all shadow-sm active:scale-95 disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            Обновить
          </button>
        </div>
      </header>

      {/* CONTENT AREA */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-6">
          {/* МЕТРИКИ (Статистика) */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <StatCard
              label="Запросы к API"
              value={stats?.total_requests ?? 0}
              icon={<Activity className="w-5 h-5 text-indigo-500" />}
            />
            <StatCard
              label="Потрачено токенов"
              value={(stats?.total_tokens ?? 0).toLocaleString('ru-RU')}
              icon={<Database className="w-5 h-5 text-amber-500" />}
            />
            <StatCard
              label="Оценочная стоимость"
              value={`$${(stats?.total_cost_usd ?? 0).toFixed(4)}`}
              icon={<DollarSign className="w-5 h-5 text-emerald-500" />}
            />
          </div>

          {/* ТАБЛИЦА ЛОГОВ */}
          <div className={tableWrapperClass}>
            {/* ФИЛЬТРЫ */}
            <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20 flex flex-col lg:flex-row lg:items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="w-8 h-8 rounded-lg bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                  <Zap className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                    Журнал запросов
                  </h3>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                    Лог каждого обращения к LLM.
                  </p>
                </div>
              </div>

              {/* Ряд фильтров */}
              <div className="flex flex-wrap items-center gap-2">
                <FilterDateInput
                  value={from}
                  onChange={(v) => {
                    setPage(1);
                    setFrom(v);
                  }}
                  placeholder="С"
                />
                <FilterDateInput
                  value={to}
                  onChange={(v) => {
                    setPage(1);
                    setTo(v);
                  }}
                  placeholder="По"
                />
                <FilterSelect
                  value={model}
                  options={modelOptions}
                  onChange={(v) => {
                    setPage(1);
                    setModel(v);
                  }}
                />
                <FilterSelect
                  value={operation}
                  options={operationOptions}
                  onChange={(v) => {
                    setPage(1);
                    setOperation(v);
                  }}
                />
                <FilterSelect
                  value={status}
                  options={statusOptions}
                  onChange={(v) => {
                    setPage(1);
                    setStatus(v);
                  }}
                />
                <FilterSearchInput
                  value={domainQuery}
                  onChange={(v) => {
                    setPage(1);
                    setDomainQuery(v);
                  }}
                  placeholder="Поиск по домену"
                />
              </div>
            </div>

            {error && <div className="p-4 text-sm text-red-500">{error}</div>}

            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead>
                  <tr className={tableHeaderClass}>
                    <th className="py-3 px-5">Время</th>
                    <th className="py-3 px-5">Операция</th>
                    <th className="py-3 px-5">Модель</th>
                    <th className="py-3 px-5">Домен</th>
                    <th className="py-3 px-5">Статус</th>
                    <th className="py-3 px-5 text-right">Токены / $</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800/40 bg-white dark:bg-[#0f1117]">
                  {visibleItems.map((item) => (
                    <tr key={item.id} className={tableRowClass}>
                      <td className="py-3 px-5 text-slate-500 text-xs">
                        {new Date(item.created_at).toLocaleString('ru-RU', {
                          day: '2-digit',
                          month: 'short',
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit',
                        })}
                      </td>
                      <td className="py-3 px-5">
                        <div className="font-medium text-slate-900 dark:text-slate-200">
                          {item.operation.split('/').pop()}
                        </div>
                        <div className="text-[10px] text-slate-400 font-mono mt-0.5">
                          {item.stage || '—'}
                        </div>
                      </td>
                      <td className="py-3 px-5 text-slate-600 dark:text-slate-300 text-xs font-mono">
                        {item.model}
                      </td>
                      <td className="py-3 px-5 font-medium text-indigo-600 dark:text-indigo-400">
                        {item.domain_id ? domainLookup.get(item.domain_id) || '—' : '—'}
                      </td>
                      <td className="py-3 px-5">
                        <span
                          className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold ${item.status === 'error' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'}`}>
                          {item.status}
                        </span>
                      </td>
                      <td className="py-3 px-5 text-right">
                        <div className="font-mono text-slate-700 dark:text-slate-300">
                          {item.total_tokens?.toLocaleString('ru-RU') || 'n/a'}
                        </div>
                        <div className="text-[10px] text-emerald-600 dark:text-emerald-500 mt-0.5 font-mono">
                          <UsageCostValue value={item.estimated_cost_usd} />
                        </div>
                      </td>
                    </tr>
                  ))}
                  {visibleItems.length === 0 && !loading && (
                    <tr>
                      <td colSpan={6} className="py-12 text-center text-slate-500">
                        Запросов по выбранным фильтрам не найдено.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* ПАГИНАЦИЯ */}
            <div className="p-4 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] flex items-center justify-between">
              <span className="text-xs text-slate-500">Показано {visibleItems.length} записей</span>
              <div className="flex items-center gap-3 text-sm">
                <button
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page <= 1 || loading}
                  className="px-3 py-1.5 rounded-lg border border-slate-300 bg-white text-slate-600 hover:bg-slate-50 disabled:opacity-50 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                  Назад
                </button>
                <span className="font-medium text-slate-700 dark:text-slate-300">
                  {page} / {totalPages}
                </span>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page >= totalPages || loading}
                  className="px-3 py-1.5 rounded-lg border border-slate-300 bg-white text-slate-600 hover:bg-slate-50 disabled:opacity-50 dark:bg-slate-900 dark:border-slate-700 dark:text-slate-300 dark:hover:bg-slate-800 transition-colors">
                  Вперёд
                </button>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}

function StatCard({
  label,
  value,
  icon,
}: {
  label: string;
  value: string | number;
  icon: React.ReactNode;
}) {
  return (
    <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-800 rounded-2xl p-5 shadow-sm flex items-center gap-4">
      <div className="w-12 h-12 rounded-full bg-slate-50 dark:bg-[#0a1020] border border-slate-100 dark:border-slate-800/80 flex items-center justify-center flex-shrink-0">
        {icon}
      </div>
      <div>
        <div className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1">
          {label}
        </div>
        <div className="text-2xl font-black text-slate-900 dark:text-white tracking-tight">
          {value}
        </div>
      </div>
    </div>
  );
}
