export type QueueTab = "domains" | "links";

export function resolveQueueTab(value: string | null | undefined, fallback: QueueTab = "domains"): QueueTab {
  const normalized = (value || "").trim().toLowerCase();
  if (normalized === "links") {
    return "links";
  }
  if (normalized === "domains") {
    return "domains";
  }
  return fallback;
}

export function hasNextPageByPageSize(itemsLength: number, pageSize: number): boolean {
  return itemsLength >= pageSize;
}

export function getTotalPages(totalItems: number, pageSize: number): number {
  if (!Number.isFinite(totalItems) || totalItems <= 0) {
    return 1;
  }
  if (!Number.isFinite(pageSize) || pageSize <= 0) {
    return 1;
  }
  return Math.max(1, Math.ceil(totalItems / pageSize));
}

export function hasNextPageByTotal(page: number, pageSize: number, totalItems: number): boolean {
  if (!Number.isFinite(page) || page < 1) {
    return false;
  }
  return page * pageSize < totalItems;
}

export const QUEUE_LINK_STATUS_LABELS: Record<string, string> = {
  all: "Все",
  pending: "Ожидает",
  searching: "Поиск",
  removing: "Удаление",
  inserted: "Вставлено",
  generated: "Вставлено (ген. текст)",
  removed: "Удалено",
  failed: "Ошибка"
};

export const PROJECT_QUEUE_STATUS_LABELS: Record<string, string> = {
  all: "Все",
  pending: "Ожидает",
  queued: "В очереди"
};

export const PROJECT_QUEUE_HISTORY_STATUS_LABELS: Record<string, string> = {
  all: "Все",
  completed: "Обработано",
  failed: "Ошибка"
};
