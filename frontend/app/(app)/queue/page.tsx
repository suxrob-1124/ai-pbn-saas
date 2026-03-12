'use client';

import { Suspense, useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import {
  FiClock,
  FiPlay,
  FiCheck,
  FiAlertTriangle,
  FiRefreshCw,
  FiPause,
  FiRotateCw,
  FiTrash2,
  FiX,
} from 'react-icons/fi';
import { ChevronRight, ListFilter, Database, AlertCircle, Layers, Link2, Zap } from 'lucide-react';

import { authFetch } from '@/lib/http';
import { getGlobalUnifiedQueue, type UnifiedQueueItem } from '@/lib/unifiedQueueApi';
import { useAuthGuard } from '@/lib/useAuth';
import { deleteLinkTask, listLinkTasks, retryLinkTask } from '@/lib/linkTasksApi';
import { showToast } from '@/lib/toastStore';
import type { LinkTaskDTO } from '@/types/linkTasks';
import { Badge } from '@/components/Badge';

// ВОССТАНОВЛЕНЫ ВСЕ ИМПОРТЫ СТАТУСОВ
import {
  getLinkTaskStatusMeta,
  isLinkTaskInProgress,
  normalizeLinkTaskStatus,
  canRetryLinkTask,
  type LinkTaskCanonicalStatus,
} from '@/lib/linkTaskStatus';
import { useActionLocks } from '@/features/editor-v3/hooks/useActionLocks';
import { FlowStateBanner } from '@/features/queue-monitoring/components/FlowStateBanner';
import { FilterSearchInput } from '@/features/queue-monitoring/components/FilterSearchInput';
import { useFlowState } from '@/features/queue-monitoring/hooks/useFlowState';
import { PaginationControls } from '@/features/queue-monitoring/components/PaginationControls';
import { TableState } from '@/features/queue-monitoring/components/TableState';
import { canDelete, canRetry, canRun } from '@/features/queue-monitoring/services/actionGuards';
import { matchesSearch } from '@/features/queue-monitoring/services/filters';
import { queueMonitoringRu, toDiagnosticsText } from '@/features/queue-monitoring/services/i18n-ru';
import {
  hasNextPageByPageSize,
  resolveQueueTab,
} from '@/features/queue-monitoring/services/primitives';
import {
  GLOBAL_GENERATION_FILTER_KEYS,
  LINK_QUEUE_FILTER_KEYS,
  getGenerationFilterLabel,
  getGenerationStatusMeta,
  getLinkQueueStatusLabel,
  normalizeGenerationStatusForFilter,
  normalizeLinkQueueStatus,
  type GlobalGenerationFilterKey,
  type StatusIcon,
} from '@/features/queue-monitoring/services/statusMeta';

type Generation = {
  id: string;
  domain_id: string;
  domain_url?: string;
  status: string;
  progress: number;
  created_at?: string;
  updated_at?: string;
};

type ProjectDTO = { id: string; name?: string | null };

export default function QueuePage() {
  return (
    <Suspense
      fallback={
        <div className="flex h-full items-center justify-center text-sm text-slate-500">
          <FiRefreshCw className="w-5 h-5 animate-spin mr-2" /> Загрузка очереди...
        </div>
      }>
      <QueuePageContent />
    </Suspense>
  );
}

function QueuePageContent() {
  const { me } = useAuthGuard();
  const [items, setItems] = useState<Generation[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errorDiagnostics, setErrorDiagnostics] = useState<string | null>(null);
  const [filter, setFilter] = useState<GlobalGenerationFilterKey>('all');
  const [search, setSearch] = useState('');
  const [genPage, setGenPage] = useState(1);
  const genPageSize = 20;

  const [genActiveOnly, setGenActiveOnly] = useState(true);
  const [linkTasks, setLinkTasks] = useState<LinkTaskDTO[]>([]);
  const [linkLoading, setLinkLoading] = useState(false);
  const [linkError, setLinkError] = useState<string | null>(null);
  const [linkErrorDiagnostics, setLinkErrorDiagnostics] = useState<string | null>(null);
  const [linkFilter, setLinkFilter] = useState<'all' | LinkTaskCanonicalStatus>('all');
  const [linkSearch, setLinkSearch] = useState('');
  const [linkPage, setLinkPage] = useState(1);
  const linkPageSize = 20;
  const [linkActiveOnly, setLinkActiveOnly] = useState(true);
  const [linkDomains, setLinkDomains] = useState<Record<string, string>>({});

  // Unified queue state
  const [unifiedItems, setUnifiedItems] = useState<UnifiedQueueItem[]>([]);
  const [unifiedLoading, setUnifiedLoading] = useState(false);
  const [unifiedError, setUnifiedError] = useState<string | null>(null);
  const [unifiedTypeFilter, setUnifiedTypeFilter] = useState<'all' | 'generation' | 'link'>('all');
  const [unifiedStatusFilter, setUnifiedStatusFilter] = useState<'all' | 'pending' | 'processing' | 'completed' | 'failed'>('all');
  const [unifiedSearch, setUnifiedSearch] = useState('');
  const [unifiedPage, setUnifiedPage] = useState(1);
  const unifiedPageSize = 50;

  const searchParams = useSearchParams();
  const activeTab = resolveQueueTab(searchParams.get('tab'));
  const { isLocked, lockReason, runLocked } = useActionLocks();
  const refreshFlow = useFlowState();
  const linkFlow = useFlowState();

  const refreshLockKey = 'queue:global:refresh';
  const linkRetryLockKey = (taskId: string) => `queue:global:link:${taskId}:retry`;
  const linkDeleteLockKey = (taskId: string) => `queue:global:link:${taskId}:delete`;

  const loadUnified = useCallback(async () => {
    setUnifiedLoading(true);
    setUnifiedError(null);
    try {
      const data = await getGlobalUnifiedQueue({
        type: unifiedTypeFilter,
        status: unifiedStatusFilter,
        limit: unifiedPageSize + 1,
        page: unifiedPage,
      });
      setUnifiedItems(Array.isArray(data) ? data : []);
    } catch (e: any) {
      setUnifiedError(e?.message || 'Не удалось загрузить очередь');
    } finally {
      setUnifiedLoading(false);
    }
  }, [unifiedTypeFilter, unifiedStatusFilter, unifiedPage]);

  useEffect(() => {
    if (activeTab === 'unified') void loadUnified();
  }, [activeTab, loadUnified]);

  useEffect(() => {
    setUnifiedPage(1);
  }, [unifiedTypeFilter, unifiedStatusFilter, unifiedSearch]);

  const visibleUnifiedItems = useMemo(() => {
    const s = unifiedSearch.trim().toLowerCase();
    let list = unifiedItems.slice(0, unifiedPageSize);
    if (s) list = list.filter((i) => (i.domain_url || '').toLowerCase().includes(s));
    return list;
  }, [unifiedItems, unifiedSearch]);

  const unifiedHasNext = unifiedItems.length > unifiedPageSize;

  useEffect(() => {
    if (activeTab !== 'unified') return;
    const hasActive = unifiedItems.some((i) => i.status === 'pending' || i.status === 'processing');
    if (!hasActive) return;
    const id = window.setInterval(() => void loadUnified(), 5000);
    return () => window.clearInterval(id);
  }, [activeTab, unifiedItems, loadUnified]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setErrorDiagnostics(null);
    try {
      const params = new URLSearchParams();
      params.set('limit', genActiveOnly ? '200' : String(genPageSize));
      params.set('page', genActiveOnly ? '1' : String(genPage));
      params.set('lite', '1');
      if (genActiveOnly) params.set('active', '1');
      if (search.trim()) {
        params.set('search', search.trim());
      }
      const res = await authFetch<Generation[]>(`/api/generations?${params.toString()}`);
      setItems(Array.isArray(res) ? res : []);
    } catch (err: any) {
      setError('Не удалось загрузить очередь');
      setErrorDiagnostics(toDiagnosticsText(err) || null);
    } finally {
      setLoading(false);
    }
  }, [genPage, genPageSize, search, genActiveOnly]);

  const loadLinks = useCallback(async () => {
    setLinkLoading(true);
    setLinkError(null);
    setLinkErrorDiagnostics(null);
    try {
      const params = {
        limit: linkPageSize,
        page: linkPage,
        status: linkFilter !== 'all' ? linkFilter : undefined,
        search: linkSearch.trim() ? linkSearch.trim() : undefined,
      };
      let list: LinkTaskDTO[] = [];
      try {
        const res = await listLinkTasks(params);
        list = Array.isArray(res) ? res : [];
      } catch (err: any) {
        const msg = String(err?.message || '');
        const isAdminOnly = msg.toLowerCase().includes('admin only');
        const isAdmin = (me?.role || '').toLowerCase() === 'admin';
        if (!isAdmin && isAdminOnly) {
          const projects = await authFetch<ProjectDTO[]>('/api/projects');
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
              page: 1,
            });
            if (Array.isArray(res)) {
              allTasks.push(...res);
            }
          }
          allTasks.sort(
            (a, b) => new Date(a.scheduled_for).getTime() - new Date(b.scheduled_for).getTime(),
          );
          const offset = (linkPage - 1) * linkPageSize;
          const slice = allTasks.slice(offset, offset + linkPageSize + 1);
          list = slice.slice(0, linkPageSize);
        } else {
          throw err;
        }
      }
      setLinkTasks(list);
      const ids = Array.from(new Set(list.map((task) => task.domain_id).filter(Boolean))).slice(
        0,
        200,
      );
      if (ids.length === 0) {
        setLinkDomains({});
      } else {
        try {
          const params = new URLSearchParams();
          params.set('ids', ids.join(','));
          const domainList = await authFetch<{ id: string; url: string }[]>(
            `/api/domains?${params.toString()}`,
          );
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
      setLinkError('Не удалось загрузить задачи ссылок');
      setLinkErrorDiagnostics(toDiagnosticsText(err) || null);
    } finally {
      setLinkLoading(false);
    }
  }, [linkFilter, linkPage, linkPageSize, linkSearch, me]);

  const handleRefresh = useCallback(async () => {
    refreshFlow.validating('Проверяем доступность обновления очереди');
    await runLocked(
      refreshLockKey,
      async () => {
        refreshFlow.sending('Обновляем генерации и задачи ссылок');
        await Promise.all([load(), loadLinks()]);
        refreshFlow.done('Данные очереди обновлены');
      },
      queueMonitoringRu.lockReasons.refreshInFlight,
    );
  }, [load, loadLinks, refreshFlow, runLocked]);

  useEffect(() => {
    load();
  }, [load]);
  useEffect(() => {
    loadLinks();
  }, [loadLinks]);

  useEffect(() => {
    const hasActiveGenerations = items.some((i) =>
      ['pending', 'processing', 'pause_requested', 'cancelling'].includes(i.status),
    );
    const hasActiveLinks = linkTasks.some((t) => isLinkTaskInProgress(t.status));
    if (!hasActiveGenerations && !hasActiveLinks) return;

    const timer = window.setInterval(() => {
      if (hasActiveGenerations) load();
      if (hasActiveLinks) loadLinks();
    }, 5000);
    return () => window.clearInterval(timer);
  }, [items, linkTasks, load, loadLinks]);

  const filtered = useMemo(() => {
    return items.filter((i) => {
      const normalizedStatus = normalizeGenerationStatusForFilter(i.status);
      if (filter !== 'all' && normalizedStatus !== filter) return false;
      return matchesSearch(i.domain_url, search);
    });
  }, [filter, items, search]);

  const counts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const i of items) {
      const key = normalizeGenerationStatusForFilter(i.status);
      if (key) c[key] = (c[key] || 0) + 1;
    }
    return c;
  }, [items]);

  const LINK_ACTIVE_STATUSES = new Set(['pending', 'searching', 'removing']);

  const filteredLinks = useMemo(() => {
    return linkTasks.filter((task) => {
      const normalizedStatus =
        normalizeLinkQueueStatus(task.status) || (task.status || '').trim().toLowerCase();
      if (linkActiveOnly && !LINK_ACTIVE_STATUSES.has(normalizedStatus)) return false;
      if (linkFilter !== 'all' && normalizedStatus !== linkFilter) return false;
      return matchesSearch(linkDomains[task.domain_id], linkSearch);
    });
  }, [linkFilter, linkTasks, linkSearch, linkDomains, linkActiveOnly]);

  const linkCounts = useMemo(() => {
    const c: Record<string, number> = {};
    for (const t of linkTasks) {
      const normalizedStatus =
        normalizeLinkQueueStatus(t.status) || (t.status || '').trim().toLowerCase();
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
  const genIndexBase = (genPage - 1) * genPageSize;
  const linkIndexBase = (linkPage - 1) * linkPageSize;
  const refreshGuard = canRun({
    busy: loading || linkLoading || isLocked(refreshLockKey),
    busyReason: lockReason(refreshLockKey),
  });

  const handleLinkRetry = async (task: LinkTaskDTO) => {
    const domainLabel = linkDomains[task.domain_id] || 'домен';
    if (!confirm(`Повторить задачу ссылки для домена ${domainLabel}?`)) return;
    const lockKey = linkRetryLockKey(task.id);
    linkFlow.validating('Проверяем возможность повтора задачи ссылки');
    await runLocked(
      lockKey,
      async () => {
        linkFlow.sending(`Повторяем задачу ссылки для ${domainLabel}`);
        setLinkLoading(true);
        setLinkError(null);
        setLinkErrorDiagnostics(null);
        try {
          await retryLinkTask(task.id);
          showToast({ type: 'success', title: 'Повтор поставлен в очередь', message: domainLabel });
          await loadLinks();
          linkFlow.done('Задача успешно повторно поставлена в очередь');
        } catch (err: any) {
          const userMessage = 'Не удалось повторить задачу ссылки';
          setLinkError(userMessage);
          setLinkErrorDiagnostics(toDiagnosticsText(err) || null);
          linkFlow.fail(userMessage, err);
          showToast({ type: 'error', title: 'Ошибка повтора', message: userMessage });
        } finally {
          setLinkLoading(false);
        }
      },
      queueMonitoringRu.lockReasons.retryInFlight,
    );
  };

  const handleLinkDelete = async (task: LinkTaskDTO) => {
    const domainLabel = linkDomains[task.domain_id] || 'домен';
    if (!confirm(`Удалить задачу ссылки для домена ${domainLabel}?`)) return;
    const lockKey = linkDeleteLockKey(task.id);
    linkFlow.validating('Проверяем возможность удаления задачи ссылки');
    await runLocked(
      lockKey,
      async () => {
        linkFlow.sending(`Удаляем задачу ссылки для ${domainLabel}`);
        setLinkLoading(true);
        setLinkError(null);
        setLinkErrorDiagnostics(null);
        try {
          await deleteLinkTask(task.id);
          showToast({ type: 'success', title: 'Задача ссылки удалена', message: domainLabel });
          await loadLinks();
          linkFlow.done('Задача ссылки удалена');
        } catch (err: any) {
          const userMessage = 'Не удалось удалить задачу ссылки';
          setLinkError(userMessage);
          setLinkErrorDiagnostics(toDiagnosticsText(err) || null);
          linkFlow.fail(userMessage, err);
          showToast({ type: 'error', title: 'Ошибка удаления', message: userMessage });
        } finally {
          setLinkLoading(false);
        }
      },
      queueMonitoringRu.lockReasons.deleteInFlight,
    );
  };

  const tableWrapperClass =
    'bg-white dark:bg-[#0f1523] border border-slate-200 dark:border-slate-700/60 rounded-2xl shadow-sm overflow-hidden animate-in fade-in duration-300';
  const tableHeaderClass =
    'text-left text-[11px] font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 bg-slate-50 dark:bg-[#0a1020] border-b border-slate-200 dark:border-slate-700/60';
  const tableRowClass =
    'border-b border-slate-100 dark:border-slate-800/40 hover:bg-slate-50/50 dark:hover:bg-white/[0.02] transition-colors';

  const showRefreshBanner =
    refreshFlow.flow.status !== 'idle' && refreshFlow.flow.status !== 'done';
  const showLinkBanner = linkFlow.flow.status !== 'idle' && linkFlow.flow.status !== 'done';

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      {/* HEADER: Хлебные крошки и табы */}
      <header className="px-6 py-5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-[#0b0f19] sticky top-0 z-10">
        <div className="max-w-7xl mx-auto flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <div className="flex items-center text-sm font-medium text-slate-500 dark:text-slate-400 mb-1">
              <span>Система</span>
              <ChevronRight className="w-4 h-4 mx-1 opacity-50" />
              <span className="text-slate-900 dark:text-slate-200">Глобальная очередь</span>
            </div>
            <h1 className="text-2xl font-bold tracking-tight text-slate-900 dark:text-white flex items-center gap-3">
              Центр очередей
            </h1>
          </div>

          <button
            onClick={handleRefresh}
            disabled={refreshGuard.disabled}
            title={refreshGuard.reason}
            className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl bg-indigo-600 text-white text-sm font-semibold hover:bg-indigo-500 transition-all shadow-sm active:scale-95 disabled:opacity-50">
            <FiRefreshCw className={`w-4 h-4 ${loading || linkLoading ? 'animate-spin' : ''}`} />
            Обновить всё
          </button>
        </div>

        {/* TABS (Домены / Ссылки) */}
        <div className="max-w-7xl mx-auto mt-6 flex items-center gap-6 border-b border-slate-200 dark:border-slate-800">
          <TabLink
            href={{ pathname: '/queue', query: { tab: 'domains' } } as LinkHref}
            label={genActiveOnly ? `Генерация (${items.length} активных)` : `Генерация (${items.length})`}
            icon={<ListFilter />}
            active={activeTab === 'domains'}
          />
          <TabLink
            href={{ pathname: '/queue', query: { tab: 'links' } } as LinkHref}
            label={linkActiveOnly ? `Ссылки (${filteredLinks.length} активных)` : `Ссылки (${linkTasks.length})`}
            icon={<Database />}
            active={activeTab === 'links'}
          />
          <TabLink
            href={{ pathname: '/queue', query: { tab: 'unified' } } as LinkHref}
            label="Единая очередь"
            icon={<Layers />}
            active={activeTab === 'unified'}
          />
        </div>
      </header>

      {/* CONTENT AREA */}
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-6">
          {/* Плавающие баннеры (Только во время работы) */}
          {(showRefreshBanner || showLinkBanner) && (
            <div className="fixed bottom-6 right-6 z-50 flex flex-col gap-2">
              {showRefreshBanner && (
                <FlowStateBanner
                  title={queueMonitoringRu.flowTitles.refresh}
                  flow={refreshFlow.flow}
                />
              )}
              {showLinkBanner && (
                <FlowStateBanner title={queueMonitoringRu.flowTitles.links} flow={linkFlow.flow} />
              )}
            </div>
          )}

          {error && (
            <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 flex flex-col gap-2">
              <div className="flex items-center gap-2 font-bold">
                <AlertCircle className="w-5 h-5" /> {error}
              </div>
              {errorDiagnostics && (
                <code className="text-xs opacity-80 block p-2 bg-black/5 dark:bg-black/30 rounded-lg">
                  {errorDiagnostics}
                </code>
              )}
            </div>
          )}

          {/* ========================================= */}
          {/* ВКЛАДКА: ГЕНЕРАЦИЯ (Домены)               */}
          {/* ========================================= */}
          {activeTab === 'domains' && (
            <div className={`${tableWrapperClass} flex flex-col animate-in fade-in duration-300`}>
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                <div className="flex flex-col xl:flex-row xl:items-center justify-between gap-4">
                  <div>
                    <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                      Глобальная очередь генерации
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Фильтры применяются к текущей странице, поиск работает по всем загруженным
                      записям.
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center gap-3">
                    <button
                      onClick={() => setGenActiveOnly((v) => !v)}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all ${genActiveOnly ? 'bg-emerald-50 border-emerald-200 text-emerald-700 dark:bg-emerald-500/20 dark:border-emerald-500/40 dark:text-emerald-300' : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-400'}`}>
                      {genActiveOnly ? 'Только активные' : 'Показать все'}
                    </button>
                    <FilterSearchInput
                      value={search}
                      onChange={setSearch}
                      placeholder="Поиск по URL"
                    />
                  </div>
                </div>

                {/* Фильтры по статусу (Кнопки) */}
                <div className="flex flex-wrap items-center gap-2 mt-5">
                  {GLOBAL_GENERATION_FILTER_KEYS.map((val) => (
                    <FilterButton
                      key={val}
                      label={getGenerationFilterLabel(val)}
                      active={filter === val}
                      onClick={() => setFilter(val)}
                      count={val === 'all' ? items.length : counts[val] || 0}
                    />
                  ))}
                </div>
              </div>

              <TableState
                loading={loading}
                empty={!loading && filtered.length === 0}
                emptyText={genActiveOnly ? 'Очередь пуста — нет активных генераций.' : 'Запусков пока нет.'}
              />

              {!loading && filtered.length > 0 && (
                <div className="overflow-x-auto flex-1">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className={tableHeaderClass}>
                        <th className="py-3 px-5 w-10 text-center">№</th>
                        <th className="py-3 px-5">Сайт</th>
                        <th className="py-3 px-5">Статус</th>
                        <th className="py-3 px-5">Прогресс</th>
                        <th className="py-3 px-5">Обновлено</th>
                        <th className="py-3 px-5 text-right">Действия</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800/40 bg-white dark:bg-[#0f1117]">
                      {filtered.map((g, idx) => (
                        <tr key={g.id} className={tableRowClass}>
                          <td className="py-3 px-5 text-xs text-slate-400 text-center font-mono">
                            {genIndexBase + idx + 1}
                          </td>
                          <td className="py-3 px-5 font-semibold">
                            {g.domain_url ? (
                              <Link
                                href={`/domains/${g.domain_id}`}
                                className="text-indigo-600 dark:text-indigo-400 hover:underline">
                                {g.domain_url}
                              </Link>
                            ) : (
                              <span className="text-slate-500">—</span>
                            )}
                          </td>
                          <td className="py-3 px-5">
                            <StatusBadge status={g.status} />
                          </td>
                          <td className="py-3 px-5 font-mono text-slate-600 dark:text-slate-300">
                            {g.progress}%
                          </td>
                          <td className="py-3 px-5 text-slate-500 text-xs">
                            {g.updated_at
                              ? new Date(g.updated_at).toLocaleString('ru-RU', {
                                  day: '2-digit',
                                  month: 'short',
                                  hour: '2-digit',
                                  minute: '2-digit',
                                  second: '2-digit',
                                })
                              : '—'}
                          </td>
                          <td className="py-3 px-5 text-right">
                            <Link
                              href={`/queue/${g.id}`}
                              className="inline-flex items-center px-3 py-1.5 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg dark:bg-slate-800 dark:text-slate-300 transition-colors">
                              Детали лога
                            </Link>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              {filtered.length > 0 && (
                <div className="p-4 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] flex items-center justify-between">
                  <span className="text-xs text-slate-500">Показано {filtered.length} записей</span>
                  <PaginationControls
                    page={genPage}
                    hasNext={genHasNext}
                    onPrev={() => setGenPage((p) => Math.max(1, p - 1))}
                    onNext={() => setGenPage((p) => p + 1)}
                  />
                </div>
              )}
            </div>
          )}

          {/* ========================================= */}
          {/* ВКЛАДКА: ССЫЛКИ                           */}
          {/* ========================================= */}
          {activeTab === 'links' && (
            <div className={`${tableWrapperClass} flex flex-col animate-in fade-in duration-300`}>
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                <div className="flex flex-col xl:flex-row xl:items-center justify-between gap-4">
                  <div>
                    <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                      Глобальная очередь ссылок
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Задачи на вставку и удаление ссылок (Link Flow) по всем проектам.
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center gap-3">
                    <button
                      onClick={() => setLinkActiveOnly((v) => !v)}
                      className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all ${linkActiveOnly ? 'bg-emerald-50 border-emerald-200 text-emerald-700 dark:bg-emerald-500/20 dark:border-emerald-500/40 dark:text-emerald-300' : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-400'}`}>
                      {linkActiveOnly ? 'Активные задачи' : 'Все задачи'}
                    </button>
                    <FilterSearchInput
                      value={linkSearch}
                      onChange={setLinkSearch}
                      placeholder="Поиск по URL"
                    />
                  </div>
                </div>

                {/* Фильтры по статусу (Кнопки) */}
                <div className="flex flex-wrap items-center gap-2 mt-5">
                  {LINK_QUEUE_FILTER_KEYS.map((val) => (
                    <FilterButton
                      key={val}
                      label={getLinkQueueStatusLabel(val)}
                      active={linkFilter === val}
                      onClick={() => setLinkFilter(val)}
                      count={val === 'all' ? linkTasks.length : linkCounts[val] || 0}
                    />
                  ))}
                </div>
              </div>

              {linkError && (
                <div className="p-4 bg-red-50 text-red-600 rounded-xl text-sm border border-red-100 dark:bg-red-950/30 dark:border-red-900/50 flex flex-col gap-2">
                  <div className="flex items-center gap-2 font-bold">
                    <AlertCircle className="w-5 h-5" /> {linkError}
                  </div>
                  {linkErrorDiagnostics && (
                    <code className="text-xs opacity-80 block p-2 bg-black/5 dark:bg-black/30 rounded-lg">
                      {linkErrorDiagnostics}
                    </code>
                  )}
                </div>
              )}

              <TableState
                loading={linkLoading}
                error={linkError}
                empty={!linkLoading && !linkError && filteredLinks.length === 0}
                emptyText={linkActiveOnly ? 'Нет активных задач ссылок.' : 'Задач ссылок нет.'}
              />

              {!linkLoading && !linkError && filteredLinks.length > 0 && (
                <div className="overflow-x-auto flex-1">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className={tableHeaderClass}>
                        <th className="py-3 px-5 w-10 text-center">№</th>
                        <th className="py-3 px-5">Сайт / Действие</th>
                        <th className="py-3 px-5">Тайминг</th>
                        <th className="py-3 px-5">Статус</th>
                        <th className="py-3 px-5">Событие (Лог)</th>
                        <th className="py-3 px-5 text-right">Действия</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800/40 bg-white dark:bg-[#0f1117]">
                      {filteredLinks.map((task, idx) => {
                        const actionLabel =
                          (task.action || 'insert') === 'remove' ? 'Удаление' : 'Вставка';
                        const lastLog = task.log_lines?.length
                          ? task.log_lines[task.log_lines.length - 1]
                          : '';
                        const eventText = task.error_message || lastLog || '—';
                        const domainLabel = linkDomains[task.domain_id] || 'Домен';
                        const normalizedStatus =
                          normalizeLinkTaskStatus(task.status) || task.status;
                        const canRetryByStatus = canRetryLinkTask(task.status);
                        const retryGuard = canRetry({
                          busy: linkLoading || isLocked(linkRetryLockKey(task.id)),
                          allowed: canRetryByStatus,
                        });
                        const deleteGuard = canDelete({
                          busy: linkLoading || isLocked(linkDeleteLockKey(task.id)),
                          status: task.status,
                        });
                        const isError = normalizedStatus === 'failed';

                        return (
                          <tr key={task.id} className={tableRowClass}>
                            <td className="py-3 px-5 text-xs text-slate-400 text-center font-mono">
                              {linkIndexBase + idx + 1}
                            </td>
                            <td className="py-3 px-5">
                              <Link
                                href={`/domains/${task.domain_id}`}
                                className="font-semibold text-indigo-600 dark:text-indigo-400 hover:underline block mb-1">
                                {domainLabel}
                              </Link>
                              <Badge
                                label={actionLabel}
                                tone={task.action === 'remove' ? 'red' : 'blue'}
                                className="text-[10px] px-1.5 py-0"
                              />
                            </td>
                            <td className="py-3 px-5 text-slate-500 text-xs space-y-1">
                              <div>
                                {new Date(task.scheduled_for).toLocaleString('ru-RU', {
                                  day: '2-digit',
                                  month: 'short',
                                  hour: '2-digit',
                                  minute: '2-digit',
                                  second: '2-digit',
                                })}
                              </div>
                              <div className="opacity-70">Попыток: {task.attempts}</div>
                            </td>
                            <td className="py-3 px-5">
                              <LinkTaskStatusBadge status={normalizedStatus} />
                            </td>
                            <td
                              className={`py-3 px-5 max-w-[250px] truncate ${isError ? 'text-red-500 font-medium' : 'text-slate-500'}`}
                              title={eventText}>
                              {eventText}
                            </td>
                            <td className="py-3 px-5 text-right">
                              <div className="flex items-center justify-end gap-2">
                                <Link
                                  href={`/links/${task.id}`}
                                  className="px-3 py-1.5 text-xs font-semibold text-slate-700 bg-slate-100 hover:bg-slate-200 rounded-lg dark:bg-slate-800 dark:text-slate-300 transition-colors">
                                  Детали
                                </Link>
                                {retryGuard.enabled && (
                                  <button
                                    onClick={() => handleLinkRetry(task)}
                                    disabled={retryGuard.disabled}
                                    className="p-2 text-amber-500 hover:bg-amber-50 dark:hover:bg-amber-900/20 rounded-md transition-colors"
                                    title="Повторить">
                                    <FiRotateCw className="w-4 h-4" />
                                  </button>
                                )}
                                <button
                                  onClick={() => handleLinkDelete(task)}
                                  disabled={deleteGuard.disabled}
                                  className="p-2 text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-md transition-colors"
                                  title="Удалить">
                                  <FiTrash2 className="w-4 h-4" />
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
                <div className="p-4 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] flex items-center justify-between">
                  <span className="text-xs text-slate-500">
                    Показано {filteredLinks.length} записей
                  </span>
                  <PaginationControls
                    page={linkPage}
                    hasNext={linkHasNext}
                    onPrev={() => setLinkPage((p) => Math.max(1, p - 1))}
                    onNext={() => setLinkPage((p) => p + 1)}
                  />
                </div>
              )}
            </div>
          )}

          {/* ========================================= */}
          {/* ВКЛАДКА: ЕДИНАЯ ОЧЕРЕДЬ                  */}
          {/* ========================================= */}
          {activeTab === 'unified' && (
            <div className={`${tableWrapperClass} flex flex-col animate-in fade-in duration-300`}>
              <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/20">
                <div className="flex flex-col xl:flex-row xl:items-center justify-between gap-4">
                  <div>
                    <h3 className="text-base font-bold text-slate-900 dark:text-white flex items-center gap-2">
                      Единая глобальная очередь
                    </h3>
                    <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                      Генерации и задачи ссылок всех проектов в одной таблице.
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <select
                      value={unifiedTypeFilter}
                      onChange={(e) => setUnifiedTypeFilter(e.target.value as typeof unifiedTypeFilter)}
                      className="text-xs px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#060d18] text-slate-700 dark:text-slate-300 outline-none focus:ring-1 focus:ring-indigo-500">
                      <option value="all">Все типы</option>
                      <option value="generation">Генерация</option>
                      <option value="link">Ссылки</option>
                    </select>
                    <select
                      value={unifiedStatusFilter}
                      onChange={(e) => setUnifiedStatusFilter(e.target.value as typeof unifiedStatusFilter)}
                      className="text-xs px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-[#060d18] text-slate-700 dark:text-slate-300 outline-none focus:ring-1 focus:ring-indigo-500">
                      <option value="all">Все статусы</option>
                      <option value="pending">Ожидание</option>
                      <option value="processing">В работе</option>
                      <option value="completed">Завершено</option>
                      <option value="failed">Ошибка</option>
                    </select>
                    <FilterSearchInput value={unifiedSearch} onChange={setUnifiedSearch} placeholder="Поиск по URL" />
                    <button
                      onClick={loadUnified}
                      disabled={unifiedLoading}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold rounded-lg bg-indigo-50 text-indigo-700 hover:bg-indigo-100 dark:bg-indigo-900/30 dark:text-indigo-300 disabled:opacity-50 transition-colors">
                      <FiRefreshCw className={`w-3.5 h-3.5 ${unifiedLoading ? 'animate-spin' : ''}`} />
                      Обновить
                    </button>
                  </div>
                </div>
              </div>

              {unifiedError && (
                <div className="p-4 text-red-600 text-sm border-b border-red-100 dark:border-red-900/30 bg-red-50/50 dark:bg-red-950/20">
                  {unifiedError}
                </div>
              )}

              <TableState
                loading={unifiedLoading}
                error={unifiedError}
                empty={!unifiedLoading && !unifiedError && visibleUnifiedItems.length === 0}
                emptyText="Очередь пуста."
              />

              {!unifiedLoading && !unifiedError && visibleUnifiedItems.length > 0 && (
                <div className="overflow-x-auto flex-1">
                  <table className="min-w-full text-sm">
                    <thead>
                      <tr className={tableHeaderClass}>
                        <th className="py-3 px-5 w-8">Тип</th>
                        <th className="py-3 px-5">Домен</th>
                        <th className="py-3 px-5">Статус</th>
                        <th className="py-3 px-5">Запланировано</th>
                        <th className="py-3 px-5">Детали</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800/40 bg-white dark:bg-[#0f1117]">
                      {visibleUnifiedItems.map((item) => {
                        const isLink = item.type === 'link';
                        const statusMeta = UNIFIED_STATUS_LABELS[item.status] || { label: item.status, tone: 'slate' as const };
                        const detail = item.error_message || (isLink && item.anchor_text ? `Анкор: ${item.anchor_text}` : '');
                        return (
                          <tr key={item.id} className={tableRowClass}>
                            <td className="py-3 px-5">
                              {isLink ? (
                                <span title="Ссылка"><Link2 className="w-4 h-4 text-indigo-500" /></span>
                              ) : (
                                <span title="Генерация"><Zap className="w-4 h-4 text-emerald-500" /></span>
                              )}
                            </td>
                            <td className="py-3 px-5 font-semibold">
                              <Link
                                href={`/domains/${item.domain_id}`}
                                className="text-indigo-600 dark:text-indigo-400 hover:underline">
                                {item.domain_url || item.domain_id}
                              </Link>
                              <span className="ml-2 text-[10px] text-slate-400">{item.status_detail}</span>
                            </td>
                            <td className="py-3 px-5">
                              <Badge label={statusMeta.label} tone={statusMeta.tone} />
                            </td>
                            <td className="py-3 px-5 text-slate-500 text-xs">
                              {new Date(item.scheduled_for).toLocaleString('ru-RU', {
                                day: '2-digit', month: 'short',
                                hour: '2-digit', minute: '2-digit',
                              })}
                            </td>
                            <td className="py-3 px-5 text-xs max-w-[250px] truncate text-slate-500" title={detail}>
                              {detail || '—'}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
              {visibleUnifiedItems.length > 0 && (
                <div className="p-4 border-t border-slate-200 dark:border-slate-800/80 bg-slate-50/50 dark:bg-[#0a1020] flex items-center justify-between">
                  <span className="text-xs text-slate-500">Показано {visibleUnifiedItems.length} записей</span>
                  <PaginationControls
                    page={unifiedPage}
                    hasNext={unifiedHasNext}
                    onPrev={() => setUnifiedPage((p) => Math.max(1, p - 1))}
                    onNext={() => setUnifiedPage((p) => p + 1)}
                  />
                </div>
              )}
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

const UNIFIED_STATUS_LABELS: Record<string, { label: string; tone: 'amber' | 'blue' | 'green' | 'red' | 'slate' }> = {
  pending:    { label: 'Ожидание',  tone: 'amber' },
  processing: { label: 'В работе',  tone: 'blue'  },
  completed:  { label: 'Завершено', tone: 'green' },
  failed:     { label: 'Ошибка',    tone: 'red'   },
};

const tableRowClass =
  'border-b border-slate-100 dark:border-slate-800/40 hover:bg-slate-50/50 dark:hover:bg-white/[0.02] transition-colors';

// --- ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ---

function StatusBadge({ status }: { status: string }) {
  const meta = getGenerationStatusMeta(status);
  const icon = renderStatusIcon(meta.icon);
  return <Badge label={meta.label} tone={meta.tone} icon={icon} className="text-xs" />;
}

function LinkTaskStatusBadge({ status }: { status: string }) {
  const meta = getLinkTaskStatusMeta(status);
  const icon =
    meta.icon === 'refresh' ? (
      <FiRefreshCw className="w-3 h-3" />
    ) : meta.icon === 'check' ? (
      <FiCheck className="w-3 h-3" />
    ) : meta.icon === 'alert' ? (
      <FiAlertTriangle className="w-3 h-3" />
    ) : (
      <FiClock className="w-3 h-3" />
    );
  return <Badge label={meta.text} tone={meta.tone} icon={icon} className="text-xs" />;
}

function renderStatusIcon(icon: StatusIcon): ReactNode {
  if (icon === 'play') return <FiPlay className="w-3 h-3" />;
  if (icon === 'pause') return <FiPause className="w-3 h-3" />;
  if (icon === 'x') return <FiX className="w-3 h-3" />;
  if (icon === 'check') return <FiCheck className="w-3 h-3" />;
  if (icon === 'alert') return <FiAlertTriangle className="w-3 h-3" />;
  if (icon === 'refresh') return <FiRefreshCw className="w-3 h-3" />;
  return <FiClock className="w-3 h-3" />;
}

function FilterButton({
  label,
  active,
  onClick,
  count,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
  count: number;
}) {
  return (
    <button
      onClick={onClick}
      className={`inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all border ${
        active
          ? 'bg-indigo-50 border-indigo-200 text-indigo-700 dark:bg-indigo-500/20 dark:border-indigo-500/40 dark:text-indigo-300'
          : 'bg-white border-slate-200 text-slate-600 hover:bg-slate-50 dark:bg-[#060d18] dark:border-slate-700 dark:text-slate-400 dark:hover:bg-[#0a1020]'
      }`}>
      {label} {count > 0 && <span className="opacity-60 font-mono">({count})</span>}
    </button>
  );
}

type LinkHref = Parameters<typeof Link>[0]['href'];

function TabLink({
  href,
  label,
  icon,
  active,
}: {
  href: any;
  label: string;
  icon: ReactNode;
  active: boolean;
}) {
  return (
    <Link
      href={href}
      className={`flex items-center gap-2 pb-4 px-1 text-sm font-medium border-b-2 transition-all ${active ? 'border-indigo-600 text-indigo-600 dark:text-indigo-400 dark:border-indigo-400' : 'border-transparent text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'}`}>
      {icon && <span className="flex-shrink-0 opacity-80 [&>svg]:w-4 [&>svg]:h-4">{icon}</span>}
      {label}
    </Link>
  );
}
