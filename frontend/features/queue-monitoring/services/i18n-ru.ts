export type QueueMonitoringFlowStatus = "idle" | "validating" | "sending" | "done" | "error";

export const queueMonitoringRu = {
  flowTitles: {
    refresh: "Обновление",
    links: "Операции со ссылками",
    queue: "Операции очереди",
    monitoring: "Мониторинг",
    manualRun: "Ручной запуск"
  },
  flowStatusLabels: {
    idle: "Ожидание",
    validating: "Проверка",
    sending: "Выполняется",
    done: "Готово",
    error: "Ошибка"
  } as Record<QueueMonitoringFlowStatus, string>,
  diagnostics: {
    title: "Диагностика",
    empty: "Диагностические данные отсутствуют."
  },
  lockReasons: {
    refreshInFlight: "Обновление уже выполняется.",
    retryInFlight: "Повтор уже выполняется.",
    deleteInFlight: "Удаление уже выполняется.",
    cleanupInFlight: "Очистка уже выполняется.",
    manualRunInFlight: "Запуск уже выполняется."
  }
} as const;

export function getQueueMonitoringFlowStatusLabel(status: QueueMonitoringFlowStatus): string {
  return queueMonitoringRu.flowStatusLabels[status];
}

export function toDiagnosticsText(value: unknown): string | undefined {
  if (value === null || value === undefined) {
    return undefined;
  }
  if (typeof value === "string") {
    const trimmed = value.trim();
    return trimmed || undefined;
  }
  if (value instanceof Error) {
    return value.stack || value.message || undefined;
  }
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
