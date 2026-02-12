"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { FiActivity, FiRefreshCw } from "react-icons/fi";
import { FailedChecksAlert } from "../../../components/indexing/FailedChecksAlert";
import { IndexCalendar } from "../../../components/indexing/IndexCalendar";
import { IndexFiltersBar } from "../../../components/indexing/IndexFiltersBar";
import { IndexStats } from "../../../components/indexing/IndexStats";
import { IndexTable } from "../../../components/indexing/IndexTable";
import { useAuthGuard } from "../../../lib/useAuth";
import {
  listAdmin,
  listAdminHistory,
  listByDomain,
  listByProject,
  listFailed,
  listDomainHistory,
  listProjectHistory,
  runManual,
  runManualProject
} from "../../../lib/indexChecksApi";
import type {
  IndexCheckDTO,
  IndexCheckHistoryDTO,
  IndexCheckStatus,
  IndexChecksFilters
} from "../../../types/indexChecks";
import type { IndexFiltersValue } from "../../../components/indexing/IndexFiltersBar";

const DEFAULT_LIMIT = 20;

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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [failedChecks, setFailedChecks] = useState<IndexCheckDTO[]>([]);
  const [failedLoading, setFailedLoading] = useState(false);
  const [failedError, setFailedError] = useState<string | null>(null);

  const [statusFilter, setStatusFilter] = useState<IndexCheckStatus[]>(() =>
    parseStatusParam(searchParams.get("status"))
  );
  const [indexedFilter, setIndexedFilter] = useState<"all" | "true" | "false">(
    (searchParams.get("isIndexed") as "all" | "true" | "false") || "all"
  );
  const [domainFilter, setDomainFilter] = useState(searchParams.get("domainId") || "");
  const [dateFrom, setDateFrom] = useState(searchParams.get("from") || "");
  const [dateTo, setDateTo] = useState(searchParams.get("to") || "");
  const [page, setPage] = useState(() => {
    const initial = Number(searchParams.get("page") || 1);
    return Number.isFinite(initial) && initial > 0 ? initial : 1;
  });
  const [limit, setLimit] = useState(DEFAULT_LIMIT);

  const [history, setHistory] = useState<Record<string, IndexCheckHistoryDTO[]>>({});
  const [historyLoading, setHistoryLoading] = useState<Record<string, boolean>>({});
  const [historyError, setHistoryError] = useState<Record<string, string | null>>({});
  const [openHistory, setOpenHistory] = useState<Record<string, boolean>>({});

  const domainScope = domainFilter.trim();
  const permissionDenied = !authLoading && !isAdmin && !projectId && !domainScope;
  const querySnapshot = searchParams.toString();

  const filters = useMemo<IndexChecksFilters>(() => {
    const base: IndexChecksFilters = {
      limit,
      page
    };
    if (statusFilter.length === 1) {
      base.status = statusFilter[0];
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
  }, [dateFrom, dateTo, domainScope, indexedFilter, limit, page, statusFilter]);

  const loadChecks = useCallback(async () => {
    if (permissionDenied) {
      return;
    }
    setLoading(true);
    setError(null);
    try {
      let list: IndexCheckDTO[] = [];
      if (projectId) {
        list = await listByProject(projectId, filters);
      } else if (isAdmin) {
        list = await listAdmin(filters);
      } else if (domainScope) {
        list = await listByDomain(domainScope, filters);
      }
      setChecks(Array.isArray(list) ? list : []);
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
    const fromParam = searchParams.get("from") || "";
    const toParam = searchParams.get("to") || "";
    const domainParam = searchParams.get("domainId") || "";
    const indexedParam = (searchParams.get("isIndexed") || "all") as "all" | "true" | "false";
    const pageParam = Number(searchParams.get("page") || 1);
    const nextPage = Number.isFinite(pageParam) && pageParam > 0 ? pageParam : 1;

    if (!sameStatusList(statusParam, statusFilter)) {
      setStatusFilter(statusParam);
    }
    if (indexedParam !== indexedFilter) {
      setIndexedFilter(indexedParam);
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
    if (nextPage !== page) {
      setPage(nextPage);
    }
  }, [dateFrom, dateTo, domainFilter, indexedFilter, page, querySnapshot, searchParams, statusFilter]);

  useEffect(() => {
    const params = new URLSearchParams(searchParams.toString());
    if (statusFilter.length === 0) {
      params.delete("status");
    } else {
      params.set("status", statusFilter.join(","));
    }
    params.delete("search");
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
    const nextQuery = params.toString();
    const currentQuery = searchParams.toString();
    if (nextQuery !== currentQuery) {
      router.replace(nextQuery ? `${pathname}?${nextQuery}` : pathname, { scroll: false });
    }
  }, [dateFrom, dateTo, domainScope, indexedFilter, page, pathname, router, searchParams, statusFilter]);

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
      if (domainScope) {
        await runManual(domainScope);
      } else if (projectId) {
        await runManualProject(projectId);
      } else {
        return;
      }
      await loadChecks();
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
      setFailedChecks(Array.isArray(list) ? list : []);
    } catch (err: any) {
      setFailedError(err?.message || "Не удалось загрузить проблемные проверки");
    } finally {
      setFailedLoading(false);
    }
  }, [canShowFailedAlert, domainScope]);

  useEffect(() => {
    loadFailed();
  }, [loadFailed]);

  const appliedFilters = useMemo<IndexFiltersValue>(
    () => ({
      statuses: statusFilter,
      from: dateFrom,
      to: dateTo,
      domainId: domainFilter,
      isIndexed: indexedFilter
    }),
    [dateFrom, dateTo, domainFilter, indexedFilter, statusFilter]
  );

  const defaultFilters = useMemo<IndexFiltersValue>(
    () => ({
      statuses: [],
      from: "",
      to: "",
      domainId: "",
      isIndexed: "all"
    }),
    []
  );

  const applyFilters = (next: IndexFiltersValue) => {
    setStatusFilter(normalizeStatusList(next.statuses));
    setDateFrom(next.from);
    setDateTo(next.to);
    setDomainFilter(next.domainId);
    setIndexedFilter(next.isIndexed);
    setPage(1);
  };

  const handleRefresh = async () => {
    await Promise.all([loadChecks(), loadFailed()]);
  };

  const visibleChecks = useMemo(
    () => filterChecks(checks, statusFilter, indexedFilter, dateFrom, dateTo),
    [checks, dateFrom, dateTo, indexedFilter, statusFilter]
  );

  return (
    <div className="space-y-4">
      <div className="bg-white/80 dark:bg-slate-900/60 border border-slate-200 dark:border-slate-800 rounded-xl p-6 shadow-xl">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-xl font-semibold">Monitoring · Indexing</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400">
            {domainScope
                ? `Доменные проверки: ${domainScope}`
                : projectId
                  ? `Проектные проверки: ${projectId}`
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
          failedCount={failedChecks.length}
          loading={failedLoading}
          error={failedError}
          onRefresh={loadFailed}
          onViewDetails={() =>
            applyFilters({
              statuses: ["failed_investigation"],
              from: "",
              to: "",
              domainId: domainFilter,
              isIndexed: "all"
            })
          }
        />
      )}

      {!permissionDenied && <IndexStats checks={visibleChecks} loading={loading} />}

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
            />
          </div>
          <IndexCalendar
            checks={visibleChecks}
            baseDate={dateFrom || undefined}
            loading={loading}
            selectedDate={dateFrom && dateFrom === dateTo ? dateFrom : undefined}
            onSelectDate={(date) => {
              const next = dateFrom === date && dateTo === date ? { from: "", to: "" } : { from: date, to: date };
              applyFilters({
                statuses: statusFilter,
                from: next.from,
                to: next.to,
                domainId: domainFilter,
                isIndexed: indexedFilter
              });
            }}
          />
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

function filterChecks(
  checks: IndexCheckDTO[],
  statuses: IndexCheckStatus[],
  isIndexed: "all" | "true" | "false",
  from: string,
  to: string
) {
  const statusList = normalizeStatusList(statuses);
  const fromKey = from.trim();
  const toKey = to.trim();
  return checks.filter((check) => {
    if (statusList.length > 0 && !statusList.includes(check.status as IndexCheckStatus)) {
      return false;
    }
    if (isIndexed !== "all") {
      if (isIndexed === "true" && check.is_indexed !== true) {
        return false;
      }
      if (isIndexed === "false" && check.is_indexed !== false) {
        return false;
      }
    }
    const dateKey = toDateKey(check.check_date);
    if (fromKey && dateKey && dateKey < fromKey) {
      return false;
    }
    if (toKey && dateKey && dateKey > toKey) {
      return false;
    }
    return true;
  });
}

function toDateKey(value?: string | null): string {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  return date.toISOString().slice(0, 10);
}
