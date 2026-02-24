export type GenerationStatusTone = "slate" | "amber" | "green" | "yellow" | "orange" | "red";
export type GenerationStatusIcon = "clock" | "play" | "pause" | "check" | "alert" | "x";

type GenerationStatusMeta = {
  text: string;
  tone: GenerationStatusTone;
  icon: GenerationStatusIcon;
};

const GENERATION_STATUS_META: Record<string, GenerationStatusMeta> = {
  waiting: { text: "Ожидает генерацию", tone: "slate", icon: "clock" },
  pending: { text: "В очереди", tone: "amber", icon: "clock" },
  processing: { text: "В работе", tone: "amber", icon: "play" },
  running: { text: "В работе", tone: "amber", icon: "play" },
  published: { text: "Опубликован", tone: "green", icon: "play" },
  draft: { text: "Черновик", tone: "slate", icon: "pause" },
  active: { text: "Активен", tone: "green", icon: "play" },
  paused: { text: "Приостановлено", tone: "slate", icon: "pause" },
  pause_requested: { text: "Пауза запрошена", tone: "yellow", icon: "pause" },
  cancelling: { text: "Отмена...", tone: "orange", icon: "x" },
  cancelled: { text: "Отменено", tone: "red", icon: "x" },
  success: { text: "Готово", tone: "green", icon: "check" },
  error: { text: "Ошибка", tone: "red", icon: "alert" }
};

const FALLBACK_META: GenerationStatusMeta = {
  text: "Неизвестно",
  tone: "slate",
  icon: "clock"
};

export const DOMAIN_PROJECT_CTA = {
  generationRun: "Запустить генерацию",
  generationRerunAll: "Перегенерировать всё",
  generationContinue: "Продолжить генерацию",
  generationResume: "Возобновить",
  generationPause: "Пауза",
  generationPauseRequested: "Пауза запрошена...",
  generationCancel: "Отменить",
  generationCancelling: "Отмена...",
  linkAdd: "Добавить ссылку",
  linkUpdate: "Обновить ссылку",
  linkRemove: "Удалить ссылку",
  linkTaskInProgress: "Задача в работе...",
  linkActionInProgress: "В работе...",
  linkTaskInProgressShort: "Задача в работе",
  runs: "Запуски"
} as const;

export function getGenerationStatusMeta(status?: string | null): GenerationStatusMeta {
  const key = (status || "").toLowerCase();
  if (!key) return FALLBACK_META;
  const meta = GENERATION_STATUS_META[key];
  if (meta) return meta;
  return { ...FALLBACK_META, text: status || FALLBACK_META.text };
}

export function getLinkActionLabel(hasActiveLink: boolean, inProgress: boolean, shortInProgress = false): string {
  if (inProgress) {
    return shortInProgress ? DOMAIN_PROJECT_CTA.linkActionInProgress : DOMAIN_PROJECT_CTA.linkTaskInProgress;
  }
  return hasActiveLink ? DOMAIN_PROJECT_CTA.linkUpdate : DOMAIN_PROJECT_CTA.linkAdd;
}

export function getMainGenerationActionLabel(hasAttempt: boolean, isRegenerate: boolean): string {
  if (!hasAttempt) return DOMAIN_PROJECT_CTA.generationRun;
  if (isRegenerate) return DOMAIN_PROJECT_CTA.generationRerunAll;
  return DOMAIN_PROJECT_CTA.generationContinue;
}
