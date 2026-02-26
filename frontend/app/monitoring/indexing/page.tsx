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
import { showToast } from "../../../lib/toastStore";
import { useActionLocks } from "../../../features/editor-v3/hooks/useActionLocks";
import { FlowStateBanner } from "../../../features/queue-monitoring/components/FlowStateBanner";
import { useFlowState } from "../../../features/queue-monitoring/hooks/useFlowState";
import { PaginationControls } from "../../../features/queue-monitoring/components/PaginationControls";
import { canRun } from "../../../features/queue-monitoring/services/actionGuards";
import { queueMonitoringRu, toDiagnosticsText } from "../../../features/queue-monitoring/services/i18n-ru";
import {
  readEnumParam,
  readPositiveIntParam,
  readStringParam,
  setOptionalNumberParam,
  setOptionalParam
} from "../../../features/queue-monitoring/services/queryParams";
import {
  getTotalPages,
  hasNextPageByTotal
} from "../../../features/queue-monitoring/services/primitives";
import {
  getIndexCheckStatusMeta,
  normalizeIndexCheckStatusList
} from "../../../features/queue-monitoring/services/statusMeta";
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
const INDEXED_FILTER_VALUES = ["all", "true", "false"] as const;

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
  const [errorDiagnostics, setErrorDiagnostics] = useState<string | null>(null);
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
  const [indexedFilter, setIndexedFilter] = useState<"all" | "true" | "false">(() =>
    readEnumParam(searchParams, "isIndexed", INDEXED_FILTER_VALUES, "all")
  );
  const [domainFilter, setDomainFilter] = useState(() => readStringParam(searchParams, "domainId"));
  const [dateFrom, setDateFrom] = useState(() => readStringParam(searchParams, "from"));
  const [dateTo, setDateTo] = useState(() => readStringParam(searchParams, "to"));
  const [search, setSearch] = useState(() => readStringParam(searchParams, "search"));
  const debouncedSearch = useDebouncedValue(search, SEARCH_DEBOUNCE_MS);
  const [sort, setSort] = useState<IndexCheckSort>(() => parseSortParam(searchParams.get("sort")));
  const [page, setPage] = useState(() => {
    return readPositiveIntParam(searchParams, "page", 1);
  });
  const [limit, setLimit] = useState(() => {
    return readPositiveIntParam(searchParams, "limit", DEFAULT_LIMIT);
  });

  const [history, setHistory] = useState<Record<string, IndexCheckHistoryDTO[]>>({});
  const [historyLoading, setHistoryLoading] = useState<Record<string, boolean>>({});
  const [historyError, setHistoryError] = useState<Record<string, string | null>>({});
  const [openHistory, setOpenHistory] = useState<Record<string, boolean>>({});
  const { isLocked, lockReason, runLocked } = useActionLocks();
  const refreshFlow = useFlowState();
  const runFlow = useFlowState();

  const domainScope = domainFilter.trim();
  const permissionDenied = !authLoading && !isAdmin && !projectId && !domainScope;
  const querySnapshot = searchParams.toString();
  const refreshLockKey = projectId
    ? `monitoring:indexing:project:${projectId}:refresh`
    : domainScope
      ? `monitoring:indexing:domain:${domainScope}:refresh`
      : "monitoring:indexing:global:refresh";
  const manualRunLockKey = projectId
    ? `monitoring:indexing:project:${projectId}:run-manual`
    : domainScope
      ? `monitoring:indexing:domain:${domainScope}:run-manual`
      : "monitoring:indexing:run-manual";
  const adminRunNowLockKey = (domainId: string) => `monitoring:indexing:admin:${domainId}:run-now`;

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
    setErrorDiagnostics(null);
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
      setError("Не удалось загрузить проверки индексации");
      setErrorDiagnostics(toDiagnosticsText(err) || null);
    } finally {
      setLoading(false);
    }
  }, [domainScope, filters, isAdmin, permissionDenied, projectId]);

  useEffect(() => {
    loadChecks();
  }, [loadChecks]);

  useEffect(() => {
    const statusParam = parseStatusParam(searchParams.get("status"));
    const searchParam = readStringParam(searchParams, "search");
    const fromParam = readStringParam(searchParams, "from");
    const toParam = readStringParam(searchParams, "to");
    const domainParam = readStringParam(searchParams, "domainId");
    const indexedParam = readEnumParam(searchParams, "isIndexed", INDEXED_FILTER_VALUES, "all");
    const sortParam = parseSortParam(searchParams.get("sort"));
    const nextPage = readPositiveIntParam(searchParams, "page", 1);
    const nextLimit = readPositiveIntParam(searchParams, "limit", DEFAULT_LIMIT);

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
    setOptionalParam(params, "search", debouncedSearch, "");
    if (indexedFilter === "all") {
      params.delete("isIndexed");
    } else {
      params.set("isIndexed", indexedFilter);
    }
    setOptionalParam(params, "from", dateFrom, "");
    setOptionalParam(params, "to", dateTo, "");
    setOptionalParam(params, "domainId", domainScope, "");
    setOptionalNumberParam(params, "page", page, 1);
    setOptionalNumberParam(params, "limit", limit, DEFAULT_LIMIT);
    setOptionalParam(params, "sort", sortParam, DEFAULT_SORT_PARAM);
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
    runFlow.validating("Проверяем возможность ручного запуска");
    await runLocked(
      manualRunLockKey,
      async () => {
        runFlow.sending("Запускаем проверку индексации вручную");
        setError(null);
        setErrorDiagnostics(null);
        try {
          invalidateAuthCache("index-checks/stats");
          invalidateAuthCache("index-checks/calendar");
          if (domainScope) {
            const result = await runManual(domainScope);
            if (result.run_now_enqueued === false) {
              const pendingLabel = getIndexCheckStatusMeta("pending").label;
              showToast({
                type: "warning",
                title: `Проверка поставлена в статус «${pendingLabel}»`,
                message: result.run_now_error || "Немедленная постановка в очередь не удалась, обработка продолжится по плановому циклу."
              });
            }
          } else if (projectId) {
            const result = await runManualProject(projectId);
            const enqueueFailed = result.enqueue_failed || 0;
            const upsertFailed = result.upsert_failed || 0;
            if (enqueueFailed > 0 || upsertFailed > 0) {
              showToast({
                type: "warning",
                title: "Часть проверок выполнена с ошибками",
                message: `Успешно поставлено: ${result.enqueued || 0}, ошибок upsert: ${upsertFailed}, ошибок enqueue: ${enqueueFailed}.`
              });
            }
          } else {
            return;
          }
          await Promise.all([loadChecks(), loadStats(), loadCalendar()]);
          runFlow.done("Ручной запуск успешно отправлен");
        } catch (err: any) {
          const userMessage = "Не удалось запустить проверку";
          setError(userMessage);
          setErrorDiagnostics(toDiagnosticsText(err) || null);
          runFlow.fail(userMessage, err);
        }
      },
      queueMonitoringRu.lockReasons.manualRunInFlight
    );
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
      runFlow.validating("Проверяем возможность запуска проверки");
      await runLocked(
        adminRunNowLockKey(domainId),
        async () => {
          runFlow.sending("Запускаем проверку по выбранному домену");
          setError(null);
          setErrorDiagnostics(null);
          try {
            invalidateAuthCache("index-checks/stats");
            invalidateAuthCache("index-checks/calendar");
            const result = await runAdminManual(domainId);
            if (result.run_now_enqueued === false) {
              const pendingLabel = getIndexCheckStatusMeta("pending").label;
              showToast({
                type: "warning",
                title: `Проверка поставлена в статус «${pendingLabel}»`,
                message: result.run_now_error || "Немедленная постановка в очередь не удалась, обработка продолжится по плановому циклу."
              });
            }
            await Promise.all([loadChecks(), loadStats(), loadCalendar(), loadFailed()]);
            runFlow.done("Проверка успешно отправлена");
          } catch (err: any) {
            const userMessage = "Не удалось запустить проверку";
            setError(userMessage);
            setErrorDiagnostics(toDiagnosticsText(err) || null);
            runFlow.fail(userMessage, err);
          }
        },
        queueMonitoringRu.lockReasons.manualRunInFlight
      );
    },
    [isAdmin, loadCalendar, loadChecks, loadFailed, loadStats, runFlow, runLocked]
  );

  const getAdminRunNowGuard = useCallback(
    (domainId: string) =>
      canRun({
        busy: loading || isLocked(adminRunNowLockKey(domainId)),
        busyReason: lockReason(adminRunNowLockKey(domainId))
      }),
    [isLocked, loading, lockReason]
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
    setStatusFilter(normalizeIndexCheckStatusList(next.statuses));
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
    refreshFlow.validating("Проверяем доступность обновления мониторинга");
    await runLocked(
      refreshLockKey,
      async () => {
        refreshFlow.sending("Обновляем проверки, статистику и календарь");
        invalidateAuthCache("index-checks/stats");
        invalidateAuthCache("index-checks/calendar");
        await Promise.all([loadChecks(), loadFailed(), loadStats(), loadCalendar()]);
        refreshFlow.done("Данные мониторинга обновлены");
      },
      queueMonitoringRu.lockReasons.refreshInFlight
    );
  };

  const visibleChecks = checks;
  // Эквивалент старой формулы: page * limit < totalChecks.
  const hasNextPage = hasNextPageByTotal(page, limit, totalChecks);
  const totalPages = getTotalPages(totalChecks, limit);
  const pageLabel = Math.min(page, totalPages);
  const refreshGuard = canRun({
    busy: loading || isLocked(refreshLockKey),
    busyReason: lockReason(refreshLockKey)
  });
  const manualRunGuard = canRun({
    busy: loading || isLocked(manualRunLockKey),
    busyReason: lockReason(manualRunLockKey)
  });

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
              onClick={handleRefresh}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              type="button"
              disabled={refreshGuard.disabled}
              title={refreshGuard.reason}
            >
              <FiRefreshCw /> Обновить
            </button>
            {(domainScope || projectId) && (
              <button
                onClick={handleManualRun}
                className="inline-flex items-center gap-2 rounded-lg bg-slate-800 px-3 py-2 text-xs font-semibold text-white hover:bg-slate-700 disabled:opacity-50"
                type="button"
                disabled={manualRunGuard.disabled}
                title={manualRunGuard.reason}
              >
                <FiActivity /> Запустить вручную
              </button>
            )}
          </div>
        </div>
      </div>

      <div className="grid gap-2 md:grid-cols-2">
        <FlowStateBanner title={queueMonitoringRu.flowTitles.monitoring} flow={refreshFlow.flow} />
        <FlowStateBanner title={queueMonitoringRu.flowTitles.manualRun} flow={runFlow.flow} />
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
      {error && !permissionDenied && errorDiagnostics && (
        <details className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700 dark:border-red-700/50 dark:bg-red-900/20 dark:text-red-200">
          <summary className="cursor-pointer select-none">{queueMonitoringRu.diagnostics.title}</summary>
          <pre className="mt-2 whitespace-pre-wrap break-words font-mono text-[11px]">{errorDiagnostics}</pre>
        </details>
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
              runNowGuard={isAdmin && !projectId ? getAdminRunNowGuard : undefined}
            />
            <div className="mt-4">
              <PaginationControls
                page={page}
                hasNext={hasNextPage}
                onPrev={() => setPage((p) => Math.max(1, p - 1))}
                onNext={() => setPage((p) => p + 1)}
                prevDisabled={loading}
                nextDisabled={loading}
                nextLabel="Вперед"
                pageLabel={`Страница ${pageLabel} из ${totalPages}`}
                middleSlot={
                  <>
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
                  </>
                }
              />
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
  return normalizeIndexCheckStatusList(raw.split(",") as IndexCheckStatus[]);
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
