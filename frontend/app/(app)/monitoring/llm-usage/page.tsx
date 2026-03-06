'use client';

import Link from 'next/link';
import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Activity,
  RefreshCw,
  DollarSign,
  Database,
  Zap,
  ChevronRight,
  CreditCard,
  Search,
  Key,
  Globe,
  FolderGit2,
} from 'lucide-react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
  Legend,
} from 'recharts'; // <--- ДОБАВЛЕНО ДЛЯ ГРАФИКОВ

import {
  listAdminLLMPricing,
  listAdminLLMUsageEvents,
  listAdminLLMUsageStats,
} from '@/lib/llmUsageApi';
import { authFetch } from '@/lib/http';
import { useAuthGuard } from '@/lib/useAuth';
import type {
  LLMPricingDTO,
  LLMUsageEventDTO,
  LLMUsageFilters,
  LLMUsageStatsDTO,
} from '@/types/llmUsage';
import { getTotalPages } from '@/features/queue-monitoring/services/primitives';
import { canRun } from '@/features/queue-monitoring/services/actionGuards';
import { UsageCostValue } from '@/features/llm-usage/components/UsageCostValue';
import { UsageTokenSourceBadge } from '@/features/llm-usage/components/UsageTokenSourceBadge';

import { FilterDateInput } from '@/features/queue-monitoring/components/FilterDateInput';
import { FilterSelect } from '@/features/queue-monitoring/components/FilterSelect';
import { FilterSearchInput } from '@/features/queue-monitoring/components/FilterSearchInput';

const DEFAULT_LIMIT = 50;
const CHART_COLORS = ['#6366f1', '#8b5cf6', '#d946ef', '#f43f5e', '#f97316'];

export default function LLMUsageMonitoringPage() {
  const { me, loading: authLoading } = useAuthGuard();
  const [events, setEvents] = useState<LLMUsageEventDTO[]>([]);
  const [total, setTotal] = useState(0);
  const [stats, setStats] = useState<LLMUsageStatsDTO | null>(null);
  const [pricing, setPricing] = useState<LLMPricingDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [allProjects, setAllProjects] = useState<any[]>([]);
  const [projectLookup, setProjectLookup] = useState<Map<string, string>>(new Map());
  const [domainLookup, setDomainLookup] = useState<Map<string, string>>(new Map());

  const [page, setPage] = useState(1);
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const [model, setModel] = useState('all');
  const [operation, setOperation] = useState('all');
  const [status, setStatus] = useState('all');
  const [userEmail, setUserEmail] = useState('');

  const [projectQuery, setProjectQuery] = useState('');
  const [domainQuery, setDomainQuery] = useState('');

  const isAdmin = (me?.role || '').toLowerCase() === 'admin';
  const totalPages = getTotalPages(total, DEFAULT_LIMIT);
  const refreshGuard = canRun({ busy: loading });

  useEffect(() => {
    if (!isAdmin) return;
    authFetch<any[]>('/api/projects')
      .then((res) => {
        if (Array.isArray(res)) {
          setAllProjects(res);
          const map = new Map<string, string>();
          res.forEach((p) => map.set(p.id, p.name));
          setProjectLookup(map);
        }
      })
      .catch(() => {});
  }, [isAdmin]);

  const resolvedProjectId = useMemo(() => {
    const q = projectQuery.trim().toLowerCase();
    if (!q) return '';
    const p = allProjects.find((p) => p.name.toLowerCase() === q);
    return p ? p.id : projectQuery.trim();
  }, [projectQuery, allProjects]);

  const resolvedDomainId = useMemo(() => {
    const q = domainQuery.trim().toLowerCase();
    if (!q) return '';
    for (const [id, url] of domainLookup.entries()) {
      if (url.toLowerCase() === q) return id;
    }
    return domainQuery.trim();
  }, [domainQuery, domainLookup]);

  const modelOptions = useMemo(() => {
    const values = new Set<string>([
      'gemini-2.5-flash',
      'gemini-2.5-pro',
      'gemini-2.5-flash-image',
    ]);
    events.forEach((item) => {
      if (item.model?.trim()) values.add(item.model.trim());
    });
    return [
      { value: 'all', label: 'Все модели' },
      ...Array.from(values)
        .sort()
        .map((v) => ({ value: v, label: v })),
    ];
  }, [events]);

  const operationOptions = useMemo(() => {
    const values = new Set<string>([
      'generation_step/competitor_analysis',
      'generation_step/content_generation',
      'editor_ai_suggest',
      'editor_ai_create_page',
    ]);
    events.forEach((item) => {
      if (item.operation?.trim()) values.add(item.operation.trim());
    });
    return [
      { value: 'all', label: 'Все операции' },
      ...Array.from(values)
        .sort()
        .map((v) => ({ value: v, label: v.split('/').pop() || v })),
    ];
  }, [events]);

  const statusOptions = [
    { value: 'all', label: 'Все статусы' },
    { value: 'success', label: 'Успех' },
    { value: 'error', label: 'Ошибка' },
  ];

  const filters = useMemo<LLMUsageFilters>(() => {
    const value: LLMUsageFilters = { page, limit: DEFAULT_LIMIT };
    if (from.trim()) value.from = from.trim();
    if (to.trim()) value.to = to.trim();
    if (model !== 'all') value.model = model.trim();
    if (operation !== 'all') value.operation = operation.trim();
    if (status !== 'all') value.status = status.trim();
    if (userEmail.trim()) value.userEmail = userEmail.trim();
    if (resolvedProjectId) value.projectId = resolvedProjectId;
    if (resolvedDomainId) value.domainId = resolvedDomainId;
    return value;
  }, [resolvedDomainId, from, model, operation, page, resolvedProjectId, status, to, userEmail]);

  const load = useCallback(async () => {
    if (!isAdmin) return;
    setLoading(true);
    setError(null);
    try {
      const [eventsRes, statsRes, pricingRes] = await Promise.all([
        listAdminLLMUsageEvents(filters),
        listAdminLLMUsageStats(filters),
        listAdminLLMPricing(),
      ]);

      const evs = Array.isArray(eventsRes.items) ? eventsRes.items : [];
      setEvents(evs);
      setTotal(eventsRes.total || 0);
      setStats(statsRes);
      setPricing(Array.isArray(pricingRes) ? pricingRes : []);

      const dIds = Array.from(new Set(evs.map((e) => e.domain_id).filter(Boolean))) as string[];
      if (dIds.length > 0) {
        const domainsRes = await authFetch<any[]>(`/api/domains?ids=${dIds.join(',')}`).catch(
          () => [],
        );
        setDomainLookup((prev) => {
          const next = new Map(prev);
          if (Array.isArray(domainsRes)) domainsRes.forEach((d) => next.set(d.id, d.url));
          return next;
        });
      }
    } catch (err: any) {
      setError(err?.message || 'Не удалось загрузить данные');
    } finally {
      setLoading(false);
    }
  }, [filters, isAdmin]);

  useEffect(() => {
    if (authLoading || !isAdmin) return;
    load();
  }, [authLoading, isAdmin, load]);

  if (authLoading)
    return <div className="p-10 text-center text-sm text-slate-500">Загрузка...</div>;
  if (!isAdmin)
    return <div className="p-10 text-center text-red-500 font-bold">Доступ запрещен</div>;

  const tableWrapperClass =
    'bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in duration-300';
  const tableHeaderClass =
    'text-left text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-[#0a1020] border-b border-slate-200 dark:border-slate-700/60';
  const tableRowClass =
    'border-b border-slate-100 dark:border-slate-800/40 hover:bg-slate-50/50 dark:hover:bg-white/[0.02] transition-colors';

  // --- ДАННЫЕ ДЛЯ ГРАФИКОВ ---
  // Форматируем дату для гистограммы (чтобы было коротко "02 Мар")
  const chartDataByDay = (stats?.by_day || []).map((d) => {
    const date = new Date(d.key);
    return {
      name: Number.isNaN(date.getTime())
        ? d.key
        : date.toLocaleDateString('ru-RU', { day: 'numeric', month: 'short' }),
      cost: Number(d.cost_usd.toFixed(2)),
      requests: d.requests,
    };
  });

  const chartDataByModel = (stats?.by_model || []).map((m, idx) => ({
    name: m.key,
    value: Number(m.cost_usd.toFixed(2)),
  }));

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm text-slate-500 dark:text-slate-400 mb-1">
              <span className="text-slate-900 dark:text-slate-200 font-medium">Мониторинг</span>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <span>Глобальный Биллинг</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              Расход токенов (LLM Usage)
            </h1>
          </div>
          <button
            onClick={load}
            disabled={refreshGuard.disabled}
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-500 transition-all shadow-sm active:scale-95 disabled:opacity-50">
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} /> Обновить
          </button>
        </div>
      </header>

      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-6">
          {/* ГЛАВНЫЕ МЕТРИКИ */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <StatCard
              label="Всего запросов"
              value={stats?.total_requests ?? 0}
              icon={<Activity className="w-5 h-5 text-indigo-500" />}
            />
            <StatCard
              label="Потрачено токенов"
              value={(stats?.total_tokens ?? 0).toLocaleString('ru-RU')}
              icon={<Database className="w-5 h-5 text-amber-500" />}
            />
            <StatCard
              label="Общая стоимость"
              value={`$${(stats?.total_cost_usd ?? 0).toFixed(2)}`}
              icon={<DollarSign className="w-5 h-5 text-emerald-500" />}
            />
          </div>

          {/* ГРАФИКИ */}
          <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-6">
            {/* Гистограмма расходов по дням */}
            <div className={`${tableWrapperClass} p-5 flex flex-col`}>
              <h3 className="font-bold text-slate-900 dark:text-white mb-4">
                Расходы по дням (USD)
              </h3>
              <div className="flex-1 min-h-[220px]">
                {chartDataByDay.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart
                      data={chartDataByDay}
                      margin={{ top: 0, right: 0, left: -20, bottom: 0 }}>
                      <XAxis
                        dataKey="name"
                        axisLine={false}
                        tickLine={false}
                        tick={{ fontSize: 12, fill: '#888' }}
                        dy={10}
                      />
                      <YAxis
                        axisLine={false}
                        tickLine={false}
                        tick={{ fontSize: 12, fill: '#888' }}
                        tickFormatter={(val: number | string) => `$${val}`}
                      />

                      <Tooltip
                        cursor={{ fill: 'rgba(0,0,0,0.05)' }}
                        contentStyle={{
                          borderRadius: '12px',
                          border: 'none',
                          boxShadow: '0 10px 25px -5px rgba(0, 0, 0, 0.1)',
                        }}
                        formatter={(value: any) => [`$${value}`, 'Стоимость']}
                      />
                      <Bar dataKey="cost" fill="#6366f1" radius={[4, 4, 0, 0]} maxBarSize={40} />
                    </BarChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="h-full flex items-center justify-center text-sm text-slate-400">
                    Нет данных для графика
                  </div>
                )}
              </div>
            </div>

            {/* Круговая диаграмма по моделям */}
            <div className={`${tableWrapperClass} p-5 flex flex-col`}>
              <h3 className="font-bold text-slate-900 dark:text-white mb-4">Траты по моделям</h3>
              <div className="flex-1 min-h-[220px]">
                {chartDataByModel.length > 0 ? (
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie
                        data={chartDataByModel}
                        innerRadius={60}
                        outerRadius={80}
                        paddingAngle={5}
                        dataKey="value">
                        {chartDataByModel.map((entry, index) => (
                          <Cell
                            key={`cell-${index}`}
                            fill={CHART_COLORS[index % CHART_COLORS.length]}
                          />
                        ))}
                      </Pie>
                      <Tooltip
                        contentStyle={{
                          borderRadius: '12px',
                          border: 'none',
                          boxShadow: '0 10px 25px -5px rgba(0, 0, 0, 0.1)',
                        }}
                        formatter={(value: any) => [`$${value}`, 'Потрачено']}
                      />
                      <Legend
                        verticalAlign="bottom"
                        height={36}
                        iconType="circle"
                        wrapperStyle={{ fontSize: '11px' }}
                      />
                    </PieChart>
                  </ResponsiveContainer>
                ) : (
                  <div className="h-full flex items-center justify-center text-sm text-slate-400">
                    Нет данных для графика
                  </div>
                )}
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-[1fr_300px] gap-6">
            {/* ТАБЛИЦА ЛОГОВ И ФИЛЬТРЫ */}
            <div className={`${tableWrapperClass} flex flex-col`}>
              {/* ФИЛЬТРЫ */}
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                <div className="flex items-center gap-3 mb-4">
                  <div className="w-8 h-8 rounded-lg bg-indigo-50 dark:bg-indigo-900/30 flex items-center justify-center text-indigo-600 dark:text-indigo-400">
                    <Zap className="w-4 h-4" />
                  </div>
                  <div>
                    <h3 className="text-base font-bold text-slate-900 dark:text-white">
                      Логи операций
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      История запросов по всей платформе.
                    </p>
                  </div>
                </div>

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

                  <div className="w-full h-px bg-slate-200 dark:bg-slate-800 my-1 hidden lg:block"></div>

                  <FilterSearchInput
                    value={userEmail}
                    onChange={(v) => {
                      setPage(1);
                      setUserEmail(v);
                    }}
                    placeholder="Email пользователя"
                  />

                  <FilterSearchInput
                    value={projectQuery}
                    onChange={(v) => {
                      setPage(1);
                      setProjectQuery(v);
                    }}
                    placeholder="Название проекта"
                    list="usage-projects-list"
                  />
                  <datalist id="usage-projects-list">
                    {allProjects.map((p) => (
                      <option key={p.id} value={p.name} />
                    ))}
                  </datalist>

                  <FilterSearchInput
                    value={domainQuery}
                    onChange={(v) => {
                      setPage(1);
                      setDomainQuery(v);
                    }}
                    placeholder="URL домена"
                    list="usage-domains-list"
                  />
                  <datalist id="usage-domains-list">
                    {Array.from(domainLookup.values()).map((url) => (
                      <option key={url} value={url} />
                    ))}
                  </datalist>
                </div>
              </div>

              {error && <div className="p-4 text-sm text-red-500">{error}</div>}

              <div className="overflow-x-auto flex-1">
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className={tableHeaderClass}>
                      <th className="py-3 px-5">Дата и Время</th>
                      <th className="py-3 px-5">Пользователь / Сущности</th>
                      <th className="py-3 px-5">Операция</th>
                      <th className="py-3 px-5">Статус</th>
                      <th className="py-3 px-5 text-right">Токены / Cost</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-800/40 bg-white dark:bg-[#0f1117]">
                    {events.map((item) => {
                      const pName = item.project_id
                        ? projectLookup.get(item.project_id) || item.project_id
                        : null;
                      const dUrl = item.domain_id
                        ? domainLookup.get(item.domain_id) || item.domain_id
                        : null;

                      return (
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
                            <div className="font-medium text-slate-900 dark:text-slate-200 mb-1.5">
                              {item.requester_email}
                            </div>
                            <div className="flex flex-col gap-1">
                              {pName && (
                                <Link
                                  href={`/projects/${item.project_id}`}
                                  className="inline-flex items-center gap-1.5 text-[11px] text-slate-500 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors w-fit">
                                  <FolderGit2 className="w-3 h-3" />{' '}
                                  <span className="truncate max-w-[200px]">{pName}</span>
                                </Link>
                              )}
                              {dUrl && (
                                <Link
                                  href={`/domains/${item.domain_id}`}
                                  className="inline-flex items-center gap-1.5 text-[11px] text-slate-500 hover:text-indigo-600 dark:hover:text-indigo-400 transition-colors w-fit">
                                  <Globe className="w-3 h-3" />{' '}
                                  <span className="truncate max-w-[200px]">{dUrl}</span>
                                </Link>
                              )}
                            </div>
                          </td>
                          <td className="py-3 px-5">
                            <div className="font-medium text-slate-900 dark:text-slate-200">
                              {item.operation.split('/').pop()}
                            </div>
                            <div className="text-[10px] text-slate-500 font-mono mt-1 px-1.5 py-0.5 bg-slate-100 dark:bg-slate-800 rounded inline-block">
                              {item.model}
                            </div>
                          </td>
                          <td className="py-3 px-5">
                            <span
                              className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-bold ${item.status === 'error' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400 border border-red-200 dark:border-red-800/50' : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800/50'}`}>
                              {item.status}
                            </span>
                          </td>
                          <td className="py-3 px-5 text-right">
                            <div className="font-mono text-slate-700 dark:text-slate-300 font-semibold">
                              {item.total_tokens?.toLocaleString('ru-RU') || '0'}
                            </div>
                            <div className="text-[10px] text-emerald-600 dark:text-emerald-500 mt-0.5 font-mono">
                              <UsageCostValue
                                value={item.estimated_cost_usd}
                                naTooltip="Тариф не задан"
                              />
                            </div>
                          </td>
                        </tr>
                      );
                    })}
                    {events.length === 0 && !loading && (
                      <tr>
                        <td colSpan={5} className="py-12 text-center text-slate-500">
                          Запросов по выбранным фильтрам не найдено.
                        </td>
                      </tr>
                    )}
                  </tbody>
                </table>
              </div>

              {/* ПАГИНАЦИЯ */}
              <div className="p-4 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] flex items-center justify-between">
                <span className="text-xs text-slate-500">Показано {events.length} записей</span>
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

            {/* ПРАВАЯ ПАНЕЛЬ: СПРАВОЧНИК ТАРИФОВ */}
            <div className="flex flex-col gap-4">
              <div className="bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm p-5 animate-in fade-in duration-500">
                <div className="flex items-center gap-2 mb-4">
                  <CreditCard className="w-4 h-4 text-slate-400" />
                  <h3 className="font-bold text-slate-900 dark:text-white">
                    Тарифы (за 1M токенов)
                  </h3>
                </div>
                <div className="space-y-3">
                  {pricing.map((item) => (
                    <div
                      key={item.id}
                      className="p-3 rounded-xl border border-slate-100 bg-slate-50 dark:border-slate-800/60 dark:bg-[#0a1020]">
                      <div className="font-mono text-xs font-bold text-indigo-600 dark:text-indigo-400 mb-2">
                        {item.model}
                      </div>
                      <div className="flex justify-between text-xs text-slate-600 dark:text-slate-400 mb-1">
                        <span>Input:</span>
                        <span className="font-mono">${item.input_usd_per_million}</span>
                      </div>
                      <div className="flex justify-between text-xs text-slate-600 dark:text-slate-400">
                        <span>Output:</span>
                        <span className="font-mono">${item.output_usd_per_million}</span>
                      </div>
                    </div>
                  ))}
                  {pricing.length === 0 && (
                    <div className="text-xs text-slate-500 text-center py-4">
                      Тарифы не загружены
                    </div>
                  )}
                </div>
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
    <div className="bg-white dark:bg-[#0f1117] border border-slate-200 dark:border-slate-700/60 rounded-2xl p-5 shadow-sm flex items-center gap-4 animate-in zoom-in-95 duration-300">
      <div className="w-12 h-12 rounded-full bg-slate-50 dark:bg-[#0a1020] border border-slate-100 dark:border-slate-800/80 flex items-center justify-center flex-shrink-0">
        {icon}
      </div>
      <div>
        <div className="text-[10px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1">
          {label}
        </div>
        <div className="text-2xl font-black text-slate-900 dark:text-white tracking-tight">
          {value}
        </div>
      </div>
    </div>
  );
}
