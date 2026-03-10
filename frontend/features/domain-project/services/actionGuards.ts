import type { DomainProjectRole } from "../types/view";

export const GENERATION_ACTIVE_STATUSES = ["pending", "processing", "pause_requested", "cancelling"] as const;

export function isGenerationActive(status?: string | null): boolean {
  return Boolean(status && GENERATION_ACTIVE_STATUSES.includes(status as (typeof GENERATION_ACTIVE_STATUSES)[number]));
}

export function isMainGenerationActionDisabled(loading: boolean, status?: string | null): boolean {
  return loading || isGenerationActive(status);
}

export function canEditPromptOverrides(role?: string | null): boolean {
  return role === "admin" || role === "owner";
}

export function canOpenDomainEditor(domain: unknown): boolean {
  return !!domain;
}

export function hasLinkSettings(anchor?: string | null, acceptor?: string | null): boolean {
  return Boolean((anchor || "").trim()) && Boolean((acceptor || "").trim());
}

export function normalizeRole(role?: string | null): DomainProjectRole {
  if (role === "admin" || role === "owner" || role === "editor" || role === "viewer") {
    return role;
  }
  return "viewer";
}
