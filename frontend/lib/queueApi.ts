import { authFetch } from "./http";
import type { QueueItemDTO } from "../types/queue";

const encodeProjectId = (projectId: string) => {
  const trimmed = projectId.trim();
  if (!trimmed) {
    throw new Error("projectId is required");
  }
  return encodeURIComponent(trimmed);
};

const encodeQueueItemId = (itemId: string) => {
  const trimmed = itemId.trim();
  if (!trimmed) {
    throw new Error("queue item id is required");
  }
  return encodeURIComponent(trimmed);
};

type QueueListParams = {
  limit?: number;
  page?: number;
  search?: string;
  status?: "all" | "completed" | "failed";
  dateFrom?: string;
  dateTo?: string;
};

const buildQueueParams = (params?: QueueListParams) => {
  if (!params) {
    return "";
  }
  const query = new URLSearchParams();
  if (typeof params.limit === "number") {
    query.set("limit", String(params.limit));
  }
  if (typeof params.page === "number") {
    query.set("page", String(params.page));
  }
  if (params.search && params.search.trim()) {
    query.set("search", params.search.trim());
  }
  if (params.status && params.status !== "all") {
    query.set("status", params.status);
  }
  if (params.dateFrom && params.dateFrom.trim()) {
    query.set("date_from", params.dateFrom.trim());
  }
  if (params.dateTo && params.dateTo.trim()) {
    query.set("date_to", params.dateTo.trim());
  }
  const qs = query.toString();
  return qs ? `?${qs}` : "";
};

/** Получить очередь генераций проекта. */
export async function listQueue(projectId: string, params?: QueueListParams): Promise<QueueItemDTO[]> {
  const encoded = encodeProjectId(projectId);
  const query = buildQueueParams(params);
  return authFetch<QueueItemDTO[]>(`/api/projects/${encoded}/queue${query}`);
}

/** Получить историю очереди генераций проекта. */
export async function listQueueHistory(projectId: string, params?: QueueListParams): Promise<QueueItemDTO[]> {
  const encoded = encodeProjectId(projectId);
  const query = buildQueueParams(params);
  return authFetch<QueueItemDTO[]>(`/api/projects/${encoded}/queue/history${query}`);
}

/** Очистить устаревшие элементы очереди. */
export async function cleanupQueue(projectId: string): Promise<{ removed: number }> {
  const encoded = encodeProjectId(projectId);
  return authFetch<{ removed: number }>(`/api/projects/${encoded}/queue/cleanup`, {
    method: "POST"
  });
}

/** Удалить элемент из очереди. */
export async function deleteQueueItem(itemId: string): Promise<{ status: string }> {
  const encoded = encodeQueueItemId(itemId);
  return authFetch<{ status: string }>(`/api/queue/${encoded}`, { method: "DELETE" });
}
