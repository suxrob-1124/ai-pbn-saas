"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { FiActivity, FiRefreshCw } from "react-icons/fi";
import { FailedChecksAlert } from "../../../components/indexing/FailedChecksAlert";
import { IndexCalendar } from "../../../components/indexing/IndexCalendar";
import { IndexFiltersBar } from "../../../components/indexing/IndexFiltersBar";
import { IndexStats, type PeriodKey } from "../../../components/indexing/IndexStats";
import { IndexTable, type IndexCheckSort, type IndexCheckSortKey } from "../../../components/indexing/IndexTable";
import { useAuthGuard } from "../../../lib/useAuth";
import { authFetchCached, invalidateAuthCache } from "../../../lib/http";
import { useDebouncedValue } from "../../../lib/useDebouncedValue";
import {
  listAdmin,
  listAdminCalendar,
  listAdminStats,
  listAdminHistory,
  listByDomain,
  listByProject,
  listDomainCalendar,
  listDomainStats,
  listFailed,
  listDomainHistory,
  listProjectCalendar,
  listProjectHistory,
  listProjectStats,
  runAdminManual,
  runManual,
  runManualProject
} from "../../../lib/indexChecksApi";
import type {
  IndexCheckCalendarDayDTO,
  IndexCheckDTO,
  IndexCheckHistoryDTO,
  IndexCheckStatus,
  IndexCheckStatsDTO,
  IndexChecksFilters,
  IndexChecksResponse
} from "../../../types/indexChecks";
import type { IndexFiltersValue } from "../../../components/indexing/IndexFiltersBar";

const DEFAULT_LIMIT = 20;
const SEARCH_DEBOUNCE_MS = 400;
const DEFAULT_SORT: IndexCheckSort = { key: "check_date", dir: "desc" };
const DEFAULT_SORT_PARAM = sortToParam(DEFAULT_SORT);

export default function IndexingMonitoringPage() {
  return (
    <Suspense
      fallback={<div className="p-4 text-sm text-slate-500 dark:text-slate-400">Загрузка мониторинга...</div>}
    >
      <IndexingMonitoringContent />
    </Suspense>
  );
}

function IndexingMonitoringContent() {
  const { me, loading: authLoading } = useAuthGuard();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const projectId = (searchParams.get("projectId") || "").trim();

  const isAdmin = (me?.role || "").toLowerCase() === "admin";

  const [checks, setChecks] = useState<IndexCheckDTO[]>([]);
  const [totalChecks, setTotalChecks] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [projectName, setProjectName] = useState("");
  const [domainLabel, setDomainLabel] = useState("");
  const [failedChecks, setFailedChecks] = useState<IndexCheckDTO[]>([]);
  const [failedTotal, setFailedTotal] = useState(0);
  const [failedLoading, setFailedLoading] = useState(false);
  const [failedError, setFailedError] = useState<string | null>(null);
  const [stats, setStats] = useState<IndexCheckStatsDTO | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [statsPeriod, setStatsPeriod] = useState<PeriodKey>("30d");
  const [calendarDays, setCalendarDays] = useState<IndexCheckCalendarDayDTO[]>([]);
  const [calendarLoading, setCalendarLoading] = useState(false);
  const [calendarError, setCalendarError] = useState<string | null>(null);

  const [statusFilter, setStatusFilter] = useState<IndexCheckStatus[]>(() =>
    parseStatusParam(searchParams.get("status"))
  );
  const [indexedFilter, setIndexedFilter] = useState<"all" | "true" | "false">(
    (searchParams.get("isIndexed") as "all" | "true" | "false") || "all"
  );
  const [domainFilter, setDomainFilter] = useState(searchParams.get("domainId") || "");
  const [dateFrom, setDateFrom] = useState(searchParams.get("from") || "");
  const [dateTo, setDateTo] = useState(searchParams.get("to") || "");
  const [search, setSearch] = useState(searchParams.get("search") || "");
  const debouncedSearch = useDebouncedValue(search, SEARCH_DEBOUNCE_MS);
  const [sort, setSort] = useState<IndexCheckSort>(() => parseSortParam(searchParams.get("sort")));
  const [page, setPage] = useState(() => {
    const initial = Number(searchParams.get("page") || 1);
    return Number.isFinite(initial) && initial > 0 ? initial : 1;
  });
  const [limit, setLimit] = useState(() => {
    const initial = Number(searchParams.get("limit") || DEFAULT_LIMIT);
    return Number.isFinite(initial) && initial > 0 ? initial : DEFAULT_LIMIT;
  });

  const [history, setHistory] = useState<Record<string, IndexCheckHistoryDTO[]>>({});
  const [historyLoading, setHistoryLoading] = useState<Record<string, boolean>>({});
  const [historyError, setHistoryError] = useState<Record<string, string | null>>({});
  const [openHistory, setOpenHistory] = useState<Record<string, boolean>>({});

  const domainScope = domainFilter.trim();
  const permissionDenied = !authLoading && !isAdmin && !projectId && !domainScope;
  const querySnapshot = searchParams.toString();

  useEffect(() => {
    let cancelled = false;
    if (!projectId) {
      setProjectName("");
      return;
    }
    setProjectName("");
    authFetchCached<{ project?: { name?: string } }>(
      `/api/projects/${projectId}/summary`,
      undefined,
      { ttlMs: 15000, key: `project-summary:${projectId}` }
    )
      .then((data) => {
        if (!cancelled) {
          setProjectName((data?.project?.name || "").trim());
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProjectName("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  useEffect(() => {
    let cancelled = false;
    if (!domainScope) {
      setDomainLabel("");
      return;
    }
    setDomainLabel("");
    authFetchCached<{ domain?: { url?: string } }>(
      `/api/domains/${domainScope}/summary?gen_limit=1&link_limit=1`,
      undefined,
      { ttlMs: 15000, key: `domain-summary:${domainScope}` }
    )
      .then((data) => {
        const label = (data?.domain?.url || "").trim();
        if (!cancelled) {
          setDomainLabel(label);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDomainLabel("");
        }
      });
    return () => {
      cancelled = true;
    };
  }, [domainScope]);

  const sortParam = useMemo(() => sortToParam(sort), [sort]);

  const filters = useMemo<IndexChecksFilters>(() => {
    const base: IndexChecksFilters = {
      limit,
      page
    };
    if (statusFilter.length > 0) {
      base.status = statusFilter.join(",");
    }
    if (debouncedSearch.trim()) {
      base.search = debouncedSearch.trim();
    }
    if (sortParam) {
      base.sort = sortParam;
    }
    if (indexedFilter !== "all") {
      base.isIndexed = indexedFilter === "true";
    }
    if (dateFrom) {
      base.from = dateFrom;
    }
    if (dateTo) {
      base.to = dateTo;
    }
    if (domainScope) {
      base.domainId = domainScope;
    }
    return base;
  }, [
    dateFrom,
    dateTo,
    debouncedSearch,
    domainScope,
    indexedFilter,
    limit,
    page,
    sortParam,
    statusFilter
  ]);

  const statsRange = useMemo(() => {
    const days = periodToDays(statsPeriod);
    const to = new Date();
    const from = new Date(Date.UTC(to.getUTCFullYear(), to.getUTCMonth(), to.getUTCDate()));
    from.setUTCDate(from.getUTCDate() - days + 1);
    return { from: formatDateKey(from), to: formatDateKey(to) };
  }, [statsPeriod]);

  const calendarMonth = useMemo(() => {
    const base = dateFrom || dateTo || new Date().toISOString().slice(0, 10);
    return base.slice(0, 7);
  }, [dateFrom, dateTo]);

  const loadChecks = useCallback(async () => {
    if (permissionDenied) {
      return;
    }
    setLoading(true);
    setError(null);
    try {
      let list: IndexChecksResponse | null = null;
      if (projectId) {
        list = await listByProject(projectId, filters);
      } else if (isAdmin) {
        list = await listAdmin(filters);
      } else if (domainScope) {
        list = await listByDomain(domainScope, filters);
      }
      setChecks(Array.isArray(list?.items) ? list!.items : []);
      setTotalChecks(typeof list?.total === "number" ? list.total : 0);
    } catch (err: any) {
      setError(err?.message || "Не удалось загрузить проверки индексации");
    } finally {
      setLoading(false);
    }
  }, [domainScope, filters, isAdmin, permissionDenied, projectId]);

  useEffect(() => {
    loadChecks();
  }, [loadChecks]);

  useEffect(() => {
    const statusParam = parseStatusParam(searchParams.get("status"));
    const searchParam = searchParams.get("search") || "";
    const fromParam = searchParams.get("from") || "";
    const toParam = searchParams.get("to") || "";
    const domainParam = searchParams.get("domainId") || "";
    const indexedParam = (searchParams.get("isIndexed") || "all") as "all" | "true" | "false";
    const sortParam = parseSortParam(searchParams.get("sort"));
    const pageParam = Number(searchParams.get("page") || 1);
    const nextPage = Number.isFinite(pageParam) && pageParam > 0 ? pageParam : 1;
    const limitParam = Number(searchParams.get("limit") || DEFAULT_LIMIT);
    const nextLimit = Number.isFinite(limitParam) && limitParam > 0 ? limitParam : DEFAULT_LIMIT;

    if (!sameStatusList(statusParam, statusFilter)) {
      setStatusFilter(statusParam);
    }
    if (indexedParam !== indexedFilter) {
      setIndexedFilter(indexedParam);
    }
    if (searchParam !== search) {
      setSearch(searchParam);
    }
    if (fromParam !== dateFrom) {
      setDateFrom(fromParam);
    }
    if (toParam !== dateTo) {
      setDateTo(toParam);
    }
    if (domainParam !== domainFilter) {
      setDomainFilter(domainParam);
    }
    if (!sameSort(sortParam, sort)) {
      setSort(sortParam);
    }
    if (nextPage !== page) {
      setPage(nextPage);
    }
    if (nextLimit !== limit) {
      setLimit(nextLimit);
    }
  }, [querySnapshot, searchParams]);

  useEffect(() => {
    const params = new URLSearchParams(searchParams.toString());
    if (statusFilter.length === 0) {
      params.delete("status");
    } else {
      params.set("status", statusFilter.join(","));
    }
    if (debouncedSearch.trim()) {
      params.set("search", debouncedSearch.trim());
    } else {
      params.delete("search");
    }
    if (indexedFilter === "all") {
      params.delete("isIndexed");
    } else {
      params.set("isIndexed", indexedFilter);
    }
    if (dateFrom) {
      params.set("from", dateFrom);
    } else {
      params.delete("from");
    }
    if (dateTo) {
      params.set("to", dateTo);
    } else {
      params.delete("to");
    }
    if (domainScope) {
      params.set("domainId", domainScope);
    } else {
      params.delete("domainId");
    }
    if (page > 1) {
      params.set("page", String(page));
    } else {
      params.delete("page");
    }
    if (limit !== DEFAULT_LIMIT) {
      params.set("limit", String(limit));
    } else {
      params.delete("limit");
    }
    if (sortParam && sortParam !== DEFAULT_SORT_PARAM) {
      params.set("sort", sortParam);
    } else {
      params.delete("sort");
    }
    const nextQuery = params.toString();
    const currentQuery = searchParams.toString();
    if (nextQuery !== currentQuery) {
      const href = nextQuery ? `${pathname}?${nextQuery}` : pathname;
      router.replace(href as any, { scroll: false });
    }
  }, [
    dateFrom,
    dateTo,
    debouncedSearch,
    domainScope,
    indexedFilter,
    page,
    pathname,
    router,
    searchParams,
    sortParam,
    statusFilter
  ]);

  const loadHistory = useCallback(
    async (checkId: string) => {
      setHistoryLoading((prev) => ({ ...prev, [checkId]: true }));
      setHistoryError((prev) => ({ ...prev, [checkId]: null }));
      try {
        let list: IndexCheckHistoryDTO[] = [];
        if (projectId) {
          list = await listProjectHistory(projectId, checkId, 50);
        } else if (!isAdmin && domainScope) {
          list = await listDomainHistory(domainScope, checkId, 50);
        } else {
          list = await listAdminHistory(checkId, 50);
        }
        setHistory((prev) => ({ ...prev, [checkId]: Array.isArray(list) ? list : [] }));
      } catch (err: any) {
        setHistoryError((prev) => ({
          ...prev,
          [checkId]: err?.message || "Не удалось загрузить историю"
        }));
      } finally {
        setHistoryLoading((prev) => ({ ...prev, [checkId]: false }));
      }
    },
    [domainScope, isAdmin, projectId]
  );

  const toggleHistory = (checkId: string) => {
    setOpenHistory((prev) => {
      const next = !prev[checkId];
      if (next && !history[checkId] && !historyLoading[checkId]) {
        loadHistory(checkId).catch(() => {});
      }
      return { ...prev, [checkId]: next };
    });
  };

  const handleManualRun = async () => {
    setError(null);
    try {
      invalidateAuthCache("index-checks/stats");
      invalidateAuthCache("index-checks/calendar");
      if (domainScope) {
        await runManual(domainScope);
      } else if (projectId) {
        await runManualProject(projectId);
      } else {
        return;
      }
      await Promise.all([loadChecks(), loadStats(), loadCalendar()]);
    } catch (err: any) {
      setError(err?.message || "Не удалось запустить проверку");
    }
  };

  const canShowFailedAlert = isAdmin && !projectId;

  const loadFailed = useCallback(async () => {
    if (!canShowFailedAlert) {
      return;
    }
    setFailedLoading(true);
    setFailedError(null);
    try {
      const list = await listFailed({ limit: 5, domainId: domainScope || undefined });
      setFailedChecks(Array.isArray(list?.items) ? list.items : []);
      setFailedTotal(typeof list?.total === "number" ? list.total : 0);
    } catch (err: any) {
      setFailedError(err?.message || "Не удалось загрузить проблемные проверки");
    } finally {
      setFailedLoading(false);
    }
  }, [canShowFailedAlert, domainScope]);

  useEffect(() => {
    loadFailed();
  }, [loadFailed]);

  const loadStats = useCallback(async () => {
    if (permissionDenied) {
      return;
    }
    setStatsLoading(true);
    setStatsError(null);
    try {
      const statsFilters: IndexChecksFilters = {
        from: statsRange.from,
        to: statsRange.to
      };
      if (statusFilter.length > 0) {
        statsFilters.status = statusFilter.join(",");
      }
      if (indexedFilter !== "all") {
        statsFilters.isIndexed = indexedFilter === "true";
      }
      if (domainScope) {
        statsFilters.domainId = domainScope;
      }
      let data: IndexCheckStatsDTO | null = null;
      if (projectId) {
        data = await listProjectStats(projectId, statsFilters);
      } else if (isAdmin) {
        data = await listAdminStats(statsFilters);
      } else if (domainScope) {
        data = await listDomainStats(domainScope, statsFilters);
      }
      setStats(data);
    } catch (err: any) {
      setStatsError(err?.message || "Не удалось загрузить статистику");
    } finally {
      setStatsLoading(false);
    }
  }, [domainScope, indexedFilter, isAdmin, permissionDenied, projectId, statsRange, statusFilter]);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  const loadCalendar = useCallback(async () => {
    if (permissionDenied) {
      return;
    }
    setCalendarLoading(true);
    setCalendarError(null);
    try {
      let list: IndexCheckCalendarDayDTO[] = [];
      if (projectId) {
        list = await listProjectCalendar(projectId, { month: calendarMonth });
      } else if (isAdmin) {
        list = await listAdminCalendar({ month: calendarMonth, domainId: domainScope || undefined });
      } else if (domainScope) {
        list = await listDomainCalendar(domainScope, { month: calendarMonth });
      }
      setCalendarDays(Array.isArray(list) ? list : []);
    } catch (err: any) {
      setCalendarError(err?.message || "Не удалось загрузить календарь");
    } finally {
      setCalendarLoading(false);
    }
  }, [calendarMonth, domainScope, isAdmin, permissionDenied, projectId]);

  useEffect(() => {
    loadCalendar();
  }, [loadCalendar]);

  const handleAdminRunNow = useCallback(
    async (domainId: string) => {
      if (!isAdmin) {
        return;
      }
      setError(null);
      try {
        invalidateAuthCache("index-checks/stats");
        invalidateAuthCache("index-checks/calendar");
        await runAdminManual(domainId);
        await Promise.all([loadChecks(), loadStats(), loadCalendar(), loadFailed()]);
      } catch (err: any) {
        setError(err?.message || "Не удалось запустить проверку");
      }
    },
    [isAdmin, loadCalendar, loadChecks, loadFailed, loadStats]
  );

  const appliedFilters = useMemo<IndexFiltersValue>(
    () => ({
      statuses: statusFilter,
      from: dateFrom,
      to: dateTo,
      domainId: domainFilter,
      isIndexed: indexedFilter,
      search
    }),
    [dateFrom, dateTo, domainFilter, indexedFilter, search, statusFilter]
  );

  const defaultFilters = useMemo<IndexFiltersValue>(
    () => ({
      statuses: [],
      from: "",
      to: "",
      domainId: "",
      isIndexed: "all",
      search: ""
    }),
    []
  );

  const applyFilters = (next: IndexFiltersValue) => {
    setStatusFilter(normalizeStatusList(next.statuses));
    setDateFrom(next.from);
    setDateTo(next.to);
    setDomainFilter(next.domainId);
    setIndexedFilter(next.isIndexed);
    setSearch(next.search);
    setPage(1);
  };

  const handleSearchChange = useCallback((value: string) => {
    setSearch(value);
    setPage(1);
  }, []);

  const handleSortChange = (next: IndexCheckSort) => {
    setSort(next);
    setPage(1);
  };

  const handleRefresh = async () => {
    invalidateAuthCache("index-checks/stats");
    invalidateAuthCache("index-checks/calendar");
    await Promise.all([loadChecks(), loadFailed(), loadStats(), loadCalendar()]);
  };

  const visibleChecks = checks;
  const hasNextPage = page * limit < totalChecks;
  const totalPages = Math.max(1, Math.ceil(totalChecks / limit));
  const pageLabel = Math.min(page, totalPages);

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages);
    }
  }, [page, totalPages]);

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-xl font-semibold">Мониторинг · Индексация</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
            {domainScope
                ? domainLabel
                  ? `Доменные проверки: ${domainLabel}`
                  : "Доменные проверки"
                : projectId
                  ? projectName
                    ? `Проектные проверки: ${projectName}`
                    : "Проектные проверки"
                  : "Глобальный список проверок"}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={loadChecks}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              type="button"
              disabled={loading}
            >
              <FiRefreshCw /> Обновить
            </button>
            {(domainScope || projectId) && (
              <button
                onClick={handleManualRun}
                className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
                type="button"
                disabled={loading}
              >
                <FiActivity /> Запустить вручную
              </button>
            )}
          </div>
        </div>
      </div>

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

      {permissionDenied && (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-700 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-200">
          Доступ запрещен. Глобальный список доступен только администраторам.
        </div>
      )}

      {error && !permissionDenied && (
        <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
          {error}
        </div>
      )}

      {canShowFailedAlert && !permissionDenied && (
        <FailedChecksAlert
          checks={failedChecks}
          failedCount={failedTotal}
          loading={failedLoading}
          error={failedError}
          onRefresh={loadFailed}
          onViewDetails={() =>
            applyFilters({
              statuses: ["failed_investigation"],
              from: "",
              to: "",
              domainId: domainFilter,
              isIndexed: "all",
              search
            })
          }
        />
      )}

      {statsError && !permissionDenied && (
        <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
          {statsError}
        </div>
      )}

      {!permissionDenied && (
        <IndexStats
          stats={stats}
          daily={stats?.daily || []}
          loading={statsLoading}
          period={statsPeriod}
          onPeriodChange={setStatsPeriod}
        />
      )}

      {!permissionDenied && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
          <div className="lg:col-span-2 bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow">
            <IndexTable
              checks={visibleChecks}
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
            />
            <div className="mt-4 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500 dark:text-slate-400">
              <div>
                Страница {pageLabel} из {totalPages}
              </div>
              <div className="flex items-center gap-2">
                <span>Размер страницы</span>
                <select
                  value={limit}
                  onChange={(e) => {
                    const next = Number(e.target.value) || DEFAULT_LIMIT;
                    setLimit(next);
                    setPage(1);
                  }}
                  className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs dark:border-slate-800 dark:bg-slate-950"
                  disabled={loading}
                >
                  {[10, 20, 50, 100].map((size) => (
                    <option key={size} value={size}>
                      {size}
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={loading || page <= 1}
                  className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Назад
                </button>
                <button
                  type="button"
                  onClick={() => setPage((p) => p + 1)}
                  disabled={loading || !hasNextPage}
                  className="rounded-lg border border-slate-200 bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                >
                  Вперед
                </button>
              </div>
            </div>
          </div>
          <div className="space-y-2">
            <IndexCalendar
              days={calendarDays}
              baseDate={dateFrom || undefined}
              loading={calendarLoading}
              selectedDate={dateFrom && dateFrom === dateTo ? dateFrom : undefined}
              onSelectDate={(date) => {
                const next = dateFrom === date && dateTo === date ? { from: "", to: "" } : { from: date, to: date };
                applyFilters({
                  statuses: statusFilter,
                  from: next.from,
                  to: next.to,
                  domainId: domainFilter,
                  isIndexed: indexedFilter,
                  search
                });
              }}
            />
            {calendarError && (
              <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
                {calendarError}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function formatDate(value?: string | null) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleDateString();
}

function formatDateTime(value?: string | null) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function parseStatusParam(raw: string | null): IndexCheckStatus[] {
  if (!raw) {
    return [];
  }
  return normalizeStatusList(raw.split(",") as IndexCheckStatus[]);
}

function normalizeStatusList(statuses: IndexCheckStatus[]): IndexCheckStatus[] {
  const trimmed = statuses.map((item) => item.trim()).filter(Boolean);
  return Array.from(new Set(trimmed));
}

const SORT_KEYS: IndexCheckSortKey[] = [
  "domain",
  "check_date",
  "status",
  "attempts",
  "is_indexed",
  "last_attempt_at",
  "next_retry_at"
];

function parseSortParam(raw: string | null): IndexCheckSort {
  if (!raw) {
    return DEFAULT_SORT;
  }
  const cleaned = raw.trim();
  if (!cleaned) {
    return DEFAULT_SORT;
  }
  const [keyRaw, dirRaw] = cleaned.split(":", 2);
  const key = keyRaw.trim();
  if (!isSortKey(key)) {
    return DEFAULT_SORT;
  }
  const dir = dirRaw && dirRaw.trim().toLowerCase() === "asc" ? "asc" : "desc";
  return { key, dir };
}

function sortToParam(sort: IndexCheckSort): string {
  return `${sort.key}:${sort.dir}`;
}

function sameSort(a: IndexCheckSort, b: IndexCheckSort): boolean {
  return a.key === b.key && a.dir === b.dir;
}

function isSortKey(value: string): value is IndexCheckSortKey {
  return SORT_KEYS.includes(value as IndexCheckSortKey);
}

function sameStatusList(a: IndexCheckStatus[], b: IndexCheckStatus[]): boolean {
  if (a.length !== b.length) return false;
  const setA = new Set(a.map((item) => item.trim()));
  for (const item of b) {
    if (!setA.has(item.trim())) {
      return false;
    }
  }
  return true;
}

function periodToDays(period: PeriodKey) {
  switch (period) {
    case "7d":
      return 7;
    case "90d":
      return 90;
    default:
      return 30;
  }
}

function formatDateKey(date: Date) {
  return date.toISOString().slice(0, 10);
}
