'use client';

import { Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import {
  Activity,
  AlertCircle,
  AlertTriangle,
  Calendar,
  CheckCircle2,
  Loader2,
  Play,
  RefreshCw,
  XCircle,
} from 'lucide-react';
import { FailedChecksAlert } from '@/components/indexing/FailedChecksAlert';
import { IndexFiltersBar } from '@/components/indexing/IndexFiltersBar';
import { IndexTable, type IndexCheckSort } from '@/components/indexing/IndexTable';
import { useAuthGuard } from '@/lib/useAuth';
import { authFetch, invalidateAuthCache } from '@/lib/http';
import { useDebouncedValue } from '@/lib/useDebouncedValue';
import { showToast } from '@/lib/toastStore';
import { useActionLocks } from '@/features/editor-v3/hooks/useActionLocks';
import { useIndexCheckHistory } from '@/features/queue-monitoring/hooks/useIndexCheckHistory';
import { useIndexMonitoringScopeLabels } from '@/features/queue-monitoring/hooks/useIndexMonitoringScopeLabels';
import { FlowStateBanner } from '@/features/queue-monitoring/components/FlowStateBanner';
import { useFlowState } from '@/features/queue-monitoring/hooks/useFlowState';
import { PaginationControls } from '@/features/queue-monitoring/components/PaginationControls';
import { canRun } from '@/features/queue-monitoring/services/actionGuards';
import {
  DEFAULT_SORT_PARAM,
  formatDate,
  formatDateKey,
  formatDateTime,
  parseSortParam,
  parseStatusParam,
  sameSort,
  sameStatusList,
  sortToParam,
} from '@/features/queue-monitoring/services/indexingPageUtils';
import { queueMonitoringRu, toDiagnosticsText } from '@/features/queue-monitoring/services/i18n-ru';
import {
  readEnumParam,
  readPositiveIntParam,
  readStringParam,
  setOptionalNumberParam,
  setOptionalParam,
} from '@/features/queue-monitoring/services/queryParams';
import { getTotalPages, hasNextPageByTotal } from '@/features/queue-monitoring/services/primitives';
import { normalizeIndexCheckStatusList } from '@/features/queue-monitoring/services/statusMeta';
import {
  listAdmin,
  listAdminStats,
  listByDomain,
  listByProject,
  listDomainStats,
  listFailed,
  listProjectStats,
  getGlobalIndexCheckerControl,
  runAdminManual,
  runManual,
  runManualProject,
  setGlobalIndexCheckerControl,
} from '@/lib/indexChecksApi';
import type {
  IndexCheckCalendarDayDTO,
  IndexCheckDTO,
  IndexCheckStatus,
  IndexCheckStatsDTO,
  IndexChecksFilters,
  IndexChecksResponse,
} from '@/types/indexChecks';
import type { IndexFiltersValue } from '@/components/indexing/IndexFiltersBar';

const DEFAULT_LIMIT = 20;
const SEARCH_DEBOUNCE_MS = 400;
const INDEXED_FILTER_VALUES = ['all', 'true', 'false'] as const;

// ─── Page entry point ──────────────────────────────────────────────────────

export default function IndexingMonitoringPage() {
  return (
    <Suspense
      fallback={
        <div className="flex h-full items-center justify-center text-sm text-slate-500">
          <RefreshCw className="w-5 h-5 animate-spin mr-2" />
          Загрузка мониторинга...
        </div>
      }
    >
      <IndexingMonitoringContent />
    </Suspense>
  );
}

// ─── GitHub-style Heatmap ──────────────────────────────────────────────────

type HeatmapCell = {
  date: string;
  total: number;
  indexed: number;
  percent: number | null;
  isFuture: boolean;
};

type WeekColumn = { cells: HeatmapCell[]; monthLabel: string | null };

function buildHeatmapGrid(data: IndexCheckCalendarDayDTO[]): WeekColumn[] {
  const lookup = new Map(data.map((d) => [d.date, d]));
  const today = new Date();

  const start = new Date(today);
  start.setDate(start.getDate() - 364);
  const dow = start.getDay();
  start.setDate(start.getDate() + (dow === 0 ? -6 : 1 - dow));

  const weeks: WeekColumn[] = [];
  const cur = new Date(start);
  let prevMonth = -1;

  while (cur <= today) {
    const week: HeatmapCell[] = [];
    let monthLabel: string | null = null;

    for (let d = 0; d < 7; d++) {
      const month = cur.getMonth();
      if (d === 0 && month !== prevMonth) {
        monthLabel = cur.toLocaleDateString('ru-RU', { month: 'short' });
        prevMonth = month;
      }
      const dateStr = formatDateKey(cur);
      const day = lookup.get(dateStr);
      week.push({
        date: dateStr,
        total: day?.total ?? 0,
        indexed: day?.indexed_true ?? 0,
        percent: day && day.total > 0 ? Math.round((day.indexed_true / day.total) * 100) : null,
        isFuture: cur > today,
      });
      cur.setDate(cur.getDate() + 1);
    }
    weeks.push({ cells: week, monthLabel });
  }
  return weeks;
}

function getCellBg(cell: HeatmapCell): string {
  if (cell.isFuture || cell.total === 0) return 'bg-slate-100 dark:bg-slate-800/70';
  const p = cell.percent ?? 0;
  if (p >= 90) return 'bg-emerald-600 dark:bg-emerald-500';
  if (p >= 70) return 'bg-emerald-400';
  if (p >= 50) return 'bg-yellow-400';
  if (p >= 25) return 'bg-orange-400';
  return 'bg-red-400';
}

function IndexingHeatmap({
  data,
  loading,
  selectedDate,
  onDateClick,
}: {
  data: IndexCheckCalendarDayDTO[];
  loading: boolean;
  selectedDate?: string;
  onDateClick?: (date: string) => void;
}) {
  const weeks = useMemo(() => buildHeatmapGrid(data), [data]);
  const DAY_LABELS = ['Пн', '', 'Ср', '', 'Пт', '', ''];

  if (loading) {
    return (
      <div className="flex items-center justify-center py-10 gap-2 text-sm text-slate-400">
        <Loader2 className="w-4 h-4 animate-spin" />
        Загрузка данных за год...
      </div>
    );
  }

  if (data.length === 0) {
    return (
      <div className="py-10 text-center text-sm text-slate-400 dark:text-slate-500">
        Нет данных за последний год
      </div>
    );
  }

  return (
    <div className="w-full select-none">
      {/* Month labels — aligned above week columns */}
      <div className="flex gap-[3px] mb-0.5">
        <div className="w-5 flex-shrink-0 mr-1" />
        {weeks.map((w, wi) => (
          <div
            key={wi}
            className="flex-1 text-[9px] font-medium text-slate-400 dark:text-slate-500 leading-none overflow-hidden min-w-0"
          >
            {w.monthLabel ?? ''}
          </div>
        ))}
      </div>

      {/* Grid: day-of-week rows × week columns */}
      <div className="flex gap-[3px]">
        <div className="flex flex-col gap-[3px] mr-1 w-5 flex-shrink-0">
          {DAY_LABELS.map((label, i) => (
            <div
              key={i}
              className="h-3 text-[9px] text-slate-400 dark:text-slate-500 leading-none flex items-center justify-end pr-0.5"
            >
              {label}
            </div>
          ))}
        </div>
        {weeks.map((week, wi) => (
          <div key={wi} className="flex-1 flex flex-col gap-[3px] min-w-0">
            {week.cells.map((cell, di) => (
              <div
                key={di}
                className={[
                  'h-3 w-full rounded-[2px] transition-all',
                  getCellBg(cell),
                  cell.isFuture
                    ? 'opacity-0 cursor-default'
                    : 'cursor-pointer hover:ring-1 hover:ring-slate-400 dark:hover:ring-slate-400',
                  selectedDate === cell.date
                    ? 'ring-2 ring-indigo-500 dark:ring-indigo-400'
                    : '',
                ].join(' ')}
                title={
                  cell.total > 0
                    ? `${cell.date}: ${cell.indexed} из ${cell.total} в индексе (${cell.percent}%)`
                    : cell.isFuture
                    ? ''
                    : `${cell.date}: нет данных`
                }
                onClick={() => {
                  if (!cell.isFuture && onDateClick) onDateClick(cell.date);
                }}
              />
            ))}
          </div>
        ))}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-1.5 pl-7 mt-2">
        <span className="text-[10px] text-slate-400">Меньше</span>
        {[
          'bg-slate-100 dark:bg-slate-800/70',
          'bg-red-400',
          'bg-orange-400',
          'bg-yellow-400',
          'bg-emerald-400',
          'bg-emerald-600',
        ].map((c, i) => (
          <div key={i} className={`w-3 h-3 rounded-[2px] ${c}`} />
        ))}
        <span className="text-[10px] text-slate-400">Больше</span>
      </div>
    </div>
  );
}

// ─── Stat card ─────────────────────────────────────────────────────────────

type StatColor = 'green' | 'red' | 'amber' | 'slate';

function StatCard({
  label,
  value,
  sub,
  icon,
  color,
  pulse = false,
}: {
  label: string;
  value: string | number | null;
  sub?: string;
  icon: React.ReactNode;
  color: StatColor;
  pulse?: boolean;
}) {
  const colorMap: Record<StatColor, string> = {
    green: 'text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-900/30',
    red: 'text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-900/30',
    amber: 'text-amber-600 dark:text-amber-400 bg-amber-50 dark:bg-amber-900/30',
    slate: 'text-slate-500 dark:text-slate-400 bg-slate-100 dark:bg-slate-800/50',
  };
  return (
    <div className="bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-800 rounded-2xl p-5 shadow-sm">
      <div
        className={`inline-flex items-center justify-center w-9 h-9 rounded-xl mb-3 ${colorMap[color]} ${pulse ? 'animate-pulse' : ''}`}
      >
        {icon}
      </div>
      <div className="text-2xl font-bold tabular-nums text-slate-900 dark:text-white">
        {value === null ? (
          <div className="h-7 w-14 bg-slate-100 dark:bg-slate-800 rounded-lg animate-pulse" />
        ) : (
          value
        )}
      </div>
      <div className="text-xs font-medium text-slate-500 dark:text-slate-400 mt-0.5">{label}</div>
      {sub && <div className="text-[11px] text-slate-400 dark:text-slate-500 mt-0.5">{sub}</div>}
    </div>
  );
}

// ─── Main content ──────────────────────────────────────────────────────────

function IndexingMonitoringContent() {
  const { me, loading: authLoading } = useAuthGuard();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const projectId = (searchParams.get('projectId') || '').trim();
  const userRole = (me?.role || '').toLowerCase();
  const isAdmin = userRole === 'admin';
  const isManager = userRole === 'manager';

  const [checks, setChecks] = useState<IndexCheckDTO[]>([]);
  const [totalChecks, setTotalChecks] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [errorDiagnostics, setErrorDiagnostics] = useState<string | null>(null);

  const [failedChecks, setFailedChecks] = useState<IndexCheckDTO[]>([]);
  const [failedTotal, setFailedTotal] = useState(0);
  const [failedLoading, setFailedLoading] = useState(false);
  const [failedError, setFailedError] = useState<string | null>(null);

  const [stats, setStats] = useState<IndexCheckStatsDTO | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);

  const [heatmapData, setHeatmapData] = useState<IndexCheckCalendarDayDTO[]>([]);
  const [heatmapLoading, setHeatmapLoading] = useState(false);

  const [globalEnabled, setGlobalEnabled] = useState(true);
  const [globalToggleLoading, setGlobalToggleLoading] = useState(false);

  const [statusFilter, setStatusFilter] = useState<IndexCheckStatus[]>(() =>
    parseStatusParam(searchParams.get('status')),
  );
  const [indexedFilter, setIndexedFilter] = useState<'all' | 'true' | 'false'>(() =>
    readEnumParam(searchParams, 'isIndexed', INDEXED_FILTER_VALUES, 'all'),
  );
  const [domainFilter, setDomainFilter] = useState(() => readStringParam(searchParams, 'domainId'));
  const [dateFrom, setDateFrom] = useState(() => readStringParam(searchParams, 'from'));
  const [dateTo, setDateTo] = useState(() => readStringParam(searchParams, 'to'));
  const [search, setSearch] = useState(() => readStringParam(searchParams, 'search'));
  const debouncedSearch = useDebouncedValue(search, SEARCH_DEBOUNCE_MS);
  const [sort, setSort] = useState<IndexCheckSort>(() => parseSortParam(searchParams.get('sort')));
  const [page, setPage] = useState(() => readPositiveIntParam(searchParams, 'page', 1));
  const [limit, setLimit] = useState(() =>
    readPositiveIntParam(searchParams, 'limit', DEFAULT_LIMIT),
  );

  const { isLocked, lockReason, runLocked } = useActionLocks();
  const refreshFlow = useFlowState();
  const runFlow = useFlowState();

  const domainScope = domainFilter.trim();
  const { projectName, domainLabel } = useIndexMonitoringScopeLabels(projectId, domainScope);
  const { history, historyLoading, historyError, openHistory, toggleHistory } =
    useIndexCheckHistory({ projectId, domainScope, isAdmin });

  const needsProjectPicker = !authLoading && isManager && !projectId && !domainScope;
  const permissionDenied = !authLoading && !isAdmin && !isManager && !projectId && !domainScope;

  const [managerProjects, setManagerProjects] = useState<{ id: string; name: string }[]>([]);
  useEffect(() => {
    if (!isManager) return;
    authFetch<{ id: string; name: string }[]>('/api/projects')
      .then((res) => setManagerProjects(Array.isArray(res) ? res : []))
      .catch(() => {});
  }, [isManager]);
  const querySnapshot = searchParams.toString();

  const refreshLockKey = projectId
    ? `monitoring:indexing:project:${projectId}:refresh`
    : domainScope
      ? `monitoring:indexing:domain:${domainScope}:refresh`
      : 'monitoring:indexing:global:refresh';
  const manualRunLockKey = projectId
    ? `monitoring:indexing:project:${projectId}:run-manual`
    : domainScope
      ? `monitoring:indexing:domain:${domainScope}:run-manual`
      : 'monitoring:indexing:run-manual';
  const adminRunNowLockKey = (id: string) => `monitoring:indexing:admin:${id}:run-now`;

  const sortParam = useMemo(() => sortToParam(sort), [sort]);

  const filters = useMemo<IndexChecksFilters>(() => {
    const base: IndexChecksFilters = { limit, page };
    if (statusFilter.length > 0) base.status = statusFilter.join(',');
    if (debouncedSearch.trim()) base.search = debouncedSearch.trim();
    if (sortParam) base.sort = sortParam;
    if (indexedFilter !== 'all') base.isIndexed = indexedFilter === 'true';
    if (dateFrom) base.from = dateFrom;
    if (dateTo) base.to = dateTo;
    if (domainScope) base.domainId = domainScope;
    return base;
  }, [dateFrom, dateTo, debouncedSearch, domainScope, indexedFilter, limit, page, sortParam, statusFilter]);

  // ─── Loaders ───────────────────────────────────────────────────────

  const loadChecks = useCallback(async () => {
    if (permissionDenied) return;
    setLoading(true);
    setError(null);
    setErrorDiagnostics(null);
    try {
      let list: IndexChecksResponse | null = null;
      if (projectId) list = await listByProject(projectId, filters);
      else if (isAdmin) list = await listAdmin(filters);
      else if (domainScope) list = await listByDomain(domainScope, filters);
      setChecks(Array.isArray(list?.items) ? list!.items : []);
      setTotalChecks(typeof list?.total === 'number' ? list.total : 0);
    } catch (err: any) {
      setError('Не удалось загрузить проверки индексации');
      setErrorDiagnostics(toDiagnosticsText(err) || null);
    } finally {
      setLoading(false);
    }
  }, [domainScope, filters, isAdmin, permissionDenied, projectId]);

  const loadFailed = useCallback(async () => {
    if (!isAdmin || projectId) return;
    setFailedLoading(true);
    setFailedError(null);
    try {
      const since = new Date();
      since.setDate(since.getDate() - 7);
      const list = await listFailed({ limit: 5, domainId: domainScope || undefined, from: since });
      setFailedChecks(Array.isArray(list?.items) ? list.items : []);
      setFailedTotal(typeof list?.total === 'number' ? list.total : 0);
    } catch (err: any) {
      setFailedError(err?.message || 'Не удалось загрузить проблемные проверки');
    } finally {
      setFailedLoading(false);
    }
  }, [isAdmin, projectId, domainScope]);

  const loadStats = useCallback(async () => {
    if (permissionDenied) return;
    setStatsLoading(true);
    try {
      const today = new Date();
      const from30 = new Date(today);
      from30.setDate(from30.getDate() - 29);
      const f: IndexChecksFilters = { from: formatDateKey(from30), to: formatDateKey(today) };
      if (domainScope) f.domainId = domainScope;
      let data: IndexCheckStatsDTO | null = null;
      if (projectId) data = await listProjectStats(projectId, f);
      else if (isAdmin) data = await listAdminStats(f);
      else if (domainScope) data = await listDomainStats(domainScope, f);
      setStats(data);
    } catch {
      /* non-critical */
    } finally {
      setStatsLoading(false);
    }
  }, [domainScope, isAdmin, permissionDenied, projectId]);

  const loadHeatmap = useCallback(async () => {
    if (permissionDenied) return;
    setHeatmapLoading(true);
    try {
      const today = new Date();
      const from365 = new Date(today);
      from365.setDate(from365.getDate() - 364);
      const f: IndexChecksFilters = { from: formatDateKey(from365), to: formatDateKey(today) };
      if (domainScope) f.domainId = domainScope;
      let data: IndexCheckStatsDTO | null = null;
      if (projectId) data = await listProjectStats(projectId, f);
      else if (isAdmin) data = await listAdminStats(f);
      else if (domainScope) data = await listDomainStats(domainScope, f);
      setHeatmapData(Array.isArray(data?.daily) ? data!.daily : []);
    } catch {
      setHeatmapData([]);
    } finally {
      setHeatmapLoading(false);
    }
  }, [domainScope, isAdmin, permissionDenied, projectId]);

  useEffect(() => { loadChecks(); }, [loadChecks]);
  useEffect(() => { loadFailed(); }, [loadFailed]);
  useEffect(() => { loadStats(); }, [loadStats]);
  useEffect(() => { loadHeatmap(); }, [loadHeatmap]);

  useEffect(() => {
    if (!isAdmin) return;
    getGlobalIndexCheckerControl()
      .then((r) => setGlobalEnabled(r.enabled))
      .catch(() => {});
  }, [isAdmin]);

  // ─── URL sync ──────────────────────────────────────────────────────

  useEffect(() => {
    const sp = searchParams;
    const statusP = parseStatusParam(sp.get('status'));
    const searchP = readStringParam(sp, 'search');
    const fromP = readStringParam(sp, 'from');
    const toP = readStringParam(sp, 'to');
    const domainP = readStringParam(sp, 'domainId');
    const indexedP = readEnumParam(sp, 'isIndexed', INDEXED_FILTER_VALUES, 'all');
    const sortP = parseSortParam(sp.get('sort'));
    const nextPage = readPositiveIntParam(sp, 'page', 1);
    const nextLimit = readPositiveIntParam(sp, 'limit', DEFAULT_LIMIT);

    if (!sameStatusList(statusP, statusFilter)) setStatusFilter(statusP);
    if (indexedP !== indexedFilter) setIndexedFilter(indexedP);
    if (searchP !== search) setSearch(searchP);
    if (fromP !== dateFrom) setDateFrom(fromP);
    if (toP !== dateTo) setDateTo(toP);
    if (domainP !== domainFilter) setDomainFilter(domainP);
    if (!sameSort(sortP, sort)) setSort(sortP);
    if (nextPage !== page) setPage(nextPage);
    if (nextLimit !== limit) setLimit(nextLimit);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [querySnapshot]);

  useEffect(() => {
    const params = new URLSearchParams(searchParams.toString());
    if (statusFilter.length === 0) params.delete('status');
    else params.set('status', statusFilter.join(','));
    setOptionalParam(params, 'search', debouncedSearch, '');
    if (indexedFilter === 'all') params.delete('isIndexed');
    else params.set('isIndexed', indexedFilter);
    setOptionalParam(params, 'from', dateFrom, '');
    setOptionalParam(params, 'to', dateTo, '');
    setOptionalParam(params, 'domainId', domainScope, '');
    setOptionalNumberParam(params, 'page', page, 1);
    setOptionalNumberParam(params, 'limit', limit, DEFAULT_LIMIT);
    setOptionalParam(params, 'sort', sortParam, DEFAULT_SORT_PARAM);
    const next = params.toString();
    if (next !== searchParams.toString()) {
      router.replace((next ? `${pathname}?${next}` : pathname) as any, { scroll: false });
    }
  }, [dateFrom, dateTo, debouncedSearch, domainScope, indexedFilter, limit, page, pathname, router, searchParams, sortParam, statusFilter]);

  useEffect(() => {
    const total = getTotalPages(totalChecks, limit);
    if (page > total && total > 0) setPage(total);
  }, [page, totalChecks, limit]);

  // ─── Handlers ──────────────────────────────────────────────────────

  const appliedFilters = useMemo<IndexFiltersValue>(
    () => ({ statuses: statusFilter, from: dateFrom, to: dateTo, domainId: domainFilter, isIndexed: indexedFilter, search }),
    [dateFrom, dateTo, domainFilter, indexedFilter, search, statusFilter],
  );

  const defaultFilters = useMemo<IndexFiltersValue>(
    () => ({ statuses: [], from: '', to: '', domainId: '', isIndexed: 'all', search: '' }),
    [],
  );

  const applyFilters = (next: IndexFiltersValue) => {
    setStatusFilter(normalizeIndexCheckStatusList(next.statuses));
    setDateFrom(next.from);
    setDateTo(next.to);
    setDomainFilter(next.domainId);
    setIndexedFilter(next.isIndexed);
    setSearch(next.search);
    setPage(1);
  };

  const handleSearchChange = useCallback((v: string) => { setSearch(v); setPage(1); }, []);
  const handleSortChange = (next: IndexCheckSort) => { setSort(next); setPage(1); };

  const handleManualRun = async () => {
    await runLocked(
      manualRunLockKey,
      async () => {
        runFlow.sending('Запускаем проверку индексации');
        setError(null);
        setErrorDiagnostics(null);
        try {
          invalidateAuthCache('index-checks/stats');
          if (domainScope) {
            const r = await runManual(domainScope);
            if (r.run_now_enqueued === false) {
              showToast({ type: 'warning', title: 'Поставлено в очередь', message: r.run_now_error || '' });
            }
          } else if (projectId) {
            const r = await runManualProject(projectId);
            if ((r.enqueue_failed || 0) > 0) {
              showToast({ type: 'warning', title: 'Часть с ошибками', message: `Ошибок: ${r.enqueue_failed}` });
            }
          } else return;
          await Promise.all([loadChecks(), loadStats(), loadHeatmap()]);
          runFlow.done('Запуск отправлен');
        } catch (err: any) {
          const msg = 'Не удалось запустить проверку';
          setError(msg);
          setErrorDiagnostics(toDiagnosticsText(err) || null);
          runFlow.fail(msg, err);
        }
      },
      queueMonitoringRu.lockReasons.manualRunInFlight,
    );
  };

  const handleAdminRunNow = useCallback(
    async (domainId: string) => {
      if (!isAdmin) return;
      await runLocked(
        adminRunNowLockKey(domainId),
        async () => {
          runFlow.sending('Запуск по домену');
          try {
            invalidateAuthCache('index-checks/stats');
            const r = await runAdminManual(domainId);
            if (r.run_now_enqueued === false) {
              showToast({ type: 'warning', title: 'Поставлено в очередь', message: r.run_now_error || '' });
            }
            await Promise.all([loadChecks(), loadStats(), loadHeatmap(), loadFailed()]);
            runFlow.done('Отправлено');
          } catch (err: any) {
            runFlow.fail('Ошибка', err);
          }
        },
        queueMonitoringRu.lockReasons.manualRunInFlight,
      );
    },
    [isAdmin, loadChecks, loadFailed, loadHeatmap, loadStats, runFlow, runLocked],
  );

  const getAdminRunNowGuard = useCallback(
    (id: string) =>
      canRun({ busy: loading || isLocked(adminRunNowLockKey(id)), busyReason: lockReason(adminRunNowLockKey(id)) }),
    [isLocked, loading, lockReason],
  );

  const handleRefresh = async () => {
    await runLocked(
      refreshLockKey,
      async () => {
        refreshFlow.sending('Обновляем данные...');
        invalidateAuthCache('index-checks/stats');
        await Promise.all([loadChecks(), loadFailed(), loadStats(), loadHeatmap()]);
        refreshFlow.done('Данные обновлены');
      },
      queueMonitoringRu.lockReasons.refreshInFlight,
    );
  };

  const handleToggleGlobalIndexChecker = async () => {
    setGlobalToggleLoading(true);
    try {
      const next = !globalEnabled;
      await setGlobalIndexCheckerControl(next);
      setGlobalEnabled(next);
      showToast({
        type: 'success',
        title: next
          ? 'Автоматические проверки включены'
          : 'Автоматические проверки отключены глобально',
      });
    } catch (err: any) {
      showToast({ type: 'error', title: 'Ошибка', message: err?.message });
    } finally {
      setGlobalToggleLoading(false);
    }
  };

  // ─── Derived values ────────────────────────────────────────────────

  const scopeLabel = domainScope
    ? domainLabel || 'Доменные проверки'
    : projectId
      ? projectName || 'Проектные проверки'
      : 'Глобальный мониторинг';

  const indexedPct =
    stats
      ? stats.total_checks > 0
        ? `${Math.round(stats.percent_indexed)}%`
        : '—'
      : null;

  const notIndexed = stats ? stats.total_checks - stats.indexed_true : null;
  const hasNextPage = hasNextPageByTotal(page, limit, totalChecks);
  const totalPages = getTotalPages(totalChecks, limit);
  const refreshGuard = canRun({ busy: loading || isLocked(refreshLockKey), busyReason: lockReason(refreshLockKey) });
  const manualRunGuard = canRun({ busy: loading || isLocked(manualRunLockKey), busyReason: lockReason(manualRunLockKey) });

  // ─── Render ────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col h-full bg-slate-50 dark:bg-[#080b13]">
      <main className="flex-1 overflow-y-auto p-6">
        <div className="max-w-7xl mx-auto space-y-5">

      {/* ── Header ─────────────────────────────────────────────────── */}
      <div className="bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-800 rounded-2xl p-6 shadow-sm">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="text-xl font-bold text-slate-900 dark:text-white">
              Мониторинг индексации
            </h1>
            <p className="text-sm text-slate-500 dark:text-slate-400 mt-0.5">
              {scopeLabel} · Показывает, какие сайты попали в поисковик
            </p>
          </div>
          <div className="flex items-center gap-2 flex-shrink-0">
            <button
              onClick={handleRefresh}
              disabled={refreshGuard.disabled}
              title={refreshGuard.reason}
              type="button"
              className="inline-flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100 disabled:opacity-50 transition-colors"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
              Обновить
            </button>
            {(domainScope || projectId) && (
              <button
                onClick={handleManualRun}
                disabled={manualRunGuard.disabled}
                title={manualRunGuard.reason}
                type="button"
                className="inline-flex items-center gap-2 rounded-xl bg-indigo-600 px-3 py-2 text-xs font-semibold text-white hover:bg-indigo-500 disabled:opacity-50 transition-colors"
              >
                <Play className="w-3.5 h-3.5" />
                Запустить проверку
              </button>
            )}
          </div>
        </div>
        {isAdmin && (
          <div className="mt-4 pt-4 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <Activity className={`w-4 h-4 ${globalEnabled ? 'text-emerald-500' : 'text-red-400'}`} />
              <span className="text-sm font-medium text-slate-700 dark:text-slate-300">
                Автоматические проверки
              </span>
              {!globalEnabled && (
                <span className="text-[11px] font-semibold text-red-500 bg-red-50 dark:bg-red-500/10 px-2 py-0.5 rounded-full">
                  Отключено глобально
                </span>
              )}
            </div>
            <button
              onClick={handleToggleGlobalIndexChecker}
              disabled={globalToggleLoading}
              className={`relative inline-flex h-6 w-10 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500/20 disabled:opacity-50 ${
                globalEnabled ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-slate-600'
              }`}>
              <span className={`inline-block h-4 w-4 rounded-full bg-white shadow-sm transform transition-transform ${
                globalEnabled ? 'translate-x-5' : 'translate-x-1'
              }`} />
            </button>
          </div>
        )}
      </div>

      {/* ── Flow banners ───────────────────────────────────────────── */}
      <div className="grid gap-2 sm:grid-cols-2">
        <FlowStateBanner title={queueMonitoringRu.flowTitles.monitoring} flow={refreshFlow.flow} />
        <FlowStateBanner title={queueMonitoringRu.flowTitles.manualRun} flow={runFlow.flow} />
      </div>

      {/* ── Stat cards ─────────────────────────────────────────────── */}
      {!permissionDenied && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          <StatCard
            label="Проверено за 30 дней"
            value={statsLoading ? null : (stats?.total_checks ?? 0)}
            icon={<Activity className="w-4 h-4" />}
            color="slate"
          />
          <StatCard
            label="В поисковике"
            value={statsLoading ? null : (indexedPct ?? '—')}
            sub={stats ? `${stats.indexed_true} из ${stats.total_checks} сайтов` : undefined}
            icon={<CheckCircle2 className="w-4 h-4" />}
            color="green"
          />
          <StatCard
            label="Не в поисковике"
            value={statsLoading ? null : (notIndexed !== null ? notIndexed : '—')}
            sub={
              stats && stats.avg_attempts_to_success > 0
                ? `Ср. попыток до успеха: ${Math.round(stats.avg_attempts_to_success)}`
                : undefined
            }
            icon={<XCircle className="w-4 h-4" />}
            color={notIndexed && notIndexed > 0 ? 'red' : 'slate'}
          />
          <StatCard
            label="Требует внимания"
            value={statsLoading ? null : (stats?.failed_investigation ?? 0)}
            sub="Не удалось проверить"
            icon={<AlertTriangle className="w-4 h-4" />}
            color={stats && stats.failed_investigation > 0 ? 'amber' : 'slate'}
            pulse={stats != null && stats.failed_investigation > 0}
          />
        </div>
      )}

      {/* ── Heatmap ────────────────────────────────────────────────── */}
      {!permissionDenied && (
        <div className="bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-800 rounded-2xl p-6 shadow-sm">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 mb-5">
            <div className="flex items-center gap-2">
              <Calendar className="w-4 h-4 text-slate-400" />
              <h3 className="text-sm font-semibold text-slate-900 dark:text-white">
                Активность за год
              </h3>
            </div>
            <p className="text-xs text-slate-400 dark:text-slate-500">
              Зелёный = много сайтов в индексе, красный = мало, серый = нет данных.
              Нажмите на день для фильтрации.
            </p>
            {dateFrom && dateTo && dateFrom === dateTo && (
              <button
                onClick={() => applyFilters({ ...appliedFilters, from: '', to: '' })}
                className="ml-auto text-xs text-indigo-600 dark:text-indigo-400 hover:underline"
                type="button"
              >
                Сбросить: {dateFrom} ×
              </button>
            )}
          </div>
          <IndexingHeatmap
            data={heatmapData}
            loading={heatmapLoading}
            selectedDate={dateFrom && dateFrom === dateTo ? dateFrom : undefined}
            onDateClick={(date) =>
              applyFilters({
                ...appliedFilters,
                from: dateFrom === date && dateTo === date ? '' : date,
                to: dateFrom === date && dateTo === date ? '' : date,
              })
            }
          />
        </div>
      )}

      {/* ── Manager project picker ────────────────────────────────── */}
      {needsProjectPicker && (
        <div className="rounded-2xl border border-indigo-200 bg-indigo-50 px-5 py-5 dark:border-indigo-700/50 dark:bg-indigo-900/20">
          <p className="text-sm font-medium text-indigo-800 dark:text-indigo-200 mb-3">
            Выберите проект для просмотра данных индексации:
          </p>
          <div className="flex flex-wrap gap-2">
            {managerProjects.map((p) => (
              <button
                key={p.id}
                onClick={() => router.push(`${pathname}?projectId=${p.id}` as any)}
                className="px-4 py-2 text-sm font-medium rounded-xl bg-white dark:bg-slate-800 border border-indigo-200 dark:border-indigo-700/50 text-indigo-700 dark:text-indigo-300 hover:bg-indigo-100 dark:hover:bg-indigo-900/40 transition-colors shadow-sm">
                {p.name || p.id}
              </button>
            ))}
            {managerProjects.length === 0 && (
              <span className="text-sm text-slate-500">Нет доступных проектов.</span>
            )}
          </div>
        </div>
      )}

      {/* ── Errors / permission denied ─────────────────────────────── */}
      {permissionDenied && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-5 py-4 text-sm text-amber-700 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-200">
          Доступ запрещён. Глобальный список доступен только администраторам.
        </div>
      )}
      {error && !permissionDenied && (
        <div className="rounded-2xl border border-red-200 bg-red-50 px-5 py-4 text-sm text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200 flex items-start gap-2">
          <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          {error}
        </div>
      )}
      {error && !permissionDenied && errorDiagnostics && (
        <details className="rounded-2xl border border-red-200 bg-red-50 px-5 py-4 text-xs text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
          <summary className="cursor-pointer select-none">{queueMonitoringRu.diagnostics.title}</summary>
          <pre className="mt-2 whitespace-pre-wrap font-mono text-[11px]">{errorDiagnostics}</pre>
        </details>
      )}

      {/* ── Failed checks alert ─────────────────────────────────────── */}
      {isAdmin && !projectId && !permissionDenied && (
        <FailedChecksAlert
          checks={failedChecks}
          failedCount={failedTotal}
          loading={failedLoading}
          error={failedError}
          onRefresh={loadFailed}
          onViewDetails={() =>
            applyFilters({ ...appliedFilters, statuses: ['failed_investigation'] })
          }
        />
      )}

      {/* ── Filters + Table (unified card) ───────────────────────────── */}
      {!permissionDenied && (
        <div className="bg-white dark:bg-slate-900/80 border border-slate-200 dark:border-slate-800 rounded-2xl shadow-sm overflow-hidden">
          {/* Filters header */}
          <div className="p-5 border-b border-slate-100 dark:border-slate-800/60 bg-slate-50/50 dark:bg-slate-800/10">
            <IndexFiltersBar
              value={appliedFilters}
              defaultValue={defaultFilters}
              onApply={applyFilters}
              onReset={applyFilters}
              onRefresh={handleRefresh}
              onSearchChange={handleSearchChange}
              showDomain={isAdmin && !projectId}
              disabled={loading}
            />
          </div>
          {/* Table */}
          <IndexTable
            checks={checks}
            loading={loading}
            history={history}
            historyLoading={historyLoading}
            historyError={historyError}
            openHistory={openHistory}
            onToggleHistory={toggleHistory}
            formatDate={formatDate}
            formatDateTime={formatDateTime}
            sort={sort}
            onSortChange={handleSortChange}
            onRunNow={isAdmin && !projectId ? handleAdminRunNow : undefined}
            runNowGuard={isAdmin && !projectId ? getAdminRunNowGuard : undefined}
          />
          {/* Pagination */}
          <div className="p-4 border-t border-slate-100 dark:border-slate-800/60">
            <PaginationControls
              page={page}
              hasNext={hasNextPage}
              onPrev={() => setPage((p) => Math.max(1, p - 1))}
              onNext={() => setPage((p) => p + 1)}
              prevDisabled={loading}
              nextDisabled={loading}
              nextLabel="Вперёд"
              pageLabel={`Стр. ${Math.min(page, totalPages)} из ${totalPages}`}
              middleSlot={
                <>
                  <span className="text-xs text-slate-500">Строк</span>
                  <select
                    value={limit}
                    onChange={(e) => {
                      setLimit(Number(e.target.value) || DEFAULT_LIMIT);
                      setPage(1);
                    }}
                    className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-800 dark:bg-slate-950"
                    disabled={loading}
                  >
                    {[10, 20, 50, 100].map((s) => (
                      <option key={s} value={s}>{s}</option>
                    ))}
                  </select>
                </>
              }
            />
          </div>
        </div>
      )}
        </div>
      </main>
    </div>
  );
}

// verify markers: listByDomain listAdmin listDomainStats "status" "from" "to" "domainId" "isIndexed" "search" "sort"
