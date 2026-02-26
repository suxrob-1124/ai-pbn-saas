import { hasInsertedLink, isLinkTaskInProgress, normalizeLinkTaskStatus } from "../../../lib/linkTaskStatus";
import { getMainGenerationActionLabel } from "./statusCta";
import type { DomainLinkView, LinkTaskView } from "../types/view";

export function getEffectiveDomainLinkStatus(domain: Pick<DomainLinkView, "link_status_effective" | "link_status"> | null | undefined): string {
  return domain?.link_status_effective || domain?.link_status || "";
}

export function getDomainLinkBadgeStatus(domain: DomainLinkView | null | undefined): string {
  const effective = getEffectiveDomainLinkStatus(domain);
  if (effective) return effective;
  return domain?.link_last_task_id ? "pending" : "ready";
}

export function deriveMainGenerationMeta(
  latestAttempt: null | undefined,
  generations: []
): { currentAttempt: null; isRegenerate: boolean; mainButtonText: string };
export function deriveMainGenerationMeta<T extends { status: string }>(
  latestAttempt: T | null | undefined,
  generations: T[]
): { currentAttempt: T | null; isRegenerate: boolean; mainButtonText: string };
export function deriveMainGenerationMeta<T extends { status: string }>(
  latestAttempt: T | null | undefined,
  generations: T[]
): { currentAttempt: T | null; isRegenerate: boolean; mainButtonText: string } {
  const currentAttempt = latestAttempt || generations[0] || null;
  const isRegenerate = Boolean(currentAttempt && currentAttempt.status === "success");
  const mainButtonText = getMainGenerationActionLabel(Boolean(currentAttempt), isRegenerate);
  return { currentAttempt, isRegenerate, mainButtonText };
}

export function deriveDomainLinkActionMeta(
  domain: DomainLinkView | null | undefined,
  linkTasks: LinkTaskView[]
): {
  effectiveStatus: string;
  normalizedStatus: string;
  hasActiveLink: boolean;
  hasLinkInTasks: boolean;
  linkInProgress: boolean;
  canRemoveLink: boolean;
} {
  const effectiveStatus = getEffectiveDomainLinkStatus(domain);
  const normalizedStatus = normalizeLinkTaskStatus(effectiveStatus);
  const hasActiveLink = hasInsertedLink(effectiveStatus);
  const hasLinkInTasks =
    !normalizedStatus && linkTasks.some((task) => (task.action || "insert") !== "remove" && hasInsertedLink(task.status));
  const linkInProgress =
    isLinkTaskInProgress(effectiveStatus) ||
    linkTasks.some((task) => isLinkTaskInProgress(task.status));
  const canRemoveLink = (hasActiveLink || hasLinkInTasks) && !linkInProgress;

  return {
    effectiveStatus,
    normalizedStatus,
    hasActiveLink,
    hasLinkInTasks,
    linkInProgress,
    canRemoveLink
  };
}

export function getLinkTaskSteps(action?: string): Array<{ id: string; label: string }> {
  const isRemove = (action || "insert") === "remove";
  if (isRemove) {
    return [
      { id: "pending", label: "В очереди" },
      { id: "searching", label: "Поиск ссылки" },
      { id: "removing", label: "Удаление" },
      { id: "removed", label: "Удалено" }
    ];
  }
  return [
    { id: "pending", label: "В очереди" },
    { id: "searching", label: "Поиск места" },
    { id: "inserted", label: "Вставка ссылки" },
    { id: "generated", label: "Генерация текста" }
  ];
}
