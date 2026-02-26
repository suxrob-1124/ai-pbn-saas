import { canDeleteLinkTask, canRetryLinkTask } from "../../../lib/linkTaskStatus";

export type ActionGuard = {
  enabled: boolean;
  disabled: boolean;
  reason?: string;
};

const GUARD_REASON = {
  busy: "Дождитесь завершения текущей операции.",
  retryOnlyFailed: "Повтор доступен только для задач со статусом «Ошибка».",
  deleteActive: "Удаление недоступно для активных задач (ожидает/поиск/удаление).",
  cancelUnavailable: "Отмена доступна только для активных задач.",
  unavailable: "Действие недоступно в текущем состоянии."
} as const;

function blocked(reason: string): ActionGuard {
  return { enabled: false, disabled: true, reason };
}

function allowed(): ActionGuard {
  return { enabled: true, disabled: false };
}

type GuardOptions = {
  busy?: boolean;
  busyReason?: string;
  allowed?: boolean;
  reason?: string;
};

export function canRun(options: GuardOptions = {}): ActionGuard {
  if (options.busy) {
    return blocked(options.busyReason || GUARD_REASON.busy);
  }
  if (options.allowed === false) {
    return blocked(options.reason || GUARD_REASON.unavailable);
  }
  return allowed();
}

type RetryGuardOptions = GuardOptions & {
  status?: string | null;
};

export function canRetry(options: RetryGuardOptions = {}): ActionGuard {
  if (options.busy) {
    return blocked(options.busyReason || GUARD_REASON.busy);
  }
  const byStatus = options.status ? canRetryLinkTask(options.status) : true;
  const isAllowed = options.allowed ?? byStatus;
  if (!isAllowed) {
    return blocked(options.reason || GUARD_REASON.retryOnlyFailed);
  }
  return allowed();
}

type DeleteGuardOptions = GuardOptions & {
  status?: string | null;
};

export function canDelete(options: DeleteGuardOptions = {}): ActionGuard {
  if (options.busy) {
    return blocked(options.busyReason || GUARD_REASON.busy);
  }
  const byStatus = options.status ? canDeleteLinkTask(options.status) : true;
  const isAllowed = options.allowed ?? byStatus;
  if (!isAllowed) {
    return blocked(options.reason || GUARD_REASON.deleteActive);
  }
  return allowed();
}

type CancelGuardOptions = GuardOptions & {
  status?: string | null;
};

const CANCELABLE_STATUSES = new Set([
  "pending",
  "processing",
  "pause_requested",
  "cancelling",
  "running",
  "searching",
  "removing",
  "checking"
]);

export function canCancel(options: CancelGuardOptions = {}): ActionGuard {
  if (options.busy) {
    return blocked(options.busyReason || GUARD_REASON.busy);
  }
  const normalizedStatus = (options.status || "").trim().toLowerCase();
  const byStatus = normalizedStatus ? CANCELABLE_STATUSES.has(normalizedStatus) : true;
  const isAllowed = options.allowed ?? byStatus;
  if (!isAllowed) {
    return blocked(options.reason || GUARD_REASON.cancelUnavailable);
  }
  return allowed();
}
