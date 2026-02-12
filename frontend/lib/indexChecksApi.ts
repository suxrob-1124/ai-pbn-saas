import { authFetch } from "./http";
import type {
  IndexCheckBatchResponse,
  IndexCheckDTO,
  IndexCheckHistoryDTO,
  IndexChecksFilters
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

/** Получить список проверок индексации по домену. */
export async function listByDomain(
  domainId: string,
  filters?: IndexChecksFilters
): Promise<IndexCheckDTO[]> {
  const encoded = encodeDomainId(domainId);
  const query = buildFilters(filters);
  return authFetch<IndexCheckDTO[]>(`/api/domains/${encoded}/index-checks${query}`);
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
): Promise<IndexCheckDTO[]> {
  const encoded = encodeProjectId(projectId);
  const query = buildFilters(filters);
  return authFetch<IndexCheckDTO[]>(`/api/projects/${encoded}/index-checks${query}`);
}

/** Запустить ручные проверки индексации по проекту. */
export async function runManualProject(projectId: string): Promise<IndexCheckBatchResponse> {
  const encoded = encodeProjectId(projectId);
  return authFetch<IndexCheckBatchResponse>(`/api/projects/${encoded}/index-checks`, {
    method: "POST"
  });
}

/** Получить список проверок индексации (admin). */
export async function listAdmin(filters?: IndexChecksFilters): Promise<IndexCheckDTO[]> {
  const query = buildFilters(filters, { allowDomainId: true });
  return authFetch<IndexCheckDTO[]>(`/api/admin/index-checks${query}`);
}

/** Получить список проблемных проверок индексации (admin). */
export async function listFailed(filters?: IndexChecksFilters): Promise<IndexCheckDTO[]> {
  const query = buildFilters(filters, { allowDomainId: true });
  return authFetch<IndexCheckDTO[]>(`/api/admin/index-checks/failed${query}`);
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
