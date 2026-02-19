import { authFetch } from "./http";
import type {
  LLMPricingDTO,
  LLMUsageFilters,
  LLMUsageListDTO,
  LLMUsageStatsDTO
} from "../types/llmUsage";

const toISO = (value: string | Date) => (typeof value === "string" ? value : value.toISOString());

const buildQuery = (filters?: LLMUsageFilters) => {
  if (!filters) {
    return "";
  }
  const params = new URLSearchParams();
  if (filters.from) params.set("from", toISO(filters.from));
  if (filters.to) params.set("to", toISO(filters.to));
  if (filters.userEmail?.trim()) params.set("user_email", filters.userEmail.trim());
  if (filters.projectId?.trim()) params.set("project_id", filters.projectId.trim());
  if (filters.domainId?.trim()) params.set("domain_id", filters.domainId.trim());
  if (filters.model?.trim()) params.set("model", filters.model.trim());
  if (filters.operation?.trim()) params.set("operation", filters.operation.trim());
  if (filters.status?.trim()) params.set("status", filters.status.trim());
  if (typeof filters.page === "number" && filters.page > 0) params.set("page", String(filters.page));
  if (typeof filters.limit === "number" && filters.limit > 0) params.set("limit", String(filters.limit));
  const qs = params.toString();
  return qs ? `?${qs}` : "";
};

export async function listAdminLLMUsageEvents(filters?: LLMUsageFilters): Promise<LLMUsageListDTO> {
  return authFetch<LLMUsageListDTO>(`/api/admin/llm-usage/events${buildQuery(filters)}`);
}

export async function listAdminLLMUsageStats(filters?: LLMUsageFilters): Promise<LLMUsageStatsDTO> {
  return authFetch<LLMUsageStatsDTO>(`/api/admin/llm-usage/stats${buildQuery(filters)}`);
}

export async function listProjectLLMUsageEvents(projectId: string, filters?: LLMUsageFilters): Promise<LLMUsageListDTO> {
  const id = projectId.trim();
  if (!id) throw new Error("projectId is required");
  return authFetch<LLMUsageListDTO>(`/api/projects/${encodeURIComponent(id)}/llm-usage/events${buildQuery(filters)}`);
}

export async function listProjectLLMUsageStats(projectId: string, filters?: LLMUsageFilters): Promise<LLMUsageStatsDTO> {
  const id = projectId.trim();
  if (!id) throw new Error("projectId is required");
  return authFetch<LLMUsageStatsDTO>(`/api/projects/${encodeURIComponent(id)}/llm-usage/stats${buildQuery(filters)}`);
}

export async function listAdminLLMPricing(): Promise<LLMPricingDTO[]> {
  return authFetch<LLMPricingDTO[]>(`/api/admin/llm-pricing`);
}

export async function updateAdminLLMPricing(
  model: string,
  payload: { provider?: string; input_usd_per_million: number; output_usd_per_million: number }
): Promise<LLMPricingDTO> {
  const cleanModel = model.trim();
  if (!cleanModel) {
    throw new Error("model is required");
  }
  return authFetch<LLMPricingDTO>(`/api/admin/llm-pricing/${encodeURIComponent(cleanModel)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}
