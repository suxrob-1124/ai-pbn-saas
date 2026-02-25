import {
  getLinkTaskStatusMeta,
  normalizeLinkTaskStatus,
  type LinkTaskCanonicalStatus
} from "../../../lib/linkTaskStatus";
import type { IndexCheckStatus } from "../../../types/indexChecks";

type BadgeTone =
  | "indigo"
  | "emerald"
  | "amber"
  | "slate"
  | "red"
  | "green"
  | "yellow"
  | "blue"
  | "orange"
  | "sky";

export type StatusIcon = "clock" | "play" | "pause" | "check" | "alert" | "x" | "refresh";

export type StatusMeta = {
  label: string;
  tone: BadgeTone;
  icon: StatusIcon;
};

export type GlobalGenerationFilterKey = "all" | "pending" | "processing" | "success" | "error";

export const GLOBAL_GENERATION_FILTER_KEYS: GlobalGenerationFilterKey[] = [
  "all",
  "pending",
  "processing",
  "success",
  "error"
];

const GLOBAL_GENERATION_FILTER_LABELS: Record<GlobalGenerationFilterKey, string> = {
  all: "Все",
  pending: "В очереди",
  processing: "В работе",
  success: "Готово",
  error: "Ошибка"
};

const GENERATION_STATUS_META: Record<string, StatusMeta> = {
  waiting: { label: "Ожидает генерацию", tone: "slate", icon: "clock" },
  pending: { label: "В очереди", tone: "amber", icon: "clock" },
  queued: { label: "В очереди", tone: "amber", icon: "clock" },
  processing: { label: "В работе", tone: "amber", icon: "play" },
  running: { label: "В работе", tone: "amber", icon: "play" },
  pause_requested: { label: "Пауза запрошена", tone: "yellow", icon: "pause" },
  paused: { label: "Приостановлено", tone: "slate", icon: "pause" },
  cancelling: { label: "Отмена...", tone: "orange", icon: "x" },
  cancelled: { label: "Отменено", tone: "red", icon: "x" },
  success: { label: "Готово", tone: "green", icon: "check" },
  completed: { label: "Обработано", tone: "green", icon: "check" },
  published: { label: "Опубликовано", tone: "green", icon: "check" },
  error: { label: "Ошибка", tone: "red", icon: "alert" },
  failed: { label: "Ошибка", tone: "red", icon: "alert" }
};

const GENERATION_FILTER_ALIASES: Record<string, GlobalGenerationFilterKey> = {
  waiting: "pending",
  pending: "pending",
  queued: "pending",
  processing: "processing",
  running: "processing",
  pause_requested: "processing",
  paused: "processing",
  cancelling: "processing",
  success: "success",
  completed: "success",
  published: "success",
  error: "error",
  failed: "error",
  cancelled: "error"
};

export function normalizeGenerationStatusForFilter(status?: string | null): GlobalGenerationFilterKey | "" {
  const key = (status || "").trim().toLowerCase();
  if (!key) {
    return "";
  }
  return GENERATION_FILTER_ALIASES[key] || "";
}

export function getGenerationFilterLabel(value: GlobalGenerationFilterKey): string {
  return GLOBAL_GENERATION_FILTER_LABELS[value];
}

export function getGenerationStatusMeta(status?: string | null): StatusMeta {
  const key = (status || "").trim().toLowerCase();
  if (key && GENERATION_STATUS_META[key]) {
    return GENERATION_STATUS_META[key];
  }
  const label = (status || "").trim() || "Неизвестно";
  return { label, tone: "slate", icon: "clock" };
}

export type ProjectQueueActiveFilterKey = "all" | "pending" | "queued";
export type ProjectQueueHistoryFilterKey = "all" | "completed" | "failed";

export const PROJECT_QUEUE_ACTIVE_FILTER_KEYS: ProjectQueueActiveFilterKey[] = ["all", "pending", "queued"];
export const PROJECT_QUEUE_HISTORY_FILTER_KEYS: ProjectQueueHistoryFilterKey[] = ["all", "completed", "failed"];

const PROJECT_QUEUE_ACTIVE_LABELS: Record<ProjectQueueActiveFilterKey, string> = {
  all: "Все",
  pending: "Ожидает",
  queued: "В очереди"
};

const PROJECT_QUEUE_HISTORY_LABELS: Record<ProjectQueueHistoryFilterKey, string> = {
  all: "Все",
  completed: "Обработано",
  failed: "Ошибка"
};

export function normalizeProjectQueueActiveStatus(status?: string | null): ProjectQueueActiveFilterKey | "" {
  const key = (status || "").trim().toLowerCase();
  if (!key) {
    return "";
  }
  if (key === "waiting") {
    return "pending";
  }
  if (key === "pending" || key === "queued") {
    return key;
  }
  return "";
}

export function normalizeProjectQueueHistoryStatus(status?: string | null): ProjectQueueHistoryFilterKey | "" {
  const key = (status || "").trim().toLowerCase();
  if (!key) {
    return "";
  }
  if (key === "success") {
    return "completed";
  }
  if (key === "error") {
    return "failed";
  }
  if (key === "completed" || key === "failed") {
    return key;
  }
  return "";
}

export function getProjectQueueActiveStatusLabel(status?: string | null): string {
  const normalized = normalizeProjectQueueActiveStatus(status);
  if (normalized) {
    return PROJECT_QUEUE_ACTIVE_LABELS[normalized];
  }
  const raw = (status || "").trim();
  return raw || "—";
}

export function getProjectQueueHistoryStatusLabel(status?: string | null): string {
  const normalized = normalizeProjectQueueHistoryStatus(status);
  if (normalized) {
    return PROJECT_QUEUE_HISTORY_LABELS[normalized];
  }
  const raw = (status || "").trim();
  return raw || "—";
}

export const LINK_QUEUE_FILTER_KEYS: Array<"all" | LinkTaskCanonicalStatus> = [
  "all",
  "pending",
  "searching",
  "removing",
  "inserted",
  "generated",
  "removed",
  "failed"
];

export function normalizeLinkQueueStatus(status?: string | null): LinkTaskCanonicalStatus | "" {
  return normalizeLinkTaskStatus(status);
}

export function getLinkQueueStatusLabel(status: "all" | LinkTaskCanonicalStatus | string): string {
  if (status === "all") {
    return "Все";
  }
  return getLinkTaskStatusMeta(status).text;
}

export type IndexCheckCanonicalStatus =
  | "pending"
  | "checking"
  | "success"
  | "failed_investigation";

export const INDEX_CHECK_STATUS_KEYS: IndexCheckCanonicalStatus[] = [
  "pending",
  "checking",
  "success",
  "failed_investigation"
];

const INDEX_CHECK_STATUS_META: Record<IndexCheckCanonicalStatus, StatusMeta> = {
  pending: { label: "Ожидает", tone: "amber", icon: "clock" },
  checking: { label: "Проверяется", tone: "blue", icon: "refresh" },
  success: { label: "Успешно", tone: "green", icon: "check" },
  failed_investigation: { label: "Требует внимания", tone: "red", icon: "alert" }
};

export function normalizeIndexCheckStatus(status?: string | null): IndexCheckCanonicalStatus | "" {
  const key = (status || "").trim().toLowerCase();
  if (!key) {
    return "";
  }
  if (key === "failed" || key === "error") {
    return "failed_investigation";
  }
  if (key === "queued") {
    return "pending";
  }
  if (key === "processing") {
    return "checking";
  }
  if (INDEX_CHECK_STATUS_KEYS.includes(key as IndexCheckCanonicalStatus)) {
    return key as IndexCheckCanonicalStatus;
  }
  return "";
}

export function normalizeIndexCheckStatusList(statuses: IndexCheckStatus[]): IndexCheckStatus[] {
  const normalized = statuses
    .map((item) => normalizeIndexCheckStatus(item) || String(item || "").trim().toLowerCase())
    .filter(Boolean);
  return Array.from(new Set(normalized));
}

export function getIndexCheckStatusMeta(status?: string | null): StatusMeta {
  const normalized = normalizeIndexCheckStatus(status);
  if (normalized) {
    return INDEX_CHECK_STATUS_META[normalized];
  }
  const label = (status || "").trim() || "—";
  return { label, tone: "slate", icon: "clock" };
}

const SCHEDULE_STRATEGY_LABELS: Record<string, string> = {
  immediate: "Сразу",
  daily: "Ежедневно",
  weekly: "Еженедельно",
  custom: "CRON"
};

export function getScheduleStrategyLabel(strategy?: string | null): string {
  const key = (strategy || "").trim().toLowerCase();
  if (!key) {
    return "—";
  }
  return SCHEDULE_STRATEGY_LABELS[key] || strategy || "—";
}

export function getScheduleActivityLabel(isActive: boolean): string {
  return isActive ? "Да" : "Нет";
}

export function getScheduleToggleLabel(isActive: boolean): string {
  return isActive ? "Пауза" : "Активировать";
}

type MonitoringServiceStatus = "ok" | "warn" | "error";

const MONITORING_SERVICE_STATUS_META: Record<MonitoringServiceStatus, StatusMeta> = {
  ok: { label: "ОК", tone: "green", icon: "check" },
  warn: { label: "Внимание", tone: "amber", icon: "alert" },
  error: { label: "Ошибка", tone: "red", icon: "alert" }
};

export function normalizeMonitoringServiceStatus(status?: string | null): MonitoringServiceStatus {
  const key = (status || "").trim().toLowerCase();
  if (key === "ok" || key === "success") {
    return "ok";
  }
  if (key === "error" || key === "failed") {
    return "error";
  }
  return "warn";
}

export function getMonitoringServiceStatusMeta(status?: string | null): StatusMeta {
  return MONITORING_SERVICE_STATUS_META[normalizeMonitoringServiceStatus(status)];
}

export function buildStatusFilterOptions(values: string[], getLabel: (value: string) => string) {
  return values.map((value) => ({ value, label: getLabel(value) }));
}
