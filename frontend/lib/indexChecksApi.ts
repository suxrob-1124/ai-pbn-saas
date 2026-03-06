import { authFetch, authFetchCached } from "./http";
import type {
  IndexCheckBatchResponse,
  IndexCheckCalendarDayDTO,
  IndexCheckDTO,
  IndexCheckHistoryDTO,
  IndexCheckStatsDTO,
  IndexChecksFilters,
  IndexChecksResponse
} from "../types/indexChecks";

const encodeDomainId = (domainId: string) => {
  const trimmed = domainId.trim();
  if (!trimmed) {
    throw new Error("domainId is required");
  }
  return encodeURIComponent(trimmed);
};

const encodeProjectId = (projectId: string) => {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error("projectId is required");
  }
  return encodeURIComponent(trimmed);
};

const encodeCheckId = (checkId: string) => {
  const trimmed = checkId.trim();
  if (!trimmed) {
    throw new Error("checkId is required");
  }
  return encodeURIComponent(trimmed);
};

const toISO = (value: string | Date) =>
  typeof value === "string" ? value : value.toISOString();

const buildFilters = (filters?: IndexChecksFilters, opts?: { allowDomainId?: boolean }) => {
  if (!filters) {
    return "";
  }
  const params = new URLSearchParams();
  if (filters.status) {
    params.set("status", filters.status);
  }
  if (typeof filters.isIndexed === "boolean") {
    params.set("is_indexed", String(filters.isIndexed));
  }
  if (filters.from) {
    params.set("from", toISO(filters.from));
  }
  if (filters.to) {
    params.set("to", toISO(filters.to));
  }
  if (typeof filters.limit === "number") {
    params.set("limit", String(filters.limit));
  }
  if (typeof filters.page === "number") {
    params.set("page", String(filters.page));
  } else if (typeof filters.offset === "number") {
    if (typeof filters.limit === "number" && filters.limit > 0) {
      const page = Math.floor(filters.offset / filters.limit) + 1;
      params.set("page", String(page));
    } else {
      params.set("offset", String(filters.offset));
    }
  }
  if (filters.search && filters.search.trim()) {
    params.set("search", filters.search.trim());
  }
  if (filters.sort && filters.sort.trim()) {
    params.set("sort", filters.sort.trim());
  }
  if (opts?.allowDomainId && filters.domainId && filters.domainId.trim()) {
    const domainId = filters.domainId.trim();
    params.set("domain_id", domainId);
    if (!filters.search || !filters.search.trim()) {
      params.set("search", domainId);
    }
  }
  const query = params.toString();
  return query ? `?${query}` : "";
};

const buildHistoryQuery = (limit?: number) => {
  if (typeof limit !== "number") {
    return "";
  }
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  return `?${params.toString()}`;
};

const buildCalendarQuery = (filters?: { month?: string } & IndexChecksFilters) => {
  if (!filters) {
    return "";
  }
  const params = new URLSearchParams();
  if (filters.month) {
    params.set("month", filters.month);
  }
  if (filters.from) {
    params.set("from", toISO(filters.from));
  }
  if (filters.to) {
    params.set("to", toISO(filters.to));
  }
  if (filters.domainId && filters.domainId.trim()) {
    params.set("domain_id", filters.domainId.trim());
  }
  const query = params.toString();
  return query ? `?${query}` : "";
};

const buildStatsQuery = (filters?: IndexChecksFilters) => {
  if (!filters) {
    return "";
  }
  const params = new URLSearchParams();
  if (filters.status) {
    params.set("status", filters.status);
  }
  if (typeof filters.isIndexed === "boolean") {
    params.set("is_indexed", String(filters.isIndexed));
  }
  if (filters.from) {
    params.set("from", toISO(filters.from));
  }
  if (filters.to) {
    params.set("to", toISO(filters.to));
  }
  if (filters.domainId && filters.domainId.trim()) {
    params.set("domain_id", filters.domainId.trim());
  }
  const query = params.toString();
  return query ? `?${query}` : "";
};

const AGGREGATE_TTL_MS = 30000;

/** Получить список проверок индексации по домену. */
export async function listByDomain(
  domainId: string,
  filters?: IndexChecksFilters
): Promise<IndexChecksResponse> {
  const encoded = encodeDomainId(domainId);
  const query = buildFilters(filters);
  return authFetch<IndexChecksResponse>(`/api/domains/${encoded}/index-checks${query}`);
}

/** Запустить ручную проверку индексации по домену. */
export async function runManual(domainId: string): Promise<IndexCheckDTO> {
  const encoded = encodeDomainId(domainId);
  return authFetch<IndexCheckDTO>(`/api/domains/${encoded}/index-checks`, {
    method: "POST"
  });
}

/** Получить список проверок индексации по проекту. */
export async function listByProject(
  projectId: string,
  filters?: IndexChecksFilters
): Promise<IndexChecksResponse> {
  const encoded = encodeProjectId(projectId);
  const query = buildFilters(filters);
  return authFetch<IndexChecksResponse>(`/api/projects/${encoded}/index-checks${query}`);
}

/** Запустить ручные проверки индексации по проекту. */
export async function runManualProject(projectId: string): Promise<IndexCheckBatchResponse> {
  const encoded = encodeProjectId(projectId);
  return authFetch<IndexCheckBatchResponse>(`/api/projects/${encoded}/index-checks`, {
    method: "POST"
  });
}

/** Запустить ручную проверку индексации (admin). */
export async function runAdminManual(domainId: string): Promise<IndexCheckDTO> {
  const trimmed = domainId.trim();
  if (!trimmed) {
    throw new Error("domainId is required");
  }
  return authFetch<IndexCheckDTO>(`/api/admin/index-checks/run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ domain_id: trimmed })
  });
}

/** Получить список проверок индексации (admin). */
export async function listAdmin(filters?: IndexChecksFilters): Promise<IndexChecksResponse> {
  const query = buildFilters(filters, { allowDomainId: true });
  return authFetch<IndexChecksResponse>(`/api/admin/index-checks${query}`);
}

/** Получить список проблемных проверок индексации (admin). */
export async function listFailed(filters?: IndexChecksFilters): Promise<IndexChecksResponse> {
  const query = buildFilters(filters, { allowDomainId: true });
  return authFetch<IndexChecksResponse>(`/api/admin/index-checks/failed${query}`);
}

/** Получить агрегированную статистику (admin). */
export async function listAdminStats(filters?: IndexChecksFilters): Promise<IndexCheckStatsDTO> {
  const query = buildStatsQuery(filters);
  return authFetchCached<IndexCheckStatsDTO>(`/api/admin/index-checks/stats${query}`, undefined, {
    ttlMs: AGGREGATE_TTL_MS
  });
}

/** Получить агрегированную статистику по домену. */
export async function listDomainStats(
  domainId: string,
  filters?: IndexChecksFilters
): Promise<IndexCheckStatsDTO> {
  const encoded = encodeDomainId(domainId);
  const query = buildStatsQuery(filters);
  return authFetchCached<IndexCheckStatsDTO>(
    `/api/domains/${encoded}/index-checks/stats${query}`,
    undefined,
    { ttlMs: AGGREGATE_TTL_MS }
  );
}

/** Получить агрегированную статистику по проекту. */
export async function listProjectStats(
  projectId: string,
  filters?: IndexChecksFilters
): Promise<IndexCheckStatsDTO> {
  const encoded = encodeProjectId(projectId);
  const query = buildStatsQuery(filters);
  return authFetchCached<IndexCheckStatsDTO>(
    `/api/projects/${encoded}/index-checks/stats${query}`,
    undefined,
    { ttlMs: AGGREGATE_TTL_MS }
  );
}

/** Получить агрегаты по дням для календаря (admin). */
export async function listAdminCalendar(
  filters?: { month?: string } & IndexChecksFilters
): Promise<IndexCheckCalendarDayDTO[]> {
  const query = buildCalendarQuery(filters);
  return authFetchCached<IndexCheckCalendarDayDTO[]>(
    `/api/admin/index-checks/calendar${query}`,
    undefined,
    { ttlMs: AGGREGATE_TTL_MS }
  );
}

/** Получить агрегаты по дням для календаря по домену. */
export async function listDomainCalendar(
  domainId: string,
  filters?: { month?: string } & IndexChecksFilters
): Promise<IndexCheckCalendarDayDTO[]> {
  const encoded = encodeDomainId(domainId);
  const query = buildCalendarQuery(filters);
  return authFetchCached<IndexCheckCalendarDayDTO[]>(
    `/api/domains/${encoded}/index-checks/calendar${query}`,
    undefined,
    { ttlMs: AGGREGATE_TTL_MS }
  );
}

/** Получить агрегаты по дням для календаря по проекту. */
export async function listProjectCalendar(
  projectId: string,
  filters?: { month?: string } & IndexChecksFilters
): Promise<IndexCheckCalendarDayDTO[]> {
  const encoded = encodeProjectId(projectId);
  const query = buildCalendarQuery(filters);
  return authFetchCached<IndexCheckCalendarDayDTO[]>(
    `/api/projects/${encoded}/index-checks/calendar${query}`,
    undefined,
    { ttlMs: AGGREGATE_TTL_MS }
  );
}

/** Получить историю попыток по домену. */
export async function listDomainHistory(
  domainId: string,
  checkId: string,
  limit?: number
): Promise<IndexCheckHistoryDTO[]> {
  const encodedDomain = encodeDomainId(domainId);
  const encodedCheck = encodeCheckId(checkId);
  const query = buildHistoryQuery(limit);
  return authFetch<IndexCheckHistoryDTO[]>(
    `/api/domains/${encodedDomain}/index-checks/${encodedCheck}/history${query}`
  );
}

/** Получить историю попыток по проекту. */
export async function listProjectHistory(
  projectId: string,
  checkId: string,
  limit?: number
): Promise<IndexCheckHistoryDTO[]> {
  const encodedProject = encodeProjectId(projectId);
  const encodedCheck = encodeCheckId(checkId);
  const query = buildHistoryQuery(limit);
  return authFetch<IndexCheckHistoryDTO[]>(
    `/api/projects/${encodedProject}/index-checks/${encodedCheck}/history${query}`
  );
}

/** Получить историю попыток по проверке (admin). */
export async function listAdminHistory(
  checkId: string,
  limit?: number
): Promise<IndexCheckHistoryDTO[]> {
  const encodedCheck = encodeCheckId(checkId);
  const query = buildHistoryQuery(limit);
  return authFetch<IndexCheckHistoryDTO[]>(
    `/api/admin/index-checks/${encodedCheck}/history${query}`
  );
}

// ─── Index Checker Control ──────────────────────────────────────────────

type IndexCheckerControlDTO = { enabled: boolean };

/** Получить глобальный статус index checker (admin). */
export async function getGlobalIndexCheckerControl(): Promise<IndexCheckerControlDTO> {
  return authFetch<IndexCheckerControlDTO>(`/api/admin/index-checker/control`);
}

/** Установить глобальный статус index checker (admin). */
export async function setGlobalIndexCheckerControl(enabled: boolean): Promise<IndexCheckerControlDTO> {
  return authFetch<IndexCheckerControlDTO>(`/api/admin/index-checker/control`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled })
  });
}

/** Получить статус index checker для проекта. */
export async function getProjectIndexCheckerControl(projectId: string): Promise<IndexCheckerControlDTO> {
  const encoded = encodeProjectId(projectId);
  return authFetch<IndexCheckerControlDTO>(`/api/projects/${encoded}/index-checker`);
}

/** Установить статус index checker для проекта. */
export async function setProjectIndexCheckerControl(projectId: string, enabled: boolean): Promise<IndexCheckerControlDTO> {
  const encoded = encodeProjectId(projectId);
  return authFetch<IndexCheckerControlDTO>(`/api/projects/${encoded}/index-checker`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled })
  });
}

/** Получить статус index checker для домена. */
export async function getDomainIndexCheckerControl(domainId: string): Promise<IndexCheckerControlDTO> {
  const encoded = encodeDomainId(domainId);
  return authFetch<IndexCheckerControlDTO>(`/api/domains/${encoded}/index-checker`);
}

/** Установить статус index checker для домена. */
export async function setDomainIndexCheckerControl(domainId: string, enabled: boolean): Promise<IndexCheckerControlDTO> {
  const encoded = encodeDomainId(domainId);
  return authFetch<IndexCheckerControlDTO>(`/api/domains/${encoded}/index-checker`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ enabled })
  });
}
