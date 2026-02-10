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

/** Получить очередь генераций проекта. */
export async function listQueue(projectId: string): Promise<QueueItemDTO[]> {
  const encoded = encodeProjectId(projectId);
  return authFetch<QueueItemDTO[]>(`/api/projects/${encoded}/queue`);
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
