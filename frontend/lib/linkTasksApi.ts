import { authFetch } from "./http";
import type {
  LinkTaskCreateInput,
  LinkTaskDTO,
  LinkTaskFilters,
  LinkTaskImportInput,
  LinkTaskListParams
} from "../types/linkTasks";

const encodeDomainId = (domainId: string) => {
  const trimmed = domainId.trim();
  if (!trimmed) {
    throw new Error("domainId is required");
  }
  return encodeURIComponent(trimmed);
};

const encodeTaskId = (taskId: string) => {
  const trimmed = taskId.trim();
  if (!trimmed) {
    throw new Error("taskId is required");
  }
  return encodeURIComponent(trimmed);
};

const toISO = (value: string | Date) =>
  typeof value === "string" ? value : value.toISOString();

const buildFilters = (filters?: LinkTaskFilters) => {
  if (!filters) {
    return "";
  }
  const params = new URLSearchParams();
  if (filters.status) {
    params.set("status", filters.status);
  }
  if (filters.scheduledFrom) {
    params.set("scheduled_from", toISO(filters.scheduledFrom));
  }
  if (filters.scheduledTo) {
    params.set("scheduled_to", toISO(filters.scheduledTo));
  }
  if (typeof filters.limit === "number") {
    params.set("limit", String(filters.limit));
  }
  const query = params.toString();
  return query ? `?${query}` : "";
};

const normalizeAnchor = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("anchorText is required");
  }
  return trimmed;
};

const normalizeTarget = (value: string) => {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("targetUrl is required");
  }
  return trimmed;
};

const buildScheduledFor = (value?: string | Date) =>
  value ? toISO(value) : undefined;

const buildCreatePayload = (input: LinkTaskCreateInput) => {
  const payload: Record<string, unknown> = {
    anchor_text: normalizeAnchor(input.anchorText),
    target_url: normalizeTarget(input.targetUrl)
  };
  const scheduledFor = buildScheduledFor(input.scheduledFor);
  if (scheduledFor) {
    payload.scheduled_for = scheduledFor;
  }
  return payload;
};

const buildImportPayload = (input: LinkTaskImportInput) => {
  const payload: Record<string, unknown> = {};
  if (input.items && input.items.length > 0) {
    payload.items = input.items
      .map((item) => {
        if (!item.anchorText || !item.targetUrl) {
          return null;
        }
        return {
          anchor_text: normalizeAnchor(item.anchorText),
          target_url: normalizeTarget(item.targetUrl),
          scheduled_for: buildScheduledFor(item.scheduledFor)
        };
      })
      .filter(Boolean);
  }
  if (input.text && input.text.trim()) {
    payload.text = input.text.trim();
  }
  if (!payload.items && !payload.text) {
    throw new Error("items or text is required for import");
  }
  return payload;
};

const buildListParams = (params?: LinkTaskListParams) => {
  if (!params) {
    return "";
  }
  const query = new URLSearchParams();
  if (params.domainId) {
    query.set("domain_id", params.domainId.trim());
  }
  if (params.projectId) {
    query.set("project_id", params.projectId.trim());
  }
  if (params.status) {
    query.set("status", params.status);
  }
  if (params.scheduledFrom) {
    query.set("scheduled_from", toISO(params.scheduledFrom));
  }
  if (params.scheduledTo) {
    query.set("scheduled_to", toISO(params.scheduledTo));
  }
  if (typeof params.limit === "number") {
    query.set("limit", String(params.limit));
  }
  const queryString = query.toString();
  return queryString ? `?${queryString}` : "";
};

/** Получить список link tasks для домена. */
export async function listDomainLinkTasks(
  domainId: string,
  filters?: LinkTaskFilters
): Promise<LinkTaskDTO[]> {
  const encoded = encodeDomainId(domainId);
  const query = buildFilters(filters);
  return authFetch<LinkTaskDTO[]>(`/api/domains/${encoded}/links${query}`);
}

/** Создать link task. */
export async function createLinkTask(
  domainId: string,
  input: LinkTaskCreateInput
): Promise<LinkTaskDTO> {
  const encoded = encodeDomainId(domainId);
  const payload = buildCreatePayload(input);
  return authFetch<LinkTaskDTO>(`/api/domains/${encoded}/links`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

/** Импортировать link tasks для домена. */
export async function importLinkTasks(
  domainId: string,
  input: LinkTaskImportInput
): Promise<{ created: number }> {
  const encoded = encodeDomainId(domainId);
  const payload = buildImportPayload(input);
  return authFetch<{ created: number }>(`/api/domains/${encoded}/links/import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

/** Получить список link tasks (admin/all/projects/domains). */
export async function listLinkTasks(params?: LinkTaskListParams): Promise<LinkTaskDTO[]> {
  const query = buildListParams(params);
  return authFetch<LinkTaskDTO[]>(`/api/links${query}`);
}

/** Обновить дату запуска link task. */
export async function updateLinkTask(
  taskId: string,
  scheduledFor: string | Date
): Promise<LinkTaskDTO> {
  const encoded = encodeTaskId(taskId);
  const payload = { scheduled_for: toISO(scheduledFor) };
  return authFetch<LinkTaskDTO>(`/api/links/${encoded}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
}

/** Удалить link task. */
export async function deleteLinkTask(taskId: string): Promise<{ status: string }> {
  const encoded = encodeTaskId(taskId);
  return authFetch<{ status: string }>(`/api/links/${encoded}`, { method: "DELETE" });
}

/** Повторить link task. */
export async function retryLinkTask(taskId: string): Promise<LinkTaskDTO> {
  const encoded = encodeTaskId(taskId);
  return authFetch<LinkTaskDTO>(`/api/links/${encoded}/retry`, { method: "POST" });
}
