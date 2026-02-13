export type LinkTaskCanonicalStatus =
  | "pending"
  | "searching"
  | "removing"
  | "inserted"
  | "generated"
  | "removed"
  | "failed";

export type LinkStatusTone = "amber" | "blue" | "orange" | "green" | "yellow" | "slate" | "red";
export type LinkStatusIcon = "clock" | "refresh" | "check" | "alert";

type StatusMeta = {
  text: string;
  tone: LinkStatusTone;
  icon: LinkStatusIcon;
};

const TASK_STATUS_META: Record<LinkTaskCanonicalStatus, StatusMeta> = {
  pending: { text: "Ожидает", tone: "amber", icon: "clock" },
  searching: { text: "Поиск", tone: "blue", icon: "refresh" },
  removing: { text: "Удаление", tone: "orange", icon: "refresh" },
  inserted: { text: "Вставлено", tone: "green", icon: "check" },
  generated: { text: "Вставлено (ген. текст)", tone: "yellow", icon: "check" },
  removed: { text: "Удалено", tone: "slate", icon: "check" },
  failed: { text: "Ошибка", tone: "red", icon: "alert" },
};

export const LINK_TASK_ACTIVE_STATUSES: LinkTaskCanonicalStatus[] = ["pending", "searching", "removing"];
export const LINK_TASK_HAS_LINK_STATUSES: LinkTaskCanonicalStatus[] = ["inserted", "generated"];

export function normalizeLinkTaskStatus(status?: string | null): LinkTaskCanonicalStatus | "" {
  const normalized = (status || "").trim().toLowerCase();
  if (!normalized) {
    return "";
  }
  if (normalized === "found") {
    // legacy compatibility: old status treated as searching
    return "searching";
  }
  if ((Object.keys(TASK_STATUS_META) as LinkTaskCanonicalStatus[]).includes(normalized as LinkTaskCanonicalStatus)) {
    return normalized as LinkTaskCanonicalStatus;
  }
  return "";
}

export function isLinkTaskInProgress(status?: string | null): boolean {
  const normalized = normalizeLinkTaskStatus(status);
  return Boolean(normalized && LINK_TASK_ACTIVE_STATUSES.includes(normalized));
}

export function hasInsertedLink(status?: string | null): boolean {
  const normalized = normalizeLinkTaskStatus(status);
  return Boolean(normalized && LINK_TASK_HAS_LINK_STATUSES.includes(normalized));
}

export function getLinkTaskStatusMeta(status?: string | null): StatusMeta {
  const normalized = normalizeLinkTaskStatus(status);
  if (normalized && TASK_STATUS_META[normalized]) {
    return TASK_STATUS_META[normalized];
  }
  const text = (status || "").trim() || "—";
  return { text, tone: "slate", icon: "clock" };
}

export function getDomainLinkStatusMeta(status: string | undefined, hasSettings: boolean): StatusMeta {
  if (!hasSettings) {
    return { text: "Ожидает настройки", tone: "slate", icon: "clock" };
  }
  const normalized = normalizeLinkTaskStatus(status);
  if (normalized) {
    return TASK_STATUS_META[normalized];
  }
  const raw = (status || "").trim().toLowerCase();
  if (raw === "needs_relink") {
    return { text: "Требуется обновление", tone: "amber", icon: "clock" };
  }
  return { text: "Готово к запуску", tone: "slate", icon: "clock" };
}
